<script lang="ts">
  import { ArrowLeftIcon, ArrowSquareOutIcon, CheckCircleIcon, WarningCircleIcon } from "phosphor-svelte";
  import { Browser, Events } from "@wailsio/runtime";
  import { Service as Jobs } from "../../../bindings/github.com/Impre-visible/mihari/internal/jobs";
  import type { Bench, BenchRun } from "../../../bindings/github.com/Impre-visible/mihari/internal/jobs/models";
  import { Service as Settings } from "../../../bindings/github.com/Impre-visible/mihari/internal/settings";
  import { Button, Checkbox, FormGroup, FormRow, Spinner, TextField } from "$lib/ui";

  const RUN_STATES: Record<string, string> = {
    pending: "en attente",
    loading: "chargement du modèle",
    running: "review en cours",
    done: "terminé",
    failed: "échec",
  };
  const KINDS: Record<string, string> = { comment: "Commentaire", suggestion: "Suggestion", reply: "Réponse" };

  let mrURL = $state("");
  let profiles = $state<{ id: string; name: string; model: string }[]>([]);
  let chosen = $state<Record<string, boolean>>({});
  let error = $state<string | null>(null);
  let benches = $state<Bench[]>([]);
  // The bench shown in detail; the running one updates live.
  let open = $state<Bench | null>(null);

  let running = $derived(benches.some((b) => b.running));
  let selected = $derived(profiles.filter((p) => chosen[p.id]).map((p) => p.id));

  Settings.GetConfig().then((config) => {
    const list = Array.isArray(config?.profiles) ? config.profiles : [];
    profiles = list
      .filter((p: Record<string, unknown>) => typeof p?.id === "string")
      .map((p: Record<string, unknown>) => ({ id: p.id as string, name: String(p.name ?? ""), model: String(p.model ?? "") }));
  });
  refresh();

  function refresh() {
    Jobs.Benches().then((list) => (benches = list ?? []));
  }

  $effect(() =>
    Events.On("bench:updated", (event) => {
      const b = event.data;
      const i = benches.findIndex((x) => x.id === b.id);
      if (i >= 0) benches[i] = b;
      else benches = [b, ...benches];
      // Keep the detail live, with transcripts fetched once a run ends.
      if (open?.id === b.id) Jobs.BenchDetail(b.id).then((full) => (open = full));
    }),
  );

  async function start() {
    error = null;
    try {
      const id = await Jobs.StartBench(mrURL.trim(), selected);
      open = await Jobs.BenchDetail(id);
    } catch (err) {
      error = err instanceof Error ? err.message : String(err);
    }
  }

  async function show(b: Bench) {
    open = await Jobs.BenchDetail(b.id);
  }

  function seconds(ms: number) {
    return ms ? `${(ms / 1000).toFixed(1).replace(".", ",")} s` : "–";
  }

  function count(n: number) {
    return n.toLocaleString("fr-FR");
  }

  function date(at: string) {
    return new Date(at).toLocaleString("fr-FR", { dateStyle: "short", timeStyle: "short" });
  }
</script>

