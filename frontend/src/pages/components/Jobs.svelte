<script lang="ts">
  import { tick } from "svelte";
  import {
    ArrowDownIcon,
    ArrowLeftIcon,
    ArrowSquareOutIcon,
    CheckCircleIcon,
    ClockIcon,
    ProhibitIcon,
    XCircleIcon,
  } from "phosphor-svelte";
  import { Browser, Events } from "@wailsio/runtime";
  import { Service as Jobs } from "../../../bindings/github.com/Impre-visible/mihari/internal/jobs";
  import type { Summary } from "../../../bindings/github.com/Impre-visible/mihari/internal/jobs/models";
  import type { Event as JobEvent } from "../../../bindings/github.com/Impre-visible/mihari/internal/agent/models";
  import { Button, FormGroup, Spinner } from "$lib/ui";
  import { ref } from "$lib/refs";
  import { nav } from "$lib/nav.svelte";
  import JobChat from "./chat/JobChat.svelte";

  type Props = { now: number };
  let { now }: Props = $props();

  const STATES: Record<string, string> = {
    queued: "en attente",
    running: "en cours",
    succeeded: "terminé",
    failed: "échec",
    canceled: "annulé",
  };

  let jobs = $state<Summary[]>([]);
  let selected = $state<number | null>(null);
  let events = $state<JobEvent[]>([]);
  // The page's scrolling area, and whether its view rests on the end of the
  // transcript: new lines then keep it there, otherwise the reader stays put.
  let header = $state<HTMLElement | null>(null);
  let scroller = $derived(header?.closest<HTMLElement>(".ui-app-content") ?? null);
  let atBottom = $state(true);

  let active = $derived(jobs.filter((j) => j.state === "queued" || j.state === "running"));
  let history = $derived(jobs.filter((j) => j.state !== "queued" && j.state !== "running"));
  let current = $derived(jobs.find((j) => j.id === selected) ?? null);

  Jobs.List()
    .then((list) => (jobs = list ?? []))
    .catch((err) => console.error("Load jobs:", err));

  $effect(() => {
    const offJobs = Events.On("jobs:updated", (event) => (jobs = event.data ?? []));
    const offLines = Events.On("job:event", (event) => {
      if (event.data.jobId === selected) events.push(event.data.event);
    });
    return () => {
      offJobs();
      offLines();
    };
  });

  // Asked from elsewhere, like a notification.
  $effect(() => {
    if (nav.jobId !== null) {
      open(nav.jobId);
      nav.jobId = null;
    }
  });

  function open(id: number) {
    selected = id;
    events = [];
    atBottom = true;
    Jobs.Transcript(id)
      .then((list) => {
        // Lines that arrived while loading are in the list too.
        if (selected === id) events = list ?? [];
      })
      .catch((err) => console.error("Load transcript:", err));
  }

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

  function duration(j: Summary) {
    const start = Date.parse(j.startedAt);
    if (j.startedAt.startsWith("0001")) return "";
    const end = j.endedAt.startsWith("0001") ? now : Date.parse(j.endedAt);
    const s = Math.max(0, Math.round((end - start) / 1000));
    return s < 60 ? `${s} s` : `${Math.floor(s / 60)} min ${String(s % 60).padStart(2, "0")}`;
  }
</script>

{#snippet stateIcon(state: string)}
  {#if state === "running"}
    <Spinner small label="En cours" />
  {:else if state === "queued"}
    <ClockIcon size={16} class="shrink-0 text-secondary-label" aria-label="En attente" />
  {:else if state === "succeeded"}
    <CheckCircleIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-green)" aria-label="Terminé" />
  {:else if state === "failed"}
    <XCircleIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-red)" aria-label="Échec" />
  {:else}
    <ProhibitIcon size={16} class="shrink-0 text-secondary-label" aria-label="Annulé" />
  {/if}
{/snippet}

{#snippet jobRow(j: Summary)}
  <div class="p-form-row gap-2">
    {@render stateIcon(j.state)}
    <button type="button" class="flex min-w-0 flex-1 flex-col items-start py-1.5 text-start" onclick={() => open(j.id)}>
      <span class="w-full truncate">{j.title}</span>
      <span class="p-subhead-2 w-full truncate text-secondary-label">
        {ref(j.project, j.targetType, j.iid)}{j.kind === "issues" ? " · label" : ""} · @{j.author} · {STATES[j.state] ?? j.state}{duration(j) ? ` · ${duration(j)}` : ""}{j.simulated
          ? " · simulé"
          : ""}
      </span>
    </button>
    {#if j.state === "running" || j.state === "queued"}
      <Button size="small" onclick={() => Jobs.Cancel(j.id)}>Annuler</Button>
    {/if}
  </div>
{/snippet}

{#if current}
  <!-- Stays at the top of the page while the transcript scrolls under it;
       the negative offsets cancel the page padding. -->
  <div bind:this={header} class="sticky -top-[20px] z-10 -mx-[24px] mt-2 border-b border-separator bg-window px-[24px] pt-3 pb-2">
    <div class="flex items-center gap-2">
      <Button size="small" variant="borderless" onclick={() => (selected = null)}>
        <ArrowLeftIcon size={14} aria-hidden="true" />
        Jobs
      </Button>
      <div class="flex min-w-0 flex-1 items-center gap-1.5">
        {@render stateIcon(current.state)}
        <span class="truncate">{current.title}</span>
      </div>
      <Button size="small" variant="borderless" onclick={() => Browser.OpenURL(current.url)}>
        <ArrowSquareOutIcon size={14} aria-hidden="true" />
        {current.targetType === "Issue" ? "Issue" : "MR"}
      </Button>
      {#if current.openedMr}
        <Button size="small" variant="borderless" onclick={() => Browser.OpenURL(current.openedMr)}>
          <ArrowSquareOutIcon size={14} aria-hidden="true" />
          MR ouverte
        </Button>
      {/if}
      {#if current.state === "running" || current.state === "queued"}
        <Button size="small" onclick={() => Jobs.Cancel(current.id)}>Annuler</Button>
      {/if}
    </div>
    <p class="p-subhead-2 mt-1 ms-2.5 text-secondary-label">
      {ref(current.project, current.targetType, current.iid)} · @{current.author} · {STATES[current.state]}{current.profile
        ? ` · ${current.profile}`
        : ""}{duration(current) ? ` · ${duration(current)}` : ""}{current.turns ? ` · ${current.turns} tours` : ""}
    </p>
    {#if current.error}
      <p class="p-subhead-2 mt-2 ms-2.5 line-clamp-3 text-label select-text">{current.error}</p>
    {/if}
  </div>

  <JobChat {events} live={current.state === "running" || current.state === "queued"} />

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
{:else}
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
{/if}
