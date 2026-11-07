"""Construction and fail-closed verification of a reviewed source record."""
import hashlib
import os
import re
import subprocess
from pathlib import Path

import f13_release_source as release
import f13_source_dag as dag

SOURCE = 'ffad84570c38455ec77f387ad9ec04fd94411004'
VERSION = 'F13-CURRENT-SOURCE-SUCCESSOR-v1'
RECORD_PATH = 'docs/testing/f13-current-source-v1.json'
RECORD_COMMIT = 'ad5aa28465e4f79a9e9202d8e6d0045470b0b2d9'
RECORD_SHA256 = '921afcc5d6a4117bbc8f5af178cc7b7c796971409f9264bfd0508193e684c97d'
PREFIX = 'docs/testing/f13-current-source-v1/'
FALSE_FLAGS = ('runtime_pass', 'activation_authority', 'current_or_future_path_authority')
CARRIER_PATHS = release.CARRIER_PATHS | frozenset((
    'scripts/ci/f13_current_source.py', 'scripts/ci/f13_current_source_test.py',
    'scripts/ci/f13_current_source_integration_test.py',
    'scripts/ci/f13_base_bridge_test.py',
    'scripts/ci/f13_source_dag.py', 'scripts/ci/f13_source_dag_test.py',
    '.github/ci/path-filters.yml', 'scripts/ci/fast-gate-routing_test.py'))
NATIVE_VERSION = 'NATIVE-P3-SOURCE-v6'
PRODUCTION_VERSION = 'F13-PRODUCTION-SOURCE-v12'


def bridge_event(repo, base, source, target, event, ref, event_sha, before, base_ref,
                 reviewed_source, record_commit, collection_base):
    """Bind a real event to the first master merge that introduced this record."""
    require, git = release.require, release.git
    base, source, target, reviewed_source, record_commit, collection_base = map(
        release.legacy.oid, (base, source, target, reviewed_source, record_commit, collection_base))
    require(git(repo, 'rev-parse', '--is-shallow-repository').strip() == b'false', 'bridge_shallow')
    require(git(repo, 'rev-parse', 'HEAD').decode().strip() == target, 'bridge_actual_head')
    require(event_sha == target, 'bridge_event_sha')
    require((event == 'push' and ref == 'refs/heads/master' and before == base) or
            (event == 'pull_request' and base_ref == 'master' and
             re.fullmatch(r'refs/pull/[1-9][0-9]*/merge', ref)), 'bridge_master_event')
    require(git(repo, 'rev-list', '--parents', '-n', '1', record_commit).decode().split() ==
            [record_commit, reviewed_source], 'bridge_record_direct_parent')
    require(git(repo, 'rev-list', '--parents', '-n', '1', target).decode().split() ==
            [target, base, source], 'bridge_ordered_parents')
    release.legacy.ancestor(repo, reviewed_source, base)
    release.legacy.ancestor(repo, base, source)
    release.legacy.ancestor(repo, record_commit, source)
    for revision in (base, source):
        require(collection_base in git(repo, 'rev-list', '--first-parent', revision).decode().splitlines(),
                'bridge_collection_first_parent')
    require(git(repo, 'rev-parse', source + '^{tree}') == git(repo, 'rev-parse', target + '^{tree}'),
            'bridge_result_tree')

    def contains(older, newer):
        result = subprocess.run(['git', '--no-replace-objects', '-C', str(repo), 'merge-base',
                                 '--is-ancestor', older, newer], capture_output=True, check=False)
        require(result.returncode in (0, 1), 'bridge_ancestry')
        return result.returncode == 0

    if contains(record_commit, base):
        first_parent = git(repo, 'rev-list', '--first-parent', '--reverse',
                           reviewed_source + '..' + base).decode().splitlines()
        require(reviewed_source in git(repo, 'rev-list', '--first-parent', base).decode().splitlines(),
                'bridge_master_first_parent')
        first_merge = next((rev for rev in first_parent if contains(record_commit, rev)), None)
        require(first_merge is not None, 'bridge_first_merge')
        parents = git(repo, 'rev-list', '--parents', '-n', '1', first_merge).decode().split()
        require(len(parents) == 3 and not contains(record_commit, parents[1]) and
                contains(record_commit, parents[2]), 'bridge_first_merge_parents')
        initial_base, initial_source = parents[1:]
    else:
        initial_base, initial_source, first_merge = base, source, target
    require(initial_base != reviewed_source and not contains(record_commit, initial_base),
            'bridge_distinct_initial_base')
    release.legacy.ancestor(repo, reviewed_source, initial_base)
    release.legacy.ancestor(repo, initial_base, initial_source)
    require(git(repo, 'rev-parse', initial_source + '^{tree}') ==
            git(repo, 'rev-parse', first_merge + '^{tree}'), 'bridge_first_result_tree')
    chain = git(repo, 'rev-list', '--first-parent', '--reverse',
                record_commit + '..' + initial_source).decode().splitlines()
    require(record_commit in git(repo, 'rev-list', '--first-parent', initial_source).decode().splitlines(),
            'bridge_record_first_parent')
    previous, incorporation = record_commit, None
    for revision in chain:
        if contains(initial_base, revision):
            incorporation = revision
            break
        previous = revision
    carrier = previous
    require(incorporation is not None and carrier != record_commit, 'bridge_carrier_missing')
    require(git(repo, 'rev-list', '--parents', '-n', '1', incorporation).decode().split() ==
            [incorporation, carrier, initial_base], 'bridge_incorporation_parents')
    return dict(type='PRESERVED-UNIT-BASE-BRIDGE-v1', reviewed_source=reviewed_source,
        record=record_commit, initial_base=initial_base, carrier=carrier, incorporation=incorporation,
        initial_source=initial_source, first_merge=first_merge, base=base, source=source, target=target)


