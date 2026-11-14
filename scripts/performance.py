#!/usr/bin/env python3
"""Measure a real macOS window with isolated documents and libraries.

Run through mise so the Go toolchain matches the project. All binaries, fixture
files, histories and sockets are removed on exit. JSON output can be redirected
to a task-local file; README contains the checked-in measurements and methodology.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import socket
import statistics
import subprocess
import sys
import tempfile
import threading
import time


def fixture(kind, size):
    if kind == "structured":
        unit = ("## A structured section\n\n:::tip Keep writing\nA useful thought.\n:::\n\n"
                "| Name | Value |\n| --- | --- |\n| Alpha | 123 |\n\n")
    elif kind == "table-heavy":
        header = "| " + " | ".join(f"Column {i}" for i in range(1, 17)) + " |\n"
        divider = "| " + " | ".join("---" for _ in range(16)) + " |\n"
        unit = header + divider + "| " + " | ".join(
            "**value** with [link](https://example.com) and `code`" for _ in range(16)
        ) + " |\n"
    elif kind == "memory-heavy":
        unit = ("## Memory pressure section\n\n"
                "A paragraph with **bold**, *italic*, ~~strike~~, `inline code`, "
                "[a link](https://example.com), $x^2+y^2$, 中文文本 and emoji 🧪.\n\n"
                "> A quoted paragraph with **nested emphasis** and a [reference](https://example.com).\n\n"
                "- [ ] task one\n- [x] task two\n- nested item with `code`\n\n"
                "```go\nfunc render() string { return \"large retained code\" }\n```\n\n")
    elif kind == "long-line":
        unit = "A long unbroken paragraph with words and spaces. "
    else:
        unit = ("A paragraph of local writing, with **bold text**, a [link](https://example.com), "
                "and Unicode: 中文笔记。 Keep the next sentence readable.\n\n")
    chunk = unit.encode()
    # Complete repetitions preserve valid UTF-8 and Markdown. Report exact size.
    return chunk * max(1, size // len(chunk))


class NativeRun:
    def __init__(self, binary, root, width, height, timeout):
        self.root = root
        self.sock_path = root / "p.sock"
        self.samples = []
        self.stop = threading.Event()
        self.timeout = timeout
        self.errors = (root / "stderr.log").open("wb")
        begin = time.perf_counter()
        self.process = subprocess.Popen([str(binary)], stdout=subprocess.DEVNULL, stderr=self.errors,
            env=dict(os.environ, MORY_LIBRARY=str(root / "library"),
                     MORY_PERFORMANCE_SOCKET=str(self.sock_path),
                     MORY_PERFORMANCE_WIDTH=str(width), MORY_PERFORMANCE_HEIGHT=str(height)))
        self.monitor = threading.Thread(target=self.sample_rss, daemon=True)
        self.monitor.start()
        self.channel = socket.socket(socket.AF_UNIX)
        self.channel.settimeout(timeout)
        while not self.sock_path.exists():
            if self.process.poll() is not None or time.perf_counter() - begin > 30:
                self.close()
                raise RuntimeError("Native benchmark did not start")
            time.sleep(.01)
        self.channel.connect(str(self.sock_path))
        self.stream = self.channel.makefile("rwb")
        try:
            self.call("capture")
        except Exception:
            self.close()
            raise
        self.startup_ms = (time.perf_counter() - begin) * 1000

    def sample_rss(self):
        while not self.stop.is_set():
            result = subprocess.run(["ps", "-o", "rss=", "-p", str(self.process.pid)], capture_output=True, text=True)
            if result.returncode == 0 and result.stdout.strip():
                self.samples.append((time.monotonic(), int(result.stdout.strip()) / 1024))
            self.stop.wait(.05)

    def call(self, action, **kwargs):
        self.stream.write((json.dumps(dict(Action=action, **kwargs)) + "\n").encode())
        self.stream.flush()
        try:
            line = self.stream.readline()
        except TimeoutError as error:
            raise RuntimeError(f"{action} did not complete within {self.timeout:g} seconds") from error
        if not line:
            raise RuntimeError("Native benchmark exited during " + action)
        result = json.loads(line)
        if result["error"]:
            raise RuntimeError(result["error"])
        return result

    def settled_rss(self):
        # Leave autosave and normal GC enabled. No forced GC or cache purge.
        time.sleep(2)
        before = self.call("stats")["cpu_seconds"]
        start = time.monotonic()
        time.sleep(1)
        after = self.call("stats")["cpu_seconds"]
        self.idle_cpu_percent = (after - before) / (time.monotonic() - start) * 100
        return statistics.median(value for stamp, value in self.samples if stamp >= start)

    def close(self):
        if hasattr(self, "stream"):
            try:
                self.stream.write(b'{"Action":"quit"}\n')
                self.stream.flush()
            except (OSError, ValueError):
                pass
        try:
            self.process.wait(timeout=10)
        except subprocess.TimeoutExpired:
            self.process.terminate()
            try:
                self.process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                self.process.kill()
                self.process.wait()
        self.stop.set()
        self.monitor.join(timeout=2)
        if hasattr(self, "stream"):
            try:
                self.stream.close()
            except OSError:
                pass
        try:
            self.channel.close()
        except OSError:
            pass
        self.errors.close()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--runs", type=int, default=3)
    parser.add_argument("--edits", type=int, default=30)
    parser.add_argument("--sizes", default="102400,1048576,10485760")
    parser.add_argument("--kinds", default="prose,structured,long-line")
    parser.add_argument("--fixture-dir", type=Path,
                        help="measure the checked-in Markdown fixtures in this directory")
    parser.add_argument("--fixtures", default="",
                        help="comma-separated fixture stems to select with --fixture-dir")
    parser.add_argument("--modes", default="Write,Source")
    parser.add_argument("--width", type=int, default=2560)
    parser.add_argument("--height", type=int, default=1312)
    parser.add_argument("--timeout", type=float, default=60,
                        help="per-action native benchmark timeout in seconds")
    args = parser.parse_args()
    if args.runs < 1 or args.edits < 1 or args.width < 1 or args.height < 1 or args.timeout <= 0:
        parser.error("runs, edits, dimensions and timeout must be positive")
    if args.fixture_dir is None and not set(args.kinds.split(",")) <= {"prose", "structured", "table-heavy", "memory-heavy", "long-line"}:
        parser.error("kinds must be prose, structured, table-heavy, memory-heavy or long-line")
    if not set(args.modes.split(",")) <= {"Write", "Source"}:
        parser.error("editing measurements support Write and Source modes")
    try:
        sizes = [int(value) for value in args.sizes.split(",")]
    except ValueError:
        parser.error("sizes must be comma-separated byte counts")
    if any(size < 1 for size in sizes):
        parser.error("sizes must be positive")
    repo = Path(__file__).resolve().parent.parent
    output = repo / ".test-output"
    output.mkdir(exist_ok=True)
    fixture_cases = []
    if args.fixture_dir is not None:
        fixture_dir = args.fixture_dir
        if not fixture_dir.is_absolute():
            fixture_dir = repo / fixture_dir
        fixture_paths = sorted(fixture_dir.glob("*.md"))
        selected = {value for value in args.fixtures.split(",") if value}
        if selected:
            fixture_paths = [path for path in fixture_paths if path.stem in selected]
        if not fixture_paths:
            parser.error(f"fixture directory has no Markdown files: {fixture_dir}")
        for path in fixture_paths:
            fixture_cases.append((path.stem, str(path.relative_to(repo)), path.read_bytes()))
    else:
        fixture_cases = [(kind, "", None) for kind in args.kinds.split(",")]
    metadata = dict(date=time.strftime("%Y-%m-%d"), os=platform.platform(),
                    product_version=json.loads((repo / "mygo.json").read_text())["version"],
                    cpu=subprocess.check_output(["sysctl", "-n", "machdep.cpu.brand_string"], text=True).strip(),
                    ram_bytes=int(subprocess.check_output(["sysctl", "-n", "hw.memsize"], text=True)),
                    go=subprocess.check_output(["go", "version"], text=True).strip(),
                    commit=subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=repo, text=True).strip(),
                    tracked_worktree_dirty=bool(subprocess.check_output(["git", "diff", "HEAD", "--name-only"], cwd=repo, text=True).strip()),
                    requested_window=f"{args.width}x{args.height}", sample_interval_ms=50,
                    fixture_dir=str(args.fixture_dir) if args.fixture_dir else None,
                    action_timeout_seconds=args.timeout)
    with tempfile.TemporaryDirectory(prefix="perf-", dir=output) as temp:
        work = Path(temp)
        binary = work / "mory-perf"
        subprocess.run(["go", "build", "-trimpath", "-tags=performance,mygo_noinspector",
                        "-ldflags=-s -w", "-o", str(binary), "."], cwd=repo, check=True)
        metadata["binary_sha256"] = hashlib.sha256(binary.read_bytes()).hexdigest()
        metadata["source_diff_sha256"] = hashlib.sha256(subprocess.check_output(
            ["git", "diff", "HEAD", "--", "*.go", "go.mod", "go.sum"], cwd=repo)).hexdigest()
        print(json.dumps(dict(environment=metadata)), flush=True)
        for kind, fixture_path, fixture_data in fixture_cases:
            case_sizes = [len(fixture_data)] if fixture_data is not None else sizes
            for size in case_sizes:
                # Dense tables and unbroken lines are stress fixtures; cap their
                # default sizes rather than silently representing them as prose.
                if fixture_data is None and kind != "prose" and size > 1048576:
                    continue
                for mode in args.modes.split(","):
                    results, failures = [], []
                    for repeat in range(args.runs):
                        safe_kind = re.sub(r"[^A-Za-z0-9_.-]+", "_", kind)
                        case = work / f"{safe_kind}-{size}-{mode}-{repeat}"
                        case.mkdir()
                        path = case / "document.md"
                        data = fixture_data if fixture_data is not None else fixture(kind, size)
                        path.write_bytes(data)
                        native = None
                        try:
                            native = NativeRun(binary, case, args.width, args.height, args.timeout)
                            idle = native.settled_rss()
                            idle_cpu = native.idle_cpu_percent
                            opened = native.call("open", Path=str(path), Mode=mode)
                            loaded = native.settled_rss()
                            loaded_cpu = native.idle_cpu_percent
                            edits = []
                            for i in range(args.edits):
                                edits.append(native.call("edit", Position="start", Text="x"))
                            before_save = path.read_bytes()
                            saved = native.call("save")
                            if path.read_bytes() == before_save:
                                raise AssertionError("Edits did not change the original file")
                            undo = native.call("undo")
                            scrolling = [native.call("scroll") for _ in range(10)] if mode == "Source" else []
                            after_edit = native.settled_rss()
                            record = dict(startup_ms=native.startup_ms, idle_rss_mib=idle,
                                open=opened, loaded_rss_mib=loaded, edits=edits, save=saved, undo=undo,
                                scroll=scrolling, after_edit_rss_mib=after_edit,
                                home_idle_cpu_percent=idle_cpu, loaded_idle_cpu_percent=loaded_cpu,
                                after_edit_idle_cpu_percent=native.idle_cpu_percent,
                                sampled_peak_rss_mib=max(value for _, value in native.samples))
                            results.append(record)
                        except (OSError, RuntimeError, AssertionError) as error:
                            failures.append(str(error))
                        finally:
                            if native:
                                native.close()
                        print(f"Measured {kind} {len(data)} bytes {mode} run {repeat+1}/{args.runs}", file=sys.stderr, flush=True)
                    print(json.dumps(dict(kind=kind, fixture=fixture_path, bytes=len(data),
                        sha256=hashlib.sha256(data).hexdigest(), lines=data.count(b"\n")+1,
                        mode=mode, runs=results, failures=failures)), flush=True)


if __name__ == "__main__":
    main()
