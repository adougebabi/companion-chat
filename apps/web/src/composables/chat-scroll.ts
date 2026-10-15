export const NEAR_BOTTOM_PX = 72;

export function isNearBottom(metrics: { scrollTop: number; scrollHeight: number; clientHeight: number }, threshold = NEAR_BOTTOM_PX): boolean {
  return metrics.scrollHeight - metrics.scrollTop - metrics.clientHeight <= threshold;
}

export function scrollTopAfterPrepend(previousScrollHeight: number, previousScrollTop: number, nextScrollHeight: number): number {
  return previousScrollTop + Math.max(0, nextScrollHeight - previousScrollHeight);
}
