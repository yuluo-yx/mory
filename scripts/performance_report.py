#!/usr/bin/env python3
"""Render measured editor and full-table reading performance.

Requires matplotlib. Raw JSONL files remain the source of all plotted values.
"""
import argparse
import json
import math
from pathlib import Path
import statistics

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt


def load(path):
    records = [json.loads(line) for line in path.read_text().splitlines()]
    cases = {}
    for record in records[1:]:
        if record["failures"] or len(record["runs"]) != 3:
            raise ValueError(f"Incomplete benchmark: {record['kind']} {record['mode']}")
        for run in record["runs"]:
            actions = [run["open"], run["save"], run["undo"], *run["edits"], *run["scroll"]]
            if len(run["edits"]) != 30 or any(action["error"] for action in actions):
                raise ValueError(f"Incomplete actions: {record['kind']} {record['mode']}")
        cases[(record["kind"], record["mode"])] = record
    return records[0]["environment"], cases


def metrics(case):
    runs = case["runs"]
    median = statistics.median
    def p95(values):
        values = sorted(values)
        return values[math.ceil(.95 * len(values)) - 1]
    return {
        "open": median(r["open"]["action_ms"] for r in runs),
        "edit": median(median(e["action_ms"] for e in r["edits"]) for r in runs),
        "edit_p95": median(p95(e["action_ms"] for e in r["edits"]) for r in runs),
        "capture": median(median(e["captured_ms"] for e in r["edits"]) for r in runs),
        "save": median(r["save"]["action_ms"] for r in runs),
        "loaded": median(r["loaded_rss_mib"] for r in runs),
        "peak": median(r["sampled_peak_rss_mib"] for r in runs),
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--data", type=Path, default=Path("benchmark/performance-data.jsonl"))
    parser.add_argument("--table-data", type=Path, default=Path("benchmark/large-table-performance-data.jsonl"))
    parser.add_argument("--preview", type=Path, help="optional PNG preview for visual review")
    args = parser.parse_args()
    env, cases = load(args.data)
    values = {k: metrics(v) for k, v in cases.items()}
    records = [json.loads(line) for line in args.table_data.read_text().splitlines()]
    for field in ("cpu", "os", "ram_bytes"):
        if env[field] != records[0]["environment"][field]:
            raise ValueError(f"Environment differs: {field}")
    tables = {}
    for case in records[1:]:
        if case["mode"] != "Read":
            continue
        if case["failures"] or len(case["runs"]) != 3 or any(len(r["jumps"]) != 9 for r in case["runs"]):
            raise ValueError("Incomplete table measurements")
        tables[case["kind"]] = {
            "capture": statistics.median(statistics.median(j["captured_ms"] for j in r["jumps"]) for r in case["runs"]),
            "loaded": statistics.median(r["loaded_rss_mib"] for r in case["runs"]),
        }
    editing = [("structured-1m", "Write"), ("memory-heavy-1m", "Write")]
    table_keys = ["table-heavy-1m", "table-long-cells-1m", "target-shape-12x1m"]
    panels = [
        ("Edit handler", "ms", ["Structured document", "Mixed-format document"], [values[k]["edit"] for k in editing], "%.3f"),
        ("Save, including serialization", "ms", ["Structured document", "Mixed-format document"], [values[k]["save"] for k in editing], "%.2f"),
        ("Full-table navigation + PNG capture", "ms", ["Dense table", "Long cells", "Wide table"], [tables[k]["capture"] for k in table_keys], "%.2f"),
        ("Loaded table resident memory", "MiB", ["Dense table", "Long cells", "Wide table"], [tables[k]["loaded"] for k in table_keys], "%.2f"),
    ]
    plt.rcParams.update({"font.family": "sans-serif", "font.size": 11,
                         "svg.fonttype": "none", "svg.hashsalt": "mory-performance"})
    figure, axes = plt.subplots(2, 2, figsize=(12, 6.3), layout="constrained")
    for ax, (title, unit, labels, numbers, fmt) in zip(axes.flat, panels):
        bars = ax.barh(labels, numbers, height=.45, color="#287b9b")
        ax.bar_label(bars, fmt=fmt, padding=5, fontsize=11)
        ax.invert_yaxis()
        ax.set_title(title, loc="left", fontweight="bold", pad=12)
        ax.set_xlabel(unit)
        ax.margins(x=.23, y=.25)
        ax.grid(axis="x", alpha=.15)
        ax.set_axisbelow(True)
        ax.spines[["top", "right", "left"]].set_visible(False)
        ax.tick_params(axis="y", length=0)
    version = env["product_version"]
    figure.suptitle(f"Mory {version} · measured performance\n{env['cpu']} · {env['ram_bytes'] / 2**30:g} GiB · 1 MiB documents · medians of 3 runs", fontsize=16)
    figure.supxlabel("Edit excludes rendering. Table capture includes PNG encoding. Neither is keyboard-to-display latency.", fontsize=9)
    out = args.data.parent
    chart = out / "performance-overview.svg"
    figure.savefig(chart, metadata={"Date": None})
    chart.write_text("\n".join(line.rstrip() for line in chart.read_text().splitlines()) + "\n")
    if args.preview:
        figure.savefig(args.preview, dpi=120)
    plt.close(figure)
    lines = ["# Mory performance data", "",
             f"Mory **{version}**, {env['cpu']}, {env['ram_bytes'] / 2**30:g} GiB, {env['os']}. "
             "Six fixed 1 MiB fixtures, Write and Source modes, three fresh processes per case and 30 edits per process. "
             "Values are medians across runs; edit values first take each run's median. RSS is sampled every 50 ms.", "",
             "![Mory performance overview](performance-overview.svg)", "",
             "The chart combines the structured/mixed-format editing measurements with the separate "
             "[full-table reading measurements](large-table-performance-report.md). The table cases in the editing "
             "matrix below used the earlier head/tail preview. Long-line Write requests use Source mode. "
             "Raw records identify each measured build and fixture.", "",
             "## Editing measurements", "",
             "| Fixture | Mode | Open (ms) | Edit (ms) | Save (ms) | Loaded RSS (MiB) | Peak RSS (MiB) |",
             "| --- | --- | ---: | ---: | ---: | ---: | ---: |"]
    for k in sorted(values):
        cells = [f"{values[k][m]:.3f}" if m == "edit" else f"{values[k][m]:.2f}" for m in ("open", "edit", "save", "loaded", "peak")]
        lines.append("| " + " | ".join([f"`{k[0]}`", k[1]] + cells) + " |")
    lines += ["", "## Edit tail latency and capture", "",
              "P95 uses the nearest-rank percentile within each run, then the median across runs. "
              "Edits update the model; large documents serialize at autosave, save or source-consuming commands. "
              "Save timings include serialization. Capture includes rendering and PNG encoding, not display latency.", "",
              "| Fixture | Mode | Edit P95 (ms) | Edit + rendering + PNG capture (ms) |",
              "| --- | --- | ---: | ---: |"]
    for k in sorted(values):
        lines.append(f"| `{k[0]}` | {k[1]} | {values[k]['edit_p95']:.3f} | {values[k]['capture']:.2f} |")
    lines += ["", "[Editing samples](performance-data.jsonl) · [Full-table reading samples](large-table-performance-data.jsonl)", "",
              "Raw records retain build and fixture hashes and per-action samples. Runs are serial; other desktop "
              "activity is uncontrolled. See the [editor comparison](performance-comparison.md) for the scope of "
              "the ArcMD and Quiet.md reference measurements.", "",
              "Reproduce editing measurements: `mise exec -- python3 scripts/performance.py --fixture-dir benchmark/fixtures --runs 3 --edits 30`. "
              "Regenerate the report and chart: `python3 scripts/performance_report.py`, with dependencies from "
              "`scripts/performance-requirements.txt` installed in a virtual environment.", ""]
    (out / "performance-report.md").write_text("\n".join(lines))


if __name__ == "__main__":
    main()
