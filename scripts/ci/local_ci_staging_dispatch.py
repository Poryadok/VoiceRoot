"""Generate serial Make dispatchers and validate exact source-bound receipts."""
import ast
import hashlib
import re
import shlex

ROOT = '/workspace/repo/'
PRESERVATION = 'scripts/ci/nats-rollout-preservation_test.sh'
PHASE0_SOURCE_SHA256 = '3f7fde41dd779bfeb712265a475493160a47ecc748264d1f4505d369c8e70a49'
PHASE0_ROOT_TEST = '__main__.RestorerPathTests.test_cleanup_restores_service_owned_fixture_for_real_caller_uid'
PHASE0_USER_TEST = '__main__.FixtureTests.test_general_fixture_inputs_remain_readable_by_the_test_user'
PHASE0_SKIP_REASON = 'requires Linux root to exercise real service and caller UIDs'


def validate_phase0_proof(root, suite, proof):
    """Bind the hosted UID-1000 suite's sole skip to its isolated root pass."""
    source = (root / 'scripts/ci/phase0-fixture_test.py').read_bytes()
    if hashlib.sha256(source).hexdigest() != PHASE0_SOURCE_SHA256:
        raise ValueError('phase0 test source differs')
    tree = ast.parse(source)
    expected = {f'__main__.{cls.name}.{method.name}'.encode() for cls in tree.body if isinstance(cls, ast.ClassDef)
                for method in cls.body if isinstance(method, ast.FunctionDef) and method.name.startswith('test_')}
    if len(expected) != 78:
        raise ValueError('phase0 test inventory differs')
    def rows(data):
        if not isinstance(data, bytes) or len(data) > 16 * 1024 * 1024:
            raise ValueError('phase0 output exceeds limit')
        events = re.findall(rb'(?m)^test_[^\r\n]* \((__main__\.[A-Za-z0-9_.]+)\) \.\.\. (ok|skipped \'[^\r\n]*\')\r?$', data)
        if len(events) != len({event[0] for event in events}):
            raise ValueError('duplicate phase0 test')
        return dict(events)
    observed = rows(suite)
    root_result = rows(proof)
    expected_skip = ("skipped '" + PHASE0_SKIP_REASON + "'").encode()
    if (set(observed) != expected or observed.get(PHASE0_ROOT_TEST.encode()) != expected_skip or
            observed.get(PHASE0_USER_TEST.encode()) != b'ok' or
            sum(result != b'ok' for result in observed.values()) != 1 or
            root_result != {PHASE0_ROOT_TEST.encode(): b'ok'} or
            len(re.findall(rb'(?m)^Ran 78 tests in [^\r\n]+\r?$', suite)) != 1 or
            len(re.findall(rb'(?m)^OK \(skipped=1\)\r?$', suite)) != 1 or
            len(re.findall(rb'(?m)^Ran 1 test in [^\r\n]+\r?$', proof)) != 1 or
            len(re.findall(rb'(?m)^OK\r?$', proof)) != 1 or
            re.search(rb'(?m)^(?:FAIL|ERROR)', suite + b'\n' + proof)):
        raise ValueError('phase0 complementary proof incomplete')


