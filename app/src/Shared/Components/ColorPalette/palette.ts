import { useState } from 'react';

export interface ColorPair {
  bg: string;
  text: string;
}

/**
 * 100 hand-curated Tailwind (background, text) pairs. Every pair has been
 * chosen for readable contrast (WCAG AA normal-text or better).
 *
 * All class names are written as literal strings so Tailwind's JIT can
 * detect and include them at build time.
 */
export const COLOR_PAIRS: ColorPair[] = [
  // red (5)
  { bg: 'bg-red-50',  text: 'text-red-900' },
  { bg: 'bg-red-100', text: 'text-red-800' },
  { bg: 'bg-red-200', text: 'text-red-900' },
  { bg: 'bg-red-700', text: 'text-white' },
  { bg: 'bg-red-900', text: 'text-white' },
  // orange (5)
  { bg: 'bg-orange-50',  text: 'text-orange-900' },
  { bg: 'bg-orange-100', text: 'text-orange-800' },
  { bg: 'bg-orange-200', text: 'text-orange-900' },
  { bg: 'bg-orange-700', text: 'text-white' },
  { bg: 'bg-orange-900', text: 'text-white' },
  // amber (5)
  { bg: 'bg-amber-50',  text: 'text-amber-900' },
  { bg: 'bg-amber-100', text: 'text-amber-800' },
  { bg: 'bg-amber-200', text: 'text-amber-900' },
  { bg: 'bg-amber-700', text: 'text-white' },
  { bg: 'bg-amber-900', text: 'text-white' },
  // yellow (5)
  { bg: 'bg-yellow-50',  text: 'text-yellow-900' },
  { bg: 'bg-yellow-100', text: 'text-yellow-800' },
  { bg: 'bg-yellow-200', text: 'text-yellow-900' },
  { bg: 'bg-yellow-700', text: 'text-white' },
  { bg: 'bg-yellow-900', text: 'text-white' },
  // lime (5)
  { bg: 'bg-lime-50',  text: 'text-lime-900' },
  { bg: 'bg-lime-100', text: 'text-lime-800' },
  { bg: 'bg-lime-200', text: 'text-lime-900' },
  { bg: 'bg-lime-700', text: 'text-white' },
  { bg: 'bg-lime-900', text: 'text-white' },
  // green (5)
  { bg: 'bg-green-50',  text: 'text-green-900' },
  { bg: 'bg-green-100', text: 'text-green-800' },
  { bg: 'bg-green-200', text: 'text-green-900' },
  { bg: 'bg-green-700', text: 'text-white' },
  { bg: 'bg-green-900', text: 'text-white' },
  // emerald (5)
  { bg: 'bg-emerald-50',  text: 'text-emerald-900' },
  { bg: 'bg-emerald-100', text: 'text-emerald-800' },
  { bg: 'bg-emerald-200', text: 'text-emerald-900' },
  { bg: 'bg-emerald-700', text: 'text-white' },
  { bg: 'bg-emerald-900', text: 'text-white' },
  // teal (5)
  { bg: 'bg-teal-50',  text: 'text-teal-900' },
  { bg: 'bg-teal-100', text: 'text-teal-800' },
  { bg: 'bg-teal-200', text: 'text-teal-900' },
  { bg: 'bg-teal-700', text: 'text-white' },
  { bg: 'bg-teal-900', text: 'text-white' },
  // cyan (5)
  { bg: 'bg-cyan-50',  text: 'text-cyan-900' },
  { bg: 'bg-cyan-100', text: 'text-cyan-800' },
  { bg: 'bg-cyan-200', text: 'text-cyan-900' },
  { bg: 'bg-cyan-700', text: 'text-white' },
  { bg: 'bg-cyan-900', text: 'text-white' },
  // sky (5)
  { bg: 'bg-sky-50',  text: 'text-sky-900' },
  { bg: 'bg-sky-100', text: 'text-sky-800' },
  { bg: 'bg-sky-200', text: 'text-sky-900' },
  { bg: 'bg-sky-700', text: 'text-white' },
  { bg: 'bg-sky-900', text: 'text-white' },
  // blue (5)
  { bg: 'bg-blue-50',  text: 'text-blue-900' },
  { bg: 'bg-blue-100', text: 'text-blue-800' },
  { bg: 'bg-blue-200', text: 'text-blue-900' },
  { bg: 'bg-blue-700', text: 'text-white' },
  { bg: 'bg-blue-900', text: 'text-white' },
  // indigo (5)
  { bg: 'bg-indigo-50',  text: 'text-indigo-900' },
  { bg: 'bg-indigo-100', text: 'text-indigo-800' },
  { bg: 'bg-indigo-200', text: 'text-indigo-900' },
  { bg: 'bg-indigo-700', text: 'text-white' },
  { bg: 'bg-indigo-900', text: 'text-white' },
  // violet (5)
  { bg: 'bg-violet-50',  text: 'text-violet-900' },
  { bg: 'bg-violet-100', text: 'text-violet-800' },
  { bg: 'bg-violet-200', text: 'text-violet-900' },
  { bg: 'bg-violet-700', text: 'text-white' },
  { bg: 'bg-violet-900', text: 'text-white' },
  // purple (5)
  { bg: 'bg-purple-50',  text: 'text-purple-900' },
  { bg: 'bg-purple-100', text: 'text-purple-800' },
  { bg: 'bg-purple-200', text: 'text-purple-900' },
  { bg: 'bg-purple-700', text: 'text-white' },
  { bg: 'bg-purple-900', text: 'text-white' },
  // fuchsia (5)
  { bg: 'bg-fuchsia-50',  text: 'text-fuchsia-900' },
  { bg: 'bg-fuchsia-100', text: 'text-fuchsia-800' },
  { bg: 'bg-fuchsia-200', text: 'text-fuchsia-900' },
  { bg: 'bg-fuchsia-700', text: 'text-white' },
  { bg: 'bg-fuchsia-900', text: 'text-white' },
  // pink (5)
  { bg: 'bg-pink-50',  text: 'text-pink-900' },
  { bg: 'bg-pink-100', text: 'text-pink-800' },
  { bg: 'bg-pink-200', text: 'text-pink-900' },
  { bg: 'bg-pink-700', text: 'text-white' },
  { bg: 'bg-pink-900', text: 'text-white' },
  // rose (5)
  { bg: 'bg-rose-50',  text: 'text-rose-900' },
  { bg: 'bg-rose-100', text: 'text-rose-800' },
  { bg: 'bg-rose-200', text: 'text-rose-900' },
  { bg: 'bg-rose-700', text: 'text-white' },
  { bg: 'bg-rose-900', text: 'text-white' },
  // slate (5)
  { bg: 'bg-slate-50',  text: 'text-slate-900' },
  { bg: 'bg-slate-100', text: 'text-slate-800' },
  { bg: 'bg-slate-200', text: 'text-slate-900' },
  { bg: 'bg-slate-700', text: 'text-white' },
  { bg: 'bg-slate-900', text: 'text-white' },
  // gray (5)
  { bg: 'bg-gray-50',  text: 'text-gray-900' },
  { bg: 'bg-gray-100', text: 'text-gray-800' },
  { bg: 'bg-gray-200', text: 'text-gray-900' },
  { bg: 'bg-gray-700', text: 'text-white' },
  { bg: 'bg-gray-900', text: 'text-white' },
  // stone (5)
  { bg: 'bg-stone-50',  text: 'text-stone-900' },
  { bg: 'bg-stone-100', text: 'text-stone-800' },
  { bg: 'bg-stone-200', text: 'text-stone-900' },
  { bg: 'bg-stone-700', text: 'text-white' },
  { bg: 'bg-stone-900', text: 'text-white' },
];

