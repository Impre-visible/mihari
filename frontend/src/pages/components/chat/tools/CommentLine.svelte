<script lang="ts" module>
  import { ChatCircleTextIcon } from "phosphor-svelte";
  import { str, type View } from "../view";

  export function view(a: Record<string, unknown>): View {
    return { icon: ChatCircleTextIcon, title: "Commentaire", detail: `${str(a.path)}:${str(a.line)}`, open: true };
  }
</script>

<script lang="ts">
  import Comment from "../Comment.svelte";
  import Note from "../Note.svelte";
  import type { ToolProps } from "../view";

  let { call, output, isError }: ToolProps = $props();
</script>

<Comment body={str(call.args.body)} />
<Note {output} {isError} />
