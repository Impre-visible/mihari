<script lang="ts">
  import type { HTMLInputAttributes } from "svelte/elements";
  import { sizeClass, type Size } from "./types";

  type Props = Omit<HTMLInputAttributes, "size"> & {
    value?: string;
    size?: Size;
  };

  let { value = $bindable(""), size = "regular", type = "text", class: className, ...rest }: Props = $props();
</script>

<input
  {type}
  class={["p-field", sizeClass("p-field", size), className]}
  bind:value
  autocomplete="off"
  autocorrect="off"
  spellcheck="false"
  {...rest}
/>
