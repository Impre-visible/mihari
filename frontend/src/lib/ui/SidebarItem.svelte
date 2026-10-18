<script lang="ts">
  import type { Component, Snippet } from "svelte";

  type Props = {
    selected?: boolean;
    /** A phosphor-svelte icon component. Sidebar icons take the accent tint. */
    icon?: Component<{ size?: number | string; weight?: "regular" | "fill" }>;
    /** Trailing secondary text, such as a count. */
    detail?: string | number;
    /** Indentation level, 0 to 4. */
    level?: number;
    disabled?: boolean;
    onclick?: () => void;
    children: Snippet;
  };

  let { selected = false, icon: Icon, detail, level, disabled = false, onclick, children }: Props = $props();
</script>

<button
  type="button"
  class={["p-sidebar-row", selected && "p-selected", disabled && "p-dimmed"]}
  aria-current={selected ? "page" : undefined}
  data-p-level={level || undefined}
  {disabled}
  {onclick}
>
  {#if Icon}
    <span class="p-sidebar-icon"><Icon size="100%" weight={selected ? "fill" : "regular"} /></span>
  {/if}
  <span class="ui-sidebar-label">{@render children()}</span>
  {#if detail !== undefined}<span class="p-sidebar-detail">{detail}</span>{/if}
</button>
