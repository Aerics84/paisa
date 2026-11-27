export function nextFocusIndex(current: number, total: number, shift: boolean): number {
  if (total <= 0) return -1;
  if (current < 0 || current >= total) return shift ? total - 1 : 0;
  if (shift) return current === 0 ? total - 1 : current - 1;
  return current === total - 1 ? 0 : current + 1;
}
