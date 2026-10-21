<script lang="ts">
  import { CheckCircleIcon, CpuIcon, WarningCircleIcon } from "phosphor-svelte";
  import { Service as Advisor } from "../../../bindings/github.com/Impre-visible/mihari/internal/advisor";
  import type { Onboarding, Slot } from "../../../bindings/github.com/Impre-visible/mihari/internal/advisor/models";
  import { Service as Engine } from "../../../bindings/github.com/Impre-visible/mihari/internal/engine";
  import type { Kind } from "../../../bindings/github.com/Impre-visible/mihari/internal/engine/models";
  import { Button, FormGroup, FormRow, Spinner } from "$lib/ui";
  import { tokens } from "$lib/context";
  import { PROMPT_PRESETS } from "../components/prompts";
  import { reload } from "../agent/store.svelte";
  import Machine from "../agent/MachineInspector.svelte";

  // The day and night setup that suits the machine, applied in one click: from
  // its memory on a Mac, from its GPUs on a PC. Without one, or on request,
  // the generic advisor.
  type Props = { onapplied: (profileId: string | null) => void };
  let { onapplied }: Props = $props();

  let onboarding = $state<Onboarding | null>(null);
  let manual = $state(false);
  let busy = $state(false);
  let error = $state<string | null>(null);

  Advisor.Onboarding()
    .then((o) => (onboarding = o))
    .catch((err) => {
      console.error("Onboarding:", err);
      manual = true;
    });

  function size(bytes: number) {
    return `${(bytes / 2 ** 30).toLocaleString("fr-FR", { maximumFractionDigits: 1 })} Go`;
  }

  // "Puce Apple M5 Ultra détectée…", or the GPUs of a PC.
  function detected(o: Onboarding) {
    const h = o.hardware;
    if (h.unified) return `Puce ${o.chip || h.cpu} détectée avec ${size(h.ram)} de mémoire unifiée`;
    const gpus = h.gpus ?? [];
    const names = [...new Set(gpus.map((g) => g.name))];
    const label = names.length === 1 && gpus.length > 1 ? `${gpus.length} × ${names[0]}` : names.join(" + ");
    const vram = gpus.reduce((sum, g) => sum + g.vram, 0);
    return `GPU ${label} détecté${gpus.length > 1 ? "s" : ""} avec ${size(vram)} de VRAM`;
  }

  function prompt(id: string) {
    return PROMPT_PRESETS.find((p) => p.id === id)?.prompt ?? "";
  }

  // Both profiles written, then each model fetched in the background: the
  // Profile step shows how far the download is.
  async function apply() {
    busy = true;
    error = null;
    try {
      const p = await Advisor.ApplyPreset(prompt("review"), prompt("unit-tests"));
      for (const model of new Set([p.day.model, p.night.model])) {
        if ((await Engine.Presence(p.engine as Kind, model)) !== "ready") Engine.Download(p.engine as Kind, model);
      }
      await reload();
      onapplied(null);
    } catch (err) {
      error = err instanceof Error ? err.message : String(err);
    } finally {
      busy = false;
    }
  }
</script>

{#snippet slot(title: string, s: Slot, footer: string)}
  <FormGroup {title} {footer} class="mt-6">
    <FormRow label="Modèle">
      <span class="p-form-row-detail font-mono select-text">{s.model}</span>
    </FormRow>
    <FormRow label="Jobs en parallèle" detail={String(s.jobs)} />
    <FormRow label="Contexte par job" detail="{tokens(s.contextPerJob)} tokens" />
    <FormRow
      label="Contexte alloué"
      detail="{tokens(s.contextPerJob * s.jobs)} tokens{s.weights ? ` · poids ${size(s.weights)}` : ''}"
    />
    {#if s.tokensPerSecond}
      <FormRow label="Vitesse estimée" detail="~{Math.round(s.tokensPerSecond)} tok/s par job" />
    {/if}
  </FormGroup>
{/snippet}

{#if manual || (onboarding && !onboarding.preset)}
  <Machine {onapplied} />
{:else if !onboarding}
  <p class="p-subhead-2 mt-6 flex items-center gap-1.5 text-secondary-label">
    <Spinner small label="Détection du matériel" />
    Détection du matériel…
  </p>
{:else if onboarding.preset}
  {@const h = onboarding.hardware}
  {@const p = onboarding.preset}
  {@const tooBig = Math.max(p.day.weights, p.night.weights) > p.budget}
  <div class="p-form-group flex items-center gap-3 px-4 py-3">
    <CpuIcon size={28} class="shrink-0 text-accent" aria-hidden="true" />
    <div class="min-w-0">
      <p class="p-headline-2">{detected(onboarding)}</p>
      <p class="p-subhead-2 text-secondary-label">
        {h.model ? `${h.model} · ` : ""}{h.unified ? `${h.cores} cœurs` : h.cpu} · préréglage {p.tier}
      </p>
    </div>
  </div>

  <FormGroup title="Mémoire" class="mt-6">
    <FormRow label={h.unified ? "Réservée à macOS" : "Réservée au pilote"} detail={size(p.reserved)} />
    <FormRow label={h.unified ? "Pour les modèles" : "VRAM pour les modèles"} detail={size(p.budget)} />
    <div class="p-form-row gap-1.5">
      <span class="p-form-row-label">Moteur</span>
      {#if onboarding.engine.installed}
        <CheckCircleIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-green)" aria-hidden="true" />
        <span class="text-secondary-label">{onboarding.engine.label}</span>
      {:else}
        <WarningCircleIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-orange)" aria-hidden="true" />
        <span class="text-secondary-label">
          {onboarding.engine.label}, pas installé :
          <code class="font-mono text-label select-text">{onboarding.engine.install}</code>
        </span>
      {/if}
    </div>
  </FormGroup>

  {@render slot("Profil Jour", p.day, "Relit les merge requests du lundi au vendredi, de 8 h 30 à 17 h.")}
  {@render slot("Profil Nuit", p.night, "Écrit les tests des issues étiquetées mihari::tests, le soir, la nuit et le week-end.")}

  {#if tooBig || error}
    <p class="p-subhead-2 mt-4 flex items-center gap-1.5">
      <WarningCircleIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-red)" aria-hidden="true" />
      <span class="text-label select-text">{error ?? "Le modèle pèse plus que la mémoire laissée aux modèles : personnalise plutôt."}</span>
    </p>
  {/if}

  <div class="mt-6 flex items-center justify-end gap-2">
    {#if busy}<Spinner small label="Application" />{/if}
    <Button onclick={() => (manual = true)}>Personnaliser manuellement</Button>
    <Button variant="primary" disabled={busy} onclick={apply}>Appliquer et télécharger le modèle recommandé</Button>
  </div>
{/if}
