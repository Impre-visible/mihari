<script lang="ts">
  import { CaretDownIcon, CaretRightIcon, CheckCircleIcon, WarningCircleIcon } from "phosphor-svelte";
  import { Service as Projects } from "../../../bindings/github.com/Impre-visible/mihari/internal/projects";
  import type {
    Settings as ProjectSettings,
    View,
  } from "../../../bindings/github.com/Impre-visible/mihari/internal/projects/models";
  import type { Member } from "../../../bindings/github.com/Impre-visible/mihari/internal/gitlab/models";
  import { Service as Settings } from "../../../bindings/github.com/Impre-visible/mihari/internal/settings";
  import { Service as Tasks } from "../../../bindings/github.com/Impre-visible/mihari/internal/tasks";
  import { Service as Sandbox } from "../../../bindings/github.com/Impre-visible/mihari/internal/sandbox";
  import {
    Button,
    Checkbox,
    FormGroup,
    FormRow,
    SearchField,
    Select,
    Spinner,
    Switch,
    TextArea,
    TextField,
  } from "$lib/ui";

  type Status =
    | { kind: "idle" }
    | { kind: "busy"; label: string }
    | { kind: "ok"; message: string }
    | { kind: "error"; message: string };

  // GitLab access levels, as in internal/gitlab.
  const ROLES = [
    { value: "10", label: "Guest" },
    { value: "20", label: "Reporter" },
    { value: "30", label: "Developer" },
    { value: "40", label: "Maintainer" },
    { value: "50", label: "Owner" },
  ];
  const DEVELOPER = 30;

  let projects = $state<View[]>([]);
  let defaultImage = $state("");
  let loading = $state(true);
  let loadError = $state<string | null>(null);
  let status = $state<Status>({ kind: "idle" });
  let query = $state("");
  let expanded = $state<Record<number, boolean>>({});
  let members = $state<Record<number, Member[] | string>>({});
  // What the agent noted about each project, loaded when it is unfolded.
  let memory = $state<Record<number, string>>({});
  let memoryStatus = $state<Record<number, string>>({});

  let busy = $derived(status.kind === "busy");
  let visible = $derived(
    query.trim() === ""
      ? projects
      : projects.filter((p) => p.path.toLowerCase().includes(query.trim().toLowerCase())),
  );
  let enabledCount = $derived(projects.filter((p) => p.settings.enabled).length);

  Projects.List()
    .then((overview) => {
      projects = overview.projects ?? [];
      defaultImage = overview.defaultImage;
    })
    .catch((err) => (loadError = errorMessage(err)))
    .finally(() => (loading = false));

  function errorMessage(err: unknown) {
    return err instanceof Error ? err.message : String(err);
  }

  function roleLabel(level: number) {
    return ROLES.find((r) => Number(r.value) === level)?.label ?? `Niveau ${level}`;
  }

  function toggle(p: View) {
    expanded[p.id] = !expanded[p.id];
    if (expanded[p.id] && memory[p.id] === undefined) {
      Projects.Memory(p.id).then((m) => (memory[p.id] = m));
    }
    if (expanded[p.id] && members[p.id] === undefined) {
      Projects.Members(p.id)
        .then((list) => (members[p.id] = (list ?? []).sort((a, b) => b.accessLevel - a.accessLevel)))
        .catch((err) => (members[p.id] = errorMessage(err)));
    }
  }

  // Go encodes an empty slice as null.
  function extras(s: ProjectSettings) {
    return s.extraUsers ?? [];
  }

  function blocked(s: ProjectSettings) {
    return s.blockedUsers ?? [];
  }

  // Same rule as Settings.Authorizes in internal/projects.
  function authorized(s: ProjectSettings, m: Member) {
    if (blocked(s).includes(m.username)) return false;
    return m.accessLevel >= s.minRole || extras(s).includes(m.username);
  }

  function why(s: ProjectSettings, m: Member) {
    if (blocked(s).includes(m.username)) return "bloqué";
    if (m.accessLevel >= s.minRole) return "par son rôle";
    return extras(s).includes(m.username) ? "ajouté à la main" : "rôle insuffisant";
  }

  // Unticking someone the role lets in blocks them; ticking someone below it adds them.
  function toggleAccess(s: ProjectSettings, m: Member) {
    const without = (list: string[]) => list.filter((u) => u !== m.username);
    if (authorized(s, m)) {
      s.extraUsers = without(extras(s));
      if (m.accessLevel >= s.minRole) s.blockedUsers = [...blocked(s), m.username];
    } else {
      s.blockedUsers = without(blocked(s));
      if (m.accessLevel < s.minRole) s.extraUsers = [...extras(s), m.username];
    }
    resetStatus();
  }

  // Only projects that differ from the defaults are worth storing.
  function customized(s: ProjectSettings) {
    return (
      s.enabled ||
      s.minRole !== DEVELOPER ||
      extras(s).length > 0 ||
      blocked(s).length > 0 ||
      s.image.trim() !== "" ||
      s.instructions.trim() !== ""
    );
  }

  let cacheStatus = $state<Record<number, string>>({});

  async function clearCache(id: number) {
    cacheStatus[id] = "Suppression…";
    try {
      await Sandbox.ClearCache(id);
      cacheStatus[id] = "Cache vidé.";
    } catch (err) {
      cacheStatus[id] = errorMessage(err);
    }
  }

  async function saveMemory(id: number) {
    memoryStatus[id] = "Enregistrement…";
    try {
      await Projects.SetMemory(id, memory[id] ?? "");
      memoryStatus[id] = "Notes enregistrées.";
    } catch (err) {
      memoryStatus[id] = errorMessage(err);
    }
  }

  async function save() {
    status = { kind: "busy", label: "Enregistrement…" };
    try {
      await Settings.Set("default_image", defaultImage.trim());
      await Settings.Set(
        "projects",
        projects
          .map((p) => p.settings)
          .filter(customized)
          .map((s) => ({ ...s, extraUsers: extras(s), blockedUsers: blocked(s), image: s.image.trim(), instructions: s.instructions.trim() })),
      );
      // Todos of a project just switched on can be sorted right away.
      Tasks.PollNow();
      status = { kind: "ok", message: "Projets enregistrés." };
    } catch (err) {
      status = { kind: "error", message: errorMessage(err) };
    }
  }

  function resetStatus() {
    if (status.kind !== "busy") status = { kind: "idle" };
  }
