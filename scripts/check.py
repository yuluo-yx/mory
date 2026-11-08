#!/usr/bin/env python3
"""Run unit/UI tests and native E2E, then merge their coverage by source region."""
import os
from pathlib import Path
import subprocess
import tempfile


def run(*args, env=None):
    subprocess.run(args, check=True, env=env)


root = Path(__file__).resolve().parent.parent
os.chdir(root)
output = root / ".test-output"
output.mkdir(exist_ok=True)
with tempfile.TemporaryDirectory(prefix="check-", dir=output) as temp:
    work = Path(temp)
    run("go", "test", "-covermode=atomic", "-coverprofile=" + str(work / "unit.out"), "./...")
    run("go", "vet", "./...")
    run("go", "build", "-cover", "-covermode=atomic", "-coverpkg=./...", "-tags=e2e", "-o", str(work / "mory"), ".")
    coverage_dir = work / "native"
    coverage_dir.mkdir()
    run("python3", "scripts/e2e.py", str(work / "mory"), env=dict(os.environ, GOCOVERDIR=str(coverage_dir)))
    run("go", "tool", "covdata", "textfmt", "-i=" + str(coverage_dir), "-o=" + str(work / "native.out"))
    regions = {}
    for profile in [work / "unit.out", work / "native.out"]:
        for line in profile.read_text().splitlines()[1:]:
            location, statements, count = line.split()
            if "/automation.go:" in location:
                continue  # This instrumentation is absent from production builds.
            key = (location, int(statements))
            regions[key] = max(regions.get(key, 0), int(count) > 0)
    covered = sum(size for (_, size), hit in regions.items() if hit)
    total = sum(size for _, size in regions)
    target = root / "coverage.out"
    target.write_text("mode: set\n" + "".join(f"{location} {size} {int(hit)}\n" for (location, size), hit in sorted(regions.items())))
    percent = covered / total * 100
    print(f"Combined production coverage: {covered}/{total} statements = {percent:.2f}%", flush=True)
    if percent < 90:
        raise SystemExit("Coverage is below the required 90%.")
