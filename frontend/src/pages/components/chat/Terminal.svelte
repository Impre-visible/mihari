<script lang="ts">
  import Fold from "./Fold.svelte";
  import { exitCode, withoutExitCode } from "./transcript";

  // A shell command and what it printed, on a dark background.
  type Props = { command: string; output: string; pending: boolean };
  let { command, output, pending }: Props = $props();

  let code = $derived(exitCode(output));
</script>

<div class="rounded-md bg-black/85 px-3 py-2 text-white/90">
  <pre class="p-subhead-2 font-mono whitespace-pre-wrap select-text"><span class="text-(--p-sys-green)">$</span> {command}</pre>
  {#if !pending}
    <Fold text={withoutExitCode(output)} class="mt-1 text-white/75" />
    {#if code !== null}
      <p class={["p-subhead-2 mt-1 font-mono", code === 0 ? "text-white/50" : "text-(--p-sys-red)"]}>
        code de sortie {code}
      </p>
    {/if}
  {/if}
</div>