def bridge_history(repo, binding, units, historical_paths, carrier_paths, archive_path):
    """Verify the full pre-carrier, base-side and post-carrier Git DAGs."""
    require, git = release.require, release.git
    h, r, b0, k, s0, g, b, s, t = (binding[name] for name in
        ('reviewed_source', 'record', 'initial_base', 'carrier', 'initial_source',
         'first_merge', 'base', 'source', 'target'))
    require(h in git(repo, 'rev-list', '--first-parent', b0).decode().splitlines(),
            'bridge_base_first_parent')
    baseline = release.documentation.tree_entries(repo, h)
    recorded = release.documentation.tree_entries(repo, r)
    carrier = release.documentation.tree_entries(repo, k)
    base_tree = release.documentation.tree_entries(repo, b0)
    additions = set(recorded) - set(baseline)
    carrier_paths = set(carrier_paths)
    require(not additions & carrier_paths and all(path in carrier for path in carrier_paths),
            'bridge_carrier_paths')
    require(not additions & set(base_tree), 'bridge_base_proof_collision')
    unit_paths = {row['path'] for unit in units.values() for row in unit['entries']}
    require(not unit_paths & carrier_paths, 'bridge_carrier_unit_overlap')
    old_paths = set(historical_paths) | {archive_path} | {
        p for p in baseline if p.startswith(('docs/testing/native-p3-source',
            'docs/testing/f13-production', 'docs/testing/role-checkpermission',
            'docs/testing/role-source-v1-evidence/', 'docs/testing/role-source-v2-evidence/'))}
    require(all(path in baseline for path in old_paths | unit_paths), 'bridge_missing_baseline')
    directories = {row['directory']: row['entries'] for unit in units.values()
                   for row in unit['directory_inventories']}
    voice = {p: value for p, value in baseline.items() if p.startswith('src/backend/voice/')}

    def contains(older, newer):
        result = subprocess.run(['git', '--no-replace-objects', '-C', str(repo), 'merge-base',
                                 '--is-ancestor', older, newer], capture_output=True, check=False)
        require(result.returncode in (0, 1), 'bridge_history_ancestry')
        return result.returncode == 0

    def common(tree, label):
        require(all(tree.get(p) == baseline.get(p) for p in unit_paths | old_paths), label + '_entry')
        actual = release.documentation.immediate_directory_entries(tree)
        require(all(actual.get(d, {}) == entries for d, entries in directories.items()), label + '_directory')
        require({p: value for p, value in tree.items() if p.startswith('src/backend/voice/')} == voice,
                label + '_voice')

    def base_revision(revision):
        tree = release.documentation.tree_entries(repo, revision)
        common(tree, 'bridge_base')
        require(all(tree.get(p) == baseline.get(p) for p in carrier_paths), 'bridge_base_carrier')
        require(not additions & set(tree), 'bridge_base_proof')

    def post_revision(revision):
        tree = release.documentation.tree_entries(repo, revision)
        common(tree, 'bridge_after_carrier')
        require(all(tree.get(p) == recorded[p] for p in additions), 'bridge_after_carrier_proof')
        require(all(tree.get(p) == carrier[p] for p in carrier_paths), 'bridge_after_carrier_bytes')

    for revision in git(repo, 'rev-list', '--topo-order', '--reverse', b0, '^' + h).decode().splitlines():
        base_revision(revision)
    for revision in git(repo, 'rev-list', '--topo-order', '--reverse', k, '^' + r).decode().splitlines():
        tree = release.documentation.tree_entries(repo, revision)
        common(tree, 'bridge_precarrier')
        if contains(r, revision):
            require(all(tree.get(p) == recorded[p] for p in additions), 'bridge_precarrier_proof')
        else:
            require(not additions & set(tree), 'bridge_precarrier_proof')

    overlay = dict(base_tree)
    overlay.update({p: recorded[p] for p in additions})
    overlay.update({p: carrier[p] for p in carrier_paths})
    require(release.documentation.tree_entries(repo, s0) == overlay, 'bridge_initial_overlay')

    def verify_interval(end, exclude, qualified_base):
        for revision in git(repo, 'rev-list', '--topo-order', '--reverse', end,
                            '^' + exclude).decode().splitlines():
            if contains(k, revision):
                post_revision(revision)
            else:
                require(not contains(r, revision) and contains(revision, qualified_base),
                        'bridge_unqualified_side_history')
                base_revision(revision)

    verify_interval(s0, k, b0)
    verify_interval(s, g, b)
    for revision in (k, s0, g, s, t):
        post_revision(revision)


