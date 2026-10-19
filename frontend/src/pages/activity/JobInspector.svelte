<script lang="ts">
  import { ArrowSquareOutIcon } from "phosphor-svelte";
  import { Browser } from "@wailsio/runtime";
  import { Service as Jobs } from "../../../bindings/github.com/Impre-visible/mihari/internal/jobs";
  import type { Summary } from "../../../bindings/github.com/Impre-visible/mihari/internal/jobs/models";
  import { Button } from "$lib/ui";
  import { ref } from "$lib/refs";
  import { Transcript, live } from "$lib/live.svelte";
  import JobChat from "../components/chat/JobChat.svelte";
  import { latestPlan } from "../components/chat/transcript";
  import JobState from "./JobState.svelte";
  import Progress from "./Progress.svelte";
  import { STATES, duration, live as running } from "./jobState";

  // One job: what it is, how it goes, and its conversation, which follows
  // the end while the reader stays there.
  type Props = { job: Summary; scroller: HTMLElement | null };
  let { job, scroller }: Props = $props();

  let transcript = $derived(new Transcript(job.id));
  $effect(() => {
    const t = transcript;
    return () => t.close();
  });
  let plan = $derived(latestPlan(transcript.events));

  let atBottom = $state(true);
  $effect(() => {
    const el = scroller;
    if (!el) return;
    const measure = () => (atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 40);
    el.addEventListener("scroll", measure, { passive: true });
    return () => el.removeEventListener("scroll", measure);
  });
  $effect(() => {
    void transcript.events.length;
    if (atBottom) requestAnimationFrame(() => scroller?.scrollTo({ top: scroller.scrollHeight }));
  });
</script>

<div class="flex flex-col gap-3">
  <header class="flex items-start gap-3">
    <div class="mt-1.5"><JobState state={job.state} /></div>
    <div class="min-w-0 flex-1">
      <h2 class="p-title-3 font-semibold select-text">{job.title}</h2>
      <p class="p-subhead-2 text-secondary-label">
        {ref(job.project, job.targetType, job.iid)} · @{job.author} · {STATES[job.state] ?? job.state}{duration(job, live.now)
          ? ` · ${duration(job, live.now)}`
          : ""}{job.turns ? ` · ${job.turns} tours` : ""}{job.profile ? ` · ${job.profile}` : ""}{job.simulated ? " · simulé" : ""}
      </p>
    </div>
    <div class="flex shrink-0 items-center gap-1">
      <Button size="small" variant="borderless" onclick={() => Browser.OpenURL(job.url)}>
        <ArrowSquareOutIcon size={14} aria-hidden="true" />
        {job.targetType === "Issue" ? "Issue" : "MR"}
      </Button>
      {#if job.openedMr}
        <Button size="small" variant="borderless" onclick={() => Browser.OpenURL(job.openedMr)}>
          <ArrowSquareOutIcon size={14} aria-hidden="true" /> MR ouverte
        </Button>
      {/if}
      {#if running(job)}<Button size="small" onclick={() => Jobs.Cancel(job.id)}>Annuler</Button>{/if}
    </div>
  </header>
  {#if job.error}
    <p class="p-subhead-2 rounded-lg bg-(--p-sys-red)/10 px-3 py-2 text-label select-text">{job.error}</p>
  {/if}
  {#if plan.length}<Progress todos={plan} />{/if}
  <JobChat events={transcript.events} live={running(job)} />
</div>
