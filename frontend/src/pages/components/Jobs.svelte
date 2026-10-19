<script lang="ts">
  import { Events } from "@wailsio/runtime";
  import { Service as Jobs } from "../../../bindings/github.com/Impre-visible/mihari/internal/jobs";
  import type { Summary } from "../../../bindings/github.com/Impre-visible/mihari/internal/jobs/models";
  import { Button, FormGroup } from "$lib/ui";
  import { ref } from "$lib/refs";
  import { nav } from "$lib/nav.svelte";
  import JobState from "./JobState.svelte";
  import { STATES, duration, live } from "./jobState";

  // The list of jobs; one opens as its own page, JobView.
  type Props = { now: number };
  let { now }: Props = $props();

  let jobs = $state<Summary[]>([]);
  let active = $derived(jobs.filter(live));
  let history = $derived(jobs.filter((j) => !live(j)));

  Jobs.List()
    .then((list) => (jobs = list ?? []))
    .catch((err) => console.error("Load jobs:", err));

  $effect(() => Events.On("jobs:updated", (event) => (jobs = event.data ?? [])));
</script>

{#snippet jobRow(j: Summary)}
  <div class="p-form-row gap-2">
    <JobState state={j.state} />
    <button type="button" class="flex min-w-0 flex-1 flex-col items-start py-1.5 text-start" onclick={() => (nav.jobId = j.id)}>
      <span class="w-full truncate">{j.title}</span>
      <span class="p-subhead-2 w-full truncate text-secondary-label">
        {ref(j.project, j.targetType, j.iid)}{j.kind === "issues" ? " · label" : ""} · @{j.author} · {STATES[j.state] ?? j.state}{duration(j, now)
          ? ` · ${duration(j, now)}`
          : ""}{j.simulated ? " · simulé" : ""}
      </span>
    </button>
    {#if live(j)}
      <Button size="small" onclick={() => Jobs.Cancel(j.id)}>Annuler</Button>
    {/if}
    <!-- The same disclosure chevron as a FormRow that opens more detail. -->
    <button type="button" class="flex shrink-0 self-stretch items-center ps-1" aria-label="Ouvrir le job" title="Ouvrir le job" onclick={() => (nav.jobId = j.id)}>
      <span class="p-form-row-chevron" aria-hidden="true"></span>
    </button>
  </div>
{/snippet}

<FormGroup
  title="Jobs ({active.length})"
  class="mt-6"
  footer={active.length === 0 && history.length === 0
    ? "Un job démarre pour chaque tâche prête, quand le moteur est prêt et que le profil actif est déclenché par ping."
    : undefined}
>
  {#each active as j (j.id)}
    {@render jobRow(j)}
  {:else}
    <p class="p-body py-4 text-center text-tertiary-label">Aucun job en cours</p>
  {/each}
</FormGroup>

{#if history.length > 0}
  <FormGroup title="Historique" footer="Les {history.length} derniers jobs, jusqu'au redémarrage de Mihari.">
    {#each history as j (j.id)}
      {@render jobRow(j)}
    {/each}
  </FormGroup>
{/if}