def story_media_adapter(source, dockerfile, docker_cli_id, python_id):
    """Adapt the exact runtime fixture to preloaded private images and socket."""
    if (not all(isinstance(value, str) and re.fullmatch(r'sha256:[0-9a-f]{64}', value)
                for value in (docker_cli_id, python_id)) or
            hashlib.sha256(source).hexdigest() != '3183b5add1c8be07197e2446f1228e53aafc07d9f83b0e231ffded63993bfd9f' or
            hashlib.sha256(dockerfile).hexdigest() != '43f3566e9bdcfa28c77b7492cab1bda5da7af5c4a06a65081c1f148726bed641'):
        raise ValueError('story-media fixture source differs')
    text = source.decode('utf-8')
    replacements = (
        ('ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"', 'ROOT="/workspace/repo"'),
        ("postgres='postgres@sha256:cf78e76683b9ca8c5733cbbdce6c9262b45b6767934dd0a95e671f9a0fc20685'\ndocker pull --platform linux/amd64 \"$postgres\"",
         "postgres='sha256:75f5a96988cdf694a215073c3e9c001b706b371e2f94df3967f2efdec2787f6b'"),
        ('docker build --platform linux/amd64',
         '[[ "$(docker image inspect docker:27-cli --format \'{{.Id}}\')" == ' + docker_cli_id + ' ]]\n'
         '[[ "$(docker image inspect python:3.12-bookworm --format \'{{.Id}}\')" == ' + python_id + ' ]]\n'
         'docker build --network none --pull=false --platform linux/amd64'),
        ('-v /var/run/docker.sock:/var/run/docker.sock -v "$ROOT:/repo:ro"',
         '-v /run/user/1000/docker.sock:/var/run/docker.sock -v /run/user/1000/source-mirror:/repo:ro'),
    )
    for old, new in replacements:
        if text.count(old) != 1: raise ValueError('story-media fixture shape differs')
        text = text.replace(old, new, 1)
    return text.encode('utf-8')

def inventory(rows, *, shard='staging'):
    if shard not in ('staging', 'proofs', 'runtime', 'policy'):
        raise ValueError('unsupported shard')
    result = []
    for row in rows:
        recipe = row['recipe']
        match = re.fullmatch(r'(\$\(BASH\)|python3) "\$\(ROOT\)/([^"\s]+)"', recipe)
        if not match:
            discovery = re.fullmatch(r'python3 -m unittest discover -s "\$\(ROOT\)/([^"\s]+)" -p ([A-Za-z0-9_.-]+)', recipe)
            if shard != 'runtime' or not discovery:
                raise ValueError('unsupported shard recipe')
            path, pattern = discovery.groups()
            if '..' in path.split('/') or not re.fullmatch(r'[A-Za-z0-9_./-]+', path) or '..' in pattern:
                raise ValueError('unsafe discovery recipe')
            result.append(('discover', (path, pattern)))
            continue
        kind, path = match.groups()
        if '..' in path.split('/') or not re.fullmatch(r'[A-Za-z0-9_./-]+', path):
            raise ValueError('unsafe recipe path')
        result.append(('bash' if kind == '$(BASH)' else 'python', path))
    if (len(result) != {'staging': 34, 'proofs': 15, 'runtime': 23, 'policy': 73}[shard] or len(set(result)) != len(result) or
            result.count(('bash', PRESERVATION)) != (1 if shard == 'staging' else 0)):
        raise ValueError('shard recipe inventory differs')
    return result

def command(recipe):
    kind, path = recipe
    if kind == 'discover':
        directory, pattern = path
        return ['python3', '-m', 'unittest', 'discover', '-s', ROOT + directory, '-p', pattern]
    if kind not in ('bash', 'python'):
        raise ValueError('unsupported recipe kind')
    return [('/run/user/1000/adapter-root/bash-guard' if kind == 'bash' else 'python3'), ROOT + path]

