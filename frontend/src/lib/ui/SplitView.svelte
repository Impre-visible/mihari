<script lang="ts">
  import type { Snippet } from "svelte";

  type Props = {
    /** The list on the leading side; it scrolls on its own. */
    list: Snippet;
    /** Fixed above the list, such as a status or a search field. */
    header?: Snippet;
    /** Fixed under the list, such as an add button. */
    footer?: Snippet;
    /** Width of the list, in pixels. */
    width?: number;
    /** The selected item. */
    children: Snippet;
    /** The scrolling element of the inspector, bound by a page that follows its end. */
    detail?: HTMLElement | null;
  };

  let { list, header, footer, width = 300, children, detail = $bindable(null) }: Props = $props();
</script>

<!-- A list and its inspector filling the window's content, like Mail. The
     negative margins cancel the content padding. -->
<div class="-mx-[24px] -mt-[20px] -mb-[24px] flex h-[calc(100%+44px)]">
  <aside class="flex shrink-0 flex-col border-e border-separator" style:width="{width}px">
    {#if header}<div class="border-b border-separator p-3">{@render header()}</div>{/if}
    <div class="min-h-0 flex-1 overflow-y-auto p-2">{@render list()}</div>
    {#if footer}<div class="border-t border-separator p-2">{@render footer()}</div>{/if}
  </aside>
  <section bind:this={detail} class="min-w-0 flex-1 overflow-y-auto px-6 py-5">{@render children()}</section>
</div>
