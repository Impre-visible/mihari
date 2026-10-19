<script lang="ts" module>
  import { GitBranchIcon } from "phosphor-svelte";
  import { type View } from "../view";

  export function view(a: Record<string, unknown>): View {
    return { icon: GitBranchIcon, title: "État git", open: false };
  }
</script>

<script lang="ts">
  import Output from "../Output.svelte";
  import type { ToolProps } from "../view";

  let { output, isError, pending }: ToolProps = $props();
</script>

{#if !pending}<Output {output} {isError} />{/if}
