"""Inventory the reviewed CI-script shard entrypoints without running them.

This is a pinned top-level inventory, not a test receipt or a sandbox. Shell
and Python entrypoints may execute nested commands not enumerated here.
"""
import hashlib
import json
from pathlib import Path
import re
import sys


MAKEFILE_SHA256 = 'c141ff9ff807b05a9812aa8c4a14d163f25f917e95ca384f5c06a257bb1d8a86'
WORKFLOW_SHA256 = 'e036f5fc2b1a8241bd2728c7bc80e01fad9cf57d4f480dbf86dbc0852d21f4aa'
SHARDS = ('policy', 'runtime', 'staging', 'proofs')
SHARD_RECIPE_COUNTS = {'policy': 75, 'runtime': 23, 'staging': 34, 'proofs': 15}


def _sha(data):
    return hashlib.sha256(data).hexdigest()


def _recipe_source(root, name):
    path = Path(name)
    if path.is_absolute() or any(part in ('.', '..') for part in path.parts):
        raise ValueError('CI matrix recipe source unsafe: ' + name)
    current = root
    for part in path.parts:
        current = current / part
        if current.is_symlink():
            raise ValueError('CI matrix recipe source symlink: ' + name)
    if current.is_dir():
        return name + '/', None  # Discovery remains unexpanded.
    if current.is_file():
        return name, _sha(current.read_bytes())
    raise ValueError('CI matrix recipe source missing or unsupported: ' + name)


def collect(text, target):
    """Resolve literal single-target rules, without evaluating Make syntax."""
    rules, active = {}, None
    for line_number, line in enumerate(text.splitlines(), 1):
        if line.startswith('\t'):
            if active is not None:
                rules[active]['recipes'].append((line_number, line[1:]))
            continue
        if not line.strip() or line.lstrip().startswith('#'):
            continue
        match = re.fullmatch(r'([A-Za-z0-9_.-]+):\s*(.*)', line)
        if not match or match.group(2).startswith('export '):
            active = None
            continue
        name, dependencies = match.groups()
        if name.startswith('.'):
            active = None
            continue
        rules.setdefault(name, {'deps': [], 'recipes': []})['deps'].extend(dependencies.split())
        active = name
    seen, visiting, result = set(), set(), []
    def visit(name):
        if name in visiting:
            raise ValueError('cyclic Make target dependency')
        if name in seen:
            return
        if name not in rules:
            raise ValueError('missing Make target: ' + name)
        visiting.add(name)
        for dependency in rules[name]['deps']:
            if not re.fullmatch('[A-Za-z0-9_.-]+', dependency):
                raise ValueError('unsupported Make target dependency')
            visit(dependency)
        for line_number, recipe in rules[name]['recipes']:
            if not recipe or recipe.endswith('\\') or recipe.startswith(('+', '@', '-')):
                raise ValueError('unsupported Make recipe')
            result.append({'target': name, 'line': line_number, 'recipe': recipe, 'status': 'not_run'})
        visiting.remove(name)
        seen.add(name)
    visit(target)
    if not result:
        raise ValueError('empty Make target inventory')
    return result


def build(snapshot):
    root = Path(snapshot).resolve()
    try:
        makefile = (root / 'Makefile').read_bytes()
        workflow = (root / '.github/workflows/ci.yml').read_bytes()
    except OSError as error:
        raise ValueError('CI matrix inventory safety surface missing') from error
    if _sha(makefile) != MAKEFILE_SHA256 or _sha(workflow) != WORKFLOW_SHA256:
        raise ValueError('CI matrix inventory safety surface changed; re-audit required')
    text = makefile.decode('utf-8')
    shards = {name: collect(text, 'ci-script-tests-' + name) for name in SHARDS}
    for rows in shards.values():
        for row in rows:
            sources = re.findall(r'\$\(ROOT\)/([^"\s]+)', row['recipe'])
            hashes = {}
            for name in sources:
                key, digest = _recipe_source(root, name)
                hashes[key] = digest
            row['source_sha256'] = hashes
    workflow_steps = {}
    for name in SHARDS:
        rows = [{'action': 'actions/checkout@v5', 'with': {'fetch-depth': 0}, 'status': 'not_run'}]
        if name == 'policy':
            rows.append({'command': 'python3 scripts/ci/compiled_source_build_test.py', 'status': 'not_run'})
        rows.append({'action': 'actions/setup-python@v6', 'with': {'python-version': '3.12'}, 'status': 'not_run'})
        rows.append({'command': 'python3 -m pip install PyYAML==6.0.3', 'status': 'not_run'})
        rows.append({'command': 'base_sha="$(bash scripts/ci/resolve-voice-r22-base.sh)"\n'
                     'echo "VOICE_R22_BASE_SHA=${base_sha}" >> "${GITHUB_ENV}"\n'
                     'echo "VOICE_CI_PUSH_BEFORE_SHA=${VOICE_CI_PUSH_BEFORE_SHA}" >> "${GITHUB_ENV}"',
                     'status': 'not_run'})
        rows.append({'command': 'make ci-script-tests-' + name, 'status': 'not_run'})
        rows.append({'action': 'actions/setup-go@v5', 'with': {'go-version': '${{ env.GO_VERSION }}'}, 'status': 'not_run'})
        if name == 'staging':
            rows.append({'command': 'go test -count=1 -timeout=90s ./...',
                         'working_directory': 'scripts/staging/nats-leaf-routing-test', 'status': 'not_run'})
        workflow_steps[name] = rows
    return {'schema': 'voice-local-ci-matrix-inventory-v1', 'status': 'incomplete',
            'makefile_sha256': MAKEFILE_SHA256, 'workflow_sha256': WORKFLOW_SHA256,
            'shards': shards, 'workflow_steps': workflow_steps,
            'omitted': ['all execution and no-skip receipts', 'nested script commands and dynamic tests',
                        'whole-tree source identity and snapshot custody', 'hosted environment and event-value reproduction',
                        'Makefile, workflow and snapshot root may follow links', 'hosted-only input files are not hashed',
                        'toolchain/dependency provenance', 'filesystem/network/Docker isolation',
                        'historical replay and merge-candidate verification']}


if __name__ == '__main__':
    if len(sys.argv) != 2:
        raise SystemExit('usage: local_ci_matrix_inventory.py SNAPSHOT_ROOT')
    print(json.dumps(build(sys.argv[1]), indent=2, sort_keys=True))
