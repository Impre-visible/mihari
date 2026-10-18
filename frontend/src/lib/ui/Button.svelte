<script lang="ts">
  import type { Snippet } from "svelte";
  import type { HTMLButtonAttributes } from "svelte/elements";
  import { sizeClass, tintClass, type Size, type Tint } from "./types";

  type Props = HTMLButtonAttributes & {
    /** `primary` is the single default action of a view; `default` is the neutral gray button. */
    variant?: "default" | "primary" | "colored" | "secondary" | "destructive" | "borderless";
    size?: Size;
    tint?: Tint;
    /** Round button holding a single glyph. Requires an `aria-label`. */
    iconOnly?: boolean;
    children?: Snippet;
  };

  let {
    variant = "default",
    size = "regular",
    tint,
    iconOnly = false,
    type = "button",
    class: className,
    children,
    ...rest
  }: Props = $props();
</script>

<button
  {type}
  class={[
    "p-button",
    `p-button-${variant}`,
    sizeClass("p-button", size),
    iconOnly && "p-button-icon-only",
    tintClass(tint),
    className,
  ]}
  {...rest}
>
  {@render children?.()}
</button>
