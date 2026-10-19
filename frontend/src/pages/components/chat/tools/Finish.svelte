<script lang="ts" module>
  import { FlagCheckeredIcon } from "phosphor-svelte";
  import type { View } from "../view";

  export function view(): View {
    return { icon: FlagCheckeredIcon, title: "Fin", open: true };
  }
</script>

<script lang="ts">
  import { str, type ToolProps } from "../view";

  let { call }: ToolProps = $props();
</script>

<p class={["p-subhead-2 select-text", call.args.success === false && "text-(--p-sys-red)"]}>
  {call.args.success === false ? "Abandon : " : ""}{str(call.args.summary)}
</p>
