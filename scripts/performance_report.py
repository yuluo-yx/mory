#!/usr/bin/env python3
"""Render the checked-in native benchmark and its before/after comparison.

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
    parser.add_argument("--before", type=Path, default=Path("benchmark/performance-before.jsonl"))
    parser.add_argument("--after", type=Path, default=Path("benchmark/performance-data.jsonl"))
    parser.add_argument("--preview", type=Path, help="optional PNG preview for visual review")
    args = parser.parse_args()
    before_env, before = load(args.before)
    after_env, after = load(args.after)
    for field in ("cpu", "os", "ram_bytes", "requested_window", "sample_interval_ms"):
        if before_env[field] != after_env[field]:
            raise ValueError(f"Environment differs: {field}")
    if before.keys() != after.keys():
        raise ValueError("Case matrices differ")
    for key in before:
        if before[key]["sha256"] != after[key]["sha256"]:
            raise ValueError(f"Fixture changed: {key}")
        for field in ("effective_mode", "window_width", "window_height"):
            old = {r["open"][field] for r in before[key]["runs"]}
            new = {r["open"][field] for r in after[key]["runs"]}
            if old != new or len(new) != 1:
                raise ValueError(f"Rendering conditions differ: {key} {field}")
    keys = sorted(after)
    values = {k: metrics(v) for k, v in after.items()}
    previous = {k: metrics(v) for k, v in before.items()}
    out = args.after.parent
    version = after_env.get("product_version", "unrecorded version")
    plt.rcParams.update({"font.family": "sans-serif", "font.size": 10,
                         "svg.fonttype": "none", "svg.hashsalt": "mory-performance"})
    figure, axes = plt.subplots(3, 2, figsize=(13, 12), layout="constrained")
    short = [k[0].replace("-1m", "").replace("-12x1m", "") + " / " + k[1] for k in keys]
    for ax, (metric, title, unit) in zip(axes.flat, [
        ("loaded", "Loaded resident memory", "MiB"),
        ("peak", "Peak resident memory", "MiB"),
        ("edit", "Edit action (excludes rendering)", "ms"),
        ("capture", "Edit + rendering + PNG capture", "ms"),
        ("open", "Open action", "ms"),
        ("save", "Save action", "ms"),
    ]):
        y = list(range(len(keys)))
        old_bars = ax.barh([i - .19 for i in y], [previous[k][metric] for k in keys], height=.36,
                          label="Before", color="#a7b0ba")
        new_bars = ax.barh([i + .19 for i in y], [values[k][metric] for k in keys], height=.36,
                          label="After", color="#287b9b")
        if metric == "edit":
            for bars in (old_bars, new_bars):
                ax.bar_label(bars, fmt="%.2f", fontsize=7, padding=2)
            ax.margins(x=.15)
        ax.set_yticks(y, short, fontsize=8)
        ax.invert_yaxis()
        ax.set_title(title, loc="left", fontweight="bold")
        ax.set_xlabel(unit)
        ax.grid(axis="x", alpha=.2)
        ax.set_axisbelow(True)
        ax.spines[["top", "right"]].set_visible(False)
    axes[0, 0].legend(loc="lower right", frameon=False)
    figure.suptitle(f"Mory {version} · same-machine optimization\n"
                   f"{after_env['cpu']} · 1 MiB fixtures · medians of 3 runs", fontsize=16)
    figure.supxlabel("Large-document serialization is deferred to save boundaries. Capture is not display latency.",
                     fontsize=10)
    chart = out / "performance-overview.svg"
    figure.savefig(chart, metadata={"Date": None})
    chart.write_text("\n".join(line.rstrip() for line in chart.read_text().splitlines()) + "\n")
    if args.preview:
        figure.savefig(args.preview, dpi=120)
    plt.close(figure)
    lines = ["# Mory performance data", "",
             f"Mory **{version}**, {after_env['cpu']}, {after_env['ram_bytes'] / 2**30:g} GiB, "
             f"{after_env['os']}; requested window {after_env['requested_window']}. "
             "Six checked-in 1 MiB fixtures, Write and Source modes, three fresh processes per case, "
             "30 edits per process. Values below are medians across runs; edit values use each run's median. "
             "RSS is sampled every 50 ms. Long-line Write requests use the existing Source fallback; "
             "large tables retain the existing compact preview. These results do not measure full rich rendering of every table cell.", "",
             "The baseline build is identified by commit `" + before_env["commit"][:12] + "` "
             "and the worktree diff and binary hashes in its raw records. "
             "The optimized build adds indexed block lookup, compact typing undo records and deferred serialization "
             "for large block documents. Edit actions update the editor model; full source serialization now occurs "
             "at autosave, save and source-consuming commands. The edit speedup therefore includes work moved out "
             "of the input handler. Save timings include pending serialization; capture timings include rendering "
             "and PNG encoding. Neither is keyboard-to-display latency.", "",
             "![Mory performance overview](performance-overview.svg)", "",
             "## Before / after", "",
             "| Fixture | Mode | Open (ms) | Edit (ms) | Save (ms) | Loaded RSS (MiB) | Peak RSS (MiB) |",
             "| --- | --- | ---: | ---: | ---: | ---: | ---: |"]
    for k in keys:
        cells = [f"{previous[k][m]:.2f} / {values[k][m]:.2f}" for m in ("open", "edit", "save", "loaded", "peak")]
        lines.append("| " + " | ".join([f"`{k[0]}`", k[1]] + cells) + " |")
    lines += ["", "## Edit tail latency and capture", "",
              "P95 uses the nearest-rank percentile within each run, then the median of the three runs. "
              "Capture includes the action, window rendering and PNG encoding; it is not display latency.", "",
              "| Fixture | Mode | Edit P95 before / after (ms) | Capture median before / after (ms) |",
              "| --- | --- | ---: | ---: |"]
    for k in keys:
        lines.append(f"| `{k[0]}` | {k[1]} | {previous[k]['edit_p95']:.2f} / {values[k]['edit_p95']:.2f} "
                     f"| {previous[k]['capture']:.2f} / {values[k]['capture']:.2f} |")
    lines += ["", "[Before measurements](performance-before.jsonl) · [After measurements](performance-data.jsonl)", "",
              "The raw records retain fixture and binary SHA-256 identities, commit/worktree state and per-action samples. "
              "Runs are serial; other desktop activity is uncontrolled. This comparison measures Mory before and after "
              "optimization. The separate ArcMD and Quiet.md process probes do not establish an editor latency ranking.", "",
              "Reproduce: `mise exec -- python3 scripts/performance.py --fixture-dir benchmark/fixtures --runs 3 --edits 30`. "
              "Regenerate this report and chart with `python3 scripts/performance_report.py` after installing "
              "`scripts/performance-requirements.txt` in a virtual environment.", ""]
    (out / "performance-report.md").write_text("\n".join(lines))


if __name__ == "__main__":
    main()
