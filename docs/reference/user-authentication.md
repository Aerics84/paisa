---
description: "How to secure Paisa by adding user account"
---

# User Authentication

You can setup username and password to make sure only you can access
the application. Go to `Configuration` page and expand the `User
Accounts` section. You can click the :fontawesome-solid-circle-plus:
icon to add a new username and password. Once you save the
configuration, you will get logged out if you had not logged in
earlier via another account.

It is important to understand that authentication only protects the
application user interface. If someone can access your computer and
they can access the folder where Paisa stores the ledger and database
files, they will be able to view your data.

## Implementation details

Paisa never stores your password in plain text. The configuration file only
contains a salted hash, and Paisa compares hashes whenever you log in. No one
can read the configuration file and recover the password, including you. If you
forget the password, remove the user accounts from the
[configuration](./config.md) file to regain access.

### Password hashing

Paisa hashes passwords with [Argon2id](https://en.wikipedia.org/wiki/Argon2)
using `m=65536` (64 MiB), `t=3`, and `p=2`, with a fresh 16-byte random salt for
each password. The stored value has the form
`argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>`.

Configuration files written by older releases contain legacy `sha256:` hashes
(SHA-256 applied twice). These hashes still verify. After the next successful
login Paisa replaces the stored hash with an Argon2id hash, provided the
configuration file is writable. With `readonly: true` the login succeeds and
the upgrade is retried on a later login.

### Sessions

A successful login creates a session in memory and sets the `paisa_session`
cookie. The token is 256 bits of randomness. The cookie is `HttpOnly`,
`SameSite=Strict`, scoped to `Path=/`, expires after 24 hours, and is marked
`Secure` when the request arrives over TLS. Logging out deletes the session on
the server and clears the cookie.

Sessions exist only inside the running Paisa process. Restarting the server
invalidates every session, and multiple Paisa instances do not share sessions.
There is no session database; for a single local instance this is an accepted
trade-off.

The authentication endpoints are:

- `POST /api/auth/login` with `{"username": "...", "password": "..."}` starts a
  session.
- `POST /api/auth/logout` ends the current session.
- `GET /api/auth/status` reports `auth_enabled`, `authenticated`, and the
  current `username`.

### Failed logins

Failed login attempts are throttled per client IP address using an in-memory
limiter (about 6 attempts per minute with a burst of 3). Additional attempts
receive `429 Too Many Requests`. The limiter is per process, so restarting
Paisa resets it.

### CSRF protection

The session cookie uses `SameSite=Strict`. In addition, Paisa rejects
state-changing API requests (`POST`, `PUT`, `PATCH`, and `DELETE` below `/api`)
when the `Origin` or `Referer` header names a different host. Paisa sends no
CORS headers and expects the API to be used from the same origin as the user
interface. Requests without an `Origin` or `Referer` header, such as CLI
clients, are allowed.

### Legacy `X-Auth` header

API clients can still authenticate with the deprecated `X-Auth` header. Its
value is `username:<sha256-hex>`, where `<sha256-hex>` is the hexadecimal
SHA-256 digest of the password. Paisa marks responses to these requests with
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

When Paisa receives a request over TLS, it marks the session cookie `Secure`
automatically. If a reverse proxy terminates TLS and forwards plain HTTP to
Paisa, start Paisa with `--secure-cookies` or set
`PAISA_SECURE_COOKIES=true` so the cookie is still marked `Secure`.

### Configuration and backups

The configuration file contains password hashes rather than plaintext
passwords, but it should still be treated as sensitive. Paisa redacts password
values when it creates a backup archive by default. Passwords are included
only when the backup is exported with `include_secrets=true`.
