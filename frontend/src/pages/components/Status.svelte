<script lang="ts">
  import { ArrowClockwiseIcon, CheckCircleIcon, WarningCircleIcon } from "phosphor-svelte";
  import { Browser, Events } from "@wailsio/runtime";
  import { Service as Tasks } from "../../../bindings/github.com/Impre-visible/mihari/internal/tasks";
  import type { Snapshot } from "../../../bindings/github.com/Impre-visible/mihari/internal/tasks/models";
  import { Service as Scheduler } from "../../../bindings/github.com/Impre-visible/mihari/internal/scheduler";
  import type { Snapshot as EngineSnapshot } from "../../../bindings/github.com/Impre-visible/mihari/internal/scheduler/models";
  import { Button, FormGroup, FormRow, Spinner } from "$lib/ui";

  const ACTIONS: Record<string, string> = {
    review_requested: "review demandée",
    assigned: "assignée",
    mentioned: "mentionné",
    directly_addressed: "interpellé",
  };

  let snapshot = $state<Snapshot | null>(null);
  let engine = $state<EngineSnapshot | null>(null);
  let now = $state(Date.now());

  // Go's zero time.Time means no poll has finished yet.
  let polledAt = $derived(snapshot && !snapshot.polledAt.startsWith("0001") ? Date.parse(snapshot.polledAt) : null);
  let todos = $derived(snapshot?.todos ?? []);

  Scheduler.Snapshot()
    .then((s) => (engine = s))
    .catch((err) => console.error("Load engine:", err));

  Tasks.Snapshot()
    .then((s) => (snapshot = s))
    .catch((err) => console.error("Load tasks:", err));

  $effect(() => {
    const off = Events.On("tasks:updated", (event) => (snapshot = event.data));
    const offEngine = Events.On("engine:updated", (event) => (engine = event.data));
    const timer = setInterval(() => (now = Date.now()), 1000);
    return () => {
      off();
      offEngine();
      clearInterval(timer);
    };
  });

  function ago(time: number) {
    const seconds = Math.max(0, Math.round((now - time) / 1000));
    if (seconds < 5) return "à l'instant";
    if (seconds < 60) return `il y a ${seconds} s`;
    return `il y a ${Math.floor(seconds / 60)} min`;
  }
</script>

<p class="p-body text-secondary-label">Modèle chargé, jobs en cours et en attente.</p>

<FormGroup title="Moteur" class="mt-6">
  {#if !engine || engine.state === "idle"}
    <FormRow label="Aucun profil prévu en ce moment" detail="Le planning est en pause." />
  {:else}
    <FormRow label="Profil" detail={engine.profileName} />
    <FormRow label="Modèle" detail="{engine.model} · {engine.engine}" />
    <FormRow label="Jobs en parallèle" detail={String(engine.maxJobs)} />
    <div class="p-form-row gap-1.5">
      <span class="p-form-row-label">État</span>
      {#if engine.state === "starting"}
        <Spinner small label="Chargement du modèle" />
        <span class="text-secondary-label">Chargement du modèle…</span>
      {:else if engine.state === "ready"}
        <CheckCircleIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-green)" aria-hidden="true" />
        <span class="text-secondary-label">Prêt sur {engine.url}</span>
      {:else}
        <WarningCircleIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-red)" aria-hidden="true" />
        <span class="text-secondary-label">Erreur</span>
        <Button size="small" onclick={() => Scheduler.Reload()}>Relancer</Button>
      {/if}
    </div>
    {#if engine.state === "error"}
      <p class="p-form-row p-subhead-2 py-2.5 text-label select-text">{engine.error}</p>
    {/if}
  {/if}
</FormGroup>

<div class="mt-6 flex items-center gap-2">
  <p class="p-subhead-2 flex min-w-0 flex-1 items-center gap-1.5" role="status">
    {#if polledAt === null}
      <Spinner small label="Première vérification…" />
      <span class="text-secondary-label">Première vérification…</span>
    {:else if snapshot?.notConfigured}
      <WarningCircleIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-orange)" aria-hidden="true" />
      <span class="text-label">GitLab n'est pas configuré. Renseigne l'URL et le token dans GitLab.</span>
    {:else if snapshot?.error}
      <WarningCircleIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-red)" aria-hidden="true" />
      <span class="text-label">{snapshot.error}</span>
    {:else}
      <span class="text-secondary-label">Vérifié {ago(polledAt)}, toutes les 30 s.</span>
    {/if}
  </p>
  <Button onclick={() => Tasks.PollNow()}>
    <ArrowClockwiseIcon size={14} aria-hidden="true" />
    Vérifier maintenant
  </Button>
</div>

<FormGroup title="En attente ({todos.length})" class="mt-4">
  {#each todos as todo (todo.id)}
    <FormRow
      label={todo.title}
      detail="{todo.project}!{todo.mrIid} · {ACTIONS[todo.action] ?? todo.action}{todo.simulated ? ' · simulée' : ''}"
      chevron
      onclick={() => Browser.OpenURL(todo.url)}
    />
  {:else}
    <p class="p-body py-4 text-center text-tertiary-label">Aucune tâche</p>
  {/each}
</FormGroup>
