<script lang="ts">
  // PROTOTYPE (UI redesign): the app's shell with the navigation by use:
  // Activité · Agent · GitLab · Banc d'essai · Réglages. The activity page
  // comes in three variants, switched from the floating bar. Throwaway.
  import { CpuIcon, FlaskIcon, GearSixIcon, GitlabLogoIcon, PulseIcon, WrenchIcon } from "phosphor-svelte";
  import { AppWindow, SegmentedControl, Sidebar, SidebarItem, SidebarSection } from "$lib/ui";
  import Profiles from "../components/Profiles.svelte";
  import Schedule from "../components/Schedule.svelte";
  import Machine from "../components/Machine.svelte";
  import Gitlab from "../components/Gitlab.svelte";
  import Projects from "../components/Projects.svelte";
  import Bench from "../components/Bench.svelte";
  import General from "../components/General.svelte";
  import Dev from "../components/Dev.svelte";
  import UpdateBanner from "../components/UpdateBanner.svelte";
  import Switcher from "./Switcher.svelte";
  import { live, start } from "./data.svelte";
  import { proto } from "./state.svelte";
  import VariantA from "./VariantA.svelte";
  import VariantB from "./VariantB.svelte";
  import VariantC from "./VariantC.svelte";

  start();

  let section = $state("activity");
  let agentTab = $state("profiles");
  let gitlabTab = $state("connection");
  let running = $derived(live.jobs.filter((j) => j.state === "running" || j.state === "queued").length);

  const TITLES: Record<string, string> = {
    activity: "Activité",
    agent: "Agent",
    gitlab: "GitLab",
    bench: "Banc d'essai",
    settings: "Réglages",
    dev: "Dev",
  };
</script>

<AppWindow title={TITLES[section]}>
  {#snippet sidebar()}
    <Sidebar label="Sections">
      <SidebarItem icon={PulseIcon} selected={section === "activity"} detail={running || undefined} onclick={() => (section = "activity")}>
        Activité
      </SidebarItem>
      <SidebarSection title="Configuration">
        <SidebarItem icon={CpuIcon} selected={section === "agent"} onclick={() => (section = "agent")}>Agent</SidebarItem>
        <SidebarItem icon={GitlabLogoIcon} selected={section === "gitlab"} onclick={() => (section = "gitlab")}>GitLab</SidebarItem>
        <SidebarItem icon={GearSixIcon} selected={section === "settings"} onclick={() => (section = "settings")}>Réglages</SidebarItem>
      </SidebarSection>
      <SidebarSection title="Outils">
        <SidebarItem icon={FlaskIcon} selected={section === "bench"} onclick={() => (section = "bench")}>Banc d'essai</SidebarItem>
        <SidebarItem icon={WrenchIcon} selected={section === "dev"} onclick={() => (section = "dev")}>Dev</SidebarItem>
      </SidebarSection>
    </Sidebar>
  {/snippet}

  <UpdateBanner />
  {#if section === "activity"}
    {#if proto.variant === "B"}
      <VariantB />
    {:else if proto.variant === "C"}
      <VariantC />
    {:else}
      <VariantA />
    {/if}
    <Switcher />
  {:else if section === "agent"}
    <SegmentedControl
      label="Agent"
      class="mb-4"
      options={[
        { value: "profiles", label: "Profils" },
        { value: "schedule", label: "Planning" },
        { value: "machine", label: "Machine" },
      ]}
      bind:value={agentTab}
    />
    {#if agentTab === "profiles"}<Profiles />{:else if agentTab === "schedule"}<Schedule />{:else}<Machine />{/if}
  {:else if section === "gitlab"}
    <SegmentedControl
      label="GitLab"
      class="mb-4"
      options={[
        { value: "connection", label: "Connexion" },
        { value: "projects", label: "Projets" },
      ]}
      bind:value={gitlabTab}
    />
    {#if gitlabTab === "connection"}<Gitlab />{:else}<Projects />{/if}
  {:else if section === "bench"}
    <Bench />
  {:else if section === "settings"}
    <General />
  {:else}
    <Dev />
  {/if}
</AppWindow>
