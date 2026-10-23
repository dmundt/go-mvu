# Reliability and operational hardening

## Validation gates

[Go CI](../../.github/workflows/test.yml) runs on pushes and pull requests.
Linux race testing explicitly sets `CGO_ENABLED=1` and runs all
packages with a five-minute test timeout. `go vet` is a separate required step.
Configure these jobs as required branch checks in the hosting repository;
workflow files alone do not enforce branch protection.

Six isolated [fuzz jobs](../../.github/workflows/fuzz.yml) run for 30 seconds
with two workers each, including manual dispatch:
trusted proxy headers, traceparent, gzip negotiation, locale negotiation,
action IDs, and signed session cookies. Failing corpus inputs are uploaded
as artifacts. Keep minimized reproducers as regression corpus entries after
investigation. Ordinary `go test ./...` also executes all seed corpora.

Cookie seeds are deterministic across fuzz subprocesses: randomly generated
valid cookies must not be compared against independently generated worker
cookies. Invariants concern verified signed identity and expiry, not equivalent
serialized cookie spellings.

Browser CI installs Chromium and runs `npm run test:browser`, including the
ordering regression in all four viewport/mode projects. `npm run test:ordering`
runs only this regression using the same Playwright configuration and server
lifecycle. The test uses real example handlers and gated browser interception,
not timing guesses. It holds the page-2 response, requests page 9, then releases page 2 and verifies
that page 9 remains visible and full-page reload works. No application
JavaScript is introduced; browser automation lives only in the test runner.
The test observes the canceled old request and the newer HTMX settle event;
network-idle delays are not used as proof of response ordering.

For local Windows browser runs with installed Chrome:

```powershell
$env:PLAYWRIGHT_CHANNEL = "chrome"
$env:GOMVU_BROWSER_BASE_PORT = "20080" # Optional alternate four-port range.
npm run test:ordering
```

For local race testing, use a supported Go installation and C compiler,
including an existing Linux/WSL environment. Do not treat repeated non-race
tests as a substitute for race detection.

## Durable actions and effects

[Action ledger example](../../examples/actionledger/README.md) keeps model
transition, scoped action receipt, and pending effect in one synced application
WAL frame. Restart preserves duplicate suppression and pending effects.
The bounded ledger detects corrupt frames, recovers only incomplete tails,
and fails closed after uncertain writes.

The consumer still needs a durable inbox keyed by stable effect ID. Delivery
is at-least-once: success before a missing completion receipt can redeliver.
This is a single-process, application-owned example, not a new framework store
or a claim that ordinary CAS plus commands are atomic.
Filesystem sync, directory durability, lock ownership, and capacity caveats
are explicit in the example. Do not delete a ledger or its receipt history to
retry an uncertain action.

## Authorization before replay

Authentication, session checks, CSRF, and replay access authorization belong
outside action recording. Transition authorization alone is insufficient:
replay deliberately skips the transition.

Use a stable scope containing the required account, tenant, and session
identity, derived from authenticated server state. `IdempotencyOptions.Validate`
must be read-only and reentrant. It checks current permission and input before
reservation and again immediately before duplicate replay, including after
waiting for an in-flight action. Scope is likewise rechecked after waiting.

Tests cover logout, permission revocation, copied invalidated/expired cookies,
changed principal, changed tenant, changed session, and access revoked while
a duplicate waits. Different legitimate scopes execute their own actions;
they never receive another scope's cached response.

No callback can eliminate the race between its final authorization check and
subsequent response delivery. Stronger guarantees require application
transactional access checks and an explicit policy for already-admitted work.
Session ID presence alone is not proof of current authorization.

## Shutdown ownership

The counter and users examples now use
[`examples/internal/server`](../../examples/internal/server/server.go).
It owns `http.Server` startup and shutdown, explicit header/idle timeouts, and
one shutdown budget:

1. Stop accepting HTTP requests; wait for admitted requests.
2. Stop command acceptance and drain accepted commands.
3. Only after successful drains, close application dependencies in reverse order.

