import type { Component } from "svelte";
import type { ToolCall } from "./transcript";

// The header of a tool call: what the tool is doing, on what. What changes
// something starts open; what only reads starts folded.
export type View = { icon: Component; title: string; detail?: string; open: boolean };

// What each tool's view gets to draw the body of its call.
export type ToolProps = {
  call: ToolCall;
  // The result's text, "" while the call runs.
  output: string;
  isError: boolean;
  pending: boolean;
};

// A tool's view: tools/RunCommand.svelte for run_command, exporting view()
// from its module script and drawing the body as its component.
export type ToolModule = {
  default: Component<ToolProps>;
  view: (args: Record<string, unknown>) => View;
};

export function str(value: unknown) {
  return typeof value === "string" ? value : value === undefined || value === null ? "" : String(value);
}

// ":12-20" from start_line and end_line, "" without them.
export function range(a: Record<string, unknown>) {
  const start = a.start_line, end = a.end_line;
  if (!start && !end) return "";
  return `:${str(start) || 1}${end && end !== start ? `-${str(end)}` : ""}`;
}

export function numbered(text: string) {
  const lines = text.replace(/\n$/, "").split("\n");
  const width = String(lines.length).length;
  return lines.map((l, i) => `${String(i + 1).padStart(width)}  ${l}`).join("\n");
}

// Lines as a diff adds (+) or removes (-) them.
export function marked(text: string, sign: "+" | "-") {
  return text.replace(/\n$/, "").split("\n").map((l) => sign + l).join("\n");
}

export type Todo = { content: string; status: "pending" | "in_progress" | "completed" };

export function todos(args: Record<string, unknown>): Todo[] {
  return Array.isArray(args.todos) ? (args.todos as Todo[]).filter((t) => t && typeof t.content === "string") : [];
}
