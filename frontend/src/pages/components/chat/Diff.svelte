<script lang="ts">
  // A unified diff with added and removed lines in colour. The "=== path"
  // headers get_diff writes between files stand out as titles.
  type Props = { text: string; limit?: number };
  let { text, limit = 400 }: Props = $props();

  let open = $state(false);
  let lines = $derived(text.split("\n"));
  let shown = $derived(open ? lines : lines.slice(0, limit));

  function tone(line: string) {
    if (line.startsWith("=== ")) return "mt-2 font-semibold text-label first:mt-0";
    if (line.startsWith("@@")) return "text-(--p-sys-blue)";
    if (line.startsWith("+")) return "bg-(--p-sys-green)/15 text-label";
    if (line.startsWith("-")) return "bg-(--p-sys-red)/15 text-label";
    return "text-secondary-label";
  }
</script>

<div class="p-subhead-2 overflow-x-auto font-mono select-text">
  {#each shown as line, i (i)}
    <div class={["px-1 whitespace-pre", tone(line)]}>{line || " "}</div>
  {/each}
</div>
{#if lines.length > limit}
  <button type="button" class="p-subhead-2 mt-1 self-start text-(--p-sys-blue)" onclick={() => (open = !open)}>
    {open ? "Replier" : `Tout afficher (${lines.length} lignes)`}
  </button>
{/if}
