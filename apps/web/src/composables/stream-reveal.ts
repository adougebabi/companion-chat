const segmenter = typeof Intl !== "undefined" && "Segmenter" in Intl
  ? new Intl.Segmenter("zh", { granularity: "grapheme" })
  : null;

export const STREAM_MAX_LAG = 24;

export function graphemes(value: string): string[] {
  if (!value) return [];
  if (!segmenter) return Array.from(value);
  return [...segmenter.segment(value)].map((part) => part.segment);
}

export function revealStep(target: string, shown: string, options: { animate: boolean; reducedMotion: boolean; maxLag?: number }): string {
  if (!options.animate || options.reducedMotion) return target;
  if (!target.startsWith(shown)) return target;
  const targetParts = graphemes(target);
  const shownCount = graphemes(shown).length;
  const lag = targetParts.length - shownCount;
  if (lag <= 0) return target;
  if (lag > (options.maxLag ?? STREAM_MAX_LAG)) return target;
  return targetParts.slice(0, shownCount + 1).join("");
}
