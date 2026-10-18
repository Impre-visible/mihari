<script lang="ts">
  import { CheckCircleIcon, PaperPlaneRightIcon, TrashIcon, WarningCircleIcon } from "phosphor-svelte";
  import { Events } from "@wailsio/runtime";
  import { Service as Tasks } from "../../../bindings/github.com/Impre-visible/mihari/internal/tasks";
  import type { Todo } from "../../../bindings/github.com/Impre-visible/mihari/internal/gitlab/models";
  import type { Snapshot } from "../../../bindings/github.com/Impre-visible/mihari/internal/tasks/models";
  import { Service as Scheduler } from "../../../bindings/github.com/Impre-visible/mihari/internal/scheduler";
  import type {
    ChatResult,
    Snapshot as EngineSnapshot,
  } from "../../../bindings/github.com/Impre-visible/mihari/internal/scheduler/models";
  import { Button, Checkbox, FormGroup, FormRow, Spinner, TextArea, TextField } from "$lib/ui";

  type Status =
    | { kind: "idle" }
    | { kind: "busy"; label: string }
    | { kind: "ok"; message: string }
    | { kind: "error"; message: string };

  function errorMessage(err: unknown) {
    return err instanceof Error ? err.message : String(err);
  }

  // Simulated ping

  let mrURL = $state("");
  let author = $state("");
  let mention = $state("");
  let pingStatus = $state<Status>({ kind: "idle" });
  let simulated = $state<{ todo: Todo; where: string }[]>([]);

  // Simulated todos go through the same sorting as real ones.
  function keepSimulated(s: Snapshot) {
    simulated = [
      ...(s.todos ?? []).map((todo) => ({ todo, where: "prête" })),
      ...(s.waiting ?? []).map((w) => ({ todo: w.todo, where: w.reason })),
    ].filter((entry) => entry.todo.simulated);
  }

  async function simulate() {
    pingStatus = { kind: "busy", label: "Lecture de la MR…" };
    try {
      const todo = await Tasks.Simulate(mrURL.trim(), author.trim(), mention);
      pingStatus = { kind: "ok", message: `Tâche ajoutée : ${todo.project}!${todo.mrIid}. Voir la page État.` };
    } catch (err) {
      pingStatus = { kind: "error", message: errorMessage(err) };
    }
  }

  // Model test

  let engine = $state<EngineSnapshot | null>(null);
  let message = $state("Présente-toi en une phrase.");
  let withProfilePrompt = $state(false);
  let chatStatus = $state<Status>({ kind: "idle" });
  let result = $state<ChatResult | null>(null);

  async function chat() {
    chatStatus = { kind: "busy", label: "Le modèle répond…" };
    result = null;
    try {
      result = await Scheduler.Chat(message, withProfilePrompt);
      chatStatus = { kind: "idle" };
    } catch (err) {
      chatStatus = { kind: "error", message: errorMessage(err) };
    }
  }

  Tasks.Snapshot().then(keepSimulated);
  Scheduler.Snapshot().then((s) => (engine = s));

  $effect(() => {
    const offTasks = Events.On("tasks:updated", (event) => keepSimulated(event.data));
    const offEngine = Events.On("engine:updated", (event) => (engine = event.data));
    return () => {
      offTasks();
      offEngine();
    };
  });
</script>

{#snippet statusLine(status: Status)}
  <p class="p-subhead-2 flex min-w-0 flex-1 items-center gap-1.5" role="status">
    {#if status.kind === "busy"}
      <Spinner small label={status.label} />
      <span class="text-secondary-label">{status.label}</span>
    {:else if status.kind === "ok"}
      <CheckCircleIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-green)" aria-hidden="true" />
      <span class="text-secondary-label">{status.message}</span>
    {:else if status.kind === "error"}
      <WarningCircleIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-red)" aria-hidden="true" />
      <span class="text-label select-text">{status.message}</span>
    {/if}
  </p>
{/snippet}

<p class="p-body text-secondary-label">Outils de test, visibles seulement avec wails3 dev.</p>

