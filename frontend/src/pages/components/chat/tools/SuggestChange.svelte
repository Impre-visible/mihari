<script lang="ts" module>
  import { ChatCircleTextIcon } from "phosphor-svelte";
  import { range, str, type View } from "../view";

  export function view(a: Record<string, unknown>): View {
    return { icon: ChatCircleTextIcon, title: "Suggestion", detail: str(a.path) + range(a), open: true };
  }
</script>

<script lang="ts">
  import Comment from "../Comment.svelte";
  import Diff from "../Diff.svelte";
  import Note from "../Note.svelte";
  import { marked, type ToolProps } from "../view";

  let { call, output, isError }: ToolProps = $props();
</script>

{#if str(call.args.comment)}<Comment body={str(call.args.comment)} />{/if}
<Diff text={marked(str(call.args.code), "+")} />
<Note {output} {isError} />
