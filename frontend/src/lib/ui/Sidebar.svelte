<script lang="ts">
  import type { Snippet } from "svelte";

  type Props = {
    /** Accessible name of the navigation landmark. */
    label: string;
    size?: "small" | "medium" | "large";
    children: Snippet;
  };

  let { label, size = "medium", children }: Props = $props();
</script>

<nav class={["p-sidebar", "p-sidebar-attached", size !== "large" && `p-sidebar-${size}`, "ui-sidebar"]} aria-label={label}>
  <!-- Header zone: the native traffic lights sit here, and it drags the window. -->
  <div class="p-sidebar-titlebar" data-drag></div>
  {@render children()}
</nav>
