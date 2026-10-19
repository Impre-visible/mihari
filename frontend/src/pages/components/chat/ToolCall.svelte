<script lang="ts">
  import { CheckCircleIcon, WrenchIcon, XCircleIcon } from "phosphor-svelte";
  import { Spinner } from "$lib/ui";
  import Generic from "./Generic.svelte";
  import { TOOLS } from "./tools";
  import { exitCode, failed, type ToolCall } from "./transcript";
  import type { View } from "./view";

  // live: the job still runs, so a call without a result is under way.
  type Props = { call: ToolCall; live: boolean };
  let { call, live }: Props = $props();

  // Each tool draws its own header and body, from tools/<Name>.svelte.
  let tool = $derived(TOOLS[call.name]);
  let view: View = $derived(tool?.view(call.args) ?? { icon: WrenchIcon, title: call.name, open: false });
  let Body = $derived(tool?.default ?? Generic);

  let output = $derived(call.result?.text ?? "");
  let isError = $derived(failed(call));
  let code = $derived(exitCode(output));
  // Shells say how a command went; a non-zero exit is a failure worth seeing.
  let bad = $derived(isError || (code !== null && code !== 0));
  let pending = $derived(call.result === null);
</script>

<details class="group rounded-lg border border-separator" open={view.open || bad}>
  <summary class="flex cursor-default list-none items-center gap-2 px-3 py-2 [&::-webkit-details-marker]:hidden">
    <view.icon size={15} class="shrink-0 text-secondary-label" aria-hidden="true" />
    <span class="p-subhead-2 shrink-0 font-semibold">{view.title}</span>
    {#if view.detail}
      <span class="p-subhead-2 min-w-0 truncate font-mono text-secondary-label">{view.detail}</span>
    {/if}
    <span class="ms-auto flex shrink-0 items-center gap-1.5">
      {#if pending && live}
        <Spinner small label="En cours" />
      {:else if pending}
        <span class="p-subhead-2 text-tertiary-label">sans résultat</span>
      {:else if bad}
        <XCircleIcon size={15} weight="fill" class="text-(--p-sys-red)" aria-label="Échec" />
      {:else}
        <CheckCircleIcon size={15} weight="fill" class="text-(--p-sys-green)" aria-label="Réussi" />
      {/if}
    </span>
  </summary>

  <div class="flex flex-col gap-2 border-t border-separator px-3 py-2">
    <Body {call} {output} {isError} {pending} />
  </div>
</details>
