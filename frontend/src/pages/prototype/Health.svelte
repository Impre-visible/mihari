<script lang="ts">
  // PROTOTYPE (UI redesign): engine, GitLab and containers as three pills.
  import { Spinner } from "$lib/ui";
  import { ago, live } from "./data.svelte";

  type Props = { vertical?: boolean };
  let { vertical = false }: Props = $props();

  type Tone = "ok" | "warn" | "bad" | "wait";
  let items = $derived.by(() => {
    const e = live.engine, t = live.tasks, s = live.sandbox;
    const engine: [Tone, string, string] =
      !e || e.state === "idle"
        ? ["warn", "Moteur en pause", "Aucun profil prévu"]
        : e.state === "ready"
          ? ["ok", e.profileName, e.model]
          : e.state === "starting"
            ? ["wait", "Chargement…", e.model]
            : ["bad", "Moteur en erreur", e.error];
    const gitlab: [Tone, string, string] =
      !t || t.polledAt.startsWith("0001")
        ? ["wait", "GitLab", "Première vérification…"]
        : t.notConfigured
          ? ["warn", "GitLab", "Pas configuré"]
          : t.error
            ? ["bad", "GitLab", t.error]
            : ["ok", "GitLab", `Vérifié ${ago(t.polledAt, live.now)}`];
    const box: [Tone, string, string] = !s
      ? ["wait", "Containers", "Vérification…"]
      : s.ready
        ? ["ok", s.label, "Prêt"]
        : ["bad", s.label, s.error];
    return [engine, gitlab, box];
  });

  const DOT: Record<Tone, string> = {
    ok: "bg-(--p-sys-green)",
    warn: "bg-(--p-sys-orange)",
    bad: "bg-(--p-sys-red)",
    wait: "",
  };
</script>

<div class={["flex gap-2", vertical ? "flex-col" : "flex-wrap"]}>
  {#each items as [tone, title, detail], i (i)}
    <div class="flex min-w-0 items-center gap-2 rounded-lg bg-fill px-2.5 py-1.5" title={detail}>
      {#if tone === "wait"}<Spinner small label={detail} />{:else}<span class={["size-2 shrink-0 rounded-full", DOT[tone]]}></span>{/if}
      <span class="p-subhead-2 shrink-0 font-semibold">{title}</span>
      <span class="p-subhead-2 min-w-0 truncate text-secondary-label">{detail}</span>
    </div>
  {/each}
</div>
