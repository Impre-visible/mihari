<script lang="ts" module>
  import { NotebookIcon } from "phosphor-svelte";
  import type { View } from "../view";

  export function view(): View {
    return { icon: NotebookIcon, title: "Note pour le projet", open: true };
  }
</script>

<script lang="ts">
  import Note from "../Note.svelte";
  import { str, type ToolProps } from "../view";

  let { call, output, isError }: ToolProps = $props();
</script>

<p class="p-subhead-2 select-text">{str(call.args.note)}</p>
{#if isError}<Note {output} {isError} />{/if}
