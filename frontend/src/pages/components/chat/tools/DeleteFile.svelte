<script lang="ts" module>
  import { TrashIcon } from "phosphor-svelte";
  import { str, type View } from "../view";

  export function view(a: Record<string, unknown>): View {
    return { icon: TrashIcon, title: "Suppression", detail: str(a.path), open: true };
  }
</script>

<script lang="ts">
  import Note from "../Note.svelte";
  import type { ToolProps } from "../view";

  let { output, isError }: ToolProps = $props();
</script>

<Note {output} {isError} />
