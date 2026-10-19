<script lang="ts">
  // PROTOTYPE (UI redesign): a running job as a card, with its plan and what
  // it is doing right now.
  import { Service as Jobs } from "../../../bindings/github.com/Impre-visible/mihari/internal/jobs";
  import type { Summary } from "../../../bindings/github.com/Impre-visible/mihari/internal/jobs/models";
  import { Button } from "$lib/ui";
  import { ref } from "$lib/refs";
  import { TOOLS } from "../components/chat/tools";
  import { latestPlan } from "../components/chat/transcript";
  import JobState from "../components/JobState.svelte";
  import { duration } from "../components/jobState";
  import Progress from "./Progress.svelte";
  import { Transcript, live } from "./data.svelte";

  type Props = { job: Summary; onopen: () => void };
  let { job, onopen }: Props = $props();

  let transcript = $derived(new Transcript(job.id));
  $effect(() => {
    const t = transcript;
    return () => t.close();
  });
  let plan = $derived(latestPlan(transcript.events));
  // The latest tool call, told the way its block in the chat tells it.
  let action = $derived.by(() => {
    const e = transcript.events.findLast((e) => e.kind === "tool" && !e.agent);
    if (!e) return null;
    let args = {};
    try {
      args = JSON.parse(e.text);
    } catch {}
    const v = TOOLS[e.title]?.view(args);
    return v ? { icon: v.icon, text: v.title + (v.detail ? ` · ${v.detail}` : "") } : { icon: null, text: e.title };
  });
</script>

<div class="flex flex-col gap-3 rounded-xl border border-separator bg-window p-4 shadow-sm">
  <div class="flex items-start gap-3">
    <div class="mt-0.5"><JobState state={job.state} /></div>
    <button type="button" class="min-w-0 flex-1 text-start" onclick={onopen}>
      <p class="p-headline truncate">{job.title}</p>
      <p class="p-subhead-2 truncate text-secondary-label">
        {ref(job.project, job.targetType, job.iid)} · @{job.author}{duration(job, live.now) ? ` · ${duration(job, live.now)}` : ""}{job.turns
          ? ` · ${job.turns} tours`
          : ""}
      </p>
    </button>
    <Button size="small" onclick={() => Jobs.Cancel(job.id)}>Annuler</Button>
    <Button size="small" variant="primary" onclick={onopen}>Suivre</Button>
  </div>
  {#if plan.length}<Progress todos={plan} />{/if}
  {#if action}
    <div class="flex items-center gap-2 rounded-lg bg-fill px-3 py-2">
      {#if action.icon}<action.icon size={14} class="shrink-0 text-secondary-label" aria-hidden="true" />{/if}
      <span class="p-subhead-2 min-w-0 truncate font-mono text-secondary-label">{action.text}</span>
    </div>
  {:else if job.state === "queued"}
    <p class="p-subhead-2 text-tertiary-label">En attente d'une place sur le moteur…</p>
  {/if}
</div>
