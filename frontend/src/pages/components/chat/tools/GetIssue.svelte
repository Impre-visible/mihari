<script lang="ts" module>
  import { ListBulletsIcon } from "phosphor-svelte";
  import { str, type View } from "../view";

  export function view(a: Record<string, unknown>): View {
    return { icon: ListBulletsIcon, title: "Lecture de l'issue", detail: a.iid ? `#${str(a.iid)}` : undefined, open: false };
  }
</script>

<script lang="ts">
  import Output from "../Output.svelte";
  import type { ToolProps } from "../view";

  let { output, isError, pending }: ToolProps = $props();
</script>

{#if !pending}<Output {output} {isError} />{/if}
