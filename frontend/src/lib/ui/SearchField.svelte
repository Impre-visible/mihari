<script lang="ts">
  import type { HTMLInputAttributes } from "svelte/elements";
  import { sizeClass, type Size } from "./types";

  type Props = Omit<HTMLInputAttributes, "size" | "type"> & {
    value?: string;
    size?: Size;
  };

  let {
    value = $bindable(""),
    size = "regular",
    placeholder = "Rechercher",
    class: className,
    ...rest
  }: Props = $props();
</script>

<div class={["p-search", sizeClass("p-search", size), className]}>
  <input type="search" bind:value {placeholder} aria-label={placeholder} spellcheck="false" {...rest} />
</div>
