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

{#if info && shown}
  <div class="mb-4 flex items-center gap-3 rounded-xl px-4 py-3 ring-1 ring-(--p-sys-blue)">
    <ArrowCircleUpIcon size={20} weight="fill" class="shrink-0 text-(--p-sys-blue)" aria-hidden="true" />
    <div class="flex min-w-0 flex-1 flex-col">
      <span>Mihari {info.version} est disponible</span>
      {#if info.installing}
        <div class="mt-1.5">
          <ProgressBar
            label="Téléchargement de la mise à jour"
            value={info.total > 0 ? info.downloaded : undefined}
            max={info.total || 1}
          />
        </div>
      {:else if info.error}
        <span class="p-subhead-2 flex items-center gap-1 text-label">
          <WarningCircleIcon size={14} weight="fill" class="shrink-0 text-(--p-sys-red)" aria-hidden="true" />
          {info.error}
        </span>
      {:else}
        <span class="p-subhead-2 text-secondary-label">
          Version actuelle {info.current}. {NOTES[info.method] ?? NOTES.inplace}
        </span>
      {/if}
    </div>
    {#if !info.installing}
      <Button size="small" variant="borderless" onclick={() => (dismissed = info?.version ?? null)}>Plus tard</Button>
      <Button size="small" variant="primary" onclick={() => Updates.Install()}>
        {info.method === "manual" ? "Télécharger" : "Mettre à jour"}
      </Button>
    {/if}
  </div>
{/if}
