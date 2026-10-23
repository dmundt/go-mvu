---
type: Guide
title: Deploy a durable authenticated application
description: Connect CASK persistence, generic OIDC, middleware, and single-host operations.
version: 2
---

# Deploy a durable authenticated application

Start with [your first app](first-app.md), then use the runnable
[production example](../../examples/production/README.md).
It integrates public APIs without adding a database, ORM, or identity system
to the framework core.

## 1. Choose deployment scope

This example supports **one Go process on one host with a local persistent
volume**. [CASK adapter](../../adapters/caskstore/README.md) holds an exclusive
directory lock; a second owner fails startup instead of racing CAS operations.
Do not deploy multiple replicas over the same volume.

For horizontal scaling, supply a genuinely shared, transactional
`store.Store`, shared authentication/revocation policy, and distributed
quota enforcement. A reverse proxy alone does not make local state distributed.

Persisted MVU models are application state, not a domain database abstraction.
Use an application repository for domain storage when needed; perform I/O in
decoders/services/commands, never in pure `Update`. Returning a command is not
a durable enqueue; see [outbox boundaries](durable-store-outbox.md).

## 2. Configure identity

Register an OIDC confidential client with exact HTTPS callback
`/oidc/callback` and PKCE S256 support. Configure issuer, client ID, secret,
and allowed users/tenant at your provider.

The example uses established `go-oidc` ID-token verification and `oauth2` code
exchange, not hand-rolled JWT validation. It validates issuer, audience,
signature, expiry, nonce, session-bound single-use state, and PKCE.
Identity key combines issuer and subject, never mutable email.

Login rotates the opaque session ID. Private pages require authentication
before model initialization; transitions recheck account ownership and the
durable authentication grant on every CAS retry and command follow-up.
Do not treat a session ID, HTMX header, or forwarded address as authentication.

Local logout deletes the durable grant before invalidating the cookie.
The old cookie remains unauthenticated after restart, even though framework
cookie revocations themselves are in memory. This does not cancel already
admitted work or terminate provider sessions. No refresh tokens are stored.
When ID-token expiry is reached, users log in again.

## 3. Provide stable configuration

Follow [example environment setup](../../examples/production/README.md#configure-and-run).
Supply separate random session/CSRF secrets from a secret manager, not source
control. Keep stable keys and a stable data directory across restarts.
Set explicit data-directory ACLs; model/authentication history is not encrypted.

The example uses separate Go modules: root and CASK adapter Go 1.24,
OIDC example Go 1.25. Framework packages retain stdlib-only dependencies; the
root module also includes CASK for its separate durable outbox example.
Dependency-module `replace` directives are not inherited;
the example explicitly replaces both local modules. Nested modules need their
own release tags before independent published consumption.

## 4. Put HTTPS and abuse controls in front

Bind HTTP to a private listener. TLS proxy must preserve configured public
Host and prevent direct public access to the backend.
Keep session cookies Secure and HttpOnly; use explicit loopback-only
development mode only during local testing.

Middleware supplies security headers, request IDs, structured path-only logs,
recovery, 64 KiB body limit, cooperative 15-second request timeout, no-store
private responses, and CSRF. Static HTMX sits outside session/CSRF wrappers.
Callback responses use `Referrer-Policy: no-referrer`; avoid logging query
strings, authorization codes, tokens, and cookies at your proxy too.

Existing `middleware.RateLimit` is a bounded fixed-window limiter, not a
missing feature. Example uses 20 login-route requests/minute/direct peer and
120 private-route requests/minute/account; denial is 429 with `Retry-After`.
Key-table saturation is 503. Per-process limits reset on restart and allow
boundary bursts. Behind a proxy, peer identity is the proxy, not the browser.

Use gateway/WAF controls for login attacks and distributed/global quotas.
Configure trusted forwarding at the deployment edge; never trust arbitrary
`X-Forwarded-For`. Tune quotas and admission limits from measured traffic; do
not silently turn a full limiter table into unrestricted access.

## 5. Operate durable storage

- Drain requests and close stores before a consistent whole-directory backup.
- Protect backups and test restores on a separate local directory.
- Monitor disk space, write errors, authentication failures, CAS conflicts,
  latency, and 429/503 responses. Alert on uncertain publication errors.
- JSON models need explicit application migrations; unknown record schema
  fails rather than initializing an empty model.
- Deletion is logical. Immutable objects/reflogs retain history; define
  retention and physical cleanup before storing personal data.
- No automated CASK GC runs. Do not sweep active repositories without a
  coordinated reference/root retention protocol.
- A failed HTTP response may follow a committed transition. Reconcile before
  replay; external effects need stable idempotency keys and an outbox.

## 6. Validate before release

Run each module independently; root `go test ./...` skips nested modules:

```powershell
go test ./...
go vet ./...
Push-Location adapters\caskstore
go test ./...
go vet ./...
Pop-Location
Push-Location examples\production
go test ./...
go vet ./...
Pop-Location
```

Also exercise your actual identity provider, HTTPS proxy, expired credentials,
restart, copied-cookie logout denial, storage failure, and graceful shutdown.
Browser-check computed styling and both JavaScript-enabled and disabled forms.
The test-only OIDC fixture proves integration logic, not provider availability
or your deployment's access policy.