def closure(repo, source, raw, inherited_paths, inherited_directories, extra_paths):
    release.legacy.oid(source)
    tree = release.documentation.tree_entries(repo, source)
    compiled, discovered_directories, manifests = release.role_evidence_paths(raw, tree)
    release.require(compiled and discovered_directories, 'current_source_empty_closure')
    directories = sorted(set(discovered_directories) | set(inherited_directories))
    directory_entries = release.documentation.immediate_directory_entries(tree)
    inventories = [dict(directory=d, entries=directory_entries.get(d, {})) for d in directories]
    release.require(all(row['entries'] for row in inventories), 'current_source_missing_directory')
    paths = set(compiled) | set(manifests) | set(inherited_paths) | set(extra_paths)
    paths |= {path for row in inventories for path in row['entries']}
    entries = [dict(path=path, entry=release._meta(tree, path)) for path in sorted(paths)]
    release.require(all(row['entry'] is not None for row in entries), 'current_source_missing_context')
    return dict(source=source, compiled_paths=compiled, directories=directories,
                module_manifests=manifests, entries=entries, directory_inventories=inventories)


def derive_units(repo, source, native_raw, role_raw, native, role):
    """Reconstruct both units from already verified historical records."""
    def inherited(record, old_closure):
        paths = set(old_closure['compiled_paths'])
        for group in ('projection', 'context', 'required_current_source'):
            paths.update(row['path'] for row in record.get(group, []))
        return sorted(paths), [row['directory'] for row in old_closure['directory_inventories']]

    native_record = native['release_record']
    role_record = role['release_record']
    native_paths, native_dirs = inherited(native_record, native_record['closure'])
    role_paths, role_dirs = inherited(role_record, role['closure'])
    tree = release.documentation.tree_entries(repo, source)
    extras = [p for p in tree if p.startswith('src/backend/voice/')]
    extras.append('protos/voice/user/v1/user_source_custody.proto')
    return dict(native=closure(repo, source, native_raw, native_paths, native_dirs, extras),
                role=closure(repo, source, role_raw, role_paths, role_dirs, []))


