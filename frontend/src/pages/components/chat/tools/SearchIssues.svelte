<script lang="ts" module>
  import { MagnifyingGlassIcon } from "phosphor-svelte";
  import { str, type View } from "../view";

  export function view(a: Record<string, unknown>): View {
    return { icon: MagnifyingGlassIcon, title: "Recherche GitLab", detail: str(a.query), open: false };
  }
</script>

<script lang="ts">
  import Output from "../Output.svelte";
  import type { ToolProps } from "../view";

  let { output, isError, pending }: ToolProps = $props();
</script>

{#if !pending}<Output {output} {isError} />{/if}
