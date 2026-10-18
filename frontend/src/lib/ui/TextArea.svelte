<script lang="ts">
  import type { HTMLTextareaAttributes } from "svelte/elements";

  type Props = HTMLTextareaAttributes & { value?: string };

  let { value = $bindable(""), class: className, ...rest }: Props = $props();
</script>

<textarea class={["p-field", className]} bind:value spellcheck="false" {...rest}></textarea>