def validate_header(record):
    require = release.require
    require(isinstance(record, dict) and set(record) == {
        'version', 'status', 'source', 'source_tree', 'predecessor', 'dag', 'native', 'role',
        'runtime_evidence', 'source_review', *FALSE_FLAGS}, 'current_record_shape')
    require(record['version'] == VERSION and record['status'] == 'SOURCE_CONSUMPTION_CONTRACT_CLEAR'
            and record['source'] == SOURCE and all(record[k] is False for k in FALSE_FLAGS),
            'current_record_not_reviewed')


def artifact(repo, proof):
    require, git = release.require, release.git
    require(isinstance(proof, dict) and set(proof) == {'path', 'sha256'}, 'current_proof_shape')
    path, digest = proof['path'], proof['sha256']
    require(isinstance(path, str) and path.startswith(PREFIX) and '..' not in path.split('/')
            and '\\' not in path and isinstance(digest, str) and re.fullmatch('[0-9a-f]{64}', digest),
            'current_proof_identity')
    meta = release.legacy.entry(repo, RECORD_COMMIT, path)
    require(meta is not None and meta['mode'] == '100644' and meta['type'] == 'blob', 'current_proof_entry')
    raw = git(repo, 'cat-file', 'blob', meta['oid'])
    require(hashlib.sha256(raw).hexdigest() == digest, 'current_proof_hash')
    return raw


