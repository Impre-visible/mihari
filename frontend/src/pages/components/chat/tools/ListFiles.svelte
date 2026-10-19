<script lang="ts" module>
  import { FolderOpenIcon } from "phosphor-svelte";
  import { str, type View } from "../view";

  export function view(a: Record<string, unknown>): View {
    return { icon: FolderOpenIcon, title: "Fichiers", detail: str(a.path) || ".", open: false };
  }
</script>

<script lang="ts">
  import Output from "../Output.svelte";
  import type { ToolProps } from "../view";

  let { output, isError, pending }: ToolProps = $props();
</script>

{#if !pending}<Output {output} {isError} />{/if}
