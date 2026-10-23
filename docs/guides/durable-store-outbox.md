---
type: Guide
title: Durable storage and outbox
description: Persist model transitions with retryable, idempotent external effects.
version: 1
---

# Durable storage and outbox

The framework's default `store.Memory` is process-local. Replace it with an
application-owned `store.Store` when model state must survive restarts. See the
[durable example](../../examples/durable/README.md) for an example-only JSON
file store; it is not part of the framework.

## Transaction boundary

An `mvu.Command` runs after the model transition has committed. A process can
stop between those steps, so a command alone is not a durable job queue.
Persist pending effect records in the same commit unit as the model transition.
Treat wake-up signals as hints; scan pending records after startup and
periodically.

The example appends each event to `CounterModel.Outbox` in `Update`. The store
serializes model and pending event together into one JSON record and publishes
it by renaming a completed temporary file. The worker delivers the oldest
pending event before acknowledging its removal with another file replacement:

```text
Update → write temporary JSON file(model + pending event)
      → atomic file rename
      → wake worker
      → deliver(event ID)
      → write temporary JSON file(model - acknowledged event)
      → atomic file rename
```

Delivery is **at least once**. A crash after the external effect succeeds but
before the acknowledgement causes a retry. Use a stable event ID as the
destination's idempotency key; do not claim exactly-once delivery across an
uncoordinated external system.

## Example limits

`examples/durable` uses one JSON file per model key. Its example store
serializes check/write/rename operations within one process only. Run one
writer process per data directory. The sample does not provide replication,
multi-host coordination, encryption, or backup policy. Its delivery sink is a
local idempotency ledger, not an external service.

For multi-process deployments, select a database/store with transactional
compare-and-swap or a distributed lock and fencing tokens. Keep domain state
and outbox data in one commit unit. Use leased workers, bounded retries with
backoff, dead-letter handling, retention, metrics, and idempotency keys
supported by each external destination.
