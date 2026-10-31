"""Run implemented local diagnostics from the affected plan, never issuing PASS.

This is an incomplete preflight foundation. Unimplemented checks and unbound
inputs prevent PR handoff; GitHub's merge-candidate checks remain mandatory.
"""
import argparse
import json
from pathlib import Path
import re
import subprocess

import affected_check_plan
import local_current_diff
from local_docker_owned import OwnershipError
import local_flutter_execution
import local_go_plan
import local_proto_execution
import local_source_snapshot


def run(root, base, head, output, flutter, head_ref='', go_binary='go', buf_binary='buf'):
    root = Path(root).resolve()
    output = Path(output).resolve()
    if output == root or root in output.parents or output.exists():
        raise ValueError('output must be a new directory outside the source tree')
    output.mkdir(parents=True, exist_ok=False)
    report = {'schema': 'voice-local-preflight-diagnostic-v1', 'status': 'incomplete', 'plan': None, 'checks': {}}
    try:
        plan = affected_check_plan.build_local_plan(root, base, head, head_ref)
        report['plan'] = plan
        report['checks'] = {name: {'status': 'not_run'} for name in plan['checks']}
        if 'current-diff-contract' in plan['checks']:
            diagnostic = local_current_diff.run(root, plan['source'], base, output / 'current-diff')
            if (diagnostic.get('status') != 'diagnostic' or diagnostic.get('source') != plan['source'] or
                    diagnostic.get('base') != base):
                raise ValueError('current-diff diagnostic did not match the planned source and base')
            report['checks']['current-diff-contract'] = {'status': 'diagnostic',
                                                         'report': 'current-diff/diagnostic.json'}
        if 'protobuf' in plan['checks']:
            diagnostic = local_proto_execution.run(root, plan['source'], base, output / 'protobuf',
                                                   go_binary, buf_binary)
            commands = diagnostic.get('commands')
            if (diagnostic.get('status') != 'incomplete' or diagnostic.get('source') != plan['source'] or
                    diagnostic.get('base') != base or not isinstance(commands, list) or
                    [item.get('name') for item in commands if isinstance(item, dict)] !=
                    ['buf-lint', 'buf-format', 'buf-breaking', 'go-codegen-selftest', 'go-codegen',
                     'auth-proto-sync', 'auth-compile', 'r23-contract'] or
                    any(item.get('exit_code') != 0 for item in commands) or
                    not isinstance(diagnostic.get('r23'), dict) or
                    type(diagnostic['r23'].get('passed')) is not int or diagnostic['r23']['passed'] < 1 or
                    diagnostic['r23'].get('skipped') != []):
                raise ValueError('protobuf diagnostic failed or does not match the planned source and base')
            report['checks']['protobuf'] = {'status': 'diagnostic', 'report': 'protobuf/diagnostic.json'}
        if 'backend-go' in plan['checks']:
            services = plan['components']['go']
            diagnostic = local_go_plan.run(root, plan['source'], services, output / 'go', go_binary)
            if (diagnostic.get('status') != 'incomplete' or diagnostic.get('source') != plan['source'] or
                    not isinstance(diagnostic.get('services'), dict) or
                    set(diagnostic['services']) != set(services)):
                raise ValueError('Go diagnostic did not match the planned source and services')
            image = diagnostic.get('image')
            if (not isinstance(image, dict) or image.get('reference') != local_go_plan.GO_IMAGE or
                    not isinstance(image.get('id'), str) or not re.fullmatch(r'sha256:[0-9a-f]{64}', image['id'])):
                raise ValueError('Go diagnostic image identity is missing')
            for service, result in diagnostic['services'].items():
                acquisition, transfer = result['acquisition'], result['transfer']
                inventory, tests = result['inventory'], result['diagnostic']
                with_tests, without_tests = inventory['packages_with_tests'], inventory['packages_without_tests']
                if (acquisition.get('status') != 'incomplete' or acquisition.get('source') != plan['source'] or
                        acquisition.get('module_dir') != 'src/backend/' + service or
                        not isinstance(acquisition.get('archive_sha256'), str) or
                        not re.fullmatch(r'[0-9a-f]{64}', acquisition['archive_sha256']) or
                        not isinstance(result.get('source_archive_sha256'), str) or
                        not re.fullmatch(r'[0-9a-f]{64}', result['source_archive_sha256']) or
                        type(result.get('slot')) is not int or not 0 <= result['slot'] < 2 or
                        not isinstance(result.get('ownership_journal'), str) or
                        Path(result['ownership_journal']).is_absolute() or
                        '..' in Path(result['ownership_journal']).parts or
                        not Path(result['ownership_journal']).parts or
                        Path(result['ownership_journal']).parts[0] != service or
                        Path(result['ownership_journal']).name != 'ownership.json' or
                        transfer.get('status') != 'incomplete' or
                        transfer.get('archive_sha256') != acquisition.get('archive_sha256') or
                        inventory.get('status') != 'incomplete' or tests.get('status') != 'incomplete' or
                        tests.get('exit_code') != 0 or not isinstance(with_tests, list) or
                        not isinstance(without_tests, list) or not (with_tests or without_tests) or
                        not isinstance(tests.get('tests'), dict)):
                    raise ValueError('Go diagnostic is incomplete or mismatched')
                summary = tests['tests']
                if (summary.get('packages') != sorted(with_tests + without_tests) or
                        type(summary.get('passed')) is not int or summary['passed'] < 0 or
                        not isinstance(summary.get('skipped'), list)):
                    raise ValueError('Go diagnostic package inventory or test counts differ')
            report['go_report'] = 'go/diagnostic.json'
            report['checks']['backend-go'] = {'status': 'diagnostic', 'report': 'go/diagnostic.json'}
        flutter_checks = {'flutter', 'flutter-unit-tests'} & set(plan['checks'])
        if flutter_checks:
            diagnostic = local_flutter_execution.run(root, head, output / 'flutter', flutter)
            report['flutter_report'] = 'flutter/diagnostic.json'
            if diagnostic.get('source') != plan['source']:
                raise ValueError('Flutter diagnostic source does not match the plan')
            if diagnostic.get('status') != 'incomplete':
                raise ValueError('Flutter diagnostic failed or has an unsupported status')
            if not diagnostic.get('full_inventory') or not isinstance(diagnostic.get('tests'), dict):
                raise ValueError('Flutter diagnostic did not execute the full inventory')
            tests = diagnostic['tests']
            if type(tests.get('passed')) is not int or tests['passed'] < 1 or not isinstance(tests.get('skipped'), list):
                raise ValueError('Flutter diagnostic test counts are missing or invalid')
            if tests['skipped']:
                raise ValueError('Flutter diagnostic skipped required tests')
            for name in flutter_checks:
                report['checks'][name] = {'status': 'diagnostic', 'report': 'flutter/diagnostic.json'}
        if local_source_snapshot.capture(root, head) != plan['source']:
            raise ValueError('source changed during local preflight')
    except (OSError, ValueError, KeyError, TypeError, OwnershipError,
            subprocess.CalledProcessError, subprocess.TimeoutExpired) as error:
        report['status'] = 'failed'
        if report['plan'] is None and isinstance(error, subprocess.CalledProcessError) and error.stderr:
            detail = error.stderr.decode('utf-8', errors='replace') if isinstance(error.stderr, bytes) else error.stderr
            report['error'] = 'Go matrix resolver failed: ' + detail.strip()
        else:
            report['error'] = str(error)
    (output / 'preflight.json').write_text(json.dumps(report, indent=2, sort_keys=True), encoding='utf-8')
    return report


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--base', required=True)
    parser.add_argument('--head', required=True)
    parser.add_argument('--output', required=True)
    parser.add_argument('--flutter', default='flutter')
    parser.add_argument('--go', default='go', help='Go executable matching the pinned diagnostic image')
    parser.add_argument('--buf', default='buf', help='Buf executable for protobuf diagnostics')
    parser.add_argument('--head-ref', required=True, help='PR head branch for branch-specific checks')
    args = parser.parse_args()
    report = run(Path(__file__).resolve().parents[2], args.base, args.head, args.output, args.flutter, args.head_ref, args.go, args.buf)
    print(json.dumps(report, sort_keys=True))
    return 1 if report['status'] == 'failed' else 2


if __name__ == '__main__':
    raise SystemExit(main())
