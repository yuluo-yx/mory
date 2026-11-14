#!/usr/bin/env python3
"""Summarize the native full-table navigation benchmark without timestamps."""
import json
from pathlib import Path
from statistics import median


def main():
    root = Path(__file__).resolve().parent.parent / "benchmark"
    records = [json.loads(line) for line in (root / "large-table-performance-data.jsonl").read_text().splitlines()]
    env, cases = records[0]["environment"], records[1:]
    if len(cases) != 6:
        raise ValueError("Expected three fixtures in Write and Read modes")
    lines = ["# Large-table reading performance", "",
             f"Mory **{env['product_version']}**, {env['cpu']}, {env['ram_bytes'] / 2**30:g} GiB, {env['os']}. "
             "Three fixed approximately 1 MiB tables, Write and Read modes, three independent processes per case. "
             "Each process opens the document and makes nine jumps across its middle, final and first data rows. "
             "Every jump verifies that its target row is in the rendered viewport. Document bytes must remain unchanged.", "",
             "Large tables now expose every row through a virtualized viewport, with a pinned header, horizontal "
             "scrolling and full wrapped cell contents. The source cell index retains Markdown slices; only nearby "
             "rows are parsed and laid out for display. Editing these large tables still uses Source mode.", "",
             "| Fixture | Mode | Rows | Open action (ms) | Jump + rendering + PNG (ms) | Loaded RSS (MiB) | Peak RSS (MiB) | Max visible body rows |",
             "| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |"]
    for case in cases:
        runs = case["runs"]
        if case["failures"] or len(runs) != 3:
            raise ValueError(f"Incomplete runs: {case['kind']} {case['mode']}")
        jumps = [j for r in runs for j in r["jumps"]]
        if any(len(r["jumps"]) != 9 for r in runs) or any(
            j["error"] or not j["table_first_visible_row"] <= j["table_target_row"] <= j["table_last_visible_row"]
            for j in jumps
        ):
            raise ValueError("A table navigation did not reach its target")
        metrics = [median(r["open"]["action_ms"] for r in runs),
                   median(median(j["captured_ms"] for j in r["jumps"]) for r in runs),
                   median(r["loaded_rss_mib"] for r in runs),
                   median(r["sampled_peak_rss_mib"] for r in runs)]
        visible = max(j["table_last_visible_row"] - max(1, j["table_first_visible_row"]) + 1 for j in jumps)
        lines.append("| " + " | ".join([f"`{case['kind']}`", case["mode"], str(jumps[0]["table_rows"]),
                                       *(f"{v:.2f}" for v in metrics), str(visible)]) + " |")
    lines += ["", "Values are medians across runs; jump values first take each run's median. "
              "RSS is sampled every 50 ms. The visible-row count excludes the header and the list's "
              "overscan; it is not a count of every cached UI node. Capture includes PNG encoding and is not display "
              "latency or continuous-scroll FPS. Other desktop activity is uncontrolled.", "",
              "The earlier [editing benchmark](performance-report.md) used a head/tail-only table preview. "
              "Its table results describe that earlier rendering behavior and are not a like-for-like baseline "
              "for full-table navigation. This report measures the new reading workflow; it does not claim a "
              "speedup over the partial preview.", "",
              "[Raw measurements and build identities](large-table-performance-data.jsonl)", "",
              "Reproduce: `mise exec -- python3 scripts/table_performance.py > benchmark/large-table-performance-data.jsonl`. "
              "Regenerate: `python3 scripts/table_performance_report.py`.", ""]
    (root / "large-table-performance-report.md").write_text("\n".join(lines))


if __name__ == "__main__":
    main()