def wrappers(recipes, *, shard='staging'):
    if shard not in ('staging', 'proofs', 'runtime', 'policy') or len(recipes) != {'staging': 34, 'proofs': 15, 'runtime': 23, 'policy': 73}[shard]:
        raise ValueError('shard recipe count differs')
    result = {}
    for kind in ('bash','python'):
        shell = ['#!/bin/bash', 'set -u -o pipefail', 'case "${1-}" in']
        for index, (entry_kind,path) in enumerate(recipes):
            if kind == entry_kind:
                shell.append('  ' + shlex.quote(ROOT+path) + ') idx=' + str(index) + ' ;;')
        binary = '/bin/bash' if kind == 'bash' else '/usr/local/bin/python3'
        discovered = [(index, path) for index, (entry_kind, path) in enumerate(recipes) if entry_kind == 'discover']
        if kind == 'python' and discovered:
            shell.append('  -m)')
            shell.append('    if [ "$#" = 7 ] && [ "$2" = unittest ] && [ "$3" = discover ] && [ "$4" = -s ] && [ "$6" = -p ]; then')
            for number, (index, (directory, pattern)) in enumerate(discovered):
                shell.append('      ' + ('if' if number == 0 else 'elif') + ' [ "$5" = ' + shlex.quote(ROOT+directory) + ' ] && [ "$7" = ' + shlex.quote(pattern) + ' ]; then idx=' + str(index))
            shell.extend(['      else exec '+binary+' "$@"; fi', '    else exec '+binary+' "$@"; fi ;;'])
        shell.extend(['  *) exec '+binary+' "$@" ;;', 'esac',
            'printf "%s\\tstart\\n" "$idx" >> /run/user/1000/recipe-events',
            'rc=0'])
        if shard == 'policy': shell.append('export VOICE_CI_RECIPE_INDEX="$idx"')
        pipe = ' 2>&1 | tee "/run/user/1000/recipe-${idx}.log" | head -c 16777217'
        if kind == 'bash' and shard == 'staging':
            shell.append('if [ "$1" = '+shlex.quote(ROOT+PRESERVATION)+' ]; then '+binary+' "$@" --owned-temp-root /run/user/1000/fixture-root'+pipe+'; codes=("${PIPESTATUS[@]}"); else '+binary+' "$@"'+pipe+'; codes=("${PIPESTATUS[@]}"); fi')
        elif kind == 'bash' and shard == 'runtime':
            shell.append('if [ "$1" = '+shlex.quote(ROOT+'scripts/ci/story-media-cohort-chain.sh')+' ]; then '+binary+' /run/user/1000/adapter-root/story-media.sh "${@:2}"'+pipe+'; codes=("${PIPESTATUS[@]}"); else '+binary+' "$@"'+pipe+'; codes=("${PIPESTATUS[@]}"); fi')
        else:
            shell.append(binary+' "$@"'+pipe+'; codes=("${PIPESTATUS[@]}")')
        shell.append('rc=${codes[0]}; if [ "${codes[1]}" != 0 ] || [ "${codes[2]}" != 0 ]; then rc=125; fi')
        shell.extend(['printf "%s\\texit\\t%s\\n" "$idx" "$rc" >> /run/user/1000/recipe-events', 'exit "$rc"'])
        result[kind] = ('\n'.join(shell)+'\n').encode()
    return result

def validate_events(data, recipes):
    if not data.endswith(b'\n'):
        raise ValueError('staging recipe event stream truncated')
    lines = data.decode('ascii').splitlines()
    expected = []
    for index in range(len(recipes)):
        expected.extend((f'{index}\tstart', f'{index}\texit\t0'))
    if lines != expected:
        raise ValueError('staging recipe execution incomplete or out of order')
    return len(recipes)

def validate_make_log(data, *, shard='staging', phase0_proof_verified=False):
    if len(data) > 32*1024*1024:
        raise ValueError('staging Make output exceeded limit')
    if shard == 'runtime' and phase0_proof_verified:
        skip_line = (b'test_cleanup_restores_service_owned_fixture_for_real_caller_uid (' +
                     PHASE0_ROOT_TEST.encode() + b") ... skipped '" + PHASE0_SKIP_REASON.encode() + b"'")
        if data.splitlines().count(skip_line) != 1 or data.splitlines().count(b'OK (skipped=1)') != 1:
            raise ValueError('phase0 hosted disposition differs')
        data = b'\n'.join(line for line in data.splitlines() if line not in (skip_line, b'OK (skipped=1)'))
    if shard != 'policy' and re.search(rb'(?i)\bskip(?:ped)?\b', data):
        raise ValueError('staging Make output skipped or exceeded limit')

