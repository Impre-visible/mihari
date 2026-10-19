// The agent's setup, profiles and schedule, loaded once and saved as it
// changes. A profile or a schedule that does not hold together is not saved,
// and says why, rather than blocking everything else.
import { Service as Settings } from "../../../bindings/github.com/Impre-visible/mihari/internal/settings";
import { Service as Scheduler } from "../../../bindings/github.com/Impre-visible/mihari/internal/scheduler";
import type { Info as EngineInfo } from "../../../bindings/github.com/Impre-visible/mihari/internal/engine/models";
import { debounced, save } from "$lib/save.svelte";

// The schedule refers to profiles by id, so renaming one keeps its segments.
export type Trigger = "ping" | "issues";
export type Profile = {
  id: string;
  name: string;
  prompt: string;
  // One of the engine.Kind values.
  engine: string;
  model: string;
  contextPerJob: number;
  maxJobs: number;
  trigger: Trigger;
  label: string;
  extraArgs: string;
  maxTurns: number;
  maxMinutes: number;
  unlimitedTurns: boolean;
  unlimitedTime: boolean;
  vision: boolean;
};

// Days use Go's time.Weekday numbering (0 = Sunday) so the backend reads them as is.
export type Segment = { key: number; profile: string; start: string; end: string; days: number[] };

