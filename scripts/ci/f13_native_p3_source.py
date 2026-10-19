"""Fixed complete dormant/restrictive native source qualification.

Source authority only; no installed rights, A3/R22 activation or environment
selected record. The shell producer retains real Git rows and runtime history.
"""
import argparse
from functools import lru_cache
import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import subprocess
import sys

PINNED_BASE = "8191a36ca3bea7d053df0dd25963ca5133284b41"
PINNED_SOURCE = "5bf6c4857c19fba2b91fb44eb9e9bc6ae816c485"
PINNED_RECORD_COMMIT = "ab98f78239b1777a45b65f8755b5f6f3ac579094"
PINNED_RECORD_SHA256 = "55eed972bedaf87515a7a05657d97c357790db0fe54c311ef3896cc30589a417"
RECORD_PATH = "docs/testing/native-p3-source-v1.json"
VERIFIER_PATH = "scripts/ci/f13_native_p3_source.py"
SHELL_PATH = "scripts/ci/voice-db-runtime-provisioning-contract_test.sh"
EVIDENCE_CASES = tuple("A1 A2 A3 A4 B1 B2 B3 B4".split())


class Rejected(ValueError):
    pass


def require(value, reason):
    if not value:
        raise Rejected(reason)


def digest(data):
    return hashlib.sha256(data).hexdigest()


def oid(value):
    require(isinstance(value, str) and re.fullmatch(r"[0-9a-f]{40}", value), "object_id")
    return value


def path(value):
    require(isinstance(value, str) and value and not re.search(r"[\s\x00-\x1f:]", value), "path")
    p = PurePosixPath(value)
    require(not p.is_absolute() and str(p) == value and ".." not in p.parts, "path")
    return value


def unique_pairs(items):
    result = {}
    for key, value in items:
        require(key not in result, "duplicate_record_key")
        result[key] = value
    return result


def git(repo, *args):
    r = subprocess.run(["git", "--no-replace-objects", "-C", str(repo), *args], capture_output=True, check=False)
    require(r.returncode == 0, "git_" + args[0])
    return r.stdout


def ancestor(repo, older, newer):
    git(repo, "merge-base", "--is-ancestor", older, newer)


@lru_cache(maxsize=128)
def tree(repo, revision):
    result = {}
    for row in git(repo, "ls-tree", "-rz", revision).split(b"\0"):
        if row:
            meta, name = row.decode().split("\t", 1)
            mode, kind, value = meta.split()
            result[name] = {"mode": mode, "type": kind, "oid": value}
    return result


def entry(repo, revision, name):
    return tree(str(repo), revision).get(name)


def entry_shape(value):
    if value is None:
        return
    require(isinstance(value, dict) and set(value) == {"mode", "type", "oid"}, "entry_shape")
    require(value["mode"] in ("100644", "100755", "120000") and value["type"] == "blob", "entry_type")
    oid(value["oid"])


def rows(value, fields):
    require(isinstance(value, list) and value, "empty_record_set")
    names = []
    for row in value:
        require(isinstance(row, dict) and set(row) == set(fields), "row_shape")
        names.append(path(row["path"]))
        for key in set(fields) - {"path"}:
            entry_shape(row[key])
    require(names == sorted(set(names)), "record_order_or_duplicate")
    return names


