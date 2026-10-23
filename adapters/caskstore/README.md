# CASK model store

Optional module implementing `store.Store[string, M]` using
[go-cask](https://github.com/dmundt/go-cask) v1.7.0.
Framework packages remain stdlib-only; the root module also uses CASK for its
durable outbox example. This adapter requires Go 1.24 or newer; run its tests
separately from the repository root module.

```go
models, err := caskstore.Open[Model]("data/models")
if err != nil {
	return err
}
defer func() {
	if err := models.Close(); err != nil {
		slog.Error("close model store", "error", err)
	}
}()
// RuntimeConfig[string, Model, Message]{Store: models, ...}
```

Import `github.com/dmundt/go-mvu/adapters/caskstore`. Within this checkout,
the adapter's module replaces the core with `../..`; the
[production example](../../examples/production/README.md) demonstrates consuming
both local modules. Separate applications need their own module requirements
and, until nested modules are published, local replacements for both modules.
Replacements in dependency modules are not inherited.

## Guarantees

- Restart persistence: immutable JSON records addressed by SHA-256; CASK
  filesystem backend syncs objects before durable reference publication.
- Integrity: loaded record bytes must match their referenced digest.
- One exclusive OS lock per directory, held until `Close`. A second owner,
  including another process, receives `ErrLocked`; process termination releases
  the OS lock. The lock file's presence alone does not mean it is locked.
- Concurrent requests through one instance serialize Load/Create/CAS/Delete.
  CASK references offer atomic replacement, not CAS; directory ownership and the
  adapter mutex provide the version check boundary.
- `Create` starts at version 1; CAS version 0 creates missing models; stale
  versions return `store.ErrConflict`; deleting a missing key succeeds.
- JSON encode/decode separates stored, returned, and caller-owned maps/pointers.
  Models must support a lossless JSON round trip; use exported, explicitly
  typed fields. Interfaces, custom codecs, and arbitrary Go values need care.
- Records carry schema version 1; unknown schemas fail explicitly.
  Application model evolution needs application-owned migration tooling and
  backups. There is no automatic migration.
- Storage, decoding, integrity, cancellation, and publication errors propagate.
  A publication error can occur after the head changed (for example, reflog
  failure); `ErrCommitUncertain` identifies this case. Reload/reconcile explicitly. Do not blindly
  retry an action or treat failure as proof that nothing persisted.

## Deployment boundary

**Single host, single owning process, local persistent filesystem.** No shared
network filesystem, replication, distributed CAS, or horizontal scaling.
Durability also depends on OS/filesystem/hardware sync guarantees; Windows
cannot fsync directories. This is not a transactional database or ORM.

The root directory is created with private permissions where supported.
Restrict Windows ACLs explicitly. No encryption at rest is provided.
Keys become hashed reference names, but model contents are not encrypted.

`Delete` writes a tombstone. Old objects and CASK reflogs retain history,
including prior model data. No automatic garbage collection or retention
policy runs. Do not put access/refresh tokens or unnecessary personal data
in models. Plan retention, protected backups, restore drills, disk monitoring,
and eventual physical cleanup separately.

Stop traffic, drain handlers, and close the store before copying its whole
directory for a consistent backup. Do not run CASK GC against an active adapter
without a coordinated roots/retention protocol. Keep effects in model-embedded
outbox records when atomic persistence is needed; this adapter does not supply
multi-model transactions or an outbox worker.

```powershell
Set-Location adapters\caskstore
go test ./...
go vet ./...
```

Tests cover restart recovery, isolation, CAS contention, process ownership,
logical deletion, corrupt metadata, dangling objects, schema rejection,
cancellation, closed-store use, and serialization failure.
