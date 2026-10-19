<script lang="ts">
  import Fold from "./Fold.svelte";
  import Output from "./Output.svelte";
  import type { ToolProps } from "./view";

  // A tool without its own view: raw arguments, then the result.
  let { call, output, isError, pending }: ToolProps = $props();
</script>

{#if call.raw && call.raw !== "{}"}
  <Fold text={JSON.stringify(call.args, null, 2)} class="text-secondary-label" />
{/if}
{#if !pending}<Output {output} {isError} />{/if}