<form
  class="mt-6"
  oninput={() => pingStatus.kind !== "busy" && (pingStatus = { kind: "idle" })}
  onsubmit={(e) => {
    e.preventDefault();
    simulate();
  }}
>
  <FormGroup
    title="Simuler un ping"
    footer="Crée la tâche qu'un ping créerait, sans rien poster sur GitLab. Sans mention, c'est une demande de review."
  >
    <FormRow label="URL de la MR" for="dev-mr">
      <TextField
        id="dev-mr"
        type="url"
        class="w-96"
        placeholder="https://gitlab.com/groupe/projet/-/merge_requests/1"
        bind:value={mrURL}
      />
    </FormRow>
    <FormRow label="Auteur du ping" for="dev-author">
      <TextField id="dev-author" class="w-96" placeholder="Le bot, si vide" bind:value={author} />
    </FormRow>
    <div class="p-form-row flex-col items-stretch! py-2.5">
      <label class="p-form-row-label" for="dev-mention">Texte de la mention</label>
      <TextArea
        id="dev-mention"
        rows={3}
        class="w-full resize-y"
        placeholder="@bot ajoute des tests pour le parser"
        bind:value={mention}
      />
    </div>
  </FormGroup>

  <div class="mt-4 flex items-center gap-2">
    {@render statusLine(pingStatus)}
    <Button type="submit" variant="primary" disabled={mrURL.trim() === "" || pingStatus.kind === "busy"}>
      Créer la tâche
    </Button>
  </div>
</form>

{#if simulated.length > 0}
  <FormGroup title="Tâches simulées ({simulated.length})" class="mt-6">
    {#each simulated as { todo, where } (todo.id)}
      <FormRow label={todo.title} detail="{todo.project}!{todo.mrIid} · @{todo.author} · {where}">
        <Button
          size="small"
          variant="borderless"
          iconOnly
          aria-label="Retirer la tâche"
          onclick={() => Tasks.RemoveSimulated(todo.id)}
        >
          <TrashIcon size={14} aria-hidden="true" />
        </Button>
      </FormRow>
    {/each}
  </FormGroup>
{/if}

<form
  class="mt-8"
  onsubmit={(e) => {
    e.preventDefault();
    chat();
  }}
>
  <FormGroup
    title="Tester le modèle"
    footer={engine?.state === "ready"
      ? `Envoie le message à ${engine.model} (${engine.profileName}).`
      : "Aucun moteur prêt pour l'instant : voir la page État."}
  >
    <div class="p-form-row flex-col items-stretch! py-2.5">
      <label class="p-form-row-label" for="dev-message">Message</label>
      <TextArea id="dev-message" rows={3} class="w-full resize-y" bind:value={message} />
    </div>
    <FormRow label="Envoyer aussi le prompt du profil" for="dev-system">
      <Checkbox id="dev-system" bind:checked={withProfilePrompt} />
    </FormRow>
  </FormGroup>

  <div class="mt-4 flex items-center gap-2">
    {@render statusLine(chatStatus)}
    <Button
      type="submit"
      variant="primary"
      disabled={engine?.state !== "ready" || message.trim() === "" || chatStatus.kind === "busy"}
    >
      <PaperPlaneRightIcon size={14} aria-hidden="true" />
      Envoyer
    </Button>
  </div>
</form>

{#if result}
  <FormGroup
    title="Réponse"
    class="mt-6"
    footer="{(result.durationMs / 1000).toFixed(1)} s · {result.promptTokens} tokens en entrée · {result.completionTokens} en sortie"
  >
    {#if result.reasoning}
      <details class="p-form-row flex-col items-stretch! py-2.5">
        <summary class="p-form-row-label cursor-default text-secondary-label">Réflexion</summary>
        <p class="p-subhead-2 mt-2 whitespace-pre-wrap text-secondary-label select-text">{result.reasoning}</p>
      </details>
    {/if}
    <p class="p-form-row p-body py-2.5 whitespace-pre-wrap select-text">{result.text || "(réponse vide)"}</p>
  </FormGroup>
{/if}
