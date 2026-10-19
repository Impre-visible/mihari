<script lang="ts">
  // PROTOTYPE (UI redesign), variant A: list + inspector, like Mail. The
  // jobs and the queue on the left, the selected job on the right; nothing
  // ever leaves the page.
  import { ArrowClockwiseIcon, TrayIcon } from "phosphor-svelte";
  import { Service as Tasks } from "../../../bindings/github.com/Impre-visible/mihari/internal/tasks";
  import type { Summary } from "../../../bindings/github.com/Impre-visible/mihari/internal/jobs/models";
  import { Button } from "$lib/ui";
  import { ref } from "$lib/refs";
  import JobState from "../components/JobState.svelte";
  import { live as isLive } from "../components/jobState";
  import Health from "./Health.svelte";
  import JobDetail from "./JobDetail.svelte";
  import { ago, live } from "./data.svelte";

  let running = $derived(live.jobs.filter(isLive));
  let recent = $derived(live.jobs.filter((j) => !isLive(j)));
  let queue = $derived([
    ...(live.tasks?.todos ?? []).map((t) => ({ todo: t, note: "prête" })),
    ...(live.tasks?.waiting ?? []).map((w) => ({ todo: w.todo, note: w.reason })),
  ]);
  let selectedId = $state<number | null>(null);
  let selected = $derived(
    live.jobs.find((j) => j.id === selectedId) ?? running[0] ?? recent[0] ?? null,
  );
</script>

{#snippet row(j: Summary)}
  <button
    type="button"
    class={[
      "flex w-full items-start gap-2 rounded-lg px-2.5 py-2 text-start",
      selected?.id === j.id ? "bg-(--p-sys-blue)/15" : "hover:bg-fill",
    ]}
    onclick={() => (selectedId = j.id)}
  >
    <div class="mt-0.5"><JobState state={j.state} /></div>
    <div class="min-w-0 flex-1">
      <p class="p-subhead truncate font-medium">{j.title}</p>
      <p class="p-subhead-2 truncate text-secondary-label">
        {ref(j.project, j.targetType, j.iid)} · {ago(j.startedAt.startsWith("0001") ? j.queuedAt : j.startedAt, live.now)}
      </p>
    </div>
  </button>
{/snippet}

<div class="-mx-[24px] -mt-[20px] -mb-[24px] flex h-[calc(100%+44px)]">
  <aside class="flex w-80 shrink-0 flex-col border-e border-separator">
    <div class="flex flex-col gap-2 border-b border-separator p-3">
      <Health vertical />
    </div>
    <div class="flex-1 overflow-y-auto p-2">
      <p class="p-caption-1 px-2.5 pt-2 pb-1 font-semibold text-secondary-label uppercase">En cours</p>
      {#each running as j (j.id)}
        {@render row(j)}
      {:else}
        <p class="p-subhead-2 px-2.5 py-1 text-tertiary-label">Rien en cours</p>
      {/each}

      <div class="flex items-center px-2.5 pt-4 pb-1">
        <p class="p-caption-1 flex-1 font-semibold text-secondary-label uppercase">File d'attente</p>
        <Button size="small" variant="borderless" iconOnly aria-label="Vérifier GitLab" title="Vérifier GitLab" onclick={() => Tasks.PollNow()}>
          <ArrowClockwiseIcon size={13} aria-hidden="true" />
        </Button>
      </div>
      {#each queue as q (q.todo.id)}
        <div class="flex items-start gap-2 px-2.5 py-1.5">
          <TrayIcon size={15} class="mt-0.5 shrink-0 text-tertiary-label" aria-hidden="true" />
          <div class="min-w-0">
            <p class="p-subhead truncate">{q.todo.title}</p>
            <p class="p-subhead-2 truncate text-secondary-label">{ref(q.todo.project, q.todo.targetType, q.todo.iid)} · {q.note}</p>
          </div>
        </div>
      {:else}
        <p class="p-subhead-2 px-2.5 py-1 text-tertiary-label">Vide</p>
      {/each}

      <p class="p-caption-1 px-2.5 pt-4 pb-1 font-semibold text-secondary-label uppercase">Récents</p>
      {#each recent as j (j.id)}
        {@render row(j)}
      {/each}
    </div>
  </aside>

  <section class="min-w-0 flex-1 overflow-y-auto p-5">
    {#if selected}
      {#key selected.id}<JobDetail job={selected} />{/key}
    {:else}
      <div class="flex h-full flex-col items-center justify-center gap-1 text-center">
        <p class="p-title-3 text-secondary-label">Aucun job pour l'instant</p>
        <p class="p-subhead-2 text-tertiary-label">Mentionne le bot sur une MR ou une issue : le job apparaît ici.</p>
      </div>
    {/if}
  </section>
</div>
