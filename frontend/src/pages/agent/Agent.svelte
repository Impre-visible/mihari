<script lang="ts">
  import { untrack } from "svelte";
  import { CpuIcon, DesktopTowerIcon, PlusIcon } from "phosphor-svelte";
  import { Button, ListHeading, ListRow, Select, SplitView, Spinner } from "$lib/ui";
  import { live } from "$lib/live.svelte";
  import MachineInspector from "./MachineInspector.svelte";
  import ProfileInspector from "./ProfileInspector.svelte";
  import WeekStrip from "./WeekStrip.svelte";
  import { agent, blankProfile, changed, hue, load, profileProblem } from "./store.svelte";

  // Profiles on the left, under the week they share; the profile picked, or
  // the machine, on the right. Every change saves itself.
  load();

  let selected = $state<string | null>(null);
  let current = $derived(agent.profiles.find((p) => p.id === selected) ?? (selected === "machine" ? null : agent.profiles[0]) ?? null);
  let showMachine = $derived(selected === "machine" || (agent.loaded && agent.profiles.length === 0));

  // Saves after an edit of the profiles or the schedule, not on loading:
  // only when they differ from what was last seen.
  let seen: string | null = null;
  $effect(() => {
    const now = JSON.stringify([agent.profiles, agent.segments, agent.fallback]);
    if (!agent.loaded) return;
    if (seen !== null && seen !== now) untrack(changed);
    seen = now;
  });

  let fallbackOptions = $derived([
    { value: "", label: "En pause" },
    ...agent.profiles.filter((p) => p.name.trim()).map((p) => ({ value: p.id, label: p.name })),
  ]);

  function add() {
    const p = blankProfile();
    agent.profiles.push(p);
    selected = p.id;
  }

  function remove(id: string) {
    const name = agent.profiles.find((p) => p.id === id)?.name.trim() || "ce profil";
    if (!confirm(`Supprimer ${name} et ses plages horaires ?`)) return;
    agent.profiles = agent.profiles.filter((p) => p.id !== id);
    agent.segments = agent.segments.filter((s) => s.profile !== id);
    if (agent.fallback === id) agent.fallback = "";
    selected = null;
  }

  function hours(id: string) {
    const segs = agent.segments.filter((s) => s.profile === id);
    if (segs.length === 0) return agent.fallback === id ? "le reste du temps" : "aucune plage";
    return segs.map((s) => `${s.start}–${s.end}`).join(", ") + (agent.fallback === id ? " + le reste" : "");
  }
</script>

<SplitView width={300}>
  {#snippet header()}
    <WeekStrip selected={showMachine ? null : current?.id} onpick={(id) => (selected = id)} />
    <div class="mt-2 flex items-center gap-2">
      <span class="p-subhead-2 flex-1 text-secondary-label">Le reste du temps</span>
      <Select size="small" aria-label="Profil le reste du temps" options={fallbackOptions} bind:value={agent.fallback} />
    </div>
  {/snippet}

  {#snippet list()}
    <ListHeading title="Profils" />
    {#each agent.profiles as p (p.id)}
      {@const problem = profileProblem(p)}
      <ListRow
        title={p.name.trim() || "Nouveau profil"}
        subtitle={problem ? `Incomplet : ${problem}` : `${p.model} · ${hours(p.id)}`}
        alert={!!problem}
        selected={!showMachine && current?.id === p.id}
        onclick={() => (selected = p.id)}
      >
        {#snippet leading()}
          <span class="size-2.5 rounded-full" style:background="hsl({hue(p.id)} 70% 55%)" aria-hidden="true"></span>
        {/snippet}
        {#snippet trailing()}
          {#if live.engine?.profileId === p.id && live.engine.state !== "idle"}
            <span class="p-caption-1 text-(--p-sys-green)">actif</span>
          {/if}
        {/snippet}
      </ListRow>
    {:else}
      {#if !agent.loaded}<div class="px-2.5 py-2"><Spinner small label="Chargement" /></div>{/if}
    {/each}

    <ListHeading title="Ce Mac / ce PC" />
    <ListRow title="Machine" subtitle="Matériel et modèles qui tiennent" selected={showMachine} onclick={() => (selected = "machine")}>
      {#snippet leading()}<DesktopTowerIcon size={16} class="text-secondary-label" aria-hidden="true" />{/snippet}
    </ListRow>
  {/snippet}

  {#snippet footer()}
    <Button size="small" variant="borderless" onclick={add}><PlusIcon size={13} aria-hidden="true" /> Nouveau profil</Button>
  {/snippet}

  {#if showMachine}
    <MachineInspector onapplied={(id) => (selected = id)} />
  {:else if current}
    {#key current.id}<ProfileInspector profile={current} ondelete={() => remove(current!.id)} />{/key}
  {:else}
    <div class="flex h-full flex-col items-center justify-center gap-2 text-center">
      <CpuIcon size={32} class="text-tertiary-label" aria-hidden="true" />
      <p class="p-title-3 text-secondary-label">Aucun profil</p>
      <p class="p-subhead-2 max-w-sm text-tertiary-label">Un profil, c'est un moteur, un modèle, un prompt et les heures où il tourne.</p>
      <Button variant="primary" onclick={add}>Créer un profil</Button>
    </div>
  {/if}
</SplitView>
