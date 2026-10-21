"""Historical actual-row contract RED through the production-shared J guard.

This is a fixture, not a producer/resume API or production admission. Setup or
provenance failure has exit2 and can never stand in for the assertion RED(exit1).
Execution requires independent CODE review and the caller's explicit test stage.
"""
import hashlib
import importlib.util
import json
import os
from pathlib import Path, PurePosixPath
import re
import shlex
import shutil
import stat
import subprocess
import sys
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parents[2]
FIXTURE = ROOT / 'scripts/ci/testdata/f13-exact-j-547'
ARCHIVE_SHA = 'ba6a775f6c38664d8603ac8080abca5c5e464d865b2e94b297127b6217549bd1'
ARCHIVE_BYTES = 92933183
SOURCE = '547df286d3e8c5846b09885a6c21d0610b42d7f5'
TARGET = 'c6a7876239ddbf70ac1d084c64a1e3f95eeda846'
BASE = 'ea845cc695b3cde2a13ba9a97345dd78795833e0'
TRACE_SHA = '08259a181aca2a185a3310aa2f745f663fc6a0baddc458418136b02da02e4fe8'
MAP_SHA = 'f088e093d271ea39b5394be0dd217d1d17f99782b8f7349d39be5eac0dde9e45'
LEDGER_SHA = '974d3d65066541c254645d87bb0939e66e30341dd59ea47e5310755c45b1ed17'
PENDING_SHA = 'd54ddd7340b0de4b1e9aed91286ccc1e5148bad7b8f832af898b452245da552a'
OBSERVED = Path('/tmp/F13-author-547')
PATH = 'src/backend/space/internal/grpcsvc/audit.go'
OPERATION = 'F5/3071ddad390443de4119c796cdad9899ff08f7fd/2'
OUTER_SOURCE_HASHES = {
    'scripts/ci/f13_f54_history.py': 'ead5eb6b3c0548f5b7d64ed332929cafdb3adbe1860e517ab8fe46a9837db27f',
    'scripts/ci/f13_f54_history_test.py': '6f3f21e921a3b68e2263940b169cf8a4b99ed2f8ff84b4042a215ef433495c08',
    'scripts/ci/voice-db-runtime-provisioning-contract_test.sh': 'e2c4d941f08e89737ecfd4298d7eac638f574998c054961413112edf7a14e4d2',
    '.agent/execplans/native-p3-master-source.md': 'e341dd23c6c1833e2a33cd7e306cff4d618c9772b9b97453c66accd809403afc',
}
HARNESS_SOURCE_PATHS = list(OUTER_SOURCE_HASHES) + [
    'scripts/ci/f13_exact_j_historical_red_test.py', 'scripts/ci/f13_guarded_merge_test.py',
    'scripts/ci/f13_guarded_correction_test.py', 'Makefile',
    'scripts/ci/testdata/f13-exact-j-547/source-unit.json',
]


class SetupFailure(Exception):
    pass


def require(value, reason):
    if not value:
        raise SetupFailure(reason)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def run(args, **kwargs):
    p = subprocess.run(args, capture_output=True, **kwargs)
    require(p.returncode == 0, 'command:' + repr(args) + ':' + p.stderr.decode(errors='replace')[-2000:])
    return p.stdout


