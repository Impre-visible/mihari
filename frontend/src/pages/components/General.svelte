<script lang="ts">
  import { Service as Desktop } from "../../../bindings/github.com/Impre-visible/mihari/internal/desktop";
  import { Events } from "@wailsio/runtime";
  import { Service as Updates } from "../../../bindings/github.com/Impre-visible/mihari/internal/updates";
  import type { Info } from "../../../bindings/github.com/Impre-visible/mihari/internal/updates/models";
  import { CheckCircleIcon, WarningCircleIcon } from "phosphor-svelte";
  import { Service as Discord } from "../../../bindings/github.com/Impre-visible/mihari/internal/discord";
  import { Service as Settings } from "../../../bindings/github.com/Impre-visible/mihari/internal/settings";
  import { Button, FormGroup, FormRow, Spinner, Switch, TextField } from "$lib/ui";
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

  // Web search: a SearXNG instance, DuckDuckGo without one.

  let searxng = $state("");
  let searxngSaved = $state("");
  let searxngError = $state<string | null>(null);

  // Not Settings.Get: Wails serializes a nil result as {}, not null.
  Settings.GetConfig().then((config) => {
    searxng = searxngSaved = typeof config?.searxng_url === "string" ? config.searxng_url : "";
  });

  async function saveSearxng() {
    searxngError = null;
    const value = searxng.trim();
    if (value && !/^https?:\/\/[^/]+/.test(value)) {
      searxngError = "Adresse http(s) attendue, par exemple http://searxng.lan:8080.";
      return;
    }
    try {
      await Settings.Set("searxng_url", value);
      searxng = searxngSaved = value;
    } catch (err) {
      searxngError = messageOf(err);
    }
  }

  // Discord

  type Status =
    | { kind: "idle" }
    | { kind: "busy"; label: string }
    | { kind: "ok"; message: string }
    | { kind: "error"; message: string };

  // Shown in place of the saved webhook, which never leaves the keyring.
  const MASK = "*".repeat(32);
  let webhook = $state("");
  let discordStatus = $state<Status>({ kind: "idle" });

  Discord.Configured().then((configured) => {
    if (configured) webhook = MASK;
  });

  function messageOf(err: unknown) {
    return err instanceof Error ? err.message : String(err);
  }

  async function discordAction(label: string, run: () => Promise<void>, done: string) {
    discordStatus = { kind: "busy", label };
    try {
      await run();
      discordStatus = { kind: "ok", message: done };
    } catch (err) {
      discordStatus = { kind: "error", message: messageOf(err) };
    }
  }

  const testDiscord = () =>
    discordAction("Envoi d'un message de test…", () => Discord.Test(webhook === MASK ? "" : webhook), "Message envoyé, regarde le salon.");

  const saveDiscord = () =>
    discordAction(
      "Enregistrement…",
      async () => {
        await Discord.SetWebhook(webhook);
        webhook = MASK;
      },
      "Webhook enregistré.",
    );

  const removeDiscord = () =>
    discordAction(
      "Suppression…",
      async () => {
        await Discord.SetWebhook("");
        webhook = "";
      },
      "Notifications Discord désactivées.",
    );

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

<FormGroup
  title="Recherche web"
  footer="L'agent cherche sur DuckDuckGo depuis le container du job. Avec une instance SearXNG (format json activé), il l'utilise à la place : plus fiable. Elle doit être joignable depuis les containers."
>
  <FormRow label="Instance SearXNG" for="general-searxng">
    <div class="flex items-center gap-2">
      <TextField id="general-searxng" class="w-60" placeholder="http://searxng.lan:8080" bind:value={searxng} />
      <Button size="small" disabled={searxng.trim() === searxngSaved} onclick={saveSearxng}>Enregistrer</Button>
    </div>
  </FormRow>
  {#if searxngError}
    <p class="p-form-row p-subhead-2 py-2.5 text-label select-text">{searxngError}</p>
  {/if}
</FormGroup>

<FormGroup
  title="Discord"
  footer="Envoie dans un salon les jobs en échec, les erreurs du moteur, les MR ouvertes par le bot et le résumé de fin de nuit. L'URL est rangée dans le trousseau du système."
>
  <FormRow label="Webhook" for="general-discord">
    <TextField
      id="general-discord"
      type="password"
      class="w-72"
      placeholder="https://discord.com/api/webhooks/…"
      bind:value={webhook}
      oninput={() => discordStatus.kind !== "busy" && (discordStatus = { kind: "idle" })}
    />
  </FormRow>
</FormGroup>

<div class="mt-4 flex items-center gap-2">
  <p class="p-subhead-2 flex min-w-0 flex-1 items-center gap-1.5" role="status">
    {#if discordStatus.kind === "busy"}
      <Spinner small label={discordStatus.label} />
      <span class="text-secondary-label">{discordStatus.label}</span>
    {:else if discordStatus.kind === "ok"}
      <CheckCircleIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-green)" aria-hidden="true" />
      <span class="text-secondary-label">{discordStatus.message}</span>
    {:else if discordStatus.kind === "error"}
      <WarningCircleIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-red)" aria-hidden="true" />
      <span class="text-label select-text">{discordStatus.message}</span>
    {/if}
  </p>
  {#if webhook === MASK}
    <Button variant="borderless" disabled={discordStatus.kind === "busy"} onclick={removeDiscord}>Retirer</Button>
  {/if}
  <Button disabled={webhook === "" || discordStatus.kind === "busy"} onclick={testDiscord}>Tester</Button>
  <Button
    variant="primary"
    disabled={webhook === "" || webhook === MASK || discordStatus.kind === "busy"}
    onclick={saveDiscord}
  >
    Enregistrer
  </Button>
</div>
