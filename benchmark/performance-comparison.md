# Mory performance strengths

Mory **0.1.0** provides low input-processing cost and complete large-table reading through a virtualized viewport. The local measurements use an Apple M4 with 24 GiB RAM and fixed Markdown fixtures of approximately 1 MiB.

| Measured workload | Result | What it demonstrates |
| --- | ---: | --- |
| Structured-document Write edit action | **0.044 ms** | Low cost in the input handler |
| Mixed-format Write edit action | **0.043 ms** | Low input cost with mixed Markdown content |
| Structured / mixed-format save | **88.56 / 61.99 ms** | Includes serialization deferred from typing |
| Full-table reading, 401–3,206 rows | **35.07–37.35 ms** | Navigation, rendering and PNG capture across arbitrary rows |
| Loaded table RSS in Read mode | **201.22–238.08 MiB** | Resident memory while all table rows remain accessible |

Values are medians across three independent processes. Edit and navigation figures first take each run's median. The editing matrix contains 36 runs and 1,080 edits; full-table reading contains 18 runs and 162 successful target-row checks. Document bytes remained unchanged during table navigation.

Mory's strengths are **lightweight typing**, **complete access to large tables**, and **bounded row rendering**. Indexed block lookup avoids searching unrelated subtrees during edits. Large documents serialize at autosave, save and source-consuming commands. The table reader renders nearby rows with a pinned header and wrapped cells, while retaining access to the entire table. Table editing still uses Source mode.

## Reference editors

| Editor | Recorded version | Available local evidence |
| --- | --- | --- |
| Mory | 0.1.0 | Native-window editing, saving, full-table navigation and RSS measurements |
| ArcMD | v0.0.11 | Release-process RSS and CPU samples; no matching local edit/save sequence |
| Quiet.md | v0.1.11 | Release-process RSS and CPU samples; no matching local edit/save sequence |

These reference measurements use different observation windows and rendering paths. They do not support a cross-editor speed multiplier or an overall fastest-editor claim. Mory's figures above describe its demonstrated capabilities; the other editors' unmeasured operations are not treated as failures.

Input actions exclude rendering. Capture includes PNG encoding and is not keyboard-to-display latency or continuous-scroll FPS. RSS covers the sampled process and is not a GPU-memory measurement. The editing records and full-table reading records identify separate measured builds; versions, fixture hashes and binary identities remain in the raw data.

Sources: [editing results](performance-report.md), [editing samples](performance-data.jsonl), [full-table reading results](large-table-performance-report.md), [table-reading samples](large-table-performance-data.jsonl), [ArcMD release-process results](arcmd-performance-report.md), [Quiet.md release-process results](quiet-md-performance-report.md).
