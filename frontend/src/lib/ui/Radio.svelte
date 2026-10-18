<script lang="ts">
  import type { Snippet } from "svelte";
  import type { HTMLInputAttributes } from "svelte/elements";
  import { tintClass, type Tint } from "./types";

  type Props = Omit<HTMLInputAttributes, "type"> & {
    /** The selected value of the whole group; bind the same variable on every radio. */
    group?: unknown;
    tint?: Tint;
    children?: Snippet;
  };

  let { group = $bindable(), tint, class: className, children, ...rest }: Props = $props();
</script>

<label class={["p-radio", tintClass(tint), className]}>
  <input type="radio" bind:group {...rest} />
  <span></span>
  {@render children?.()}
</label>
