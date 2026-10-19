<script lang="ts">
  import { SegmentedControl } from "$lib/ui";
  import Profiles from "../components/Profiles.svelte";
  import Schedule from "../components/Schedule.svelte";
  import Machine from "../components/Machine.svelte";

  let tab = $state("profiles");
</script>

<SegmentedControl
  label="Agent"
  class="mb-4"
  options={[
    { value: "profiles", label: "Profils" },
    { value: "schedule", label: "Planning" },
    { value: "machine", label: "Machine" },
  ]}
  bind:value={tab}
/>
{#if tab === "profiles"}<Profiles />{:else if tab === "schedule"}<Schedule />{:else}<Machine />{/if}
