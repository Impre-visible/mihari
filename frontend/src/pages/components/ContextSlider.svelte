<script lang="ts">
  import { Slider } from "$lib/ui";

  // Context sizes a model is usually trained for, from 4K to 1M tokens.
  const STEPS = [4096, 8192, 16384, 32768, 65536, 131072, 262144, 524288, 1048576];

  type Props = { id?: string; value: number; class?: string };
  let { id, value = $bindable(), class: className = "" }: Props = $props();

  // A value saved off the steps shows at the nearest one, and stays as it is
  // until the slider moves.
  let index = $derived(
    STEPS.reduce((best, step, i) => (Math.abs(step - value) < Math.abs(STEPS[best] - value) ? i : best), 0),
  );

  function label(tokens: number) {
    return tokens >= 1048576 ? `${Math.round(tokens / 104857.6) / 10}M` : `${Math.round(tokens / 1024)}K`;
  }
</script>

<div class={["flex items-center gap-3", className]}>
  <Slider
    {id}
    class="w-48"
    size="small"
    min={0}
    max={STEPS.length - 1}
    step={1}
    ticks={STEPS.length - 1}
    aria-valuetext="{label(value)} tokens"
    value={index}
    oninput={(e) => (value = STEPS[Number(e.currentTarget.value)])}
  />
  <span class="p-subhead-2 w-10 text-end text-secondary-label tabular-nums">{label(value)}</span>
</div>
