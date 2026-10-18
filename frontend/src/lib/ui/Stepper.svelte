<script lang="ts">
  import type { HTMLInputAttributes } from "svelte/elements";

  type Props = Omit<HTMLInputAttributes, "type" | "value"> & {
    value?: number;
    /** Sink the stepper into the field's trailing edge instead of sitting beside it. */
    inset?: boolean;
  };

  let { value = $bindable(0), inset = false, disabled, class: className, ...rest }: Props = $props();

  let input: HTMLInputElement;
  let timer: ReturnType<typeof setTimeout> | undefined;

  // Native stepUp/stepDown respect min, max and step; the events keep bind:value in sync.
  function step(direction: 1 | -1) {
    if (direction > 0) input.stepUp();
    else input.stepDown();
    input.dispatchEvent(new Event("input", { bubbles: true }));
    input.dispatchEvent(new Event("change", { bubbles: true }));
  }

  // Like AppKit: one step on press, then repeat after 400ms while held.
  function press(direction: 1 | -1) {
    step(direction);
    timer = setTimeout(function repeat() {
      step(direction);
      timer = setTimeout(repeat, 60);
    }, 400);
  }

  function release() {
    clearTimeout(timer);
  }
</script>

<div class={["p-stepper-field", inset && "p-stepper-field-inset", className]}>
  <input bind:this={input} class="p-field" type="number" bind:value {disabled} {...rest} />
  <span class="p-stepper">
    {#each [1, -1] as const as direction}
      <button
        type="button"
        tabindex="-1"
        data-p-step={direction > 0 ? "up" : "down"}
        aria-label={direction > 0 ? "Augmenter" : "Diminuer"}
        {disabled}
        onpointerdown={() => press(direction)}
        onpointerup={release}
        onpointerleave={release}
        onpointercancel={release}
      ></button>
    {/each}
  </span>
</div>
