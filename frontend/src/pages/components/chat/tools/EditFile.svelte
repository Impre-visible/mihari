<script lang="ts" module>
  import { PencilLineIcon } from "phosphor-svelte";
  import { str, type View } from "../view";

  export function view(a: Record<string, unknown>): View {
    return { icon: PencilLineIcon, title: "Modification", detail: str(a.path) + (a.replace_all ? " (partout)" : ""), open: true };
  }
</script>

<script lang="ts">
  import Diff from "../Diff.svelte";
  import Note from "../Note.svelte";
  import { marked, type ToolProps } from "../view";

  let { call, output, isError }: ToolProps = $props();
</script>

<Diff text={marked(str(call.args.old_string), "-") + "\n" + marked(str(call.args.new_string), "+")} />
<Note {output} {isError} />
