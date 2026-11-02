#!/usr/bin/env python3
"""Validate diagnostic test2json without converting a failed Ban into PASS."""
import json
import math
import re
import sys

TEST = "TestNative50Over100ProfileCatalog"
PACKAGE = "voice/backend/space/internal/store"
PROFILE = re.compile(r"NATIVE50_PROFILE_V1 result=NONE elapsed_ns=([0-9]+) catalog_calls=([0-9]+) catalog_total_ms=([0-9]+(?:\.[0-9]+)?) catalog_self_ms=([0-9]+(?:\.[0-9]+)?)\s*$")


def validate(lines, result):
    events = [json.loads(line) for line in lines if line.strip()]
    if any(not isinstance(e, dict) or e.get("Package") != PACKAGE or (e.get("Test") and e.get("Test") != TEST) for e in events):
        raise ValueError("unexpected event")
    if any(e.get("Action") in ("fail", "skip") for e in events):
        raise ValueError("failed or skipped event")
    tests = [e for e in events if e.get("Test") == TEST]
    runs = [e for e in tests if e.get("Action") == "run"]
    terminal = [e for e in tests if e.get("Action") in ("pass", "fail", "skip")]
    packages = [e for e in events if e.get("Package") == PACKAGE and not e.get("Test") and e.get("Action") in ("pass", "fail", "skip")]
    profiles = [e for e in tests if e.get("Action") == "output" and "NATIVE50_PROFILE_V1 " in e.get("Output", "")]
    if len(runs) != 1 or len(terminal) != 1 or len(packages) != 1:
        raise ValueError("incomplete or duplicate test events")
    if result != 0 or terminal[0]["Action"] != "pass" or packages[0]["Action"] != "pass":
        raise ValueError("diagnostic test failed or skipped")
    if len(profiles) != 1 or "result=NONE " not in profiles[0]["Output"]:
        raise ValueError("missing or invalid diagnostic observation")
    output = profiles[0]["Output"]
    match = PROFILE.search(output)
    if match is None:
        raise ValueError("malformed diagnostic observation")
    elapsed, calls = int(match[1]), int(match[2])
    total, self_time = float(match[3]), float(match[4])
    if elapsed <= 0 or calls <= 0 or not math.isfinite(total) or not math.isfinite(self_time) or self_time > total:
        raise ValueError("invalid diagnostic counters")
    if not (events.index(runs[0]) < events.index(profiles[0]) < events.index(terminal[0]) < events.index(packages[0])):
        raise ValueError("diagnostic events out of order")


def main():
    try:
        with open(sys.argv[1], encoding="utf-8") as events:
            validate(events, int(sys.argv[2]))
    except (ValueError, OSError, IndexError, TypeError, KeyError, json.JSONDecodeError):
        print("M50 diagnostic evidence rejected", file=sys.stderr)
        return 1
    print("M50 diagnostic evidence recorded; not acceptance proof")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