def load_record(repo):
    oid(PINNED_BASE)
    oid(PINNED_SOURCE)
    oid(PINNED_RECORD_COMMIT)
    require(re.fullmatch(r"[0-9a-f]{64}", PINNED_RECORD_SHA256), "record_pin")
    raw = git(repo, "show", PINNED_RECORD_COMMIT + ":" + RECORD_PATH)
    require(digest(raw) == PINNED_RECORD_SHA256, "record_hash")
    require(entry(repo, PINNED_RECORD_COMMIT, RECORD_PATH)["mode"] == "100644", "record_mode")
    r = json.loads(raw, object_pairs_hook=unique_pairs)
    require(set(r) == {"version", "base", "source", "classification", "consumers", "projection", "context",
                       "context_only_changed_generated", "closure_review", "down_review", "source_proposal_sha256",
                       "evidence", "excluded_authority", "evidence_matrix"}, "record_shape")
    require(r["version"] == "NATIVE-P3-SOURCE-v1" and r["base"] == PINNED_BASE
            and r["source"] == PINNED_SOURCE and r["classification"] == "DORMANT-A3-RESTRICTIVE-P3-SOURCE", "record_identity")
    require(r["consumers"] == ["DEVELOP-SOURCE-v1", "MASTER-SOURCE-v1"], "consumer_types")
    for key in ("closure_review", "down_review"):
        require(isinstance(r[key], str) and r[key].startswith(("http://", "https://")), "review_reference")
    require(re.fullmatch(r"[0-9a-f]{64}", r["source_proposal_sha256"]), "proposal_hash")
    names = rows(r["projection"], ("path", "before", "after"))
    context = rows(r["context"], ("path", "entry"))
    require(r["context_only_changed_generated"] == sorted(set(r["context_only_changed_generated"])), "context_order")
    require(set(r["context_only_changed_generated"]) <= set(context) - set(names), "context_not_authority")
    require(isinstance(r["evidence_matrix"], dict) and set(r["evidence_matrix"]) == set(EVIDENCE_CASES), "evidence_matrix")
    for case, proof in r["evidence_matrix"].items():
        require(isinstance(proof, dict) and set(proof) == {"result", "evidence", "review"}, "evidence_shape")
        require(proof["result"] == "PASS" and proof["evidence"] and proof["review"], "evidence_open_" + case)
    require(isinstance(r["evidence"], list) and r["evidence"], "evidence")
    for proof in r["evidence"]:
        require(set(proof) == {"name", "sha256"} and re.fullmatch(r"[0-9a-f]{64}", proof["sha256"]), "evidence_hash")
    ancestor(repo, PINNED_BASE, PINNED_SOURCE)
    ancestor(repo, PINNED_SOURCE, PINNED_RECORD_COMMIT)
    require(PINNED_SOURCE != PINNED_RECORD_COMMIT, "self_record")
    for row in r["projection"]:
        require(row["before"] != row["after"] and entry(repo, PINNED_BASE, row["path"]) == row["before"]
                and entry(repo, PINNED_SOURCE, row["path"]) == row["after"], "projection_entry")
    for row in r["context"]:
        require(row["entry"] is not None and entry(repo, PINNED_SOURCE, row["path"]) == row["entry"], "context_entry")
    voice = {p for p in tree(str(repo), PINNED_SOURCE) if p.startswith("src/backend/voice/")}
    require(voice <= set(names) | set(context), "voice_context_incomplete")
    changed = set(git(repo, "diff", "--name-only", "--no-renames", PINNED_BASE, PINNED_SOURCE).decode().splitlines())
    require({p for p in changed if p.startswith("src/backend/voice/")} <= set(names) | set(r["context_only_changed_generated"]),
            "voice_projection_incomplete")
    return r


def qualify(repo, base, source, target, event, ref, event_sha, before, base_ref):
    repo = str(Path(repo).resolve())
    base, source, target = oid(base), oid(source), oid(target)
    require(event_sha == target, "event_target")
    require(event in ("push", "pull_request"), "event")
    if event == "push" and ref == "refs/heads/develop":
        require(source == target and before == base, "develop_event_before")
        consumer = "DEVELOP-SOURCE-v1"
    else:
        require((event == "push" and ref == "refs/heads/master" and before == base)
                or (event == "pull_request" and base_ref == "master" and re.fullmatch(r"refs/pull/[0-9]+/merge", ref)), "master_event")
        parents = git(repo, "rev-list", "--parents", "-n", "1", target).decode().split()
        require(parents == [target, base, source], "master_shape_or_before")
        require(git(repo, "diff", "--raw", "--no-abbrev", "--no-renames", base, source)
                == git(repo, "diff", "--raw", "--no-abbrev", "--no-renames", base, target), "master_result")
        consumer = "MASTER-SOURCE-v1"
    require(base != "0" * 40, "event_before_zero")
    ancestor(repo, base, source)
    r = load_record(repo)
    ancestor(repo, PINNED_RECORD_COMMIT, source)
    require(source != PINNED_RECORD_COMMIT, "missing_carrier")
    require(git(repo, "show", source + ":" + VERIFIER_PATH) == Path(__file__).read_bytes(), "verifier_carrier")
    require(git(repo, "show", source + ":" + SHELL_PATH)
            == Path(__file__).with_name(Path(SHELL_PATH).name).read_bytes(), "shell_carrier")
    require(entry(repo, PINNED_RECORD_COMMIT, VERIFIER_PATH) != entry(repo, source, VERIFIER_PATH), "later_carrier")
    voice_paths = {name for name in tree(str(repo), PINNED_SOURCE) if name.startswith("src/backend/voice/")}
    for revision in (source, target):
        require({name for name in tree(str(repo), revision) if name.startswith("src/backend/voice/")} == voice_paths,
                "voice_tree_added_or_removed")
    for row in r["projection"]:
        for revision in (source, target):
            require(entry(repo, revision, row["path"]) == row["after"], "projection_drift")
    for row in r["context"]:
        for revision in (source, target):
            require(entry(repo, revision, row["path"]) == row["entry"], "context_drift")
    require(entry(repo, source, RECORD_PATH) == entry(repo, PINNED_RECORD_COMMIT, RECORD_PATH), "record_drift")
    return {"version": r["version"], "consumer": consumer, "base": base, "source": source, "target": target,
            "content_base": PINNED_BASE, "snapshot": PINNED_SOURCE, "record": PINNED_RECORD_COMMIT,
            "record_sha256": PINNED_RECORD_SHA256, "paths": [row["path"] for row in r["projection"]],
            "retain_guarded_and_runtime_history": True}


