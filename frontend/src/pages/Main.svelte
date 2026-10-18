<script lang="ts">
  import type { Component } from "svelte";
  import { ClockIcon, CpuIcon, GaugeIcon, GitlabLogoIcon } from "phosphor-svelte";
  import { AppWindow, Sidebar, SidebarItem, SidebarSection } from "$lib/ui";
  import Status from "./components/Status.svelte";
  import Profiles from "./components/Profiles.svelte";
  import Schedule from "./components/Schedule.svelte";
  import Gitlab from "./components/Gitlab.svelte";

  type Section = { id: string; label: string; icon: typeof GaugeIcon; component: Component };

  const sections: Section[] = [
    { id: "status", label: "État", icon: GaugeIcon, component: Status },
    { id: "gitlab", label: "GitLab", icon: GitlabLogoIcon, component: Gitlab },
    { id: "profiles", label: "Profils", icon: CpuIcon, component: Profiles },
    { id: "schedule", label: "Planning", icon: ClockIcon, component: Schedule },
  ];

  let activeId = $state(sections[0].id);
  let active = $derived(sections.find((s) => s.id === activeId) ?? sections[0]);
</script>

<AppWindow title={active.label}>
  {#snippet sidebar()}
    <Sidebar label="Sections">
      <SidebarItem icon={GaugeIcon} selected={activeId === "status"} onclick={() => (activeId = "status")}>
        État
      </SidebarItem>

      <SidebarSection title="Réglages">
        {#each sections.slice(1) as section (section.id)}
          <SidebarItem
            icon={section.icon}
            selected={section.id === activeId}
            onclick={() => (activeId = section.id)}
          >
            {section.label}
          </SidebarItem>
        {/each}
      </SidebarSection>
    </Sidebar>
  {/snippet}

  <active.component />
</AppWindow>
