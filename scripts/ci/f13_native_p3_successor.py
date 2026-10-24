"""Exact reviewed successor record and source qualification.

The historical v1 verifier remains immutable. This later carrier pins the
reviewed record commit and bytes; history and original routing remain mandatory.
"""
import json
from pathlib import Path
import sys

sys.path.insert(0, str(Path(__file__).resolve().parent))
import f13_native_p3_source as legacy


VERSION = "NATIVE-P3-SOURCE-v2"
RECORD_PATH = "docs/testing/native-p3-source-v2.json"
PINNED_RECORD_COMMIT = '3ef197bc0fd6bde17a538581df4e4fe99a1765d4'
PINNED_RECORD_SHA256 = 'edda541ab45be2ff17f74d626c657a7611274d0149c2bca808092b5a9bfe4f8b'
VERIFIER_PATH = 'scripts/ci/f13_native_p3_successor.py'
CARRIER_PATHS = (VERIFIER_PATH, 'scripts/ci/f13_native_p3_authority.py',
                 legacy.VERIFIER_PATH, legacy.SHELL_PATH, 'scripts/ci/f13_f54_history.py')
FIELDS = {"version", "base", "source", "classification", "consumers", "predecessor",
          "projection", "context", "compiled_closure", "delta_review", "evidence_matrix",
          "evidence", "excluded_authority", "review_status", "promoted_context",
          "context_only_changed_generated", "compiled_closure_manifest"}


def validate_structure(record, previous, closure_manifest):
    """Validate typed completeness without treating a draft as permission."""
    require = legacy.require
    require(isinstance(record, dict) and set(record) == FIELDS, "successor_shape")
    require(record["version"] == VERSION, "successor_version")
    require(record["base"] == previous["base"], "successor_base")
    legacy.oid(record["source"])
    require(record["source"] != previous["source"], "successor_source")
    require(record["classification"] == previous["classification"]
            and record["consumers"] == previous["consumers"], "successor_authority")
    require(record["predecessor"] == {
        "version": "NATIVE-P3-SOURCE-v1", "source": legacy.PINNED_SOURCE,
        "record": legacy.PINNED_RECORD_COMMIT, "sha256": legacy.PINNED_RECORD_SHA256,
        "path": legacy.RECORD_PATH,
    }, "successor_lineage")
    projection = set(legacy.rows(record["projection"], ("path", "before", "after")))
    context = set(legacy.rows(record["context"], ("path", "entry")))
    require(not projection & context, "successor_overlapping_sets")
    old_projection = {row["path"] for row in previous["projection"]}
    old_context = {row["path"] for row in previous["context"]}
    require(old_projection <= projection, "successor_projection_dropped")
    promoted = record["promoted_context"]
    require(isinstance(promoted, list) and promoted == sorted(set(promoted))
            and set(promoted) == old_context & projection, "successor_promoted_context")
    require(all(name.startswith('src/backend/voice/') and '/pb/' not in name for name in promoted),
            "successor_promoted_scope")
    require(old_context <= context | set(promoted), "successor_context_dropped")
    generated = record["context_only_changed_generated"]
    require(isinstance(generated, list) and generated == sorted(set(generated))
            and set(generated) <= context
            and all(name.startswith('src/backend/voice/pb/') and name.endswith('.pb.go') for name in generated),
            "successor_generated_context")
    # Expansion has explicit row-level review, never a broad runtime/path waiver.
    all_paths = projection | context
    closure = record["compiled_closure"]
    require(isinstance(closure, list) and closure and closure == sorted(set(closure))
            and set(closure) <= all_paths, "successor_compiled_closure")
    manifest_ref = record['compiled_closure_manifest']
    require(isinstance(manifest_ref, dict) and set(manifest_ref) == {'path', 'sha256'}
            and manifest_ref['path'] == 'docs/testing/native-p3-source-v2-closure.json'
            and isinstance(manifest_ref['sha256'], str)
            and legacy.re.fullmatch(r'[0-9a-f]{64}', manifest_ref['sha256']), 'successor_closure_reference')
    require(isinstance(closure_manifest, dict) and closure_manifest.get('source') == record['source']
            and closure_manifest.get('paths') == closure, 'successor_compiled_closure_omitted')
    for name in closure:
        legacy.path(name)
    reviews = record["delta_review"]
    require(isinstance(reviews, list), "successor_delta_review")
    names = []
    for row in reviews:
        require(isinstance(row, dict) and set(row) == {"path", "kind", "basis", "status"},
                "successor_delta_review_shape")
        names.append(legacy.path(row["path"]))
        require(row["kind"] in ("projection", "context") and isinstance(row['basis'], list)
                and all(isinstance(ref, str) and ref for ref in row['basis'])
                and row["status"] in ("OPEN", "ACCEPT"), "successor_delta_review_value")
        require(row["path"] in (projection if row["kind"] == "projection" else context),
                "successor_delta_review_scope")
    require(names == sorted(set(names)), "successor_delta_review_order")
    old_entries = {row["path"]: row["after"] for row in previous["projection"]}
    old_entries.update({row["path"]: row["entry"] for row in previous["context"]})
    current = {row["path"]: row["after"] for row in record["projection"]}
    current.update({row["path"]: row["entry"] for row in record["context"]})
    changed = {name for name, value in current.items() if old_entries.get(name) != value} | set(promoted)
    require(set(names) == changed, "successor_delta_review_incomplete")
    require({name for name in changed if name.startswith('src/backend/voice/')}
            <= projection | set(generated), "successor_voice_change_hidden_in_context")
    matrix = record["evidence_matrix"]
    require(isinstance(matrix, dict) and set(matrix) == set(legacy.EVIDENCE_CASES),
            "successor_matrix")
    for case, proof in matrix.items():
        require(isinstance(proof, dict) and set(proof) == {"result", "historical", "current", "review"},
                "successor_case_shape")
        require(proof["result"] in ("OPEN", "PASS")
                and proof['historical'] == previous['evidence_matrix'][case]
                and isinstance(proof['current'], list) and isinstance(proof['review'], list)
                and all(isinstance(ref, str) and ref for ref in proof['current'] + proof['review']),
                "successor_case_evidence_" + case)
    require(record["excluded_authority"] == previous["excluded_authority"],
            "successor_excluded_authority")
    require(record["review_status"] in ("OPEN", "ACCEPT"), "successor_review_status")
    require(isinstance(record["evidence"], list) and record["evidence"], "successor_evidence")
    evidence_names = []
    for item in record["evidence"]:
        require(isinstance(item, dict) and set(item) == {"name", "sha256"}, "successor_evidence_shape")
        evidence_names.append(legacy.path(item["name"]))
        require(isinstance(item["sha256"], str) and legacy.re.fullmatch(r"[0-9a-f]{64}", item["sha256"]),
                "successor_evidence_hash")
    require(evidence_names == sorted(set(evidence_names)), "successor_evidence_order")
    references = [ref for row in reviews for ref in row['basis']]
    references += [ref for proof in matrix.values() for ref in proof['current'] + proof['review']]
    require(set(references) <= set(evidence_names), 'successor_unknown_evidence_reference')
    return record


