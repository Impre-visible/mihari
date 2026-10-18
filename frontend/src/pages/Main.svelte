<script lang="ts">
  import { ClockIcon, CpuIcon, GaugeIcon, GitlabLogoIcon } from "phosphor-svelte";
  import { AppWindow, Box, Sidebar, SidebarItem, SidebarSection } from "$lib/ui";

  type Section = { id: string; label: string; description: string; icon: typeof GaugeIcon };

  const sections: Section[] = [
    { id: "status", label: "État", icon: GaugeIcon, description: "Modèle chargé, jobs en cours et en attente." },
    {
      id: "profiles",
      label: "Profils",
      icon: CpuIcon,
      description: "Le modèle de journée, le modèle de nuit et leurs jobs en parallèle.",
    },
    {
      id: "schedule",
      label: "Planning",
      icon: ClockIcon,
      description: "Les plages horaires et le profil actif sur chacune.",
    },
    {
      id: "gitlab",
      label: "GitLab",
      icon: GitlabLogoIcon,
      description: "L'URL de l'instance, le compte bot et son token.",
    },
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

  <p class="p-body text-secondary-label">{active.description}</p>

  <Box class="mt-6">
    <p class="py-8 text-center text-tertiary-label">À venir</p>
  </Box>
</AppWindow>
