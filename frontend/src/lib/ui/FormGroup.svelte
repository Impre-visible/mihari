<script lang="ts">
  import type { Snippet } from "svelte";

  type Props = {
    /** Section heading above the box, like the groups in System Settings. */
    title?: string;
    /** Explanatory footnote under the box. */
    footer?: string;
    class?: string;
    children: Snippet;
  };

  let { title, footer, class: className, children }: Props = $props();
</script>

<section class={["ui-form-section", className]}>
  {#if title}<h2 class="p-box-title">{title}</h2>{/if}
  <div class="p-form-group">
    {@render children()}
  </div>
  {#if footer}<p class="p-sublabel ui-form-footer">{footer}</p>{/if}
</section>