def require_accepted(record):
    legacy.require(record["review_status"] == "ACCEPT", "successor_review_open")
    for row in record["delta_review"]:
        legacy.require(row["status"] == "ACCEPT" and row['basis'], "successor_delta_open")
    for case, proof in record["evidence_matrix"].items():
        legacy.require(proof["result"] == "PASS" and proof["current"] and proof["review"],
                       "successor_evidence_open_" + case)


def load_record(repo):
    # An author draft, even with edited PASS strings, cannot select its own pin.
    legacy.require(PINNED_RECORD_COMMIT is not None and PINNED_RECORD_SHA256 is not None,
                   "successor_carrier_not_pinned")
    legacy.oid(PINNED_RECORD_COMMIT)
    legacy.require(isinstance(PINNED_RECORD_SHA256, str)
                   and legacy.re.fullmatch(r'[0-9a-f]{64}', PINNED_RECORD_SHA256), 'successor_record_pin')
    raw = legacy.git(repo, "show", PINNED_RECORD_COMMIT + ":" + RECORD_PATH)
    legacy.require(legacy.digest(raw) == PINNED_RECORD_SHA256, "successor_record_hash")
    legacy.require(legacy.entry(repo, PINNED_RECORD_COMMIT, RECORD_PATH)['mode'] == '100644',
                   'successor_record_mode')
    previous = legacy.load_record(repo)
    record = json.loads(raw, object_pairs_hook=legacy.unique_pairs)
    manifest_ref = record['compiled_closure_manifest']
    manifest_raw = legacy.git(repo, 'show', PINNED_RECORD_COMMIT + ':' + manifest_ref['path'])
    legacy.require(legacy.digest(manifest_raw) == manifest_ref['sha256'], 'successor_closure_hash')
    record = validate_structure(record, previous, json.loads(manifest_raw, object_pairs_hook=legacy.unique_pairs))
    require_accepted(record)
    for proof in record['evidence']:
        proof_raw = legacy.git(repo, 'show', PINNED_RECORD_COMMIT + ':' + proof['name'])
        legacy.require(legacy.digest(proof_raw) == proof['sha256'], 'successor_evidence_bytes')
    legacy.ancestor(repo, legacy.PINNED_RECORD_COMMIT, record["source"])
    legacy.ancestor(repo, record["source"], PINNED_RECORD_COMMIT)
    legacy.require(record["source"] != PINNED_RECORD_COMMIT, "successor_self_record")
    for row in record["projection"]:
        legacy.require(row["before"] != row["after"]
                       and legacy.entry(repo, record["base"], row["path"]) == row["before"]
                       and legacy.entry(repo, record["source"], row["path"]) == row["after"],
                       "successor_projection_entry")
    for row in record["context"]:
        legacy.require(row["entry"] is not None
                       and legacy.entry(repo, record["source"], row["path"]) == row["entry"],
                       "successor_context_entry")
    represented = {row["path"] for kind in ("projection", "context") for row in record[kind]}
    voice = {name for name in legacy.tree(str(repo), record["source"])
             if name.startswith("src/backend/voice/")}
    legacy.require(voice <= represented, "successor_voice_context_incomplete")
    changed = set(legacy.git(repo, 'diff', '--name-only', '--no-renames', record['base'],
                             record['source']).decode().splitlines())
    projection = {row['path'] for row in record['projection']}
    legacy.require({name for name in changed if name.startswith('src/backend/voice/')}
                   <= projection | set(record['context_only_changed_generated']),
                   'successor_voice_projection_incomplete')
    return record


