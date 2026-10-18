<script lang="ts">
  import type { Snippet } from "svelte";

  type Props = {
    label: string;
    /** Id of the control in the row, so clicking the label focuses it. */
    for?: string;
    /** Secondary value text at the trailing edge. */
    detail?: string;
    /** Shows a disclosure chevron, for a row that opens more detail. */
    chevron?: boolean;
    onclick?: () => void;
    children?: Snippet;
  };

  let { label, for: htmlFor, detail, chevron = false, onclick, children }: Props = $props();
</script>

{#snippet content()}
  {#if htmlFor}
    <label class="p-form-row-label" for={htmlFor}>{label}</label>
  {:else}
    <span class="p-form-row-label">{label}</span>
  {/if}
  {#if detail}<span class="p-form-row-detail">{detail}</span>{/if}
  {@render children?.()}
  {#if chevron}<span class="p-form-row-chevron" aria-hidden="true"></span>{/if}
{/snippet}

{#if onclick}
  <button type="button" class="p-form-row ui-form-row-button" {onclick}>{@render content()}</button>
{:else}
  <div class="p-form-row">{@render content()}</div>
{/if}
