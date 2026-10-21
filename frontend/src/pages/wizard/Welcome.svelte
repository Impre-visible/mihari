<script lang="ts">
  import { CpuIcon, GitMergeIcon, ShieldCheckIcon, TestTubeIcon } from "phosphor-svelte";
  import { Service as Advisor } from "../../../bindings/github.com/Impre-visible/mihari/internal/advisor";
  import { Service as Updates } from "../../../bindings/github.com/Impre-visible/mihari/internal/updates";
  import { AppWindow, Button } from "$lib/ui";
  import logo from "$lib/logo-icon.svg";

  let { onstart, onskip }: { onstart: () => void; onskip: () => void } = $props();

  let mihariVersion = $state("");
  Updates.Status().then((i) => (mihariVersion = "v" + i.current));

  // On an Apple Silicon Mac, the chip and its memory, which the setup's model step builds on.
  let detected = $state("");
  Advisor.Machine().then(({ hardware: h }) => {
    if (h.unified && h.cpu) detected = `Puce ${h.cpu} détectée avec ${Math.round(h.ram / 2 ** 30)} Go de mémoire unifiée`;
  });

  // The macOS "Welcome to…" sheet: a few features, each a glyph, a title and a sentence.
  const features = [
    {
      icon: GitMergeIcon,
      title: "Relecture des merge requests",
      text: "Un LLM local relit vos merge requests GitLab pendant la journée.",
    },
    {
      icon: TestTubeIcon,
      title: "Tests écrits la nuit",
      text: "Quand la machine est libre, un autre modèle écrit les tests manquants.",
    },
    {
      icon: ShieldCheckIcon,
      title: "Tout reste en local",
      text: "Le code ne quitte jamais la machine.",
    },
  ];
</script>

<AppWindow>
  <div class="mx-auto flex h-full max-w-lg flex-col items-center justify-center py-6 text-center">
    <img src={logo} alt="" class="size-20 shadow-md rounded-[18px]" />

    <h1 class="p-large-title-2 p-emphasized mt-5">Bienvenue dans Mihari</h1>

    <ul class="mt-8 flex flex-col gap-5 text-left">
      {#each features as feature (feature.title)}
        <li class="flex items-start gap-4">
          <feature.icon size={28} class="mt-0.5 shrink-0 text-accent" aria-hidden="true" />
          <div>
            <p class="p-headline-2 text-label">{feature.title}</p>
            <p class="p-subhead-2 text-secondary-label">{feature.text}</p>
          </div>
        </li>
      {/each}
    </ul>

    {#if detected}
      <p class="p-subhead-2 mt-8 flex items-center gap-2 rounded-lg px-3 py-2 text-left text-secondary-label ring-1 ring-(--p-sys-gray)">
        <CpuIcon size={18} class="shrink-0 text-accent" aria-hidden="true" />
        {detected}
      </p>
    {/if}

    <Button variant="primary" size="large" class="mt-10 min-w-48" onclick={onstart}>Configurer</Button>
    <Button variant="borderless" class="mt-2" onclick={onskip}>Plus tard</Button>

    <p class="p-caption-1 mt-4 text-tertiary-label tabular-nums">{mihariVersion}</p>
  </div>
</AppWindow>
