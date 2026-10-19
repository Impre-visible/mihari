<script lang="ts">
  import { CheckCircleIcon, CircleIcon, HourglassMediumIcon } from "phosphor-svelte";
  import type { Todo } from "./view";

  // The agent's plan as a checklist, the way todo_write left it.
  type Props = { todos: Todo[] };
  let { todos }: Props = $props();
</script>

<ul class="flex flex-col gap-1">
  {#each todos as todo, i (i)}
    <li class="flex items-start gap-2">
      {#if todo.status === "completed"}
        <CheckCircleIcon size={15} weight="fill" class="mt-px shrink-0 text-(--p-sys-green)" aria-label="Fait" />
      {:else if todo.status === "in_progress"}
        <HourglassMediumIcon size={15} weight="fill" class="mt-px shrink-0 text-(--p-sys-blue)" aria-label="En cours" />
      {:else}
        <CircleIcon size={15} class="mt-px shrink-0 text-tertiary-label" aria-label="À faire" />
      {/if}
      <span
        class={[
          "p-subhead-2 select-text",
          todo.status === "completed" && "text-secondary-label line-through",
          todo.status === "in_progress" && "font-semibold",
        ]}>{todo.content}</span
      >
    </li>
  {/each}
</ul>
