import type { EnMessageKey } from "./en";

/**
 * German catalog.
 *
 * The explicit `Record<EnMessageKey, string>` annotation makes type checking
 * fail when a key is missing; the object literal also fails on extra keys via
 * excess property checking. Runtime parity is asserted in i18n.test.ts.
 */
export const de: Record<EnMessageKey, string> = {
  greeting: "Hallo, {name}",
  bookings: "{count} Buchungen",
  bookings_one: "{count} Buchung",
  bookings_other: "{count} Buchungen",

  "nav.dashboard": "Übersicht",
  "nav.accounts": "Konten",
  "nav.expenses": "Ausgaben",
  "nav.income": "Einnahmen",
  "nav.more": "Mehr",

  "login.title": "Anmelden",
  "login.username": "Benutzername",
  "login.password": "Passwort",
  "login.submit": "Anmelden",

  "common.cancel": "Abbrechen",
  "common.save": "Speichern",
  "common.delete": "Löschen"
};
