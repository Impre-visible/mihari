<script lang="ts">
  import type { Snippet } from "svelte";

  type Props = {
    selected?: boolean;
    title: string;
    subtitle?: string;
    /** Red subtitle, for an error. */
    alert?: boolean;
    onclick?: () => void;
    /** Leading icon or status. */
    leading?: Snippet;
    /** Trailing control or detail. */
    trailing?: Snippet;
  };

  let { selected = false, title, subtitle, alert = false, onclick, leading, trailing }: Props = $props();
</script>

<!-- A row of a SplitView list: a title, a line under it, an icon before. -->
<div
  class={[
    "flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2",
    selected ? "bg-(--p-sys-blue)/15" : onclick && "hover:bg-fill",
  ]}
>
  {#if leading}<div class="flex shrink-0 items-center">{@render leading()}</div>{/if}
  <button type="button" class="min-w-0 flex-1 cursor-default text-start" {onclick} aria-current={selected || undefined} disabled={!onclick}>
    <p class="p-subhead truncate font-medium">{title}</p>
    {#if subtitle}
      <p class={["p-subhead-2 truncate", alert ? "text-(--p-sys-red)" : "text-secondary-label"]}>{subtitle}</p>
    {/if}
  </button>
  {#if trailing}<div class="flex shrink-0 items-center">{@render trailing()}</div>{/if}
</div>
