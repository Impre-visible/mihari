<script lang="ts" module>
  import { ArrowsLeftRightIcon } from "phosphor-svelte";
  import { str, type View } from "../view";

  export function view(a: Record<string, unknown>): View {
    return { icon: ArrowsLeftRightIcon, title: "Déplacement", detail: `${str(a.from)} → ${str(a.to)}`, open: true };
  }
</script>

<script lang="ts">
  import Note from "../Note.svelte";
  import type { ToolProps } from "../view";

  let { output, isError }: ToolProps = $props();
</script>

<Note {output} {isError} />
