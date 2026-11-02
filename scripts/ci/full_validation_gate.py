"""Fail closed when an exact-SHA full CI run has incomplete check results.

Promotion requires this aggregation in its own workflow run. Job checkout,
matrix and no-skip bindings must also be reviewed before unresolved inventory
can be cleared; aggregation alone does not establish those properties.
"""
import json
import os
from pathlib import Path
import re
import subprocess
import sys

import go_integration_source_audit


REQUIRED_JOBS = (
    'changes', 'protobuf', 'compose-config', 'ci-script-tests', 'f13-historical-replay', 'chat-auth-java-integration',
    'match-squad-overlay', 'local-docker-isolation',
    'flutter-unit-plan', 'flutter-unit-tests', 'flutter-unit-receipts',
    'flutter', 'flutter-music-proof', 'flutter-help-proof',
    'flutter-startup-proof', 'flutter-device-driver',
    'flutter-key-backup-goldens', 'flutter-windows', 'flutter-android-smoke',
    'flutter-web-integration', 'flutter-ios', 'golangci', 'backend-go-pkg',
    'backend-go', 'backend-go-integration-pr', 'backend-go-integration-evidence',
    'backend-go-integration', 'nats-hub-config-renderer-image',
    'story-lfp-integration', 'user-association-integration',
    'notification-quota-integration', 'role-migration-integration',
    'role-space-receipt-integration', 'bot-membership-integration',
    'voice-ledger-integration', 'messaging-threadlist-integration',
    'space-autovacuum-integration', 'space-native-installer-integration',
    'chat-lifecycle-integration', 'realtime-lifecycle-integration',
    'backend-auth', 'backend-auth-image', 'developer-portal', 'web', 'admin',
    't31-critical', 'compose-e2e', 'a1-e2e', 'a1-flutter-profile-handoff',
    'a1-attachment-restart-proof', 'analytics-clickhouse-integration',
    'grafana-analytics-smoke', 'local-ci-parity',
)


def qualify(context, needs, audit, services):
    if not isinstance(context, dict) or set(context) != {
            'event', 'profile', 'ref', 'sha', 'checkout', 'run', 'attempt'}:
        raise ValueError('invalid full-validation context')
    if (context['event'] not in ('workflow_dispatch', 'push') or context['profile'] != 'full'
            or context['ref'] != 'refs/heads/master'
            or not isinstance(context['sha'], str)
            or not re.fullmatch(r'[0-9a-f]{40}', context['sha'])
            or context['checkout'] != context['sha']
            or any(not isinstance(context[k], str) or not re.fullmatch(r'[1-9][0-9]*', context[k])
                   for k in ('run', 'attempt'))):
        raise ValueError('full-validation identity or profile mismatch')
    issues = []
    if (not isinstance(audit, dict) or set(audit) != {'source_cases', 'unresolved'}
            or type(audit['source_cases']) is not int or audit['source_cases'] <= 0
            or type(audit['unresolved']) is not int or audit['unresolved'] != 0):
        unresolved = audit.get('unresolved') if isinstance(audit, dict) else None
        issues.append('integration source inventory incomplete (unresolved=' + str(unresolved) + ')')
    if not isinstance(needs, dict):
        raise ValueError('invalid full-validation job results')
    missing = sorted(set(REQUIRED_JOBS) - set(needs))
    unexpected = sorted(set(needs) - set(REQUIRED_JOBS))
    if missing:
        issues.append('missing jobs: ' + ', '.join(missing))
    if unexpected:
        issues.append('unexpected jobs: ' + ', '.join(unexpected))
    failed = [name + '=' + str(needs[name].get('result')) if isinstance(needs[name], dict)
              else name + '=invalid' for name in REQUIRED_JOBS if name in needs and
              (not isinstance(needs[name], dict) or needs[name].get('result') != 'success')]
    if failed:
        issues.append('jobs did not succeed: ' + ', '.join(failed))
    if (not isinstance(services, list) or not services or
            any(not isinstance(name, str) or not re.fullmatch(r'[a-z][a-z0-9]*', name) for name in services) or
            len(services) != len(set(services))):
        raise ValueError('invalid required integration service inventory')
    changes = needs.get('changes')
    outputs = changes.get('outputs') if isinstance(changes, dict) else None
    if not isinstance(outputs, dict) or any(outputs.get(key) != 'true' for key in
                                            ('run_go', 'run_pkg', 'code', 'run_flutter_tier2')):
        issues.append('full-validation scope is incomplete')
    for key in ('go_services', 'integration_go_services') if isinstance(outputs, dict) else ():
        try:
            matrix = json.loads(outputs[key])
        except (KeyError, TypeError, json.JSONDecodeError):
            issues.append('invalid full-validation service matrix: ' + key)
            continue
        if (not isinstance(matrix, list) or any(not isinstance(name, str) for name in matrix)
                or len(matrix) != len(services) or len(set(matrix)) != len(matrix)
                or set(matrix) != set(services)):
            issues.append('incomplete full-validation service matrix: ' + key)
    if issues:
        raise ValueError('; '.join(issues))
    return {'schema': 'voice-full-check-aggregation-v1',
            'identity': {key: context[key] for key in ('ref', 'sha', 'run', 'attempt')},
            'jobs': list(REQUIRED_JOBS), 'services': sorted(services),
            'integration_source_cases': audit['source_cases']}


def _unique_object(pairs):
    value = {}
    for key, item in pairs:
        if key in value:
            raise ValueError('duplicate JSON key')
        value[key] = item
    return value


def main():
    root = Path(__file__).resolve().parents[2]
    try:
        context = {'event': os.environ['GITHUB_EVENT_NAME'], 'profile': os.environ['FULL_PROFILE'],
                   'ref': os.environ['GITHUB_REF'], 'sha': os.environ['GITHUB_SHA'],
                   'checkout': subprocess.check_output(
                       ['git', '-C', str(root), 'rev-parse', 'HEAD'], text=True).strip(),
                   'run': os.environ['GITHUB_RUN_ID'], 'attempt': os.environ['GITHUB_RUN_ATTEMPT']}
        needs = json.loads(os.environ['NEEDS_JSON'], object_pairs_hook=_unique_object)
        inventory = json.loads((root / 'scripts/ci/go_integration_required.json').read_text(encoding='utf-8'),
                               object_pairs_hook=_unique_object)
        exceptions = json.loads((root / 'scripts/ci/go_integration_exceptions.json').read_text(encoding='utf-8'),
                                object_pairs_hook=_unique_object)
        audit = go_integration_source_audit.audit(root, inventory, exceptions)
        receipt = qualify(context, needs, audit, sorted(inventory))
    except (KeyError, OSError, ValueError, subprocess.CalledProcessError) as error:
        print('full-validation aggregation rejected: ' + str(error), file=sys.stderr)
        return 1
    print(json.dumps(receipt, sort_keys=True))
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
