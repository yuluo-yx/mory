# Mory performance fixtures

This directory contains deterministic Markdown stress fixtures for native performance checks. The files are intentionally hostile to layout and allocation behavior: dense tables, long table cells, repeated structured blocks, mixed inline formatting, code blocks, math, tasks, nested lists and an unbroken long line.

Generate or refresh the fixtures with:

```sh
python3 benchmark/generate.py
```

The default output is approximately 1 MiB per fixture under `benchmark/fixtures/`. Use `--size` to generate a different byte budget.

The native harness can exercise equivalent generated workloads with:

```sh
python3 scripts/performance.py --runs 1 --edits 1 --sizes 1048576 --kinds table-heavy,memory-heavy --modes Write
```

For a real fixture file, open it in Mory in Write mode and record RSS, physical footprint, open latency, first-frame latency, edit latency, save latency and scroll behavior. Keep Source mode as a comparison point for the same file. Do not use a single successful open as a smooth-editing guarantee; table density, block count and long unbroken lines stress different subsystems.

The fixture names are stable so results can be compared between revisions:

- `table-heavy-1m.md`: 16 columns with repeated inline formatting.
- `table-long-cells-1m.md`: 12 columns with long multilingual cell text.
- `target-shape-12x1m.md`: 12-column table shaped like the reported production document.
- `memory-heavy-1m.md`: headings, inline styles, links, math, quotes, tasks, nested lists and code blocks.
- `structured-1m.md`: repeated headings, Docusaurus callouts and tables.
- `long-line-1m.md`: one unbroken paragraph.
