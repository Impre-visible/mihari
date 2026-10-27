<script lang="ts">
  import { untrack, type Snippet } from "svelte";

  type Props = {
    /** The list on the leading side; it scrolls on its own. */
    list: Snippet;
    /** Fixed above the list, such as a status or a search field. */
    header?: Snippet;
    /** Fixed under the list, such as an add button. */
    footer?: Snippet;
    /** Width of the list until the user drags the divider, in pixels. */
    width?: number;
    /** Remembers the width the user chose under this name; not kept without it. */
    id?: string;
    /** The selected item. */
    children: Snippet;
    /** The scrolling element of the inspector, bound by a page that follows its end. */
    detail?: HTMLElement | null;
  };

  let { list, header, footer, width = 300, id, children, detail = $bindable(null) }: Props = $props();

  const MIN = 220;
  // The inspector keeps at least this much.
  const MIN_DETAIL = 320;
  let key = $derived(id ? `split:${id}` : null);

  let box: HTMLDivElement;
  // The saved width, read once; the default only until the user drags.
  let current = $state(untrack(() => saved() ?? width));

  // Saved widths are a convenience: storage may be missing or refuse.
  function saved(): number | null {
    try {
      const n = key ? Number(localStorage.getItem(key)) : NaN;
      return Number.isFinite(n) && n >= MIN ? n : null;
    } catch {
      return null;
    }
  }

  function save() {
    try {
      if (key) localStorage.setItem(key, String(Math.round(current)));
    } catch {
      // Kept for this session only.
    }
  }

  function clamp(w: number) {
    const most = Math.max(MIN, (box?.clientWidth ?? 1200) - MIN_DETAIL);
    return Math.min(Math.max(w, MIN), most);
  }

  function drag(e: PointerEvent) {
    const handle = e.currentTarget as HTMLElement;
    handle.setPointerCapture(e.pointerId);
    const startX = e.clientX, startWidth = current;
    const move = (m: PointerEvent) => (current = clamp(startWidth + m.clientX - startX));
    const end = () => {
      handle.removeEventListener("pointermove", move);
      handle.removeEventListener("pointerup", end);
      handle.removeEventListener("pointercancel", end);
      save();
    };
    handle.addEventListener("pointermove", move);
    handle.addEventListener("pointerup", end);
    handle.addEventListener("pointercancel", end);
  }

  function keys(e: KeyboardEvent) {
    const step = e.shiftKey ? 64 : 16;
    if (e.key === "ArrowLeft") current = clamp(current - step);
    else if (e.key === "ArrowRight") current = clamp(current + step);
    else return;
    e.preventDefault();
    save();
  }

  function reset() {
    current = clamp(width);
    save();
  }
</script>

<!-- A list and its inspector filling the window's content, like Mail. The
     negative margins cancel the content padding. The divider between them
     drags; a double click puts it back. -->
<div bind:this={box} class="relative -mx-[24px] -mt-[20px] -mb-[24px] flex h-[calc(100%+44px)]">
  <aside class="flex shrink-0 flex-col border-e border-separator" style:width="{current}px">
    {#if header}<div class="border-b border-separator p-3">{@render header()}</div>{/if}
    <div class="min-h-0 flex-1 overflow-y-auto p-2">{@render list()}</div>
    {#if footer}<div class="border-t border-separator p-2">{@render footer()}</div>{/if}
  </aside>
  <!-- A focusable separator is the ARIA pattern for a splitter: the arrow
       keys move it like the mouse does. -->
  <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
  <div
    role="separator"
    aria-orientation="vertical"
    aria-label="Largeur de la liste"
    aria-valuenow={Math.round(current)}
    aria-valuemin={MIN}
    tabindex="0"
    title="Glisser pour redimensionner, double-clic pour revenir à la largeur d'origine"
    class="absolute top-0 bottom-0 z-10 w-2 -translate-x-1/2 cursor-col-resize touch-none outline-none hover:bg-(--p-sys-blue)/30 focus-visible:bg-(--p-sys-blue)/40"
    style:left="{current}px"
    onpointerdown={drag}
    ondblclick={reset}
    onkeydown={keys}
  ></div>
  <section bind:this={detail} class="min-w-0 flex-1 overflow-y-auto px-6 py-5">{@render children()}</section>
</div>
