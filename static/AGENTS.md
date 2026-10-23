# static rules

- `htmx.min.js` vendored upstream build (2.0.4). Never edit.
- Version bump: replace file, recompute `HTMXIntegrity` (`sha384`, base64),
  update version in docs, run `go test ./static ./examples/...`.
- Embedded via `embed`; no CDN, no runtime fetch.
- `Handler()` strips `Prefix` itself; mount at `static.Prefix` without
  `http.StripPrefix`. Directory listing → 404.
- Responses `Cache-Control: immutable`, 1 year. Changing asset content →
  change file name or accept stale clients.
