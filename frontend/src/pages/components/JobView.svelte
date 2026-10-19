<script lang="ts">
  import { tick } from "svelte";
  import { ArrowDownIcon, ArrowLeftIcon, ArrowSquareOutIcon, ListChecksIcon } from "phosphor-svelte";
  import { Browser, Events } from "@wailsio/runtime";
  import { Service as Jobs } from "../../../bindings/github.com/Impre-visible/mihari/internal/jobs";
  import type { Summary } from "../../../bindings/github.com/Impre-visible/mihari/internal/jobs/models";
  import type { Event as JobEvent } from "../../../bindings/github.com/Impre-visible/mihari/internal/agent/models";
  import { Button, Spinner } from "$lib/ui";
  import { ref } from "$lib/refs";
  import JobChat from "./chat/JobChat.svelte";
  import Plan from "./chat/Plan.svelte";
  import { latestPlan } from "./chat/transcript";
  import JobState from "./JobState.svelte";
  import { STATES, duration, live } from "./jobState";

  // One job as its own page: its header stays at the top while the
  // transcript scrolls under it; onback returns to the list.
  type Props = { id: number; now: number; onback: () => void };
  let { id, now, onback }: Props = $props();

  let job = $state<Summary | null>(null);
  let events = $state<JobEvent[]>([]);
  // The page's scrolling area, and whether its view rests on the end of the
  // transcript: new lines then keep it there, otherwise the reader stays put.
  let header = $state<HTMLElement | null>(null);
  let scroller = $derived(header?.closest<HTMLElement>(".ui-app-content") ?? null);
  let atBottom = $state(true);
  // The agent's plan as todo_write last left it, kept in sight in the header.
  let plan = $derived(latestPlan(events));

  $effect(() => {
    const open = id;
    Jobs.List()
      .then((list) => (job = list?.find((j) => j.id === open) ?? null))
      .catch((err) => console.error("Load jobs:", err));
    Jobs.Transcript(open)
      // Lines that arrived while loading are in the list too.
      .then((list) => (events = list ?? []))
      .catch((err) => console.error("Load transcript:", err));
  });

  $effect(() => {
    const offJobs = Events.On("jobs:updated", (event) => {
      job = (event.data as Summary[] | null)?.find((j) => j.id === id) ?? job;
    });
    const offLines = Events.On("job:event", (event) => {
      if (event.data.jobId === id) events.push(event.data.event);
    });
    return () => {
      offJobs();
      offLines();
    };
  });

  function measure() {
    if (scroller) atBottom = scroller.scrollHeight - scroller.scrollTop - scroller.clientHeight < 40;
  }

  function toBottom(behavior: ScrollBehavior = "instant") {
    scroller?.scrollTo({ top: scroller.scrollHeight, behavior });
  }

  $effect(() => {
    const el = scroller;
    if (!el) return;
    el.addEventListener("scroll", measure, { passive: true });
    return () => el.removeEventListener("scroll", measure);
  });

  // Runs on each new line, after the page has grown.
  $effect(() => {
    void events.length;
    if (atBottom) tick().then(() => toBottom());
  });
</script>

<!-- The negative offsets cancel the page padding. -->
<div bind:this={header} class="sticky -top-[20px] z-10 -mx-[24px] -mt-[20px] border-b border-separator bg-window px-[24px] pt-3 pb-2">
  <div class="flex items-center gap-2">
    <Button size="small" variant="borderless" aria-label="Retour à l'état" onclick={onback}>
      <ArrowLeftIcon size={14} aria-hidden="true" />
      État
    </Button>
    {#if job}
      <div class="flex min-w-0 flex-1 items-center gap-1.5">
        <JobState state={job.state} />
        <span class="truncate">{job.title}</span>
      </div>
      <Button size="small" variant="borderless" onclick={() => job && Browser.OpenURL(job.url)}>
        <ArrowSquareOutIcon size={14} aria-hidden="true" />
        {job.targetType === "Issue" ? "Issue" : "MR"}
      </Button>
      {#if job.openedMr}
        <Button size="small" variant="borderless" onclick={() => job && Browser.OpenURL(job.openedMr)}>
          <ArrowSquareOutIcon size={14} aria-hidden="true" />
          MR ouverte
        </Button>
      {/if}
      {#if live(job)}
        <Button size="small" onclick={() => job && Jobs.Cancel(job.id)}>Annuler</Button>
      {/if}
    {:else}
      <Spinner small label="Chargement" />
    {/if}
  </div>
  {#if job}
    <p class="p-subhead-2 mt-1 ms-2.5 text-secondary-label">
      {ref(job.project, job.targetType, job.iid)} · @{job.author} · {STATES[job.state]}{job.profile
        ? ` · ${job.profile}`
        : ""}{duration(job, now) ? ` · ${duration(job, now)}` : ""}{job.turns ? ` · ${job.turns} tours` : ""}
    </p>
    {#if job.error}
      <p class="p-subhead-2 mt-2 ms-2.5 line-clamp-3 text-label select-text">{job.error}</p>
    {/if}
  {/if}
  {#if plan.length}
    {@const done = plan.filter((t) => t.status === "completed").length}
    {@const doing = plan.find((t) => t.status === "in_progress")}
    <details class="mt-2 ms-2.5">
      <summary class="flex cursor-default list-none items-center gap-1.5 text-secondary-label [&::-webkit-details-marker]:hidden">
        <ListChecksIcon size={14} aria-hidden="true" />
        <span class="p-subhead-2 font-semibold">Plan {done}/{plan.length}</span>
        {#if doing}<span class="p-subhead-2 min-w-0 truncate">· {doing.content}</span>{/if}
      </summary>
      <div class="mt-1.5 max-h-48 overflow-y-auto">
        <Plan todos={plan} />
      </div>
    </details>
  {/if}
</div>

<JobChat {events} live={job ? live(job) : false} />

{#if !atBottom}
  <div class="pointer-events-none sticky bottom-0 flex justify-end pt-2">
    <Button
      class="pointer-events-auto rounded-full! bg-window! shadow-md"
      size="small"
      aria-label="Aller tout en bas"
      title="Aller tout en bas"
      onclick={() => toBottom("smooth")}
    >
      <ArrowDownIcon size={14} aria-hidden="true" />
      Tout en bas
    </Button>
  </div>
{/if}
