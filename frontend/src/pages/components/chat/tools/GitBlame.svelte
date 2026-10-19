<script lang="ts" module>
  import { UserListIcon } from "phosphor-svelte";
  import { range, str, type View } from "../view";

  export function view(a: Record<string, unknown>): View {
    return { icon: UserListIcon, title: "Blame", detail: str(a.path) + range(a), open: false };
  }
</script>

<script lang="ts">
  import Output from "../Output.svelte";
  import type { ToolProps } from "../view";

  let { output, isError, pending }: ToolProps = $props();
</script>

{#if !pending}<Output {output} {isError} />{/if}
