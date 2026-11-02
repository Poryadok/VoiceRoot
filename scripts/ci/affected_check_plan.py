"""Source-bound affected-check planning primitives.

Filter parsing deliberately supports only the repository's simple glob subset.
An unsupported rule raises instead of quietly selecting fewer checks.
"""
from pathlib import Path
import argparse
import json
import os
import re
import subprocess
import tempfile

KNOWN_FILTERS = frozenset('''global ci_global ci_scripts chat_auth_java_inputs local_docker_isolation go_build_shared nats_renderer t31
    history_audit f13_replay match_squad_overlay a1_t106_live a1_e2e a1_multi_account_pr a1_attachment_restart flutter_live_suite
    staging_infra analytics_integration_inputs protos pkg frontend music_inputs help_inputs startup_inputs dev_web_publish windows_desktop auth auth_test_impact
    developer-portal admin compose full_application_request'''.split()) | frozenset(
        'svc_' + name for name in '''analytics bot chat controlledgame federation file
        gameintegration gateway matchmaking messaging moderation notification realtime
        role search social space story subscription user voice'''.split())


def candidate_base(root, event_base, head, candidate):
    """Resolve a PR merge candidate's actual base, allowing an older event base."""
    if any(not re.fullmatch(r'(?:[0-9a-f]{40}|[0-9a-f]{64})', value)
           for value in (event_base, head, candidate)):
        raise ValueError('full commit identities are required')
    try:
        parents = subprocess.check_output(
            ['git', '-C', str(root), 'show', '-s', '--format=%P', candidate],
            stderr=subprocess.PIPE, text=True).strip().split()
    except subprocess.CalledProcessError as exc:
        raise ValueError('candidate commit unavailable') from exc
    if len(parents) != 2 or parents[1] != head:
        raise ValueError('candidate does not have the event head as its second parent')
    base = parents[0]
    if subprocess.run(['git', '-C', str(root), 'merge-base', '--is-ancestor', event_base, base],
                      stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode != 0:
        raise ValueError('event base is not an ancestor of candidate base')
    validate_identity(root, base, head, candidate)
    return base


def validate_identity(root, base, head, candidate):
    def git(*args):
        return subprocess.check_output(['git', '-C', str(root), *args],
                                       stderr=subprocess.PIPE, text=True).strip()

    for oid in (base, head, candidate):
        if not re.fullmatch(r'(?:[0-9a-f]{40}|[0-9a-f]{64})', oid):
            raise ValueError('full commit identities are required')
        try:
            if git('rev-parse', '--verify', oid + '^{commit}') != oid:
                raise ValueError('identity is not a commit')
        except subprocess.CalledProcessError as exc:
            raise ValueError('commit identity unavailable') from exc
    if candidate == head:
        if subprocess.run(['git', '-C', str(root), 'merge-base', '--is-ancestor',
                           base, head], stdout=subprocess.DEVNULL,
                          stderr=subprocess.DEVNULL).returncode != 0:
            raise ValueError('base is not an ancestor of local candidate')
    else:
        parents = git('show', '-s', '--format=%P', candidate).split()
        if parents != [base, head]:
            raise ValueError('candidate does not have exact base/head parents')
        try:
            expected_tree = git('merge-tree', '--write-tree', base, head).splitlines()[0]
        except subprocess.CalledProcessError as exc:
            raise ValueError('base/head merge is not clean') from exc
        if git('rev-parse', candidate + '^{tree}') != expected_tree:
            raise ValueError('candidate tree differs from clean merge')
    return {'base': base, 'head': head, 'candidate': candidate}


def validate_checkout(root, candidate):
    actual = subprocess.check_output(['git', '-C', str(root), 'rev-parse', 'HEAD'], text=True).strip()
    status = subprocess.check_output(['git', '-C', str(root), 'status', '--porcelain=v1',
                                      '-z', '--untracked-files=all'])
    if actual != candidate or status:
        raise ValueError('checkout must be clean and at the exact candidate')


def load_filters(path):
    filters = {}
    current = None
    for raw in Path(path).read_text(encoding='utf-8').splitlines():
        line = raw.strip()
        if not line or line.startswith('#'):
            continue
        if not raw.startswith(' ') and re.fullmatch(r'[A-Za-z0-9_-]+:', line):
            current = line[:-1]
            if current in filters:
                raise ValueError('duplicate filter: ' + current)
            filters[current] = []
        elif raw.startswith('  - ') and current:
            pattern = line[2:].strip()
            if len(pattern) >= 2 and pattern[0] in "'\"" and pattern[-1] == pattern[0]:
                pattern = pattern[1:-1]
            if not re.fullmatch(r'!?[A-Za-z0-9_./*+-]+', pattern):
                raise ValueError('unsupported filter pattern: ' + pattern)
            if any('**' in part and part != '**' for part in pattern.lstrip('!').split('/')):
                raise ValueError('unsupported globstar placement: ' + pattern)
            filters[current].append(pattern)
        else:
            raise ValueError('unsupported filter syntax: ' + raw)
    if not filters or any(not patterns for patterns in filters.values()):
        raise ValueError('empty filter configuration')
    return filters


def _glob(pattern):
    out = ''
    i = 0
    while i < len(pattern):
        if pattern[i:i + 3] == '**/':
            out += '(?:.*/)?'
            i += 3
        elif pattern[i:i + 2] == '**':
            out += '.*'
            i += 2
        elif pattern[i] == '*':
            out += '[^/]*'
            i += 1
        else:
            out += re.escape(pattern[i])
            i += 1
    return re.compile('^' + out + '$')


def match_filters(filters, paths):
    result = {}
    for name, patterns in filters.items():
        positive = [_glob(p) for p in patterns if not p.startswith('!')]
        negative = [_glob(p[1:]) for p in patterns if p.startswith('!')]
        result[name] = sorted(path for path in set(paths)
                              if any(p.fullmatch(path) for p in positive)
                              and not any(p.fullmatch(path) for p in negative))
    return result


def unclassified_paths(matched):
    # T31 is an additional proof, not a complete impact classification. Its
    # conservative backend catchall must not hide unknown executable inputs.
    classified = {path for name, paths in matched.items()
                  if name in KNOWN_FILTERS and name != 't31' for path in paths}
    unknown_rules = {path for name, paths in matched.items()
                     if name != 'code' and name not in KNOWN_FILTERS for path in paths}
    code = set(matched.get('code', ()))
    return sorted((code - classified) | (code & unknown_rules))


def effective_filters(matched):
    result = {name: bool(paths) for name, paths in matched.items()}
    if unclassified_paths(matched):
        result['ci_global'] = True
    return result


def verify_results(plan, results):
    return [name for name in sorted(plan['checks']) if results.get(name) != 'success']


def verify_gate(plan, needs, identity):
    """Check job-level results for a source-bound PR plan.

    Matrix-member completeness requires separate receipts; this function does
    not claim to establish it from GitHub's aggregate job result.
    """
    if (not isinstance(identity, dict) or set(identity) != {'base', 'head', 'candidate'} or
            any(not isinstance(oid, str) or not re.fullmatch(r'(?:[0-9a-f]{40}|[0-9a-f]{64})', oid)
                for oid in identity.values()) or
            plan.get('schema') != 'voice-affected-check-plan-v1' or
            plan.get('identity') != identity or
            not isinstance(plan.get('checks'), dict) or
            'current-diff-contract' not in plan['checks']):
        raise ValueError('invalid or mismatched affected-check plan')
    failures = []
    for name in sorted(plan['checks']):
        jobs = ('flutter-unit-tests', 'flutter-unit-receipts') if name == 'flutter-unit-tests' else (name,)
        if any(not isinstance(needs.get(job), dict) or needs[job].get('result') != 'success'
               for job in jobs):
            failures.append(name)
    return failures


def resolve_go_matrix(root, filters):
    env = dict(os.environ)
    env['FILTER_JSON'] = json.dumps({key: str(value).lower() for key, value in filters.items()})
    env['FORCE_FULL'] = 'false'
    bash = 'C:/Program Files/Git/bin/bash.exe' if os.name == 'nt' else 'bash'
    with tempfile.TemporaryDirectory() as directory:
        output_file = Path(directory) / 'output'
        env['GITHUB_OUTPUT'] = output_file.as_posix()
        subprocess.run([bash, str(Path(root) / 'scripts/ci/resolve-go-matrix.sh')],
                       env=env, cwd=root, text=True, capture_output=True, check=True)
        output = output_file.read_text(encoding='utf-8')
    rows = dict(line.split('=', 1) for line in output.splitlines()
                if re.match(r'^(go_services|docker_go_services|integration_go_services|run_pkg|run_go)=', line))
    keys = ('go_services', 'docker_go_services', 'integration_go_services', 'run_pkg', 'run_go')
    if set(rows) != set(keys):
        raise ValueError('incomplete Go matrix output')
    result = {key: json.loads(rows[key]) for key in keys}
    for key in keys[:3]:
        if (not isinstance(result[key], list) or
                any(not isinstance(value, str) or not value for value in result[key]) or
                len(set(result[key])) != len(result[key])):
            raise ValueError('invalid Go matrix: ' + key)
    if any(type(result[key]) is not bool for key in keys[3:]):
        raise ValueError('invalid Go matrix flags')
    return result


def make_plan(matched, matrix, head_ref='', widen_unknown=True):
    """Build an advisory PR plan; reasons retain the matching input paths."""
    unknown = unclassified_paths(matched) if widen_unknown else []
    flags = effective_filters(matched) if widen_unknown else {
        name: bool(paths) for name, paths in matched.items()}
    integration = matrix['integration_go_services']
    if widen_unknown and any(flags.get(key) for key in
                             ('global', 'ci_global', 'pkg', 'protos', 'go_build_shared')):
        integration = matrix['go_services']
    checks = {'current-diff-contract': ['every pull request']}
    all_paths = sorted({path for paths in matched.values() for path in paths})

    def selected(name, keys=(), enabled=False, reason_paths=None):
        paths = set(unknown if 'ci_global' in keys else ())
        for key in keys:
            paths.update(matched.get(key, ()))
        if enabled or any(flags.get(key, False) for key in keys):
            checks[name] = sorted(paths or set(reason_paths or all_paths))

    broad = ('global', 'ci_global')
    selected('ci-script-tests', ('global', 'ci_global', 'ci_scripts'))
    selected('f13-historical-replay', broad + ('f13_replay',),
             head_ref.startswith('codex/voice-master-release-') or
             head_ref == 'codex/game-sdk-federation-docs')
    selected('local-docker-isolation', ('global', 'ci_global', 'local_docker_isolation'))
    selected('golangci', broad, matrix['run_go'])
    selected('backend-go', broad, matrix['run_go'])
    selected('backend-go-pkg', broad, matrix['run_pkg'])
    selected('nats-hub-config-renderer-image', broad + ('nats_renderer',))
    selected('a1-e2e', broad + ('a1_multi_account_pr',))
    selected('a1-flutter-profile-handoff', broad + ('a1_t106_live',))
    selected('a1-attachment-restart-proof', broad + ('a1_attachment_restart',))
    selected('compose-e2e', broad + ('flutter_live_suite',))
    selected('t31-critical', broad + ('t31',))
    selected('analytics-clickhouse-integration', ('analytics_integration_inputs',), enabled='analytics' in integration)
    if flags.get('code'):
        selected('protobuf', broad + ('protos',))
        selected('compose-config', broad + ('compose',))
        selected('match-squad-overlay', broad + ('match_squad_overlay',))
        for name in ('flutter', 'flutter-unit-tests', 'flutter-device-driver',
                     'flutter-key-backup-goldens'):
            selected(name, broad + ('frontend', 'protos'))
        selected('flutter-music-proof', broad + ('music_inputs', 'protos'))
        selected('flutter-help-proof', broad + ('help_inputs', 'protos'))
        selected('flutter-startup-proof', broad + ('startup_inputs', 'protos'))
        selected('flutter-windows', ('windows_desktop',))
        selected('web', broad + ('frontend',))
        selected('backend-go-integration-pr', enabled=bool(integration))
        selected('backend-go-integration-evidence', enabled=bool(integration))
        selected('story-lfp-integration', enabled='story' in integration)
        selected('user-association-integration', enabled='user' in integration)
        selected('notification-quota-integration', enabled='notification' in integration)
        selected('role-migration-integration', enabled='role' in integration)
        selected('role-space-receipt-integration', enabled='role' in integration)
        selected('bot-membership-integration', enabled='bot' in integration)
        selected('voice-ledger-integration', enabled='voice' in integration)
        selected('messaging-threadlist-integration', enabled='messaging' in integration)
        selected('space-autovacuum-integration', enabled='space' in integration)
        selected('space-native-installer-integration', enabled='space' in integration)
        selected('chat-lifecycle-integration', enabled='chat' in integration)
        selected('chat-auth-java-integration', broad + ('auth', 'protos', 'pkg', 'chat_auth_java_inputs'), enabled='chat' in integration or matrix['run_pkg'])
        selected('realtime-lifecycle-integration', enabled='realtime' in integration)
        selected('backend-auth', broad + ('auth', 'auth_test_impact'))
        selected('backend-auth-image', broad + ('auth',))
        selected('admin', broad + ('admin',))
        selected('developer-portal', broad + ('developer-portal',))
    return {'components': {'go': matrix['go_services'],
                           'go_images': matrix['docker_go_services'],
                           'go_integration': integration},
            'checks': dict(sorted(checks.items())),
            'filters': flags, 'unclassified_paths': unknown}


def compare_hosted(root, plan, hosted, head_ref):
    keys = plan['filters']
    if any(hosted.get(key) not in ('true', 'false') for key in keys):
        raise ValueError('hosted filter output is incomplete or invalid')
    flags = {key: hosted[key] == 'true' for key in keys}
    matrix = resolve_go_matrix(root, flags)
    matched = {key: ['hosted filter'] if value else [] for key, value in flags.items()}
    previous = make_plan(matched, matrix, head_ref, widen_unknown=False)
    proposed_checks, previous_checks = set(plan['checks']), set(previous['checks'])
    return {'filter_disagreements': sorted(key for key in keys if flags[key] != plan['filters'][key]),
            'planner_only_checks': sorted(proposed_checks - previous_checks),
            'hosted_only_checks': sorted(previous_checks - proposed_checks),
            'hosted_components': previous['components']}


def changed_paths(root, base, head, *, git=None):
    args = ('diff', '--name-status', '-z', '--find-renames', base, head, '--')
    raw = (subprocess.check_output(['git', '-C', str(root), *args]) if git is None
           else git(root, *args))
    fields = raw.decode('utf-8', errors='strict').split('\0')
    if fields[-1] != '':
        raise ValueError('incomplete git diff')
    fields.pop()
    paths = set()
    while fields:
        status = fields.pop(0)
        count = 2 if status.startswith(('R', 'C')) else 1
        if len(fields) < count or not re.fullmatch(r'[ACDMRTUXB][0-9]*', status):
            raise ValueError('unsupported git diff status: ' + status)
        paths.update(fields[:count])
        del fields[:count]
    return sorted(paths)


def plan_paths(root, paths, head_ref=''):
    """Apply the same filter and dependency resolution to an explicit path set."""
    filter_path = Path(root) / '.github/ci/path-filters.yml'
    unresolved = None
    try:
        matched = match_filters(load_filters(filter_path), paths)
    except ValueError as exc:
        # Never interpret an unsupported rule as a non-match. The complete
        # existing PR gate is selected until the grammar is understood.
        unresolved = str(exc)
        keys = set(re.findall(r'^([A-Za-z0-9_-]+):\s*$',
                              filter_path.read_text(encoding='utf-8'), re.M))
        keys.update(('code', 'global', 'ci_global', 'ci_scripts', 'chat_auth_java_inputs', 'protos', 'pkg',
                     'go_build_shared', 'frontend', 'music_inputs', 'help_inputs', 'startup_inputs', 'auth', 'auth_test_impact', 'admin', 'developer-portal',
                     'compose', 'windows_desktop', 'history_audit', 'a1_t106_live',
                     'a1_attachment_restart', 'nats_renderer', 'match_squad_overlay'))
        matched = {key: paths or ['unresolved filter configuration'] for key in keys}
    matrix = resolve_go_matrix(root, effective_filters(matched))
    plan = make_plan(matched, matrix, head_ref)
    plan.update(head_ref=head_ref, changed_paths=paths, filter_reasons=matched,
                unresolved_filter_configuration=unresolved)
    return plan


def build_plan(root, base, head, candidate, head_ref=''):
    identity = validate_identity(root, base, head, candidate)
    validate_checkout(root, candidate)
    plan = plan_paths(root, changed_paths(root, base, candidate), head_ref)
    plan.update(schema='voice-affected-check-plan-v1', identity=identity)
    return plan


def build_local_plan(root, base, head, head_ref=''):
    """Plan the Git-visible working tree; this is not an execution receipt."""
    from local_source_snapshot import capture
    validate_identity(root, base, head, head)
    source = capture(root, head)
    paths = sorted(set(changed_paths(root, base, source['tree'])) |
                   set(source['raw_mismatch_paths']))
    plan = plan_paths(root, paths, head_ref)
    if capture(root, head) != source:
        raise ValueError('source changed while building local plan')
    plan.update(schema='voice-local-affected-check-plan-v1', base=base, source=source)
    return plan


def main():
    parser = argparse.ArgumentParser(description='Create an advisory PR affected-check plan')
    parser.add_argument('--base', required=True)
    parser.add_argument('--head', required=True)
    parser.add_argument('--candidate', required=True)
    parser.add_argument('--head-ref', required=True)
    parser.add_argument('--event-base', action='store_true',
                        help='resolve the actual merge parent from an older PR event base')
    parser.add_argument('--verify-needs', action='store_true',
                        help='reject required PR jobs that did not succeed')
    parser.add_argument('--compare-hosted', action='store_true',
                        help='compare with dorny outputs in HOSTED_FILTER_JSON')
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[2]
    base = candidate_base(root, args.base, args.head, args.candidate) if args.event_base else args.base
    plan = build_plan(root, base, args.head, args.candidate, args.head_ref)
    if args.verify_needs:
        needs = json.loads(os.environ['NEEDS_JSON'])
        failures = verify_gate(plan, needs, plan['identity'])
        if failures:
            raise SystemExit('affected checks did not succeed: ' + ', '.join(failures))
    if args.compare_hosted:
        plan['hosted_comparison'] = compare_hosted(
            root, plan, json.loads(os.environ['HOSTED_FILTER_JSON']), args.head_ref)
    print(json.dumps(plan, sort_keys=True, indent=2))


if __name__ == '__main__':
    main()
