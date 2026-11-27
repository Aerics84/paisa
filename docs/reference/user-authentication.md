---
description: "How to secure Paisa by adding user account"
---

# User Authentication

You can setup username and password to make sure only you can access
the application. Go to `Configuration` page and expand the `User
Accounts` section. You can click the :fontawesome-solid-circle-plus:
icon to add a new username and password. Saving a changed set of accounts or a
new password logs out every session, so you must sign in again; unrelated
configuration edits leave sessions untouched.

Authentication controls access through Paisa's UI and API. It does not
encrypt the ledger or database files; anyone with filesystem access can
read them.

## Implementation details

Paisa never stores your password in plain text. The configuration file contains
a hash, not the password: new hashes are salted Argon2id, while older `sha256:`
hashes are unsalted double SHA-256 and are upgraded on the next successful
login. If you forget the password, remove the user accounts from the
[configuration](./config.md) file to regain access.

### Password hashing

Paisa hashes passwords with [Argon2id](https://en.wikipedia.org/wiki/Argon2)
using `m=65536` (64 MiB), `t=3`, and `p=2`, with a fresh 16-byte random salt for
each password. The stored value has the form
`argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>`.

Configuration files written by older releases contain legacy `sha256:` hashes
(SHA-256 applied twice without a salt). These hashes still verify. After the
next successful login Paisa replaces the stored hash with an Argon2id hash,
provided the configuration file is writable. With `readonly: true` the login
succeeds and the upgrade is retried on a later login.

### Sessions

A successful login creates a session in memory and sets the `paisa_session`
cookie. The token is 256 bits of randomness. The cookie is `HttpOnly`,
`SameSite=Strict`, scoped to `Path=/`, and expires after 24 hours. Use
`--secure-cookies` or `PAISA_SECURE_COOKIES=true` to mark it `Secure` when
HTTPS is terminated by a reverse proxy (see below). Logging out deletes the
session on the server and clears the cookie.

Sessions exist only inside the running Paisa process. Restarting the server
invalidates every session, and multiple Paisa instances do not share sessions.
There is no session database; for a single local instance this is an accepted
trade-off.

Saving a configuration that adds or removes a user account, or that changes a
password, revokes every existing session, including sessions for accounts that
were not modified. Other configuration edits keep current sessions. A request
whose cookie names an account that no longer exists is treated as
unauthenticated even if the session was not revoked.

The authentication endpoints are:

- `POST /api/auth/login` with `{"username": "...", "password": "..."}` starts a
  session.
- `POST /api/auth/logout` ends the current session.
- `GET /api/auth/status` reports `auth_enabled`, `authenticated`, and the
  current `username`.
- `POST /api/auth/hash` with `{"password": "..."}` returns an Argon2id hash for
  the Configuration page. It is reachable without a session only during
  bootstrap (when no accounts exist); once accounts exist it requires
  authentication. Passwords must be at least 8 characters long.

### Failed logins

Failed login attempts are throttled per client IP address using an in-memory
limiter (about 6 attempts per minute with a burst of 3). Additional attempts
receive `429 Too Many Requests`. The limiter is per process, so restarting
Paisa resets it.

### CSRF protection

The session cookie uses `SameSite=Strict`. In addition, Paisa rejects
state-changing API requests (`POST`, `PUT`, `PATCH`, and `DELETE` below `/api`)
when the `Origin` header names a different host. `Referer` is checked only as a
fallback when `Origin` is absent. Paisa sends no CORS headers and expects the
API to be used from the same origin as the user interface. Requests without
either header, such as CLI clients, are allowed.

### Security headers

Every response, including API responses, carries a Content-Security-Policy that
allows only same-origin scripts plus the hashes of Paisa's inline bootstrap
scripts, and forbids framing (`frame-ancestors 'none'`). `'unsafe-eval'` is
allowed because the user-authored Handlebars templates on the import page are
compiled in the browser with `new Function`; removing it silently disables the
import preview. Inline scripts remain limited to the hashed bootstrap script.
If the inline-script
hash extraction fails during startup, the server falls back to
`'unsafe-inline'` for that build and logs the fallback instead of shipping a
broken UI. Paisa also sends
`X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`,
`Referrer-Policy: no-referrer` and a `Permissions-Policy` that disables camera,
microphone and geolocation. These headers are set by the server and are not
configurable.

### Legacy `X-Auth` header

While an account still stores a legacy `sha256:` hash, API clients can
authenticate with the deprecated `X-Auth` header. Its value is
`username:<sha256-hex>`, where `<sha256-hex>` is the hexadecimal SHA-256 digest
of the password. Paisa marks responses to these requests with
`Deprecation: true` and a `Sunset` header, and logs a warning. The header will
be removed in 0.10.0. Once a legacy password hash has been upgraded to
Argon2id, the old `X-Auth` value no longer authenticates.

### HTTPS and reverse proxies

!!! warning

    If you run Paisa on a server and access it over the public internet,
    make sure to use it over **https** and use a **strong password**. If you
    run it over plain http, [man in the
    middle](https://en.wikipedia.org/wiki/Man-in-the-middle_attack)
    attack can be performed to obtain the data including your password and
    session token.

`paisa serve` has no TLS listener; it only serves plain HTTP. It binds to
`127.0.0.1` by default. If the reverse proxy runs on another host, start Paisa
with `--host 0.0.0.0` and restrict that port to the proxy. Expose it through
a reverse proxy that terminates HTTPS and forwards requests to Paisa. Start
Paisa with `--secure-cookies` or set `PAISA_SECURE_COOKIES=true`: this is the
supported way to mark the session cookie `Secure` even though the proxied
request reaches Paisa over plain HTTP. Without that flag, the built-in server
does not mark the session cookie `Secure`.

### Configuration and backups

The configuration file contains password hashes rather than plaintext
passwords, but it should still be treated as sensitive. Paisa redacts password
values when it creates a backup archive by default. Passwords are included
only when the backup is exported with `include_secrets=true`.
