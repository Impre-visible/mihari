<script lang="ts">
  import type { Snippet } from "svelte";

  type Props = {
    /** Accessible name of the group. */
    label: string;
    small?: boolean;
    vertical?: boolean;
    children: Snippet;
  };

  let { label, small = false, vertical = false, children }: Props = $props();
</script>

<!-- A Liquid Glass capsule of toolbar items. -->
<div
  class={["p-toolbar-group", small && "p-toolbar-group-small", vertical && "p-toolbar-group-vertical"]}
  role="toolbar"
  aria-label={label}
  aria-orientation={vertical ? "vertical" : "horizontal"}
>
  {@render children()}
</div>
