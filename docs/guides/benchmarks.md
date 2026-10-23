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
JSON-file-backed CAS transitions, and an explicitly delayed client transport.
Run controlled cases with:

```powershell
go test '-run=^$' '-bench=HTTP' -benchmem -benchtime=5s -cpu=1,4,8 ./benchmarks
```

Benchmark names distinguish memory and JSON file stores. Parallel cases report
`-cpu`/GOMAXPROCS results; they are local contention measurements, not a
multi-host storage comparison. The JSON file benchmark uses a temporary
filesystem root and is not a distributed storage comparison. The synthetic
20ms-RTT case adds 10ms before and after each loopback request; it isolates
request/response latency effects but does not emulate packet loss, bandwidth
limits, TLS, or a real network.

Single run, Go 1.27.0, Windows/amd64, 13th Gen Intel Core i7-13800H,
`-benchtime=1s -cpu=1,4`:

| Benchmark | CPU | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: |
| HTTP parallel memory | 1 | 152218 | 60843 | 139 |
| HTTP parallel memory | 4 | 70536 | 61953 | 142 |
| HTTP parallel JSON file | 1 | 2054542 | 62694 | 153 |
| HTTP parallel JSON file | 4 | 1082056 | 63166 | 155 |
| HTTP loopback synthetic 20ms RTT | 1 | 21336904 | 66529 | 134 |

Illustrative measurements only. JSON file timings include durable filesystem
writes; do not compare with memory-only numbers as a framework speed ranking.
No deployment latency or throughput claim without matching target hardware,
storage, traffic mix, and concurrency.

## Reading

- 1 alloc = `html.Context` per render; component output allocation-free.
- Dashboard allocs constant in row count (7 at 10 and 100 rows) → streaming, no per-row garbage.
- Counter ~1.45× faster, 8× fewer allocs than `html/template` equivalent.
- Pagination cost dominated by URL building (`url.Parse` + query encode per
  link); window size bounded, so pages=1000 ≈ pages=10.
- Benchmarks write to `io.Discard`; network and compression excluded.

## Saturated idempotency

Windows amd64, i7-13800H, Go 1.27.0, four logical CPUs; one before/after run:

```powershell
go test ./middleware '-run=^$' '-bench=BenchmarkIdempotencySaturation' -benchmem -benchtime=300ms -count=1 -cpu=4
```

| Full table | Operation | Before ns/op | After ns/op |
| --- | --- | ---: | ---: |
| 128 | Replay | 4257 | 3507 |
| 128 | Reject reservation | 3974 | 4217 |
| 4096 | Replay | 55583 | 5400 |
| 4096 | Reject reservation | 59875 | 5541 |
| 16384 | Replay | 232810 | 4166 |
| 16384 | Reject reservation | 251242 | 3845 |

Both runs use parallel callers, hour-long receipt TTL, untimed table prefill,
and the same benchmark-specific error responder without expected-503 logging.
Replay stays at 43 allocs/op; rejection at 45–46 allocs/op. These are
microbenchmarks with in-memory HTTP recorders, not end-to-end latency percentiles.
Skipping cleanup until the earliest completed receipt expires removes
per-request table scans. Cleanup at expiry remains O(entries) under the mutex.
The small-table rejection difference is not evidence of an improvement there;
single-run timing varies.

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
