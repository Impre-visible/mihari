<script lang="ts">
  // PROTOTYPE (UI redesign): floating bar to flip between variants. Dev builds only.
  import { proto } from "./state.svelte";

  const VARIANTS = [
    { key: "A", name: "Liste + inspecteur" },
    { key: "B", name: "Fil d'activité" },
    { key: "C", name: "Tableau" },
  ];

  let index = $derived(Math.max(0, VARIANTS.findIndex((v) => v.key === proto.variant)));

  function go(step: number) {
    proto.variant = VARIANTS[(index + step + VARIANTS.length) % VARIANTS.length].key;
  }

  function onkey(e: KeyboardEvent) {
    const el = document.activeElement;
    if (el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement || (el as HTMLElement)?.isContentEditable) return;
    if (e.key === "ArrowLeft") go(-1);
    if (e.key === "ArrowRight") go(1);
  }
</script>

<svelte:window onkeydown={onkey} />

<div class="fixed bottom-4 left-1/2 z-50 flex -translate-x-1/2 items-center gap-1 rounded-full bg-black/85 px-1.5 py-1 text-[12px] text-white shadow-xl">
  <button type="button" class="rounded-full px-2 py-0.5 hover:bg-white/15" onclick={() => go(-1)} aria-label="Variante précédente">←</button>
  <span class="px-2 tabular-nums">Prototype {VARIANTS[index].key} · {VARIANTS[index].name}</span>
  <button type="button" class="rounded-full px-2 py-0.5 hover:bg-white/15" onclick={() => go(1)} aria-label="Variante suivante">→</button>
  <span class="mx-1 h-4 w-px bg-white/25"></span>
  <button type="button" class="rounded-full px-2 py-0.5 text-white/70 hover:bg-white/15" onclick={() => (proto.enabled = false)}>Ancienne interface</button>
</div>
