<script lang="ts">
  import type { Snippet } from "svelte";

  type Props = { title?: string; class?: string; children: Snippet };

  let { title, class: className, children }: Props = $props();
</script>

<section class={className}>
  {#if title}<h2 class="p-box-title">{title}</h2>{/if}
  <div class="p-box">
    {@render children()}
  </div>
</section>
