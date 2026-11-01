"""Run implemented local diagnostics from the affected plan, never issuing PASS.

This is an incomplete preflight foundation. Unimplemented checks and unbound
inputs prevent PR handoff; GitHub's merge-candidate checks remain mandatory.
The opt-in Auth case assumes trusted exclusive daemon/socket custody and does
not authenticate images, helper or Maven cache, or establish crash coverage.
"""
import argparse
import json
from pathlib import Path
import re
import subprocess

import affected_check_plan
import local_ci_routing
import local_auth_orchestration
import local_chat_orchestration
import local_current_diff
from local_docker_owned import OwnershipError
import local_flutter_execution
import local_go_plan
import local_proto_execution
from local_process_supervision import SupervisionError
import local_source_snapshot


def _validate_chat_inputs(inputs):
    keys = {'helper', 'helper_sha256', 'postgres_archive', 'postgres_archive_sha256', 'postgres_image_id'}
    if (not isinstance(inputs, dict) or set(inputs) != keys or
            any(not isinstance(inputs[k], str) or not inputs[k] for k in keys) or
            any(not Path(inputs[k]).is_absolute() for k in ('helper', 'postgres_archive')) or
            any(not re.fullmatch('[0-9a-f]{64}', inputs[k]) for k in ('helper_sha256', 'postgres_archive_sha256')) or
            not re.fullmatch('sha256:[0-9a-f]{64}', inputs['postgres_image_id'])):
        raise ValueError('complete absolute-path Chat diagnostic inputs and exact digests are required')


def _validate_chat_report(result, source, inputs):
    if not isinstance(result, dict):
        raise ValueError('invalid Chat diagnostic report')
    diagnostic, identities = result.get('diagnostic'), result.get('inputs')
    if (result.get('schema') != 'voice-local-chat-private-diagnostic-v1' or
            result.get('status') != 'incomplete' or result.get('source') != source or
            not isinstance(diagnostic, dict) or diagnostic.get('status') != 'incomplete' or
            diagnostic.get('source') != source or diagnostic.get('exit_code') != 0 or
            not isinstance(identities, dict) or identities.get('creator_sha256') != inputs['helper_sha256'] or
            identities.get('postgres_archive_sha256') != inputs['postgres_archive_sha256'] or
            identities.get('postgres_image_id') != inputs['postgres_image_id'] or
            not isinstance(diagnostic.get('events_sha256'), str) or
            not re.fullmatch('[0-9a-f]{64}', diagnostic['events_sha256'])):
        raise ValueError('Chat diagnostic identity or status mismatch')
    tests, journal, slot = diagnostic.get('tests'), result.get('ownership_journal'), result.get('slot')
    if (not isinstance(tests, dict) or tests.get('packages') != ['voice/backend/chat/internal/store'] or
            type(tests.get('passed')) is not int or tests['passed'] < 1 or tests.get('skipped') != [] or
            type(slot) is not int or not 0 <= slot < 2 or not isinstance(journal, str) or
            Path(journal).is_absolute() or '..' in Path(journal).parts or
            not Path(journal).parts or Path(journal).parts[0] != 'run' or Path(journal).name != 'ownership.json'):
        raise ValueError('Chat diagnostic test or ownership evidence mismatch')


def _validate_auth_inputs(inputs):
    keys = {'helper', 'helper_sha256', 'postgres_archive', 'postgres_archive_sha256',
            'postgres_image_id', 'maven_image_id', 'maven_cache', 'maven_cache_sha256'}
    if (not isinstance(inputs, dict) or set(inputs) != keys or
            any(not isinstance(inputs[k], str) or not inputs[k] for k in keys) or
            any(not Path(inputs[k]).is_absolute() for k in ('helper', 'postgres_archive', 'maven_cache')) or
            any(not re.fullmatch(r'[0-9a-f]{64}', inputs[k]) for k in
                ('helper_sha256', 'postgres_archive_sha256', 'maven_cache_sha256')) or
            any(not re.fullmatch(r'sha256:[0-9a-f]{64}', inputs[k]) for k in
                ('postgres_image_id', 'maven_image_id'))):
        raise ValueError('complete absolute-path Auth inputs and exact digests are required')


def _validate_auth_report(result, source, inputs):
    if not isinstance(result, dict):
        raise ValueError('invalid Auth diagnostic report')
    diagnostic, identities = result.get('diagnostic'), result.get('inputs')
    if (result.get('schema') != 'voice-local-auth-private-diagnostic-v1' or result.get('status') != 'incomplete' or
            not isinstance(diagnostic, dict) or diagnostic.get('status') != 'incomplete' or
            diagnostic.get('source') != source or not isinstance(identities, dict) or
            identities.get('creator_sha256') != inputs['helper_sha256'] or
            identities.get('postgres_archive_sha256') != inputs['postgres_archive_sha256'] or
            identities.get('postgres_image_id') != inputs['postgres_image_id'] or
            identities.get('maven_image_id') != inputs['maven_image_id'] or
            diagnostic.get('maven_cache_sha256') != inputs['maven_cache_sha256'] or
            any(not isinstance(diagnostic.get(k), str) or not re.fullmatch(r'[0-9a-f]{64}', diagnostic[k])
                for k in ('report_sha256', 'source_archive_sha256', 'build_archive_sha256'))):
        raise ValueError('Auth diagnostic identity or status mismatch')
    journal, slot = result.get('ownership_journal'), result.get('slot')
    if (type(slot) is not int or not 0 <= slot < 2 or not isinstance(journal, str) or
            Path(journal).is_absolute() or '..' in Path(journal).parts or not Path(journal).parts or
            Path(journal).parts[0] != 'run' or Path(journal).name != 'ownership.json'):
        raise ValueError('Auth diagnostic ownership evidence mismatch')


