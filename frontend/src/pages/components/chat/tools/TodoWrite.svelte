<script lang="ts" module>
  import { ListChecksIcon } from "phosphor-svelte";
  import { todos, type View } from "../view";

  export function view(a: Record<string, unknown>): View {
    const list = todos(a);
    const done = list.filter((t) => t.status === "completed").length;
    return { icon: ListChecksIcon, title: "Plan", detail: `${done}/${list.length}`, open: true };
  }
</script>

<script lang="ts">
  import Note from "../Note.svelte";
  import Plan from "../Plan.svelte";
  import type { ToolProps } from "../view";

  let { call, output, isError }: ToolProps = $props();
</script>

<Plan todos={todos(call.args)} />
{#if isError || output.includes("pas affiché")}<Note {output} {isError} />{/if}
