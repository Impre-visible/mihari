<script lang="ts" module>
  import { TreeStructureIcon } from "phosphor-svelte";
  import { str, type View } from "../view";

  export function view(a: Record<string, unknown>): View {
    return { icon: TreeStructureIcon, title: "Sous-agent", detail: str(a.task), open: true };
  }
</script>

<script lang="ts">
  import Comment from "../Comment.svelte";
  import JobChat from "../JobChat.svelte";
  import Note from "../Note.svelte";
  import type { ToolProps } from "../view";

  // The subagent's own conversation, folded inside its call, then its conclusion.
  let { call, output, isError, pending }: ToolProps = $props();
</script>

<details class="rounded-md border border-separator px-3 py-2" open={pending}>
  <summary class="p-subhead-2 cursor-default text-secondary-label">
    Son travail ({call.children.filter((e) => e.kind === "tool").length} appels)
  </summary>
  <JobChat events={call.children} live={pending} />
</details>
{#if !pending}
  {#if isError}<Note {output} {isError} />{:else}<Comment body={output} />{/if}
{/if}