`server.Config.Drain` accepts callbacks such as a bounded runner's `Close`.
`Dependencies` are `io.Closer`s. HTTP/serve/drain/close errors are returned,
including joined errors. A timed-out HTTP drain force-closes connections;
dependency closure is withheld when active work may still use them.
`ErrDependenciesOpen` explicitly reports that ownership remains with the caller.
Do not call a timeout-shaped return "clean shutdown".
The budget bounds HTTP shutdown and supplies drain deadlines, not arbitrary
cleanup execution. Drain hooks must honor context cancellation. Dependency
`Close` calls are synchronous and have no enforceable deadline; an uncooperative
hook or closer can delay return indefinitely. Run does not abandon cleanup in
background goroutines. Process-level termination remains deployment-owned.

The bounded runner uses one shared worker-completion channel. Repeated timed-out
`Close` calls do not create more waiter goroutines. `Close` stops acceptance and
drains; timeout limits waiting, not execution. Jobs retain their own deadlines.
An effect that ignores context cannot be forcibly terminated safely.

## Operational counters

`DiagnosticsSnapshot.Counters` exposes fixed event names, never user/route/session
labels:

- `admission_overload`, `rate_limited`, `rate_capacity`
- `stale_action`, `authorization_rejected`
- `committed_effect_failure`, `response_aborted`
- `idempotency_capacity`, `idempotency_wait_capacity`
- `revocation_capacity`, `command_overload`

Install `Diagnostics.Middleware` outside the relevant policies and runtime.
It attaches a concurrency-safe event sink through stdlib-only `httpx` context
contracts. Signals work even without `RuntimeConfig.ObserveOutcome`.
Async jobs retain the sink after request cancellation.
`Diagnostics.RecordEvent(httpx.Event)` supports application workers and rejects
unknown enum values without allocating a new label. Snapshot copies and Reset
are synchronized. A missing counter means zero.

Counters describe events, not unique requests: one failed command and one
aborted response can both arise from a committed action. Async effect failures
may occur after HTTP completion; do not infer them from HTTP 500 alone.
Existing `ObserveOutcome` retains independent `Committed` truth.
Mount diagnostics on an authenticated/internal route and export through
deployment-owned monitoring. Alert on counter increases and saturation:
repeated 503s are not the only indication of dropped work.
Choose thresholds from workload budgets; do not bake universal alert thresholds
or a telemetry vendor into the framework.

### Saturated idempotency costs

`go test ./middleware -run='^$' -bench=BenchmarkIdempotencySaturation -benchmem -cpu=4`
measures parallel replay and rejected new reservations against full tables of
128, 4096, and 16384 entries. Prefill is outside the timed region; an explicit
benchmark error handler avoids measuring expected-503 logging overhead.
Expiry scans run only when the earliest completed receipt expires, not on every
request. Pending actions remain unevictable. Expiry-boundary scans are still
O(entries); steady-state benchmark results do not describe that tail latency.

## Response ordering and multiple tabs

The users pagination example uses existing attribute passthrough:

```go
Attrs: html.Attrs{html.A("hx-sync", "closest main:replace")}
```

This groups navigation requests under a stable `main` element. A newer request
aborts the older browser request before replacing its response. Keep the group
outside the swapped subtree. Plain links still navigate without HTMX.

Use `replace` for navigation/search, not commands with non-idempotent effects:
aborting a browser request does not undo a committed server mutation.
For mutations, serialize requests from stable controls, disable controls while
busy, and use stable action IDs/expected versions as appropriate.
Queued callbacks whose source element is removed by a swap may not execute;
do not rely on queues attached to transient controls to preserve user intent.

HTMX synchronization is document-local, not cross-tab coordination.
Two-tab tests submit the same expected version concurrently: one CAS succeeds,
the other returns 409 without rebasing stale intent. Applications expose the
version with the rendered form and parse it through `Route.ExpectedVersion`.
Show conflict/reload UI; never automatically replay an unsafe mutation.
Cross-tab live updates need an explicit application notification strategy;
no middleware can guarantee every tab displays the newest model automatically.
