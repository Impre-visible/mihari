<script lang="ts">
  import { CheckCircleIcon, WarningCircleIcon } from "phosphor-svelte";
  import { Service as GitLab } from "../../../bindings/github.com/Impre-visible/mihari/internal/gitlab";
  import { Service as Settings } from "../../../bindings/github.com/Impre-visible/mihari/internal/settings";
  import { Button, FormGroup, FormRow, Spinner, TextField } from "$lib/ui";

  type Status =
    | { kind: "idle" }
    | { kind: "busy"; label: string }
    | { kind: "ok"; message: string }
    | { kind: "error"; message: string };

  // Shown in place of the stored token, which never leaves the keyring.
  const MASK = "*".repeat(32);

  let url = $state("");
  let username = $state("");
  let token = $state("");
  let status = $state<Status>({ kind: "idle" });

  let busy = $derived(status.kind === "busy");
  let canSubmit = $derived(url.trim() !== "" && username.trim() !== "" && token !== "");

  // Not Settings.Get: Wails serializes a nil result as {}, not null, so a missing key would show "[object Object]".
  Promise.all([Settings.GetConfig(), Settings.HasGitLabToken()])
    .then(([config, storedToken]) => {
      url = typeof config?.gitlab_url === "string" ? config.gitlab_url : "";
      username = typeof config?.gitlab_username === "string" ? config.gitlab_username : "";
      if (storedToken) token = MASK;
    })
    .catch((err) => console.error("Load GitLab settings:", err));

  function errorMessage(err: unknown) {
    return err instanceof Error ? err.message : String(err);
  }

  async function test() {
    status = { kind: "busy", label: "Test de la connexion…" };
    try {
      const name = await GitLab.TestConnection(url.trim(), username.trim(), token === MASK ? "" : token);
      status = { kind: "ok", message: `Connecté en tant que ${name} (@${username.trim()}).` };
    } catch (err) {
      status = { kind: "error", message: errorMessage(err) };
    }
  }

  async function save() {
    status = { kind: "busy", label: "Enregistrement…" };
    try {
      await Settings.Set("gitlab_url", url.trim());
      await Settings.Set("gitlab_username", username.trim());
      if (token !== MASK) {
        await Settings.SetGitLabToken(token);
        token = MASK;
      }
      status = { kind: "ok", message: "Réglages enregistrés." };
    } catch (err) {
      status = { kind: "error", message: errorMessage(err) };
    }
  }

  // A result only describes the values it was obtained with.
  function resetStatus() {
    if (status.kind !== "busy") status = { kind: "idle" };
  }
</script>

<p class="p-body text-secondary-label">L'URL de l'instance, le compte bot et son token.</p>

<form
  class="mt-6"
  oninput={resetStatus}
  onsubmit={(e) => {
    e.preventDefault();
    test().then(() => {
      if (status.kind === "ok") save();
    });
  }}
>
  <FormGroup
    title="Connexion"
  >
    <FormRow label="URL de l'instance" for="gitlab-url">
      <TextField id="gitlab-url" type="url" class="w-72" placeholder="https://gitlab.example.com" bind:value={url} />
    </FormRow>
    <FormRow label="Compte bot" for="gitlab-username">
      <TextField id="gitlab-username" class="w-72" placeholder="mihari-bot" bind:value={username} />
    </FormRow>
    <FormRow label="Token d'accès" for="gitlab-token">
      <TextField
        id="gitlab-token"
        type="password"
        class="w-72"
        placeholder="glpat-…"
        bind:value={token}
      />
    </FormRow>
  </FormGroup>

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

    <Button disabled={!canSubmit || busy} onclick={test}>Tester la connexion</Button>
    <Button type="submit" variant="primary" disabled={!canSubmit || busy}>Enregistrer</Button>
  </div>
</form>
