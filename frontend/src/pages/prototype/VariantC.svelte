<script lang="ts">
  // PROTOTYPE (UI redesign), variant C: a table. Tabs for running, queued and
  // past jobs, sortable columns, the job in a drawer over the table, the
  // system's health in a status bar at the bottom.
  import { fly, fade } from "svelte/transition";
  import { cubicOut } from "svelte/easing";
  import { ArrowSquareOutIcon, XIcon } from "phosphor-svelte";
  import { Browser } from "@wailsio/runtime";
  import type { Summary } from "../../../bindings/github.com/Impre-visible/mihari/internal/jobs/models";
  import { Button, SegmentedControl } from "$lib/ui";
  import { ref } from "$lib/refs";
  import JobState from "../components/JobState.svelte";
  import { STATES, duration, live as isLive } from "../components/jobState";
  import Health from "./Health.svelte";
  import JobDetail from "./JobDetail.svelte";
  import { ago, live } from "./data.svelte";

  let tab = $state<"running" | "queue" | "history">("running");
  let running = $derived(live.jobs.filter(isLive));
  let history = $derived(live.jobs.filter((j) => !isLive(j)));
  let queue = $derived([
    ...(live.tasks?.todos ?? []).map((t) => ({ todo: t, note: "Prête" })),
    ...(live.tasks?.waiting ?? []).map((w) => ({ todo: w.todo, note: w.reason })),
  ]);
  let rows = $derived(tab === "running" ? running : history);

  type Sort = "when" | "title" | "duration" | "turns";
  let sort = $state<Sort>("when");
  let sorted = $derived(
    [...rows].sort((a, b) => {
      if (sort === "title") return a.title.localeCompare(b.title);
      if (sort === "turns") return b.turns - a.turns;
      if (sort === "duration") return seconds(b) - seconds(a);
      return b.id - a.id;
    }),
  );
  function seconds(j: Summary) {
    if (j.startedAt.startsWith("0001")) return 0;
    const end = j.endedAt.startsWith("0001") ? live.now : Date.parse(j.endedAt);
    return end - Date.parse(j.startedAt);
  }

  let drawer = $state<Summary | null>(null);
  let shown = $derived(drawer ? (live.jobs.find((j) => j.id === drawer!.id) ?? drawer) : null);
</script>

<svelte:window onkeydown={(e) => e.key === "Escape" && (drawer = null)} />

{#snippet th(label: string, key: Sort | null, cls = "")}
  <th class={["p-caption-1 border-b border-separator px-3 py-2 text-start font-semibold text-secondary-label", cls]}>
    {#if key}
      <button type="button" class={sort === key ? "text-label" : ""} onclick={() => (sort = key)}>{label}{sort === key ? " ↓" : ""}</button>
    {:else}{label}{/if}
  </th>
{/snippet}

<div class="-mx-[24px] -mt-[20px] -mb-[24px] flex h-[calc(100%+44px)] flex-col">
  <div class="flex items-center gap-3 border-b border-separator px-6 py-3">
    <SegmentedControl
      label="Jobs"
      options={[
        { value: "running", label: `En cours (${running.length})` },
        { value: "queue", label: `File d'attente (${queue.length})` },
        { value: "history", label: `Historique (${history.length})` },
      ]}
      bind:value={tab}
    />
  </div>

  <div class="min-h-0 flex-1 overflow-y-auto">
    {#if tab === "queue"}
      <table class="w-full table-fixed border-collapse">
        <thead><tr>{@render th("Titre", null)}{@render th("Cible", null, "w-72")}{@render th("Demandeur", null, "w-36")}{@render th("État", null, "w-40")}{@render th("", null, "w-12")}</tr></thead>
        <tbody>
          {#each queue as q (q.todo.id)}
            <tr class="border-b border-separator/60">
              <td class="p-subhead px-3 py-2">{q.todo.title}</td>
              <td class="p-subhead-2 truncate px-3 py-2 font-mono text-secondary-label">{ref(q.todo.project, q.todo.targetType, q.todo.iid)}</td>
              <td class="p-subhead-2 px-3 py-2 text-secondary-label">@{q.todo.author}</td>
              <td class="p-subhead-2 px-3 py-2 text-secondary-label">{q.note}</td>
              <td class="px-3 py-2">
                <Button size="small" variant="borderless" iconOnly aria-label="Ouvrir sur GitLab" onclick={() => Browser.OpenURL(q.todo.url)}>
                  <ArrowSquareOutIcon size={13} aria-hidden="true" />
                </Button>
              </td>
            </tr>
          {:else}
            <tr><td colspan="5" class="p-subhead py-10 text-center text-tertiary-label">Rien en attente.</td></tr>
          {/each}
        </tbody>
      </table>
    {:else}
      <table class="w-full table-fixed border-collapse">
        <thead>
          <tr>
            {@render th("", null, "w-10")}{@render th("Titre", "title")}{@render th("Demandeur", null, "w-36")}{@render th("Profil", null, "w-44")}{@render th("Durée", "duration", "w-24")}{@render th("Tours", "turns", "w-16 text-end")}{@render th("Quand", "when", "w-28")}
          </tr>
        </thead>
        <tbody>
          {#each sorted as j (j.id)}
            <tr
              class={["cursor-default border-b border-separator/60", shown?.id === j.id ? "bg-(--p-sys-blue)/15" : "hover:bg-fill"]}
              onclick={() => (drawer = j)}
            >
              <td class="px-3 py-2"><JobState state={j.state} /></td>
              <td class="px-3 py-2">
                <p class="p-subhead truncate">{j.title}</p>
                <p class={["p-subhead-2 truncate", j.error ? "text-(--p-sys-red)" : "text-secondary-label"]}>
                  {j.error || `${ref(j.project, j.targetType, j.iid)} · ${STATES[j.state]}`}
                </p>
              </td>
              <td class="p-subhead-2 truncate px-3 py-2 text-secondary-label">@{j.author}</td>
              <td class="p-subhead-2 truncate px-3 py-2 text-secondary-label">{j.profile}</td>
              <td class="p-subhead-2 px-3 py-2 whitespace-nowrap text-secondary-label tabular-nums">{duration(j, live.now)}</td>
              <td class="p-subhead-2 px-3 py-2 text-end text-secondary-label tabular-nums">{j.turns || ""}</td>
              <td class="p-subhead-2 px-3 py-2 whitespace-nowrap text-secondary-label">{ago(j.queuedAt, live.now)}</td>
            </tr>
          {:else}
            <tr><td colspan="7" class="p-subhead py-10 text-center text-tertiary-label">{tab === "running" ? "Aucun job en cours." : "Aucun job passé."}</td></tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>

  <footer class="border-t border-separator px-4 py-2"><Health /></footer>
</div>

{#if shown}
  <button type="button" class="fixed inset-0 z-20 bg-black/15" aria-label="Fermer" transition:fade={{ duration: 150 }} onclick={() => (drawer = null)}></button>
  <aside
    class="fixed inset-y-0 right-0 z-30 flex w-[min(720px,70vw)] flex-col border-s border-separator bg-window shadow-2xl"
    transition:fly={{ x: 80, duration: 220, easing: cubicOut }}
  >
    <div class="flex justify-end p-2">
      <Button size="small" variant="borderless" iconOnly aria-label="Fermer" onclick={() => (drawer = null)}>
        <XIcon size={14} aria-hidden="true" />
      </Button>
    </div>
    <div class="min-h-0 flex-1 overflow-y-auto px-5 pb-5">
      {#key shown.id}<JobDetail job={shown} />{/key}
    </div>
  </aside>
{/if}
