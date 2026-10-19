<script lang="ts" module>
  import { QuestionIcon } from "phosphor-svelte";
  import type { View } from "../view";

  export function view(): View {
    return { icon: QuestionIcon, title: "Question", open: true };
  }
</script>

<script lang="ts">
  import Comment from "../Comment.svelte";
  import Note from "../Note.svelte";
  import { str, type ToolProps } from "../view";

  let { call, output, isError, pending }: ToolProps = $props();
  let choices = $derived(Array.isArray(call.args.choices) ? call.args.choices.map(str) : []);
</script>

<Comment body={str(call.args.question) + choices.map((c, i) => `\n${i + 1}. ${c}`).join("")} />
{#if pending}
  <p class="p-subhead-2 text-tertiary-label">En attente de réponse (30 minutes au plus)…</p>
{:else if isError}
  <Note {output} {isError} />
{:else}
  <div class="flex flex-col items-end ps-16">
    <div class="max-w-full rounded-2xl rounded-tr-sm bg-(--p-sys-blue)/15 px-3 py-2">
      <p class="p-subhead-2 whitespace-pre-wrap select-text">{output}</p>
    </div>
  </div>
{/if}
