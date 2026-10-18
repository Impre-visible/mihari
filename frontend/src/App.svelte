<script lang="ts">
  import { Service as Settings } from "../bindings/github.com/Impre-visible/mihari/internal/settings";
  import Main from "./pages/Main.svelte";
  import Welcome from "./pages/Welcome.svelte";
  import Wizard from "./pages/Wizard.svelte";
  import { nav } from "$lib/nav.svelte";

  type Route = "loading" | "welcome" | "wizard" | "main";

  let route = $state<Route>("loading");

  Settings.IsOnboarded()
    .then((onboarded) => (route = onboarded ? "main" : "welcome"))
    .catch((err) => {
      console.error("IsOnboarded:", err);
      route = "welcome";
    });

  async function finish() {
    try {
      await Settings.CompleteOnboarding();
    } catch (err) {
      console.error("CompleteOnboarding:", err);
    }
    nav.wizard = false;
    nav.section = "status";
    route = "main";
  }

  // Relaunched from the General page.
  $effect(() => {
    if (nav.wizard) route = "wizard";
  });
</script>

{#if route === "welcome"}
  <Welcome onstart={() => (route = "wizard")} onskip={finish} />
{:else if route === "wizard"}
  <Wizard ondone={finish} />
{:else if route === "main"}
  <Main />
{/if}
