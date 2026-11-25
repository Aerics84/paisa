/**
 * English source catalog. Keys are flat and dotted by feature area.
 *
 * This catalog defines the canonical key set: `MessageKey` is derived from it,
 * and `de.ts` is typed against it so missing or extra keys fail type checking.
 */
export const en = {
  greeting: "Hello, {name}",
  bookings: "{count} bookings",
  bookings_one: "{count} booking",
  bookings_other: "{count} bookings",

  "nav.dashboard": "Dashboard",
  "nav.accounts": "Accounts",
  "nav.expenses": "Expenses",
  "nav.income": "Income",
  "nav.more": "More",

  "login.title": "Sign in",
  "login.username": "Username",
  "login.password": "Password",
  "login.submit": "Sign in",

  "common.cancel": "Cancel",
  "common.save": "Save",
  "common.delete": "Delete"
} satisfies Record<string, string>;

export type EnMessageKey = keyof typeof en;
