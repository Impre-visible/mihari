<script lang="ts">
  import type { Component } from "svelte";
  import { CpuIcon, FlaskIcon, GearSixIcon, GitlabLogoIcon, PulseIcon, WrenchIcon } from "phosphor-svelte";
  import { Events } from "@wailsio/runtime";
  import { AppWindow, Sidebar, SidebarItem, SidebarSection } from "$lib/ui";
  import { live, start } from "$lib/live.svelte";
  import { nav } from "$lib/nav.svelte";
  import { subtitle } from "$lib/save.svelte";
  import Activity from "./activity/Activity.svelte";
  import Agent from "./agent/Agent.svelte";
  import GitLab from "./gitlab/GitLab.svelte";
  import General from "./components/General.svelte";
  import Bench from "./components/Bench.svelte";
  import Dev from "./components/Dev.svelte";
  import UpdateBanner from "./components/UpdateBanner.svelte";
  import { live as running } from "./activity/jobState";

  // The window: what the bot does first, what it is set up with next, its
  // tools last.
  type Section = { id: string; label: string; icon: typeof PulseIcon; component: Component };

  const activity: Section = { id: "activity", label: "Activité", icon: PulseIcon, component: Activity };
  const setup: Section[] = [
    { id: "agent", label: "Agent", icon: CpuIcon, component: Agent },
    { id: "gitlab", label: "GitLab", icon: GitlabLogoIcon, component: GitLab },
    { id: "settings", label: "Réglages", icon: GearSixIcon, component: General },
  ];
  // Vite sets DEV under wails3 dev only, so production builds never show Dev.
  const tools: Section[] = [
    { id: "bench", label: "Banc d'essai", icon: FlaskIcon, component: Bench },
    ...(import.meta.env.DEV ? [{ id: "dev", label: "Dev", icon: WrenchIcon, component: Dev }] : []),
  ];

  start();

  // A click on a job notification opens it.
  $effect(() =>
    Events.On("desktop:open-job", (event) => {
      nav.section = "activity";
      nav.jobId = Number(event.data);
    }),
  );

  let active = $derived([activity, ...setup, ...tools].find((s) => s.id === nav.section) ?? activity);
  let busy = $derived(live.jobs.filter(running).length);
</script>

{#snippet item(s: Section, detail?: number)}
  <SidebarItem icon={s.icon} selected={active.id === s.id} detail={detail || undefined} onclick={() => (nav.section = s.id)}>
    {s.label}
  </SidebarItem>
{/snippet}

<AppWindow title={active.label} subtitle={active.id === "activity" ? undefined : subtitle()}>
  {#snippet sidebar()}
    <Sidebar label="Sections">
      {@render item(activity, busy)}
      <SidebarSection title="Configuration">
        {#each setup as s (s.id)}{@render item(s)}{/each}
      </SidebarSection>
      <SidebarSection title="Outils">
        {#each tools as s (s.id)}{@render item(s)}{/each}
      </SidebarSection>
    </Sidebar>
  {/snippet}

  <UpdateBanner />
  <active.component />
</AppWindow>
