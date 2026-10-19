<script lang="ts" module>
  import { GitCommitIcon } from "phosphor-svelte";
  import type { View } from "../view";

  export function view(): View {
    return { icon: GitCommitIcon, title: "Push", open: true };
  }
</script>

<script lang="ts">
  import Output from "../Output.svelte";
  import { str, type ToolProps } from "../view";

  let { call, output, isError }: ToolProps = $props();
</script>

<pre class="p-subhead-2 font-mono whitespace-pre-wrap select-text">{str(call.args.message)}</pre>
<Output {output} {isError} tone="text-secondary-label" />
