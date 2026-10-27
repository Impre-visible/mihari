<script lang="ts" module>
  import { SealCheckIcon } from "phosphor-svelte";
  import type { View } from "../view";

  export function view(): View {
    return { icon: SealCheckIcon, title: "Relecture indépendante", open: true };
  }
</script>

<script lang="ts">
  import Comment from "../Comment.svelte";
  import JobChat from "../JobChat.svelte";
  import type { ToolProps } from "../view";

  // Not a tool the model calls: the reviewer that checks the work when the
  // model first calls finish, with its own conversation, then its verdict.
  let { call, output, pending }: ToolProps = $props();
</script>

<details class="rounded-md border border-separator px-3 py-2" open={pending}>
  <summary class="p-subhead-2 cursor-default text-secondary-label">
    Son travail ({call.children.filter((e) => e.kind === "tool").length} appels)
  </summary>
  <JobChat events={call.children} live={pending} />
</details>
{#if !pending}
  <Comment body={output} />
{/if}
