<script lang="ts" module>
  import { ArrowBendDownRightIcon } from "phosphor-svelte";
  import type { View } from "../view";

  export function view(): View {
    return { icon: ArrowBendDownRightIcon, title: "Réponse dans un fil", open: true };
  }
</script>

<script lang="ts">
  import Comment from "../Comment.svelte";
  import Note from "../Note.svelte";
  import { str, type ToolProps } from "../view";

  let { call, output, isError }: ToolProps = $props();
</script>

<Comment body={str(call.args.body)} />
<Note {output} {isError} />
