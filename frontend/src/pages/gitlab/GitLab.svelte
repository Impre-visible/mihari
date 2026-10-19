<script lang="ts">
  import { untrack } from "svelte";
  import { PlugsConnectedIcon, WarningCircleIcon } from "phosphor-svelte";
  import { ListHeading, ListRow, SearchField, Spinner, SplitView, Switch } from "$lib/ui";
  import { live } from "$lib/live.svelte";
  import Connection from "./Connection.svelte";
  import ProjectInspector from "./ProjectInspector.svelte";
  import { changed, gitlab, load } from "./store.svelte";

  // The connection, then the bot's projects, each switched on or off from
  // the list; the one picked on the right. Every change saves itself.
  load();

  let selected = $state<number | "connection" | null>(null);
  let query = $state("");
  let notConfigured = $derived(live.tasks?.notConfigured ?? false);
  let showConnection = $derived(selected === "connection" || notConfigured || (gitlab.loaded && gitlab.projects.length === 0));
  let current = $derived(gitlab.projects.find((p) => p.id === selected) ?? null);
  let visible = $derived(
    query.trim() === "" ? gitlab.projects : gitlab.projects.filter((p) => p.path.toLowerCase().includes(query.trim().toLowerCase())),
  );
  let on = $derived(gitlab.projects.filter((p) => p.settings.enabled).length);

  let connection = $derived.by(() => {
    const t = live.tasks;
    if (!t || t.polledAt.startsWith("0001")) return { alert: false, text: "Vérification…" };
    if (t.notConfigured) return { alert: true, text: "Pas configurée" };
    if (t.error) return { alert: true, text: t.error };
    return { alert: false, text: "Connectée" };
  });

  // Saves after an edit of the projects, not on loading.
  let seen: string | null = null;
  $effect(() => {
    const now = JSON.stringify([gitlab.projects.map((p) => p.settings), gitlab.defaultImage]);
    if (!gitlab.loaded) return;
    if (seen !== null && seen !== now) untrack(changed);
    seen = now;
  });
</script>

<SplitView width={300}>
  {#snippet list()}
    <ListRow title="Connexion" subtitle={connection.text} alert={connection.alert} selected={showConnection} onclick={() => (selected = "connection")}>
      {#snippet leading()}
        {#if connection.alert}
          <WarningCircleIcon size={16} weight="fill" class="text-(--p-sys-red)" aria-hidden="true" />
        {:else}
          <PlugsConnectedIcon size={16} class="text-secondary-label" aria-hidden="true" />
        {/if}
      {/snippet}
    </ListRow>

    <ListHeading title="Projets · {on} sur {gitlab.projects.length}" />
    {#if gitlab.projects.length > 6}
      <div class="px-1.5 pb-1"><SearchField placeholder="Filtrer" bind:value={query} /></div>
    {/if}
    {#if gitlab.error}
      <p class="p-subhead-2 px-2.5 py-1 text-(--p-sys-red) select-text">{gitlab.error}</p>
    {:else if !gitlab.loaded}
      <div class="px-2.5 py-2"><Spinner small label="Chargement des projets" /></div>
    {/if}
    {#each visible as p (p.id)}
      <ListRow
        title={p.name || p.path}
        subtitle={!p.accessible ? "inaccessible au bot" : p.settings.enabled ? p.path : `${p.path} · éteint`}
        alert={!p.accessible}
        selected={!showConnection && current?.id === p.id}
        onclick={() => (selected = p.id)}
      >
        {#snippet trailing()}
          <Switch label="Le bot travaille sur {p.path}" bind:checked={p.settings.enabled} />
        {/snippet}
      </ListRow>
    {:else}
      {#if gitlab.loaded}
        <p class="p-subhead-2 px-2.5 py-1 text-tertiary-label">{gitlab.projects.length ? "Aucun projet ne correspond." : "Le bot n'est membre d'aucun projet."}</p>
      {/if}
    {/each}
  {/snippet}

  {#if showConnection || !current}
    <Connection onsaved={load} />
  {:else}
    {#key current.id}<ProjectInspector project={current} />{/key}
  {/if}
</SplitView>
