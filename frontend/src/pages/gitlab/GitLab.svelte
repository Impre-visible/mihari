<script lang="ts">
  import { SegmentedControl } from "$lib/ui";
  import Gitlab from "../components/Gitlab.svelte";
  import Projects from "../components/Projects.svelte";

  let tab = $state("connection");
</script>

<SegmentedControl
  label="GitLab"
  class="mb-4"
  options={[
    { value: "connection", label: "Connexion" },
    { value: "projects", label: "Projets" },
  ]}
  bind:value={tab}
/>
{#if tab === "connection"}<Gitlab />{:else}<Projects />{/if}