def runtime_changed(paths):
    """Same runtime/build/storage lower bound as voice-r22-runtime-scope.sh."""
    for name in paths:
        if name.startswith("src/backend/voice/pb/") or re.fullmatch(r"src/backend/voice/[^/]+_test\.go", name):
            continue
        if name.startswith("src/backend/migrations/voice_db/") or name in (
                "src/backend/voice/go.mod", "src/backend/voice/go.sum", "src/backend/voice/Dockerfile"):
            return True
        if re.fullmatch(r"src/backend/voice/[^/]+\.go", name) or re.fullmatch(r"src/backend/voice/internal/roomlifecycle/[^/]+\.go", name):
            return True
    return False


def audit_map(repo, binding, map_path, history_path):
    """Validate real collector rows; never synthesize or edit the map."""
    r = load_record(repo)
    expected = {row["path"]: row["after"] for row in r["projection"]}
    historical = {}
    history = Path(history_path).read_text().splitlines()
    for line in history:
        fields = line.split("\t")
        if fields[0] == "supersession":
            require(len(fields) == 6 and fields[-1] in ("0", "2"), "history_supersession")
            continue
        require(len(fields) == 8 and fields[0] in expected and fields[1] in ("true", "false"), "history_row")
        historical.setdefault(fields[0], []).append(line)
    selected = {}
    for line in Path(map_path).read_text().splitlines():
        fields = line.split("\t")
        require(len(fields) == 8, "map_shape")
        name, trigger, owner, parent, source_base, source_head, content_base, best = fields
        if name not in expected:
            continue
        require(name not in selected and trigger in ("true", "false"), "map_duplicate_or_trigger")
        require(name in historical and historical[name][-1] == line, "map_history_binding")
        for value in (owner, parent, source_base, source_head, content_base):
            oid(value)
        owner_parents = git(repo, "rev-list", "--parents", "-n", "1", owner).decode().split()
        require(len(owner_parents) >= 2 and owner_parents[1] == parent, "map_owner_parent")
        ancestor(repo, owner, binding["source"])
        ancestor(repo, source_head, PINNED_SOURCE)
        ancestor(repo, source_head, owner)
        ancestor(repo, source_base, source_head)
        ancestor(repo, content_base, parent)
        require(all(re.fullmatch(r"[0-9a-f]{40}", v) for v in best.split(",")), "map_best_bases")
        recorded_bases = best.split(",")
        if len(owner_parents) == 2:
            require(source_head == owner and source_base == parent and recorded_bases == [parent], "map_direct_owner")
        else:
            require(any(sorted(git(repo, "merge-base", "--all", parent, actual_source).decode().splitlines())
                        == sorted(recorded_bases) and (source_head == owner or
                        subprocess.run(["git", "--no-replace-objects", "-C", str(repo), "merge-base", "--is-ancestor",
                                        source_head, actual_source], capture_output=True).returncode == 0)
                        for actual_source in owner_parents[2:]), "map_imported_source")
        require(entry(repo, source_head, name) == expected[name] == entry(repo, binding["target"], name), "map_entry")
        delta = git(repo, "diff", "--name-only", "--no-renames", source_base, source_head).decode().splitlines()
        require(name in delta, "map_source_delta")
        require(trigger == "true" or not runtime_changed(delta), "map_runtime_taint")
        require(trigger == "true" or all(row.split("\t")[1] == "false" for row in historical[name]), "map_prior_runtime_taint")
        selected[name] = line
    changed = set(git(repo, "diff", "--name-only", "--no-renames", binding["base"], binding["source"]).decode().splitlines())
    require(set(selected) == set(expected) & changed, "map_incomplete")
    return {"binding": binding, "map_sha256": digest(Path(map_path).read_bytes()),
            "history_sha256": digest(Path(history_path).read_bytes()), "rows": selected}


def main():
    p = argparse.ArgumentParser(description=__doc__)
    for key in ("repo", "base", "source", "target", "event", "ref", "event-sha", "before", "base-ref"):
        p.add_argument("--" + key, required=True)
    p.add_argument("--output", required=True)
    p.add_argument("--map")
    p.add_argument("--history")
    args = p.parse_args()
    try:
        binding = qualify(args.repo, args.base, args.source, args.target, args.event, args.ref, args.event_sha, args.before, args.base_ref)
        require(not args.map or args.history, "missing_history")
        result = audit_map(args.repo, binding, args.map, args.history) if args.map else binding
        Path(args.output).write_text(json.dumps(result, sort_keys=True) + "\n")
        if not args.map:
            print("\n".join(binding["paths"]))
    except (Rejected, ValueError, KeyError, TypeError, OSError) as exc:
        print("NATIVE-P3 source rejected: " + str(exc), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
