<script lang="ts">
  import { PlusIcon, TrashIcon, WarningCircleIcon } from "phosphor-svelte";
  import {
    Button,
    FormGroup,
    FormRow,
    SegmentedControl,
    Select,
    SteppedSlider,
    Stepper,
    Switch,
    TextArea,
    TextField,
  } from "$lib/ui";
  import { CONTEXT_STEPS, tokens } from "$lib/context";
  import { live } from "$lib/live.svelte";
  import { PROMPT_PRESETS } from "../components/prompts";
  import ModelField from "./ModelField.svelte";
  import { WEEK, agent, hue, newSegment, profileProblem, segmentProblem, type Profile, type Segment } from "./store.svelte";

  // One profile: its model, what starts it, its prompt, its limits and the
  // hours it runs.
  type Props = { profile: Profile; ondelete: () => void };
  let { profile: p, ondelete }: Props = $props();

  let engine = $derived(agent.engines.find((e) => e.kind === p.engine));
  let problem = $derived(profileProblem(p));
  let segments = $derived(agent.segments.filter((s) => s.profile === p.id));
  let activeNow = $derived(live.engine?.profileId === p.id && live.engine.state !== "idle");

  const engineOptions = $derived(
    agent.engines.map((e) => ({
      value: e.kind as string,
      label: e.supported ? (e.installed ? e.label : `${e.label} (non installé)`) : `${e.label} (indisponible ici)`,
      disabled: !e.supported,
    })),
  );
  const presetOptions = [
    { value: "", label: "Charger un prompt prêt…", disabled: true },
    ...PROMPT_PRESETS.map((preset) => ({ value: preset.id, label: preset.label })),
  ];

  // The menu is an action, not a setting, so it snaps back once used.
  function loadPreset(select: HTMLSelectElement) {
    const preset = PROMPT_PRESETS.find((c) => c.id === select.value);
    select.value = "";
    if (!preset) return;
    p.prompt = preset.prompt;
  }

  function toggleDay(s: Segment, day: number) {
    s.days = s.days.includes(day) ? s.days.filter((d) => d !== day) : [...s.days, day];
  }

  function addSegment() {
    agent.segments.push(newSegment(p.id, segments.length ? "19:00" : "09:00", segments.length ? "23:00" : "18:00"));
  }

  function removeSegment(s: Segment) {
    agent.segments = agent.segments.filter((o) => o !== s);
  }
</script>

