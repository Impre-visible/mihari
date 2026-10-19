<script lang="ts">
  // A block of monospaced text that folds past a few thousand characters, so
  // a long transcript stays scrollable.
  type Props = { text: string; class?: string; limit?: number };
  let { text, class: className = "", limit = 2000 }: Props = $props();

  let open = $state(false);
  let long = $derived(text.length > limit);
</script>

<pre
  class={["p-subhead-2 overflow-x-auto font-mono whitespace-pre-wrap select-text", className]}>{long && !open
    ? text.slice(0, limit) + "…"
    : text}</pre>
{#if long}
  <button type="button" class="p-subhead-2 mt-1 self-start text-(--p-sys-blue)" onclick={() => (open = !open)}>
    {open ? "Replier" : `Tout afficher (${text.length} caractères)`}
  </button>
{/if}
