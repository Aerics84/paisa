export interface NavigationGuardState {
  /** Whether the editor currently has unsaved changes. */
  dirty: boolean;
  /** Whether the user has confirmed leaving. */
  confirmed?: boolean;
  /** Whether the navigation was already cancelled. */
  cancelled?: boolean;
}

/**
 * True when a navigation must not continue yet.
 *
 * A dirty page waits for the user's answer, while an already cancelled
 * navigation stays cancelled. A clean page, or a dirty page the user has
 * confirmed leaving, can proceed.
 */
export function shouldDeferNavigation({
  dirty,
  confirmed = false,
  cancelled = false
}: NavigationGuardState): boolean {
  if (cancelled) return true;
  return dirty && !confirmed;
}
