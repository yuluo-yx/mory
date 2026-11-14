# Mory performance data

Mory **0.1.0**, Apple M4, 24 GiB, macOS-27.0.1-arm64-arm-64bit-Mach-O. Six fixed 1 MiB fixtures, Write and Source modes, three fresh processes per case and 30 edits per process. Values are medians across runs; edit values first take each run's median. RSS is sampled every 50 ms.

![Mory performance overview](performance-overview.svg)

The chart combines the structured/mixed-format editing measurements with the separate [full-table reading measurements](large-table-performance-report.md). The table cases in the editing matrix below used the earlier head/tail preview. Long-line Write requests use Source mode. Raw records identify each measured build and fixture.

## Editing measurements

| Fixture | Mode | Open (ms) | Edit (ms) | Save (ms) | Loaded RSS (MiB) | Peak RSS (MiB) |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| `long-line-1m` | Source | 65.97 | 0.030 | 38.42 | 196.48 | 235.50 |
| `long-line-1m` | Write | 63.96 | 0.030 | 38.00 | 195.61 | 246.16 |
| `memory-heavy-1m` | Source | 199.75 | 0.034 | 33.69 | 293.72 | 369.59 |
| `memory-heavy-1m` | Write | 201.58 | 0.043 | 61.99 | 320.12 | 427.72 |
| `structured-1m` | Source | 302.07 | 0.035 | 38.87 | 378.34 | 490.94 |
| `structured-1m` | Write | 309.28 | 0.044 | 88.56 | 422.44 | 505.02 |
| `table-heavy-1m` | Source | 76.85 | 0.030 | 32.98 | 219.45 | 302.69 |
| `table-heavy-1m` | Write | 81.69 | 0.034 | 37.97 | 210.77 | 237.77 |
| `table-long-cells-1m` | Source | 73.16 | 0.030 | 40.22 | 196.25 | 225.86 |
| `table-long-cells-1m` | Write | 78.33 | 0.033 | 36.23 | 195.89 | 223.16 |
| `target-shape-12x1m` | Source | 73.47 | 0.030 | 33.32 | 221.27 | 298.58 |
| `target-shape-12x1m` | Write | 75.34 | 0.034 | 32.04 | 213.30 | 243.25 |

## Edit tail latency and capture

P95 uses the nearest-rank percentile within each run, then the median across runs. Edits update the model; large documents serialize at autosave, save or source-consuming commands. Save timings include serialization. Capture includes rendering and PNG encoding, not display latency.

| Fixture | Mode | Edit P95 (ms) | Edit + rendering + PNG capture (ms) |
| --- | --- | ---: | ---: |
| `long-line-1m` | Source | 0.037 | 66.75 |
| `long-line-1m` | Write | 0.038 | 66.97 |
| `memory-heavy-1m` | Source | 0.042 | 55.21 |
| `memory-heavy-1m` | Write | 0.059 | 53.53 |
| `structured-1m` | Source | 0.053 | 52.81 |
| `structured-1m` | Write | 0.053 | 55.88 |
| `table-heavy-1m` | Source | 0.038 | 55.87 |
| `table-heavy-1m` | Write | 0.040 | 42.67 |
| `table-long-cells-1m` | Source | 0.051 | 53.81 |
| `table-long-cells-1m` | Write | 0.042 | 46.02 |
| `target-shape-12x1m` | Source | 0.039 | 56.46 |
| `target-shape-12x1m` | Write | 0.046 | 40.12 |

[Editing samples](performance-data.jsonl) · [Full-table reading samples](large-table-performance-data.jsonl)

Raw records retain build and fixture hashes and per-action samples. Runs are serial; other desktop activity is uncontrolled. See the [editor comparison](performance-comparison.md) for the scope of the ArcMD and Quiet.md reference measurements.

Reproduce editing measurements: `mise exec -- python3 scripts/performance.py --fixture-dir benchmark/fixtures --runs 3 --edits 30`. Regenerate the report and chart: `python3 scripts/performance_report.py`, with dependencies from `scripts/performance-requirements.txt` installed in a virtual environment.
