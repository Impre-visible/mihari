<script lang="ts" module>
  import { BandaidsIcon } from "phosphor-svelte";
  import type { View } from "../view";

  export function view(a: Record<string, unknown>): View {
    const files = typeof a.patch === "string" ? (a.patch.match(/^\+\+\+ /gm) ?? []).length : 0;
    return { icon: BandaidsIcon, title: "Patch", detail: files ? `${files} fichier${files > 1 ? "s" : ""}` : undefined, open: true };
  }
</script>

<script lang="ts">
  import Diff from "../Diff.svelte";
  import Note from "../Note.svelte";
  import { str, type ToolProps } from "../view";

  let { call, output, isError }: ToolProps = $props();
</script>

<Diff text={str(call.args.patch)} />
<Note {output} {isError} />
