// Settings save as they change, like macOS: what the window's subtitle says
// about the latest save, and how a page saves.

export const saving = $state<{ state: "idle" | "saving" | "saved" | "error"; error: string }>({
  state: "idle",
  error: "",
});

let pending = 0;
let fade: ReturnType<typeof setTimeout> | undefined;

// save runs write, and tells the window how it went. The error stays with
// the caller too, to show it next to the field.
export async function save(write: () => Promise<unknown>): Promise<string | null> {
  pending++;
  saving.state = "saving";
  clearTimeout(fade);
  try {
    await write();
    if (--pending === 0) {
      saving.state = "saved";
      fade = setTimeout(() => saving.state === "saved" && (saving.state = "idle"), 2000);
    }
    return null;
  } catch (err) {
    pending--;
    const message = err instanceof Error ? err.message : String(err);
    saving.state = "error";
    saving.error = message;
    return message;
  }
}

// debounced returns a function that runs fn once the calls stop for wait ms:
// for text typed in a field.
export function debounced<A extends unknown[]>(fn: (...args: A) => void, wait = 600) {
  let timer: ReturnType<typeof setTimeout> | undefined;
  return (...args: A) => {
    clearTimeout(timer);
    timer = setTimeout(() => fn(...args), wait);
  };
}

export function subtitle() {
  switch (saving.state) {
    case "saving":
      return "Enregistrement…";
    case "saved":
      return "Enregistré";
    case "error":
      return "Non enregistré : " + saving.error;
  }
  return undefined;
}
