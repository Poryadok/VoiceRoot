"""Only two fixed historical Space test-source duplicates; no path grant."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys

PINNED_RECORD_COMMIT = "b04a556971250a11666848b414054fae8382e13b"
PINNED_RECORD_SHA256 = "310afcda1ecf7bef45c6cd8182c145f2e2eaa9349122bf9dff046c974df30871"
PINNED_SOURCE = "8fc9e542acd8039a356c4149ccd0fd4a16f79564"
PINNED_BASE = "698d4f064f032ea74abf3ede2017100b26c8ee69"
RECORD_PATH = "docs/testing/space-audit-test-duplicates-v1.json"
VERIFIER_PATH = "scripts/ci/f13_space_audit.py"


class Rejected(ValueError): pass


def require(value, reason):
    if not value: raise Rejected(reason)


def oid(value):
    require(isinstance(value, str) and re.fullmatch(r'[0-9a-f]{40}', value) and value != '0' * 40, 'object_id')
    return value


def git(repo, *args):
    p = subprocess.run(['git', '--no-replace-objects', '-C', str(repo), *args], capture_output=True)
    require(p.returncode == 0, 'git_' + args[0]); return p.stdout


def pairs(items):
    result = {}
    for key, value in items:
        require(key not in result, 'duplicate_record_key'); result[key] = value
    return result


def entry(repo, rev, name):
    data = git(repo, 'ls-tree', '-z', rev, '--', ':(literal)' + name).split(b'\0')
    require(len(data) == 2 and data[-1] == b'', 'single_entry')
    meta, actual = data[0].decode().split('\t', 1); require(actual == name, 'literal_path')
    mode, kind, value = meta.split(); return dict(mode=mode, type=kind, oid=value)


def actual_entry(repo, rev, name, expected):
    require(isinstance(expected, dict) and set(expected) == {'mode', 'type', 'oid'}, 'entry_shape')
    require(expected['mode'] == '100644' and expected['type'] == 'blob', 'entry_mode_type')
    oid(expected['oid']); require(entry(repo, rev, name) == expected, 'actual_entry')


def row(values):
    require(isinstance(values, list) and len(values) == 8 and values[1] in ('true', 'false'), 'row_shape')
    require(isinstance(values[0], str) and re.fullmatch(r'[A-Za-z0-9_./-]+_test\.go', values[0])
            and not values[0].startswith('/') and '..' not in values[0].split('/'), 'test_only_literal_path')
    for value in values[2:]: oid(value)
    return values


def load_unit(repo, target):
    for value in (target, PINNED_RECORD_COMMIT, PINNED_SOURCE, PINNED_BASE): oid(value)
    raw = git(repo, 'show', PINNED_RECORD_COMMIT + ':' + RECORD_PATH)
    require(hashlib.sha256(raw).hexdigest() == PINNED_RECORD_SHA256, 'record_hash')
    meta = entry(repo, PINNED_RECORD_COMMIT, RECORD_PATH)
    require(meta['mode'] == '100644' and meta['type'] == 'blob', 'record_entry')
    r = json.loads(raw, object_pairs_hook=pairs)
    require(isinstance(r, dict) and set(r) == {'version', 'status', 'source', 'base', 'cases', 'criterion',
        'runtime_pass', 'generic_path_authority', 'activation_authority', 'inventory_limitation', 'artifacts',
        'review', 'proposal_sha256', 'carrier_status'}, 'record_shape')
    require(r['version'] == 'MASTER-SPACE-AUDIT-TEST-DUPLICATES-v1'
            and r['status'] == 'SOURCE_CONSUMPTION_CONTRACT_CLEAR', 'typed_source_contract')
    require(r['source'] == PINNED_SOURCE and r['base'] == PINNED_BASE, 'source_boundary')
    require(all(r[k] is False for k in ('runtime_pass', 'generic_path_authority', 'activation_authority')), 'source_only_authority')
    require(r['review'].startswith(('https://', 'http://')) and r['inventory_limitation'], 'review_and_limitations')
    for older, newer in ((PINNED_BASE, PINNED_SOURCE), (PINNED_SOURCE, PINNED_RECORD_COMMIT), (PINNED_RECORD_COMMIT, target)):
        git(repo, 'merge-base', '--is-ancestor', older, newer)
    require(target != PINNED_RECORD_COMMIT, 'later_carrier')
    require(git(repo, 'show', target + ':' + VERIFIER_PATH) == Path(__file__).read_bytes(), 'executed_carrier')
    require(git(repo, 'show', target + ':' + RECORD_PATH) == raw, 'record_drift')
    require(isinstance(r['cases'], list) and len(r['cases']) == 2, 'complete_two_cases')
    cases, bindings = {}, set()
    for case in r['cases']:
        require(set(case) == {'path', 'merge', 'parents', 'best_bases', 'actual_prior_row', 'actual_incoming_row', 'entries'}, 'case_shape')
        prior, incoming = row(case['actual_prior_row']), row(case['actual_incoming_row'])
        name = case['path']; require(name == prior[0] == incoming[0] and name not in cases, 'unique_test_path')
        merge = oid(case['merge']); parents = case['parents']; bases = case['best_bases']
        require(isinstance(parents, list) and len(parents) == 2, 'two_parents')
        for value in parents: oid(value)
        require(git(repo, 'rev-list', '--parents', '-n', '1', merge).decode().split() == [merge, *parents], 'actual_parents')
        actual_bases = git(repo, 'merge-base', '--all', *parents).decode().splitlines()
        require(len(actual_bases) == 1 and bases == actual_bases, 'actual_best_base')
        git(repo, 'merge-base', '--is-ancestor', merge, PINNED_SOURCE)
        old_parents = git(repo, 'rev-list', '--parents', '-n', '1', prior[2]).decode().split()
        require(len(old_parents) == 3 and old_parents[1:] == [prior[3], prior[5]], 'prior_actual_parents')
        old_bases = git(repo, 'merge-base', '--all', prior[3], prior[5]).decode().splitlines()
        require(old_bases == [prior[4]] and prior[7] == prior[4] and prior[6] == prior[3], 'prior_source_and_content_base')
        require(prior[2] == parents[0] and incoming[2:] == [merge, parents[0], bases[0], parents[1], prior[6], bases[0]], 'actual_row_metadata')
        revisions = dict(base=bases[0], first=parents[0], incoming=parents[1], result=merge,
                         prior_owner=prior[2], prior_source=prior[5], current=PINNED_SOURCE)
        require(set(case['entries']) == set(revisions), 'seven_raw_bindings')
        for label, revision in revisions.items():
            proof = case['entries'][label]
            require(set(proof) == {'revision', 'entry', 'sha256', 'artifact', 'bytes'} and proof['revision'] == revision, 'binding_revision')
            actual_entry(repo, revision, name, proof['entry'])
            data = git(repo, 'cat-file', 'blob', proof['entry']['oid'])
            require(hashlib.sha256(data).hexdigest() == proof['sha256']
                    and type(proof['bytes']) is int and len(data) == proof['bytes'], 'raw_hash_and_length')
            artifact = name.replace('/', '__') + '__' + label
            require(proof['artifact'] == artifact and artifact not in bindings, 'artifact_binding')
            bindings.add(artifact)
        e = case['entries']
        require(e['base']['entry'] == e['first']['entry'] == e['prior_owner']['entry'] == e['prior_source']['entry']
                and e['incoming']['entry'] == e['result']['entry'] == e['current']['entry']
                and e['base']['entry'] != e['incoming']['entry'], 'reviewed_test_delta_not_identical_parents')
        actual_entry(repo, target, name, e['current']['entry'])
        cases[name] = case
    require(list(cases) == sorted(cases) and len(bindings) == 14, 'complete_fourteen_binding_unit')
    require(isinstance(r['artifacts'], dict) and len(r['artifacts']) == 5, 'diagnostic_metadata')
    for name, value in r['artifacts'].items():
        require(isinstance(name, str) and set(value) == {'sha256', 'bytes'}
                and re.fullmatch(r'[0-9a-f]{64}', value['sha256'])
                and type(value['bytes']) is int and value['bytes'] >= 0, 'diagnostic_binding')
    require(re.fullmatch(r'[0-9a-f]{64}', r['proposal_sha256']), 'proposal_binding')
    return cases, r['inventory_limitation']


def qualify(repo, target, prior_raw, incoming_raw):
    prior, incoming = row(prior_raw.split('\t')), row(incoming_raw.split('\t'))
    cases, limitation = load_unit(repo, target)
    case = cases.get(prior[0]); require(case is not None, 'historical_path')
    require(prior == case['actual_prior_row'] and incoming == case['actual_incoming_row'], 'exact_two_actual_rows_and_flags')
    return dict(version='MASTER-SPACE-AUDIT-TEST-DUPLICATES-v1', record=PINNED_RECORD_COMMIT,
                record_sha256=PINNED_RECORD_SHA256, source_boundary=PINNED_SOURCE, cases=2, raw_bindings=14,
                prior_row=prior, incoming_row=incoming, runtime_pass=False, activation_authority=False,
                generic_path_authority=False, retain_runtime_and_content_taint=True, inventory_limitation=limitation)


def main():
    p = argparse.ArgumentParser(description=__doc__)
    for key in ('repo', 'target', 'prior-row', 'incoming-row'): p.add_argument('--' + key, required=True)
    args = p.parse_args()
    try: print(json.dumps(qualify(args.repo, args.target, args.prior_row, args.incoming_row), sort_keys=True))
    except (Rejected, ValueError, KeyError, TypeError, AttributeError, OSError) as exc:
        print('Space audit source rejected: ' + str(exc), file=sys.stderr); return 1
    return 0


if __name__ == '__main__': sys.exit(main())
