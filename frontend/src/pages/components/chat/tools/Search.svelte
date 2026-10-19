<script lang="ts" module>
  import { MagnifyingGlassIcon } from "phosphor-svelte";
  import { str, type View } from "../view";

  export function view(a: Record<string, unknown>): View {
    return { icon: MagnifyingGlassIcon, title: "Recherche", detail: str(a.pattern) + (a.glob ? ` dans ${str(a.glob)}` : ""), open: false };
  }
</script>

<script lang="ts">
  import Output from "../Output.svelte";
  import type { ToolProps } from "../view";

  let { output, isError, pending }: ToolProps = $props();
</script>

{#if !pending}<Output {output} {isError} />{/if}
