<script lang="ts">
  import type { Component } from "svelte";
  import { CheckIcon } from "phosphor-svelte";
  import { AppWindow, Button, Sidebar, SidebarSection } from "$lib/ui";
  import { start } from "$lib/live.svelte";
  import { subtitle } from "$lib/save.svelte";
  import Connection from "../gitlab/Connection.svelte";
  import GitLabPage from "../gitlab/GitLab.svelte";
  import Agent from "../agent/Agent.svelte";
  import Preset from "./Preset.svelte";
  import Check from "./Check.svelte";

  let { ondone }: { ondone: () => void } = $props();

  // full: a page in list + inspector, which fills the window like in Main.
  type Step = { label: string; title: string; hint: string; component: Component<any>; full?: boolean };

  // The settings pages themselves, in the order a first setup needs them.
  // Each saves on its own, so any step can be skipped and done later.
  const steps: Step[] = [
    { label: "GitLab", title: "Connecte GitLab", hint: "L'instance et le compte du bot", component: Connection },
    { label: "Projets", title: "Choisis les projets", hint: "Allume ceux où le bot travaille", component: GitLabPage, full: true },
    { label: "Modèle", title: "Choisis un modèle", hint: "Selon ce que la machine peut faire tourner", component: Preset },
    { label: "Profil", title: "Règle le profil", hint: "Prompt, modèle et heures de travail", component: Agent, full: true },
    { label: "Vérification", title: "Tout est prêt ?", hint: "Ce qu'il faut au bot pour travailler", component: Check },
  ];

  start();

  let index = $state(0);
  let step = $derived(steps[index]);
  let last = $derived(index === steps.length - 1);
  const next = () => (index = Math.min(index + 1, steps.length - 1));
</script>

<AppWindow title={step.title} subtitle={subtitle() ?? step.hint}>
  {#snippet sidebar()}
    <Sidebar label="Étapes">
      <SidebarSection title="Configuration">
        <ol class="flex flex-col">
          {#each steps as s, i (s.label)}
            <li>
              <button
                type="button"
                class={["p-sidebar-row", i === index && "p-selected"]}
                aria-current={i === index ? "step" : undefined}
                onclick={() => (index = i)}
              >
                <span
                  class={[
                    "p-caption-1 me-2 flex size-5 shrink-0 items-center justify-center rounded-full tabular-nums",
                    i < index ? "bg-(--p-sys-blue) text-white" : i === index ? "ring-2 text-(--p-sys-blue) ring-(--p-sys-blue)" : "text-secondary-label ring-1 ring-(--p-sys-gray)",
                  ]}
                  aria-hidden="true"
                >
                  {#if i < index}<CheckIcon size={11} weight="bold" />{:else}{i + 1}{/if}
                </span>
                <span class="ui-sidebar-label">{s.label}</span>
              </button>
            </li>
          {/each}
        </ol>
      </SidebarSection>
    </Sidebar>
  {/snippet}

  {#snippet toolbar()}
    <div class="flex items-center gap-2">
      {#if index > 0}<Button size="small" onclick={() => index--}>Retour</Button>{/if}
      {#if last}
        <Button size="small" variant="primary" onclick={ondone}>Commencer</Button>
      {:else}
        <Button size="small" variant="borderless" onclick={next}>Passer</Button>
        <Button size="small" variant="primary" onclick={next}>Suivant</Button>
      {/if}
    </div>
  {/snippet}

  {#key index}
    {#if step.full}
      <step.component />
    {:else}
      <div class="mx-auto max-w-[620px]">
        <!-- Connection and the machine move on by themselves once done. -->
        <step.component onsaved={next} onapplied={next} goto={(i: number) => (index = i)} />
      </div>
    {/if}
  {/key}
</AppWindow>
