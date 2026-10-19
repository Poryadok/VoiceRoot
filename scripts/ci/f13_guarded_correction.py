"""Verify a source-pinned, complete guarded correction against real Git rows.

This qualifies provenance only. The caller must retain guarded taint, runtime
trigger/content history and the downstream R22 path/activation oracle.
"""
import argparse
import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import subprocess
import sys

PINNED_RECORD_COMMIT = "fd8b08b5245ea5272c3084e9cb76838777a843d8"
PINNED_RECORD_SHA256 = "1f8893672c7843ec00f35b6ef3301b9ab609b30d167a2461080a142e2e191d94"
RECORD_PATH = "docs/testing/guarded-correction-voice013-v1.json"


class Rejected(ValueError):
    pass


def require(condition, reason):
    if not condition:
        raise Rejected(reason)


def git(repo, *args):
    r = subprocess.run(["git", "--no-replace-objects", "-C", str(repo), *args],
                       capture_output=True, check=False)
    require(r.returncode == 0, "git_object_or_ancestry")
    return r.stdout


def ancestor(repo, older, newer):
    git(repo, "merge-base", "--is-ancestor", older, newer)


def oid(value):
    require(isinstance(value, str) and re.fullmatch(r"[0-9a-f]{40}", value), "object_id")
    return value


def path(value):
    require(isinstance(value, str) and value and not re.search(r"[\s\x00-\x1f:]", value), "path")
    p = PurePosixPath(value)
    require(not p.is_absolute() and ".." not in p.parts and str(p) == value, "path")
    return value


def shape(value, keys):
    require(isinstance(value, dict) and set(value) == set(keys), "record_shape")


def entry_shape(value):
    if value is None:
        return
    shape(value, ("mode", "type", "oid"))
    require(value["mode"] in ("100644", "100755", "120000", "160000", "040000"), "entry_mode")
    require(value["type"] in ("blob", "tree", "commit"), "entry_type")
    oid(value["oid"])


def entry(repo, revision, name):
    raw = git(repo, "ls-tree", revision, "--", ":(literal)" + name).decode().rstrip("\n")
    if not raw:
        return None
    fields, actual_path = raw.split("\t")
    require(actual_path == name, "tree_path")
    mode, kind, blob = fields.split()
    return {"mode": mode, "type": kind, "oid": blob}


def unique_pairs(items):
    result = {}
    for key, value in items:
        require(key not in result, "duplicate_record_key")
        result[key] = value
    return result


def rows(value, keys):
    require(isinstance(value, list) and value, "empty_record_set")
    names = []
    for row in value:
        shape(row, keys)
        names.append(path(row["path"]))
        for key in set(keys) - {"path"}:
            entry_shape(row[key])
    require(names == sorted(set(names)), "record_order_or_duplicate")
    return names


