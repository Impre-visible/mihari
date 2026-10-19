<script lang="ts" module>
  import { GitPullRequestIcon } from "phosphor-svelte";
  import { str, type View } from "../view";

  export function view(a: Record<string, unknown>): View {
    return { icon: GitPullRequestIcon, title: "MR ouverte", detail: str(a.title), open: true };
  }
</script>

<script lang="ts">
  import Comment from "../Comment.svelte";
  import Output from "../Output.svelte";
  import type { ToolProps } from "../view";

  let { call, output, isError }: ToolProps = $props();
</script>

<Comment body={str(call.args.description)} />
<Output {output} {isError} tone="text-secondary-label" />
