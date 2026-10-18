<script lang="ts">
  import type { HTMLInputAttributes } from "svelte/elements";
  import { tintClass, type Tint } from "./types";

  type Props = Omit<HTMLInputAttributes, "size" | "type" | "value" | "min" | "max"> & {
    value?: number;
    min?: number;
    max?: number;
    size?: "mini" | "small" | "regular" | "large";
    /** Fill grows from the middle, for values with a neutral midpoint. */
    centered?: boolean;
    /** Number of tick intervals under the track; match it to `step`. */
    ticks?: number;
    tint?: Tint;
  };

  let {
    value = $bindable(0),
    min = 0,
    max = 100,
    size = "regular",
    centered = false,
    ticks,
    tint,
    class: className,
    ...rest
  }: Props = $props();

  // Puppertino's sliders.js publishes this on input; doing it here keeps the fill reactive.
  let fill = $derived(`${max === min ? 0 : ((value - min) / (max - min)) * 100}%`);
</script>

<input
  type="range"
  class={[
    "p-slider",
    size !== "regular" && `p-slider-${size}`,
    centered && "p-slider-centered",
    ticks && "p-slider-ticks",
    tintClass(tint),
    className,
  ]}
  style:--p-slider-fill={fill}
  style:--p-slider-ticks={ticks}
  data-p-lens="clear"
  {min}
  {max}
  bind:value
  {...rest}
/>
