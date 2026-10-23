---
type: Guide
title: Benchmarks
description: Benchmarking rules, measured results, and interpretation.
version: 1
---

# Benchmarks

Package [benchmarks](../../benchmarks/). Run:

```powershell
go test '-run=^$' '-bench=.' -benchmem ./benchmarks
```

(Quote args in PowerShell.)

## Results

i7-13800H, Windows, Go 1.23+. Indicative only; compare on same machine.

| Benchmark | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| Static | 34.7 | 96 | 1 |
| EscapedText | 170.7 | 96 | 1 |
| CounterFragment | 444.5 | 96 | 1 |
| Dashboard rows=10 | 2782 | 400 | 7 |
| Dashboard rows=100 | 18955 | 400 | 7 |
| Pagination pages=10 | 7755 | 6480 | 76 |
| Pagination pages=1000 | 9161 | 6728 | 94 |
| html/template counter | 644.9 | 242 | 8 |
| html/template escaped text | 791.9 | 432 | 9 |

Single run, observed output of the command above; not averaged.

## HTTP end-to-end

Run loopback HTTP benchmarks separately:

```powershell
go test '-run=^$' '-bench=HTTP' -benchmem -benchtime=2s ./benchmarks
```

These use `httptest.Server` over loopback TCP and an HTTP client. Timed work
includes request construction, middleware, routing, runtime, in-memory store,
view rendering, response transfer, and body draining. Cases cover page GET,
fragment POST with a model transition, and a 90/10 GET/POST mix. Setup is outside
the timer.

These are local end-to-end measurements, not WAN latency or cross-framework
comparisons. They exclude TLS, proxies, database I/O, browser parsing, and
concurrent load; use deployment-like load tests for those claims.

Go 1.27.0, Windows/amd64, 13th Gen Intel Core i7-13800H, single run,
`-benchtime=2s`:

| Benchmark | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| HTTP page GET | 135675 | 66849 | 134 |
| HTTP fragment POST | 82616 | 8200 | 116 |
| HTTP mixed requests (90/10 GET/POST) | 135123 | 61066 | 135 |

Loopback client/server on one machine; not a latency SLO or throughput claim.

## Concurrency, latency, and persistent-store paths

Additional benchmarks exercise parallel clients with unique session keys,
go-cask-backed CAS transitions, and an explicitly delayed client transport.
Run controlled cases with:

```powershell
go test '-run=^$' '-bench=HTTP' -benchmem -benchtime=5s -cpu=1,4,8 ./benchmarks
```

Benchmark names distinguish memory and go-cask stores. Parallel cases report
`-cpu`/GOMAXPROCS results; they are local contention measurements, not a
multi-host storage comparison. The CASK benchmark uses a temporary filesystem
root and is not a distributed storage comparison. The synthetic 20ms-RTT case
adds 10ms before and after each loopback request; it isolates request/response
latency effects but does not emulate packet loss, bandwidth limits, TLS, or a
real network.

Single run, Go 1.27.0, Windows/amd64, 13th Gen Intel Core i7-13800H,
`-benchtime=1s -cpu=1,4`:

| Benchmark | CPU | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: |
| HTTP parallel memory | 1 | 152218 | 60843 | 139 |
| HTTP parallel memory | 4 | 70536 | 61953 | 142 |
| HTTP parallel CASK | 1 | 7204112 | 65561 | 180 |
| HTTP parallel CASK | 4 | 1327087 | 67499 | 182 |
| HTTP loopback synthetic 20ms RTT | 1 | 21336904 | 66529 | 134 |

Illustrative measurements only. CASK timings include durable filesystem writes;
do not compare with memory-only numbers as a framework speed ranking. No
deployment latency or throughput claim without matching target hardware,
storage, traffic mix, and concurrency.

## Reading

- 1 alloc = `html.Context` per render; component output allocation-free.
- Dashboard allocs constant in row count (7 at 10 and 100 rows) → streaming, no per-row garbage.
- Counter ~1.45× faster, 8× fewer allocs than `html/template` equivalent.
- Pagination cost dominated by URL building (`url.Parse` + query encode per
  link); window size bounded, so pages=1000 ≈ pages=10.
- Benchmarks write to `io.Discard`; network and compression excluded.

## Performance claims (spec §21)

- Rendering API designed to minimize framework allocations.
- Specific components may reach zero allocations with a suitable writer.
- `io.Writer` implementations, HTTP internals, interfaces, model
  construction, URLs, dynamic data may allocate.
- Full HTTP request is **not** zero-allocation.
- Benchmark evidence determines actual behavior; no invented numbers.

## Rules

- New hot-path component → add benchmark.
- Regression > 20% ns/op or new allocs on hot path → justify in PR.
