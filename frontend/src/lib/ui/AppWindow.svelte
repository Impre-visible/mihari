<script lang="ts">
  import type { Snippet } from "svelte";

  type Props = {
    title?: string;
    /** Second line under the title, for state such as "Modifié". */
    subtitle?: string;
    /** A full-height `Sidebar`, laid out on the leading edge. */
    sidebar?: Snippet;
    /** Toolbar groups, aligned to the trailing edge of the titlebar. */
    toolbar?: Snippet;
    children: Snippet;
  };

  let { title, subtitle, sidebar, toolbar, children }: Props = $props();

  // macOS grays out the accent, the sidebar icons and the toolbar glass of a
  // window that is not key. The webview gets blur/focus when that changes.
  let focused = $state(document.hasFocus());
</script>

<svelte:window onfocus={() => (focused = true)} onblur={() => (focused = false)} />

<div class={["p-window", "p-window-split", "ui-app-window", !focused && "p-window-inactive"]}>
  {@render sidebar?.()}

  <div class="p-window-main">
    <header class="p-window-titlebar p-window-titlebar-unified" data-drag>
      <div class="p-window-titlebar-row">
        {#if title}
          <div class="p-window-titles">
            <h1 class="p-window-title">{title}</h1>
            {#if subtitle}<span class="p-window-subtitle">{subtitle}</span>{/if}
          </div>
        {/if}
        <div class="p-window-spacer"></div>
        {@render toolbar?.()}
      </div>
    </header>

    <main class="ui-app-content">
      {@render children()}
    </main>
  </div>
</div>
