<script lang="ts" module>
  import { PlayIcon } from "phosphor-svelte";
  import { str, type View } from "../view";

  export function view(a: Record<string, unknown>): View {
    return { icon: PlayIcon, title: "Process lancé", detail: str(a.name) || str(a.command), open: true };
  }
</script>

<script lang="ts">
  import Terminal from "../Terminal.svelte";
  import type { ToolProps } from "../view";

  let { call, output, pending }: ToolProps = $props();
</script>

<Terminal command={`${str(call.args.command)} &`} {output} {pending} />
