"""Generate serial Make dispatchers and validate exact source-bound receipts."""
import re
import shlex

ROOT = '/workspace/repo/'
PRESERVATION = 'scripts/ci/nats-rollout-preservation_test.sh'

def inventory(rows, *, shard='staging'):
    if shard not in ('staging', 'proofs'):
        raise ValueError('unsupported shard')
    result = []
    for row in rows:
        recipe = row['recipe']
        match = re.fullmatch(r'(\$\(BASH\)|python3) "\$\(ROOT\)/([^"\s]+)"', recipe)
        if not match:
            raise ValueError('unsupported staging recipe')
        kind, path = match.groups()
        if '..' in path.split('/') or not re.fullmatch(r'[A-Za-z0-9_./-]+', path):
            raise ValueError('unsafe recipe path')
        result.append(('bash' if kind == '$(BASH)' else 'python', path))
    if (len(result) != {'staging': 34, 'proofs': 15}[shard] or len(set(result)) != len(result) or
            result.count(('bash', PRESERVATION)) != (1 if shard == 'staging' else 0)):
        raise ValueError('shard recipe inventory differs')
    return result

def wrappers(recipes, *, shard='staging'):
    if shard not in ('staging', 'proofs') or len(recipes) != {'staging': 34, 'proofs': 15}[shard]:
        raise ValueError('shard recipe count differs')
    result = {}
    for kind in ('bash','python'):
        shell = ['#!/bin/bash', 'set -u -o pipefail', 'case "${1-}" in']
        for index, (entry_kind,path) in enumerate(recipes):
            if kind == entry_kind:
                shell.append('  ' + shlex.quote(ROOT+path) + ') idx=' + str(index) + ' ;;')
        binary = '/bin/bash' if kind == 'bash' else '/usr/local/bin/python3'
        shell.extend(['  *) exec '+binary+' "$@" ;;', 'esac',
            'printf "%s\\tstart\\n" "$idx" >> /run/user/1000/recipe-events',
            'rc=0'])
        pipe = ' 2>&1 | tee "/run/user/1000/recipe-${idx}.log" | head -c 16777217'
        if kind == 'bash' and shard == 'staging':
            shell.append('if [ "$1" = '+shlex.quote(ROOT+PRESERVATION)+' ]; then '+binary+' "$@" --owned-temp-root /run/user/1000/fixture-root'+pipe+'; codes=("${PIPESTATUS[@]}"); else '+binary+' "$@"'+pipe+'; codes=("${PIPESTATUS[@]}"); fi')
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

def validate_make_log(data):
    if len(data) > 32*1024*1024 or re.search(rb'(?i)\bskip(?:ped)?\b', data):
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

def validate_recipe(index, data, *, shard='staging'):
    if shard not in ('staging', 'proofs'): raise ValueError('unsupported shard')
    markers = MARKERS if shard == 'staging' else PROOFS_MARKERS
    if type(index) is not int or not 0 <= index < len(markers) or len(data) > 16*1024*1024:
        raise ValueError('recipe identity or output size invalid')
    if re.search(rb'(?i)\bskip(?:ped)?\b', data): raise ValueError('recipe skipped')
    marker = markers[index]
    if marker is not None:
        if data.splitlines().count(marker.encode()) != 1: raise ValueError('recipe marker missing or duplicated')
        return
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
