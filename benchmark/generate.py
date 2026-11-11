#!/usr/bin/env python3
"""Generate deterministic Markdown stress fixtures for native performance runs."""

from pathlib import Path
import argparse


def repeat_to_size(unit: str, size: int) -> str:
    if not unit:
        raise ValueError("fixture unit must not be empty")
    repetitions = size // len(unit.encode("utf-8")) + 1
    return (unit * repetitions).encode("utf-8")[:size].decode("utf-8", errors="ignore")


def table_fixture(columns: int, size: int, long_cells: bool = False) -> str:
    header = "| " + " | ".join(f"Column {i}" for i in range(1, columns + 1)) + " |\n"
    divider = "| " + " | ".join("---" for _ in range(columns)) + " |\n"
    cell = "**value** with [link](https://example.com), `code`, $x^2+y^2$"
    if long_cells:
        cell += " and a long cell payload " + ("中文文本 " * 42)
    row = "| " + " | ".join(cell for _ in range(columns)) + " |\n"
    return repeat_to_size(header + divider + row, size)


def mixed_fixture(size: int) -> str:
    unit = (
        "## Memory pressure section\n\n"
        "A paragraph with **bold**, *italic*, ~~strike~~, `inline code`, "
        "[a link](https://example.com), $x^2+y^2$, 中文文本 and emoji 🧪.\n\n"
        "> A quoted paragraph with **nested emphasis** and a [reference](https://example.com).\n\n"
        "- [ ] task one\n- [x] task two\n  - nested item with `code`\n\n"
        "```go\nfunc render() string { return \"large retained code\" }\n```\n\n"
    )
    return repeat_to_size(unit, size)


def structured_fixture(size: int) -> str:
    unit = (
        "## A structured section\n\n"
        ":::tip Keep writing\nA useful thought with **emphasis** and a link.\n:::\n\n"
        "| Name | Value | Status |\n| --- | --- | --- |\n"
        "| Alpha | 123 | **ready** |\n\n"
    )
    return repeat_to_size(unit, size)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, default=Path(__file__).parent / "fixtures")
    parser.add_argument("--size", type=int, default=1024 * 1024)
    args = parser.parse_args()
    if args.size < 1024:
        parser.error("size must be at least 1024 bytes")

    args.output.mkdir(parents=True, exist_ok=True)
    fixtures = {
        "table-heavy-1m.md": table_fixture(16, args.size),
        "table-long-cells-1m.md": table_fixture(12, args.size, long_cells=True),
        "target-shape-12x1m.md": table_fixture(12, args.size),
        "memory-heavy-1m.md": mixed_fixture(args.size),
        "structured-1m.md": structured_fixture(args.size),
        "long-line-1m.md": repeat_to_size("A long unbroken paragraph with words and spaces. ", args.size),
    }
    for name, content in fixtures.items():
        path = args.output / name
        path.write_text(content, encoding="utf-8")
        print(f"{path}: {path.stat().st_size} bytes, {content.count(chr(10)) + 1} lines")


if __name__ == "__main__":
    main()
