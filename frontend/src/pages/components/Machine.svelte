<script lang="ts">
  import { CheckCircleIcon, WarningCircleIcon } from "phosphor-svelte";
  import { Service as Advisor } from "../../../bindings/github.com/Impre-visible/mihari/internal/advisor";
  import type {
    Overview,
    Recommendation,
  } from "../../../bindings/github.com/Impre-visible/mihari/internal/advisor/models";
  import { Service as Settings } from "../../../bindings/github.com/Impre-visible/mihari/internal/settings";
  import { FormGroup, FormRow, Select, Spinner, Stepper } from "$lib/ui";
  import ContextSlider from "./ContextSlider.svelte";

  type Status =
    | { kind: "idle" }
    | { kind: "busy"; label: string }
    | { kind: "ok"; message: string }
    | { kind: "error"; message: string };

  const NEW_PROFILE = "__new__";

  let machine = $state<Overview | null>(null);
  let jobs = $state(1);
  let contextPerJob = $state(32768);
  let recs = $state<Recommendation[]>([]);
  let loading = $state(false);
  let error = $state<string | null>(null);
  let status = $state<Status>({ kind: "idle" });
  // Quantization picked per model, by Ref; the best one until changed.
  let picked = $state<Record<string, string>>({});
  let profiles = $state<{ id: string; name: string }[]>([]);

  let applyOptions = $derived([
    { value: "", label: "Appliquer à…", disabled: true },
    ...profiles.map((p) => ({ value: p.id, label: p.name })),
    { value: NEW_PROFILE, label: "Nouveau profil" },
  ]);

  Advisor.Machine()
    .then((m) => (machine = m))
    .catch((err) => (error = errorMessage(err)));
  loadProfiles();

  function loadProfiles() {
    Settings.GetConfig().then((config) => {
      const stored = Array.isArray(config?.profiles) ? config.profiles : [];
      profiles = stored
        .filter((p: unknown): p is { id: string; name: string } => {
          const v = p as Record<string, unknown>;
          return typeof v?.id === "string" && typeof v?.name === "string";
        })
        .map((p: { id: string; name: string }) => ({ id: p.id, name: p.name }));
    });
  }

  // Recomputed shortly after the settings stop changing, not on every step.
  let timer: ReturnType<typeof setTimeout> | undefined;
  $effect(() => {
    const j = jobs;
    const c = contextPerJob;
    clearTimeout(timer);
    timer = setTimeout(() => recommend(j, c), 400);
    return () => clearTimeout(timer);
  });

  async function recommend(j: number, c: number) {
    if (!Number.isInteger(j) || j < 1 || !Number.isInteger(c) || c < 1024) return;
    loading = true;
    error = null;
    try {
      recs = (await Advisor.Recommend(j, c)) ?? [];
      picked = {};
    } catch (err) {
      error = errorMessage(err);
    } finally {
      loading = false;
    }
  }

  function errorMessage(err: unknown) {
    return err instanceof Error ? err.message : String(err);
  }

  function size(bytes: number) {
    return `${(bytes / 2 ** 30).toLocaleString("fr-FR", { maximumFractionDigits: 1 })} Go`;
  }

  function speed(tps: number) {
    if (!tps) return "vitesse inconnue";
    return `~${tps >= 10 ? Math.round(tps) : tps.toFixed(1).replace(".", ",")} tok/s par job`;
  }

  // The recommendation shown for a model: its best fit or the quantization picked.
  function chosen(r: Recommendation) {
    const ref = picked[r.ref];
    return [r, ...(r.alternatives ?? [])].find((x) => x.ref === ref) ?? r;
  }

  async function apply(r: Recommendation, select: HTMLSelectElement) {
    const target = select.value;
    select.value = "";
    if (!target) return;
    status = { kind: "busy", label: "Application…" };
    try {
      await Advisor.Apply(target === NEW_PROFILE ? "" : target, r, jobs, contextPerJob);
      const name = target === NEW_PROFILE ? r.model : (profiles.find((p) => p.id === target)?.name ?? "");
      status = { kind: "ok", message: `${r.model} ${r.quant} appliqué à ${name}. Télécharge-le depuis Profils.` };
      loadProfiles();
    } catch (err) {
      status = { kind: "error", message: errorMessage(err) };
    }
  }
</script>

<p class="p-body text-secondary-label">
  Le matériel détecté et les modèles qui y tiennent, du meilleur au plus léger.
</p>

