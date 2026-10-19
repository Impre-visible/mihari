<script lang="ts">
  import { ArrowSquareOutIcon, WarningCircleIcon } from "phosphor-svelte";
  import { Browser } from "@wailsio/runtime";
  import { Service as Projects } from "../../../bindings/github.com/Impre-visible/mihari/internal/projects";
  import type { Settings as ProjectSettings, View } from "../../../bindings/github.com/Impre-visible/mihari/internal/projects/models";
  import type { Member } from "../../../bindings/github.com/Impre-visible/mihari/internal/gitlab/models";
  import { Service as Sandbox } from "../../../bindings/github.com/Impre-visible/mihari/internal/sandbox";
  import { Button, Checkbox, FormGroup, FormRow, Select, Spinner, Switch, TextArea, TextField } from "$lib/ui";
  import { debounced, save } from "$lib/save.svelte";
  import { blocked, extras, gitlab } from "./store.svelte";

  // One project: whether the bot works on it, who may call it, the
  // container its jobs run in, and what the agent is told and has learned.
  type Props = { project: View };
  let { project: p }: Props = $props();
  let s = $derived(p.settings);

  // GitLab access levels, as in internal/gitlab.
  const ROLES = [
    { value: "10", label: "Guest" },
    { value: "20", label: "Reporter" },
    { value: "30", label: "Developer" },
    { value: "40", label: "Maintainer" },
    { value: "50", label: "Owner" },
  ];

  let members = $state<Member[] | string | null>(null);
  let memory = $state<string | null>(null);
  let cache = $state("");

  $effect(() => {
    const id = p.id;
    members = null;
    memory = null;
    cache = "";
    Projects.Members(id)
      .then((list) => (members = (list ?? []).sort((a, b) => b.accessLevel - a.accessLevel)))
      .catch((err) => (members = err instanceof Error ? err.message : String(err)));
    Projects.Memory(id).then((m) => (memory = m));
  });

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
  function toggleAccess(m: Member) {
    const without = (list: string[]) => list.filter((u) => u !== m.username);
    if (authorized(s, m)) {
      s.extraUsers = without(extras(s));
      if (m.accessLevel >= s.minRole) s.blockedUsers = [...blocked(s), m.username];
    } else {
      s.blockedUsers = without(blocked(s));
      if (m.accessLevel < s.minRole) s.extraUsers = [...extras(s), m.username];
    }
  }

  const saveMemory = debounced((id: number, text: string) => save(() => Projects.SetMemory(id, text)));

  async function clearCache() {
    cache = "Suppression…";
    try {
      await Sandbox.ClearCache(p.id);
      cache = "Cache vidé.";
    } catch (err) {
      cache = err instanceof Error ? err.message : String(err);
    }
  }
</script>

<div class="flex flex-col gap-1">
  <header class="mb-2 flex items-center gap-3">
    <div class="min-w-0 flex-1">
      <h2 class="p-title-2 truncate font-semibold">{p.name || p.path}</h2>
      <p class="p-subhead-2 truncate text-secondary-label">{p.path} · branche {p.defaultBranch}</p>
    </div>
    <Button size="small" variant="borderless" onclick={() => Browser.OpenURL(p.webUrl)}>
      <ArrowSquareOutIcon size={14} aria-hidden="true" /> GitLab
    </Button>
  </header>

  {#if !p.accessible}
    <p class="p-subhead-2 mb-2 flex items-center gap-1.5 rounded-lg bg-(--p-sys-orange)/12 px-3 py-2 text-label">
      <WarningCircleIcon size={15} weight="fill" class="shrink-0 text-(--p-sys-orange)" aria-hidden="true" />
      Le bot n'a pas accès à ce projet : donne-lui au moins le rôle Developer sur GitLab.
    </p>
  {/if}

  <FormGroup>
    <FormRow label="Le bot travaille sur ce projet" for="pr-enabled">
      <Switch id="pr-enabled" label="Le bot travaille sur ce projet" bind:checked={s.enabled} />
    </FormRow>
  </FormGroup>
  <p class="p-subhead-2 mb-3 ms-3 text-tertiary-label">Éteint, ses mentions et ses issues attendent sur GitLab sans être traitées.</p>

  <FormGroup title="Qui peut appeler le bot">
    <FormRow label="Rôle minimum" for="pr-role">
      <Select id="pr-role" options={ROLES} value={String(s.minRole)} onchange={(e) => (s.minRole = Number(e.currentTarget.value))} />
    </FormRow>
    {#if members === null}
      <div class="p-form-row gap-1.5 text-secondary-label"><Spinner small label="Chargement des membres" /> Chargement des membres…</div>
    {:else if typeof members === "string"}
      <p class="p-form-row p-subhead-2 text-(--p-sys-red) select-text">{members}</p>
    {:else}
      {#each members as m (m.username)}
        <FormRow label="{m.name} (@{m.username})" detail="{ROLES.find((r) => Number(r.value) === m.accessLevel)?.label ?? m.accessLevel} · {why(s, m)}">
          <Checkbox aria-label="Autoriser @{m.username}" checked={authorized(s, m)} onchange={() => toggleAccess(m)} />
        </FormRow>
      {:else}
        <p class="p-form-row p-subhead-2 text-tertiary-label">Aucun autre membre que le bot.</p>
      {/each}
    {/if}
  </FormGroup>

  <FormGroup title="Container des jobs" class="mt-3">
    <FormRow label="Image" for="pr-image">
      <TextField id="pr-image" class="w-72 font-mono" placeholder="celle de la CI, sinon {gitlab.defaultImage || 'par défaut'}" bind:value={s.image} />
    </FormRow>
    <FormRow label="Cache des dépendances" detail={cache || "npm, pip, go… gardés entre les jobs"}>
      <Button size="small" onclick={clearCache}>Vider</Button>
    </FormRow>
  </FormGroup>

  <FormGroup title="Consignes pour l'agent" class="mt-3">
    <div class="p-form-row flex-col items-stretch! py-2.5">
      <TextArea rows={4} class="w-full resize-y" placeholder="Les tests vont dans tests/ et utilisent pytest. Lance make check avant de finir." bind:value={s.instructions} />
      <p class="p-sublabel">Ajoutées au prompt du profil, avec AGENTS.md et .mihari.md du repo s'ils existent.</p>
    </div>
  </FormGroup>

  <FormGroup title="Notes de Mihari" class="mt-3">
    <div class="p-form-row flex-col items-stretch! py-2.5">
      {#if memory === null}
        <Spinner small label="Chargement des notes" />
      {:else}
        <TextArea
          rows={5}
          class="w-full resize-y font-mono"
          placeholder="Vide : l'agent y note ce qu'il apprend du projet (commande de test, conventions…)."
          value={memory}
          oninput={(e) => {
            memory = e.currentTarget.value;
            saveMemory(p.id, memory);
          }}
        />
        <p class="p-sublabel">Ce que l'agent garde d'un job à l'autre, avec son outil memory. Corrige ou efface ce qui est faux.</p>
      {/if}
    </div>
  </FormGroup>
</div>
