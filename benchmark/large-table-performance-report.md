# Large-table reading performance

Mory **0.1.0**, Apple M4, 24 GiB, macOS-27.0.1-arm64-arm-64bit-Mach-O. Three fixed approximately 1 MiB tables, Write and Read modes, three independent processes per case. Each process opens the document and makes nine jumps across its middle, final and first data rows. Every jump verifies that its target row is in the rendered viewport. Document bytes must remain unchanged.

Large tables now expose every row through a virtualized viewport, with a pinned header, horizontal scrolling and full wrapped cell contents. The source cell index retains Markdown slices; only nearby rows are parsed and laid out for display. Editing these large tables still uses Source mode.

| Fixture | Mode | Rows | Open action (ms) | Jump + rendering + PNG (ms) | Loaded RSS (MiB) | Peak RSS (MiB) | Max visible body rows |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `table-heavy-1m` | Write | 2403 | 51.05 | 35.52 | 237.08 | 289.27 | 10 |
| `table-heavy-1m` | Read | 2403 | 51.96 | 35.44 | 238.08 | 269.58 | 10 |
| `table-long-cells-1m` | Write | 401 | 47.87 | 37.22 | 203.80 | 276.94 | 4 |
| `table-long-cells-1m` | Read | 401 | 48.12 | 37.35 | 201.22 | 279.75 | 4 |
| `target-shape-12x1m` | Write | 3206 | 49.91 | 35.78 | 229.02 | 264.94 | 10 |
| `target-shape-12x1m` | Read | 3206 | 55.31 | 35.07 | 236.16 | 275.08 | 10 |

Values are medians across runs; jump values first take each run's median. RSS is sampled every 50 ms. The visible-row count excludes the header and the list's overscan; it is not a count of every cached UI node. Capture includes PNG encoding and is not display latency or continuous-scroll FPS. Other desktop activity is uncontrolled.

The earlier [editing benchmark](performance-report.md) used a head/tail-only table preview. Its table results describe that earlier rendering behavior and are not a like-for-like baseline for full-table navigation. This report measures the new reading workflow; it does not claim a speedup over the partial preview.

[Raw measurements and build identities](large-table-performance-data.jsonl)

Reproduce: `mise exec -- python3 scripts/table_performance.py > benchmark/large-table-performance-data.jsonl`. Regenerate: `python3 scripts/table_performance_report.py`.
