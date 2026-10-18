<script lang="ts">
  import { CheckCircleIcon, PlusIcon, TrashIcon, WarningCircleIcon } from "phosphor-svelte";
  import { Service as Settings } from "../../../bindings/github.com/Impre-visible/mihari/internal/settings";
  import { Button, FormGroup, FormRow, Select, Spinner, TextField } from "$lib/ui";

  type Status =
    | { kind: "idle" }
    | { kind: "busy"; label: string }
    | { kind: "ok"; message: string }
    | { kind: "error"; message: string };

  // Days use Go's time.Weekday numbering (0 = Sunday) so the backend can read them as is.
  type Segment = { key: number; profile: string; start: string; end: string; days: number[] };
  type Profile = { id: string; name: string };

  const WEEK = [
    { day: 1, short: "L", label: "Lundi" },
    { day: 2, short: "M", label: "Mardi" },
    { day: 3, short: "M", label: "Mercredi" },
    { day: 4, short: "J", label: "Jeudi" },
    { day: 5, short: "V", label: "Vendredi" },
    { day: 6, short: "S", label: "Samedi" },
    { day: 0, short: "D", label: "Dimanche" },
  ];
  const DAY_MINUTES = 24 * 60;
  const WEEK_MINUTES = 7 * DAY_MINUTES;
  const TIME = /^\d{2}:\d{2}$/;

  let nextKey = 0;
  let segments = $state<Segment[]>([]);
  let profiles = $state<Profile[]>([]);
  // Profile for every minute no segment covers; "" leaves the machine idle.
  let fallback = $state("");
  let status = $state<Status>({ kind: "idle" });

  let busy = $derived(status.kind === "busy");
  let problem = $derived(validate(segments));
  let fallbackOptions = $derived([
    { value: "", label: "Aucun, en pause" },
    ...profiles.map((p) => ({ value: p.id, label: p.name })),
  ]);
  let profileOptions = $derived([
    { value: "", label: "Choisir…", disabled: true },
    ...profiles.map((p) => ({ value: p.id, label: p.name })),
  ]);

  function segment(profile: string, start: string, end: string, days: number[]): Segment {
    return { key: nextKey++, profile, start, end, days };
  }

  function parseProfile(value: unknown): Profile | null {
    if (typeof value !== "object" || value === null) return null;
    const { id, name } = value as Record<string, unknown>;
    return typeof id === "string" && typeof name === "string" ? { id, name } : null;
  }

  // A profile deleted since the segment was saved leaves the segment to be reassigned.
  function parseSegment(value: unknown): Segment | null {
    if (typeof value !== "object" || value === null) return null;
    const { profile, start, end, days } = value as Record<string, unknown>;
    if (typeof start !== "string" || typeof end !== "string") return null;
    if (!TIME.test(start) || !TIME.test(end) || !Array.isArray(days)) return null;
    const known = typeof profile === "string" && profiles.some((p) => p.id === profile);
    return segment(known ? profile : "", start, end, days.filter((d) => Number.isInteger(d) && d >= 0 && d <= 6));
  }

  Settings.GetConfig()
    .then((config) => {
      const storedProfiles = Array.isArray(config?.profiles) ? config.profiles : [];
      profiles = storedProfiles.map(parseProfile).filter((p): p is Profile => p !== null);
      const storedFallback = config?.schedule_fallback;
      fallback = profiles.some((p) => p.id === storedFallback) ? storedFallback : "";
      const stored = Array.isArray(config?.schedule) ? config.schedule : null;
      segments = stored
        ? stored.map(parseSegment).filter((s): s is Segment => s !== null)
        : [segment(profiles[0]?.id ?? "", "09:00", "18:00", [1, 2, 3, 4, 5])];
    })
    .catch((err) => console.error("Load schedule:", err));

  function minutes(time: string) {
    const [h, m] = time.split(":").map(Number);
    return h * 60 + m;
  }

  // Minute-of-week ranges, with an end before the start meaning the segment runs past midnight.
  function ranges(s: Segment): [number, number][] {
    const start = minutes(s.start);
    let length = minutes(s.end) - start;
    if (length <= 0) length += DAY_MINUTES;
    return s.days.flatMap((day) => {
      const from = day * DAY_MINUTES + start;
      const to = from + length;
      // Sunday night wraps onto Monday morning.
      return to > WEEK_MINUTES ? [[from, WEEK_MINUTES], [0, to - WEEK_MINUTES]] : [[from, to]];
    });
  }

  function overlaps(a: Segment, b: Segment) {
    const rb = ranges(b);
    return ranges(a).some(([from, to]) => rb.some(([f, t]) => from < t && f < to));
  }

  function validate(list: Segment[]): string | null {
    for (const [i, s] of list.entries()) {
      const label = `Segment ${i + 1}`;
      if (s.profile === "") return `${label} : aucun profil choisi.`;
      if (!TIME.test(s.start) || !TIME.test(s.end)) return `${label} : horaires incomplets.`;
      if (s.start === s.end) return `${label} : le début et la fin sont identiques.`;
      if (s.days.length === 0) return `${label} : aucun jour sélectionné.`;
    }
    for (let i = 0; i < list.length; i++) {
      for (let j = i + 1; j < list.length; j++) {
        if (overlaps(list[i], list[j])) return `Les segments ${i + 1} et ${j + 1} se chevauchent.`;
      }
    }
    return null;
  }

  function toggleDay(s: Segment, day: number) {
    s.days = s.days.includes(day) ? s.days.filter((d) => d !== day) : [...s.days, day];
    resetStatus();
  }

  function add() {
    segments.push(segment("", "19:00", "23:00", [1, 2, 3, 4, 5]));
    resetStatus();
  }

  function remove(s: Segment) {
    segments = segments.filter((other) => other !== s);
    resetStatus();
  }

  async function save() {
    status = { kind: "busy", label: "Enregistrement…" };
    try {
      await Settings.Set(
        "schedule",
        segments.map((s) => ({
          profile: s.profile,
          start: s.start,
          end: s.end,
          days: WEEK.map((w) => w.day).filter((d) => s.days.includes(d)),
        })),
      );
      await Settings.Set("schedule_fallback", fallback);
      status = { kind: "ok", message: "Planning enregistré." };
    } catch (err) {
      status = { kind: "error", message: err instanceof Error ? err.message : String(err) };
    }
  }

  function resetStatus() {
    if (status.kind !== "busy") status = { kind: "idle" };
  }