def materialize(root):
    """Strict lossless private-root extraction; never writes a host logical path."""
    raw = (FIXTURE / 'historical-inputs.zip').read_bytes()
    require(len(raw) == ARCHIVE_BYTES and sha(raw) == ARCHIVE_SHA, 'immutable_archive')
    with zipfile.ZipFile(FIXTURE / 'historical-inputs.zip') as archive:
        manifest_raw = archive.read('transport-manifest.json')
    require(len(manifest_raw) == 54004, 'fixed_archive_manifest_bytes')
    m = json.loads(manifest_raw)
    require(m['unique_objects'] == 227 and m['uncompressed_object_bytes'] == 400559753, 'fixed_extract_bounds')
    objects = {}
    logical = {}
    for item in m['logical_files']:
        name, digest, size = item['logical_path'], item['sha256'], item['bytes']
        p = PurePosixPath(name)
        require(p.is_absolute() and '..' not in p.parts and str(p) == name, 'literal_logical_path')
        require(name.startswith(('/out/', '/inputs/', '/unit/', '/fixed3071-unit/', '/original/', '/tmp/F13-author-547/')), 'closed_logical_roots')
        require(re.fullmatch('[0-9a-f]{64}', digest) and type(size) is int and size >= 0, 'object_shape')
        require(name not in logical or logical[name] == (digest, size), 'conflicting_alias')
        require(digest not in objects or objects[digest] == size, 'conflicting_object')
        logical[name] = (digest, size)
        objects[digest] = size
    require(len(objects) == 227 and sum(objects.values()) == 400559753 and len(logical) == 246, 'complete_fixed_manifest')
    with zipfile.ZipFile(FIXTURE / 'historical-inputs.zip') as z:
        names = ['objects/' + x for x in objects] + ['transport-manifest.json']
        require(len(z.infolist()) == len(names) and set(z.namelist()) == set(names), 'closed_archive_entries')
        for info in z.infolist():
            require(not stat.S_ISLNK(info.external_attr >> 16) and not info.is_dir(), 'no_symlink_or_directory')
            expected = len(manifest_raw) if info.filename == 'transport-manifest.json' else objects[info.filename[8:]]
            require(info.file_size == expected, 'bounded_uncompressed_entry')
        require(z.read('transport-manifest.json') == manifest_raw, 'archive_manifest_exact')
        for name, (digest, size) in logical.items():
            dest = root / name.lstrip('/')
            require(root.resolve() in dest.resolve().parents and not dest.exists(), 'private_no_overwrite')
            data = z.read('objects/' + digest)
            require(len(data) == size and sha(data) == digest, 'raw_object_binding')
            dest.parent.mkdir(parents=True, exist_ok=True)
            dest.write_bytes(data)
    for item in m['expected_absences']:
        require(not (root / item['logical_path'].lstrip('/')).exists(), 'keep_actual_absence')
    return m


def namespace_main():
    """Mounts exist only in the isolated child's namespace; no Docker/service."""
    require(sys.platform == 'linux', 'Linux_only_no_Windows_test')
    require(os.geteuid() != 0, 'ordinary_runner_uid_required')
    require(shutil.which('sudo') and shutil.which('unshare') and shutil.which('mount'), 'namespace_tools_required')
    head = run(['git', '-C', str(ROOT), 'rev-parse', 'HEAD']).decode().strip()
    source_bindings = {}
    for name in HARNESS_SOURCE_PATHS:
        data = (ROOT / name).read_bytes()
        require(run(['git', '-C', str(ROOT), 'show', head + ':' + name]) == data, 'immutable_testhead_source:' + name)
        source_bindings[name] = sha(data)
    harness_identity = json.dumps(dict(commit=head, files=source_bindings), sort_keys=True)
    with tempfile.TemporaryDirectory(prefix='f13-exact-j-') as folder:
        root = Path(folder) / 'root'
        root.mkdir()
        materialize(root)
        for name in ('repo', 'harness', 'tmp', 'dev', 'proc'):
            (root / name).mkdir(exist_ok=True)
        # The runner's already-installed toolchain is mounted read-only. No pull,
        # package installation, host network or mutation of host root directories.
        mounts = []
        for name in ('usr', 'bin', 'lib', 'lib64', 'opt', 'etc'):
            source = Path('/') / name
            if source.exists():
                (root / name).mkdir(exist_ok=True)
                mounts.append((str(source), str(root / name)))
        mounts.append((str(ROOT), str(root / 'harness')))
        script = 'set -eu\nmount --make-rprivate /\n'
        for source, target in mounts:
            script += 'mount --bind ' + shlex.quote(source) + ' ' + shlex.quote(target) + '\n'
            script += 'mount -o remount,bind,ro ' + shlex.quote(target) + '\n'
        (root / 'dev/null').touch()
        script += 'mount --bind /dev/null ' + shlex.quote(str(root / 'dev/null')) + '\n'
        script += 'mount -t proc proc ' + shlex.quote(str(root / 'proc')) + '\n'
        executable = '/harness/scripts/ci/f13_exact_j_historical_red_test.py'
        script += 'exec chroot --userspec=' + str(os.getuid()) + ':' + str(os.getgid()) + ' ' + shlex.quote(str(root))
        script += ' /usr/bin/env -i PATH=/usr/local/bin:/usr/bin:/bin HOME=/tmp PYTHONDONTWRITEBYTECODE=1 GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null '
        script += 'F13_HARNESS_IDENTITY=' + shlex.quote(harness_identity) + ' '
        script += shlex.quote(sys.executable) + ' -B ' + executable + ' --inside\n'
        # User/mount/network/pid isolation is fixture infrastructure only. There
        # is no listener, published port, engine start, DEV or STAGING operation.
        result = subprocess.run(['sudo', '-n', 'unshare', '--mount', '--net', '--pid', '--fork', 'bash', '-c', script], capture_output=True)
        sys.stdout.buffer.write(result.stdout)
        sys.stderr.buffer.write(result.stderr)
        receipts = []
        for line in result.stdout.splitlines():
            try:
                value = json.loads(line)
            except ValueError:
                continue
            if isinstance(value, dict) and value.get('kind') == 'FAITHFUL_HISTORICAL_SHARED_GUARD':
                receipts.append(value)
        # sudo/unshare/chroot exit1 is not the contract assertion RED.
        require(len(receipts) == 1 and result.returncode in (0, 1) and
            receipts[0]['guard_observed'] == result.returncode and receipts[0]['contract_expected'] == 0,
            'namespace_must_return_actual_guard_receipt_not_setup_exit')
        return result.returncode


