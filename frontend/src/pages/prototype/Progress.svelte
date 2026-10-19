<script lang="ts">
  // PROTOTYPE (UI redesign): a plan as a progress bar and the step under way.
  import type { Todo } from "../components/chat/view";

  type Props = { todos: Todo[] };
  let { todos }: Props = $props();
  let done = $derived(todos.filter((t) => t.status === "completed").length);
  let doing = $derived(todos.find((t) => t.status === "in_progress"));
</script>

<div class="flex flex-col gap-1">
  <div class="flex h-1.5 gap-0.5 overflow-hidden rounded-full">
    {#each todos as t, i (i)}
      <div
        class={[
          "flex-1",
          t.status === "completed" ? "bg-(--p-sys-green)" : t.status === "in_progress" ? "animate-pulse bg-(--p-sys-blue)" : "bg-fill",
        ]}
      ></div>
    {/each}
  </div>
  <p class="p-subhead-2 truncate text-secondary-label">
    <span class="font-semibold text-label tabular-nums">{done}/{todos.length}</span>
    {doing ? ` · ${doing.content}` : done === todos.length ? " · plan terminé" : ""}
  </p>
</div>
