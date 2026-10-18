<script lang="ts" generics="T extends string">
  import { normalizeOptions, sizeClass, tintClass, type Option, type Size, type Tint } from "./types";

  type Props = {
    value?: T;
    options: Option<T>[];
    /** Accessible name of the group. */
    label: string;
    size?: Size;
    /** Quiet gray pill, for secondary controls that should not compete with the accent. */
    gray?: boolean;
    tint?: Tint;
    disabled?: boolean;
    class?: string;
  };

  let {
    value = $bindable(),
    options,
    label,
    size = "regular",
    gray = false,
    tint,
    disabled = false,
    class: className,
  }: Props = $props();

  const uid = $props.id();
  const name = `segmented-${uid}`;
  let items = $derived(normalizeOptions(options));
</script>

<!-- A tint class paints a swatch background on the track, so the track fill is restated. -->
<div
  class={["p-segmented", sizeClass("p-segmented", size), gray && "p-segmented-gray", tintClass(tint), className]}
  style:background={tint ? "var(--p-segmented-track)" : undefined}
  role="radiogroup"
  aria-label={label}
>
  {#each items as item (item.value)}
    <label class="p-segment">
      <input type="radio" {name} value={item.value} bind:group={value} disabled={disabled || item.disabled} />
      <span>{item.label}</span>
    </label>
  {/each}
</div>