def run(root, base, head, output, flutter, head_ref='', go_binary='go', buf_binary='buf', chat_inputs=None,
        auth_inputs=None):
    root = Path(root).resolve()
    output = Path(output).resolve()
    if output == root or root in output.parents or output.exists():
        raise ValueError('output must be a new directory outside the source tree')
    output.mkdir(parents=True, exist_ok=False)
    report = {'schema': 'voice-local-preflight-diagnostic-v1', 'status': 'incomplete', 'plan': None, 'checks': {}}
    try:
        if chat_inputs is not None:
            _validate_chat_inputs(chat_inputs)
        if auth_inputs is not None:
            _validate_auth_inputs(auth_inputs)
        plan = affected_check_plan.build_local_plan(root, base, head, head_ref)
        report['plan'] = plan
        report['checks'] = {name: {'status': 'not_run'} for name in plan['checks']}
        if auth_inputs is not None:
            if 'backend-auth' not in plan['checks']:
                raise ValueError('Auth diagnostic requires a planner-selected Auth integration')
            local_chat_orchestration._verify_file(auth_inputs['maven_cache'], auth_inputs['maven_cache_sha256'])
            diagnostic = local_auth_orchestration.run(root, head, output / 'auth-private', auth_inputs['helper'],
                auth_inputs['helper_sha256'], auth_inputs['postgres_archive'], auth_inputs['postgres_archive_sha256'],
                auth_inputs['postgres_image_id'], auth_inputs['maven_image_id'], auth_inputs['maven_cache'])
            _validate_auth_report(diagnostic, plan['source'], auth_inputs)
            local_chat_orchestration._verify_file(auth_inputs['maven_cache'], auth_inputs['maven_cache_sha256'])
            report['auth_private_report'] = 'auth-private/diagnostic.json'
        if chat_inputs is not None:
            components = plan.get('components', {})
            if ('backend-go-integration-pr' not in plan['checks'] or not isinstance(components, dict) or
                    not isinstance(components.get('go_integration'), list) or 'chat' not in components['go_integration']):
                raise ValueError('Chat diagnostic requires a planner-selected Chat integration')
            diagnostic = local_chat_orchestration.run(root, head, output / 'chat-private', chat_inputs['helper'],
                chat_inputs['helper_sha256'], chat_inputs['postgres_archive'], chat_inputs['postgres_archive_sha256'],
                chat_inputs['postgres_image_id'], go_binary)
            _validate_chat_report(diagnostic, plan['source'], chat_inputs)
            report['chat_private_report'] = 'chat-private/diagnostic.json'
        if 'ci-script-tests' in plan['checks']:
            diagnostic = local_ci_routing.run(root, plan['source'], output / 'ci-routing')
            if (diagnostic.get('schema') != 'voice-local-ci-routing-diagnostic-v1' or
                    diagnostic.get('status') != 'incomplete' or diagnostic.get('source') != plan['source'] or
                    not isinstance(diagnostic.get('commands'), list) or
                    [(row.get('name'), row.get('tests')) for row in diagnostic['commands'] if isinstance(row, dict)] !=
                    [('gate-shell', None), ('gate', len(local_ci_routing.GATE_CASES)),
                     ('router', len(local_ci_routing.ROUTER_CASES))]):
                raise ValueError('CI routing diagnostic did not match the planned source and inventory')
            report['ci_routing_report'] = 'ci-routing/diagnostic.json'
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
    except (OSError, ValueError, KeyError, TypeError, OwnershipError, SupervisionError,
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
    parser.add_argument('--chat-helper', help='explicit rootless creator executable path')
    parser.add_argument('--chat-helper-sha256')
    parser.add_argument('--chat-postgres-archive')
    parser.add_argument('--chat-postgres-archive-sha256')
    parser.add_argument('--chat-postgres-image-id')
    for name in ('helper', 'helper-sha256', 'postgres-archive', 'postgres-archive-sha256',
                 'postgres-image-id', 'maven-image-id', 'maven-cache', 'maven-cache-sha256'):
        parser.add_argument('--auth-' + name, help='opt-in private Auth diagnostic input')
    args = parser.parse_args()
    chat = {'helper': args.chat_helper, 'helper_sha256': args.chat_helper_sha256,
            'postgres_archive': args.chat_postgres_archive, 'postgres_archive_sha256': args.chat_postgres_archive_sha256,
            'postgres_image_id': args.chat_postgres_image_id}
    auth = {'helper': args.auth_helper, 'helper_sha256': args.auth_helper_sha256,
            'postgres_archive': args.auth_postgres_archive, 'postgres_archive_sha256': args.auth_postgres_archive_sha256,
            'postgres_image_id': args.auth_postgres_image_id, 'maven_image_id': args.auth_maven_image_id,
            'maven_cache': args.auth_maven_cache, 'maven_cache_sha256': args.auth_maven_cache_sha256}
    report = run(Path(__file__).resolve().parents[2], args.base, args.head, args.output, args.flutter,
                 args.head_ref, args.go, args.buf, chat if any(value is not None for value in chat.values()) else None,
                 auth if any(value is not None for value in auth.values()) else None)
    print(json.dumps(report, sort_keys=True))
    return 1 if report['status'] == 'failed' else 2


if __name__ == '__main__':
    raise SystemExit(main())
