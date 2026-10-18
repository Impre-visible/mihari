<script lang="ts">
  import { tintClass, type Tint } from "./types";

  type Props = {
    /** 0 to `max`. Leave undefined for the indeterminate bar. */
    value?: number;
    max?: number;
    label: string;
    large?: boolean;
    tint?: Tint;
  };

  let { value, max = 100, label, large = false, tint }: Props = $props();
</script>

{#if value === undefined}
  <div class={["p-progress-indeterminate", tintClass(tint)]} role="progressbar" aria-label={label}></div>
{:else}
  <progress class={["p-progress", large && "p-progress-large", tintClass(tint)]} {max} {value} aria-label={label}
  ></progress>
{/if}
