import type { Summary } from "../../../bindings/github.com/Impre-visible/mihari/internal/jobs/models";

export const STATES: Record<string, string> = {
  queued: "en attente",
  running: "en cours",
  succeeded: "terminé",
  failed: "échec",
  canceled: "annulé",
};

export function live(j: Summary) {
  return j.state === "running" || j.state === "queued";
}

// How long a job ran, or has been running at now.
export function duration(j: Summary, now: number) {
  const start = Date.parse(j.startedAt);
  if (j.startedAt.startsWith("0001")) return "";
  const end = j.endedAt.startsWith("0001") ? now : Date.parse(j.endedAt);
  const s = Math.max(0, Math.round((end - start) / 1000));
  return s < 60 ? `${s} s` : `${Math.floor(s / 60)} min ${String(s % 60).padStart(2, "0")}`;
}
