<script lang="ts">
  import { CheckCircleIcon, DownloadSimpleIcon, WarningCircleIcon } from "phosphor-svelte";
  import { Events } from "@wailsio/runtime";
  import { Service as Engine } from "../../../bindings/github.com/Impre-visible/mihari/internal/engine";
  import { Service as Scheduler } from "../../../bindings/github.com/Impre-visible/mihari/internal/scheduler";
  import type { Download, Info, Kind, Presence } from "../../../bindings/github.com/Impre-visible/mihari/internal/engine/models";
  import { Button, ProgressCircle, Spinner, TextField } from "$lib/ui";

  // A profile's model, and whether it is on this machine: downloaded, to
  // download, or downloading.
  type Props = { id: string; engine: Info | undefined; value: string };
  let { id, engine, value = $bindable() }: Props = $props();

  let presence = $state<Presence | null>(null);
  let download = $state<Download | null>(null);
  let model = $derived(value.trim());

  function refresh() {
    presence = null;
    if (!engine || model === "") return;
    Engine.Presence(engine.kind as Kind, model)
      .then((p) => (presence = p))
      .catch((err) => console.error("Model presence:", err));
  }

  $effect(() => {
    void engine?.kind;
    refresh();
    Engine.Downloads().then((list) => (download = (list ?? []).find((d) => d.engine === engine?.kind && d.model === model) ?? null));
  });

  $effect(() =>
    Events.On("model:download", (event) => {
      const d = event.data as Download;
      if (d.engine !== engine?.kind || d.model !== model) return;
      download = d;
      if (d.state === "done") {
        presence = "ready" as Presence;
        // An engine that failed for want of this model can start now.
        Scheduler.Reload();
      }
    }),
  );

  function megabytes(bytes: number) {
    return bytes >= 1e9 ? `${(bytes / 1e9).toFixed(1)} Go` : `${Math.round(bytes / 1e6)} Mo`;
  }
</script>

<div class="flex flex-col items-end gap-1">
  <div class="flex items-center gap-2">
    <TextField
      {id}
      class="w-72 font-mono"
      placeholder={engine?.modelHint ?? ""}
      bind:value
      onchange={refresh}
    />
    <span class="flex w-5 justify-center">
      {#if model === "" || !engine || engine.kind === "opencode"}
        <!-- Nothing to fetch, or served remotely. -->
      {:else if download?.state === "running"}
        <button
          type="button"
          class="cursor-default"
          title="{download.total > 0 ? `${megabytes(download.completed)} / ${megabytes(download.total)}` : 'Téléchargement…'} · cliquer pour annuler"
          onclick={() => Engine.CancelDownload(engine.kind as Kind, model)}
        >
          {#if download.total > 0}
            <ProgressCircle small label="Téléchargement du modèle" value={(download.completed / download.total) * 100} />
          {:else}
            <Spinner small label="Téléchargement du modèle" />
          {/if}
        </button>
      {:else if presence === "ready"}
        <span title="Modèle téléchargé"><CheckCircleIcon size={16} weight="fill" class="text-(--p-sys-green)" aria-label="Modèle téléchargé" /></span>
      {:else if presence === "nofile"}
        <span title="Fichier introuvable sur cette machine">
          <WarningCircleIcon size={16} weight="fill" class="text-(--p-sys-orange)" aria-label="Fichier introuvable" />
        </span>
      {:else}
        <Button
          size="small"
          variant="borderless"
          iconOnly
          aria-label={download?.state === "failed" ? "Réessayer le téléchargement" : "Télécharger le modèle"}
          title={download?.state === "failed" ? download.error : "Télécharger le modèle"}
          disabled={!engine.installed}
          onclick={() => {
            download = null;
            Engine.Download(engine.kind as Kind, model);
          }}
        >
          {#if download?.state === "failed"}
            <WarningCircleIcon size={14} weight="fill" class="text-(--p-sys-red)" aria-hidden="true" />
          {:else}
            <DownloadSimpleIcon size={14} aria-hidden="true" />
          {/if}
        </Button>
      {/if}
    </span>
  </div>
  {#if download?.state === "failed"}
    <p class="p-subhead-2 me-7 text-(--p-sys-red) select-text">{download.error}</p>
  {:else if presence && presence !== "ready" && engine?.kind !== "opencode" && download?.state !== "running"}
    <p class="p-subhead-2 me-7 text-secondary-label">Pas encore sur cette machine : télécharge-le pour que le profil puisse tourner.</p>
  {/if}
</div>