def _load_bundle(repo):
    """Revalidate predecessor, source closures and separately reviewed evidence."""
    require, git = release.require, release.git
    require(RECORD_COMMIT is not None and RECORD_SHA256 is not None, 'current_record_not_pinned')
    release.legacy.oid(RECORD_COMMIT)
    require(re.fullmatch('[0-9a-f]{64}', RECORD_SHA256), 'current_record_hash_shape')
    require(git(repo, 'rev-list', '--parents', '-n', '1', RECORD_COMMIT).decode().split()
            == [RECORD_COMMIT, SOURCE], 'current_record_direct_parent')
    meta = release.legacy.entry(repo, RECORD_COMMIT, RECORD_PATH)
    require(meta is not None and meta['mode'] == '100644' and meta['type'] == 'blob', 'current_record_entry')
    raw = git(repo, 'cat-file', 'blob', meta['oid'])
    require(hashlib.sha256(raw).hexdigest() == RECORD_SHA256, 'current_record_hash')
    record = release.read_json(raw)
    validate_header(record)
    predecessor = dict(source=release.FOUNDATION_BASE, record=release.RECORD_COMMIT,
                       native_sha256=release.NATIVE_SHA256, production_sha256=release.PRODUCTION_SHA256,
                       role_sha256=release.ROLE_SHA256)
    require(record['predecessor'] == predecessor, 'current_predecessor')
    require(record['source_tree'] == git(repo, 'rev-parse', SOURCE + '^{tree}').decode().strip(),
            'current_source_tree')
    _, native, role = release.production_record(repo, include_units=True)
    release.legacy.ancestor(repo, release.RECORD_COMMIT, SOURCE)
    old_tree = release.documentation.tree_entries(repo, release.RECORD_COMMIT)
    foundation_tree = release.documentation.tree_entries(repo, release.FOUNDATION_BASE)
    source_tree = release.documentation.tree_entries(repo, SOURCE)
    protected = (set(old_tree) - set(foundation_tree)) | {
        p for p in old_tree if p.startswith(('docs/testing/native-p3-source',
            'docs/testing/f13-production', 'docs/testing/role-checkpermission',
            'docs/testing/role-source-v1-evidence/', 'docs/testing/role-source-v2-evidence/'))}
    require(all(source_tree.get(path) == old_tree[path] for path in protected),
            'current_historical_artifact_drift')
    for group in ('native', 'role'):
        require(isinstance(record[group], dict) and set(record[group]) ==
                ({'raw_go_list', 'closure'} if group == 'native' else
                 {'raw_go_list', 'closure', 'historical_runtime_gap'}), 'current_unit_shape')
    require(record['role']['historical_runtime_gap'] == role['historical_runtime_gap'], 'current_role_gap')
    native_raw = artifact(repo, record['native']['raw_go_list'])
    role_raw = artifact(repo, record['role']['raw_go_list'])
    units = derive_units(repo, SOURCE, native_raw, role_raw, native, role)
    for group in ('native', 'role'):
        require(release.read_json(artifact(repo, record[group]['closure'])) == units[group],
                'current_closure_mismatch')
    history = artifact(repo, record['dag'])
    require(history == dag.raw_history(repo, release.FOUNDATION_BASE, SOURCE), 'current_dag_mismatch')
    manifest = release.read_json(artifact(repo, record['runtime_evidence']))
    require(isinstance(manifest, dict) and set(manifest) == {'source', 'artifacts'} and
            manifest['source'] == SOURCE and isinstance(manifest['artifacts'], list) and
            manifest['artifacts'], 'current_runtime_manifest')
    for proof in manifest['artifacts']:
        artifact(repo, proof)
    review = release.read_json(artifact(repo, record['source_review']))
    expected = dict(type='F13-CURRENT-SOURCE-REVIEW-v1', status='SOURCE_CONSUMPTION_CONTRACT_CLEAR',
        source=SOURCE, source_tree=record['source_tree'], predecessor_record=release.RECORD_COMMIT,
        dag_sha256=record['dag']['sha256'], native_raw_sha256=record['native']['raw_go_list']['sha256'],
        native_closure_sha256=record['native']['closure']['sha256'],
        role_raw_sha256=record['role']['raw_go_list']['sha256'],
        role_closure_sha256=record['role']['closure']['sha256'],
        runtime_evidence_sha256=record['runtime_evidence']['sha256'], **{key: False for key in FALSE_FLAGS})
    require(isinstance(review, dict) and set(review) == set(expected) | {'reviewer', 'limitations'} and
            all(review[k] == v and type(review[k]) is type(v) for k, v in expected.items()) and
            isinstance(review['reviewer'], str) and review['reviewer'].strip() and
            isinstance(review['limitations'], list) and review['limitations'] and
            all(isinstance(v, str) and v.strip() for v in review['limitations']), 'current_review')
    proofs = [record['dag'], record['native']['raw_go_list'], record['native']['closure'],
              record['role']['raw_go_list'], record['role']['closure'], record['runtime_evidence'],
              record['source_review'], *manifest['artifacts']]
    names = [proof['path'] for proof in proofs]
    require(len(names) == len(set(names)), 'current_duplicate_proof')
    before = release.documentation.tree_entries(repo, SOURCE)
    after = release.documentation.tree_entries(repo, RECORD_COMMIT)
    require(not set(names + [RECORD_PATH]) & set(before) and
            set(after) - set(before) == set(names + [RECORD_PATH]) and
            all(after.get(path) == entry for path, entry in before.items()), 'current_record_only_additions')
    return record, units, native, role


def load_record(repo):
    record, units, _, _ = _load_bundle(repo)
    return record, units


def applicable(repo, revision='HEAD'):
    present = subprocess.run(['git', '--no-replace-objects', '-C', str(repo), 'rev-parse', '--verify',
                              '--quiet', SOURCE + '^{commit}'], capture_output=True, check=False)
    release.require(present.returncode in (0, 1), 'current_source_lookup')
    if present.returncode == 1:
        return False
    result = subprocess.run(['git', '--no-replace-objects', '-C', str(repo),
        'merge-base', '--is-ancestor', SOURCE, revision], capture_output=True, check=False)
    release.require(result.returncode in (0, 1), 'current_ancestry')
    return result.returncode == 0


