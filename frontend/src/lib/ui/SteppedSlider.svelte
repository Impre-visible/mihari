<script lang="ts">
  import Slider from "./Slider.svelte";

  // A slider that moves between given values, such as sizes that grow by
  // powers of two, with the current one written beside it.
  type Props = {
    id?: string;
    value: number;
    /** The values to step through, in increasing order. */
    steps: number[];
    /** How a value reads beside the slider and to screen readers. */
    format?: (value: number) => string;
    class?: string;
  };
  let { id, value = $bindable(), steps, format = String, class: className = "" }: Props = $props();

  // A value off the steps shows at the nearest one, and stays as it is until
  // the slider moves.
  let index = $derived(
    steps.reduce((best, step, i) => (Math.abs(step - value) < Math.abs(steps[best] - value) ? i : best), 0),
  );
</script>

<div class={["flex items-center gap-3", className]}>
  <Slider
    {id}
    class="w-48"
    size="small"
    min={0}
    max={steps.length - 1}
    step={1}
    ticks={steps.length - 1}
    aria-valuetext={format(value)}
    value={index}
    oninput={(e) => (value = steps[Number(e.currentTarget.value)])}
  />
  <span class="p-subhead-2 w-10 text-end text-secondary-label tabular-nums">{format(value)}</span>
</div>
