<script lang="ts">
  import { CheckCircleIcon, PlusIcon, TrashIcon, WarningCircleIcon } from "phosphor-svelte";
  import { Service as Settings } from "../../../bindings/github.com/Impre-visible/mihari/internal/settings";
  import { Button, FormGroup, FormRow, Select, Spinner, TextArea, TextField } from "$lib/ui";
  import { PROMPT_PRESETS } from "./prompts";

  type Status =
    | { kind: "idle" }
    | { kind: "busy"; label: string }
    | { kind: "ok"; message: string }
    | { kind: "error"; message: string };

  // The schedule refers to profiles by id, so renaming one keeps its segments.
  type Profile = { id: string; name: string; model: string; prompt: string };

  let profiles = $state<Profile[]>([]);
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

  function parse(value: unknown): Profile | null {
    if (typeof value !== "object" || value === null) return null;
    const { id, name, model, prompt } = value as Record<string, unknown>;
    if (typeof id !== "string" || typeof name !== "string") return null;
    return {
      id,
      name,
      model: typeof model === "string" ? model : "",
      prompt: typeof prompt === "string" ? prompt : "",
    };
  }

  Settings.GetConfig()
    .then((config) => {
      const stored = Array.isArray(config?.profiles) ? config.profiles : [];
      profiles = stored.map(parse).filter((p): p is Profile => p !== null);
    })
    .catch((err) => console.error("Load profiles:", err));

  function validate(list: Profile[]): string | null {
    const names = new Set<string>();
    for (const p of list) {
      const name = p.name.trim();
      if (name === "") return "Chaque profil a besoin d'un nom.";
      if (names.has(name.toLowerCase())) return `Deux profils s'appellent ${name}.`;
      names.add(name.toLowerCase());
      if (p.model.trim() === "") return `${name} : aucun modèle renseigné.`;
      if (p.prompt.trim() === "") return `${name} : le prompt est vide.`;
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
    profiles.push({ id: newId(), name: "", model: "", prompt: "" });
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
        profiles.map((p) => ({ id: p.id, name: p.name.trim(), model: p.model.trim(), prompt: p.prompt })),
      );
      status = { kind: "ok", message: "Profils enregistrés." };
    } catch (err) {
      status = { kind: "error", message: err instanceof Error ? err.message : String(err) };
    }
  }

  function resetStatus() {
    if (status.kind !== "busy") status = { kind: "idle" };
  }
</script>

<p class="p-body text-secondary-label">Un modèle et le prompt qu'il reçoit. Le planning dit quand chacun tourne.</p>

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
      <FormRow label="Modèle" for="profile-{p.id}-model">
        <TextField
          id="profile-{p.id}-model"
          class="me-7 w-72"
          placeholder="Qwen/Qwen2.5-Coder-32B-Instruct"
          bind:value={p.model}
        />
      </FormRow>
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
      {/if}
    </p>

    <Button disabled={busy} onclick={add}>
      <PlusIcon size={14} aria-hidden="true" />
      Ajouter un profil
    </Button>
    <Button type="submit" variant="primary" disabled={!!problem || busy}>Enregistrer</Button>
  </div>
</form>
