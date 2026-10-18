<script lang="ts">
  import { CheckCircleIcon, DownloadSimpleIcon, PlusIcon, TrashIcon, WarningCircleIcon } from "phosphor-svelte";
  import { Events } from "@wailsio/runtime";
  import { Service as Settings } from "../../../bindings/github.com/Impre-visible/mihari/internal/settings";
  import { Service as Scheduler } from "../../../bindings/github.com/Impre-visible/mihari/internal/scheduler";
  import { Service as Engine } from "../../../bindings/github.com/Impre-visible/mihari/internal/engine";
  import type {
    Download,
    Info as EngineInfo,
    Kind,
    Presence,
  } from "../../../bindings/github.com/Impre-visible/mihari/internal/engine/models";
  import {
    Button,
    FormGroup,
    FormRow,
    ProgressCircle,
    SegmentedControl,
    Select,
    Spinner,
    Stepper,
    TextArea,
    TextField,
  } from "$lib/ui";
  import { PROMPT_PRESETS } from "./prompts";

  type Status =
    | { kind: "idle" }
    | { kind: "busy"; label: string }
    | { kind: "ok"; message: string }
    | { kind: "error"; message: string };

  // The schedule refers to profiles by id, so renaming one keeps its segments.
  type Trigger = "ping" | "issues";
  type Profile = {
    id: string;
    name: string;
    prompt: string;
    engine: string;
    model: string;
    contextPerJob: number;
    maxJobs: number;
    trigger: Trigger;
    label: string;
    extraArgs: string;
    maxTurns: number;
    maxMinutes: number;
  };

  const TRIGGERS: { value: Trigger; label: string }[] = [
    { value: "ping", label: "Ping" },
    { value: "issues", label: "Issues" },
  ];

  let profiles = $state<Profile[]>([]);
  let engines = $state<EngineInfo[]>([]);
  let engineOptions = $derived(
    engines.map((e) => ({
      value: e.kind as string,
      label: e.supported ? (e.installed ? e.label : `${e.label} (non installé)`) : `${e.label} (indisponible ici)`,
      disabled: !e.supported,
    })),
  );
  let status = $state<Status>({ kind: "idle" });

  let busy = $derived(status.kind === "busy");
  let problem = $derived(validate(profiles));

  const presetOptions = [
    { value: "", label: "Charger un prompt prêt…", disabled: true },
    ...PROMPT_PRESETS.map((preset) => ({ value: preset.id, label: preset.label })),
  ];

  function newId() {
    return Date.now().toString(36) + Math.random().toString(36).slice(2, 8);
  }

  function blank(): Profile {
    return {
      id: newId(),
      name: "",
      prompt: "",
      engine: engines.find((e) => e.supported && e.installed)?.kind ?? "llama-server",
      model: "",
      contextPerJob: 32768,
      maxJobs: 1,
      trigger: "ping",
      label: "",
      extraArgs: "",
      maxTurns: 50,
      maxMinutes: 30,
    };
  }

  // Profiles saved before a field existed get its default, as in internal/profiles.
  function parse(value: unknown): Profile | null {
    if (typeof value !== "object" || value === null) return null;
    const v = value as Record<string, unknown>;
    if (typeof v.id !== "string" || typeof v.name !== "string") return null;
    const text = (x: unknown) => (typeof x === "string" ? x : "");
    const count = (x: unknown, fallback: number) => (Number.isInteger(x) && (x as number) > 0 ? (x as number) : fallback);
    return {
      id: v.id,
      name: v.name,
      prompt: text(v.prompt),
      engine: text(v.engine) || "llama-server",
      model: text(v.model),
      contextPerJob: count(v.contextPerJob, 32768),
      maxJobs: count(v.maxJobs, 1),
      trigger: v.trigger === "issues" ? "issues" : "ping",
      label: text(v.label),
      extraArgs: text(v.extraArgs),
      maxTurns: count(v.maxTurns, 50),
      maxMinutes: count(v.maxMinutes, 30),
    };
  }

  Promise.all([Settings.GetConfig(), Scheduler.Engines()])
    .then(([config, catalog]) => {
      engines = catalog ?? [];
      const stored = Array.isArray(config?.profiles) ? config.profiles : [];
      profiles = stored.map(parse).filter((p): p is Profile => p !== null);
      profiles.forEach(refreshPresence);
    })
    .catch((err) => console.error("Load profiles:", err));

  function engineOf(p: Profile) {
    return engines.find((e) => e.kind === p.engine);
  }

  // Keyed by engine and model, since two profiles can share a model.
  let presences = $state<Record<string, Presence>>({});
  let downloads = $state<Record<string, Download>>({});
  let downloadError = $state<string | null>(null);

  function modelKey(engine: string, model: string) {
    return `${engine}|${model.trim()}`;
  }

  function refreshPresence(p: Profile) {
    const key = modelKey(p.engine, p.model);
    if (p.model.trim() === "") return;
    Engine.Presence(p.engine as Kind, p.model.trim())
      .then((presence) => (presences[key] = presence))
      .catch((err) => console.error("Model presence:", err));
  }

  $effect(() => {
    Engine.Downloads().then((list) => {
      for (const d of list ?? []) downloads[modelKey(d.engine, d.model)] = d;
    });
    return Events.On("model:download", (event) => {
      const d = event.data;
      const key = modelKey(d.engine, d.model);
      downloads[key] = d;
      if (d.state === "failed") downloadError = d.error;
      if (d.state === "done") {
        presences[key] = "ready" as Presence;
        // An engine that failed for want of this model can start now.
        Scheduler.Reload();
      }
    });
  });

  function startDownload(p: Profile) {
    delete downloads[modelKey(p.engine, p.model)];
    downloadError = null;
    Engine.Download(p.engine as Kind, p.model.trim());
  }

  function megabytes(bytes: number) {
    return bytes >= 1e9 ? `${(bytes / 1e9).toFixed(1)} Go` : `${Math.round(bytes / 1e6)} Mo`;
  }

  function validate(list: Profile[]): string | null {
    const names = new Set<string>();
    for (const p of list) {
      const name = p.name.trim();
      if (name === "") return "Chaque profil a besoin d'un nom.";
      if (names.has(name.toLowerCase())) return `Deux profils s'appellent ${name}.`;
      names.add(name.toLowerCase());
      if (p.model.trim() === "") return `${name} : aucun modèle renseigné.`;
      if (p.prompt.trim() === "") return `${name} : le prompt est vide.`;
      if (!Number.isInteger(p.maxJobs) || p.maxJobs < 1) return `${name} : il faut au moins 1 job en parallèle.`;
      if (!Number.isInteger(p.maxTurns) || p.maxTurns < 1) return `${name} : il faut au moins 1 tour par job.`;
      if (!Number.isInteger(p.maxMinutes) || p.maxMinutes < 1) return `${name} : il faut au moins 1 minute par job.`;
      if (!Number.isInteger(p.contextPerJob) || p.contextPerJob < 4096) return `${name} : contexte trop petit.`;
      if (p.trigger === "issues" && p.label.trim() === "") return `${name} : indique le label des issues à traiter.`;
    }
    return null;
  }

  // The menu is an action, not a setting, so it snaps back to its placeholder once used.
  function loadPreset(p: Profile, select: HTMLSelectElement) {
    const preset = PROMPT_PRESETS.find((candidate) => candidate.id === select.value);
    select.value = "";
    if (!preset) return;
    p.prompt = preset.prompt;
    resetStatus();
  }

  function add() {
    profiles.push(blank());
    resetStatus();
  }

  function remove(p: Profile) {
    profiles = profiles.filter((other) => other !== p);
    resetStatus();
  }

  async function save() {
    status = { kind: "busy", label: "Enregistrement…" };
    try {
      await Settings.Set(
        "profiles",
        profiles.map((p) => ({
          ...p,
          name: p.name.trim(),
          model: p.model.trim(),
          label: p.trigger === "issues" ? p.label.trim() : "",
          extraArgs: p.extraArgs.trim(),
        })),
      );
      // The scheduler restarts the engine if the active profile changed.
      Scheduler.Reload();
      status = { kind: "ok", message: "Profils enregistrés." };
    } catch (err) {
      status = { kind: "error", message: err instanceof Error ? err.message : String(err) };
    }
  }

  function resetStatus() {
    if (status.kind !== "busy") status = { kind: "idle" };
  }
