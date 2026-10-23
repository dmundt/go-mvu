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