</script>

<p class="p-body text-secondary-label">
  Les projets où le bot est membre. Il ne travaille que sur ceux qui sont allumés.
</p>

<form
  class="mt-6"
  oninput={resetStatus}
  onsubmit={(e) => {
    e.preventDefault();
    save();
  }}
>
  <FormGroup title="Par défaut" footer="Un projet sans image propre fait tourner ses jobs dans celle-ci.">
    <FormRow label="Image du container" for="default-image">
      <TextField id="default-image" class="w-72 font-mono" placeholder="debian:stable-slim" bind:value={defaultImage} />
    </FormRow>
  </FormGroup>

  {#if loading}
    <p class="p-subhead-2 mt-6 flex items-center gap-1.5 text-secondary-label">
      <Spinner small label="Chargement des projets" />
      Chargement des projets…
    </p>
  {:else if loadError}
    <p class="p-subhead-2 mt-6 flex items-center gap-1.5">
      <WarningCircleIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-red)" aria-hidden="true" />
      <span class="text-label select-text">{loadError}</span>
    </p>
  {:else}
    <div class="mt-6 flex items-center gap-2">
      <h2 class="p-box-title ms-2.5 flex-1">Projets ({enabledCount} allumés sur {projects.length})</h2>
      {#if projects.length > 6}
        <SearchField class="w-56" placeholder="Filtrer" bind:value={query} />
      {/if}
    </div>

    {#each visible as p (p.id)}
      {@const s = p.settings}
      <FormGroup class="mt-2">
        <div class="p-form-row">
          <Switch label="Allumer {p.path}" bind:checked={s.enabled} onchange={resetStatus} />
          <button type="button" class="flex min-w-0 flex-1 items-center gap-1.5 text-start" onclick={() => toggle(p)}>
            <span class="min-w-0 flex-1 truncate">{p.path}</span>
            {#if !p.accessible}
              <WarningCircleIcon size={14} weight="fill" class="shrink-0 text-(--p-sys-orange)" aria-hidden="true" />
              <span class="p-subhead-2 shrink-0 text-secondary-label">inaccessible au bot</span>
            {/if}
            {#if expanded[p.id]}
              <CaretDownIcon size={12} class="shrink-0 text-secondary-label" aria-label="Replier" />
            {:else}
              <CaretRightIcon size={12} class="shrink-0 text-secondary-label" aria-label="Déplier" />
            {/if}
          </button>
        </div>

        {#if expanded[p.id]}
          {@const list = members[p.id]}
          <FormRow label="Rôle minimum pour appeler le bot" for="role-{p.id}">
            <Select
              id="role-{p.id}"
              options={ROLES}
              value={String(s.minRole)}
              onchange={(e) => {
                s.minRole = Number(e.currentTarget.value);
                resetStatus();
              }}
            />
          </FormRow>
          {#if list === undefined}
            <div class="p-form-row gap-1.5 text-secondary-label">
              <Spinner small label="Chargement des membres" />
              Chargement des membres…
            </div>
          {:else if typeof list === "string"}
            <div class="p-form-row gap-1.5">
              <WarningCircleIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-red)" aria-hidden="true" />
              <span class="p-subhead-2 text-label select-text">{list}</span>
            </div>
          {:else}
            {#each list as m (m.username)}
              <FormRow label="{m.name} (@{m.username})" detail="{roleLabel(m.accessLevel)} · {why(s, m)}">
                <Checkbox
                  aria-label="Autoriser @{m.username}"
                  checked={authorized(s, m)}
                  onchange={() => toggleAccess(s, m)}
                />
              </FormRow>
            {:else}
              <div class="p-form-row p-subhead-2 text-tertiary-label">Aucun autre membre que le bot.</div>
            {/each}
          {/if}
          <FormRow label="Image du container" for="image-{p.id}">
            <TextField
              id="image-{p.id}"
              class="w-72 font-mono"
              placeholder="celle du job de test de la CI, sinon {defaultImage}"
              bind:value={s.image}
            />
          </FormRow>
          <FormRow label="Cache des dépendances" detail={cacheStatus[p.id] ?? "npm, pip, go… gardés entre les jobs"}>
            <Button size="small" onclick={() => clearCache(p.id)}>Vider</Button>
          </FormRow>
          <div class="p-form-row flex-col items-stretch! py-2.5">
            <label class="p-form-row-label" for="instructions-{p.id}">Consignes pour l'agent</label>
            <TextArea
              id="instructions-{p.id}"
              rows={4}
              class="w-full resize-y"
              placeholder="Les tests vont dans tests/ et utilisent pytest. Lance make check avant de finir."
              bind:value={s.instructions}
            />
            <p class="p-sublabel">
              Ajoutées au prompt du profil, avec AGENTS.md et .mihari.md du repo s'ils existent sur la branche où
              l'agent travaille.
            </p>
          </div>
          <div class="p-form-row flex-col items-stretch! py-2.5">
            <div class="flex items-center gap-2">
              <label class="p-form-row-label" for="memory-{p.id}">Notes de Mihari</label>
              <span class="p-subhead-2 text-secondary-label">{memoryStatus[p.id] ?? ""}</span>
              <Button size="small" onclick={() => saveMemory(p.id)}>Enregistrer les notes</Button>
            </div>
            <TextArea
              id="memory-{p.id}"
              rows={4}
              class="w-full resize-y font-mono"
              placeholder="Vide : l'agent y écrit ce qu'il apprend du projet (commande de test, conventions…)."
              value={memory[p.id] ?? ""}
              oninput={(e) => {
                memory[p.id] = e.currentTarget.value;
                memoryStatus[p.id] = "";
              }}
            />
          </div>
        {/if}
      </FormGroup>
    {:else}
      <FormGroup class="mt-2">
        <p class="p-body py-4 text-center text-tertiary-label">
          {projects.length === 0 ? "Le bot n'est membre d'aucun projet." : "Aucun projet ne correspond."}
        </p>
      </FormGroup>
    {/each}
  {/if}

  <div class="mt-4 flex items-center gap-2">
    <p class="p-subhead-2 flex min-w-0 flex-1 items-center gap-1.5" role="status">
      {#if status.kind === "busy"}
        <Spinner small label={status.label} />
        <span class="text-secondary-label">{status.label}</span>
      {:else if status.kind === "ok"}
        <CheckCircleIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-green)" aria-hidden="true" />
        <span class="text-secondary-label">{status.message}</span>
      {:else if status.kind === "error"}
        <WarningCircleIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-red)" aria-hidden="true" />
        <span class="text-label">{status.message}</span>
      {/if}
    </p>
    <Button type="submit" variant="primary" disabled={busy || loading || !!loadError}>Enregistrer</Button>
  </div>
</form>
