# Mory performance comparison

Mory **0.1.0** combines low input-processing cost with explicit handling of demanding Markdown documents. On the recorded Apple M4 / 24 GiB machine, the structured and mixed-format 1 MiB documents both have median Write edit actions below **0.05 ms**. Their edit-plus-rendering-and-PNG-capture costs fell by **59%** and **44%**, respectively, compared with Mory before this optimization.

## Measured gains

Each value is the median across three fresh processes. Edit and capture values first take the median of 30 edits within each process. Both builds used identical fixture hashes, effective modes and window dimensions.

| Write workload | Metric | Before | After | Change |
| --- | --- | ---: | ---: | --- |
| `structured-1m` | Edit action | 58.76 ms | 0.044 ms | Full serialization removed from ordinary typing |
| `structured-1m` | Edit + rendering + PNG capture | 135.33 ms | 55.88 ms | 59% lower |
| `structured-1m` | Save, including serialization | 104.07 ms | 88.56 ms | 15% lower |
| `memory-heavy-1m` | Edit action | 28.13 ms | 0.043 ms | Full serialization removed from ordinary typing |
| `memory-heavy-1m` | Edit + rendering + PNG capture | 95.04 ms | 53.53 ms | 44% lower |
| `memory-heavy-1m` | Save, including serialization | 61.78 ms | 61.99 ms | Approximately unchanged |

Mory's demonstrated strengths are:

- **Less work on each keystroke.** Indexed block lookup avoids searching unrelated subtrees. Large documents serialize at autosave, save and source-consuming commands; the measured save includes this work.
- **Lower editing-and-capture cost.** The improvement remains visible when rendering and PNG encoding are included, beyond the reduction in the input handler alone. Capture is not keyboard-to-display latency.
- **Explicit handling of stress documents.** The earlier `table-heavy-1m` and `target-shape-12x1m` Write requests completed with median sampled peak RSS of **237.77 MiB** and **243.25 MiB** using head/tail-only previews. The current reader exposes every row through a virtualized viewport; see the separate [large-table reading results](large-table-performance-report.md). The earlier numbers are not measurements of that new workflow. Exceptionally long single lines retain their Source-mode fallback.

The complete matrix contains six fixtures, two requested modes, three runs per case and 1,080 edits, with no recorded failures. Memory did not improve in every case: structured Write loaded RSS increased from **411.67 to 422.44 MiB**, while mixed-format Write peak RSS increased from **419.08 to 427.72 MiB**. The main measured gain is reduced editing cost.

## Mory, ArcMD and Quiet.md

Versions are retained because later releases may change performance. The local records cover different workflows:

| Editor | Recorded version | Local measurement coverage | Comparable edit / render / save timings |
| --- | --- | --- | --- |
| Mory | 0.1.0 | Native-window open, 30 edits, save, undo and resource sampling; Source scrolling | Available for Mory before and after optimization |
| ArcMD | v0.0.11 | Release-process RSS and CPU observation, approximately 3.7 seconds per run; one extended observation | Not measured with Mory's workflow |
| Quiet.md | v0.1.11 | Release-process RSS and CPU observation, approximately 3.7 seconds per run; extended stress observations | Not measured with Mory's workflow |

The evidence supports Mory's low measured input cost and substantial improvement on the two complex-document workloads. It does **not** establish that Mory is the fastest or most memory-efficient of all three editors: observation windows, operations and rendering paths differ. Published upstream editor benchmarks also use different timing boundaries and cannot be substituted for matching local measurements.

Sources: [Mory results](performance-report.md), [before samples](performance-before.jsonl), [after samples](performance-data.jsonl), [ArcMD release-process results](arcmd-performance-report.md), [Quiet.md release-process results](quiet-md-performance-report.md). Raw records retain build identities and fixture hashes. The chart and detailed report include all cases, including regressions and Source fallbacks.
