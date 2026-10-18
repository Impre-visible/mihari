<script lang="ts">
  import type { Snippet } from "svelte";

  type Props = { title: string; children: Snippet };

  let { title, children }: Props = $props();
</script>

<div class="p-sidebar-header">{title}</div>
{@render children()}