MARKERS = (
    'Full rollout restores Auth before User.', 'kubectl ConfigMap compatibility tests passed.',
    'Staging mail preflight workflow ordering passed.', 'Staging Resend preflight contract passed.', None,
    'staging app Secret merge tests passed.', 'Mail-only patch contract passed.', 'Mail-only staging contract passed.',
    'Staging User/File MinIO env contract passed.', 'ensure-app-secrets required Auth secret regression tests passed.',
    'kubectl Secret bootstrap compatibility tests passed.', 'ensure-minio-credentials tests passed',
    'restore-principal-secrets tests passed', 'restore-nats-secrets tests passed',
    'ensure-user-search-projection-secret tests passed', 'Observability Secret compatibility tests passed.',
    'Secret manifest privacy and cleanup tests passed.', 'NATS leaf topology gate regression tests passed.',
    'nats-search-probe-workflow: PASS', 'staging source archive workflow fixtures passed', None,
    'staging NATS bootstrap policy: OK', 'staging NATS generation contract: OK',
    'staging NATS generation restore contract: OK', 'staging NATS root rotation contract: OK',
    'staging NATS live ACL proof contract: OK', 'NATS_ROOT_DIAGNOSE_CONTRACT=PASS',
    'NATS_LEGACY_RECOVERY_CONTRACT=PASS', 'NATS JWT rollout contract passed.',
    'PASS: hosted JWT resolver + TLS leaf + exact ACL proof',
    'PASS: canonical hosted JWT/TLS bootstrap and least-privilege ACL proof', None,
    'All setup-github-runner tests passed.', 'staging NATS diagnostic workflow contract passed',
)

PROOFS_MARKERS = (
    'Missing Docker Go module download helper test passed.', 'All e2e-manifest tests passed.', None,
    'compose_fcm_diag_contract=pass', 'All compose-e2e-smoke tests passed.', None, None,
    'Compose E2E live phase tests passed.', 'Compose NATS JetStream gate regression tests passed.',
    'All compose-a1-multi-account-proof tests passed.',
    'All compose-a1-flutter-profile-handoff tests passed.',
    'All compose-file-attachment-restart-proof tests passed.', None,
    'A1 attachment restart CI reachability contract passed.',
    'A1 Flutter profile-handoff CI reachability contract passed.',
)

RUNTIME_MARKERS = (
    'Voice R22 runtime scope regressions: PASS', None,
    'Voice DB F05-F12 current provisioning/docs contract: PASS', 'apply-migrate-jobs contract tests passed.',
    'A4 disposable recovery harness contract: PASS', None, None, None, None, None, None, None,
    None, None, None, None, None, None, None, None,
    'Production ClickHouse init: 7 cases passed (isolated kubectl fixture).',
    'Production smoke offline contract: 9 cases passed.',
    'Production User/Space rollout: 7 cases passed (isolated shell fixture).',
)

# Only these exact Linux dispositions are accepted; the pinned files must be
# re-audited if their tests or decorators change. This is not Windows coverage.
POLICY_SKIP_FILES = {
    21: ('scripts/ci/local_source_snapshot_test.py', 'bdaf3b29b56dc2e879e00c1219a4dc9d7c736068ecb578451a8209b626dbb159', 1),
    24: ('scripts/ci/local_process_supervision_test.py', '371cab90b5f4693f213746590bad143ee8d71737cf3472e935579f54596dcd05', 33),
    33: ('scripts/ci/local_ci_routing_test.py', 'e5da4335ed1975d4eefeccbe4a79f09258f033e6b20a8ef640f44e89e6294c5f', 1),
    41: ('scripts/ci/local_nats_hosted_diagnostic_test.py', '1b49039756b9aabd7c7963ec22f86a0acd2591b547993fc52e4ee150fbb8a5d6', 2),
}
POLICY_SHELL_MARKERS = {
    1: 'All resolve-staging-matrix tests passed.', 2: 'All resolve-go-matrix tests passed.',
    3: 'All verify-required-jobs tests passed.', 5: '', 6: None,
    55: 'unscoped Testcontainers cleanup fails closed', 63: 'CI script tests are reachable from Actions.',
    64: 'All Auth Testcontainers report gate tests passed.', 65: 'Auth proto sync guard regression tests passed.',
    66: 'PASS: A1 proof builds and validates the pinned official mc runtime before Compose.',
    67: 'CI MinIO Compose image override contract passed',
    68: 'PASS: staging MinIO bucket Jobs are replaced only for known completed image drift.',
}


