<script lang="ts">
  import { Service as Scheduler } from "../../../bindings/github.com/Impre-visible/mihari/internal/scheduler";
  import { Service as Tasks } from "../../../bindings/github.com/Impre-visible/mihari/internal/tasks";
  import { Button, Spinner } from "$lib/ui";
  import { ago, checkSandbox, live } from "$lib/live.svelte";
  import { nav } from "$lib/nav.svelte";

  // The engine, GitLab and the containers, one line each, with what to do
  // when one goes wrong.
  type Tone = "ok" | "warn" | "bad" | "wait";
  type Item = { tone: Tone; title: string; detail: string; action?: { label: string; run: () => void } };

  let items = $derived.by((): Item[] => {
    const e = live.engine, t = live.tasks, s = live.sandbox;
    const engine: Item =
      !e || e.state === "idle"
        ? { tone: "warn", title: "Moteur en pause", detail: "Aucun profil prévu maintenant", action: { label: "Planning", run: () => (nav.section = "agent") } }
        : e.state === "ready"
          ? { tone: "ok", title: e.profileName, detail: e.model }
          : e.state === "starting"
            ? { tone: "wait", title: e.profileName, detail: "Chargement du modèle…" }
            : { tone: "bad", title: "Moteur en erreur", detail: e.error, action: { label: "Relancer", run: () => Scheduler.Reload() } };
    const gitlab: Item =
      !t || t.polledAt.startsWith("0001")
        ? { tone: "wait", title: "GitLab", detail: "Première vérification…" }
        : t.notConfigured
          ? { tone: "warn", title: "GitLab", detail: "Pas configuré", action: { label: "Configurer", run: () => (nav.section = "gitlab") } }
          : t.error
            ? { tone: "bad", title: "GitLab", detail: t.error, action: { label: "Revérifier", run: () => Tasks.PollNow() } }
            : { tone: "ok", title: "GitLab", detail: `Vérifié ${ago(t.polledAt, live.now)}` };
    const box: Item = !s
      ? { tone: "wait", title: "Containers", detail: "Vérification…" }
      : s.ready
        ? { tone: "ok", title: s.label, detail: "Prêt" }
        : { tone: "bad", title: s.label, detail: s.installed ? s.error : `${s.error} ${s.install}`, action: { label: "Revérifier", run: checkSandbox } };
    return [engine, gitlab, box];
  });

  const DOT: Record<Tone, string> = { ok: "bg-(--p-sys-green)", warn: "bg-(--p-sys-orange)", bad: "bg-(--p-sys-red)", wait: "" };
</script>

<ul class="flex flex-col gap-1.5">
  {#each items as item, i (i)}
    <li class={["flex min-w-0 gap-2", item.tone === "bad" ? "items-start" : "items-center"]} title={item.detail}>
      {#if item.tone === "wait"}
        <Spinner small label={item.detail} />
      {:else}
        <span class={["ms-0.5 me-0.5 size-2 shrink-0 rounded-full", item.tone === "bad" && "mt-1.5", DOT[item.tone]]} aria-hidden="true"></span>
      {/if}
      <span class="p-subhead-2 shrink-0 font-semibold">{item.title}</span>
      <!-- An error is read in full: it says what to fix. -->
      <span class={["p-subhead-2 min-w-0 flex-1", item.tone === "bad" ? "break-words text-(--p-sys-red) select-text" : "truncate text-secondary-label"]}>{item.detail}</span>
      {#if item.action}
        <Button size="small" variant="borderless" onclick={item.action.run}>{item.action.label}</Button>
      {/if}
    </li>
  {/each}
</ul>