def historical_module():
    spec = importlib.util.spec_from_file_location('historical547', '/repo/scripts/ci/f13_f54_history.py')
    h = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(h)
    return h


def inside():
    require(sys.platform == 'linux' and ROOT == Path('/harness'), 'private_namespace_required')
    harness_identity = json.loads(os.environ['F13_HARNESS_IDENTITY'])
    require(re.fullmatch('[0-9a-f]{40}', harness_identity['commit']), 'explicit_frozen_testhead')
    require(set(harness_identity['files']) == set(HARNESS_SOURCE_PATHS), 'complete_testhead_source_binding')
    require(all(sha((ROOT / name).read_bytes()) == digest for name, digest in harness_identity['files'].items()), 'mounted_testhead_same_bytes')
    repo = Path('/repo')
    bundle = '/out/prelaunch/private-target.bundle'
    run(['git', 'clone', '--no-checkout', bundle, str(repo)])
    run(['git', '-C', str(repo), 'checkout', '--detach', TARGET])
    git = lambda *args: run(['git', '--no-replace-objects', '-C', str(repo), *args])
    require(git('rev-list', '--parents', '-n', '1', 'HEAD').decode().split() == [TARGET, BASE, SOURCE], 'historical_actual_parents')
    for name, digest in OUTER_SOURCE_HASHES.items():
        require(sha((repo / name).read_bytes()) == digest and (repo / name).read_bytes() == git('show', SOURCE + ':' + name), 'historical_source:' + name)
    unit_raw = (FIXTURE / 'source-unit.json').read_bytes()
    require(sha(unit_raw) == 'e8fce2a60ae0975ddcaee47db3cf36a51e57744194f5885bb8953ee2406a8f02', 'c69_factual_source_unit')
    unit = json.loads(unit_raw)
    require(unit['path'] == PATH and len(unit['versions']) == 12 and len(unit['edges']) == 6, 'whole_exact_J_source_unit')
    for version in unit['versions'].values():
        require(git('ls-tree', '-z', version['ref'], '--', PATH).decode() == version['tree_record'], 'actual_source_mode_type_oid')
        blob = git('show', version['ref'] + ':' + PATH)
        require(len(blob) == version['bytes'] and sha(blob) == version['sha256'], 'actual_source_content')
        require(git('rev-list', '--parents', '-n', '1', version['ref']).decode().split()[1:] == version['ordered_parents'], 'actual_source_ordered_parents')
    for edge in unit['edges'].values():
        require(sha(git('diff', '--raw', '--no-abbrev', '--no-renames', edge['from'], edge['to'], '--', PATH)) == edge['raw_sha256'], 'actual_full_source_edge')
        require(sha(git('diff', '--no-ext-diff', '--no-renames', '--abbrev=9', edge['from'], edge['to'], '--', PATH)) == edge['patch_sha256'], 'actual_full_source_patch')
    h = historical_module()
    trace = OBSERVED / 'trace.jsonl'
    raw = trace.read_bytes()
    require(sha(raw) == TRACE_SHA, 'actual_full_failed_trace')
    events = [json.loads(line) for line in raw.splitlines()]
    require(len(events) == 10648 and sha(b''.join(raw.splitlines(keepends=True)[:10645])) == PENDING_SHA, 'actual_pending_prefix')
    pending = events[10644]
    require(pending['kind'] == 'row_before' and pending['operation'] == OPERATION and pending['payload']['path'] == PATH, 'actual_same_predicate_pending')
    context = next(e['payload']['spec']['gate_context'] for e in events if e['kind'] == 'manual_F_begin')
    os.environ.update(context['event'])
    # No authority is supplied by a self-consistent captured pathlist/hash.
    expected = h.manual_authority_expected(repo, context)
    h.validate_manual_authority(context, expected)
    captured = json.loads((OBSERVED / 'fixed3071-J-ledger.json').read_bytes())
    require(sha((OBSERVED / 'fixed3071-J-ledger.json').read_bytes()) == LEDGER_SHA, 'captured_ledger_raw')
    prefix = captured['prefix']
    checked = h.validate_trace(repo, trace, prefix['sha256'], J_checkpoint=OPERATION, sealed_prefix=prefix)
    reconstructed = h.build_3071_J_ready(checked['J_checkpoint'], prefix)
    require(h.canonical(reconstructed) == h.canonical(captured), 'independent_complete_J_not_trusted_import')
    # Fold the complete original failed trace to prove all prior transitions and
    # the exact final map, not just the old J checkpoint or its selected row.
    terminal = h.validate_trace(repo, trace, TRACE_SHA)
    require(terminal['failures'] and terminal['master_wrapper'] is None, 'old_failure_not_success')
    folded = terminal['ledger']['F5']
    map_raw = ''.join(row + '\n' for row in folded['rows']).encode()
    require(sha(map_raw) == MAP_SHA and h.file_digest(map_raw) == pending['payload']['map_before'], 'full_actual_pending_map')
    source_map = OBSERVED / 'f13-nested-map-e6757e8ac26032ac4bad7f4356e91fffbf0725ba-2'
    require(source_map.read_bytes() == map_raw, 'full_map_bytes_not_selected_rows')
    child = next(e['frame'] for e in events if e['kind'] == 'frame_enter' and e['payload']['parent_operation'] == OPERATION)
    nested_rows = [r.split('\t') for r in reconstructed['ledger'][child]['rows'] if r.split('\t')[0] == PATH]
    require(len(nested_rows) == 1, 'actual_child_owner_not_incoming_commit_guess')
    nested = nested_rows[0]
    incoming = pending['payload']['incoming']['raw'].split('\t')
    prior = pending['payload']['prior']['raw'].split('\t')
    require(pending['payload']['prior']['sha256'] == 'bdcd1df8603081f14a80d5627616466f7ede36439915a193e62345d177af8ee1' and
        pending['payload']['incoming']['sha256'] == '51a6d356845a23ab3b4f1e7d362075a284d55aea9d3fb907e7d0ec00a62fb13d', 'c69_literal_actual_eight_field_rows')
    require([r for r in folded['rows'] if r.split('\t')[0] == PATH] == [pending['payload']['prior']['raw']], 'complete_prior_rows')
    require(incoming[4:6] == nested[4:6] and incoming[1] == nested[1], 'original_child_flags_source')
    frame = next(x['context'] for x in reconstructed['frame_ancestry'] if x['frame'] == 'F5')
    op = reconstructed['operation_binding']
    require(op['classification'] == 'merge' and frame['partition_source_history'] == 'true', 'actual_merge_partition')
    require(h.classify(reconstructed['classification']['unapproved_delta']['raw'].splitlines()) == reconstructed['classification']['trigger'], 'actual_full_classifier_recomputed')
    require(PATH not in json.loads(expected['files'][context['values']['native_p3_binding']]['raw'])['paths'], 'non_native_audit_actual_membership')
    require(not expected['values']['game_checkpoint_aggregate_active'] == 'true' or PATH not in expected['files'][expected['values']['game_checkpoint_aggregate_paths']]['raw'].splitlines(), 'non_Game_audit_actual_membership')
    entry = lambda commit: git('ls-tree', commit, '--', PATH).decode().strip()
    require(entry(op['source_head']) == entry(op['commit']), 'ordinary_merge_shortcut_actual_Git')
    # This permits the original ordinary_guarded_merge=false and outcome
    # unverified defaults; no fabricated merge qualification is injected.
    values = dict(expected['values'], **context['values'])
    require(all(values[k] == v for k, v in expected['values'].items()), 'same_independent_authority_values')
    values.update(repo='/repo', path=PATH, output=str(source_map), base=frame['base'], target=frame['target'],
        commit=op['commit'], first_parent=op['first_parent'], source_base=op['boundary'], source_head=op['source_head'],
        nested_owner_commit=nested[2], nested_base=nested[4], nested_head=nested[5], source_entry=entry(op['source_head']),
        content_base=incoming[6], source_base_candidates=','.join(op['candidates']), trigger=incoming[1],
        allow_direct_followup=frame['allow_direct_followup'], is_direct='false', ordinary_guarded_merge='false',
        source_lineage='false', guarded_correction_allowed='false', role_duplicate_allowed='false', space_audit_allowed='false',
        exact_unit_supersession='false', native_supersession_status='1', f13_f54_operation=OPERATION,
        f13_f54_J_operation=OPERATION, f13_f54_J_ledger=str(OBSERVED / 'fixed3071-J-ledger.json'),
        f13_f54_J_ledger_sha=LEDGER_SHA,
        f13_f54_capability_file=str(OBSERVED / 'f54-exact-source-capabilities.json'),
        f13_f54_3071_capability_file=str(OBSERVED / 'fixed3071-structural-capabilities.json'),
        f13_f54_observer='/repo/scripts/ci/f13_f54_history.py', f13_f54_trace_file=str(trace),
        diagnostic_file='/tmp/shared-guard-unexpected-failure')
    M_operations = [e['operation'] for e in events[:10645] if e['kind'] == 'M_ledger_ready']
    # Missing or surprising dynamic state is not silently replaced with a
    # convenient empty M flag. The original M receipt supplies the identity.
    require(len(M_operations) == 1 and M_operations[0] in reconstructed['inherited_M_ledgers'], 'actual_retained_M_dynamic_identity')
    M_binding = next(e['payload']['ledger'] for e in events if e['kind'] == 'M_ledger_ready')
    require(M_binding == h.raw_binding(M_binding['raw']) and
        json.loads(M_binding['raw']) == reconstructed['inherited_M_ledgers'][M_operations[0]], 'actual_reconstructed_M_raw_record')
    M_path = OBSERVED / 'f54-actual-M-ledger.json'
    require(not M_path.exists(), 'no_unbound_M_file_overwrite')
    # Lossless materialization of already validated raw event transport, not a
    # newly assembled/trusted ledger or producer operation.
    M_path.write_bytes(M_binding['raw'].encode())
    values.update(f13_f54_M_operation=M_operations[0],
        f13_f54_M_ledger=str(M_path), f13_f54_M_ledger_sha=M_binding['sha256'])
    new_source = (ROOT / 'scripts/ci/voice-db-runtime-provisioning-contract_test.sh').read_text()
    definitions = h.original_policy_functions(new_source)
    definitions += '\n' + (repo / 'scripts/ci/voice-r22-runtime-scope.sh').read_text()
    # A private function frame supplies independently reconstructed dynamic
    # scope to the SAME two functions used by the real collector. It is not a
    # live collector frame and cannot append/restart any producer operation.
    script = 'set -uo pipefail\n' + definitions + '\nfixture_call() {\n'
    for function in ['f13_evaluate_guarded_owner_qualifiers', 'f13_guarded_owner_transition_allowed',
        'f13_path_is_guarded', 'game_checkpoint_duplicate_owner_supersession_allowed',
        'f13_guarded_correction_qualified', 'native_p3_path_bound', 'native_p3_duplicate_owner_qualified',
        'f13_role_conditional_qualified', 'f13_space_audit_qualified', 'f13_f54_exact_supersession']:
        script += 'declare -F ' + function + ' >/dev/null || return 92\n'
    for key, value in values.items():
        require(re.fullmatch('[A-Za-z_][A-Za-z0-9_]*', key), 'closed_variable_name')
        script += 'local ' + key + '=' + shlex.quote(value) + '\n'
    script += 'local -A owners owner_triggers owner_content_bases owner_source_heads owner_source_entries guarded_entries guarded_entry_commits checkpoint_merge_outcomes\n'
    for key, value in dict(owners=prior[2],owner_triggers=prior[1],owner_content_bases=prior[6],owner_source_heads=prior[5],owner_source_entries=entry(prior[5]),guarded_entries=entry(op['source_head']),guarded_entry_commits=nested[2],checkpoint_merge_outcomes='unverified').items():
        script += key + '[' + shlex.quote(PATH) + ']=' + shlex.quote(value) + '\n'
    script += 'f13_path_is_guarded "$path" || return 90\n'
    script += 'f13_evaluate_guarded_owner_qualifiers "$repo" "$path" "$output" || return 91\n'
    script += 'status=0; f13_guarded_owner_transition_allowed "$guarded_correction_allowed" "$role_duplicate_allowed" "$space_audit_allowed" "$exact_unit_supersession" "$allow_direct_followup" "$is_direct" "$incremental_merge" || status=$?\n'
    script += 'printf "F13_SHARED_GUARD_STATUS=%s\\n" "$status"\n}\nfixture_call\n'
    before = {p: sha((OBSERVED / p).read_bytes()) for p in ['trace.jsonl', 'fixed3071-J-ledger.json', source_map.name]}
    before_files = {str(p):sha(p.read_bytes()) for p in OBSERVED.rglob('*') if p.is_file()}
    result = subprocess.run(['bash'], input=script.encode(), capture_output=True)
    require(result.returncode == 0 and not Path(values['diagnostic_file']).exists(), 'shared_dispatch_setup:' + result.stderr.decode(errors='replace')[-2000:])
    require(all(sha((OBSERVED / name).read_bytes()) == digest for name, digest in before.items()), 'no_trace_ledger_map_write')
    require(before_files == {str(p):sha(p.read_bytes()) for p in OBSERVED.rglob('*') if p.is_file()}, 'unexpected_native_or_other_side_effect_is_setup_failure')
    matches = re.findall(rb'^F13_SHARED_GUARD_STATUS=([01])$', result.stdout, re.M)
    require(len(matches) == 1, 'explicit_shared_guard_status')
    observed = int(matches[0])
    print(json.dumps(dict(kind='FAITHFUL_HISTORICAL_SHARED_GUARD',historical_source=SOURCE,target=TARGET,event_base=BASE,
        pending_sequence=10645,pending_prefix_sha256=PENDING_SHA,map_sha256=MAP_SHA,ledger_sha256=LEDGER_SHA,
        guard_observed=observed,contract_expected=0,source_hashes={p:sha((ROOT/p).read_bytes())for p in OUTER_SOURCE_HASHES},
        harness_identity=harness_identity,archive_sha256=ARCHIVE_SHA,
        producer_executed=False,full_F13='NOT_RUN',admission=False)))
    if observed != 0:
        print('ASSERTION RED: accepted exact-J contract expects qualified transition; unchanged shared production guard refused', file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    try:
        raise SystemExit(inside() if sys.argv[1:] == ['--inside'] else namespace_main())
    except Exception as error:
        print('F13_HISTORICAL_SETUP_FAILURE: ' + type(error).__name__ + ': ' + str(error), file=sys.stderr)
        raise SystemExit(2)
