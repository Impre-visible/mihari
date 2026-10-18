<script lang="ts">
  const mihariVersion = "v1.0.0";

  const steps = [
    {
      title: "Profils de modèles",
      description: "Un modèle de journée, un modèle de nuit et le nombre de jobs en parallèle.",
    },
    {
      title: "Planning",
      description: "Les plages horaires et le profil actif sur chacune.",
    },
    {
      title: "Connexion GitLab",
      description: "L'URL de l'instance, le compte bot et son token, rangé dans le trousseau.",
    },
  ];
</script>

<div class="flex min-h-screen flex-col bg-ink text-text antialiased">
  <!-- Halo décoratif derrière le titre. -->
  <div
    aria-hidden="true"
    class="pointer-events-none fixed -top-40 left-1/2 h-96 w-[40rem] -translate-x-1/2 rounded-full bg-gradient-to-br from-accent to-accent-2 opacity-15 blur-3xl"
  ></div>

  <main class="relative mx-auto flex w-full max-w-4xl flex-1 flex-col justify-center px-8 py-16">
    <h1 class="text-5xl font-medium tracking-tight text-balance">
      Bienvenue sur
      <span class="bg-gradient-to-br from-accent to-accent-2 bg-clip-text text-transparent">Mihari</span>
    </h1>

    <p class="mt-4 max-w-xl text-lg text-pretty text-muted">
      Des LLM locaux qui relisent vos merge requests GitLab le jour et écrivent des tests la nuit.
      Le code ne quitte jamais la machine.
    </p>

    <section class="mt-12">
      <h2 class="mb-4 text-xs font-medium tracking-widest text-muted uppercase">Pour commencer</h2>

      <ol class="grid gap-4 sm:grid-cols-3">
        {#each steps as step, i}
          <li class="rounded-xl border border-white/10 bg-white/5 p-5 backdrop-blur">
            <span
              class="mb-4 flex size-8 items-center justify-center rounded-full bg-gradient-to-br from-accent to-accent-2 text-sm font-medium"
            >
              {i + 1}
            </span>
            <h3 class="font-medium">{step.title}</h3>
            <p class="mt-1 text-sm text-pretty text-muted">{step.description}</p>
          </li>
        {/each}
      </ol>
    </section>
  </main>

  <footer
    class="relative flex items-center justify-between border-t border-white/10 px-8 py-4 text-xs text-muted"
  >
    <span class="tabular-nums">{mihariVersion}</span>
    <span>Mihari <span aria-hidden="true">©</span></span>
    <span>Made with ❤️</span>
  </footer>
</div>
