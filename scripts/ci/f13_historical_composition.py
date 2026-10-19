"""Fixed complete historical source composition; never runtime/path authority."""
import argparse
import copy
import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import subprocess
import sys

PINNED_RECORD_COMMIT = "ee312e653589ddf76103d55cd2036b9dcc6107c4"
PINNED_RECORD_SHA256 = "fd858cc753de4d86fac572f8972737cc7f451d5f06548d40a47656e68645b92f"
PINNED_SOURCE = "5bf6c4857c19fba2b91fb44eb9e9bc6ae816c485"
PINNED_BASE = "698d4f064f032ea74abf3ede2017100b26c8ee69"
RECORD_PATH = "docs/testing/master-historical-composition-v1.json"
VERIFIER_PATH = "scripts/ci/f13_historical_composition.py"


class Rejected(ValueError):
    pass


def require(value, reason):
    if not value:
        raise Rejected(reason)


def sha(data):
    return hashlib.sha256(data).hexdigest()


def oid(value):
    require(isinstance(value, str) and re.fullmatch(r'[0-9a-f]{40}', value) and value != '0' * 40, 'object_id')
    return value


def path(value):
    require(isinstance(value, str) and not re.search(r'[\s\x00-\x1f:\\]', value), 'literal_path')
    p = PurePosixPath(value)
    require(str(p) == value and not p.is_absolute() and '..' not in p.parts, 'literal_path')
    return value


def command(repo, *args):
    return subprocess.run(['git', '--no-replace-objects', '-C', str(repo), *args], capture_output=True)


def git(repo, *args):
    r = command(repo, *args)
    require(r.returncode == 0, 'git_' + args[0])
    return r.stdout


def pairs(items):
    result = {}
    for key, value in items:
        require(key not in result, 'duplicate_record_key')
        result[key] = value
    return result


def entry(repo, rev, name):
    data = git(repo, 'ls-tree', '-z', rev, '--', ':(literal)' + name)
    if not data:
        return None
    fields = data.split(b'\0')
    require(len(fields) == 2 and fields[-1] == b'', 'single_entry')
    meta, actual = fields[0].decode().split('\t', 1)
    require(actual == name, 'actual_path')
    mode, kind, value = meta.split()
    return dict(mode=mode, type=kind, oid=value)


def same_entry(repo, revision, name, expected):
    if expected is not None:
        require(isinstance(expected, dict) and set(expected) == {'mode', 'type', 'oid'}, 'entry_shape')
        require(expected['mode'] == '100644' and expected['type'] == 'blob', 'entry_mode_type')
        oid(expected['oid'])
    require(entry(repo, revision, name) == expected, 'actual_entry')


