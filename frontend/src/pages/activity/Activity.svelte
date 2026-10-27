<script lang="ts">
  import { ArrowClockwiseIcon, ArrowSquareOutIcon, TrayIcon } from "phosphor-svelte";
  import { Browser } from "@wailsio/runtime";
  import { Service as Tasks } from "../../../bindings/github.com/Impre-visible/mihari/internal/tasks";
  import type { Summary } from "../../../bindings/github.com/Impre-visible/mihari/internal/jobs/models";
  import type { Todo } from "../../../bindings/github.com/Impre-visible/mihari/internal/gitlab/models";
  import { Button, ListHeading, ListRow, SplitView } from "$lib/ui";
  import { shortRef } from "$lib/refs";
  import { ago, live } from "$lib/live.svelte";
  import { nav } from "$lib/nav.svelte";
  import Health from "./Health.svelte";
  import JobInspector from "./JobInspector.svelte";
  import JobState from "./JobState.svelte";
  import { live as running } from "./jobState";

  // The jobs and what waits on GitLab on the left, the selected job on the
  // right: the whole activity on one screen.
  const ACTIONS: Record<string, string> = {
    review_requested: "review demandée",
    assigned: "assignée",
    mentioned: "mention",
    directly_addressed: "interpellé",
  };

  let current = $derived(live.jobs.filter(running));
  let past = $derived(live.jobs.filter((j) => !running(j)));
  let queue = $derived([
    ...(live.tasks?.todos ?? []).map((todo) => ({ todo, note: ACTIONS[todo.action] ?? todo.action, alert: false })),
    ...(live.tasks?.waiting ?? []).map((w) => ({ todo: w.todo, note: `mise de côté : ${w.reason}`, alert: false })),
    ...(live.tasks?.refused ?? []).map((r) => ({ todo: r.todo, note: `refusée ${ago(r.at, live.now)} : pas autorisé`, alert: true })),
  ]);
  // The job asked for, else the one running, else the latest.
  let selected = $derived(live.jobs.find((j) => j.id === nav.jobId) ?? current[0] ?? past[0] ?? null);
  let detail = $state<HTMLElement | null>(null);

  // The latest job on a todo's merge request or issue, to open instead of GitLab.
  function jobFor(todo: Todo) {
    return live.jobs.find((j) => j.projectId === todo.projectId && j.iid === todo.iid && j.targetType === todo.targetType) ?? null;
  }

  function when(j: Summary) {
    return ago(j.startedAt.startsWith("0001") ? j.queuedAt : j.startedAt, live.now);
  }
</script>

{#snippet jobRow(j: Summary)}
  <ListRow
    title={j.title}
    subtitle={j.error || `${shortRef(j.project, j.targetType, j.iid)} · ${when(j)}`}
    alert={!!j.error}
    selected={selected?.id === j.id}
    onclick={() => (nav.jobId = j.id)}
  >
    {#snippet leading()}<JobState state={j.state} />{/snippet}
  </ListRow>
{/snippet}

<SplitView id="activity" width={320} bind:detail>
  {#snippet header()}<Health />{/snippet}
  {#snippet list()}
    <ListHeading title="En cours" />
    {#each current as j (j.id)}
      {@render jobRow(j)}
    {:else}
      <p class="p-subhead-2 px-2.5 py-1 text-tertiary-label">Rien en cours.</p>
    {/each}

    <ListHeading title="Sur GitLab">
      <Button size="small" variant="borderless" iconOnly aria-label="Vérifier GitLab maintenant" title="Vérifier GitLab maintenant" onclick={() => Tasks.PollNow()}>
        <ArrowClockwiseIcon size={13} aria-hidden="true" />
      </Button>
    </ListHeading>
    {#each queue as q (q.todo.id)}
      {@const job = jobFor(q.todo)}
      <ListRow
        title={q.todo.title}
        subtitle={`${shortRef(q.todo.project, q.todo.targetType, q.todo.iid)} · @${q.todo.author} · ${q.note}`}
        alert={q.alert}
        onclick={job ? () => (nav.jobId = job.id) : undefined}
      >
        {#snippet leading()}<TrayIcon size={16} class="text-tertiary-label" aria-hidden="true" />{/snippet}
        {#snippet trailing()}
          <Button size="small" variant="borderless" iconOnly aria-label="Ouvrir sur GitLab" title="Ouvrir sur GitLab" onclick={() => Browser.OpenURL(q.todo.url)}>
            <ArrowSquareOutIcon size={13} aria-hidden="true" />
          </Button>
        {/snippet}
      </ListRow>
    {:else}
      <p class="p-subhead-2 px-2.5 py-1 text-tertiary-label">Rien en attente.</p>
    {/each}

    {#if past.length}
      <ListHeading title="Récents" />
      {#each past as j (j.id)}
        {@render jobRow(j)}
      {/each}
    {/if}
  {/snippet}

  {#if selected}
    {#key selected.id}<JobInspector job={selected} scroller={detail} />{/key}
  {:else}
    <div class="flex h-full flex-col items-center justify-center gap-1 text-center">
      <p class="p-title-3 text-secondary-label">Aucun job pour l'instant</p>
      <p class="p-subhead-2 max-w-sm text-tertiary-label">
        Mentionne le bot sur une merge request ou une issue, ou demande-lui une review : le job apparaît ici et se suit en direct.
      </p>
    </div>
  {/if}
</SplitView>
