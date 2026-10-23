---
type: Tutorial
title: Build your first app
description: Run, understand, and customize a complete server-rendered counter.
version: 1
---

# Build your first app

Build a counter with server-owned state, HTML components, and HTMX updates.
No Node.js, templates, database, or application JavaScript needed.
Prerequisites: Go 1.24 or newer, Git, and a browser. Commands below use PowerShell.

## 1. Run a working app

```powershell
git clone https://github.com/dmundt/go-mvu.git
Set-Location go-mvu
go run .\examples\counter
```

Open <http://localhost:8080>. Click **+**, **-**, and **Reset**.
Each button sends a request; Go updates the model and returns the counter
fragment. Disable JavaScript and reload: buttons still work through ordinary
form submissions and redirects. Stop the server with Ctrl+C.

The example binds to loopback only. If port 8080 is occupied, change the
address in your copy before running; do not stop another application's server.

## 2. Make it your app

Copy the runnable example inside this checkout:

```powershell
Copy-Item .\examples\counter .\examples\mycounter -Recurse
go run .\examples\mycounter
```

This uses the checkout's Go module. For a separate application repository,
copy its three non-test Go files into an empty directory, then run:

```powershell
go mod init example.com/mycounter
go get github.com/dmundt/go-mvu
go run .
```

## 3. Understand the three files

| File | Your responsibility |
| --- | --- |
| [app.go](../../examples/counter/app.go) | Model, typed messages, pure state transitions |
| [view.go](../../examples/counter/view.go) | Full document and replaceable counter fragment |
| [main.go](../../examples/counter/main.go) | Runtime, routes, session/CSRF middleware, HTTP server |

Request flow:

```text
POST /counter/increment
  → Increment{}
  → Update(model, message)
  → persist new model
  → render CounterFragment
  → HTMX replaces #counter
```

`CounterView.Page` builds the initial document. `CounterView.Fragment` selects
the named subtree. `RenderAuto` chooses a fragment for HTMX or a redirect for
an ordinary form POST. Both paths use the same `Update`.

## 4. Add a behavior

In your copy's `app.go`, change the increment branch:

```go
case Increment:
	model.Count += 2
```

In `view.go`, change the increment button label from `"+"` to `"+2"`.
Restart the server. Click **+2** twice: count becomes **4**. Click **Reset**:
count becomes **0**. Repeat with JavaScript disabled.

Keep `Update` free of HTTP, rendering, and I/O. CAS conflicts can retry it.
For database calls or external effects, return commands instead; see
[runtime commands](web-runtime.md#commands).

## 5. Change the presentation

Change `"Go MVU counter"` in `view.go` to your application name.
Use `components.Heading`, `Paragraph`, `Card`, and `Button` for common markup.
For custom markup, use `html.El` or implement `RenderHTML`; send user text
through `out.Text`, never `out.Static`. Return rendering errors.
See [component authoring](components.md).

Preserve the fragment's `id="counter"` and action target `#counter` together.
Forms need `middleware.CSRFInput{}`; the document's `CSRFHTMX` wrapper supplies
the HTMX token. The working example already wires both.

## 6. Verify

```powershell
go test .\examples\mycounter
go vet .\examples\mycounter
```

Copied tests expect the original increment behavior. Update their expected
counts to match the intentional `+2` change, then rerun. Browser-check both
HTMX and JavaScript-disabled paths; HTTP tests alone do not prove visual UX.

## Before deployment

This is a learning app: random secrets change on restart; model state lives
in memory; diagnostics are unauthenticated. It has no login.
Do not expose it unchanged to the internet.

Use stable secret-manager keys, HTTPS and secure cookies, durable storage,
protected diagnostics, server timeouts, and graceful shutdown. Continue with
[HTTP middleware](middleware.md), [security](security.md), and
[durable storage](durable-store-outbox.md).

For working CASK persistence and generic OIDC login, continue with the
[production walkthrough](production.md).
