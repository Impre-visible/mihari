// What the app shows live: jobs, the engine, GitLab's latest poll and the
// container runtime, loaded once and kept up to date by the backend's events.
import { Events } from "@wailsio/runtime";
import { Service as Jobs } from "../../bindings/github.com/Impre-visible/mihari/internal/jobs";
import type { Summary } from "../../bindings/github.com/Impre-visible/mihari/internal/jobs/models";
import { Service as Scheduler } from "../../bindings/github.com/Impre-visible/mihari/internal/scheduler";
import type { Snapshot as EngineSnapshot } from "../../bindings/github.com/Impre-visible/mihari/internal/scheduler/models";
import { Service as Sandbox } from "../../bindings/github.com/Impre-visible/mihari/internal/sandbox";
import type { Info as SandboxInfo } from "../../bindings/github.com/Impre-visible/mihari/internal/sandbox/models";
import { Service as Tasks } from "../../bindings/github.com/Impre-visible/mihari/internal/tasks";
import type { Snapshot } from "../../bindings/github.com/Impre-visible/mihari/internal/tasks/models";
import type { Event as JobEvent } from "../../bindings/github.com/Impre-visible/mihari/internal/agent/models";

export const live = $state({
  jobs: [] as Summary[],
  engine: null as EngineSnapshot | null,
  sandbox: null as SandboxInfo | null,
  tasks: null as Snapshot | null,
  now: Date.now(),
});

let started = false;

// start loads everything and follows the events; later calls do nothing.
export function start() {
  if (started) return;
  started = true;
  Jobs.List()
    .then((l) => (live.jobs = l ?? []))
    .catch((err) => console.error("Load jobs:", err));
  Scheduler.Snapshot()
    .then((s) => (live.engine = s))
    .catch((err) => console.error("Load engine:", err));
  Tasks.Snapshot()
    .then((s) => (live.tasks = s))
    .catch((err) => console.error("Load tasks:", err));
  checkSandbox();
  Events.On("jobs:updated", (e) => (live.jobs = e.data ?? []));
  Events.On("engine:updated", (e) => (live.engine = e.data));
  Events.On("tasks:updated", (e) => (live.tasks = e.data));
  setInterval(() => (live.now = Date.now()), 1000);
}

export function checkSandbox() {
  live.sandbox = null;
  Sandbox.Info()
    .then((s) => (live.sandbox = s))
    .catch((err) => console.error("Sandbox info:", err));
}

// A job's transcript, loaded then kept up to date; close it when done.
export class Transcript {
  events = $state<JobEvent[]>([]);
  #off: () => void;

  constructor(id: number) {
    // Lines that arrive while loading are in the list too.
    Jobs.Transcript(id)
      .then((l) => (this.events = l ?? []))
      .catch((err) => console.error("Load transcript:", err));
    this.#off = Events.On("job:event", (e) => {
      if (e.data.jobId === id) this.events.push(e.data.event);
    });
  }

  close() {
    this.#off();
  }
}

// How long ago at was, the way people say it.
export function ago(at: string, now: number) {
  const s = Math.max(0, Math.round((now - Date.parse(at)) / 1000));
  if (s < 60) return "à l'instant";
  if (s < 3600) return `il y a ${Math.floor(s / 60)} min`;
  if (s < 86400) return `il y a ${Math.floor(s / 3600)} h`;
  return new Date(at).toLocaleDateString("fr-FR", { day: "numeric", month: "short" });
}
