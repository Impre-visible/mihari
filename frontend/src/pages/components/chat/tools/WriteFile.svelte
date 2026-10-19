<script lang="ts" module>
  import { PencilSimpleIcon } from "phosphor-svelte";
  import { str, type View } from "../view";

  export function view(a: Record<string, unknown>): View {
    return { icon: PencilSimpleIcon, title: "Écriture", detail: str(a.path), open: true };
  }
</script>

<script lang="ts">
  import Fold from "../Fold.svelte";
  import Note from "../Note.svelte";
  import { numbered, type ToolProps } from "../view";

  let { call, output, isError }: ToolProps = $props();
</script>

<Fold text={numbered(str(call.args.content))} class="text-label" />
{#if isError}<Note {output} {isError} />{/if}
