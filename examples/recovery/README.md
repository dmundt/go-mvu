# File-backed action recovery and outbox

Small application-owned persistence example. Standard library only, plus public
`mvu.Functions` for the pure counter transition. No framework API changes, HTTP,
UI, database, or dependency additions.

From this directory, in PowerShell:

```powershell
go run . -ledger .\demo.wal -scope alice -action order-1 -amount 3
go run . -ledger .\demo.wal -scope alice -action order-1 -amount 3
# Same result, duplicate=true; no second model transition or completed effect.
go run . -ledger .\demo.wal -scope alice -action order-1 -amount 4
# Fails: same scoped action ID, different payload.
go run . -ledger .\demo.wal
# Drain effects left pending by an earlier process.
go run . -ledger .\demo.wal -serve
# Context-aware retry service; Ctrl+C shuts down before closing the ledger.
```

Default stdout consumer is illustrative, **not a deduplicating durable consumer**.
Output may repeat after an interruption. Replace it with a consumer that commits
its business mutation and an inbox receipt keyed by `Effect.ID` in its own
durable transaction. A duplicate delivery must verify matching payload and return
success without repeating that mutation. Retain inbox receipts at least as long
as the producer can replay effects. Tests demonstrate a file receipt for this
protocol, not a general-purpose consumer database.

## Wiring and transaction boundary

`Ledger.Apply(ctx, scope, actionID, Add)` invokes the pure
`mvu.Functions[Model, Add].Update`, then writes **one** fixed-size WAL frame
containing the new model, scoped action ID, SHA-256 payload fingerprint, and
pending effect descriptor. The frame is synced before publishing any in-memory
model/dedup/outbox changes or returning success. IDs are caller-supplied and must
remain stable across request retries. Scopes select independent counter models.
The scope/action pair also defines the consumer's effect-ID namespace: separate
ledgers must use disjoint scopes unless they refer to the same logical actions.
Changing the ledger path does not create a new consumer namespace.
Duplicate actions return their original result, even if later actions advanced
the model; `Snapshot` returns the current model.

Persistence is outside `Update`, not a command or a hidden side effect. This
example deliberately does not use `web.Runtime`/ordinary `store.CompareAndSwap`:
separate CAS and command calls cannot provide this transaction. An HTTP adapter
would call `Apply` as its application service and only render after success.
Authentication must bind the scope; client-supplied IDs are not authorization.

`DeliverPending` calls the consumer outside the ledger mutex. Only consumer
success permits a separately synced completion frame. `Run` retries failures
with the same effect ID, reports every failed attempt, and stops on cancellation
or uncertain ledger I/O. Consumers must observe context and impose their own
I/O deadlines. Shutdown cannot forcibly interrupt a consumer ignoring context.
No external effect executes during WAL replay or inside `Update`.

This is **at-least-once**, not exactly-once: a crash after consumer success and
before durable completion causes redelivery. Concurrent delivery runners may
also duplicate deliveries. Use one runner unless the consumer handles concurrent
deduplication. Pending effects are replayed in acceptance order; effects in this
example represent counter-change notifications, not an exactly-once external
counter increment.

## WAL format, recovery, and limits

* Fixed 1,024-byte frames: 8-byte format/version magic, 4-byte big-endian JSON
  length, canonical JSON, zero padding, and SHA-256 of the preceding 992 bytes.
  Length corruption cannot change frame boundaries.
* Every complete frame must pass checksum, canonical encoding, and semantic
  replay checks. Unknown formats, duplicate action/completion records, invalid
  transitions, and checksum failures fail closed, including a corrupt last
  frame. Checksums detect accidental corruption, not malicious tampering.
* Only a final **incomplete** frame (1..1,023 bytes) is discarded. All preceding
  complete frames must validate first. Recovery truncates that tail and syncs
  before opening successfully. An acknowledged append wrote and synced a full
  frame, so this recovery does not discard an acknowledged model transition
  under the filesystem assumptions below. Never manually trim a full bad frame.
* A write, short write, or sync failure returns `ErrUncertain` **before memory
  commit** and poisons the open ledger. Close and reopen, then retry the **same**
  action ID/payload. A complete frame may survive an unsuccessful call; recovery
  adopts and syncs it, so the retry is deduplicated. An incomplete frame is
  discarded. A failed completion may likewise be pending or completed on reopen.
* Maximum 128 accepted actions across all scopes and 256 KiB WAL. Each action
  reserves room for its completion; pending effects can still complete when
  action capacity is full. Completed actions remain for permanent deduplication.
  No eviction, rotation, compaction, or automatic retention window. Capacity
  fails explicitly; it never silently drops a pending effect or receipt.
  Provisioning a new ledger is an explicit operational decision that loses the
  old ledger's deduplication namespace; never do it to retry an uncertain action.
* Inputs are bounded (scope/ID 1..64 UTF-8 bytes, increment 1..1,000,000).
  Recovery bounds the file before decoding. Storage is bounded per ledger;
  the caller must also bound how many separate ledger files it provisions.

## Filesystem and ownership caveats

One cooperating process owns each ledger path. `Open` exclusively creates
`<ledger>.lock` with `O_CREATE|O_EXCL` and holds ownership until `Close`; any
existing lock fails immediately. No stale-lock detection or automatic removal.
After a crash, an operator must first establish that the old owner is dead,
then remove the lock manually. A close failure may deliberately leave the lock.
Do not remove locks from a live process.

Parent symlinks are resolved; a symlink as the ledger itself is rejected.
Use one canonical path: hard-link aliases, alternate filesystem namespaces,
noncooperating writers, and hostile directory changes are unsupported.
The directory must exist and be trusted; locks are not a security boundary.

The supported design is append-only: **no rename or replacement**, avoiding
Windows rename-over-existing/open-file semantics. `File.Sync` is required for
every successful append and recovery. However, portable Go does not provide a
uniformly usable directory-fsync guarantee on Windows; this example does **not**
sync directory entries. Creation of the WAL/lock and removal of the lock are
therefore **not fully power-loss crash-safe**. Use a pre-provisioned, durably
established file/directory and filesystem-specific operational controls if that
guarantee matters. Even then, guarantees depend on the filesystem/device honoring
sync and preserving previously synced data and file length; network filesystems,
media corruption, loss of earlier frames, or hardware lying about sync are not
covered. Back up the ledger; corruption requires operator recovery from a
validated backup, not automatic replay of actions into a blank file.

## Validation

From the repository root:

```powershell
go test ./examples/recovery
go vet ./examples/recovery
go test -count=5 ./examples/recovery
```

Tests cover restart deduplication and conflicts, scoped models/effect IDs,
concurrent duplicate submissions, failed effects, consumer inbox deduplication,
short/full failed writes and sync failure before memory commit, every incomplete
tail length, fail-closed corruption, exclusive/stale locks, cancellation/retries,
and capacity including reserved completion space. Each test invocation creates a
unique `.test-ledger-*` directory inside this example and cleans only its own
directory. Concurrent test processes (including Windows and WSL) share no fixtures.

`TestProcessCrashRecovery` additionally kills owned subprocesses at ten real
file append/sync, consumer-commit, and completion boundaries. Parent waits for
process exit before removing that child's stale locks. Recovery verifies model
and receipt consistency, stable effect IDs, durable consumer deduplication, and
completion across another reopen. Consumer inbox in this fixture is a separate
ledger, atomically recording its deduplication key and business count.
Process termination leaves the OS/filesystem running: this does not simulate
power loss or establish directory-entry durability.
