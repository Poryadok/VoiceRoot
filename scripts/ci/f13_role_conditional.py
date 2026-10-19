"""One historical Role duplicate, conditional on the exact current correction."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys

PINNED_RECORD_COMMIT = "89d846ed6bee73fd21a6a0218b2041a2cf1f89c5"
PINNED_RECORD_SHA256 = "ba422695f9bfaea8257504fcb5bb64527c5e0af25aa7a4c2cf3a881f51eaacbd"
PINNED_SOURCE = "2ce88a9892989686cd4fd5734e8b414a3c914564"
PINNED_BASE = "698d4f064f032ea74abf3ede2017100b26c8ee69"
RECORD_PATH = "docs/testing/role-checkpermission-historical-source-v1.json"
VERIFIER_PATH = "scripts/ci/f13_role_conditional.py"
ROLE_PATH = "src/backend/role/internal/principalruntime/runtime.go"


class Rejected(ValueError):
    pass


def require(value, reason):
    if not value:
        raise Rejected(reason)


def oid(value):
    require(isinstance(value, str) and re.fullmatch(r'[0-9a-f]{40}', value) and value != '0' * 40, 'object_id')
    return value


def git(repo, *args):
    p = subprocess.run(['git', '--no-replace-objects', '-C', str(repo), *args], capture_output=True)
    require(p.returncode == 0, 'git_' + args[0])
    return p.stdout


def pairs(items):
    result = {}
    for key, value in items:
        require(key not in result, 'duplicate_record_key')
        result[key] = value
    return result


def entry(repo, rev, name):
    raw = git(repo, 'ls-tree', '-z', rev, '--', ':(literal)' + name).split(b'\0')
    require(len(raw) == 2 and raw[-1] == b'', 'single_entry')
    meta, actual = raw[0].decode().split('\t', 1)
    require(actual == name, 'literal_entry')
    mode, kind, value = meta.split()
    return dict(mode=mode, type=kind, oid=value)


def actual_entry(repo, rev, name, expected):
    require(isinstance(expected, dict) and set(expected) == {'mode', 'type', 'oid'}, 'entry_shape')
    require(expected['mode'] == '100644' and expected['type'] == 'blob', 'entry_mode_type')
    oid(expected['oid'])
    require(entry(repo, rev, name) == expected, 'actual_entry')


def row(raw):
    values = raw.split('\t')
    require(len(values) == 8 and values[0] == ROLE_PATH and values[1] in ('true', 'false'), 'actual_row_shape')
    for value in values[2:]:
        oid(value)
    return values


def qualify(repo, target, prior_raw, incoming_raw):
    for value in (target, PINNED_RECORD_COMMIT, PINNED_SOURCE, PINNED_BASE):
        oid(value)
    prior, incoming = row(prior_raw), row(incoming_raw)
    raw = git(repo, 'show', PINNED_RECORD_COMMIT + ':' + RECORD_PATH)
    require(hashlib.sha256(raw).hexdigest() == PINNED_RECORD_SHA256, 'record_hash')
    meta = entry(repo, PINNED_RECORD_COMMIT, RECORD_PATH)
    require(meta['mode'] == '100644' and meta['type'] == 'blob', 'record_entry')
    r = json.loads(raw, object_pairs_hook=pairs)
    require(isinstance(r, dict) and set(r) == {'version', 'status', 'classification', 'source', 'base', 'path',
        'historical_tuple', 'actual_prior_row', 'incoming_metadata_at_failure', 'producer_contract',
        'required_current_source', 'historical_runtime_gap', 'current_correction', 'review', 'runtime_pass',
        'generic_identical_blob_authority', 'current_or_future_path_authority', 'activation_authority',
        'artifacts', 'proposal_sha256', 'prior_row_evidence_kind', 'carrier_status'}, 'record_shape')
    require(r['version'] == 'MASTER-ROLE-CHECKPERMISSION-HISTORICAL-CONSUMPTION-v1'
            and r['status'] == 'SOURCE_CONSUMPTION_CONTRACT_CLEAR'
            and r['classification'] == 'ONE_HISTORICAL_SOURCE_DUPLICATE_WITH_REQUIRED_CURRENT_CORRECTION', 'typed_conditional_source')
    require(r['source'] == PINNED_SOURCE and r['base'] == PINNED_BASE and r['path'] == ROLE_PATH, 'source_boundary')
    require(all(r[key] is False for key in ('runtime_pass', 'generic_identical_blob_authority',
                'current_or_future_path_authority', 'activation_authority')), 'no_runtime_or_path_authority')
    require(r['review'].startswith(('https://', 'http://'))
            and r['current_correction']['source_review'].startswith(('https://', 'http://')), 'independent_reviews')
    require(r['current_correction']['commit'] == PINNED_SOURCE, 'mandatory_correction')
    require(r['historical_runtime_gap'] and r['prior_row_evidence_kind'] and r['producer_contract'], 'preserved_gap_and_contract')
    for older, newer in ((PINNED_BASE, PINNED_SOURCE), (PINNED_SOURCE, PINNED_RECORD_COMMIT),
                         (PINNED_RECORD_COMMIT, target)):
        git(repo, 'merge-base', '--is-ancestor', older, newer)
    require(target != PINNED_RECORD_COMMIT, 'later_carrier')
    require(git(repo, 'show', target + ':' + VERIFIER_PATH) == Path(__file__).read_bytes(), 'executed_carrier')
    require(git(repo, 'show', target + ':' + RECORD_PATH) == raw, 'record_drift')
    bindings = r['required_current_source']
    require(isinstance(bindings, list) and len(bindings) == 11, 'complete_eleven_current_bindings')
    names = []
    for item in bindings:
        require(isinstance(item, dict) and set(item) == {'path', 'entry', 'sha256'}, 'current_binding_shape')
        name = item['path']
        require(isinstance(name, str) and re.fullmatch(r'[A-Za-z0-9_./-]+', name)
                and not name.startswith('/') and '..' not in name.split('/'), 'current_literal_path')
        names.append(name)
        for revision in (PINNED_SOURCE, target):
            actual_entry(repo, revision, name, item['entry'])
        require(hashlib.sha256(git(repo, 'cat-file', 'blob', item['entry']['oid'])).hexdigest() == item['sha256'], 'current_raw_hash')
    require(names == sorted(set(names)) and ROLE_PATH in names, 'complete_current_path_set')
    t = r['historical_tuple']
    require(set(t) == {'merge', 'parents', 'best_bases', 'previous_owner', 'previous_owner_parents',
                      'previous_source', 'previous_source_base', 'entries'}, 'historical_tuple_shape')
    merge, previous_owner = oid(t['merge']), oid(t['previous_owner'])
    require(len(t['parents']) == 2 and len(t['previous_owner_parents']) == 2, 'two_actual_parents')
    for commit, parents, bases in ((merge, t['parents'], t['best_bases']),
                                   (previous_owner, t['previous_owner_parents'], [t['previous_source_base']])):
        for value in parents: oid(value)
        require(git(repo, 'rev-list', '--parents', '-n', '1', commit).decode().split() == [commit, *parents], 'actual_parents')
        actual_bases = git(repo, 'merge-base', '--all', *parents).decode().splitlines()
        require(len(actual_bases) == 1 and actual_bases == bases, 'actual_best_base')
    require(t['best_bases'] == [PINNED_BASE] and t['previous_source'] == t['previous_owner_parents'][1], 'actual_source_identity')
    git(repo, 'merge-base', '--is-ancestor', previous_owner, t['parents'][0])
    git(repo, 'merge-base', '--is-ancestor', merge, PINNED_SOURCE)
    expected_revisions = dict(base=PINNED_BASE, first=t['parents'][0], incoming=t['parents'][1], result=merge,
                              previous_owner=previous_owner, previous_source=t['previous_source'])
    require(set(t['entries']) == set(expected_revisions), 'six_historical_entries')
    for label, revision in expected_revisions.items():
        actual_entry(repo, revision, ROLE_PATH, t['entries'][label])
    require(prior == r['actual_prior_row'], 'exact_observed_prior_row')
    require(prior[2:6] == [previous_owner, t['previous_owner_parents'][0], t['previous_source_base'],
                           t['previous_source']] and prior[7] == t['previous_source_base'], 'prior_source_identity')
    m = r['incoming_metadata_at_failure']
    require(set(m) == {'owner', 'parent', 'source_base', 'source_head', 'content_base', 'best_bases'}, 'incoming_metadata_shape')
    require([m['owner'], m['parent'], m['source_base'], m['source_head']] == [merge, t['parents'][0], PINNED_BASE, t['parents'][1]]
            and m['best_bases'] == [PINNED_BASE] and m['content_base'] == prior[6], 'incoming_source_identity')
    require(incoming[2:] == [m['owner'], m['parent'], m['source_base'], m['source_head'],
                             m['content_base'], PINNED_BASE], 'exact_actual_incoming_row')
    require(isinstance(r['artifacts'], list) and len(r['artifacts']) == 8, 'evidence_bindings')
    artifacts = {}
    for proof in r['artifacts']:
        require(set(proof) == {'name', 'sha256', 'bytes'} and isinstance(proof['name'], str)
                and proof['name'] not in artifacts and re.fullmatch(r'[0-9a-f]{64}', proof['sha256'])
                and type(proof['bytes']) is int and proof['bytes'] >= 0, 'evidence_binding')
        artifacts[proof['name']] = proof['sha256']
    require(artifacts.get(r['current_correction']['proof']) == r['current_correction']['proof_sha256']
            and re.fullmatch(r'[0-9a-f]{64}', r['proposal_sha256']), 'correction_evidence_binding')
    return dict(version=r['version'], record=PINNED_RECORD_COMMIT, record_sha256=PINNED_RECORD_SHA256,
                source_boundary=PINNED_SOURCE, prior_row=prior, incoming_row=incoming,
                historical_runtime_gap=r['historical_runtime_gap'], runtime_pass=False,
                generic_identical_blob_authority=False, current_or_future_path_authority=False,
                activation_authority=False, retain_runtime_and_content_taint=True)


def main():
    p = argparse.ArgumentParser(description=__doc__)
    for key in ('repo', 'target', 'prior-row', 'incoming-row'): p.add_argument('--' + key, required=True)
    args = p.parse_args()
    try:
        print(json.dumps(qualify(args.repo, args.target, args.prior_row, args.incoming_row), sort_keys=True))
    except (Rejected, ValueError, KeyError, TypeError, AttributeError, OSError) as exc:
        print('Conditional Role source rejected: ' + str(exc), file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__': sys.exit(main())
