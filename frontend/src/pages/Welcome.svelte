<script lang="ts">
  let { onstart }: { onstart: () => void } = $props();

  const mihariVersion = "v1.0.0";
</script>

<div class="flex flex-col bg-ink text-text antialiased min-h-screen">
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

    <button
      type="button"
      onclick={onstart}
      class="mt-12 self-start rounded-lg bg-gradient-to-br from-accent to-accent-2 px-6 py-3 font-medium [--wails-draggable:no-drag] transition hover:brightness-110 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
    >
      Commencer
    </button>
  </main>

  <footer
    class="relative flex items-center justify-between border-t border-white/10 px-8 py-4 text-xs text-muted"
  >
    <span class="tabular-nums">{mihariVersion}</span>
    <span>Mihari <span aria-hidden="true">©</span></span>
    <span>Made with ❤️</span>
  </footer>
</div>
