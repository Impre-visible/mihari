<script lang="ts">
  // PROTOTYPE (UI redesign), variant B: an activity feed. Running jobs are
  // big live cards, the queue a short list, the history compact rows by day;
  // a job opens as its own page.
  import { fly } from "svelte/transition";
  import { cubicOut } from "svelte/easing";
  import { ArrowClockwiseIcon, ArrowSquareOutIcon } from "phosphor-svelte";
  import { Browser } from "@wailsio/runtime";
  import { Service as Tasks } from "../../../bindings/github.com/Impre-visible/mihari/internal/tasks";
  import type { Summary } from "../../../bindings/github.com/Impre-visible/mihari/internal/jobs/models";
  import { Button } from "$lib/ui";
  import { ref } from "$lib/refs";
  import JobView from "../components/JobView.svelte";
  import JobState from "../components/JobState.svelte";
  import { duration, live as isLive } from "../components/jobState";
  import Health from "./Health.svelte";
  import LiveCard from "./LiveCard.svelte";
  import { live } from "./data.svelte";

  let open = $state<number | null>(null);
  let running = $derived(live.jobs.filter(isLive));
  let queue = $derived([
    ...(live.tasks?.todos ?? []).map((t) => ({ todo: t, note: "prête" })),
    ...(live.tasks?.waiting ?? []).map((w) => ({ todo: w.todo, note: w.reason })),
  ]);

  // The history, newest first, under a heading per day.
  let days = $derived.by(() => {
    const groups: { label: string; jobs: Summary[] }[] = [];
    const today = new Date(live.now).toDateString();
    const yesterday = new Date(live.now - 86400000).toDateString();
    for (const j of live.jobs.filter((j) => !isLive(j))) {
      const d = new Date(j.queuedAt);
      const label =
        d.toDateString() === today
          ? "Aujourd'hui"
          : d.toDateString() === yesterday
            ? "Hier"
            : d.toLocaleDateString("fr-FR", { weekday: "long", day: "numeric", month: "long" });
      if (groups.at(-1)?.label !== label) groups.push({ label, jobs: [] });
      groups.at(-1)!.jobs.push(j);
    }
    return groups;
  });
</script>

{#if open !== null}
  <div in:fly|global={{ x: 48, duration: 220, easing: cubicOut }}>
    <JobView id={open} now={live.now} onback={() => (open = null)} />
  </div>
{:else}
  <div class="mx-auto flex max-w-3xl flex-col gap-6" in:fly|global={{ x: -48, duration: 220, easing: cubicOut }}>
    <Health />

    <section class="flex flex-col gap-3">
      <h2 class="p-title-3 font-semibold">En cours</h2>
      {#each running as j (j.id)}
        <LiveCard job={j} onopen={() => (open = j.id)} />
      {:else}
        <div class="rounded-xl border border-dashed border-separator px-4 py-6 text-center">
          <p class="p-subhead text-secondary-label">Aucun job en cours.</p>
          <p class="p-subhead-2 text-tertiary-label">Mentionne le bot sur une MR ou une issue pour en lancer un.</p>
        </div>
      {/each}
    </section>

    {#if queue.length}
      <section class="flex flex-col gap-1">
        <div class="flex items-center">
          <h2 class="p-title-3 flex-1 font-semibold">File d'attente</h2>
          <Button size="small" variant="borderless" onclick={() => Tasks.PollNow()}>
            <ArrowClockwiseIcon size={13} aria-hidden="true" /> Vérifier
          </Button>
        </div>
        {#each queue as q (q.todo.id)}
          <div class="flex items-center gap-2 border-b border-separator py-2 last:border-0">
            <span class="p-subhead min-w-0 flex-1 truncate">{q.todo.title}</span>
            <span class="p-subhead-2 shrink-0 text-secondary-label">{ref(q.todo.project, q.todo.targetType, q.todo.iid)} · {q.note}</span>
            <Button size="small" variant="borderless" iconOnly aria-label="Ouvrir sur GitLab" onclick={() => Browser.OpenURL(q.todo.url)}>
              <ArrowSquareOutIcon size={13} aria-hidden="true" />
            </Button>
          </div>
        {/each}
      </section>
    {/if}

    {#each days as day (day.label)}
      <section class="flex flex-col">
        <h2 class="p-subhead mb-1 font-semibold text-secondary-label first-letter:uppercase">{day.label}</h2>
        {#each day.jobs as j (j.id)}
          <button type="button" class="flex items-center gap-3 rounded-lg px-2 py-2 text-start hover:bg-fill" onclick={() => (open = j.id)}>
            <JobState state={j.state} />
            <div class="min-w-0 flex-1">
              <p class="p-subhead truncate">{j.title}</p>
              <p class={["p-subhead-2 truncate", j.error ? "text-(--p-sys-red)" : "text-secondary-label"]}>
                {j.error || j.result || ref(j.project, j.targetType, j.iid)}
              </p>
            </div>
            <span class="p-subhead-2 shrink-0 text-tertiary-label tabular-nums">
              {ref(j.project, j.targetType, j.iid).split("/").pop()} · {duration(j, live.now)}
            </span>
          </button>
        {/each}
      </section>
    {/each}
  </div>
{/if}