def policy_expected_skips(root, rows):
    result = {}
    for index, (name, digest, count) in POLICY_SKIP_FILES.items():
        if index >= len(rows) or name not in rows[index].get('source_sha256', {}):
            raise ValueError('policy skip recipe changed')
        data = (root / name).read_bytes()
        if hashlib.sha256(data).hexdigest() != digest:
            raise ValueError('policy skip source changed')
        found = set()
        for cls in (node for node in ast.walk(ast.parse(data)) if isinstance(node, ast.ClassDef)):
            for method in (node for node in cls.body if isinstance(node, ast.FunctionDef)):
                for decorator in method.decorator_list:
                    if (isinstance(decorator, ast.Call) and isinstance(decorator.func, ast.Attribute) and
                            decorator.func.attr == 'skipUnless' and len(decorator.args) == 2 and
                            isinstance(decorator.args[1], ast.Constant) and
                            ast.unparse(decorator.args[0]) == "os.name == 'nt'"):
                        found.add(('__main__.' + cls.name + '.' + method.name, decorator.args[1].value))
        if len(found) != count: raise ValueError('policy skip inventory changed')
        result[index] = found
    return result


def policy_skip_logger():
    """Instrument unittest skips in the private policy client only."""
    return b'''import json, os, unittest
original = unittest.result.TestResult.addSkip
def add_skip(self, test, reason):
    index = os.environ.get('VOICE_CI_RECIPE_INDEX', '')
    if not index.isdigit() or not 0 <= int(index) <= 73:
        raise RuntimeError('invalid policy skip recipe identity')
    data = (json.dumps({'recipe': int(index), 'id': test.id(), 'reason': str(reason)}, separators=(',', ':')) + '\\n').encode('utf-8')
    if len(data) > 4096:
        raise RuntimeError('policy skip event too large')
    fd = os.open(os.environ['VOICE_CI_SKIP_LOG'], os.O_WRONLY | os.O_APPEND)
    try:
        if os.write(fd, data) != len(data):
            raise RuntimeError('short policy skip event write')
    finally:
        os.close(fd)
    return original(self, test, reason)
unittest.result.TestResult.addSkip = add_skip
'''


def parse_policy_skip_events(data):
    import json
    if len(data) > 1024 * 1024 or (data and not data.endswith(b'\n')):
        raise ValueError('policy skip log too large or truncated')
    def unique(pairs):
        obj = {}
        for key, value in pairs:
            if key in obj: raise ValueError('duplicate policy skip field')
            obj[key] = value
        return obj
    result = {}
    for line in data.splitlines():
        row = json.loads(line, object_pairs_hook=unique)
        if (not isinstance(row, dict) or set(row) != {'recipe', 'id', 'reason'} or
                type(row['recipe']) is not int or not 0 <= row['recipe'] <= 73 or
                any(not isinstance(row[key], str) or not 0 < len(row[key]) <= 512 for key in ('id', 'reason'))):
            raise ValueError('invalid policy skip event')
        item = (row['id'], row['reason'])
        entries = result.setdefault(row['recipe'], set())
        if item in entries: raise ValueError('duplicate policy skip event')
        entries.add(item)
    return result


