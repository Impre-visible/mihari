<script lang="ts" module>
  import { CodeIcon } from "phosphor-svelte";
  import { str, type View } from "../view";

  export function view(a: Record<string, unknown>): View {
    return { icon: CodeIcon, title: "Script", detail: str(a.language) || "python", open: true };
  }
</script>

<script lang="ts">
  import Fold from "../Fold.svelte";
  import Terminal from "../Terminal.svelte";
  import { numbered, type ToolProps } from "../view";

  let { call, output, pending }: ToolProps = $props();
</script>

<Fold text={numbered(str(call.args.code))} class="text-label" />
<Terminal command={`${str(call.args.language) || "python"} script`} {output} {pending} />
