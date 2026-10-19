import type { Event as JobEvent } from "../../../../bindings/github.com/Impre-visible/mihari/internal/agent/models";
import { todos, type Todo } from "./view";

// A tool call with its result, once the tool has answered.
export type ToolCall = {
  kind: "call";
  name: string;
  at: string;
  // The arguments as the model sent them, and parsed when they are JSON.
  raw: string;
  args: Record<string, unknown>;
  result: JobEvent | null;
};

export type Item = { kind: "event"; event: JobEvent } | ToolCall;

// toItems pairs each tool event with the result that follows it, so a call
// reads as one block. Tools of one turn run in order, so the first call still
// waiting under that name is the one a result answers.
export function toItems(events: JobEvent[]): Item[] {
  const items: Item[] = [];
  const waiting: ToolCall[] = [];
  for (const event of events) {
    if (event.kind === "tool") {
      const call: ToolCall = { kind: "call", name: event.title, at: event.at, raw: event.text, args: parse(event.text), result: null };
      items.push(call);
      waiting.push(call);
      continue;
    }
    if (event.kind === "result") {
      const i = waiting.findIndex((c) => c.name === event.title);
      if (i >= 0) {
        waiting[i].result = event;
        waiting.splice(i, 1);
        continue;
      }
    }
    items.push({ kind: "event", event });
  }
  return items;
}

function parse(text: string): Record<string, unknown> {
  try {
    const value = JSON.parse(text);
    return value && typeof value === "object" && !Array.isArray(value) ? value : {};
  } catch {
    return {};
  }
}

// Tools report failures as a result starting with this, rather than an error.
export function failed(call: ToolCall) {
  return call.result?.text.startsWith("Erreur") ?? false;
}

// The exit code run_command, list_files and search append to their output.
export function exitCode(output: string): number | null {
  const match = output.match(/\n\[code de sortie (\d+)[^\]]*\]$/);
  return match ? Number(match[1]) : null;
}

// Output without the exit code line, which the view shows apart.
export function withoutExitCode(output: string) {
  return output.replace(/\n\[code de sortie \d+[^\]]*\]$/, "");
}

// The plan of the last todo_write call, [] before the agent made one.
export function latestPlan(events: JobEvent[]): Todo[] {
  for (let i = events.length - 1; i >= 0; i--) {
    const e = events[i];
    if (e.kind === "tool" && e.title === "todo_write") return todos(parse(e.text));
  }
  return [];
}