def _check_carrier(repo, carrier):
    require, git = release.require, release.git
    recorded = release.documentation.tree_entries(repo, RECORD_COMMIT)
    actual = release.documentation.tree_entries(repo, carrier)
    require({p: v for p, v in recorded.items() if p not in CARRIER_PATHS} ==
            {p: v for p, v in actual.items() if p not in CARRIER_PATHS}, 'current_carrier_delta')
    for path in CARRIER_PATHS:
        meta = release.legacy.entry(repo, carrier, path)
        require(meta is not None and meta['mode'] == '100644' and meta['type'] == 'blob' and
                git(repo, 'cat-file', 'blob', meta['oid']) == (release.ROOT / path).read_bytes(),
                'current_executed_carrier')
    require(release.legacy.entry(repo, carrier, 'scripts/ci/f13_current_source.py') !=
            release.legacy.entry(repo, RECORD_COMMIT, 'scripts/ci/f13_current_source.py'),
            'current_later_carrier_bytes')


def _check_continuation(repo, foundation, source, units, source_base, record_commit, historical_paths):
    import f13_production_source as production
    require, git = release.require, release.git
    carrier, first = foundation['source'], foundation['target']
    baseline = release.documentation.tree_entries(repo, source_base)
    recorded = release.documentation.tree_entries(repo, record_commit)
    carrier_tree = release.documentation.tree_entries(repo, carrier)
    unit_paths = {row['path'] for unit in units.values() for row in unit['entries']}
    require(not unit_paths & CARRIER_PATHS, 'current_carrier_unit_overlap')
    protected = (set(recorded) - set(baseline)) | {
        p for p in baseline if p.startswith(('docs/testing/native-p3-source',
            'docs/testing/f13-production', 'docs/testing/role-checkpermission',
            'docs/testing/role-source-v1-evidence/', 'docs/testing/role-source-v2-evidence/'))}
    require(production.ARCHIVE_PATH in baseline, 'current_missing_historical_archive')
    protected.add(production.ARCHIVE_PATH)
    protected.update(historical_paths)
    require(all(path in baseline for path in historical_paths), 'current_missing_historical_artifact')
    names = unit_paths | protected | set(CARRIER_PATHS)
    expected = {p: (carrier_tree.get(p) if p in CARRIER_PATHS else
                    recorded.get(p) if p in protected and p not in baseline else baseline.get(p)) for p in names}
    dirs = {row['directory']: row['entries'] for unit in units.values() for row in unit['directory_inventories']}
    voice = {p: v for p, v in baseline.items() if p.startswith('src/backend/voice/')}
    record_additions = set(recorded) - set(baseline)
    stable = unit_paths | (protected - record_additions - set(CARRIER_PATHS))

    def verify_precarrier(revision):
        tree = release.documentation.tree_entries(repo, revision)
        require(all(tree.get(p) == baseline.get(p) for p in stable), 'current_precarrier_entry')
        actual_dirs = release.documentation.immediate_directory_entries(tree)
        require(all(actual_dirs.get(d, {}) == entries for d, entries in dirs.items()),
                'current_precarrier_directory')
        require({p: v for p, v in tree.items() if p.startswith('src/backend/voice/')} == voice,
                'current_precarrier_voice')
        ancestry = subprocess.run(['git', '--no-replace-objects', '-C', str(repo), 'merge-base',
            '--is-ancestor', record_commit, revision], capture_output=True, check=False)
        require(ancestry.returncode in (0, 1), 'current_precarrier_ancestry')
        require(all(tree.get(p) == recorded[p] if ancestry.returncode == 0 else
                    tree.get(p) in (None, recorded[p]) for p in record_additions),
                'current_precarrier_record')

    def verify(revision):
        tree = release.documentation.tree_entries(repo, revision)
        require(all(tree.get(p) == value for p, value in expected.items()), 'current_continuation_entry')
        actual_dirs = release.documentation.immediate_directory_entries(tree)
        require(all(actual_dirs.get(d, {}) == entries for d, entries in dirs.items()),
                'current_continuation_directory')
        require({p: v for p, v in tree.items() if p.startswith('src/backend/voice/')} == voice,
                'current_continuation_voice')

    for revision in git(repo, 'rev-list', '--topo-order', '--reverse', carrier,
                        '^' + record_commit).decode().splitlines():
        verify_precarrier(revision)
    for revision in (carrier, first, source):
        verify(revision)
    for revision in git(repo, 'rev-list', '--topo-order', '--reverse', source, '^' + first).decode().splitlines():
        release.legacy.ancestor(repo, first, revision)
        verify(revision)


