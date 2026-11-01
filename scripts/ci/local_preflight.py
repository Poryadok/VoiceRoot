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
import local_ci_staging_execution
import local_ci_staging_request
import local_auth_orchestration
import local_auth_full_orchestration
import local_ci_matrix_inventory
import local_chat_orchestration
import local_compose_bridge
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


def _validate_full_auth_inputs(inputs):
    keys = {'helper', 'helper_sha256', 'client_image_id', 'maven_cache', 'maven_cache_sha256', 'images', 'images_sha256'}
    if (not isinstance(inputs, dict) or set(inputs) != keys or
            any(not isinstance(inputs[k], str) or not inputs[k] for k in keys) or
            any(not Path(inputs[k]).is_absolute() for k in ('helper', 'maven_cache', 'images')) or
            any(not re.fullmatch(r'[0-9a-f]{64}', inputs[k]) for k in ('helper_sha256', 'maven_cache_sha256', 'images_sha256')) or
            not re.fullmatch(r'sha256:[0-9a-f]{64}', inputs['client_image_id'])):
        raise ValueError('complete full Auth inputs and exact digests are required')


def _validate_full_auth_report(result, source, inputs, images):
    if not isinstance(result, dict):
        raise ValueError('invalid full Auth report')
    diagnostic, identities = result.get('diagnostic'), result.get('inputs')
    expected_images = {tag: {'image_id': item['image_id'], 'archive_sha256': item['sha256']}
                       for tag, item in images.items()}
    if (result.get('schema') != 'voice-local-auth-full-orchestration-v1' or result.get('status') != 'incomplete' or
            not isinstance(diagnostic, dict) or diagnostic.get('schema') != 'voice-local-auth-full-diagnostic-v1' or
            diagnostic.get('status') != 'incomplete' or diagnostic.get('source') != source or diagnostic.get('exit_code') != 0 or
            diagnostic.get('maven_cache_sha256') != inputs['maven_cache_sha256'] or not isinstance(identities, dict) or
            identities.get('creator_sha256') != inputs['helper_sha256'] or identities.get('client_image_id') != inputs['client_image_id'] or
            identities.get('maven_cache_sha256') != inputs['maven_cache_sha256'] or identities.get('images') != expected_images or
            any(not isinstance(diagnostic.get(k), str) or not re.fullmatch(r'[0-9a-f]{64}', diagnostic[k])
                for k in ('source_archive_sha256', 'build_archive_sha256'))):
        raise ValueError('full Auth diagnostic identity or status mismatch')
    summary, journal, slot = diagnostic.get('reports'), result.get('ownership_journal'), result.get('slot')
    if (not isinstance(summary, dict) or type(summary.get('tests')) is not int or summary['tests'] < 1 or
            type(summary.get('suite_count')) is not int or type(summary.get('required_suite_count')) is not int or
            not 1 <= summary['required_suite_count'] <= summary['suite_count'] or not isinstance(summary.get('skipped'), list) or
            not isinstance(summary.get('reports_sha256'), str) or not re.fullmatch(r'[0-9a-f]{64}', summary['reports_sha256']) or
            type(slot) is not int or not 0 <= slot < 2 or not isinstance(journal, str) or Path(journal).is_absolute() or
            '..' in Path(journal).parts or not Path(journal).parts or Path(journal).parts[0] != 'run' or
            Path(journal).name != 'ownership.json'):
        raise ValueError('full Auth report or ownership evidence mismatch')


