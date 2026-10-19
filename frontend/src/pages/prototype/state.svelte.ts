// PROTOTYPE (UI redesign): which variant shows, kept across reloads. Throwaway.
function stored(key: string, fallback: string) {
  try {
    return new URLSearchParams(location.search).get(key) ?? localStorage.getItem("mihari.prototype." + key) ?? fallback;
  } catch {
    return fallback;
  }
}

export const proto = $state({
  enabled: import.meta.env.DEV && stored("prototype", "on") !== "off",
  variant: stored("variant", "A"),
});

$effect.root(() => {
  $effect(() => {
    try {
      localStorage.setItem("mihari.prototype.variant", proto.variant);
      localStorage.setItem("mihari.prototype.prototype", proto.enabled ? "on" : "off");
    } catch {}
  });
});
