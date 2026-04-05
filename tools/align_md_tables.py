#!/usr/bin/env python3
"""
Find markdown tables under docs/ and pad cells so columns align in a monospace editor.

Usage:
  python tools/align_md_tables.py              # rewrite files in place
  python tools/align_md_tables.py --dry-run    # print what would change
  python tools/align_md_tables.py --path docs/features
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

# Separator row: |---|, |:---|, |---:|, |:---:| per cell
_SEP_CELL = re.compile(r"^\s*:?-{3,}:?\s*$")


def is_separator_line(line: str) -> bool:
    s = line.strip()
    if not s.startswith("|") or "|" not in s[1:]:
        return False
    raw = s.strip()
    if not (raw.startswith("|") and raw.endswith("|")):
        return False
    inner = raw[1:-1].split("|")
    if not inner:
        return False
    return all(_SEP_CELL.match(c.strip()) for c in inner)


def split_row(line: str) -> list[str]:
    """Split a table line into cell contents (no outer pipes)."""
    s = line.rstrip("\n")
    if not s.strip().startswith("|"):
        return []
    # Trim leading pipe region: first | ... last |
    t = s.strip()
    if not t.startswith("|"):
        return []
    if t.endswith("|"):
        t = t[1:-1]
    else:
        t = t[1:]
    return [c.strip() for c in t.split("|")]


def normalize_col_count(rows: list[list[str]], n: int) -> None:
    for r in rows:
        while len(r) < n:
            r.append("")
        del r[n:]


def column_widths(header: list[str], body: list[list[str]]) -> list[int]:
    n = max(len(header), max((len(r) for r in body), default=0))
    normalize_col_count([header], n)
    normalize_col_count(body, n)
    widths = [len(header[i]) for i in range(n)]
    for r in body:
        for i in range(n):
            widths[i] = max(widths[i], len(r[i]))
    return widths


def format_separator(widths: list[int]) -> str:
    parts = ["|" + "-" * (w + 2) for w in widths]
    return "".join(parts) + "|"


def format_row(cells: list[str], widths: list[int]) -> str:
    padded = [cells[i].ljust(widths[i]) for i in range(len(widths))]
    return "| " + " | ".join(padded) + " |"


def extract_table_block(lines: list[str], start: int) -> tuple[int, int] | None:
    """
    If lines[start] starts a GFM table, return (start, end_exclusive).
    Requires a header row and a separator row immediately after.
    """
    if start >= len(lines):
        return None
    if not lines[start].strip().startswith("|"):
        return None
    if start + 1 >= len(lines):
        return None
    if not is_separator_line(lines[start + 1]):
        return None
    header_cells = split_row(lines[start])
    if len(header_cells) < 1:
        return None
    end = start + 2
    while end < len(lines):
        ln = lines[end]
        if not ln.strip().startswith("|"):
            break
        if is_separator_line(ln):
            break
        end += 1
    return start, end


def realign_table(header_line: str, sep_line: str, body_lines: list[str]) -> list[str]:
    header = split_row(header_line)
    body = [split_row(ln) for ln in body_lines]
    if not header:
        return [header_line.rstrip("\n"), sep_line.rstrip("\n"), *[ln.rstrip("\n") for ln in body_lines]]
    widths = column_widths(header, body)
    out = [
        format_row(header, widths),
        format_separator(widths),
    ]
    for r in body:
        out.append(format_row(r, widths))
    return out


def process_file(path: Path, dry_run: bool) -> bool:
    text = path.read_text(encoding="utf-8")
    has_final_nl = text.endswith("\n") or text.endswith("\r\n")
    text_n = text.replace("\r\n", "\n")
    raw_lines = text_n.split("\n")
    if raw_lines and raw_lines[-1] == "":
        raw_lines.pop()

    out: list[str] = []
    i = 0
    while i < len(raw_lines):
        block = extract_table_block(raw_lines, i)
        if block is None:
            out.append(raw_lines[i])
            i += 1
            continue
        start, end = block
        new_rows = realign_table(
            raw_lines[start],
            raw_lines[start + 1],
            raw_lines[start + 2 : end],
        )
        out.extend(new_rows)
        i = end

    new_text = "\n".join(out)
    if has_final_nl:
        new_text += "\n"

    if new_text == text_n:
        return False

    if dry_run:
        print(f"would update: {path}")
    else:
        path.write_text(new_text, encoding="utf-8", newline="\n")
        print(f"updated: {path}")
    return True


def main() -> int:
    ap = argparse.ArgumentParser(description="Align markdown tables under docs/.")
    ap.add_argument(
        "--path",
        type=Path,
        default=Path("docs"),
        help="Root directory to scan (default: docs)",
    )
    ap.add_argument("--dry-run", action="store_true", help="Do not write files")
    args = ap.parse_args()
    root: Path = args.path
    if not root.is_dir():
        print(f"not a directory: {root}", file=sys.stderr)
        return 1

    changed = 0
    for md in sorted(root.rglob("*.md")):
        if process_file(md, args.dry_run):
            changed += 1
    print(f"files {'that would change' if args.dry_run else 'updated'}: {changed}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