{#if machine}
  {@const h = machine.hardware}
  {@const b = machine.budget}
  <FormGroup title="Machine" class="mt-6">
    <FormRow label="Processeur" detail="{h.cpu || h.arch} · {h.cores} cœurs" />
    {#if h.unified}
      <FormRow label="Mémoire unifiée" detail={size(h.ram)} />
    {:else}
      <FormRow label="RAM" detail={size(h.ram)} />
      {#each h.gpus ?? [] as g}
        <FormRow label="GPU" detail="{g.name} · {size(g.vram)}" />
      {:else}
        <FormRow label="GPU" detail="Aucun GPU NVIDIA détecté : les modèles tourneront sur le CPU, lentement." />
      {/each}
    {/if}
    <FormRow
      label="Mémoire pour les modèles"
      detail={b.ram > 0 ? `${size(b.gpu)} en GPU + ${size(b.ram)} en RAM` : size(b.gpu)}
    />
    <div class="p-form-row gap-1.5">
      <span class="p-form-row-label">Moteur</span>
      {#if machine.engine.installed}
        <CheckCircleIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-green)" aria-hidden="true" />
        <span class="text-secondary-label">{machine.engine.label}</span>
      {:else}
        <WarningCircleIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-orange)" aria-hidden="true" />
        <span class="text-secondary-label">
          {machine.engine.label}, pas installé :
          <code class="font-mono text-label select-text">{machine.engine.install}</code>
        </span>
      {/if}
    </div>
  </FormGroup>
{:else if !error}
  <p class="p-subhead-2 mt-6 flex items-center gap-1.5 text-secondary-label">
    <Spinner small label="Détection du matériel" />
    Détection du matériel…
  </p>
{/if}

<FormGroup
  title="Besoin"
  class="mt-6"
  footer="Plus de jobs en parallèle ou de contexte laisse moins de place au modèle."
>
  <FormRow label="Jobs en parallèle" for="machine-jobs">
    <Stepper id="machine-jobs" class="w-20" min={1} max={64} step={1} bind:value={jobs} />
  </FormRow>
  <FormRow label="Contexte par job (tokens)" for="machine-context">
    <ContextSlider id="machine-context" bind:value={contextPerJob} />
  </FormRow>
</FormGroup>

<div class="mt-6 flex items-center gap-2">
  <h2 class="p-box-title ms-2.5 flex-1">Modèles qui tiennent ({recs.length})</h2>
  {#if loading}
    <Spinner small label="Calcul" />
  {/if}
</div>

{#if error}
  <p class="p-subhead-2 mt-2 flex items-center gap-1.5">
    <WarningCircleIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-red)" aria-hidden="true" />
    <span class="text-label select-text">{error}</span>
  </p>
{/if}

<FormGroup
  class="mt-2"
  footer="Qualité et vitesse sont des estimations pour comparer les modèles entre eux, pas des mesures."
>
  {#each recs as r (r.model)}
    {@const c = chosen(r)}
    {@const quants = [r, ...(r.alternatives ?? [])]}
    <div class="p-form-row gap-2 py-2">
      <div class="flex min-w-0 flex-1 flex-col">
        <span class="flex items-center gap-1.5">
          <span class="truncate">{r.model}</span>
          {#if c.spill > 0}
            <span
              class="p-subhead-2 shrink-0 rounded px-1.5 text-(--p-sys-orange) ring-1 ring-(--p-sys-orange)"
              title="{size(c.spill)} débordent du GPU en RAM"
            >
              lent
            </span>
          {/if}
        </span>
        <span class="p-subhead-2 truncate text-secondary-label">
          Qualité {c.quality} · {size(c.total)}{c.spill > 0 ? ` dont ${size(c.spill)} en RAM` : ""} · {speed(
            c.tokensPerSecond,
          )}
        </span>
      </div>
      {#if quants.length > 1}
        <Select
          size="small"
          aria-label="Quantification de {r.model}"
          options={quants.map((q) => ({ value: q.ref, label: q.quant }))}
          value={c.ref}
          onchange={(e) => (picked[r.ref] = e.currentTarget.value)}
        />
      {:else}
        <span class="p-subhead-2 text-secondary-label">{c.quant}</span>
      {/if}
      <Select
        size="small"
        variant="borderless"
        aria-label="Appliquer {r.model} à un profil"
        options={applyOptions}
        value=""
        disabled={status.kind === "busy"}
        onchange={(e) => apply(c, e.currentTarget)}
      />
    </div>
  {:else}
    <p class="p-body py-4 text-center text-tertiary-label">
      {loading ? "Calcul…" : "Aucun modèle du catalogue ne tient avec ces réglages."}
    </p>
  {/each}
</FormGroup>

<p class="p-subhead-2 mt-4 flex min-w-0 items-center gap-1.5" role="status">
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
