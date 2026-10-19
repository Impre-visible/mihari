// Context sizes a model is usually trained for, from 4K to 1M tokens.
export const CONTEXT_STEPS = [4096, 8192, 16384, 32768, 65536, 131072, 262144, 524288, 1048576];

// A token count as people say it: 32K, 1M.
export function tokens(count: number) {
  return count >= 1048576 ? `${Math.round(count / 104857.6) / 10}M` : `${Math.round(count / 1024)}K`;
}
