<script lang="ts" module>
  import { FilesIcon } from "phosphor-svelte";
  import { str, type View } from "../view";

  export function view(a: Record<string, unknown>): View {
    return { icon: FilesIcon, title: "Fichiers par motif", detail: (a.path ? str(a.path) + "/" : "") + str(a.pattern), open: false };
  }
</script>

<script lang="ts">
  import Output from "../Output.svelte";
  import type { ToolProps } from "../view";

  let { output, isError, pending }: ToolProps = $props();
</script>

{#if !pending}<Output {output} {isError} />{/if}
