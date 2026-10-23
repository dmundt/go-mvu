---
type: Tutorial
title: Build your first app
description: Run a counter and make a small change.
version: 2
---

# Build your first app

Want only a page and a heading? Start with [Hello World](../../examples/hello/):
`go run .\examples\hello`, then open <http://localhost:8085>. It needs no MVU
runtime, session, or HTMX. Continue below when you need changing state.

Run a working app first. You do not need to set up sessions, themes, or
middleware to make your first change.

You need Go 1.24 or newer, Git, and a browser. Commands use PowerShell.

## 1. Run the counter

```powershell
git clone https://github.com/dmundt/go-mvu.git
Set-Location go-mvu
go run .\examples\counter
```

Open <http://localhost:8080>. Try **+**, **-**, and **Reset**. The counter
updates without a page reload. Turn off JavaScript and try again; the buttons
still work.

## 2. Make one change

Open `examples/counter/app.go` and change the `Increment` case:

```go
case Increment:
	model.Count += 2
```

In `examples/counter/view.go`, change the increment button label from `"+"` to
`"+2"`. Save and restart the app. Click **+2** twice; the count should be 4.

That is enough for a first change. `app.go` holds the counter's values and
behavior. `view.go` holds what people see. Leave `main.go` alone for now: it
sets up the server and request protection.

## 3. Start your own copy

From the repository root:

```powershell
Copy-Item .\examples\counter .\examples\mycounter -Recurse
go run .\examples\mycounter
```

For a separate Go project, copy the counter's non-test `.go` files into your
project, then run:

```powershell
go mod init example.com/mycounter
go get github.com/dmundt/go-mvu
go run .
```

## 4. Add a form

Try the [contact form example](../../examples/contact/). It shows several
fields, validation, helpful errors, and keeps entered values when a form
needs correction. The same form works with and without JavaScript.

## Before putting it online

For a larger learning example, try the [admin dashboard](../../examples/admin/).
It combines filtering, paging, editable records, validation, and independent
page regions. It has no login or durable storage.

The counter is for learning: it stores data in memory, changes secrets when
restarted, and has no login. Do not expose it unchanged. Read the
[production guide](production.md) and [security guide](security.md) first.
