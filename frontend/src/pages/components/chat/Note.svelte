<script lang="ts">
  // A short status line under a tool's body, red when it failed.
  type Props = { output: string; isError: boolean };
  let { output, isError }: Props = $props();
</script>

{#if output}
  <p class={["p-subhead-2 whitespace-pre-wrap select-text", isError ? "text-(--p-sys-red)" : "text-secondary-label"]}>{output}</p>
{/if}