def load_unit(repo, target):
    for value in (target, PINNED_RECORD_COMMIT, PINNED_SOURCE, PINNED_BASE):
        oid(value)
    raw = git(repo, 'show', PINNED_RECORD_COMMIT + ':' + RECORD_PATH)
    require(sha(raw) == PINNED_RECORD_SHA256, 'record_hash')
    record_entry = entry(repo, PINNED_RECORD_COMMIT, RECORD_PATH)
    require(record_entry and record_entry['mode'] == '100644' and record_entry['type'] == 'blob', 'record_entry')
    r = json.loads(raw, object_pairs_hook=pairs)
    require(isinstance(r, dict) and set(r) == {'version', 'at', 'source', 'base', 'criterion', 'qualification',
        'runtime_pass', 'prior_records', 'rows', 'runtime_evidence_policy', 'proposal_sha256', 'source_qualification',
        'qualification_kind', 'activation_authority', 'current_or_future_path_authority', 'consumption_contract',
        'runtime_coverage', 'artifacts'}, 'record_shape')
    require(r['version'] == 'MASTER-HISTORICAL-COMPOSITION-v1'
            and r['qualification_kind'] == 'historical_source_composition_only'
            and r['qualification'] is True, 'typed_source_qualification')
    require(r['runtime_pass'] is False and r['activation_authority'] is False
            and r['current_or_future_path_authority'] is False, 'source_only_authority')
    require(r['source'] == PINNED_SOURCE and r['base'] == PINNED_BASE, 'source_boundary')
    require(r['source_qualification']['status'] == 'SOURCE_CLEAR'
            and len(r['source_qualification']['reviews']) == 2
            and all(v['comment'].startswith('c') and v['reviewer'] and v['record']
                    for v in r['source_qualification']['reviews']), 'independent_source_reviews')
    require(r['consumption_contract']['kind'] == 'typed_complete_historical_unit'
            and r['runtime_coverage']['status'] == 'NOT_GRANTED'
            and r['runtime_coverage']['source'] == PINNED_SOURCE, 'no_execution_authority')
    for older, newer in ((PINNED_BASE, PINNED_SOURCE), (PINNED_SOURCE, PINNED_RECORD_COMMIT),
                         (PINNED_RECORD_COMMIT, target)):
        git(repo, 'merge-base', '--is-ancestor', older, newer)
    require(target != PINNED_RECORD_COMMIT, 'later_carrier')
    require(git(repo, 'show', target + ':' + VERIFIER_PATH) == Path(__file__).read_bytes(), 'executed_carrier')
    require(git(repo, 'show', target + ':' + RECORD_PATH) == raw, 'record_drift')
    require(isinstance(r['rows'], list) and len(r['rows']) == 8, 'complete_eight_merges')
    artifacts = {}
    require(isinstance(r['artifacts'], list) and len(r['artifacts']) == 243, 'complete_artifacts')
    for proof in r['artifacts']:
        require(isinstance(proof, dict) and set(proof) == {'name', 'sha256', 'bytes'}, 'artifact_shape')
        name = path(proof['name'])
        require(name not in artifacts and re.fullmatch(r'[0-9a-f]{64}', proof['sha256'])
                and type(proof['bytes']) is int and proof['bytes'] >= 0, 'artifact_binding')
        artifacts[name] = proof
    checked = set()

    def bind(name, data):
        require(name in artifacts and name not in checked, 'missing_or_duplicate_artifact')
        proof = artifacts[name]
        require(proof['sha256'] == sha(data) and proof['bytes'] == len(data), 'artifact_bytes')
        checked.add(name)

    merges, occurrences = set(), {}
    for row in r['rows']:
        require(isinstance(row, dict) and set(row) == {'merge', 'parents', 'best_bases',
                'whole_conflict_paths', 'guarded_resolutions'}, 'merge_shape')
        merge = oid(row['merge'])
        require(merge not in merges, 'duplicate_merge')
        merges.add(merge)
        parents = row['parents']
        require(isinstance(parents, list) and len(parents) == 2, 'two_parents')
        first, incoming = map(oid, parents)
        require(git(repo, 'rev-list', '--parents', '-n', '1', merge).decode().split() == [merge, *parents], 'actual_parents')
        bases = git(repo, 'merge-base', '--all', first, incoming).decode().splitlines()
        require(len(bases) == 1 and bases == row['best_bases'], 'actual_best_base')
        base = oid(bases[0])
        git(repo, 'merge-base', '--is-ancestor', merge, PINNED_SOURCE)
        prefix = 'historical-composition/' + merge[:12] + '/'
        full_mt = command(repo, 'merge-tree', '--write-tree', first, incoming)
        require(full_mt.returncode == 1, 'actual_historical_conflict')
        bind(prefix + 'merge-tree.stdout', full_mt.stdout)
        bind(prefix + 'merge-tree.stderr', full_mt.stderr)
        mt = command(repo, 'merge-tree', '--write-tree', '--name-only', '-z', '--no-messages', first, incoming)
        chunks = mt.stdout.split(b'\0')
        require(mt.returncode == 1 and len(chunks) >= 3 and chunks[-1] == b''
                and re.fullmatch(rb'[0-9a-f]{40}', chunks[0]), 'merge_tree_output')
        actual_conflicts = [path(v.decode()) for v in chunks[1:-1]]
        require(sorted(set(actual_conflicts)) == row['whole_conflict_paths']
                and len(actual_conflicts) == len(set(actual_conflicts)), 'complete_whole_conflict_set')
        require(isinstance(row['guarded_resolutions'], list) and row['guarded_resolutions'], 'reviewed_occurrences')
        for item in row['guarded_resolutions']:
            require(isinstance(item, dict) and set(item) == {'path', 'category', 'entries', 'files',
                    'current_h3_entry', 'test_only', 'review'}, 'occurrence_shape')
            name = path(item['path'])
            key = (merge, name)
            require(key not in occurrences, 'duplicate_occurrence')
            require(item['review'].startswith('SOURCE_CLEAR:')
                    and item['test_only'] is name.endswith('_test.go'), 'source_review')
            require(item['category'] == ('GUARDED_ACTUAL_CONFLICT_RESOLUTION' if name in actual_conflicts
                    else 'GUARDED_RESULT_DIFFERS_DETERMINISTIC_MERGE'), 'resolution_category')
            require(set(item['entries']) == {'base', 'first', 'incoming', 'result'}, 'four_entries')
            present = set()
            for label, rev in (('base', base), ('first', first), ('incoming', incoming), ('result', merge)):
                expected = item['entries'][label]
                same_entry(repo, rev, name, expected)
                if expected is not None:
                    present.add(label)
                    data = git(repo, 'cat-file', 'blob', expected['oid'])
                    proof = item['files'][label]
                    artifact_name = prefix + label + '__' + name.replace('/', '__')
                    require(set(proof) == {'name', 'sha256'} and proof['name'] == artifact_name
                            and proof['sha256'] == sha(data), 'raw_file_binding')
                    bind(artifact_name, data)
            require(set(item['files']) == present, 'complete_raw_files')
            same_entry(repo, PINNED_SOURCE, name, item['current_h3_entry'])
            for label, rev in (('first-result', first), ('incoming-result', incoming)):
                bind(prefix + label + '__' + name.replace('/', '__') + '.diff',
                     git(repo, 'diff', '--no-ext-diff', rev, merge, '--', ':(literal)' + name))
            occurrences[key] = (first, base, incoming)
    require(len(occurrences) == 38, 'complete_thirty_eight_occurrences')
    proposal = copy.deepcopy(r)
    for key in ('proposal_sha256', 'source_qualification', 'qualification_kind', 'activation_authority',
                'current_or_future_path_authority', 'consumption_contract', 'runtime_coverage', 'artifacts'):
        proposal.pop(key)
    proposal['version'] = 'MASTER-HISTORICAL-COMPOSITION-PROPOSED-v1'
    proposal['qualification'] = False
    for row in proposal['rows']:
        for item in row['guarded_resolutions']:
            item['review'] = 'PENDING'
    proposal_raw = (json.dumps(proposal, indent=2) + '\n').encode()
    require(sha(proposal_raw) == r['proposal_sha256'], 'reviewed_proposal_hash')
    bind('historical-composition/composition-PROPOSAL.json', proposal_raw)
    require(checked == set(artifacts), 'complete_artifact_hash_binding')
    return occurrences


def qualify(repo, target, merge, first, source_base, source, name):
    for value in (merge, first, source_base, source):
        oid(value)
    unit = load_unit(repo, target)
    require(unit.get((merge, name)) == (first, source_base, source), 'exact_historical_call')
    return dict(version='MASTER-HISTORICAL-COMPOSITION-v1', record=PINNED_RECORD_COMMIT,
                record_sha256=PINNED_RECORD_SHA256, source_boundary=PINNED_SOURCE,
                merge=merge, first=first, source_base=source_base, source=source, path=name,
                merges=8, occurrences=38, artifact_bindings=243, runtime_pass=False,
                activation_authority=False, current_or_future_path_authority=False,
                retain_runtime_and_content_taint=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for key in ('repo', 'target', 'merge', 'first', 'source-base', 'source', 'path'):
        parser.add_argument('--' + key, required=True)
    args = parser.parse_args()
    try:
        print(json.dumps(qualify(args.repo, args.target, args.merge, args.first,
                                 args.source_base, args.source, args.path), sort_keys=True))
    except (Rejected, ValueError, KeyError, TypeError, AttributeError, OSError) as exc:
        print('Historical composition rejected: ' + str(exc), file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