{#snippet runState(r: BenchRun)}
  {#if r.state === "loading" || r.state === "running"}
    <Spinner small label={RUN_STATES[r.state]} />
  {:else if r.state === "done"}
    <CheckCircleIcon size={14} weight="fill" class="shrink-0 text-(--p-sys-green)" aria-hidden="true" />
  {:else if r.state === "failed"}
    <WarningCircleIcon size={14} weight="fill" class="shrink-0 text-(--p-sys-red)" aria-hidden="true" />
  {/if}
  <span>{RUN_STATES[r.state] ?? r.state}</span>
{/snippet}

{#if open}
  {@const b = open}
  <div class="flex items-center gap-2">
    <Button size="small" variant="borderless" onclick={() => (open = null)}>
      <ArrowLeftIcon size={14} aria-hidden="true" />
      Bancs
    </Button>
    <span class="min-w-0 flex-1 truncate">{b.mr} · {b.title}</span>
    <Button size="small" variant="borderless" onclick={() => Browser.OpenURL(b.url)}>
      <ArrowSquareOutIcon size={14} aria-hidden="true" />
      MR
    </Button>
    {#if b.running}
      <Button size="small" onclick={() => Jobs.StopBench()}>Arrêter</Button>
    {/if}
  </div>
  <p class="p-subhead-2 mt-1 ms-2.5 text-secondary-label">
    {date(b.startedAt)} · {(b.runs ?? []).length} profils · rien n'a été posté sur GitLab
  </p>

  <div class="mt-4 overflow-x-auto pb-2">
  <div class="grid gap-4" style:grid-template-columns="repeat({Math.max(1, (b.runs ?? []).length)}, minmax(260px, 1fr))">
    {#each b.runs ?? [] as r (r.profileId)}
      <FormGroup title={r.profileName}>
        <FormRow label="Modèle" detail={r.model} />
        <div class="p-form-row gap-1.5 p-subhead-2 text-secondary-label">{@render runState(r)}</div>
        <FormRow label="Chargement" detail={seconds(r.loadMs)} />
        <FormRow label="Review" detail={seconds(r.durationMs)} />
        <FormRow label="Tours" detail={String(r.turns)} />
        <FormRow label="Tokens lus / écrits" detail="{count(r.promptTokens)} / {count(r.completionTokens)}" />
        <FormRow label="Remarques" detail={String((r.posts ?? []).filter((p) => p.kind !== "reply").length)} />
        {#if r.error}
          <p class="p-form-row p-subhead-2 py-2 text-label select-text">{r.error}</p>
        {/if}
        {#each r.posts ?? [] as post, i (i)}
          <div class="p-form-row flex-col items-stretch! gap-1 py-2">
            <span class="p-subhead-2 font-semibold text-secondary-label">
              {KINDS[post.kind] ?? post.kind}{post.path ? ` · ${post.path}:${post.line}` : ""}
            </span>
            <pre class="p-subhead-2 font-mono whitespace-pre-wrap select-text">{post.body}</pre>
          </div>
        {/each}
        {#if r.summary}
          <p class="p-form-row p-subhead-2 py-2 text-secondary-label select-text">{r.summary}</p>
        {/if}
      </FormGroup>
    {/each}
  </div>
  </div>
{:else}
  <p class="p-body text-secondary-label">
    Plusieurs profils relisent la même MR l'un après l'autre, sans rien poster. Les jobs attendent la fin du banc.
  </p>

  <form
    class="mt-6"
    onsubmit={(e) => {
      e.preventDefault();
      start();
    }}
  >
    <FormGroup title="Nouveau banc">
      <FormRow label="URL de la MR" for="bench-mr">
        <TextField
          id="bench-mr"
          type="url"
          class="w-96"
          placeholder="https://gitlab.com/groupe/projet/-/merge_requests/1"
          bind:value={mrURL}
        />
      </FormRow>
      {#each profiles as p (p.id)}
        <FormRow label={p.name} detail={p.model}>
          <Checkbox aria-label="Inclure {p.name}" bind:checked={chosen[p.id]} />
        </FormRow>
      {:else}
        <p class="p-form-row p-subhead-2 py-2.5 text-tertiary-label">Aucun profil : crée-en dans Profils.</p>
      {/each}
    </FormGroup>
    <div class="mt-4 flex items-center gap-2">
      <p class="p-subhead-2 flex min-w-0 flex-1 items-center gap-1.5" role="status">
        {#if error}
          <WarningCircleIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-red)" aria-hidden="true" />
          <span class="text-label select-text">{error}</span>
        {/if}
      </p>
      <Button type="submit" variant="primary" disabled={running || mrURL.trim() === "" || selected.length === 0}>
        Lancer le banc
      </Button>
    </div>
  </form>

  {#if benches.length > 0}
    <FormGroup title="Bancs précédents" class="mt-6">
      {#each benches as b (b.id)}
        <FormRow
          label="{b.mr} · {b.title}"
          detail="{date(b.startedAt)} · {(b.runs ?? []).map((r) => r.profileName).join(', ')}{b.running ? ' · en cours' : ''}"
          chevron
          onclick={() => show(b)}
        />
      {/each}
    </FormGroup>
  {/if}
{/if}
