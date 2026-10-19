<script lang="ts">
  import { CheckCircleIcon, WarningCircleIcon } from "phosphor-svelte";
  import { Service as GitLab } from "../../../bindings/github.com/Impre-visible/mihari/internal/gitlab";
  import { Service as Settings } from "../../../bindings/github.com/Impre-visible/mihari/internal/settings";
  import { Service as Tasks } from "../../../bindings/github.com/Impre-visible/mihari/internal/tasks";
  import { Button, FormGroup, FormRow, Spinner, TextField } from "$lib/ui";
  import { gitlab } from "./store.svelte";

  // The instance, the bot's account and its token. The token is a secret, so
  // this saves on purpose, after a test, rather than as it is typed.
  type Props = { onsaved?: () => void };
  let { onsaved }: Props = $props();

  type Status = { kind: "idle" } | { kind: "busy"; label: string } | { kind: "ok"; message: string } | { kind: "error"; message: string };

  let url = $state("");
  let username = $state("");
  // Empty keeps the stored token, which never leaves the keyring.
  let token = $state("");
  let stored = $state(false);
  let saved = $state({ url: "", username: "" });
  let status = $state<Status>({ kind: "idle" });

  let busy = $derived(status.kind === "busy");
  let dirty = $derived(url.trim() !== saved.url || username.trim() !== saved.username || token !== "");
  let complete = $derived(url.trim() !== "" && username.trim() !== "" && (token !== "" || stored));

  // Not Settings.Get: Wails serializes a nil result as {}, not null.
  Promise.all([Settings.GetConfig(), Settings.HasGitLabToken()])
    .then(([config, hasToken]) => {
      url = typeof config?.gitlab_url === "string" ? config.gitlab_url : "";
      username = typeof config?.gitlab_username === "string" ? config.gitlab_username : "";
      saved = { url, username };
      stored = hasToken;
    })
    .catch((err) => console.error("Load GitLab settings:", err));

  function message(err: unknown) {
    return err instanceof Error ? err.message : String(err);
  }

  // Tests the values, and saves them only when GitLab accepts them.
  async function connect() {
    status = { kind: "busy", label: "Test de la connexion…" };
    try {
      const name = await GitLab.TestConnection(url.trim(), username.trim(), token);
      status = { kind: "busy", label: "Enregistrement…" };
      await Settings.Set("gitlab_url", url.trim());
      await Settings.Set("gitlab_username", username.trim());
      if (token !== "") {
        await Settings.SetGitLabToken(token);
        token = "";
        stored = true;
      }
      saved = { url: url.trim(), username: username.trim() };
      Tasks.PollNow();
      status = { kind: "ok", message: `Connecté en tant que ${name} (@${username.trim()}).` };
      onsaved?.();
    } catch (err) {
      status = { kind: "error", message: message(err) };
    }
  }

  async function test() {
    status = { kind: "busy", label: "Test de la connexion…" };
    try {
      const name = await GitLab.TestConnection(url.trim(), username.trim(), token);
      status = { kind: "ok", message: `Connexion OK : ${name} (@${username.trim()}).` };
    } catch (err) {
      status = { kind: "error", message: message(err) };
    }
  }
</script>

<header class="mb-3">
  <h2 class="p-title-2 font-semibold">Connexion</h2>
  <p class="p-subhead-2 text-secondary-label">L'instance GitLab, le compte du bot et son token d'accès (scope api).</p>
</header>

<form
  oninput={() => !busy && (status = { kind: "idle" })}
  onsubmit={(e) => {
    e.preventDefault();
    if (complete && !busy) connect();
  }}
>
  <FormGroup>
    <FormRow label="URL de l'instance" for="gl-url">
      <TextField id="gl-url" type="url" class="w-72" placeholder="https://gitlab.example.com" bind:value={url} />
    </FormRow>
    <FormRow label="Compte bot" for="gl-user">
      <TextField id="gl-user" class="w-72" placeholder="mihari-bot" bind:value={username} />
    </FormRow>
    <FormRow label="Token d'accès" for="gl-token">
      <TextField id="gl-token" type="password" class="w-72" placeholder={stored ? "Enregistré (vide pour le garder)" : "glpat-…"} bind:value={token} />
    </FormRow>
  </FormGroup>

  <div class="mt-3 flex items-center gap-2">
    <p class="p-subhead-2 flex min-w-0 flex-1 items-center gap-1.5" role="status">
      {#if status.kind === "busy"}
        <Spinner small label={status.label} /><span class="text-secondary-label">{status.label}</span>
      {:else if status.kind === "ok"}
        <CheckCircleIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-green)" aria-hidden="true" />
        <span class="text-secondary-label">{status.message}</span>
      {:else if status.kind === "error"}
        <WarningCircleIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-red)" aria-hidden="true" />
        <span class="text-label select-text">{status.message}</span>
      {/if}
    </p>
    <Button disabled={!complete || busy} onclick={test}>Tester</Button>
    <Button type="submit" variant="primary" disabled={!complete || busy || !dirty}>Connecter</Button>
  </div>
</form>

<FormGroup title="Containers des jobs" class="mt-6" footer="Un projet sans image propre, ni job de test dans sa CI, fait tourner ses jobs dans celle-ci.">
  <FormRow label="Image par défaut" for="gl-image">
    <TextField id="gl-image" class="w-72 font-mono" placeholder="debian:stable-slim" bind:value={gitlab.defaultImage} />
  </FormRow>
</FormGroup>