<div class="flex flex-col gap-1">
  <header class="mb-2 flex items-center gap-3">
    <span class="size-3 shrink-0 rounded-full" style:background="hsl({hue(p.id)} 70% 55%)" aria-hidden="true"></span>
    <input
      class="p-title-2 min-w-0 flex-1 bg-transparent font-semibold outline-none placeholder:text-tertiary-label"
      placeholder="Nom du profil"
      aria-label="Nom du profil"
      bind:value={p.name}
    />
    {#if activeNow}
      <span class="p-subhead-2 shrink-0 rounded-full bg-(--p-sys-green)/15 px-2 py-0.5 text-(--p-sys-green)">Actif maintenant</span>
    {/if}
  </header>
  {#if problem}
    <p class="p-subhead-2 mb-2 flex items-center gap-1.5 rounded-lg bg-(--p-sys-orange)/12 px-3 py-2 text-label">
      <WarningCircleIcon size={15} weight="fill" class="shrink-0 text-(--p-sys-orange)" aria-hidden="true" />
      Pas encore enregistré : {problem}
    </p>
  {/if}

  <FormGroup title="Modèle">
    <FormRow label="Moteur" for="p-engine">
      <Select id="p-engine" class="me-7" options={engineOptions} bind:value={p.engine} />
    </FormRow>
    {#if engine && !engine.installed}
      <p class="p-form-row p-subhead-2 py-2.5 text-secondary-label">
        {engine.label} n'est pas installé sur cette machine. Pour l'installer :
        <code class="font-mono text-label select-text">{engine.install}</code>
      </p>
    {/if}
    <FormRow label="Modèle" for="p-model">
      <ModelField id="p-model" {engine} bind:value={p.model} />
    </FormRow>
    <FormRow label="Arguments du moteur" for="p-args">
      <TextField id="p-args" class="me-7 w-72 font-mono" placeholder={engine?.argsHint ?? ""} bind:value={p.extraArgs} />
    </FormRow>
    <FormRow label="Le modèle voit les images" for="p-vision">
      <Switch id="p-vision" label="Le modèle voit les images" class="me-7" bind:checked={p.vision} />
    </FormRow>
  </FormGroup>
  <p class="p-subhead-2 mb-3 ms-3 text-tertiary-label">
    Les images donnent à l'agent read_image (captures jointes, rendus de tests) : seulement si le modèle les accepte.
  </p>

  <FormGroup title="Déclencheur">
    <FormRow label="Quand il travaille">
      <SegmentedControl
        label="Déclencheur"
        class="me-7"
        options={[
          { value: "ping", label: "Quand on le mentionne" },
          { value: "issues", label: "Sur les issues d'un label" },
        ]}
        bind:value={p.trigger}
       
      />
    </FormRow>
    {#if p.trigger === "issues"}
      <FormRow label="Label des issues" for="p-label">
        <TextField id="p-label" class="me-7 w-72 font-mono" placeholder="mihari::todo" bind:value={p.label} />
      </FormRow>
    {/if}
  </FormGroup>

  <FormGroup title="Prompt">
    <div class="p-form-row flex-col items-stretch! gap-2 py-2.5">
      <Select
        size="small"
        variant="borderless"
        class="self-start"
        aria-label="Charger un prompt prêt"
        options={presetOptions}
        value=""
        onchange={(e) => loadPreset(e.currentTarget)}
      />
      <TextArea rows={8} class="w-full resize-y font-mono" placeholder="Ce que le modèle doit savoir de son rôle." bind:value={p.prompt} />
    </div>
  </FormGroup>

  <FormGroup title="Limites par job" class="mt-3">
    <FormRow label="Jobs en parallèle" for="p-jobs">
      <Stepper id="p-jobs" class="me-7 w-20" min={1} max={64} step={1} bind:value={p.maxJobs} />
    </FormRow>
    <FormRow label="Contexte (tokens)" for="p-context">
      <SteppedSlider id="p-context" class="me-7" steps={CONTEXT_STEPS} format={tokens} bind:value={p.contextPerJob} />
    </FormRow>
    <FormRow label="Tours" for="p-turns">
      <div class="me-7 flex items-center gap-3">
        <Switch label="Sans limite de tours" bind:checked={p.unlimitedTurns} />
        <span class="p-subhead-2 text-secondary-label">Sans limite</span>
        <Stepper id="p-turns" class="w-20" min={5} max={2000} step={5} disabled={p.unlimitedTurns} bind:value={p.maxTurns} />
      </div>
    </FormRow>
    <FormRow label="Durée (minutes)" for="p-minutes">
      <div class="me-7 flex items-center gap-3">
        <Switch label="Sans limite de durée" bind:checked={p.unlimitedTime} />
        <span class="p-subhead-2 text-secondary-label">Sans limite</span>
        <Stepper id="p-minutes" class="w-20" min={5} max={1440} step={5} disabled={p.unlimitedTime} bind:value={p.maxMinutes} />
      </div>
    </FormRow>
  </FormGroup>

  <div class="mt-3 flex items-center">
    <h3 class="p-box-title ms-2.5 flex-1">Plages horaires</h3>
    <Button size="small" variant="borderless" onclick={addSegment}><PlusIcon size={13} aria-hidden="true" /> Ajouter</Button>
  </div>
  <div class="p-box">
    {#each segments as s (s.key)}
      {@const err = segmentProblem(s)}
      <div class="p-form-row flex-wrap gap-x-3 gap-y-1 py-2">
        <div class="flex items-center gap-2">
          <TextField type="time" aria-label="Début" bind:value={s.start} />
          <span class="p-body text-secondary-label">à</span>
          <TextField type="time" aria-label="Fin" bind:value={s.end} />
        </div>
        <div class="flex flex-1 gap-1" role="group" aria-label="Jours">
          {#each WEEK as w (w.day)}
            <Button size="small" variant={s.days.includes(w.day) ? "primary" : "default"} class="w-7" aria-label={w.label} aria-pressed={s.days.includes(w.day)} onclick={() => toggleDay(s, w.day)}>
              {w.short}
            </Button>
          {/each}
        </div>
        <Button size="small" variant="borderless" iconOnly aria-label="Supprimer la plage" onclick={() => removeSegment(s)}>
          <TrashIcon size={14} aria-hidden="true" />
        </Button>
        {#if err}<p class="p-subhead-2 w-full text-(--p-sys-red)">{err} Cette plage n'est pas enregistrée.</p>{/if}
      </div>
    {:else}
      <p class="p-form-row p-subhead-2 py-3 text-secondary-label">
        Aucune plage : ce profil ne tourne que s'il est choisi pour « le reste du temps ».
      </p>
    {/each}
  </div>

  <div class="mt-6 flex justify-end">
    <Button variant="borderless" onclick={ondelete}><TrashIcon size={14} aria-hidden="true" /> Supprimer le profil</Button>
  </div>
</div>
