<script lang="ts">
  import type { Snippet } from "svelte";

  type Props = { title: string; children?: Snippet };
  let { title, children }: Props = $props();
</script>

<!-- A group title in a SplitView list, with room for a small action. -->
<div class="flex items-center gap-1 px-2.5 pt-3 pb-1 first:pt-1">
  <p class="p-caption-1 flex-1 font-semibold tracking-wide text-secondary-label uppercase">{title}</p>
  {@render children?.()}
</div>
