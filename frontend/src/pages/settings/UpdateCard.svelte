<script lang="ts">
  import { ArrowCircleUpIcon, WarningCircleIcon } from "phosphor-svelte";
  import { Events } from "@wailsio/runtime";
  import { Service as Updates } from "../../../bindings/github.com/Impre-visible/mihari/internal/updates";
  import type { Info } from "../../../bindings/github.com/Impre-visible/mihari/internal/updates/models";
  import { Button, ProgressBar } from "$lib/ui";

  let info = $state<Info | null>(null);
  // "Plus tard" hides this version until Mihari restarts.
  let dismissed = $state<string | null>(null);

  Updates.Status().then((i) => (info = i));
  $effect(() => Events.On("updates:changed", (event) => (info = event.data)));

  // What the update button will do, by how this copy was installed.
  const NOTES: Record<string, string> = {
    inplace: "Mihari redémarre après l'installation.",
    appimage: "L'AppImage est remplacée, puis Mihari redémarre.",
    pacman: "Ton mot de passe sera demandé pour installer le paquet, puis Mihari redémarre.",
    apt: "Ton mot de passe sera demandé pour installer le paquet, puis Mihari redémarre.",
    manual: "La page de téléchargement va s'ouvrir.",
  };

  let shown = $derived(info?.available && info.version !== dismissed);
</script>

<!-- In the sidebar, under the sections, like the update card of System Settings. -->
{#if info && shown}
  <div class="mx-2.5 mt-auto mb-2.5 rounded-lg bg-(--p-box-fill) ring-1 ring-(--p-separator,transparent) p-2.5">
    <div class="flex items-center gap-1.5">
      <ArrowCircleUpIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-blue)" aria-hidden="true" />
      <span class="p-subhead-2 font-semibold">Mihari {info.version}</span>
    </div>
    {#if info.installing}
      <div class="mt-2">
        <ProgressBar
          label="Téléchargement de la mise à jour"
          value={info.total > 0 ? info.downloaded : undefined}
          max={info.total || 1}
        />
      </div>
    {:else}
      {#if info.error}
        <p class="p-caption-1 mt-1 flex items-start gap-1 text-label select-text">
          <WarningCircleIcon size={12} weight="fill" class="mt-0.5 shrink-0 text-(--p-sys-red)" aria-hidden="true" />
          {info.error}
        </p>
      {:else}
        <p class="p-caption-1 mt-1 text-secondary-label">{NOTES[info.method] ?? NOTES.inplace}</p>
      {/if}
      <div class="mt-2 flex justify-end gap-1">
        <Button size="small" variant="borderless" onclick={() => (dismissed = info?.version ?? null)}>Plus tard</Button>
        <Button size="small" variant="primary" onclick={() => Updates.Install()}>
          {info.method === "manual" ? "Télécharger" : "Installer"}
        </Button>
      </div>
    {/if}
  </div>
{/if}
