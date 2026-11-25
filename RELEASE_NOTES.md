<!--
Update this file before creating a release tag. The release workflows use it as
the GitHub release body. Keep the heading and bullets in sync with the matching
CHANGELOG section.
-->

### 0.8.6.7 (2026-11-25)

Hardening release for the frontend, authentication, data layer, localisation
and delivery. It changes defaults for new installations and removes the daisyUI
styles, so review the upgrade notes before deploying.

- Run the full frontend test suite in CI with a coverage gate, and run an
  accessibility gate that fails on new violations instead of suppressing them.
- Hash passwords with salted Argon2id while still verifying legacy `sha256:`
  hashes. Issue in-memory sessions through an `HttpOnly`, `SameSite=Strict`
  cookie, add CSRF origin checks, CSP and security headers, and throttle failed
  logins per client IP.
- Generate the TypeScript API wire types from the Go response structs, so the
  frontend no longer maintains duplicate hand-written contracts.
- Finish the Svelte 5 runes migration across routes and components, remove
  daisyUI, and enforce JavaScript and CSS asset budgets.
- Add versioned SQLite migrations with WAL-safe backups, and move configuration
  access behind a race-safe store shared by the server and CLI.
- Provide a German and English UI that follows `config.locale`, default new
  installations to the `germany-eu` profile, and add first-run onboarding with
  a choice between the German/EU and India setups.
- Ship an installable PWA app shell with an offline shell for navigations. API
  responses are never cached.
- Correct the reference documentation for first-run defaults, authentication
  and the scope of the regional profiles.

#### Upgrade notes

- Sessions live in memory, so a restart invalidates them; log in again after
  upgrading.
- Legacy `sha256:` password hashes are upgraded to Argon2id on the next
  successful login when the configuration is writable. A rollback to 0.8.6.6
  cannot verify Argon2id hashes, so restore the configuration backup or
  recreate the account.
- Existing configurations keep their previous defaults. The Germany/EU-first
  `germany-eu` defaults apply only to new installations and the onboarding
  flow. `germany-eu` covers Germany and EU-oriented defaults; it is not a full
  DACH profile.
- Custom CSS that targets daisyUI `du-*` classes must be adjusted, because
  those classes no longer exist.
- When rolling back, clear the PWA cache and unregister the service worker so
  the browser does not keep serving the 0.8.6.7 shell.

Known limitation: some copy in the advanced editor and statement-import modals
is still English. Read-only mode is enforced by the API, but the UI still
displays write controls. The maskable PWA icon is a copy of the standard icon
without a dedicated safe zone, so some Android launchers may crop it.
