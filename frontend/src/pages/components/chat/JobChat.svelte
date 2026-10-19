<script lang="ts">
  import { BrainIcon, InfoIcon, RobotIcon, UserIcon, WarningCircleIcon } from "phosphor-svelte";
  import type { Event as JobEvent } from "../../../../bindings/github.com/Impre-visible/mihari/internal/agent/models";
  import Fold from "./Fold.svelte";
  import ToolCall from "./ToolCall.svelte";
  import { toItems } from "./transcript";

  // The job's transcript as a conversation: what was asked and said in the
  // thread on the right, the model's words, reasoning and tool calls on the left.
  type Props = { events: JobEvent[]; live: boolean };
  let { events, live }: Props = $props();

  let items = $derived(toItems(events));

  function time(at: string) {
    return new Date(at).toLocaleTimeString("fr-FR", { hour: "2-digit", minute: "2-digit", second: "2-digit" });
  }
</script>

{#snippet who(icon: typeof UserIcon, name: string, at: string)}
  {@const Icon = icon}
  <div class="flex items-center gap-1.5 text-secondary-label">
    <Icon size={13} aria-hidden="true" />
    <span class="p-subhead-2 font-semibold">{name}</span>
    <span class="p-subhead-2 text-tertiary-label tabular-nums">{time(at)}</span>
  </div>
{/snippet}

<div class="mt-4 flex flex-col gap-3">
  {#each items as item, i (i)}
    {#if item.kind === "call"}
      <ToolCall call={item} {live} />
    {:else}
      {@const e = item.event}
      {#if e.kind === "task" || e.kind === "message"}
        <div class="flex flex-col items-end gap-1 ps-16">
          {@render who(UserIcon, e.kind === "task" ? "Demande" : e.title, e.at)}
          <div class="max-w-full rounded-2xl rounded-tr-sm bg-(--p-sys-blue)/15 px-3 py-2">
            <Fold text={e.text} class="font-sans! text-label" />
          </div>
        </div>
      {:else if e.kind === "assistant"}
        <div class="flex flex-col items-start gap-1 pe-16">
          {@render who(RobotIcon, "Modèle", e.at)}
          <div class="max-w-full rounded-2xl rounded-tl-sm bg-fill px-3 py-2">
            <Fold text={e.text} class="font-sans! text-label" />
          </div>
        </div>
      {:else if e.kind === "reasoning"}
        <details class="pe-16">
          <summary class="flex cursor-default list-none items-center gap-1.5 text-secondary-label [&::-webkit-details-marker]:hidden">
            <BrainIcon size={13} aria-hidden="true" />
            <span class="p-subhead-2 italic">Réflexion</span>
            <span class="p-subhead-2 text-tertiary-label tabular-nums">{time(e.at)}</span>
          </summary>
          <div class="mt-1 border-s-2 border-separator ps-3">
            <Fold text={e.text} class="font-sans! text-secondary-label italic" />
          </div>
        </details>
      {:else if e.kind === "system"}
        <details>
          <summary class="flex cursor-default list-none items-center gap-1.5 text-tertiary-label [&::-webkit-details-marker]:hidden">
            <InfoIcon size={13} aria-hidden="true" />
            <span class="p-subhead-2">Prompt système</span>
          </summary>
          <div class="mt-1 rounded-lg border border-separator px-3 py-2">
            <Fold text={e.text} class="text-secondary-label" />
          </div>
        </details>
      {:else if e.kind === "error"}
        <div class="flex items-start gap-1.5 text-(--p-sys-red)">
          <WarningCircleIcon size={15} weight="fill" class="mt-px shrink-0" aria-hidden="true" />
          <p class="p-subhead-2 select-text"><span class="font-semibold">{e.title}</span> {e.text}</p>
        </div>
      {:else}
        <!-- info, and results whose call was not seen. -->
        <p class="p-subhead-2 text-center text-tertiary-label select-text">
          {e.title}{e.text ? ` · ${e.text}` : ""}
          <span class="tabular-nums">· {time(e.at)}</span>
        </p>
      {/if}
    {/if}
  {:else}
    <p class="p-body py-4 text-center text-tertiary-label">Rien pour l'instant.</p>
  {/each}
</div>
