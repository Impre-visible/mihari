<script lang="ts">
  type Props = { label?: string; small?: boolean };

  let { label = "Chargement", small = false }: Props = $props();
</script>

<span class={["p-spinner", small && "p-spinner-small"]} role="progressbar" aria-label={label}></span>
