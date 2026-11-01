"""Run affected-plan diagnostics or the narrow current-diff/overlay handoff.

The diagnostic mode remains incomplete. Handoff mode accepts only its fully
executed current-diff and overlay plans; GitHub merge-candidate checks remain mandatory.
The opt-in Auth case assumes trusted exclusive daemon/socket custody and does
not authenticate images, helper or Maven cache, or establish crash coverage.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import stat
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
from local_docker_owned import DockerCLI, OwnedRun, OwnershipError
import local_flutter_execution
import local_go_plan
import local_go_full_integration
import local_golangci_execution
import local_heavy_slots
import local_overlay_execution
import local_proto_execution
from local_process_supervision import SupervisionError
import local_source_snapshot


def _handoff_refs(root, base, head, head_ref):
    """Require the checkout and locally fetched PR refs to retain their identities."""
    if (not re.fullmatch(r'[0-9a-f]{40}', base) or not re.fullmatch(r'[0-9a-f]{40}', head) or
            not isinstance(head_ref, str) or not head_ref or head_ref.startswith('-')):
        raise ValueError('full base, head and branch identities are required')
    def git(*args):
        result = local_current_diff._execute(['git', '-C', str(root), *args], timeout=30)
        if result.returncode:
            raise ValueError('handoff Git identity check failed')
        return result.stdout.decode('ascii', errors='strict').strip()
    git('check-ref-format', 'refs/heads/' + head_ref)
    if (git('rev-parse', '--verify', 'HEAD^{commit}') != head or
            git('rev-parse', '--verify', 'refs/remotes/origin/master^{commit}') != base or
            git('rev-parse', '--verify', 'refs/remotes/origin/' + head_ref + '^{commit}') != head):
        raise ValueError('checkout or locally fetched PR refs differ')


def _bounded_file(path, limit):
    if path.is_symlink() or not path.is_file() or path.stat().st_size > limit:
        raise ValueError('overlay evidence file is unsafe or oversized')
    raw = path.read_bytes()
    if len(raw) > limit:
        raise ValueError('overlay evidence file is oversized')
    return raw


def _verify_overlay_evidence(output, source, image_id, report):
    """Recheck actual worker logs, source archive and exact owned cleanup."""
    if (not isinstance(report, dict) or report.get('schema') != 'voice-local-overlay-diagnostic-v1' or
            report.get('status') != 'incomplete' or report.get('source') != source or
            report.get('image_id') != image_id or report.get('modules') != list(local_overlay_execution.MODULES) or
            report.get('kubectl_sha256') != local_overlay_execution.KUBECTL_SHA256 or
            not isinstance(report.get('version'), str) or not re.fullmatch(r'Python 3\.12\.[0-9]+', report['version']) or
            type(report.get('slot')) is not int or report['slot'] not in (0, 1)):
        raise ValueError('overlay diagnostic identity differs')
    summary = report.get('tests')
    if (not isinstance(summary, dict) or type(summary.get('discovered')) is not int or type(summary.get('passed')) is not int or
            summary['discovered'] < 1 or summary.get('passed') != summary['discovered'] or summary.get('skipped') != []):
        raise ValueError('overlay test inventory differs')
    discovery = _bounded_file(output / 'discovery.stdout', local_overlay_execution.OUTPUT_LIMIT)
    kubectl = _bounded_file(output / 'kubectl.stdout', local_overlay_execution.OUTPUT_LIMIT)
    stdout = _bounded_file(output / 'tests.stdout', local_overlay_execution.OUTPUT_LIMIT)
    stderr = _bounded_file(output / 'tests.stderr', local_overlay_execution.OUTPUT_LIMIT)
    if (kubectl != (local_overlay_execution.KUBECTL_SHA256 + '  /workspace/repo/tools/kubectl\n').encode('ascii') or
            local_overlay_execution.validate_discovery(discovery) != summary['discovered'] or
            local_overlay_execution.validate_summary(stderr, 0, summary['discovered']) != summary['passed'] or
            hashlib.sha256(stdout).hexdigest() != report.get('stdout_sha256') or
            hashlib.sha256(stderr).hexdigest() != report.get('stderr_sha256')):
        raise ValueError('overlay test logs differ from diagnostic')
    archive = output / 'source.tar'
    if archive.is_symlink() or not archive.is_file() or archive.stat().st_size > 2 * 1024**3:
        raise ValueError('overlay source archive is unsafe or oversized')
    digest = hashlib.sha256()
    with archive.open('rb') as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            digest.update(block)
    if digest.hexdigest() != report.get('source_archive_sha256'):
        raise ValueError('overlay source archive differs')
    journal = report.get('ownership_journal')
    if not isinstance(journal, str):
        raise ValueError('overlay ownership journal is missing')
    parts = Path(journal).parts
    if (len(parts) != 5 or parts[:3] != ('run', 'tmp', 'ci-local') or
            not re.fullmatch(r'voiceci-[0-9a-f]{32}', parts[3]) or parts[4] != 'ownership.json'):
        raise ValueError('overlay ownership journal path differs')
    directory = (output / Path(*parts[:-1])).resolve()
    if output.resolve() not in directory.parents:
        raise ValueError('overlay ownership journal escapes output')
    engine = DockerCLI()
    owner = OwnedRun.load(directory, engine)
    try:
        resources = owner.state['resources']
        if (sorted(item['kind'] for item in resources) != ['container', 'container', 'network', 'volume'] or
                any(item['state'] != 'removed' or not item['id'] for item in resources) or
                any(engine.exists(item['kind'], item['id']) for item in resources)):
            raise ValueError('overlay owned resources were not removed')
        owner._check_daemon()
    finally:
        owner.close()
    slot_path = local_heavy_slots.host_slot_directory() / ('heavy-' + str(report['slot']) + '.lock')
    if slot_path.is_symlink():
        raise ValueError('overlay slot path is unsafe')
    descriptor = os.open(slot_path, os.O_RDWR | getattr(os, 'O_BINARY', 0) | getattr(os, 'O_NOFOLLOW', 0))
    try:
        if not stat.S_ISREG(os.fstat(descriptor).st_mode) or not local_heavy_slots._try_lock(descriptor):
            raise ValueError('overlay slot is not available')
        if local_heavy_slots._marker(descriptor) != b'idle\n':
            raise ValueError('overlay slot has not been released')
    finally:
        os.close(descriptor)


def run_handoff(root, base, head, head_ref, output, *, overlay_image_id=None, overlay_kubectl=None):
    """Execute and verify a supported current-diff or overlay local plan.

    Other selected checks have no accepted evidence producer and fail closed.
    The caller must fetch origin immediately before invoking this function.
    """
    root, output = Path(root).resolve(), Path(output).resolve()
    if not local_current_diff._windows():
        raise ValueError('handoff requires Windows descendant supervision')
    if output.exists() or output == root or root in output.parents:
        raise ValueError('handoff output must be new and outside the checkout')
    _handoff_refs(root, base, head, head_ref)
    plan = affected_check_plan.build_local_plan(root, base, head, head_ref)
    selected = set(plan.get('checks', {}))
    if selected not in ({'current-diff-contract'}, {'current-diff-contract', 'match-squad-overlay'}):
        raise ValueError('unsupported selected checks prevent local handoff')
    if ('match-squad-overlay' in selected and overlay_image_id != local_overlay_execution.PYTHON_IMAGE):
        raise ValueError('pinned overlay image is required')
    if 'match-squad-overlay' in selected and not overlay_kubectl:
        raise ValueError('pinned overlay kubectl is required')
    if 'match-squad-overlay' not in selected and (overlay_image_id is not None or overlay_kubectl is not None):
        raise ValueError('overlay image supplied for an unselected check')
    if 'match-squad-overlay' in selected:
        local_chat_orchestration._verify_file(overlay_kubectl, local_overlay_execution.KUBECTL_SHA256)
    source = plan['source']
    if source.get('head') != head or source.get('changed_paths') or source.get('raw_mismatch_paths'):
        raise ValueError('handoff requires clean committed source')
    output.mkdir(parents=True)
    proof = local_current_diff.run_merge_candidate(root, source, base, output / 'current-diff')
    contract = proof.get('contract', {})
    if (proof.get('source') != source or contract.get('base') != base or
            contract.get('event_base') != base or contract.get('pr_head') != head):
        raise ValueError('merge-candidate proof differs from the plan')
    if 'match-squad-overlay' in selected:
        overlay = local_overlay_execution.run_supervised(root, plan, output / 'overlay', overlay_image_id, overlay_kubectl,
                                                          base=base, head_ref=head_ref)
        _verify_overlay_evidence(output / 'overlay', source, overlay_image_id, overlay)
    if affected_check_plan.build_local_plan(root, base, head, head_ref) != plan:
        raise ValueError('source or selected checks changed during handoff verification')
    _handoff_refs(root, base, head, head_ref)
    report = {'schema': 'voice-local-handoff-v1', 'status': 'complete', 'base': base,
              'head': head, 'head_ref': head_ref, 'source': source,
              'checks': {'current-diff-contract': 'current-diff/diagnostic.json'},
              'limitations': ['locally fetched refs are not a live remote observation',
                              'GitHub merge-candidate checks remain mandatory']}
    if 'match-squad-overlay' in selected:
        report['checks']['match-squad-overlay'] = 'overlay/diagnostic.json'
        report['limitations'].extend(['Python image publisher provenance is not authenticated',
                                      'exclusive Docker daemon and socket custody is assumed',
                                      'interruption and crash recovery acceptance is not established'])
    (output / 'handoff.json').write_text(json.dumps(report, indent=2, sort_keys=True) + '\n', encoding='utf-8')
    return report


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
    make_tool = identities.get('make_tool') if isinstance(identities, dict) else None
    expected_images = {tag: {'image_id': item['image_id'], 'archive_sha256': item['sha256']}
                       for tag, item in images.items()}
    if (result.get('schema') != 'voice-local-auth-full-orchestration-v1' or result.get('status') != 'incomplete' or
            not isinstance(diagnostic, dict) or diagnostic.get('schema') != 'voice-local-auth-full-diagnostic-v1' or
            diagnostic.get('status') != 'incomplete' or diagnostic.get('source') != source or diagnostic.get('exit_code') != 0 or
            diagnostic.get('maven_cache_sha256') != inputs['maven_cache_sha256'] or not isinstance(identities, dict) or
            identities.get('creator_sha256') != inputs['helper_sha256'] or identities.get('client_image_id') != inputs['client_image_id'] or
            identities.get('maven_cache_sha256') != inputs['maven_cache_sha256'] or identities.get('images') != expected_images or
            not isinstance(make_tool, dict) or set(make_tool) != {'image_id', 'sha256', 'version'} or
            make_tool.get('image_id') != local_auth_full_orchestration.MAKE_IMAGE or
            not isinstance(make_tool.get('sha256'), str) or not re.fullmatch(r'[0-9a-f]{64}', make_tool['sha256']) or
            not isinstance(make_tool.get('version'), str) or not re.fullmatch(r'GNU Make [0-9][^\r\n]*', make_tool['version']) or
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


def _validate_full_go_inputs(plan, inputs):
    components = plan.get('components')
    selected = components.get('go_integration') if isinstance(components, dict) else None
    if ('backend-go-integration-pr' not in plan.get('checks', {}) or not isinstance(selected, list) or
            not selected or any(not isinstance(name, str) for name in selected) or len(set(selected)) != len(selected)):
        raise ValueError('full Go diagnostic requires selected integration services')
    for service in selected:
        local_go_full_integration.profile(service)
    if not isinstance(inputs, dict) or set(inputs) != set(selected):
        raise ValueError('full Go inputs must match all selected services')
    for service in selected:
        row = inputs[service]
        if (not isinstance(row, dict) or set(row) != {'helper', 'helper_sha256', 'archives'} or
                not isinstance(row['helper'], str) or not Path(row['helper']).is_absolute() or
                not isinstance(row['helper_sha256'], str) or not re.fullmatch(r'[0-9a-f]{64}', row['helper_sha256'])):
            raise ValueError('invalid full Go helper input')
        local_go_full_integration.chat.validate_archives(row['archives'], local_go_full_integration.profile(service)['images'])
        local_chat_orchestration._verify_file(row['helper'], row['helper_sha256'])
        for image in row['archives'].values():
            local_chat_orchestration._verify_file(image['archive'], image['archive_sha256'])
    return selected


def _validate_full_go_report(result, service, source, inputs, output):
    expected = {tag: item['image_id'] for tag, item in inputs['archives'].items()}
    expected_inputs = {'helper_sha256': inputs['helper_sha256'],
                       'archives': {tag: {'archive_sha256': item['archive_sha256'], 'image_id': item['image_id']}
                                    for tag, item in inputs['archives'].items()}}
    if (not isinstance(result, dict) or result.get('schema') != 'voice-local-' + service + '-full-integration-v1' or
            result.get('status') != 'incomplete' or result.get('source') != source or result.get('exit_code') != 0 or
            result.get('images') != expected or result.get('input_identity') != expected_inputs or
            type(result.get('required_count')) is not int or result['required_count'] < 1 or
            any(not isinstance(result.get(key), str) or not re.fullmatch(r'[0-9a-f]{64}', result[key])
                for key in ('events_sha256', 'source_archive_sha256', 'required_inventory_sha256')) or
            not isinstance(result.get('go_image_id'), str) or not re.fullmatch(r'sha256:[0-9a-f]{64}', result['go_image_id']) or
            type(result.get('slot')) is not int or result['slot'] not in (0, 1) or
            not isinstance(result.get('unbound_inputs'), list) or not result['unbound_inputs'] or
            any(not isinstance(item, str) or not item for item in result['unbound_inputs'])):
        raise ValueError('full Go diagnostic identity or status differs')
    tests, inventory = result.get('tests'), result.get('inventory')
    if (not isinstance(tests, dict) or type(tests.get('passed')) is not int or tests['passed'] < 1 or
            not isinstance(tests.get('skipped'), list) or not isinstance(tests.get('packages'), list) or
            not isinstance(inventory, dict) or not isinstance(inventory.get('packages_with_tests'), list) or
            not isinstance(inventory.get('packages_without_tests'), list) or
            tests['packages'] != sorted(inventory['packages_with_tests'] + inventory['packages_without_tests']) or
            any(not isinstance(pkg, str) or not (pkg == 'voice/backend/' + service or
                pkg.startswith('voice/backend/' + service + '/')) for pkg in tests['packages']) or
            any(not isinstance(item, list) or len(item) != 2 or item[0] not in tests['packages'] or
                not isinstance(item[1], str) or not item[1] for item in tests['skipped']) or
            len({tuple(item) for item in tests['skipped']}) != len(tests['skipped'])):
        raise ValueError('full Go test inventory differs')
    journal = result.get('ownership_journal')
    if not isinstance(journal, str):
        raise ValueError('full Go ownership journal missing')
    path = Path(journal)
    try:
        relative = path.relative_to(output)
    except ValueError:
        raise ValueError('full Go ownership journal outside output') from None
    if (not path.is_absolute() or '..' in relative.parts or len(relative.parts) != 5 or
            relative.parts[:3] != ('run', 'tmp', 'ci-local') or
            not re.fullmatch(r'voiceci-[0-9a-f]{32}', relative.parts[3]) or relative.parts[4] != 'ownership.json'):
        raise ValueError('full Go ownership journal identity differs')


def run(root, base, head, output, flutter, head_ref='', go_binary='go', buf_binary='buf', chat_inputs=None,
        auth_inputs=None, compose_inputs=None, staging_assets=None, runtime_images=None, full_auth_inputs=None,
        full_go_inputs=None, lint_inputs=None, overlay_inputs=None):
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
        if overlay_inputs is not None:
            if (not isinstance(overlay_inputs, dict) or set(overlay_inputs) != {'image_id', 'kubectl'} or
                    overlay_inputs['image_id'] != local_overlay_execution.PYTHON_IMAGE or
                    not overlay_inputs['kubectl'] or
                    'match-squad-overlay' not in plan['checks']):
                raise ValueError('planner-selected overlay requires pinned Python image identity')
        if lint_inputs is not None:
            keys = {'go', 'go_sha256', 'lint', 'lint_sha256'}
            if (not isinstance(lint_inputs, dict) or set(lint_inputs) != keys or
                    any(not isinstance(lint_inputs[k], str) or not lint_inputs[k] for k in keys) or
                    any(not Path(lint_inputs[k]).is_absolute() for k in ('go', 'lint')) or
                    any(not re.fullmatch(r'[0-9a-f]{64}', lint_inputs[k]) for k in ('go_sha256', 'lint_sha256')) or
                    'golangci' not in plan['checks']):
                raise ValueError('planner-selected lint requires absolute tools and exact digests')
            local_chat_orchestration._verify_file(lint_inputs['go'], lint_inputs['go_sha256'])
            local_chat_orchestration._verify_file(lint_inputs['lint'], lint_inputs['lint_sha256'])
        selected_full_go = _validate_full_go_inputs(plan, full_go_inputs) if full_go_inputs is not None else []
        for service in selected_full_go:
            values = full_go_inputs[service]
            service_output = output / 'go-full' / service
            diagnostic = local_go_full_integration.run_supervised(plan, service, root, service_output,
                values['helper'], values['helper_sha256'], values['archives'], go_binary, base=base, head_ref=head_ref)
            _validate_full_go_report(diagnostic, service, plan['source'], values, service_output)
            if local_source_snapshot.capture(root, head) != plan['source']:
                raise ValueError('source changed during full Go diagnostic')
            report.setdefault('go_full_reports', {})[service] = {
                'report': 'go-full/' + service + '/diagnostic.json', 'skipped': diagnostic['tests']['skipped']}
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
        if overlay_inputs is not None:
            diagnostic = local_overlay_execution.run_supervised(root, plan, output / 'overlay', base=base,
                                                                head_ref=head_ref, **overlay_inputs)
            tests = diagnostic.get('tests') if isinstance(diagnostic, dict) else None
            journal = diagnostic.get('ownership_journal') if isinstance(diagnostic, dict) else None
            if (not isinstance(diagnostic, dict) or diagnostic.get('schema') != 'voice-local-overlay-diagnostic-v1' or
                    diagnostic.get('status') != 'incomplete' or diagnostic.get('source') != plan['source'] or
                    diagnostic.get('modules') != list(local_overlay_execution.MODULES) or
                    diagnostic.get('image_id') != overlay_inputs['image_id'] or
                    diagnostic.get('kubectl_sha256') != local_overlay_execution.KUBECTL_SHA256 or
                    not isinstance(diagnostic.get('version'), str) or
                    not re.fullmatch(r'Python 3\.12\.[0-9]+', diagnostic['version']) or
                    type(diagnostic.get('slot')) is not int or diagnostic['slot'] not in (0, 1) or
                    not isinstance(journal, str) or Path(journal).is_absolute() or '..' in Path(journal).parts or
                    not Path(journal).parts or Path(journal).parts[0] != 'run' or Path(journal).name != 'ownership.json' or
                    not isinstance(tests, dict) or type(tests.get('discovered')) is not int or
                    tests['discovered'] < 1 or tests.get('passed') != tests['discovered'] or tests.get('skipped') != [] or
                    any(not isinstance(diagnostic.get(key), str) or not re.fullmatch(r'[0-9a-f]{64}', diagnostic[key])
                        for key in ('stdout_sha256', 'stderr_sha256', 'source_archive_sha256')) or
                    not isinstance(diagnostic.get('unbound_inputs'), list) or not diagnostic['unbound_inputs']):
                raise ValueError('overlay diagnostic does not match selected source and inventory')
            report['overlay_report'] = 'overlay/diagnostic.json'
        if lint_inputs is not None:
            diagnostic = local_golangci_execution.run(root, plan, output / 'golangci', base=base, head_ref=head_ref,
                                                      **lint_inputs)
            expected_modules = local_golangci_execution.modules_for(plan)
            commands = diagnostic.get('commands') if isinstance(diagnostic, dict) else None
            expected = [(action, module) for action in ('resolve', 'lint') for module in expected_modules]
            if (not isinstance(diagnostic, dict) or diagnostic.get('schema') != 'voice-local-golangci-diagnostic-v1' or
                    diagnostic.get('status') != 'incomplete' or diagnostic.get('source') != plan['source'] or
                    diagnostic.get('modules') != expected_modules or not isinstance(commands, list) or
                    diagnostic.get('tools') != {'go_sha256': lint_inputs['go_sha256'],
                                                'golangci_sha256': lint_inputs['lint_sha256']} or
                    diagnostic.get('platform') != {'goos': 'windows', 'goarch': 'amd64'} or
                    any(not isinstance(row, dict) for row in commands) or
                    [(row.get('action'), row.get('module')) for row in commands if isinstance(row, dict)] != expected or
                    any(row.get('exit_code') != 0 or
                        any(not isinstance(row.get(key), str) or not re.fullmatch(r'[0-9a-f]{64}', row[key])
                            for key in ('stdout_sha256', 'stderr_sha256')) for row in commands)):
                raise ValueError('lint diagnostic does not match selected source and module inventory')
            report['golangci_report'] = 'golangci/diagnostic.json'
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
    parser.add_argument('--handoff', action='store_true', help='run a supported complete current-diff or overlay handoff plan')
    parser.add_argument('--ci-staging-assets', help='JSON mapping of pinned local staging inputs; runtime also requires phase0_jdk and phase0_compose')
    parser.add_argument('--ci-runtime-images', help='JSON mapping of three pinned runtime-shard image archives')
    parser.add_argument('--go-integration-inputs', help='JSON file mapping every selected supported Go integration service to helper and exact image archives')
    for name in ('go', 'go-sha256', 'lint', 'lint-sha256'):
        parser.add_argument('--golangci-' + name, help='opt-in source-bound Go lint tool input')
    parser.add_argument('--overlay-image-id', help='pinned Linux Python image ID for overlay diagnostic')
    parser.add_argument('--overlay-kubectl', help='absolute path to the pinned Linux kubectl binary')
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
    if args.handoff:
        allowed = {'base', 'head', 'head_ref', 'output', 'flutter', 'go', 'buf', 'handoff', 'overlay_image_id', 'overlay_kubectl'}
        if any(value is not None for key, value in vars(args).items() if key not in allowed):
            parser.error('diagnostic inputs cannot be combined with --handoff')
        report = run_handoff(Path(__file__).resolve().parents[2], args.base, args.head, args.head_ref, args.output,
                             overlay_image_id=args.overlay_image_id, overlay_kubectl=args.overlay_kubectl)
        print(json.dumps(report, sort_keys=True))
        return 0
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
    full_go_inputs = None
    if args.go_integration_inputs:
        path = Path(args.go_integration_inputs)
        if not path.is_absolute():
            raise ValueError('Go integration inputs path must be absolute')
        with path.open('rb') as stream:
            raw = stream.read(1024 * 1024 + 1)
        if len(raw) > 1024 * 1024:
            raise ValueError('Go integration inputs exceed limit')
        full_go_inputs = json.loads(raw, object_pairs_hook=local_go_full_integration.chat._unique)
    lint_inputs = {'go': args.golangci_go, 'go_sha256': args.golangci_go_sha256,
                   'lint': args.golangci_lint, 'lint_sha256': args.golangci_lint_sha256}
    overlay_inputs = {'image_id': args.overlay_image_id, 'kubectl': args.overlay_kubectl}
    report = run(Path(__file__).resolve().parents[2], args.base, args.head, args.output, args.flutter,
                 args.head_ref, args.go, args.buf, chat if any(value is not None for value in chat.values()) else None,
                 auth if any(value is not None for value in auth.values()) else None,
                 compose if any(value is not None for value in compose.values()) else None, staging_assets, runtime_images,
                 full_auth if any(value is not None for value in full_auth.values()) else None, full_go_inputs,
                 lint_inputs if any(value is not None for value in lint_inputs.values()) else None,
                 overlay_inputs if any(value is not None for value in overlay_inputs.values()) else None)
    print(json.dumps(report, sort_keys=True))
    return 1 if report['status'] == 'failed' else 2


if __name__ == '__main__':
    raise SystemExit(main())
