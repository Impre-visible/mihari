<script lang="ts">
  type Props = {
    /** "HH:MM", 24-hour. */
    value?: string;
    "aria-label"?: string;
    disabled?: boolean;
    class?: string;
  };

  let { value = $bindable(""), "aria-label": label, disabled = false, class: className }: Props = $props();

  // A time like NSDatePicker's: hours and minutes as segments, typed or
  // stepped with the arrows. Always 24-hour, as in French: the native
  // <input type="time"> follows the system's locale, not the page's.
  type Part = "h" | "m";
  const MAX: Record<Part, number> = { h: 23, m: 59 };

  let parsed = $derived.by(() => {
    const match = /^(\d{1,2}):(\d{2})$/.exec(value);
    return match ? { h: Number(match[1]), m: Number(match[2]) } : null;
  });
  // The first digit typed in a segment, waiting for the second.
  let pending = $state<number | null>(null);
  const el: Record<Part, HTMLElement | undefined> = { h: undefined, m: undefined };

  const pad = (n: number) => String(n).padStart(2, "0");

  function set(part: Part, n: number) {
    const t = { h: parsed?.h ?? 0, m: parsed?.m ?? 0, [part]: n };
    value = `${pad(t.h)}:${pad(t.m)}`;
  }

  function keydown(part: Part, e: KeyboardEvent) {
    const current = parsed?.[part] ?? 0;
    if (/^\d$/.test(e.key)) {
      const d = Number(e.key);
      if (pending === null) {
        set(part, d);
        // 3 and up cannot start an hour, 6 and up a minute: the segment is done.
        if (d * 10 > MAX[part]) done(part);
        else pending = d;
      } else {
        set(part, Math.min(pending * 10 + d, MAX[part]));
        done(part);
      }
    } else if (e.key === "ArrowUp" || e.key === "ArrowDown") {
      const n = MAX[part] + 1;
      set(part, (current + (e.key === "ArrowUp" ? 1 : -1) + n) % n);
      pending = null;
    } else if (e.key === "Backspace" || e.key === "Delete") {
      set(part, 0);
      pending = null;
    } else if ((e.key === "ArrowRight" || e.key === ":") && part === "h") {
      el.m?.focus();
    } else if (e.key === "ArrowLeft" && part === "m") {
      el.h?.focus();
    } else {
      return;
    }
    e.preventDefault();
  }

  function done(part: Part) {
    pending = null;
    if (part === "h") el.m?.focus();
  }
</script>

<div
  class={["p-field inline-flex w-auto! cursor-default items-center tabular-nums select-none", disabled && "opacity-50", className]}
  role="group"
  aria-label={label}
>
  {#each ["h", "m"] as const as part (part)}
    {#if part === "m"}<span aria-hidden="true">:</span>{/if}
    <span
      bind:this={el[part]}
      role="spinbutton"
      tabindex={disabled ? -1 : 0}
      aria-label={part === "h" ? "Heures" : "Minutes"}
      aria-valuemin={0}
      aria-valuemax={MAX[part]}
      aria-valuenow={parsed?.[part]}
      aria-disabled={disabled || undefined}
      class="rounded-[3px] px-px outline-none focus:bg-(--p-sys-blue) focus:text-white"
      onkeydown={(e) => !disabled && keydown(part, e)}
      onfocus={() => (pending = null)}
      onblur={() => (pending = null)}
    >
      {parsed ? pad(parsed[part]) : "--"}
    </span>
  {/each}
</div>