/**
 * Mutable pool — a random pair is removed on each pick. When exhausted, the
 * pool is refilled so picks never return undefined. Two concurrent renders
 * therefore get different colours until all 100 have been consumed.
 */
let availablePairs: ColorPair[] = [...COLOR_PAIRS];

/**
 * Stable-key → assigned-pair cache. Entries here survive StrictMode's
 * double-mount and React's unmount/remount cycles: the same key always
 * resolves to the same colour pair.
 */
const pairByKey = new Map<string, ColorPair>();

const takeRandomPair = (): ColorPair => {
  if (availablePairs.length === 0) {
    availablePairs = [...COLOR_PAIRS];
  }
  const idx = Math.floor(Math.random() * availablePairs.length);
  const [pair] = availablePairs.splice(idx, 1);
  return pair;
};

/**
 * Pick a unique (bg, text) pair from the pool. Picked pairs are removed
 * and won't be returned again until the pool is exhausted and refilled.
 *
 * Pass `stableKey` (e.g. an entity id) to make the pick idempotent: the
 * same key will always resolve to the same pair across the app lifetime.
 */
export const pickColorPair = (stableKey?: string): ColorPair => {
  if (stableKey) {
    const existing = pairByKey.get(stableKey);
    if (existing) return existing;
    const pair = takeRandomPair();
    pairByKey.set(stableKey, pair);
    return pair;
  }
  return takeRandomPair();
};

/**
 * Reset the pool — useful in tests or when you want to clear key
 * assignments. Normal application code should not need this.
 */
export const resetColorPool = (): void => {
  availablePairs = [...COLOR_PAIRS];
  pairByKey.clear();
};

/**
 * React hook: returns a stable colour pair for the lifetime of the
 * component. Pass a `stableKey` to share the pair across mounts.
 */
export const useColorPair = (stableKey?: string): ColorPair => {
  const [pair] = useState<ColorPair>(() => pickColorPair(stableKey));
  return pair;
};
