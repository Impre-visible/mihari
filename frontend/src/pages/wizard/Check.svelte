<script lang="ts">
  import { CheckCircleIcon, WarningCircleIcon, WarningIcon } from "phosphor-svelte";
  import { Service as Scheduler } from "../../../bindings/github.com/Impre-visible/mihari/internal/scheduler";
  import { Service as Tasks } from "../../../bindings/github.com/Impre-visible/mihari/internal/tasks";
  import { Button, FormGroup, Spinner } from "$lib/ui";
  import { checkSandbox, live } from "$lib/live.svelte";
  import { agent, load as loadAgent, profileProblem } from "../agent/store.svelte";
  import { gitlab, load as loadGitLab } from "../gitlab/store.svelte";

  // The last step: what the bot needs to work, checked again, with the step
  // that fixes each one.
  let { goto }: { goto: (step: number) => void } = $props();

  Tasks.PollNow();
  checkSandbox();
  loadGitLab();
  loadAgent();

  type Tone = "ok" | "warn" | "bad" | "wait";
  type Item = { tone: Tone; title: string; detail: string; action?: { label: string; run: () => void } };

  let items = $derived.by((): Item[] => {
    const t = live.tasks, s = live.sandbox, e = live.engine;
    const on = gitlab.projects.filter((p) => p.settings.enabled).length;
    const gl: Item =
      !t || t.polledAt.startsWith("0001")
        ? { tone: "wait", title: "GitLab", detail: "Vérification…" }
        : t.notConfigured
          ? { tone: "bad", title: "GitLab", detail: "Pas connecté.", action: { label: "Connecter", run: () => goto(0) } }
          : t.error
            ? { tone: "bad", title: "GitLab", detail: t.error, action: { label: "Revérifier", run: () => Tasks.PollNow() } }
            : on === 0
              ? { tone: "warn", title: "GitLab", detail: "Connecté, mais aucun projet allumé.", action: { label: "Choisir", run: () => goto(1) } }
              : { tone: "ok", title: "GitLab", detail: `Connecté · ${on} projet${on > 1 ? "s" : ""} allumé${on > 1 ? "s" : ""}` };
    const box: Item = !s
      ? { tone: "wait", title: "Containers", detail: "Vérification…" }
      : s.ready
        ? { tone: "ok", title: "Containers", detail: `${s.label} prêt` }
        : { tone: "bad", title: "Containers", detail: s.installed ? s.error : `${s.error} ${s.install}`, action: { label: "Revérifier", run: checkSandbox } };
    const valid = agent.profiles.filter((p) => !profileProblem(p));
    const planned = agent.segments.filter((g) => valid.some((p) => p.id === g.profile)).length;
    const engine: Item = !agent.loaded
      ? { tone: "wait", title: "Agent", detail: "Chargement…" }
      : valid.length === 0
        ? { tone: "bad", title: "Agent", detail: "Aucun profil complet.", action: { label: "Choisir un modèle", run: () => goto(2) } }
        : e?.state === "error"
          ? { tone: "bad", title: "Agent", detail: e.error, action: { label: "Relancer", run: () => Scheduler.Reload() } }
          : planned === 0 && !agent.fallback
            ? { tone: "warn", title: "Agent", detail: "Aucune plage horaire : le bot ne tournera jamais.", action: { label: "Planifier", run: () => goto(3) } }
            : {
                tone: "ok",
                title: "Agent",
                detail:
                  e?.state === "ready" || e?.state === "starting"
                    ? `${e.profileName} tourne maintenant`
                    : `${valid.length} profil${valid.length > 1 ? "s" : ""}, aucun prévu maintenant`,
              };
    return [gl, box, engine];
  });

  let ready = $derived(items.every((i) => i.tone === "ok" || i.tone === "warn"));
</script>

<div class="mx-auto max-w-[560px]">
  <FormGroup
    footer={ready
      ? "Mihari tourne dans la barre de menus. Tout se règle ensuite depuis la fenêtre."
      : "Le bot ne peut pas encore travailler. Tu peux entrer quand même et finir plus tard."}
  >
    {#each items as item (item.title)}
      <div class="p-form-row gap-3 py-2.5">
        {#if item.tone === "wait"}
          <Spinner small label={item.detail} />
        {:else if item.tone === "ok"}
          <CheckCircleIcon size={20} weight="fill" class="shrink-0 text-(--p-sys-green)" aria-hidden="true" />
        {:else if item.tone === "warn"}
          <WarningIcon size={20} weight="fill" class="shrink-0 text-(--p-sys-orange)" aria-hidden="true" />
        {:else}
          <WarningCircleIcon size={20} weight="fill" class="shrink-0 text-(--p-sys-red)" aria-hidden="true" />
        {/if}
        <div class="min-w-0 flex-1">
          <p class="p-subhead font-medium">{item.title}</p>
          <p class={["p-subhead-2 select-text", item.tone === "bad" ? "text-label" : "text-secondary-label"]}>{item.detail}</p>
        </div>
        {#if item.action}
          <Button size="small" onclick={item.action.run}>{item.action.label}</Button>
        {/if}
      </div>
    {/each}
  </FormGroup>
</div>
