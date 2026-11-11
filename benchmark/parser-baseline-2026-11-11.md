# Markdown parser allocation check

These measurements were collected on November 11, 2026 after the compact-table parser landed. They use `go test ./benchmark -run '^$' -bench Parse -benchmem -benchtime=1x` on an Apple M1 Pro with Go 1.27.3. Each fixture is parsed once per benchmark iteration; the numbers describe parser allocations only, not a full native window RSS measurement.

| Fixture | Parse time (ms) | Allocated (MiB/op) | Allocations/op |
| --- | ---: | ---: | ---: |
| `table-heavy-1m.md` | 12.3 | 55.9 | 14,513 |
| `table-long-cells-1m.md` | 3.4 | 5.8 | 2,475 |
| `target-shape-12x1m.md` | 10.7 | 56.0 | 19,327 |
| `memory-heavy-1m.md` | 220.8 | 190.3 | 3,403,293 |
| `structured-1m.md` | 418.1 | 465.8 | 5,481,003 |
| `long-line-1m.md` | 13.8 | 15.1 | 96 |

The dense ordinary Markdown table cases use the compact representation once they reach 200 rows. Their source spelling remains attached to the table node, so opening and saving an untouched document remains byte-for-byte stable. The structured and mixed-content cases still exercise the full rich block parser and remain the main allocation targets for future work.
