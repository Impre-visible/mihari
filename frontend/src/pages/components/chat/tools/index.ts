import type { ToolModule } from "../view";

// Every view in this folder, keyed by its tool's name: RunCommand.svelte
// shows run_command. A new tool only needs its file here.
const modules = import.meta.glob<ToolModule>("./*.svelte", { eager: true });

function toolName(file: string) {
  return file
    .replace(/^\.\/|\.svelte$/g, "")
    .replace(/([a-z0-9])([A-Z])/g, "$1_$2")
    .toLowerCase();
}

export const TOOLS: Record<string, ToolModule> = Object.fromEntries(
  Object.entries(modules).map(([file, module]) => [toolName(file), module]),
);