export const WEEK = [
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

export const agent = $state({
  loaded: false,
  engines: [] as EngineInfo[],
  profiles: [] as Profile[],
  segments: [] as Segment[],
  // Profile for every minute no segment covers; "" leaves the machine idle.
  fallback: "",
});

let nextKey = 0;

export function newSegment(profile: string, start = "19:00", end = "23:00", days = [1, 2, 3, 4, 5]): Segment {
  return { key: nextKey++, profile, start, end, days };
}

export function blankProfile(): Profile {
  return {
    id: Date.now().toString(36) + Math.random().toString(36).slice(2, 8),
    name: "",
    prompt: "",
    engine: agent.engines.find((e) => e.supported && e.installed)?.kind ?? "llama-server",
    model: "",
    contextPerJob: 32768,
    maxJobs: 1,
    trigger: "ping",
    label: "",
    extraArgs: "",
    maxTurns: 50,
    maxMinutes: 30,
    unlimitedTurns: false,
    unlimitedTime: false,
    vision: false,
  };
}

// Profiles saved before a field existed get its default, as in internal/profiles.
function parseProfile(value: unknown): Profile | null {
  if (typeof value !== "object" || value === null) return null;
  const v = value as Record<string, unknown>;
  if (typeof v.id !== "string" || typeof v.name !== "string") return null;
  const text = (x: unknown) => (typeof x === "string" ? x : "");
  const count = (x: unknown, fallback: number) => (Number.isInteger(x) && (x as number) > 0 ? (x as number) : fallback);
  return {
    id: v.id,
    name: v.name,
    prompt: text(v.prompt),
    engine: text(v.engine) || "llama-server",
    model: text(v.model),
    contextPerJob: count(v.contextPerJob, 32768),
    maxJobs: count(v.maxJobs, 1),
    trigger: v.trigger === "issues" ? "issues" : "ping",
    label: text(v.label),
    extraArgs: text(v.extraArgs),
    maxTurns: count(v.maxTurns, 50),
    maxMinutes: count(v.maxMinutes, 30),
    unlimitedTurns: v.unlimitedTurns === true,
    unlimitedTime: v.unlimitedTime === true,
    vision: v.vision === true,
  };
}

// A segment whose profile was deleted is kept, to be given another.
function parseSegment(value: unknown): Segment | null {
  if (typeof value !== "object" || value === null) return null;
  const { profile, start, end, days } = value as Record<string, unknown>;
  if (typeof start !== "string" || typeof end !== "string") return null;
  if (!TIME.test(start) || !TIME.test(end) || !Array.isArray(days)) return null;
  const known = typeof profile === "string" && agent.profiles.some((p) => p.id === profile);
  return newSegment(known ? profile : "", start, end, days.filter((d) => Number.isInteger(d) && d >= 0 && d <= 6));
}

let loading: Promise<void> | null = null;

// load reads the setup once; later calls wait for the same read.
export function load() {
  loading ??= Promise.all([Settings.GetConfig(), Scheduler.Engines()]).then(([config, catalog]) => {
    agent.engines = catalog ?? [];
    agent.profiles = (Array.isArray(config?.profiles) ? config.profiles : []).map(parseProfile).filter((p): p is Profile => p !== null);
    agent.fallback = agent.profiles.find((p) => p.id === config?.schedule_fallback)?.id ?? "";
    const stored = Array.isArray(config?.schedule) ? config.schedule : null;
    agent.segments = stored
      ? stored.map(parseSegment).filter((s): s is Segment => s !== null)
      : agent.profiles[0]
        ? [newSegment(agent.profiles[0].id, "09:00", "18:00", [1, 2, 3, 4, 5])]
        : [];
    agent.loaded = true;
  });
  return loading;
}

// What keeps a profile from being saved, null when it holds together.
export function profileProblem(p: Profile): string | null {
  const name = p.name.trim();
  if (name === "") return "Donne-lui un nom.";
  if (agent.profiles.some((o) => o !== p && o.name.trim().toLowerCase() === name.toLowerCase())) return `Un autre profil s'appelle déjà ${name}.`;
  if (p.model.trim() === "") return "Indique le modèle.";
  if (p.prompt.trim() === "") return "Le prompt est vide.";
  if (p.trigger === "issues" && p.label.trim() === "") return "Indique le label des issues à traiter.";
  if (!Number.isInteger(p.maxJobs) || p.maxJobs < 1) return "Il faut au moins 1 job en parallèle.";
  if (!Number.isInteger(p.maxTurns) || p.maxTurns < 1) return "Il faut au moins 1 tour par job.";
  if (!Number.isInteger(p.maxMinutes) || p.maxMinutes < 1) return "Il faut au moins 1 minute par job.";
  return null;
}

function minutes(time: string) {
  const [h, m] = time.split(":").map(Number);
  return h * 60 + m;
}

// Minute-of-week ranges; an end before the start runs past midnight.
export function ranges(s: Segment): [number, number][] {
  const start = minutes(s.start);
  let length = minutes(s.end) - start;
  if (length <= 0) length += DAY_MINUTES;
  return s.days.flatMap((day) => {
    // Monday first: the week strip reads left to right from Monday.
    const from = ((day + 6) % 7) * DAY_MINUTES + start;
    const to = from + length;
    return to > WEEK_MINUTES ? [[from, WEEK_MINUTES], [0, to - WEEK_MINUTES]] : [[from, to]];
  });
}

function overlaps(a: Segment, b: Segment) {
  const rb = ranges(b);
  return ranges(a).some(([from, to]) => rb.some(([f, t]) => from < t && f < to));
}

// What keeps a segment from being saved, null when it holds together.
export function segmentProblem(s: Segment): string | null {
  if (s.profile === "") return "Choisis un profil.";
  if (!TIME.test(s.start) || !TIME.test(s.end)) return "Horaires incomplets.";
  if (s.start === s.end) return "Le début et la fin sont identiques.";
  if (s.days.length === 0) return "Choisis au moins un jour.";
  const other = agent.segments.find((o) => o !== s && o.days.length > 0 && overlaps(s, o));
  if (other) {
    const name = agent.profiles.find((p) => p.id === other.profile)?.name || "une autre plage";
    return `Chevauche ${name} (${other.start}–${other.end}).`;
  }
  return null;
}

// Saves the profiles that hold together, then the schedule's valid segments.
const persist = debounced(() => {
  const profiles = agent.profiles
    .filter((p) => profileProblem(p) === null)
    .map((p) => ({
      ...p,
      name: p.name.trim(),
      model: p.model.trim(),
      label: p.trigger === "issues" ? p.label.trim() : "",
      extraArgs: p.extraArgs.trim(),
    }));
  const saved = new Set(profiles.map((p) => p.id));
  const segments = agent.segments
    .filter((s) => saved.has(s.profile) && segmentProblem(s) === null)
    .map((s) => ({ profile: s.profile, start: s.start, end: s.end, days: WEEK.map((w) => w.day).filter((d) => s.days.includes(d)) }));
  save(async () => {
    await Settings.Set("profiles", profiles);
    await Settings.Set("schedule", segments);
    await Settings.Set("schedule_fallback", saved.has(agent.fallback) ? agent.fallback : "");
    // The scheduler restarts the engine if the active profile changed.
    await Scheduler.Reload();
  });
}, 700);

// changed saves the setup shortly after the last edit.
export function changed() {
  if (agent.loaded) persist();
}

// reload reads the setup again, after the backend changed it (Machine's apply).
export function reload() {
  loading = null;
  return load();
}

// Steady hues, so a profile keeps its color across the page.
const HUES = [211, 145, 28, 280, 350, 180, 50];
export function hue(profile: string) {
  const i = agent.profiles.findIndex((p) => p.id === profile);
  return HUES[(i < 0 ? 0 : i) % HUES.length];
}
