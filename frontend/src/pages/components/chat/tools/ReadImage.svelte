<script lang="ts" module>
  import { ImageIcon } from "phosphor-svelte";
  import { str, type View } from "../view";

  export function view(a: Record<string, unknown>): View {
    return { icon: ImageIcon, title: "Image", detail: str(a.path) || str(a.url), open: true };
  }
</script>

<script lang="ts">
  import Note from "../Note.svelte";
  import type { ToolProps } from "../view";

  // The image itself stays out of the transcript; a web one shows from its address.
  let { call, output, isError }: ToolProps = $props();
  let url = $derived(str(call.args.url));
</script>

{#if url.startsWith("https://") && !isError}
  <img src={url} alt="Lue par l'agent" class="max-h-64 self-start rounded-md border border-separator" />
{/if}
<Note {output} {isError} />
