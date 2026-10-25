"""Qualify only a fixed independently reviewed historical test merge result.

This supplies provenance for that actual conflict resolution, never a current
Role/native/runtime path exemption. The caller retains every source row, prior
runtime flag and content base and still runs the ordinary scope/content oracle.
"""
import argparse
import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import subprocess
import sys

PINNED_RECORD_COMMIT = "aea603c70303759b6709b3e9eac13584cb80c789"
PINNED_RECORD_SHA256 = "92bdff9d8a0cac4b666eafbd33cff17131a1d1eab769f5f5fc3be11ab4e6320d"
RECORD_PATH = "docs/testing/role-test-merge-31276-v1.json"
VERIFIER_PATH = "scripts/ci/f13_reviewed_test_merge.py"


class Rejected(ValueError):
    pass


def require(value, reason):
    if not value:
        raise Rejected(reason)


def oid(value):
    require(isinstance(value, str) and re.fullmatch(r"[0-9a-f]{40}", value), "object_id")
    return value


def git(repo, *args):
    r = subprocess.run(["git", "--no-replace-objects", "-C", str(repo), *args], capture_output=True)
    require(r.returncode == 0, "git_" + args[0])
    return r.stdout


def ancestor(repo, older, newer):
    git(repo, "merge-base", "--is-ancestor", older, newer)


def pairs(items):
    result = {}
    for key, value in items:
        require(key not in result, "duplicate_record_key")
        result[key] = value
    return result


def entry(repo, revision, path):
    data = git(repo, "ls-tree", "-z", revision, "--", ":(literal)" + path).split(b"\0")
    require(len(data) == 2 and data[-1] == b"", "missing_or_multiple_entry")
    meta, actual_path = data[0].decode().split("\t", 1)
    mode, kind, value = meta.split()
    require(actual_path == path, "literal_path")
    return {"mode": mode, "type": kind, "oid": value}


def qualify(repo, target, merge, first, source_base, source, selected_path):
    for value in (target, merge, first, source_base, source, PINNED_RECORD_COMMIT):
        oid(value)
    require(PINNED_RECORD_COMMIT != "0" * 40 and re.fullmatch(r"[0-9a-f]{64}", PINNED_RECORD_SHA256), "record_pin")
    raw = git(repo, "show", PINNED_RECORD_COMMIT + ":" + RECORD_PATH)
    require(hashlib.sha256(raw).hexdigest() == PINNED_RECORD_SHA256, "record_hash")
    record_entry = entry(repo, PINNED_RECORD_COMMIT, RECORD_PATH)
    require(record_entry["mode"] == "100644" and record_entry["type"] == "blob", "record_entry")
    r = json.loads(raw, object_pairs_hook=pairs)
    require(isinstance(r, dict) and set(r) == {"version", "consumer", "merge", "parents", "best_bases", "conflicts", "review", "evidence", "exclusions"}, "record_shape")
    require(r["version"] == "F13-REVIEWED-TEST-MERGE-v1" and r["consumer"] == "HISTORICAL-TEST-MERGE-ONLY", "record_type")
    require(r["merge"] == merge and r["parents"] == [first, source], "historical_identity")
    require(isinstance(r["review"], str) and r["review"].startswith(("https://", "http://")), "independent_review")
    actual_parents = git(repo, "rev-list", "--parents", "-n", "1", merge).decode().split()
    require(actual_parents == [merge, first, source], "actual_parents")
    bases = sorted(git(repo, "merge-base", "--all", first, source).decode().splitlines())
    require(bases == r["best_bases"] and bases == [source_base], "actual_source_base")
    ancestor(repo, merge, PINNED_RECORD_COMMIT)
    ancestor(repo, PINNED_RECORD_COMMIT, target)
    require(merge != PINNED_RECORD_COMMIT and target != PINNED_RECORD_COMMIT, "record_or_carrier_lineage")
    require(git(repo, "show", target + ":" + VERIFIER_PATH) == Path(__file__).read_bytes(), "executed_carrier")
    require(git(repo, "show", target + ":" + RECORD_PATH) == raw, "record_drift")
    require(isinstance(r["conflicts"], list) and r["conflicts"], "conflict_set")
    names = []
    for row in r["conflicts"]:
        require(isinstance(row, dict) and set(row) == {"path", "base", "first", "source", "result"}, "conflict_shape")
        name = row["path"]
        require(isinstance(name, str) and name.endswith("_test.go") and not re.search(r"[\s\x00-\x1f:]", name), "test_only")
        p = PurePosixPath(name)
        require(str(p) == name and not p.is_absolute() and ".." not in p.parts, "path")
        names.append(name)
        for label, revision in (("base", source_base), ("first", first), ("source", source), ("result", merge)):
            value = row[label]
            require(isinstance(value, dict) and set(value) == {"mode", "type", "oid"}, "entry_shape")
            oid(value["oid"])
            require(value["mode"] == "100644" and value["type"] == "blob", "test_entry_type")
            require(entry(repo, revision, name) == value, "actual_entry")
    require(names == sorted(set(names)) and selected_path in names, "conflict_order_or_selection")
    # Require the entire real conflict set. A new production/schema/root/docs
    # conflict or a malformed/incomplete merge-tree cannot borrow this record.
    merged = subprocess.run(["git", "--no-replace-objects", "-C", str(repo), "merge-tree", "--write-tree", "--name-only", "-z", "--no-messages", first, source], capture_output=True)
    require(merged.returncode == 1, "actual_conflict_required")
    chunks = merged.stdout.split(b"\0")
    require(len(chunks) >= 3 and re.fullmatch(rb"[0-9a-f]{40,64}", chunks[0]) and chunks[-1] == b"", "merge_tree_output")
    conflict_names = [name.decode() for name in chunks[1:-1]]
    require(sorted(set(conflict_names)) == names and len(conflict_names) == len(names), "complete_actual_conflicts")
    require(isinstance(r["evidence"], list) and r["evidence"], "evidence")
    for proof in r["evidence"]:
        require(set(proof) == {"name", "sha256"} and isinstance(proof["name"], str)
                and re.fullmatch(r"[0-9a-f]{64}", proof["sha256"]), "evidence_hash")
    require(isinstance(r["exclusions"], list) and all(isinstance(v, str) for v in r["exclusions"]), "exclusions")
    return {"version": r["version"], "record": PINNED_RECORD_COMMIT, "record_sha256": PINNED_RECORD_SHA256,
            "merge": merge, "first": first, "source_base": source_base, "source": source, "path": selected_path,
            "review": r["review"], "historical_only": True, "retain_runtime_and_content_taint": True}


def main():
    p = argparse.ArgumentParser(description=__doc__)
    for key in ("repo", "target", "merge", "first", "source-base", "source", "path"):
        p.add_argument("--" + key, required=True)
    args = p.parse_args()
    try:
        print(json.dumps(qualify(args.repo, args.target, args.merge, args.first, args.source_base, args.source, args.path), sort_keys=True))
    except (Rejected, ValueError, KeyError, TypeError, OSError) as exc:
        print("Reviewed historical test merge rejected: " + str(exc), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