def qualify(repo, base, target, selected_path, correction, owner_map, event, ref):
    require(event == "push" and ref == "refs/heads/develop", "event")
    oid(PINNED_RECORD_COMMIT)
    require(re.fullmatch(r"[0-9a-f]{64}", PINNED_RECORD_SHA256), "record_pin")
    raw = git(repo, "show", PINNED_RECORD_COMMIT + ":" + RECORD_PATH)
    require(hashlib.sha256(raw).hexdigest() == PINNED_RECORD_SHA256, "record_hash")
    record_entry = entry(repo, PINNED_RECORD_COMMIT, RECORD_PATH)
    require(record_entry is not None and record_entry["mode"] == "100644" and record_entry["type"] == "blob", "record_entry")
    record = json.loads(raw, object_pairs_hook=unique_pairs)
    shape(record, ("version", "prior_owner", "prior_source", "content_base", "prior_runtime_trigger", "correction", "donor", "review", "corrections", "prerequisites"))
    require(record["version"] == "GUARDED-CORRECTION-V1", "record_version")
    require(isinstance(record["review"], str) and record["review"].startswith(("https://", "http://")), "review_evidence")
    owner, source, recorded_c, donor = [oid(record[k]) for k in ("prior_owner", "prior_source", "correction", "donor")]
    oid(record["content_base"])
    require(record["prior_runtime_trigger"] in ("true", "false"), "record_runtime_trigger")
    require(oid(correction) == recorded_c, "correction_identity")
    selected_path = path(selected_path)
    changed_paths = rows(record["corrections"], ("path", "before", "after"))
    prerequisites = rows(record["prerequisites"], ("path", "entry"))
    require(selected_path in changed_paths and not set(changed_paths) & set(prerequisites), "correction_scope")
    target = git(repo, "rev-parse", "--verify", target + "^{commit}").decode().strip()
    base = git(repo, "rev-parse", "--verify", base + "^{commit}").decode().strip()
    ancestor(repo, base, target)
    ancestor(repo, correction, PINNED_RECORD_COMMIT)
    require(correction != PINNED_RECORD_COMMIT, "self_record")
    ancestor(repo, PINNED_RECORD_COMMIT, target)
    chain = git(repo, "rev-list", "--first-parent", base + ".." + target).decode().splitlines()
    require(correction in chain, "correction_first_parent")
    parents = git(repo, "rev-list", "--parents", "-n", "1", correction).decode().split()
    require(parents == [correction, owner], "correction_parent")
    owner_parents = git(repo, "rev-list", "--parents", "-n", "1", owner).decode().split()
    require(len(owner_parents) >= 3 and source in owner_parents[2:], "imported_source")
    ancestor(repo, source, owner)
    selected = [line.split("\t") for line in Path(owner_map).read_text().splitlines()
                if line.split("\t")[0] == selected_path]
    require(len(selected) == 1 and len(selected[0]) == 8, "prior_row_count_or_shape")
    prior = selected[0]
    require(prior[1] in ("true", "false") and prior[2] == owner and prior[3] == owner_parents[1] and prior[5] == source, "prior_row_identity")
    require(prior[6] == record["content_base"] and prior[1] == record["prior_runtime_trigger"], "prior_content_or_runtime_taint")
    bases = git(repo, "merge-base", "--all", owner_parents[1], source).decode().splitlines()
    require(bases == [prior[4]], "prior_source_base")
    ancestor(repo, prior[6], owner)
    delta = git(repo, "diff-tree", "--no-commit-id", "--name-status", "--no-renames", "-r", "-z", owner, correction).decode().split("\0")
    require(delta[-1] == "", "delta_encoding")
    delta = delta[:-1]
    require(len(delta) % 2 == 0, "delta_encoding")
    actual_delta = list(zip(delta[::2], delta[1::2]))
    expected_delta = []
    for row in record["corrections"]:
        name, before, after = row["path"], row["before"], row["after"]
        require(before != after, "empty_correction")
        require(entry(repo, owner, name) == before and entry(repo, source, name) == before, "before_entry")
        require(entry(repo, correction, name) == after and entry(repo, donor, name) == after and entry(repo, target, name) == after, "after_or_donor_or_target_entry")
        expected_delta.append(("A" if before is None else "D" if after is None else "M", name))
        if name == selected_path:
            require(before is not None and after is not None, "replacement_entry_required")
    require(actual_delta == expected_delta, "complete_correction_delta")
    for row in record["prerequisites"]:
        require(row["entry"] is not None, "missing_prerequisite")
        require(all(entry(repo, revision, row["path"]) == row["entry"] for revision in (owner, correction, donor, target)), "prerequisite_drift")
    return {"version": record["version"], "record": PINNED_RECORD_COMMIT,
            "record_sha256": PINNED_RECORD_SHA256, "path": selected_path,
            "prior_owner": owner, "prior_source": source, "correction": correction,
            "source_base": owner, "source_head": correction,
            "content_base": prior[6], "prior_runtime_trigger": prior[1],
            "retain_guarded_taint": True, "review": record["review"]}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("repo", "base", "target", "path", "correction", "owner-map", "event", "ref"):
        parser.add_argument("--" + name, required=True)
    args = parser.parse_args()
    try:
        receipt = qualify(args.repo, args.base, args.target, args.path, args.correction,
                          args.owner_map, args.event, args.ref)
    except (Rejected, OSError, ValueError, TypeError, KeyError):
        print("F13 guarded correction qualification rejected", file=sys.stderr)
        return 1
    print(json.dumps(receipt, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
