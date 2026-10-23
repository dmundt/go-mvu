---
type: Guide
title: Durable storage and outbox
description: Persist model transitions with retryable, idempotent external effects.
version: 1
---

# Durable storage and outbox

The framework's default `store.Memory` is process-local. Replace it with an
application-owned `store.Store` when model state must survive restarts. See the
[durable example](../../examples/durable/README.md) for an adapter backed by
go-cask's filesystem object store and refs.

## Transaction boundary

An `mvu.Command` runs after the model transition has committed. A process can
stop between those steps, so a command alone is not a durable job queue.
Persist pending effect records in the same commit unit as the model transition.
Treat wake-up signals as hints; scan pending records after startup and
periodically.

The example appends each event to `CounterModel.Outbox` in `Update`. The store
serializes model and pending event together into one immutable go-cask object,
then atomically advances a ref to its digest. The ref update is the visible
commit point; a crash before it can leave an unreachable object, but does not
partially publish model and outbox state. The worker delivers the oldest
pending event before acknowledging its removal with another object/ref update:

```text
Update → Put(model + pending event object)
      → atomic ref update
      → wake worker
      → deliver(event ID)
      → Put(model - acknowledged event)
      → atomic ref update
```

Delivery is **at least once**. A crash after the external effect succeeds but
before the acknowledgement causes a retry. Use a stable event ID as the
destination's idempotency key; do not claim exactly-once delivery across an
uncoordinated external system.

## Example limits

`examples/durable` uses go-cask's content-addressed filesystem backend and
atomic ref writes. Its adapter serializes check/write/ref operations within one
process only; go-cask refs do not provide cross-process compare-and-swap.
Run one writer process per store root. Immutable objects can be orphaned if a
process stops before publishing its ref; use go-cask reachability/GC tooling
with an explicit retention policy. The sample does not provide replication,
multi-host coordination, or backup policy. Its delivery sink is a local
idempotency ledger, not an external service.

For multi-process deployments, select a database/store with transactional
compare-and-swap or a distributed lock and fencing tokens. Keep domain state
and outbox data in one commit unit. Use leased workers, bounded retries with
backoff, dead-letter handling, retention, metrics, and idempotency keys
supported by each external destination.
