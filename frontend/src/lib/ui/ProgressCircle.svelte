<script lang="ts">
  import { tintClass, type Tint } from "./types";

  type Props = {
    /** 0 to 100. */
    value: number;
    label: string;
    small?: boolean;
    tint?: Tint;
  };

  let { value, label, small = false, tint }: Props = $props();
</script>

<div
  class={["p-progress-circle", small && "p-progress-circle-small", tintClass(tint)]}
  style:--p-progress-value={value}
  role="progressbar"
  aria-label={label}
  aria-valuemin={0}
  aria-valuemax={100}
  aria-valuenow={value}
></div>
