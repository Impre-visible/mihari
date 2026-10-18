<script lang="ts">
  import type { Snippet } from "svelte";
  import type { HTMLInputAttributes } from "svelte/elements";
  import { tintClass, type Tint } from "./types";

  type Props = Omit<HTMLInputAttributes, "size" | "type"> & {
    checked?: boolean;
    size?: "regular" | "large" | "xlarge";
    tint?: Tint;
    children?: Snippet;
  };

  let {
    checked = $bindable(false),
    size = "regular",
    tint,
    class: className,
    children,
    ...rest
  }: Props = $props();
</script>

<label class={["p-checkbox", size !== "regular" && `p-checkbox-${size}`, tintClass(tint), className]}>
  <input type="checkbox" bind:checked {...rest} />
  <span></span>
  {@render children?.()}
</label>
