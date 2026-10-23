# Durable OIDC application

Single-host production-oriented example, not a turnkey identity platform.
Generic OpenID Connect authorization-code login using
[`go-oidc`](https://github.com/coreos/go-oidc) and `oauth2`, example-only JSON
file-backed models and authentication grants, and CSRF-protected forms. Rate
limiting is intentionally left to application middleware or deployment policy.
No framework package imports OIDC. This example's separate module needs
Go 1.25 or newer.

## Code layout

- [main.go](main.go): process lifecycle and HTTP shutdown.
- [config.go](config.go): environment settings, key decoding, and origin checks.
- [handler.go](handler.go): OIDC discovery, storage, routes, and middleware.
- [auth.go](auth.go): login, callback, authorization, and logout.
- [app.go](app.go) and [view.go](view.go): MVU updates and HTML composition.

## Configure and run

Register a confidential OIDC client with your provider:

- Authorization code flow and PKCE S256.
- Exact redirect URI `https://your-app.example/oidc/callback`.
- Scope `openid`; no refresh tokens or offline access needed.
- Restrict permitted users/tenant using provider configuration.

PowerShell, from repository root:

```powershell
Set-Location examples\production
$env:GOMVU_BASE_URL = "https://your-app.example"
$env:OIDC_ISSUER = "https://your-provider.example"
$env:OIDC_CLIENT_ID = "your-client-id"
# Supply OIDC_CLIENT_SECRET, GOMVU_SESSION_KEY, and GOMVU_CSRF_KEY
# from your secret manager, not source control or command history.
$env:GOMVU_DATA_DIR = "D:\AppData\go-mvu"
$env:GOMVU_LISTEN = "127.0.0.1:8085"
go run .
```

Session and CSRF keys must be distinct, stable, base64-encoded random byte
strings, at least 32 decoded bytes each. Missing/invalid configuration fails
startup. The public origin may have no path prefix, query, fragment, or userinfo.
Issuer URLs may include the provider's path. Discovery and token requests have
bounded timeouts; discovery failure prevents startup.

Terminate HTTPS at a reverse proxy that preserves the configured public
`Host`, strips untrusted forwarding headers, and reaches only the private
listener. The executable serves HTTP, not TLS. Cookies are Secure for an HTTPS
public origin. Configure gateway-level login/abuse limits for your deployment.

For local development only, set `GOMVU_DEVELOPMENT=true` and
`GOMVU_BASE_URL=http://127.0.0.1:8085`; register that exact callback URI.
HTTP issuer/endpoints are accepted only on loopback in this explicit mode.
Never use development mode for an internet-facing deployment.

## Verify behavior

1. Open `/`: redirected to login. Submit login form, authenticate with provider.
2. Increment counter. HTMX returns only `#counter`; with JavaScript disabled,
   ordinary form POST redirects to the full page.
3. Stop with Ctrl+C and restart using the same data directory and secrets.
   Counter and unexpired authentication survive.
4. Log out and restart. Replaying the old cookie must not restore authentication.
   Logging in again with the same issuer/subject restores the account counter.

```powershell
go test ./...
go vet ./...
```

Tests use an isolated, signing OIDC provider fixture; no real provider or
credentials required. They verify code exchange/PKCE, nonce, issuer, audience,
expiry, signature, session binding/rotation, callback replay, CSRF, full-page
and fragment paths, durable restart/logout, and storage errors. Configure
login and abuse limits in existing middleware or at the deployment gateway.
For manual browser checks:

```powershell
$env:GOMVU_BROWSER_TEST = "true"
go test -v -run '^TestBrowserFixture$'
Remove-Item Env:GOMVU_BROWSER_TEST
```

Open printed loopback URL within two minutes. This test-only provider logs in
a synthetic identity automatically; it is never part of the executable.

## Explicit limits

- Any identity admitted by the configured issuer/client can log in. No user
  management, passwords, account recovery, RBAC, or organization policy ships.
- Issuer + subject identify the account; email is not an authorization key.
  Page access is protected at HTTP boundary; ownership and durable grant
  expiry are rechecked on every transition attempt/follow-up.
- Auth expires at the earlier of ID-token expiry and eight hours. No silent
  renewal, access-token storage, refresh-token storage, or provider API calls.
- Logout is local application logout, not provider logout or global logout.
  Durable grant deletion denies copied cookies after restart; already admitted
  requests are not canceled. External provider sessions may log in again.
- Login attempts are bounded to 256, session-bound, single-use, expire after
  five minutes, and are lost on restart. Start a new login after restart.
- JSON file storage coordinates writers only within one process. No horizontal
  scaling, network filesystem, automatic retention, encrypted storage, or
  multi-model transactions. The store is an example, not a framework adapter.
- No process-local rate limiter is included. Configure login-abuse and
  per-account quotas in existing middleware or at the trusted deployment edge;
  do not trust arbitrary forwarded identity headers.
- SIGINT/SIGTERM stops acceptance and drains HTTP work for up to 15 seconds
  before closing stores. No asynchronous command runner is used.

Deployment checklist: [production walkthrough](../../docs/guides/production.md).
