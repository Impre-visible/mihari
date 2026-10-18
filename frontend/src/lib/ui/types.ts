/** macOS system accents, mapped to Puppertino's `.p-sys-*` classes. */
export type Tint =
  | "red"
  | "orange"
  | "yellow"
  | "green"
  | "mint"
  | "teal"
  | "cyan"
  | "blue"
  | "indigo"
  | "purple"
  | "pink"
  | "brown"
  | "gray";

/** The shared macOS control ladder: 16, 20, 24, 28 and 36px tall. */
export type Size = "mini" | "small" | "regular" | "large" | "xlarge";

export type Option<T extends string = string> = T | { value: T; label: string; disabled?: boolean };

export function tintClass(tint: Tint | undefined) {
  return tint && `p-sys-${tint}`;
}

/** Puppertino's default size has no class; every other step is `<base>-<size>`. */
export function sizeClass(base: string, size: Size | undefined) {
  return size && size !== "regular" && `${base}-${size}`;
}

export function normalizeOptions<T extends string>(options: Option<T>[]) {
  return options.map((o) => (typeof o === "string" ? { value: o, label: o, disabled: false } : o));
}
