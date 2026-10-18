<script lang="ts">
  import type { Component, Snippet } from "svelte";
  import type { HTMLButtonAttributes } from "svelte/elements";

  type Props = HTMLButtonAttributes & {
    /** Name announced and shown as the tooltip, since the button is only a glyph. */
    label: string;
    icon?: Component<{ size?: number | string; weight?: "bold" }>;
    /** Toggle state; omit for plain actions. */
    pressed?: boolean;
    /** Standalone round glass button instead of an item inside a `Toolbar` group. */
    circle?: boolean;
    children?: Snippet;
  };

  let { label, icon: Icon, pressed, circle = false, class: className, children, ...rest }: Props = $props();
</script>

<button
  type="button"
  class={[circle ? "p-toolbar-circle" : "p-toolbar-btn", children && !Icon && "p-toolbar-text-btn", className]}
  aria-label={Icon ? label : undefined}
  aria-pressed={pressed}
  data-p-tooltip={Icon ? label : undefined}
  {...rest}
>
  {#if Icon}<Icon weight="bold" />{/if}
  {@render children?.()}
</button>
