<script lang="ts">
  import type { Component } from "svelte";
  import { ClockIcon, CpuIcon, GaugeIcon, DesktopTowerIcon, FolderIcon, GearSixIcon, GitlabLogoIcon, WrenchIcon } from "phosphor-svelte";
  import { AppWindow, Sidebar, SidebarItem, SidebarSection } from "$lib/ui";
  import Status from "./components/Status.svelte";
  import Profiles from "./components/Profiles.svelte";
  import Schedule from "./components/Schedule.svelte";
  import Gitlab from "./components/Gitlab.svelte";
  import Dev from "./components/Dev.svelte";
  import Projects from "./components/Projects.svelte";
  import Machine from "./components/Machine.svelte";
  import General from "./components/General.svelte";
  import UpdateBanner from "./components/UpdateBanner.svelte";
  import { Events } from "@wailsio/runtime";
  import { nav } from "$lib/nav.svelte";

  type Section = { id: string; label: string; icon: typeof GaugeIcon; component: Component };

  const sections: Section[] = [
    { id: "status", label: "État", icon: GaugeIcon, component: Status },
    { id: "gitlab", label: "GitLab", icon: GitlabLogoIcon, component: Gitlab },
    { id: "projects", label: "Projets", icon: FolderIcon, component: Projects },
    { id: "profiles", label: "Profils", icon: CpuIcon, component: Profiles },
    { id: "machine", label: "Machine", icon: DesktopTowerIcon, component: Machine },
    { id: "schedule", label: "Planning", icon: ClockIcon, component: Schedule },
    { id: "general", label: "Général", icon: GearSixIcon, component: General },
  ];

  // Vite sets DEV under wails3 dev only, so production builds never show it.
  const devSection: Section | null = import.meta.env.DEV
    ? { id: "dev", label: "Dev", icon: WrenchIcon, component: Dev }
    : null;

  // A click on a job notification opens its transcript.
  $effect(() =>
    Events.On("desktop:open-job", (event) => {
      nav.section = "status";
      nav.jobId = Number(event.data);
    }),
  );
  let active = $derived(
    [...sections, ...(devSection ? [devSection] : [])].find((s) => s.id === nav.section) ?? sections[0],
  );
</script>

<AppWindow title={active.label}>
  {#snippet sidebar()}
    <Sidebar label="Sections">
      <SidebarItem icon={GaugeIcon} selected={nav.section === "status"} onclick={() => (nav.section = "status")}>
        État
      </SidebarItem>

      <SidebarSection title="Réglages">
        {#each sections.slice(1) as section (section.id)}
          <SidebarItem
            icon={section.icon}
            selected={section.id === nav.section}
            onclick={() => (nav.section = section.id)}
          >
            {section.label}
          </SidebarItem>
        {/each}
      </SidebarSection>

      {#if devSection}
        <SidebarSection title="Développement">
          <SidebarItem
            icon={devSection.icon}
            selected={nav.section === devSection.id}
            onclick={() => (nav.section = devSection.id)}
          >
            {devSection.label}
          </SidebarItem>
        </SidebarSection>
      {/if}
    </Sidebar>
  {/snippet}

  <UpdateBanner />
  <active.component />
</AppWindow>
