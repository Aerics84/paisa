# Upgrade and rollback: 0.8.6.6 to 0.8.6.7

This page describes what changes when upgrading a Paisa installation from
`v0.8.6.6` to `v0.8.6.7`, what still works after a rollback, and how to run a
manual binary rollback.

The database change in this release is additive. The upgrade path is therefore
safe in both directions: an old binary keeps reading a migrated database, but
newer configuration data such as Argon2id password hashes may need to be
restored from a backup.

## Upgrade and rollback matrix

| Change | Upgrade 0.8.6.6 → 0.8.6.7 | Rollback to 0.8.6.6 |
|---|---|---|
| SQLite schema | Additive migration; `PRAGMA user_version` moves from `0` to `1`. Existing rows are preserved. | The old binary keeps reading the migrated database; it ignores the new `user_version` and any new columns. |
| Authentication hashes | Legacy `sha256:` hashes still verify and are replaced with `argon2id$...` on the next successful login when the configuration file is writable. | 0.8.6.6 only verifies `sha256:` hashes. After a rehash, login with the stored `argon2id$` value fails; restore the configuration backup or recreate the user. |
| Sessions | The `paisa_session` cookie is issued again at login. | Old sessions are invalid; log in again. Sessions are process-local in both versions. |
| Configuration defaults | Existing configuration files keep their values. Missing fields still fall back to the generic defaults (India). | No change; the configuration file is unchanged. |
| daisyUI classes | The `du-*` classes were removed from the stylesheets. | Custom CSS or themes that still target `du-*` classes must be restored from a backup or adjusted. |
| PWA | A service worker for the static app shell is registered in production builds. | Clear the site data in the browser or call `navigator.serviceWorker.getRegistrations()` followed by `registration.unregister()` for each registration. |

The DACH-first profile applies to new installations only: `paisa init` defaults
to `regional_profile: germany-eu` with EUR, `de-DE`, `Europe/Berlin`, a Monday
week start and a January financial year. The onboarding dialog offers the
Deutschland/EU profile first. An existing `paisa.yaml` is never rewritten by the
upgrade.

## Migration failure behavior

Schema migrations run from `paisa serve` and `paisa update` before the
application starts serving or syncing data.

- `migrations.Migrate` aborts on `ErrDatabaseNewer` (the database was written by a
  newer Paisa build) or on the first failing migration. Both `paisa serve` and
  `paisa update` log the error and exit with a non-zero status.
- Each migration and its `PRAGMA user_version` bump run in one transaction. A
  failed migration rolls back completely, so the database stays at the last
  successfully applied `user_version` and no partial schema change remains.
- The error message names the failed migration version and name, for example
  `migration 1 (baseline) failed: ...`. Backup creation failures include the
  backup target path. The database path is not repeated in the error; it is the
  `db_path` from the active `paisa.yaml` (or the `--config` file).
- Automatic backups are created only for migrations marked destructive in the
  migration list. The current baseline migration is additive and creates no
  backup. A destructive backup is written next to the database as
  `<db_path>.backup-v<version>-<UTC timestamp>` using SQLite `VACUUM INTO`.
- Backups are never deleted automatically. Keep at least the last two backups,
  and more if the database is large or the upgrade is unusual.
- After the cause is fixed, run the same command again. `Migrate` starts from the
  retained `user_version`, so the failed migration is retried; already applied
  migrations are skipped.

## Manual binary rollback

A real binary rollback is a manual release step. It is not automated by the
migration test.

1. Stop `paisa serve` (and any `paisa update` run) so no process writes to the
   database.
2. Copy the database, its `-wal`/`-shm` files if present, and `paisa.yaml` to a
   safe location before touching anything.
3. Obtain the `v0.8.6.6` binary. Either use the published `v0.8.6.6` release
   artifact or check out the tag and build from that worktree:
   `git worktree add ../paisa-v0.8.6.6 v0.8.6.6`, then build the CLI there.
4. If a login has already been upgraded to an Argon2id hash, restore the
   `paisa.yaml` backup taken before the upgrade. Otherwise remove the affected
   user account and create it again with 0.8.6.6.
5. Point the old binary at the migrated database. The additive schema is
   compatible. Verify that it starts and that the ledger, postings and prices
   are readable.
6. Clear the PWA site data in the browser, or unregister the service worker, so
   the old frontend is not served from the 0.8.6.7 app-shell cache.
7. Record the rollback in the release protocol (the release notes or PR
   checklist used for the release): date and operator, source and target
   versions (`0.8.6.7` → `0.8.6.6`), database path, `PRAGMA user_version` before
   and after, whether the configuration was restored, and the observed result.

The plan's earlier `v0.8.6.5` example is historical. `v0.8.6.6` is the rollback
target for this release.

## Verification

`internal/model/migrations/upgrade_rollback_test.go` covers the read-back path
automatically: it copies `testdata/v0.8.6.db`, runs `Migrate`, checks that
postings and prices still count correctly, reads a row through the old column
set, asserts `user_version = 1`, and runs `PRAGMA integrity_check`.

Run it with:

```bash
go test ./internal/model/migrations -run TestMigratedV086DatabaseRemainsReadable -v
```
