<script lang="ts" generics="T extends string">
  import type { HTMLSelectAttributes } from "svelte/elements";
  import { normalizeOptions, sizeClass, type Option, type Size } from "./types";

  type Props = Omit<HTMLSelectAttributes, "size" | "value"> & {
    value?: T;
    options: Option<T>[];
    size?: Size;
    /** `borderless` is the settings-row look: the value plus a small chevron plate. */
    variant?: "popup" | "borderless";
  };

  let {
    value = $bindable(),
    options,
    size = "regular",
    variant = "popup",
    class: className,
    ...rest
  }: Props = $props();

  let items = $derived(normalizeOptions(options));
</script>

<div class={["p-popup", sizeClass("p-popup", size), variant === "borderless" && "p-popup-borderless", className]}>
  <select bind:value {...rest}>
    {#each items as item (item.value)}
      <option value={item.value} disabled={item.disabled}>{item.label}</option>
    {/each}
  </select>
</div>
