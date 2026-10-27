// The bot's GitLab projects and how it works on each, saved as they change.
import { Service as Projects } from "../../../bindings/github.com/Impre-visible/mihari/internal/projects";
import type { Settings as ProjectSettings, View } from "../../../bindings/github.com/Impre-visible/mihari/internal/projects/models";
import { Service as Settings } from "../../../bindings/github.com/Impre-visible/mihari/internal/settings";
import { Service as Tasks } from "../../../bindings/github.com/Impre-visible/mihari/internal/tasks";
import { debounced, save } from "$lib/save.svelte";

export const DEVELOPER = 30;

export const gitlab = $state({
  loaded: false,
  error: null as string | null,
  projects: [] as View[],
  defaultImage: "",
});

// Go encodes an empty slice as null.
export const extras = (s: ProjectSettings) => s.extraUsers ?? [];
export const blocked = (s: ProjectSettings) => s.blockedUsers ?? [];

// Only projects that differ from the defaults are worth storing.
function customized(s: ProjectSettings) {
  return (
    s.enabled ||
    s.minRole !== DEVELOPER ||
    extras(s).length > 0 ||
    blocked(s).length > 0 ||
    s.image.trim() !== "" ||
    s.instructions.trim() !== "" ||
    s.teaching
  );
}

export function load() {
  gitlab.loaded = false;
  gitlab.error = null;
  return Projects.List()
    .then((o) => {
      gitlab.projects = o.projects ?? [];
      gitlab.defaultImage = o.defaultImage;
      gitlab.loaded = true;
    })
    .catch((err) => (gitlab.error = err instanceof Error ? err.message : String(err)));
}

// changed saves the projects shortly after the last edit.
export const changed = debounced(() => {
  if (!gitlab.loaded) return;
  save(async () => {
    await Settings.Set("default_image", gitlab.defaultImage.trim());
    await Settings.Set(
      "projects",
      gitlab.projects
        .map((p) => p.settings)
        .filter(customized)
        .map((s) => ({ ...s, extraUsers: extras(s), blockedUsers: blocked(s), image: s.image.trim(), instructions: s.instructions.trim() })),
    );
    // Todos of a project just switched on can be sorted right away.
    await Tasks.PollNow();
  });
}, 700);
