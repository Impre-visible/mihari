<script lang="ts" module>
  import { ScrollIcon } from "phosphor-svelte";
  import { str, type View } from "../view";

  export function view(a: Record<string, unknown>): View {
    return { icon: ScrollIcon, title: "Log CI", detail: `job ${str(a.job_id)}`, open: false };
  }
</script>

<script lang="ts">
  import Output from "../Output.svelte";
  import type { ToolProps } from "../view";

  let { output, isError, pending }: ToolProps = $props();
</script>

{#if !pending}<Output {output} {isError} />{/if}