def validate_policy_recipe(index, data, observed, expected):
    if type(index) is not int or not 0 <= index < 73 or len(data) > 16 * 1024 * 1024:
        raise ValueError('policy recipe identity or output size invalid')
    if re.search(rb'(?im)^\s*skip(?:ped)?(?:\s|:|\()', data):
        raise ValueError('unreviewed policy skip marker')
    if observed != expected.get(index, set()):
        raise ValueError('policy skip identities differ')
    if index in POLICY_SHELL_MARKERS:
        if observed: raise ValueError('shell recipe unexpectedly skipped')
        marker = POLICY_SHELL_MARKERS[index]
        if marker == '' and data: raise ValueError('silent policy recipe produced output')
        if marker is None and not re.search(rb'(?m)^image-catalog drift check OK \([^\r\n]+\)\r?$', data):
            raise ValueError('image catalog marker missing')
        if marker and data.splitlines().count(marker.encode()) != 1: raise ValueError('policy marker missing or duplicated')
        return
    summaries = re.findall(rb'^Ran ([0-9]+) tests? in [^\r\n]+\r?$', data, flags=re.MULTILINE)
    results = re.findall(rb'^OK(?: \(skipped=([0-9]+)\))?\r?$', data, flags=re.MULTILINE)
    if len(summaries) != 1 or len(results) != 1 or int(summaries[0]) <= len(observed) or int(results[0] or b'0') != len(observed):
        raise ValueError('policy unittest summary or skip count differs')

def validate_recipe(index, data, *, shard='staging'):
    if shard not in ('staging', 'proofs', 'runtime'): raise ValueError('unsupported shard')
    markers = {'staging': MARKERS, 'proofs': PROOFS_MARKERS, 'runtime': RUNTIME_MARKERS}[shard]
    if type(index) is not int or not 0 <= index < len(markers) or len(data) > 16*1024*1024:
        raise ValueError('recipe identity or output size invalid')
    if re.search(rb'(?i)\bskip(?:ped)?\b', data): raise ValueError('recipe skipped')
    marker = markers[index]
    if marker is not None:
        if data.splitlines().count(marker.encode()) != 1: raise ValueError('recipe marker missing or duplicated')
        if shard != 'runtime' or index != 22: return
    summaries = re.findall(rb'^Ran ([0-9]+) tests? in [^\r\n]+\r?$', data, flags=re.MULTILINE)
    oks = re.findall(rb'^OK\r?$', data, flags=re.MULTILINE)
    expected = 3 if shard == 'staging' and index == 20 else 1
    if len(summaries) != expected or len(oks) != expected or any(int(x) <= 0 for x in summaries):
        raise ValueError('recipe test summaries incomplete')
    if shard == 'staging' and index == 20 and data.splitlines().count(b'NATS_ROLLOUT_PRESERVATION=PASS valid-receipt-paused-apply-and-second-use-veto-all-modes') != 1:
        raise ValueError('preservation marker missing')

def validate_go_json(data, packages):
    import json
    if len(data) > 16*1024*1024: raise ValueError('Go output exceeded limit')
    rows = [json.loads(line) for line in data.splitlines() if line.strip()]
    if not packages or len(set(packages)) != len(packages): raise ValueError('Go package inventory invalid')
    expected = set(packages); starts = set(); ends = set(); running = set(); passed = set()
    for row in rows:
        if not isinstance(row,dict) or row.get('Package') not in expected: raise ValueError('unexpected Go package event')
        action, package, test = row.get('Action'), row['Package'], row.get('Test')
        if action in ('skip','fail'): raise ValueError('Go tests skipped or failed')
        if action == 'start' and not test:
            if package in starts: raise ValueError('duplicate Go package start')
            starts.add(package)
        elif action == 'run' and test:
            key=(package,test)
            if package not in starts or package in ends or key in running: raise ValueError('invalid Go test start')
            running.add(key)
        elif action == 'pass' and test:
            key=(package,test)
            if package in ends or key not in running or key in passed: raise ValueError('invalid Go test pass')
            passed.add(key)
        elif action == 'pass' and not test:
            if package not in starts or package in ends: raise ValueError('invalid Go package pass')
            ends.add(package)
        elif action not in ('output','pause','cont'):
            raise ValueError('unknown Go test event')
    if starts != expected or ends != expected or not running or passed != running:
        raise ValueError('Go test lifecycle incomplete')
    return len(passed)
