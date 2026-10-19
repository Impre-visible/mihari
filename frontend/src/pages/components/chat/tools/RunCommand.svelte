<script lang="ts" module>
  import { TerminalWindowIcon } from "phosphor-svelte";
  import { str, type View } from "../view";

  export function view(a: Record<string, unknown>): View {
    return { icon: TerminalWindowIcon, title: "Commande", detail: str(a.command), open: true };
  }
</script>

<script lang="ts">
  import Terminal from "../Terminal.svelte";
  import type { ToolProps } from "../view";

  let { call, output, pending }: ToolProps = $props();
</script>

<Terminal command={str(call.args.command)} {output} {pending} />
