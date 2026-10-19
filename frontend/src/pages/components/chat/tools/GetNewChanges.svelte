<script lang="ts" module>
  import { GitDiffIcon } from "phosphor-svelte";
  import type { View } from "../view";

  export function view(): View {
    return { icon: GitDiffIcon, title: "Nouveaux changements", open: false };
  }
</script>

<script lang="ts">
  import Diff from "../Diff.svelte";
  import Output from "../Output.svelte";
  import type { ToolProps } from "../view";

  let { output, isError, pending }: ToolProps = $props();
</script>

{#if isError}<Output {output} {isError} />{:else if !pending}<Diff text={output} />{/if}
