<script lang="ts">
  import type { Component } from "svelte";
  import { CheckIcon } from "phosphor-svelte";
  import { AppWindow, Button } from "$lib/ui";
  import Gitlab from "./components/Gitlab.svelte";
  import Projects from "./components/Projects.svelte";
  import Machine from "./components/Machine.svelte";
  import Profiles from "./components/Profiles.svelte";
  import Schedule from "./components/Schedule.svelte";

  let { ondone }: { ondone: () => void } = $props();

  type Step = { title: string; hint: string; component: Component };

  // The settings pages themselves, in the order a first setup needs them.
  // Each saves on its own, so any step can be skipped and done later.
  const steps: Step[] = [
    {
      title: "Connecte GitLab",
      hint: "Teste la connexion, puis enregistre.",
      component: Gitlab,
    },
    {
      title: "Choisis les projets",
      hint: "Allume ceux où le bot peut travailler, puis enregistre.",
      component: Projects,
    },
    {
      title: "Choisis un modèle",
      hint: "Applique une recommandation à un nouveau profil.",
      component: Machine,
    },
    {
      title: "Règle le profil",
      hint: "Charge un prompt prêt, télécharge le modèle avec l'icône à côté de son nom, puis enregistre.",
      component: Profiles,
    },
    {
      title: "Planifie",
      hint: "Dis quand le profil tourne, puis enregistre.",
      component: Schedule,
    },
  ];

  let index = $state(0);
  let step = $derived(steps[index]);
  let last = $derived(index === steps.length - 1);
</script>

<AppWindow title="Configuration">
  <div class="mx-auto flex max-w-3xl flex-col">
    <ol class="flex items-center gap-2" aria-label="Étapes">
      {#each steps as s, i (s.title)}
        <li class="flex items-center gap-2">
          <span
            class={[
              "p-subhead-2 flex size-6 shrink-0 items-center justify-center rounded-full tabular-nums",
              i < index && "bg-(--p-sys-blue) text-white",
              i === index && "text-(--p-sys-blue) ring-2 ring-(--p-sys-blue)",
              i > index && "text-tertiary-label ring-1 ring-(--p-sys-gray)",
            ]}
            aria-current={i === index ? "step" : undefined}
          >
            {#if i < index}<CheckIcon size={12} weight="bold" aria-hidden="true" />{:else}{i + 1}{/if}
          </span>
          {#if i < steps.length - 1}<span class="h-px w-6 bg-(--p-sys-gray) opacity-40"></span>{/if}
        </li>
      {/each}
    </ol>

    <h1 class="p-title-1 p-emphasized mt-5">{step.title}</h1>
    <p class="p-body mt-1 text-secondary-label">{step.hint}</p>

    <div class="mt-6">
      {#key index}
        <step.component />
      {/key}
    </div>

    <div class="mt-8 mb-2 flex items-center gap-2 border-t border-(--p-sys-gray)/30 pt-4">
      {#if index > 0}
        <Button onclick={() => index--}>Retour</Button>
      {/if}
      <span class="flex-1"></span>
      {#if !last}
        <Button variant="borderless" onclick={() => index++}>Passer</Button>
        <Button variant="primary" onclick={() => index++}>Suivant</Button>
      {:else}
        <Button variant="primary" onclick={ondone}>Terminer</Button>
      {/if}
    </div>
  </div>
</AppWindow>
