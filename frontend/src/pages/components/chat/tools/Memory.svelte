<script lang="ts" module>
  import { NotebookIcon } from "phosphor-svelte";
  import type { View } from "../view";

  export function view(a: Record<string, unknown>): View {
    const ops = Array.isArray(a.operations) ? a.operations.length : 0;
    return {
      icon: NotebookIcon,
      title: "Notes du projet",
      detail: ops ? `${ops} changement${ops > 1 ? "s" : ""}` : "relecture",
      open: ops > 0,
    };
  }
</script>

<script lang="ts">
  import Diff from "../Diff.svelte";
  import Output from "../Output.svelte";
  import { str, type ToolProps } from "../view";

  let { call, output, isError, pending }: ToolProps = $props();

  type Op = { action?: string; content?: string; old_text?: string };
  let ops = $derived((Array.isArray(call.args.operations) ? call.args.operations : []) as Op[]);
  // Each change as a diff line: what goes, what comes.
  let changes = $derived(
    ops
      .map((o) => {
        const gone = o.old_text ? `-${str(o.old_text)}` : "";
        const added = o.content ? `+${str(o.content)}` : "";
        return o.action === "add" ? added : o.action === "remove" ? gone : [gone, added].filter(Boolean).join("\n");
      })
      .join("\n"),
  );
</script>

{#if changes}<Diff text={changes} />{/if}
{#if !pending}<Output {output} {isError} tone="text-secondary-label" />{/if}
