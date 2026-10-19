<script lang="ts">
  import { live } from "$lib/live.svelte";
  import { WEEK, agent, hue, ranges } from "./store.svelte";

  // The week at a glance: one row per day, the hours each profile holds.
  type Props = { selected?: string | null; onpick?: (profile: string) => void };
  let { selected = null, onpick }: Props = $props();

  const DAY = 24 * 60;
  // A range past midnight goes on to the next day's row.
  let bars = $derived(
    agent.segments.flatMap((s) =>
      ranges(s).flatMap(([from, to]) => {
        const parts = [];
        for (let at = from; at < to; at = (Math.floor(at / DAY) + 1) * DAY) {
          const end = Math.min(to, (Math.floor(at / DAY) + 1) * DAY);
          parts.push({ profile: s.profile, day: Math.floor(at / DAY), start: (at % DAY) / DAY, width: (end - at) / DAY });
        }
        return parts;
      }),
    ),
  );
  // Where the week is now: today's row and the time in it.
  let today = $derived(WEEK.findIndex((w) => w.day === new Date(live.now).getDay()));
  let nowAt = $derived.by(() => {
    const d = new Date(live.now);
    return (d.getHours() * 60 + d.getMinutes()) / DAY;
  });
  let fallbackName = $derived(agent.profiles.find((p) => p.id === agent.fallback)?.name);
</script>

<div class="flex flex-col gap-1">
  {#each WEEK as w, i (w.day)}
    <div class="flex items-center gap-1.5">
      <span
        class={["p-caption-1 w-3 text-center", i === today ? "font-bold text-label" : "text-tertiary-label"]}
        title={i === today ? `${w.label} (aujourd'hui)` : w.label}>{w.short}</span
      >
      <div class={["relative h-3 flex-1 overflow-hidden rounded-sm", fallbackName ? "bg-fill" : "bg-fill/50"]}>
        {#each bars.filter((b) => b.day === i) as b, k (k)}
          <button
            type="button"
            class={["absolute inset-y-0 cursor-default rounded-sm", selected && selected !== b.profile && "opacity-35"]}
            style:left="{b.start * 100}%"
            style:width="{b.width * 100}%"
            style:background="hsl({hue(b.profile)} 70% 55%)"
            title={agent.profiles.find((p) => p.id === b.profile)?.name ?? "Sans profil"}
            aria-label={agent.profiles.find((p) => p.id === b.profile)?.name ?? "Sans profil"}
            onclick={() => b.profile && onpick?.(b.profile)}
          ></button>
        {/each}
        {#if i === today}
          <!-- Now, in the text color: no profile has it. -->
          <span
            class="pointer-events-none absolute inset-y-0 w-0.5 -translate-x-1/2 rounded-full bg-label shadow-[0_0_0_1px_var(--p-window-bg)]"
            style:left="{nowAt * 100}%"
            title="Maintenant"
          ></span>
        {/if}
      </div>
    </div>
  {/each}
  <div class="ms-4.5 flex justify-between">
    {#each [0, 6, 12, 18, 24] as h (h)}<span class="p-caption-2 text-tertiary-label tabular-nums">{h}h</span>{/each}
  </div>
</div>
