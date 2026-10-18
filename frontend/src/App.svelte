<script lang="ts">
  import { Service as Settings } from "../bindings/github.com/Impre-visible/mihari/internal/settings";
  import Main from "./pages/Main.svelte";
  import Welcome from "./pages/Welcome.svelte";

  type Route = "loading" | "welcome" | "main";

  let route = $state<Route>("loading");

  Settings.IsOnboarded()
    .then((onboarded) => (route = onboarded ? "main" : "welcome"))
    .catch((err) => {
      console.error("IsOnboarded:", err);
      route = "welcome";
    });

  async function start() {
    try {
      await Settings.CompleteOnboarding();
    } catch (err) {
      console.error("CompleteOnboarding:", err);
    }
    route = "main";
  }
</script>

{#if route === "welcome"}
  <Welcome onstart={start} />
{:else if route === "main"}
  <Main />
{/if}
