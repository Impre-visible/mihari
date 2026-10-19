<script lang="ts" module>
  import { ClockCounterClockwiseIcon } from "phosphor-svelte";
  import { str, type View } from "../view";

  export function view(a: Record<string, unknown>): View {
    const detail = a.job_id ? `job ${str(a.job_id)}` : str(a.query) || "derniers jobs";
    return { icon: ClockCounterClockwiseIcon, title: "Jobs passés", detail, open: false };
  }
</script>

<script lang="ts">
  import Output from "../Output.svelte";
  import type { ToolProps } from "../view";

  let { output, isError, pending }: ToolProps = $props();
</script>

{#if !pending}<Output {output} {isError} />{/if}