def _event(repo, base, source, target, event, ref, event_sha, before, base_ref):
    """Choose the old event contract or the separately typed advanced-base contract."""
    require, git = release.require, release.git
    if base == SOURCE:
        return release.check_event(repo, base, source, target, event, ref, event_sha, before,
                                   base_ref, SOURCE, RECORD_COMMIT)
    result = subprocess.run(['git', '--no-replace-objects', '-C', str(repo), 'merge-base',
        '--is-ancestor', RECORD_COMMIT, base], capture_output=True, check=False)
    require(result.returncode in (0, 1), 'current_event_ancestry')
    if result.returncode == 0:
        for revision in git(repo, 'rev-list', '--first-parent', '--reverse',
                            SOURCE + '..' + base).decode().splitlines():
            ancestor = subprocess.run(['git', '--no-replace-objects', '-C', str(repo), 'merge-base',
                '--is-ancestor', RECORD_COMMIT, revision], capture_output=True, check=False)
            require(ancestor.returncode in (0, 1), 'current_event_ancestry')
            if ancestor.returncode == 0:
                parents = git(repo, 'rev-list', '--parents', '-n', '1', revision).decode().split()
                if len(parents) == 3 and parents[1] == SOURCE:
                    return release.check_event(repo, base, source, target, event, ref, event_sha,
                                               before, base_ref, SOURCE, RECORD_COMMIT)
                break
    return bridge_event(repo, base, source, target, event, ref, event_sha, before, base_ref,
                        SOURCE, RECORD_COMMIT, release.COLLECTION_BASE)


def _native_qualify(repo, base, source, target, event, ref, event_sha, before, base_ref, bundle):
    require, git = release.require, release.git
    require(Path(repo).resolve() == release.ROOT.resolve(), 'current_actual_root')
    require(git(repo, 'rev-parse', 'HEAD').decode().strip() == target, 'current_actual_head')
    record, units, old, _ = bundle
    foundation = _event(repo, base, source, target, event, ref, event_sha, before, base_ref)
    bridged = foundation.get('type') == 'PRESERVED-UNIT-BASE-BRIDGE-v1'
    _check_carrier(repo, foundation['carrier'] if bridged else foundation['source'])
    old_record_tree = release.documentation.tree_entries(repo, release.RECORD_COMMIT)
    old_foundation_tree = release.documentation.tree_entries(repo, release.FOUNDATION_BASE)
    historical_paths = set(old_record_tree) - set(old_foundation_tree)
    if bridged:
        import f13_production_source as production
        bridge_history(repo, foundation, units, historical_paths, CARRIER_PATHS, production.ARCHIVE_PATH)
    else:
        _check_continuation(repo, foundation, source, units, SOURCE, RECORD_COMMIT, historical_paths)
    return dict(version=NATIVE_VERSION, consumer='MASTER-SOURCE-v1', base=base, source=source,
        target=target, content_base=old['base'], snapshot=SOURCE, record=RECORD_COMMIT,
        record_sha256=RECORD_SHA256, paths=[x['path'] for x in old['projection']],
        retain_guarded_and_runtime_history=True, collection_base=release.COLLECTION_BASE,
        foundation=foundation, qualification=('PRESERVED_UNIT_BASE_BRIDGE' if bridged else 'REVIEWED_CURRENT_SOURCE'),
        event_delta_sha256=release.sha(git(repo, 'diff', '--raw', '--no-abbrev', '--no-renames', base, source)))


def native_qualify(repo, base, source, target, event, ref, event_sha, before, base_ref):
    release.require(Path(repo).resolve() == release.ROOT.resolve(), 'current_actual_root')
    release.require(release.git(repo, 'rev-parse', 'HEAD').decode().strip() == target, 'current_actual_head')
    foundation = _event(repo, base, source, target, event, ref, event_sha, before, base_ref)
    _check_carrier(repo, foundation['carrier'] if foundation.get('type') ==
                   'PRESERVED-UNIT-BASE-BRIDGE-v1' else foundation['source'])
    return _native_qualify(repo, base, source, target, event, ref, event_sha, before, base_ref,
                           _load_bundle(repo))


