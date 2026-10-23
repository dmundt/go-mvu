# Durable JSON counter and outbox example

Run with `go run .\examples\durable`. Server binds only to `localhost:8084`.
JSON model records, idempotent delivery records, and a stable session-signing
key are stored under `data/` by default; set `GOMVU_DATA_DIR` to change that
directory.

Each MVU update appends an outbox event to the model. The app stores a new
JSON file through the example-only `jsonstore` package. Each model and pending
event share one record, published using a temporary file and rename. A command
only wakes the worker; periodic scans recover missed wake-ups. The worker
delivers events at least once, then publishes an updated record without the
acknowledged event. If it crashes after delivery but before acknowledgement,
it retries the same event ID; the file sink deduplicates that ID.

The JSON store is a small example, not a framework adapter. It serializes
operations within one process, but does not coordinate writers across
processes, provide transactions across keys, or encrypt data. Use one process
per data directory. Add deployment-appropriate locking, backups, schema
migrations, worker leases, bounded retries, and monitoring before production
use.
