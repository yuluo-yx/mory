#!/usr/bin/env python3
"""Measure full-table navigation in isolated native windows; emit raw JSONL."""
import hashlib
import json
from pathlib import Path
import platform
import subprocess
import sys
import tempfile

from performance import NativeRun


def main():
    repo = Path(__file__).resolve().parent.parent
    output = repo / ".test-output"
    output.mkdir(exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="table-perf-", dir=output) as temp:
        work = Path(temp)
        binary = work / "mory-perf"
        subprocess.run(["go", "build", "-trimpath", "-tags=performance,mygo_noinspector",
                        "-ldflags=-s -w", "-o", str(binary), "."], cwd=repo, check=True)
        environment = dict(
            product_version=json.loads((repo / "mygo.json").read_text())["version"],
            os=platform.platform(),
            cpu=subprocess.check_output(["sysctl", "-n", "machdep.cpu.brand_string"], text=True).strip(),
            ram_bytes=int(subprocess.check_output(["sysctl", "-n", "hw.memsize"], text=True)),
            commit=subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=repo, text=True).strip(),
            binary_sha256=hashlib.sha256(binary.read_bytes()).hexdigest(),
            source_diff_sha256=hashlib.sha256(subprocess.check_output(
                ["git", "diff", "HEAD", "--", "*.go", "go.mod", "go.sum"], cwd=repo)).hexdigest(),
            requested_window="2560x1312", sample_interval_ms=50,
            workflow="Open, settle, nine jumps across start/middle/end, settle; no edits")
        print(json.dumps(dict(environment=environment)), flush=True)
        failed = False
        for name in ("table-heavy-1m", "table-long-cells-1m", "target-shape-12x1m"):
            data = (repo / "benchmark" / "fixtures" / (name + ".md")).read_bytes()
            for mode in ("Write", "Read"):
                runs, failures = [], []
                for repeat in range(3):
                    case = work / f"{name}-{mode}-{repeat}"
                    case.mkdir()
                    path = case / "document.md"
                    path.write_bytes(data)
                    native = None
                    try:
                        native = NativeRun(binary, case, 2560, 1312, 60)
                        opened = native.call("open", Path=str(path), Mode=mode)
                        if opened["effective_mode"] != mode:
                            raise AssertionError("Document switched away from the requested mode")
                        loaded = native.settled_rss()
                        jumps = []
                        for position in ("middle", "end", "start") * 3:
                            jump = native.call("table-scroll", Position=position)
                            if not jump["table_first_visible_row"] <= jump["table_target_row"] <= jump["table_last_visible_row"]:
                                raise AssertionError(f"Target row was not visible: {jump}")
                            jumps.append(jump)
                        after = native.settled_rss()
                        if path.read_bytes() != data:
                            raise AssertionError("Reading changed the document bytes")
                        runs.append(dict(open=opened, loaded_rss_mib=loaded, jumps=jumps,
                                         after_scroll_rss_mib=after,
                                         sampled_peak_rss_mib=max(value for _, value in native.samples)))
                    except (OSError, RuntimeError, AssertionError) as error:
                        failures.append(str(error))
                        failed = True
                    finally:
                        if native:
                            native.close()
                    print(f"Measured {name} {mode} run {repeat+1}/3", file=sys.stderr, flush=True)
                print(json.dumps(dict(kind=name, mode=mode, bytes=len(data),
                                      sha256=hashlib.sha256(data).hexdigest(), runs=runs,
                                      failures=failures)), flush=True)
        if failed:
            raise SystemExit("Table benchmark contains failed runs")


if __name__ == "__main__":
    main()