def qualify(repo, base, source, target, event, ref, event_sha, before, base_ref):
    """Same event/result boundary as v1, then the entire reviewed successor unit."""
    repo = str(Path(repo).resolve())
    base, source, target = legacy.oid(base), legacy.oid(source), legacy.oid(target)
    require = legacy.require
    require(event_sha == target, 'event_target')
    require(event in ('push', 'pull_request'), 'event')
    if event == 'push' and ref == 'refs/heads/develop':
        require(source == target and before == base, 'develop_event_before')
        consumer = 'DEVELOP-SOURCE-v1'
    else:
        require((event == 'push' and ref == 'refs/heads/master' and before == base)
                or (event == 'pull_request' and base_ref == 'master'
                    and legacy.re.fullmatch(r'refs/pull/[0-9]+/merge', ref)), 'master_event')
        parents = legacy.git(repo, 'rev-list', '--parents', '-n', '1', target).decode().split()
        require(parents == [target, base, source], 'master_shape_or_before')
        require(legacy.git(repo, 'diff', '--raw', '--no-abbrev', '--no-renames', base, source)
                == legacy.git(repo, 'diff', '--raw', '--no-abbrev', '--no-renames', base, target),
                'master_result')
        consumer = 'MASTER-SOURCE-v1'
    require(base != '0' * 40, 'event_before_zero')
    legacy.ancestor(repo, base, source)
    record = load_record(repo)
    legacy.ancestor(repo, PINNED_RECORD_COMMIT, source)
    require(source != PINNED_RECORD_COMMIT, 'missing_carrier')
    root = Path(__file__).resolve().parents[2]
    for name in CARRIER_PATHS:
        require(legacy.entry(repo, source, name)['mode'] == '100644'
                and legacy.entry(repo, source, name)['type'] == 'blob', 'successor_carrier_mode')
        require(legacy.git(repo, 'show', source + ':' + name) == (root / name).read_bytes(),
                'successor_carrier_' + name)
        require(legacy.entry(repo, target, name) == legacy.entry(repo, source, name),
                'successor_carrier_result')
    require(legacy.entry(repo, PINNED_RECORD_COMMIT, VERIFIER_PATH)
            != legacy.entry(repo, source, VERIFIER_PATH), 'later_carrier')
    expected_voice = {name for name in legacy.tree(repo, record['source'])
                      if name.startswith('src/backend/voice/')}
    for revision in (source, target):
        require({name for name in legacy.tree(repo, revision) if name.startswith('src/backend/voice/')}
                == expected_voice, 'voice_tree_added_or_removed')
        for row in record['projection']:
            require(legacy.entry(repo, revision, row['path']) == row['after'], 'projection_drift')
        for row in record['context']:
            require(legacy.entry(repo, revision, row['path']) == row['entry'], 'context_drift')
        for name in (RECORD_PATH, record['compiled_closure_manifest']['path'], legacy.RECORD_PATH):
            require(legacy.entry(repo, revision, name) == legacy.entry(repo, PINNED_RECORD_COMMIT, name),
                    'successor_record_drift')
        for proof in record['evidence']:
            require(legacy.entry(repo, revision, proof['name'])
                    == legacy.entry(repo, PINNED_RECORD_COMMIT, proof['name']), 'successor_evidence_drift')
    return dict(version=VERSION, consumer=consumer, base=base, source=source, target=target,
                content_base=record['base'], snapshot=record['source'], record=PINNED_RECORD_COMMIT,
                record_sha256=PINNED_RECORD_SHA256, paths=[row['path'] for row in record['projection']],
                retain_guarded_and_runtime_history=True)


def audit_map(repo, binding, map_path, history_path):
    """Validate real collector rows; never synthesize or edit the map."""
    require, git, oid = legacy.require, legacy.git, legacy.oid
    ancestor, entry, digest = legacy.ancestor, legacy.entry, legacy.digest
    subprocess, re, runtime_changed = legacy.subprocess, legacy.re, legacy.runtime_changed
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
        ancestor(repo, source_head, r['source'])
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
