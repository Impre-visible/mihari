<script lang="ts">
  import { CheckCircleIcon, ClockIcon, ProhibitIcon, XCircleIcon } from "phosphor-svelte";
  import { Spinner } from "$lib/ui";

  type Props = { state: string };
  let { state }: Props = $props();
</script>

{#if state === "running"}
  <Spinner small label="En cours" />
{:else if state === "queued"}
  <ClockIcon size={16} class="shrink-0 text-secondary-label" aria-label="En attente" />
{:else if state === "succeeded"}
  <CheckCircleIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-green)" aria-label="Terminé" />
{:else if state === "failed"}
  <XCircleIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-red)" aria-label="Échec" />
{:else}
  <ProhibitIcon size={16} class="shrink-0 text-secondary-label" aria-label="Annulé" />
{/if}
