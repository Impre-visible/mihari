<script lang="ts">
  import type { HTMLInputAttributes } from "svelte/elements";
  import { sizeClass, tintClass, type Size, type Tint } from "./types";

  type Props = Omit<HTMLInputAttributes, "size" | "type"> & {
    checked?: boolean;
    /** The switch has no visible text, so it always needs an accessible name. */
    label: string;
    size?: Size;
    tint?: Tint;
  };

  let {
    checked = $bindable(false),
    label,
    size = "mini",
    tint,
    class: className,
    ...rest
  }: Props = $props();
</script>

<!-- data-p-lens="clear": the flat macOS 27 knob instead of the magnified liquid lens. -->
<label
  class={["p-switch", sizeClass("p-switch", size), tintClass(tint), className]}
  aria-label={label}
  data-p-lens="clear"
>
  <input type="checkbox" role="switch" bind:checked {...rest} />
  <span></span>
</label>
