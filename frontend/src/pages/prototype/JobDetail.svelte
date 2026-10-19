<script lang="ts">
  // PROTOTYPE (UI redesign): one job, compact, for a pane or a drawer.
  import { ArrowSquareOutIcon } from "phosphor-svelte";
  import { Browser } from "@wailsio/runtime";
  import { Service as Jobs } from "../../../bindings/github.com/Impre-visible/mihari/internal/jobs";
  import type { Summary } from "../../../bindings/github.com/Impre-visible/mihari/internal/jobs/models";
  import { Button } from "$lib/ui";
  import { ref } from "$lib/refs";
  import JobChat from "../components/chat/JobChat.svelte";
  import { latestPlan } from "../components/chat/transcript";
  import JobState from "../components/JobState.svelte";
  import { STATES, duration, live as isLive } from "../components/jobState";
  import Progress from "./Progress.svelte";
  import { Transcript, live } from "./data.svelte";

  type Props = { job: Summary };
  let { job }: Props = $props();

  let transcript = $derived(new Transcript(job.id));
  $effect(() => {
    const t = transcript;
    return () => t.close();
  });
  let plan = $derived(latestPlan(transcript.events));
</script>

<div class="flex flex-col gap-3">
  <div class="flex items-start gap-3">
    <div class="mt-1"><JobState state={job.state} /></div>
    <div class="min-w-0 flex-1">
      <h2 class="p-title-3 font-semibold">{job.title}</h2>
      <p class="p-subhead-2 text-secondary-label">
        {ref(job.project, job.targetType, job.iid)} · @{job.author} · {STATES[job.state]}{duration(job, live.now)
          ? ` · ${duration(job, live.now)}`
          : ""}{job.turns ? ` · ${job.turns} tours` : ""}{job.profile ? ` · ${job.profile}` : ""}
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
      {#if isLive(job)}<Button size="small" onclick={() => Jobs.Cancel(job.id)}>Annuler</Button>{/if}
    </div>
  </div>
  {#if job.error}
    <p class="p-subhead-2 rounded-lg bg-(--p-sys-red)/10 px-3 py-2 text-label select-text">{job.error}</p>
  {/if}
  {#if plan.length}<Progress todos={plan} />{/if}
  <JobChat events={transcript.events} live={isLive(job)} />
</div>