def run(root, base, head, output, flutter, head_ref='', go_binary='go', buf_binary='buf', chat_inputs=None,
        auth_inputs=None, compose_inputs=None, staging_assets=None, runtime_images=None, full_auth_inputs=None):
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
        if full_auth_inputs is not None:
            _validate_full_auth_inputs(full_auth_inputs)
        if auth_inputs is not None and full_auth_inputs is not None:
            raise ValueError('choose one Auth diagnostic mode')
        if compose_inputs is not None:
            local_compose_bridge.validate_inputs(compose_inputs)
        if runtime_images is not None:
            if staging_assets is None:
                raise ValueError('runtime images require staging assets')
            local_ci_staging_request.validate_runtime_images(runtime_images)
            if not isinstance(staging_assets, dict) or set(staging_assets) != local_ci_staging_request.AGGREGATE_ASSET_NAMES:
                raise ValueError('runtime aggregate requires exact staging, kubectl and phase0 assets')
            local_ci_staging_request.parse_assets(json.dumps(staging_assets).encode())
            for name, digest in (('phase0_jdk', local_ci_staging_request.PHASE0_JDK_HASH),
                                 ('phase0_compose', local_ci_staging_request.PHASE0_COMPOSE_HASH)):
                local_chat_orchestration._verify_file(staging_assets[name], digest)
        plan = affected_check_plan.build_local_plan(root, base, head, head_ref)
        report['plan'] = plan
        report['checks'] = {name: {'status': 'not_run'} for name in plan['checks']}
        if full_auth_inputs is not None:
            if 'backend-auth' not in plan['checks']:
                raise ValueError('full Auth diagnostic requires a planner-selected Auth check')
            values = full_auth_inputs
            images = local_auth_full_orchestration.read_images(values['images'], values['images_sha256'])
            diagnostic = local_auth_full_orchestration.run(root, head, output / 'auth-full', values['helper'],
                values['helper_sha256'], values['client_image_id'], values['maven_cache'], values['maven_cache_sha256'], images)
            _validate_full_auth_report(diagnostic, plan['source'], values, images)
            local_chat_orchestration._verify_file(values['images'], values['images_sha256'])
            report['auth_full_report'] = 'auth-full/diagnostic.json'
        if staging_assets is not None and 'ci-script-tests' not in plan['checks']:
            raise ValueError('staging assets require a planner-selected ci-script-tests check')
        if staging_assets is not None and (not isinstance(staging_assets, dict) or
                not isinstance(staging_assets.get('kubectl'), str) or not Path(staging_assets['kubectl']).is_absolute()):
            raise ValueError('selected CI script shards require an absolute kubectl asset path')
        if staging_assets is not None:
            local_chat_orchestration._verify_file(staging_assets['kubectl'], local_ci_staging_request.KUBECTL_HASH)
        if runtime_images is not None and (staging_assets is None or 'ci-script-tests' not in plan['checks']):
            raise ValueError('runtime images require staging assets and a planner-selected ci-script-tests check')
        if compose_inputs is not None:
            diagnostic = local_compose_bridge.run(plan, compose_inputs, output / 'compose-private')
            expected = sorted(local_compose_bridge.CHECKS.intersection(plan['checks']))
            if (not isinstance(diagnostic, dict) or not expected or
                    diagnostic.get('schema') != 'voice-local-compose-bridge-v1' or
                    diagnostic.get('status') != 'incomplete' or diagnostic.get('selected_checks') != expected or
                    diagnostic.get('fixture_report') != 'fixture/diagnostic.json'):
                raise ValueError('Compose fixture diagnostic did not match the selected checks')
            report['compose_private_report'] = 'compose-private/bridge.json'
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
            if staging_assets is not None:
                source = plan['source']
                shards = [(name, local_ci_matrix_inventory.SHARD_RECIPE_COUNTS[name])
                          for name in ('staging', 'proofs', 'policy')]
                if runtime_images is not None: shards.append(('runtime', 23))
                requests = []
                for shard, recipe_count in shards:
                    assets = staging_assets
                    if runtime_images is not None:
                        names = (local_ci_staging_request.RUNTIME_ASSET_NAMES if shard == 'runtime'
                                 else local_ci_staging_request.POLICY_ASSET_NAMES)
                        assets = {name: staging_assets[name] for name in names}
                    request = {'root': str(root), 'output': str(output / ('ci-' + shard)),
                               'commit': source['head'], 'tree': source['tree'], 'base': base,
                               'shard': shard, 'assets': assets}
                    if shard == 'runtime': request['runtime_images'] = runtime_images
                    if runtime_images is not None: local_ci_staging_request.validate(request)
                    requests.append((shard, recipe_count, request))
                for shard, recipe_count, request in requests:
                    diagnostic = local_ci_staging_execution.run(request)
                    go_tests = diagnostic.get('hosted_go_tests')
                    policy_tests = diagnostic.get('hosted_policy_tests')
                    if (diagnostic.get('schema') != 'voice-local-ci-staging-v1' or
                            diagnostic.get('status') != 'incomplete' or diagnostic.get('source') != source['head'] or
                            diagnostic.get('base') != base or diagnostic.get('shard') != shard or
                            type(diagnostic.get('recipes')) is not int or diagnostic['recipes'] != recipe_count or
                            type(go_tests) is not int or (go_tests < 1 if shard == 'staging' else go_tests != 0) or
                            type(policy_tests) is not int or (policy_tests < 1 if shard == 'policy' else policy_tests != 0) or
                            local_source_snapshot.capture(root, head) != source):
                        raise ValueError('CI shard receipt or source identity differs')
                    report['ci_' + shard + '_report'] = 'ci-' + shard + '/diagnostic.json'
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
    except (OSError, ValueError, KeyError, TypeError, OwnershipError, SupervisionError, local_ci_staging_execution.ExecutionError,
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
    parser.add_argument('--ci-staging-assets', help='JSON mapping of pinned local staging inputs; runtime also requires phase0_jdk and phase0_compose')
    parser.add_argument('--ci-runtime-images', help='JSON mapping of three pinned runtime-shard image archives')
    parser.add_argument('--chat-helper', help='explicit rootless creator executable path')
    parser.add_argument('--chat-helper-sha256')
    parser.add_argument('--chat-postgres-archive')
    parser.add_argument('--chat-postgres-archive-sha256')
    parser.add_argument('--chat-postgres-image-id')
    for name in ('helper', 'helper-sha256', 'postgres-archive', 'postgres-archive-sha256',
                 'postgres-image-id', 'maven-image-id', 'maven-cache', 'maven-cache-sha256'):
        parser.add_argument('--auth-' + name, help='opt-in private Auth diagnostic input')
    for name in ('helper', 'helper-sha256', 'client-image-id', 'maven-cache', 'maven-cache-sha256', 'images', 'images-sha256'):
        parser.add_argument('--auth-full-' + name, help='opt-in full Auth diagnostic input')
    for name in ('helper', 'helper-sha256', 'postgres-archive', 'postgres-archive-sha256', 'postgres-image-id'):
        parser.add_argument('--compose-' + name, help='opt-in fixed private Compose fixture input')
    args = parser.parse_args()
    chat = {'helper': args.chat_helper, 'helper_sha256': args.chat_helper_sha256,
            'postgres_archive': args.chat_postgres_archive, 'postgres_archive_sha256': args.chat_postgres_archive_sha256,
            'postgres_image_id': args.chat_postgres_image_id}
    auth = {'helper': args.auth_helper, 'helper_sha256': args.auth_helper_sha256,
            'postgres_archive': args.auth_postgres_archive, 'postgres_archive_sha256': args.auth_postgres_archive_sha256,
            'postgres_image_id': args.auth_postgres_image_id, 'maven_image_id': args.auth_maven_image_id,
            'maven_cache': args.auth_maven_cache, 'maven_cache_sha256': args.auth_maven_cache_sha256}
    full_auth = {'helper': args.auth_full_helper, 'helper_sha256': args.auth_full_helper_sha256,
                 'client_image_id': args.auth_full_client_image_id, 'maven_cache': args.auth_full_maven_cache,
                 'maven_cache_sha256': args.auth_full_maven_cache_sha256,
                 'images': args.auth_full_images, 'images_sha256': args.auth_full_images_sha256}
    compose = {'helper': args.compose_helper, 'helper_sha256': args.compose_helper_sha256,
               'postgres_archive': args.compose_postgres_archive,
               'postgres_archive_sha256': args.compose_postgres_archive_sha256,
               'postgres_image_id': args.compose_postgres_image_id}
    staging_assets = None
    if args.ci_staging_assets:
        with Path(args.ci_staging_assets).open('rb') as stream:
            staging_assets = local_ci_staging_request.parse_assets(stream.read(1024 * 1024 + 1))
    runtime_images = None
    if args.ci_runtime_images:
        with Path(args.ci_runtime_images).open('rb') as stream:
            runtime_images = local_ci_staging_request.parse_runtime_images(stream.read(1024 * 1024 + 1))
    report = run(Path(__file__).resolve().parents[2], args.base, args.head, args.output, args.flutter,
                 args.head_ref, args.go, args.buf, chat if any(value is not None for value in chat.values()) else None,
                 auth if any(value is not None for value in auth.values()) else None,
                 compose if any(value is not None for value in compose.values()) else None, staging_assets, runtime_images,
                 full_auth if any(value is not None for value in full_auth.values()) else None)
    print(json.dumps(report, sort_keys=True))
    return 1 if report['status'] == 'failed' else 2


if __name__ == '__main__':
    raise SystemExit(main())
