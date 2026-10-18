<script lang="ts">
  import { Service as Desktop } from "../../../bindings/github.com/Impre-visible/mihari/internal/desktop";
  import { Events } from "@wailsio/runtime";
  import { Service as Updates } from "../../../bindings/github.com/Impre-visible/mihari/internal/updates";
  import type { Info } from "../../../bindings/github.com/Impre-visible/mihari/internal/updates/models";
  import { Button, FormGroup, FormRow, Switch } from "$lib/ui";
  import { nav } from "$lib/nav.svelte";

  let autostart = $state(true);
  let devBuild = $state(false);
  let error = $state<string | null>(null);

  Promise.all([Desktop.Autostart(), Desktop.DevBuild()])
    .then(([enabled, dev]) => {
      autostart = enabled;
      devBuild = dev;
    })
    .catch((err) => console.error("Load general settings:", err));

  let updates = $state<Info | null>(null);
  Updates.Status().then((i) => (updates = i));
  $effect(() => Events.On("updates:changed", (event) => (updates = event.data)));

  function checked(i: Info) {
    if (i.devBuild) return "Build de développement : pas de mise à jour.";
    if (i.error) return i.error;
    if (i.checkedAt.startsWith("0001")) return "Pas encore vérifié.";
    const at = new Date(i.checkedAt).toLocaleString("fr-FR", { dateStyle: "short", timeStyle: "short" });
    return i.available ? `Version ${i.version} disponible (vérifié le ${at}).` : `À jour (vérifié le ${at}).`;
  }

  async function toggle() {
    error = null;
    try {
      await Desktop.SetAutostart(autostart);
    } catch (err) {
      error = err instanceof Error ? err.message : String(err);
      autostart = !autostart;
    }
  }
</script>

<p class="p-body text-secondary-label">Comment Mihari tourne sur cette machine.</p>

<FormGroup
  title="Arrière-plan"
  class="mt-6"
  footer={devBuild
    ? "Build de développement : le lancement à l'ouverture de session n'est jamais enregistré."
    : "Fermer la fenêtre laisse Mihari tourner dans la barre de menus. Quitter se fait depuis son icône."}
>
  <FormRow label="Lancer Mihari à l'ouverture de session" for="general-autostart">
    <Switch id="general-autostart" label="Lancer Mihari à l'ouverture de session" bind:checked={autostart} onchange={toggle} />
  </FormRow>
  {#if error}
    <p class="p-form-row p-subhead-2 py-2.5 text-label select-text">{error}</p>
  {/if}
</FormGroup>

<FormGroup title="Mises à jour" footer="Mihari vérifie les nouvelles versions au démarrage puis toutes les 6 heures.">
  <FormRow label="Version" detail={updates?.current ?? ""} />
  <div class="p-form-row gap-2">
    <span class="p-form-row-label p-subhead-2 text-secondary-label">{updates ? checked(updates) : "…"}</span>
    <Button size="small" disabled={!updates || updates.devBuild} onclick={() => Updates.CheckNow()}>Vérifier maintenant</Button>
  </div>
</FormGroup>

<FormGroup title="Configuration">
  <FormRow label="Assistant de configuration" detail="GitLab, projets, modèle, profil et planning, pas à pas.">
    <Button size="small" onclick={() => (nav.wizard = true)}>Relancer</Button>
  </FormRow>
</FormGroup>
