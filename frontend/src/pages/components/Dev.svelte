<script lang="ts">
  import { ArrowSquareOutIcon, AtIcon, CheckCircleIcon, PaperPlaneRightIcon, TrashIcon, WarningCircleIcon } from "phosphor-svelte";
  import { Browser, Events } from "@wailsio/runtime";
  import { Service as Devlab } from "../../../bindings/github.com/Impre-visible/mihari/internal/devlab";
  import type { TestItem, TestProject } from "../../../bindings/github.com/Impre-visible/mihari/internal/gitlab/models";
  import { load as loadGitLab } from "../gitlab/store.svelte";
  import { Service as Tasks } from "../../../bindings/github.com/Impre-visible/mihari/internal/tasks";
  import type { Todo } from "../../../bindings/github.com/Impre-visible/mihari/internal/gitlab/models";
  import type { Snapshot } from "../../../bindings/github.com/Impre-visible/mihari/internal/tasks/models";
  import { Service as Scheduler } from "../../../bindings/github.com/Impre-visible/mihari/internal/scheduler";
  import type {
    ChatResult,
    Snapshot as EngineSnapshot,
  } from "../../../bindings/github.com/Impre-visible/mihari/internal/scheduler/models";
  import { Service as Projects } from "../../../bindings/github.com/Impre-visible/mihari/internal/projects";
  import type { View as ProjectView } from "../../../bindings/github.com/Impre-visible/mihari/internal/projects/models";
  import { Service as Engine } from "../../../bindings/github.com/Impre-visible/mihari/internal/engine";
  import { Service as Sandbox } from "../../../bindings/github.com/Impre-visible/mihari/internal/sandbox";
  import type { TryResult } from "../../../bindings/github.com/Impre-visible/mihari/internal/sandbox/models";
  import { Button, Checkbox, FormGroup, FormRow, Select, Spinner, TextArea, TextField } from "$lib/ui";
  import { ref } from "$lib/refs";

  type Status =
    | { kind: "idle" }
    | { kind: "busy"; label: string }
    | { kind: "ok"; message: string }
    | { kind: "error"; message: string };

  function errorMessage(err: unknown) {
    return err instanceof Error ? err.message : String(err);
  }

  // Remote inference

  // Mirrors OpenCodeGoURL and OpenCodeZenURL in internal/engine/remote.go.
  const OFFERS = [
    { value: "https://opencode.ai/inference/go", label: "OpenCode Go (abonnement)" },
    { value: "https://opencode.ai/zen", label: "OpenCode Zen (crédits)" },
    { value: "custom", label: "Autre API compatible OpenAI" },
  ];

  let remoteOffer = $state(OFFERS[0].value);
  let remoteURL = $state("");
  let remoteKey = $state("");
  let hasRemoteKey = $state(false);
  let remoteModel = $state("glm-5.3");

  function chosenURL() {
    return remoteOffer === "custom" ? remoteURL.trim() : remoteOffer;
  }
  let remoteStatus = $state<Status>({ kind: "idle" });

  Engine.RemoteSettings()
    .then((r) => {
      const known = OFFERS.some((o) => o.value === r.url);
      remoteOffer = known ? r.url : "custom";
      remoteURL = known ? "" : r.url;
      hasRemoteKey = r.hasKey;
    })
    .catch((err) => console.error("Load remote settings:", err));

  async function saveRemote() {
    remoteStatus = { kind: "busy", label: "Enregistrement…" };
    try {
      await Engine.SetRemote(chosenURL(), remoteKey.trim());
      hasRemoteKey ||= remoteKey.trim() !== "";
      remoteKey = "";
      remoteStatus = {
        kind: "ok",
        message: "Enregistré. Choisis le moteur OpenCode dans un profil pour l'utiliser.",
      };
    } catch (err) {
      remoteStatus = { kind: "error", message: errorMessage(err) };
    }
  }

  async function testRemote() {
    remoteStatus = { kind: "busy", label: "Le modèle distant répond…" };
    try {
      const r = await Engine.TestRemote(chosenURL(), remoteKey.trim(), remoteModel.trim());
      remoteStatus = {
        kind: "ok",
        message: `${remoteModel.trim()} a répondu en ${(r.durationMs / 1000).toFixed(1)} s : ${r.text.trim() || "(réponse vide)"}`,
      };
    } catch (err) {
      remoteStatus = { kind: "error", message: errorMessage(err) };
    }
  }

  async function forgetRemoteKey() {
    try {
      await Engine.ForgetRemoteKey();
      hasRemoteKey = false;
      remoteStatus = { kind: "ok", message: "Clé supprimée." };
    } catch (err) {
      remoteStatus = { kind: "error", message: errorMessage(err) };
    }
  }

  // Test projects on the bot's GitLab account

  let testProjects = $state<TestProject[]>([]);
  let invite = $state("");
  let labStatus = $state<Status>({ kind: "idle" });
  // Reset and delete ask for a second click.
  let confirming = $state<"reset" | "delete" | null>(null);

  Devlab.List()
    .then((list) => (testProjects = list ?? []))
    .catch((err) => (labStatus = { kind: "error", message: errorMessage(err) }));

  async function lab(label: string, run: () => Promise<TestProject[] | null | void>, done: string) {
    confirming = null;
    labStatus = { kind: "busy", label };
    try {
      testProjects = (await run()) ?? [];
      labStatus = { kind: "ok", message: done };
      // The GitLab page and the poll see the projects switched on or gone.
      loadGitLab();
      Tasks.PollNow();
    } catch (err) {
      labStatus = { kind: "error", message: errorMessage(err) };
    }
  }

  // A review request from the bot itself, as the ping form would make it.
  async function mentionBot(item: TestItem) {
    labStatus = { kind: "busy", label: "Lecture de la MR…" };
    try {
      const todo = await Tasks.Simulate(item.webUrl, "", "");
      labStatus = { kind: "ok", message: `Tâche ajoutée : ${ref(todo.project, todo.targetType, todo.iid)}. Voir la page Activité.` };
    } catch (err) {
      labStatus = { kind: "error", message: errorMessage(err) };
    }
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
      pingStatus = { kind: "ok", message: `Tâche ajoutée : ${ref(todo.project, todo.targetType, todo.iid)}. Voir la page État.` };
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

  // Environment test

  let projectList = $state<ProjectView[]>([]);
  let tryProject = $state("");
  let tryRef = $state("");
  let tryCommand = $state("ls -la && cat /etc/os-release | head -2");
  let tryStatus = $state<Status>({ kind: "idle" });
  let tryResult = $state<TryResult | null>(null);

  Projects.List()
    .then((overview) => {
      projectList = (overview.projects ?? []).filter((p) => p.accessible);
      tryProject = String(projectList[0]?.id ?? "");
    })
    .catch((err) => console.error("Load projects:", err));

  async function tryEnvironment() {
    tryStatus = { kind: "busy", label: "Clone et démarrage du container…" };
    tryResult = null;
    try {
      tryResult = await Sandbox.Try(Number(tryProject), tryRef.trim(), tryCommand);
      tryStatus = { kind: "idle" };
    } catch (err) {
      tryStatus = { kind: "error", message: errorMessage(err) };
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

<FormGroup
  title="Projets de test GitLab"
  class="mt-6"
  footer="Des projets privés sur le compte du bot, préfixés mihari-test- : du code, des MR avec des bugs à trouver et des issues étiquetées mihari::tests pour le profil de nuit. Ils sont allumés dans Mihari dès leur création. Réinitialiser les supprime et les refait à neuf ; seuls les projets mihari-test- du compte du bot sont touchés."
>
  <FormRow label="Inviter (facultatif)" for="dev-invite">
    <TextField id="dev-invite" class="w-64" placeholder="ton pseudo GitLab" bind:value={invite} />
  </FormRow>
  {#each testProjects as p (p.id)}
    <div class="p-form-row flex-col items-stretch! gap-1 py-2.5">
      <span class="flex items-center gap-2">
        <span class="p-form-row-label flex-1 font-mono">{p.path}</span>
        <Button size="small" variant="borderless" iconOnly aria-label="Ouvrir sur GitLab" title="Ouvrir sur GitLab" onclick={() => Browser.OpenURL(p.webUrl)}>
          <ArrowSquareOutIcon size={14} aria-hidden="true" />
        </Button>
      </span>
      {#each p.mergeRequests ?? [] as mr (mr.iid)}
        <span class="p-subhead-2 flex items-center gap-2 ps-3">
          <span class="flex-1 truncate text-secondary-label">!{mr.iid} {mr.title}</span>
          <Button size="small" variant="borderless" onclick={() => mentionBot(mr)} disabled={labStatus.kind === "busy"}>
            <AtIcon size={14} aria-hidden="true" />
            Demander une review
          </Button>
        </span>
      {/each}
      {#each p.issues ?? [] as is (is.iid)}
        <span class="p-subhead-2 flex items-center gap-2 ps-3">
          <span class="flex-1 truncate text-secondary-label">#{is.iid} {is.title}</span>
          <Button size="small" variant="borderless" onclick={() => Browser.OpenURL(is.webUrl)}>Ouvrir</Button>
        </span>
      {/each}
    </div>
  {:else}
    {#if labStatus.kind !== "busy"}
      <p class="p-body py-3 text-center text-tertiary-label">Aucun projet de test sur le compte du bot.</p>
    {/if}
  {/each}
</FormGroup>

<div class="mt-4 flex items-center gap-2">
  {@render statusLine(labStatus)}
  {#if confirming}
    <Button size="small" onclick={() => (confirming = null)}>Annuler</Button>
    {#if confirming === "reset"}
      <Button size="small" variant="destructive" onclick={() => lab("Suppression puis création des projets…", () => Devlab.Reset(invite.trim()), "Projets de test refaits à neuf.")}>
        Supprimer et refaire
      </Button>
    {:else}
      <Button size="small" variant="destructive" onclick={() => lab("Suppression des projets…", () => Devlab.Delete(), "Projets de test supprimés.")}>
        Supprimer pour de bon
      </Button>
    {/if}
  {:else}
    {#if testProjects.length > 0}
      <Button size="small" variant="borderless" disabled={labStatus.kind === "busy"} onclick={() => (confirming = "delete")}>Supprimer</Button>
      <Button size="small" disabled={labStatus.kind === "busy"} onclick={() => (confirming = "reset")}>Réinitialiser</Button>
    {/if}
    <Button size="small" variant="primary" disabled={labStatus.kind === "busy"} onclick={() => lab("Création des projets…", () => Devlab.Create(invite.trim()), "Projets de test prêts et allumés.")}>
      {testProjects.length > 0 ? "Compléter" : "Créer les projets de test"}
    </Button>
  {/if}
</div>

<form
  class="mt-8"
  oninput={() => remoteStatus.kind !== "busy" && (remoteStatus = { kind: "idle" })}
  onsubmit={(e) => {
    e.preventDefault();
    saveRemote();
  }}
>
  <FormGroup
    title="Inférence distante"
    footer="Pour le moteur OpenCode d'un profil : les requêtes partent vers cette API compatible OpenAI au lieu d'un modèle local. Seuls les modèles servis en /v1/chat/completions marchent, et le code des MR quitte la machine. Tester utilise les valeurs saisies, sans les enregistrer."
  >
    <FormRow label="Offre" for="dev-remote-offer">
      <Select id="dev-remote-offer" class="me-7" options={OFFERS} bind:value={remoteOffer} />
    </FormRow>
    {#if remoteOffer === "custom"}
      <FormRow label="URL de l'API (sans /v1)" for="dev-remote-url">
        <TextField
          id="dev-remote-url"
          type="url"
          class="w-96 font-mono"
          placeholder="https://openrouter.ai/api"
          bind:value={remoteURL}
        />
      </FormRow>
    {/if}
    <FormRow label="Clé API" for="dev-remote-key">
      <div class="flex items-center gap-2">
        <TextField
          id="dev-remote-key"
          type="password"
          class="w-72 font-mono"
          placeholder={hasRemoteKey ? "Enregistrée, laisser vide pour la garder" : "sk-…"}
          bind:value={remoteKey}
        />
        <Button
          size="small"
          variant="borderless"
          iconOnly
          aria-label="Supprimer la clé"
          disabled={!hasRemoteKey}
          onclick={forgetRemoteKey}
        >
          <TrashIcon size={14} aria-hidden="true" />
        </Button>
      </div>
    </FormRow>
    <FormRow label="Modèle à tester" for="dev-remote-model">
      <TextField id="dev-remote-model" class="w-72 font-mono" placeholder="glm-5.3" bind:value={remoteModel} />
    </FormRow>
  </FormGroup>

  <div class="mt-4 flex items-center gap-2">
    {@render statusLine(remoteStatus)}
    <Button
      disabled={remoteStatus.kind === "busy" ||
        remoteModel.trim() === "" ||
        (remoteOffer === "custom" && remoteURL.trim() === "") ||
        (!hasRemoteKey && remoteKey.trim() === "")}
      onclick={testRemote}
    >
      Tester
    </Button>
    <Button type="submit" variant="primary" disabled={remoteStatus.kind === "busy"}>Enregistrer</Button>
  </div>
</form>

<form
  class="mt-8"
  oninput={() => pingStatus.kind !== "busy" && (pingStatus = { kind: "idle" })}
  onsubmit={(e) => {
    e.preventDefault();
    simulate();
  }}
>
  <FormGroup
    title="Simuler un ping"
    footer="Crée la tâche qu'un ping créerait, sans rien poster sur GitLab. Sur une MR, sans texte, c'est une demande de review."
  >
    <FormRow label="URL de la MR ou de l'issue" for="dev-mr">
      <TextField
        id="dev-mr"
        type="url"
        class="w-96"
        placeholder="https://gitlab.com/groupe/projet/-/merge_requests/1 ou /-/issues/1"
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
      <FormRow label={todo.title} detail="{ref(todo.project, todo.targetType, todo.iid)} · @{todo.author} · {where}">
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

<form
  class="mt-8"
  onsubmit={(e) => {
    e.preventDefault();
    tryEnvironment();
  }}
>
  <FormGroup
    title="Tester l'environnement d'un job"
    footer="Clone le projet, démarre son container, lance la commande dans /workspace, puis supprime tout."
  >
    <FormRow label="Projet" for="dev-try-project">
      <Select
        id="dev-try-project"
        options={projectList.map((p) => ({ value: String(p.id), label: p.path }))}
        bind:value={tryProject}
      />
    </FormRow>
    <FormRow label="Branche" for="dev-try-ref">
      <TextField id="dev-try-ref" class="w-72 font-mono" placeholder="Branche par défaut, si vide" bind:value={tryRef} />
    </FormRow>
    <div class="p-form-row flex-col items-stretch! py-2.5">
      <label class="p-form-row-label" for="dev-try-command">Commande</label>
      <TextArea id="dev-try-command" rows={2} class="w-full resize-y font-mono" bind:value={tryCommand} />
    </div>
  </FormGroup>

  <div class="mt-4 flex items-center gap-2">
    {@render statusLine(tryStatus)}
    <Button
      type="submit"
      variant="primary"
      disabled={tryProject === "" || tryCommand.trim() === "" || tryStatus.kind === "busy"}
    >
      Lancer
    </Button>
  </div>
</form>

{#if tryResult}
  {@const r = tryResult.result}
  <FormGroup
    title="Sortie"
    class="mt-6"
    footer="{tryResult.image} · branche {tryResult.branch} · préparation {(tryResult.setupMs / 1000).toFixed(1)} s · commande {(r.durationMs / 1000).toFixed(1)} s · code {r.exitCode}{r.timedOut ? ' · délai dépassé' : ''}{r.truncated ? ' · sortie tronquée' : ''}"
  >
    {#if r.stdout}
      <pre class="p-form-row p-subhead-2 overflow-x-auto py-2.5 font-mono whitespace-pre select-text">{r.stdout}</pre>
    {/if}
    {#if r.stderr}
      <pre class="p-form-row p-subhead-2 overflow-x-auto py-2.5 font-mono whitespace-pre text-(--p-sys-red) select-text">{r.stderr}</pre>
    {/if}
    {#if !r.stdout && !r.stderr}
      <p class="p-form-row p-subhead-2 py-2.5 text-tertiary-label">Aucune sortie.</p>
    {/if}
  </FormGroup>
{/if}
