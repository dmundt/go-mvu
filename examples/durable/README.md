# Durable CASK counter and outbox example

Run with `go run .\examples\durable`. Server binds only to `localhost:8084`.
go-cask objects and refs, idempotent delivery records, and a stable
session-signing key are stored under `data/` by default; set `GOMVU_DATA_DIR`
to change that directory. Current go-cask release requires Go 1.24 or newer.

Each MVU update appends an outbox event to the model. The app stores a new
immutable model object through [go-cask](https://github.com/dmundt/go-cask),
then atomically advances its model ref. Counter state and pending event share
one object, so the ref is the visible commit point; a crash before the ref
switch leaves the prior state intact and may leave an unreachable object.
A command only wakes the worker; periodic scans recover missed wake-ups. The
worker delivers events at least once, then publishes an updated object without
the acknowledged event. If it crashes after delivery but before acknowledgement,
it retries the same event ID; the file sink deduplicates that ID.

The adapter uses a process-local mutex around version-check / object-write /
ref-update. CASK object writes are content-addressed; ref writes are atomic and
reflogged, but go-cask refs do not provide cross-process compare-and-swap.
Run one writer process per CASK root. This is not a relational database, a
distributed database, or a multi-host coordination service. Add deployment
appropriate locking, backup, retention/GC, schema migration, worker leases,
bounded retries, and monitoring before production use.