def verified_record(repo, binding):
    env = os.environ
    bundle = _load_bundle(repo)
    expected = _native_qualify(repo, binding['base'], binding['source'], binding['target'],
        env.get('GITHUB_EVENT_NAME', ''), env.get('GITHUB_REF', ''), env.get('GITHUB_SHA', ''),
        env.get('VOICE_CI_PUSH_BEFORE_SHA', ''), env.get('GITHUB_BASE_REF', ''), bundle)
    release.require(binding == expected, 'current_native_binding')
    record, _, native, _ = bundle
    old = dict(native)
    tree = release.documentation.tree_entries(repo, SOURCE)
    old['projection'] = [dict(row, after=release._meta(tree, row['path'])) for row in old['projection']]
    old['source'] = SOURCE
    old['release_record'] = record
    return old


def audit_map(repo, binding, map_path, history_path):
    return release.audit_map_record(repo, binding, map_path, history_path, verified_record(repo, binding))


def predecessor_record(repo, binding):
    verified_record(repo, binding)
    return release.documentation, release.documentation.load_context_record(repo)


def collection_base(repo, binding):
    verified_record(repo, binding)
    return release.COLLECTION_BASE


def _production_qualify(repo, bundle):
    import f13_production_source as production
    require, git = release.require, release.git
    require(Path(repo).resolve() == release.ROOT.resolve(), 'current_production_root')
    record, _, _, _ = bundle
    target = git(repo, 'rev-parse', 'HEAD').decode().strip()
    event = {k: os.environ.get(k, '') for k in ('GITHUB_EVENT_NAME', 'GITHUB_REF', 'GITHUB_SHA',
             'GITHUB_BASE_REF', 'VOICE_CI_PUSH_BEFORE_SHA')}
    parents = git(repo, 'rev-list', '--parents', '-n', '1', target).decode().split()
    require(len(parents) == 3, 'current_production_ordered_parents')
    native = _native_qualify(repo, parents[1], parents[2], target, event['GITHUB_EVENT_NAME'],
        event['GITHUB_REF'], event['GITHUB_SHA'], event['VOICE_CI_PUSH_BEFORE_SHA'], event['GITHUB_BASE_REF'], bundle)
    old = production.load_record(repo)
    return dict(type=PRODUCTION_VERSION, record=RECORD_COMMIT, record_sha256=RECORD_SHA256,
        predecessor_record=release.RECORD_COMMIT, historical_validation_source=old['source'],
        reviewed_source=SOURCE, reviewed_tree=record['source_tree'], base=parents[1],
        collection_base=release.COLLECTION_BASE, source=parents[2], target=target,
        foundation=native['foundation'], qualification=native['qualification'],
        event_delta_sha256=native['event_delta_sha256'], historical_collector=production.HISTORICAL_SCOPE,
        native_content_baseline=production.NATIVE_CONTENT_BASE, event=event,
        admission=False, runtime_pass=False, current_or_future_path_authority=False)


def production_qualify(repo):
    return _production_qualify(repo, _load_bundle(repo))


def role_qualify(repo, target, prior_raw, incoming_raw):
    import f13_role_conditional_successor as role
    historical = role.legacy.qualify(repo, role.HISTORICAL_CARRIER, prior_raw, incoming_raw)
    release.require(release.git(repo, 'rev-parse', 'HEAD').decode().strip() == target, 'current_role_head')
    bundle = _load_bundle(repo)
    _production_qualify(repo, bundle)
    record = bundle[0]
    return dict(version=VERSION, record=RECORD_COMMIT, record_sha256=RECORD_SHA256,
        source_boundary=SOURCE, actual_target=target, historical_validation_target=role.HISTORICAL_CARRIER,
        historical=historical, prior_row=role.legacy.row(prior_raw), incoming_row=role.legacy.row(incoming_raw),
        historical_runtime_gap=record['role']['historical_runtime_gap'], retain_runtime_and_content_taint=True,
        **{key: False for key in role.FALSE_FLAGS})