</script>

<p class="p-body text-secondary-label">Les plages horaires et le profil actif sur chacune.</p>

<form
  class="mt-6"
  oninput={resetStatus}
  onsubmit={(e) => {
    e.preventDefault();
    if (!problem) save();
  }}
>
  {#each segments as s, i (s.key)}
    <FormGroup title="Segment {i + 1}">
      <FormRow label="Profil" for="segment-{s.key}-profile">
        <div class="flex items-center gap-2">
          {#if profiles.length > 0}
            <Select
              id="segment-{s.key}-profile"
              options={profileOptions}
              bind:value={s.profile}
              onchange={resetStatus}
            />
          {:else}
            <span class="p-body text-secondary-label">Aucun profil, crée-en un dans Profils.</span>
          {/if}
          <Button
            size="small"
            variant="borderless"
            iconOnly
            aria-label="Supprimer le segment {i + 1}"
            onclick={() => remove(s)}
          >
            <TrashIcon size={14} aria-hidden="true" />
          </Button>
        </div>
      </FormRow>
      <FormRow label="Horaires" for="segment-{s.key}-start">
        <div class="flex items-center gap-2">
          <TextField id="segment-{s.key}-start" type="time" aria-label="Début" bind:value={s.start} />
          <span class="p-body text-secondary-label">à</span>
          <TextField id="segment-{s.key}-end" type="time" aria-label="Fin" bind:value={s.end} />
        </div>
      </FormRow>
      <FormRow label="Jours">
        <div class="flex gap-1" role="group" aria-label="Jours">
          {#each WEEK as w (w.day)}
            <Button
              size="small"
              variant={s.days.includes(w.day) ? "primary" : "default"}
              class="w-7"
              aria-label={w.label}
              aria-pressed={s.days.includes(w.day)}
              onclick={() => toggleDay(s, w.day)}
            >
              {w.short}
            </Button>
          {/each}
        </div>
      </FormRow>
    </FormGroup>
  {:else}
    <FormGroup>
      <p class="p-body py-4 text-center text-tertiary-label">Aucun segment</p>
    </FormGroup>
  {/each}

  <FormGroup title="Le reste du temps" footer="S'applique à chaque heure qu'aucun segment ne couvre.">
    <FormRow label="Profil" for="schedule-fallback">
      <Select id="schedule-fallback" class="me-6" options={fallbackOptions} bind:value={fallback} onchange={resetStatus} />
    </FormRow>
  </FormGroup>

  <div class="mt-4 flex items-center gap-2">
    <p class="p-subhead-2 flex min-w-0 flex-1 items-center gap-1.5" role="status">
      {#if status.kind === "busy"}
        <Spinner small label={status.label} />
        <span class="text-secondary-label">{status.label}</span>
      {:else if status.kind === "ok"}
        <CheckCircleIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-green)" aria-hidden="true" />
        <span class="text-secondary-label">{status.message}</span>
      {:else if status.kind === "error" || problem}
        <WarningCircleIcon size={16} weight="fill" class="shrink-0 text-(--p-sys-red)" aria-hidden="true" />
        <span class="text-label">{status.kind === "error" ? status.message : problem}</span>
      {/if}
    </p>

    <Button disabled={busy} onclick={add}>
      <PlusIcon size={14} aria-hidden="true" />
      Ajouter un segment
    </Button>
    <Button type="submit" variant="primary" disabled={!!problem || busy}>Enregistrer</Button>
  </div>
</form>
