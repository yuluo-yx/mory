# Mory performance data

Mory **0.1.0**, Apple M4, 24 GiB, macOS-27.0.1-arm64-arm-64bit-Mach-O; requested window 2560x1312. Six checked-in 1 MiB fixtures, Write and Source modes, three fresh processes per case, 30 edits per process. Values below are medians across runs; edit values use each run's median. RSS is sampled every 50 ms. Long-line Write requests use the existing Source fallback; large tables retain the existing compact preview. These results do not measure full rich rendering of every table cell.

The baseline build is identified by commit `627ba04460de` and the worktree diff and binary hashes in its raw records. The optimized build adds indexed block lookup, compact typing undo records and deferred serialization for large block documents. Edit actions update the editor model; full source serialization now occurs at autosave, save and source-consuming commands. The edit speedup therefore includes work moved out of the input handler. Save timings include pending serialization; capture timings include rendering and PNG encoding. Neither is keyboard-to-display latency.

![Mory performance overview](performance-overview.svg)

## Before / after

| Fixture | Mode | Open (ms) | Edit (ms) | Save (ms) | Loaded RSS (MiB) | Peak RSS (MiB) |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| `long-line-1m` | Source | 68.26 / 65.97 | 0.03 / 0.03 | 33.28 / 38.42 | 197.12 / 196.48 | 247.70 / 235.50 |
| `long-line-1m` | Write | 53.78 / 63.96 | 0.03 / 0.03 | 36.10 / 38.00 | 193.62 / 195.61 | 240.45 / 246.16 |
| `memory-heavy-1m` | Source | 213.39 / 199.75 | 0.03 / 0.03 | 40.08 / 33.69 | 297.38 / 293.72 | 371.89 / 369.59 |
| `memory-heavy-1m` | Write | 207.82 / 201.58 | 28.13 / 0.04 | 61.78 / 61.99 | 319.16 / 320.12 | 419.08 / 427.72 |
| `structured-1m` | Source | 303.77 / 302.07 | 0.03 / 0.03 | 33.00 / 38.87 | 384.14 / 378.34 | 495.17 / 490.94 |
| `structured-1m` | Write | 298.26 / 309.28 | 58.76 / 0.04 | 104.07 / 88.56 | 411.67 / 422.44 | 518.09 / 505.02 |
| `table-heavy-1m` | Source | 88.85 / 76.85 | 0.03 / 0.03 | 31.60 / 32.98 | 237.70 / 219.45 | 301.97 / 302.69 |
| `table-heavy-1m` | Write | 83.57 / 81.69 | 0.13 / 0.03 | 40.37 / 37.97 | 211.70 / 210.77 | 244.72 / 237.77 |
| `table-long-cells-1m` | Source | 79.92 / 73.16 | 0.03 / 0.03 | 40.48 / 40.22 | 197.97 / 196.25 | 233.86 / 225.86 |
| `table-long-cells-1m` | Write | 75.75 / 78.33 | 0.11 / 0.03 | 36.28 / 36.23 | 197.59 / 195.89 | 237.02 / 223.16 |
| `target-shape-12x1m` | Source | 69.79 / 73.47 | 0.03 / 0.03 | 36.23 / 33.32 | 214.58 / 221.27 | 292.61 / 298.58 |
| `target-shape-12x1m` | Write | 79.29 / 75.34 | 0.10 / 0.03 | 33.44 / 32.04 | 216.23 / 213.30 | 242.73 / 243.25 |

## Edit tail latency and capture

P95 uses the nearest-rank percentile within each run, then the median of the three runs. Capture includes the action, window rendering and PNG encoding; it is not display latency.

| Fixture | Mode | Edit P95 before / after (ms) | Capture median before / after (ms) |
| --- | --- | ---: | ---: |
| `long-line-1m` | Source | 0.05 / 0.04 | 66.92 / 66.75 |
| `long-line-1m` | Write | 0.04 / 0.04 | 67.74 / 66.97 |
| `memory-heavy-1m` | Source | 0.04 / 0.04 | 55.26 / 55.21 |
| `memory-heavy-1m` | Write | 30.64 / 0.06 | 95.04 / 53.53 |
| `structured-1m` | Source | 0.04 / 0.05 | 52.91 / 52.81 |
| `structured-1m` | Write | 60.77 / 0.05 | 135.33 / 55.88 |
| `table-heavy-1m` | Source | 0.06 / 0.04 | 56.24 / 55.87 |
| `table-heavy-1m` | Write | 0.26 / 0.04 | 43.30 / 42.67 |
| `table-long-cells-1m` | Source | 0.04 / 0.05 | 54.18 / 53.81 |
| `table-long-cells-1m` | Write | 0.23 / 0.04 | 46.93 / 46.02 |
| `target-shape-12x1m` | Source | 0.04 / 0.04 | 56.75 / 56.46 |
| `target-shape-12x1m` | Write | 0.22 / 0.05 | 40.41 / 40.12 |

[Before measurements](performance-before.jsonl) · [After measurements](performance-data.jsonl)

The raw records retain fixture and binary SHA-256 identities, commit/worktree state and per-action samples. Runs are serial; other desktop activity is uncontrolled. This comparison measures Mory before and after optimization. The separate ArcMD and Quiet.md process probes do not establish an editor latency ranking.

Reproduce: `mise exec -- python3 scripts/performance.py --fixture-dir benchmark/fixtures --runs 3 --edits 30`. Regenerate this report and chart with `python3 scripts/performance_report.py` after installing `scripts/performance-requirements.txt` in a virtual environment.
