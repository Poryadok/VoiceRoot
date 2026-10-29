#!/usr/bin/env python3
"""Run a deterministic file partition of Flutter's default test discovery.

Weights are approximate aggregate per-file intervals from hosted baseline run
40818856273; unknown files get the median measured cost. Discovery follows
Flutter 3.41.7 TestCommand._findTests, including generated files.
"""
import argparse
import hashlib
import json
import math
import os
from pathlib import Path
import statistics
import subprocess


def discover(root):
    if not root.is_dir():
        raise ValueError(f"Test directory missing: {root}")

    def fail(error):
        raise error

    files = []
    for directory, _, names in os.walk(root, followlinks=False, onerror=fail):
        for name in names:
            path = Path(directory) / name
            if name.endswith("_test.dart") and path.is_file():
                files.append(path.relative_to(root).as_posix())
    return sorted(files)


def partition(files, weights, count):
    if count < 1 or len(files) < count or len(files) != len(set(files)):
        raise ValueError("Partitions require unique files and at least one file per shard")
    if any(not isinstance(v, (int, float)) or not math.isfinite(v) or v <= 0
           for v in weights.values()):
        raise ValueError("Weights must be finite positive numbers")
    default = statistics.median(weights.values()) if weights else 1
    bins = [[] for _ in range(count)]
    totals = [0] * count
    for path in sorted(files, key=lambda p: (-weights.get(p, default), p)):
        index = min(range(count), key=lambda i: (totals[i], i))
        bins[index].append(path)
        totals[index] += weights.get(path, default)
    if any(not group for group in bins):
        raise ValueError("Empty shard")
    return [sorted(group) for group in bins]



def snapshot(root):
    return {name: hashlib.sha256((root / name).read_bytes()).hexdigest()
            for name in discover(root)}


def identity():
    return {
        "sha": subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip(),
        "run": os.environ["GITHUB_RUN_ID"],
        "attempt": os.environ["GITHUB_RUN_ATTEMPT"],
        "flutter": json.loads(subprocess.check_output(["flutter", "--version", "--machine"], text=True))["frameworkRevision"],
    }


def validate_plan(plan, root):
    if plan["identity"] != identity() or plan["files"] != snapshot(root):
        raise ValueError("Checkout, toolchain, run, or test inventory differs from plan")
    files = list(plan["files"])
    groups = plan["groups"]
    if len(groups) != 4 or any(not g for g in groups) or sorted(sum(groups, [])) != sorted(files):
        raise ValueError("Incomplete, duplicate, or empty partition")


def verify_receipts(plan, receipts):
    if len(receipts) != len(plan["groups"]):
        raise ValueError("Missing or extra shard receipts")
    for index, receipt in enumerate(sorted(receipts, key=lambda r: r["index"])):
        if receipt != {"index": index, "identity": plan["identity"], "files": plan["groups"][index]}:
            raise ValueError("Shard receipt does not match plan")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("action", choices=["plan", "run", "verify"])
    parser.add_argument("--plan", required=True)
    parser.add_argument("--shard-index", type=int)
    parser.add_argument("--receipt")
    parser.add_argument("--receipts")
    args = parser.parse_args()
    root = Path("test")
    if args.action == "plan":
        files = snapshot(root)
        weights = json.loads(Path(__file__).with_name("flutter_test_weights.json").read_text(encoding="utf-8"))
        plan = {"identity": identity(), "files": files, "groups": partition(list(files), weights, 4)}
        Path(args.plan).write_text(json.dumps(plan, sort_keys=True), encoding="utf-8")
        return 0
    plan = json.loads(Path(args.plan).read_text(encoding="utf-8"))
    if args.action == "verify":
        receipts = [json.loads(p.read_text(encoding="utf-8")) for p in Path(args.receipts).glob("**/receipt.json")]
        verify_receipts(plan, receipts)
        return 0
    validate_plan(plan, root)
    if args.shard_index is None or not 0 <= args.shard_index < len(plan["groups"]) or not args.receipt:
        parser.error("valid shard index and receipt required")
    selected = plan["groups"][args.shard_index]
    print(f"Flutter file shard {args.shard_index}: {len(selected)} files", flush=True)
    result = subprocess.run(["flutter", "test", "--reporter", "expanded", "--concurrency=1",
                             "--no-pub", *("test/" + path for path in selected)]).returncode
    if result == 0:
        receipt = {"index": args.shard_index, "identity": plan["identity"], "files": selected}
        Path(args.receipt).write_text(json.dumps(receipt, sort_keys=True), encoding="utf-8")
    return result


if __name__ == "__main__":
    raise SystemExit(main())