</script>

<p class="p-body text-secondary-label">Un moteur, un modèle et le prompt qu'il reçoit. Le planning dit quand chacun tourne.</p>

<form
  class="mt-6"
  oninput={resetStatus}
  onsubmit={(e) => {
    e.preventDefault();
    if (!problem) save();
  }}
>
  {#each profiles as p (p.id)}
    <FormGroup>
      <FormRow label="Nom" for="profile-{p.id}-name">
        <div class="flex items-center gap-2">
          <TextField id="profile-{p.id}-name" class="w-72" placeholder="Review du jour" bind:value={p.name} />
          <Button
            size="small"
            variant="borderless"
            iconOnly
            aria-label="Supprimer {p.name.trim() || 'le profil'}"
            onclick={() => remove(p)}
          >
            <TrashIcon size={14} aria-hidden="true" />
          </Button>
        </div>
      </FormRow>
      <FormRow label="Moteur" for="profile-{p.id}-engine">
        <Select
          id="profile-{p.id}-engine"
          class="me-7"
          options={engineOptions}
          bind:value={p.engine}
          onchange={() => {
            resetStatus();
            refreshPresence(p);
          }}
        />
      </FormRow>
      {#if engineOf(p) && !engineOf(p)?.installed}
        <div class="p-form-row py-2.5">
          <WarningCircleIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-orange)" aria-hidden="true" />
          <p class="p-subhead-2 min-w-0 flex-1 text-secondary-label">
            {engineOf(p)?.label} n'est pas installé sur cette machine. Pour l'installer :
            <code class="font-mono text-label select-text">{engineOf(p)?.install}</code>
          </p>
        </div>
      {/if}
      <FormRow label="Modèle" for="profile-{p.id}-model">
        {@const key = modelKey(p.engine, p.model)}
        {@const download = downloads[key]}
        <div class="flex items-center gap-2">
          <TextField
            id="profile-{p.id}-model"
            class="w-72 font-mono"
            placeholder={engineOf(p)?.modelHint ?? ""}
            bind:value={p.model}
            onchange={() => refreshPresence(p)}
          />
          <!-- Same footprint as the trash button above, so the fields line up. -->
          <span class="flex w-5 justify-center">
            {#if p.model.trim() === ""}
              <!-- Nothing to fetch yet. -->
            {:else if download?.state === "running"}
              <button
                type="button"
                class="cursor-default"
                title="{download.total > 0
                  ? `${megabytes(download.completed)} / ${megabytes(download.total)}`
                  : 'Téléchargement…'} · cliquer pour annuler"
                onclick={() => Engine.CancelDownload(p.engine as Kind, p.model.trim())}
              >
                {#if download.total > 0}
                  <ProgressCircle small label="Téléchargement du modèle" value={(download.completed / download.total) * 100} />
                {:else}
                  <Spinner small label="Téléchargement du modèle" />
                {/if}
              </button>
            {:else if presences[key] === "ready"}
              <span title="Modèle téléchargé">
                <CheckCircleIcon size={16} weight="fill" class="text-(--p-sys-green)" aria-label="Modèle téléchargé" />
              </span>
            {:else if presences[key] === "nofile"}
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
                disabled={!engineOf(p)?.installed}
                onclick={() => startDownload(p)}
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
      </FormRow>
      <FormRow label="Jobs en parallèle" for="profile-{p.id}-jobs">
        <Stepper id="profile-{p.id}-jobs" class="me-7 w-20" min={1} max={64} step={1} bind:value={p.maxJobs} />
      </FormRow>
      <FormRow label="Contexte par job (tokens)" for="profile-{p.id}-context">
        <Stepper
          id="profile-{p.id}-context"
          class="me-7 w-28"
          min={4096}
          step={4096}
          bind:value={p.contextPerJob}
        />
      </FormRow>
      <FormRow label="Arguments du moteur" for="profile-{p.id}-args">
        <TextField
          id="profile-{p.id}-args"
          class="me-7 w-72 font-mono"
          placeholder={engineOf(p)?.argsHint ?? ""}
          bind:value={p.extraArgs}
        />
      </FormRow>
      <FormRow label="Tours max par job" for="profile-{p.id}-turns">
        <Stepper id="profile-{p.id}-turns" class="me-7 w-20" min={5} max={500} step={5} bind:value={p.maxTurns} />
      </FormRow>
      <FormRow label="Durée max par job (minutes)" for="profile-{p.id}-minutes">
        <Stepper id="profile-{p.id}-minutes" class="me-7 w-20" min={5} max={600} step={5} bind:value={p.maxMinutes} />
      </FormRow>
      <FormRow label="Déclencheur">
        <SegmentedControl label="Déclencheur" class="me-7" options={TRIGGERS} bind:value={p.trigger} />
      </FormRow>
      {#if p.trigger === "issues"}
        <FormRow label="Label des issues" for="profile-{p.id}-label">
          <TextField
            id="profile-{p.id}-label"
            class="me-7 w-72 font-mono"
            placeholder="mihari::todo"
            bind:value={p.label}
          />
        </FormRow>
      {/if}
      <div class="p-form-row flex-col items-stretch! py-2.5">
        <div class="flex items-center gap-2">
          <label class="p-form-row-label" for="profile-{p.id}-prompt">Prompt</label>
          <Select
            size="small"
            variant="borderless"
            aria-label="Charger un prompt prêt"
            options={presetOptions}
            value=""
            onchange={(e) => loadPreset(p, e.currentTarget)}
          />
        </div>
        <TextArea
          id="profile-{p.id}-prompt"
          rows={8}
          class="w-full resize-y font-mono"
          placeholder="Écris ton prompt ou charge un prompt prêt."
          bind:value={p.prompt}
        />
      </div>
    </FormGroup>
  {:else}
    <FormGroup>
      <p class="p-body py-4 text-center text-tertiary-label">Aucun profil</p>
    </FormGroup>
  {/each}

  <div class="mt-4 flex items-center gap-2">
    <p class="p-subhead-2 flex min-w-0 flex-1 items-center gap-1.5" role="status">
      {#if status.kind === "busy"}
        <Spinner small label={status.label} />
        <span class="text-secondary-label">{status.label}</span>
      {:else if status.kind === "ok"}
        <CheckCircleIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-green)" aria-hidden="true" />
        <span class="text-secondary-label">{status.message}</span>
      {:else if status.kind === "error" || problem}
        <WarningCircleIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-red)" aria-hidden="true" />
        <span class="text-label">{status.kind === "error" ? status.message : problem}</span>
      {:else if downloadError}
        <WarningCircleIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-red)" aria-hidden="true" />
        <span class="text-label select-text">{downloadError}</span>
      {/if}
    </p>

    <Button disabled={busy} onclick={add}>
      <PlusIcon size={14} aria-hidden="true" />
      Ajouter un profil
    </Button>
    <Button type="submit" variant="primary" disabled={!!problem || busy}>Enregistrer</Button>
  </div>
</form>
