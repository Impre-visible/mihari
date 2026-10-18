<script lang="ts">
  type Section = { id: string; label: string; description: string };

  const sections: Section[] = [
    { id: "status", label: "État", description: "Modèle chargé, jobs en cours et en attente." },
    { id: "profiles", label: "Profils", description: "Le modèle de journée, le modèle de nuit et leurs jobs en parallèle." },
    { id: "schedule", label: "Planning", description: "Les plages horaires et le profil actif sur chacune." },
    { id: "gitlab", label: "GitLab", description: "L'URL de l'instance, le compte bot et son token." },
  ];

  let activeId = $state(sections[0].id);
  let active = $derived(sections.find((s) => s.id === activeId) ?? sections[0]);
</script>

<div class="flex h-screen bg-ink text-text antialiased">
  <aside class="flex w-56 shrink-0 flex-col border-r border-white/10 bg-white/[0.03] p-4">
    <p class="mb-8 px-3 pt-6 text-lg font-medium">
      <span class="bg-gradient-to-br from-accent to-accent-2 bg-clip-text text-transparent">Mihari</span>
    </p>

    <nav class="flex flex-col gap-1 [--wails-draggable:no-drag]">
      {#each sections as section}
        <button
          type="button"
          onclick={() => (activeId = section.id)}
          aria-current={section.id === activeId ? "page" : undefined}
          class="rounded-lg px-3 py-2 text-left text-sm text-muted transition hover:bg-white/5 hover:text-text aria-[current=page]:bg-white/10 aria-[current=page]:text-text"
        >
          {section.label}
        </button>
      {/each}
    </nav>
  </aside>

  <main class="flex-1 overflow-y-auto p-10">
    <h1 class="text-3xl font-medium tracking-tight">{active.label}</h1>
    <p class="mt-2 text-muted">{active.description}</p>

    <div class="mt-8 rounded-xl border border-dashed border-white/15 p-10 text-center text-sm text-muted">
      À venir
    </div>
  </main>
</div>
