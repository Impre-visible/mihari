<script lang="ts">
  import Fold from "./Fold.svelte";

  // A tool's result as text, red when it failed.
  type Props = { output: string; isError: boolean; tone?: string };
  let { output, isError, tone = "text-label" }: Props = $props();
</script>

{#if output}
  <Fold text={output} class={isError ? "text-(--p-sys-red)" : tone} />
{/if}
