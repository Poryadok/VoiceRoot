#!/usr/bin/env python3
"""
Find markdown tables under docs/ and pad cells so columns align in a monospace editor.

Usage:
  python tools/align_md_tables.py              # rewrite files in place
  python tools/align_md_tables.py --dry-run    # print what would change
  python tools/align_md_tables.py --path docs/features
  python tools/align_md_tables.py --path docs/GLOSSARY.md   # one file
  python tools/align_md_tables.py -v                        # per-file table count; explains 0 updates

If a file shows 0 updates but the IDE still looks unaligned, the saved file on disk is often already
padded (run with -v). Save the buffer (or reload from disk) so the runner sees the same bytes as the editor.
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

# Second table row: starts/ends with |; inside only |, -, :, whitespace; must contain at least one -.
_SEP_LINE = re.compile(r"^\|[-|:\s]+\|$")


def is_separator_line(line: str) -> bool:
    s = line.strip()
    if len(s) < 3 or "-" not in s:
        return False
    return bool(_SEP_LINE.fullmatch(s))


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


def process_file(path: Path, dry_run: bool) -> tuple[bool, int]:
    """Returns (file_changed, number_of_gfm_tables_realigned)."""
    text = path.read_text(encoding="utf-8-sig")
    has_final_nl = text.endswith("\n") or text.endswith("\r\n")
    text_n = text.replace("\r\n", "\n")
    raw_lines = text_n.split("\n")
    if raw_lines and raw_lines[-1] == "":
        raw_lines.pop()

    out: list[str] = []
    i = 0
    table_count = 0
    while i < len(raw_lines):
        block = extract_table_block(raw_lines, i)
        if block is None:
            out.append(raw_lines[i])
            i += 1
            continue
        table_count += 1
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
        return False, table_count

    if dry_run:
        print(f"would update: {path}")
    else:
        path.write_text(new_text, encoding="utf-8", newline="\n")
        print(f"updated: {path}")
    return True, table_count


def main() -> int:
    ap = argparse.ArgumentParser(description="Align markdown tables under docs/.")
    ap.add_argument(
        "--path",
        type=Path,
        default=Path("docs"),
        help="Root directory to scan, or a single .md file (default: docs)",
    )
    ap.add_argument("--dry-run", action="store_true", help="Do not write files")
    ap.add_argument(
        "-v",
        "--verbose",
        action="store_true",
        help="Per file: table count and whether content changed",
    )
    args = ap.parse_args()
    root: Path = args.path
    if root.is_file() and root.suffix.lower() == ".md":
        md_files = [root]
    elif root.is_dir():
        md_files = sorted(root.rglob("*.md"))
    else:
        print(f"not a directory or .md file: {root}", file=sys.stderr)
        return 1

    changed = 0
    tables_total = 0
    for md in md_files:
        updated, n_tables = process_file(md, args.dry_run)
        tables_total += n_tables
        if updated:
            changed += 1
        if args.verbose:
            status = "updated" if updated else "unchanged"
            print(f"{md}: {n_tables} table(s), {status}")
            if not updated and n_tables:
                print("  (already aligned on disk — save the file if the editor shows compact rows.)")
    print(f"files {'that would change' if args.dry_run else 'updated'}: {changed}")
    if args.verbose:
        print(f"tables scanned: {tables_total}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
