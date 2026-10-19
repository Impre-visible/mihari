<script lang="ts" module>
  import { GlobeIcon } from "phosphor-svelte";
  import { str, type View } from "../view";

  export function view(a: Record<string, unknown>): View {
    return { icon: GlobeIcon, title: "Recherche web", detail: str(a.query), open: false };
  }

  type Hit = { title: string; url: string; snippet: string };

  // Reads back web_search's "1. title / url / snippet" blocks.
  function hits(output: string): Hit[] {
    return output
      .split(/\n(?=\d+\. )/)
      .map((block) => {
        const [first = "", url = "", ...rest] = block.split("\n").map((l) => l.trim());
        return { title: first.replace(/^\d+\. /, ""), url, snippet: rest.join(" ") };
      })
      .filter((h) => h.url.startsWith("http"));
  }
</script>

<script lang="ts">
  import { Browser } from "@wailsio/runtime";
  import Output from "../Output.svelte";
  import type { ToolProps } from "../view";

  let { output, isError, pending }: ToolProps = $props();
  let results = $derived(isError ? [] : hits(output));
</script>

{#if results.length}
  <ol class="flex flex-col gap-2">
    {#each results as hit, i (i)}
      <li>
        <button type="button" class="p-subhead-2 text-start font-semibold text-(--p-sys-blue)" onclick={() => Browser.OpenURL(hit.url)}>
          {hit.title}
        </button>
        <p class="p-subhead-2 truncate font-mono text-tertiary-label">{hit.url}</p>
        {#if hit.snippet}<p class="p-subhead-2 text-secondary-label select-text">{hit.snippet}</p>{/if}
      </li>
    {/each}
  </ol>
{:else if !pending}
  <Output {output} {isError} />
{/if}
