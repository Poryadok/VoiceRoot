"""Fixed Kubernetes adapter for the canonical NATS contract migration plan.

All authorization, fencing, storage custody, and durable orchestration state
remain owned by the trusted host transaction. This module only drives the
producer's closed same-Pod API and never accepts shell commands or paths.
"""
from __future__ import annotations

import copy
import hashlib
import json
import re
from datetime import datetime, timezone

import nats_contract_plan
import nats_contract_runner
import verification_job


MAX_EVENTS = 10000
MAX_POD_NAME = re.compile(r'[a-z0-9](?:[a-z0-9.-]{0,251}[a-z0-9])?')


class NativeContractRuntimeError(RuntimeError):
    """A safe, non-sensitive failure code for the fixed native contract path."""


def _fail(code: str) -> None:
    raise NativeContractRuntimeError('native_contract_' + code)


def _canonical(value: object) -> bytes:
    try:
        return json.dumps(value, sort_keys=True, separators=(',', ':'),
            ensure_ascii=False, allow_nan=False).encode('utf-8')
    except (TypeError, ValueError, UnicodeError):
        _fail('json_invalid')


def _parse_line(raw: object) -> dict:
    """Accept exactly one bounded canonical receipt line from the helper."""
    if not isinstance(raw, bytes) or not 1 <= len(raw) <= nats_contract_runner.MAX_RESPONSE_BYTES:
        _fail('response_size')
    if not raw.endswith(b'\n') or raw.count(b'\n') != 1:
        _fail('response_framing')

    def pairs(items):
        result = {}
        for key, value in items:
            if key in result:
                _fail('response_duplicate_key')
            result[key] = value
        return result

    try:
        value = json.loads(raw[:-1].decode('utf-8'), object_pairs_hook=pairs,
            parse_constant=lambda _value: _fail('response_constant'))
    except NativeContractRuntimeError:
        raise
    except (ValueError, UnicodeError, TypeError):
        _fail('response_json')
    if not isinstance(value, dict) or _canonical(value) + b'\n' != raw:
        _fail('response_not_canonical')
    return value


def _identity(request: object, binding: object, *, pod_name: object, pod_uid: object,
        job_uid: object, contract_attempt: object, runner_image: object) -> tuple[dict, dict, dict]:
    try:
        row = verification_job.validate_request(request)
        storage = verification_job.validate_contract_binding(binding, row['operation'])
    except Exception:
        _fail('request_or_binding_invalid')
    if row['phase'] != 'contract-migration':
        _fail('phase_invalid')
    if contract_attempt != storage['contract_attempt']:
        _fail('attempt_mismatch')
    if (not isinstance(pod_uid, str) or not verification_job.UUID_RE.fullmatch(pod_uid)
        or not isinstance(job_uid, str) or not verification_job.UUID_RE.fullmatch(job_uid)):
        _fail('pod_or_job_uid_invalid')
    if (not isinstance(pod_name, str) or not MAX_POD_NAME.fullmatch(pod_name)
        or not pod_name.startswith(verification_job.job_name(row) + '-')):
        _fail('pod_name_invalid')
    try:
        image = verification_job.validate_image(runner_image)
    except Exception:
        _fail('runner_image_invalid')
    return row, storage, {
        'namespace': verification_job.NAMESPACE, 'pod_name': pod_name,
        'pod_uid': pod_uid, 'job_uid': job_uid, 'runner_image': image,
    }


def _runner_binding(request: dict, storage: dict, pod_uid: str) -> dict:
    binding = {
        'operation': request['operation'], 'nonce': request['nonce'], 'pod_uid': pod_uid,
        'contract_attempt': storage['contract_attempt'],
        'source_pvc_uid': storage['source_uid'], 'source_pv_uid': storage['source_pv_uid'],
        'verification_pvc_uid': storage['verification_uid'],
        'verification_pv_uid': storage['verification_pv_uid'],
    }
    for key in ('input_bundle_sha256', 'input_bundle_bytes', 'archive_sha256', 'archive_bytes',
        'server_config_sha256', 'server_config_bytes', 'plan_sha256', 'plan_bytes',
        'bootstrap_creds_sha256', 'bootstrap_creds_bytes'):
        binding[key] = storage[key]
    return binding


def _validate_input(raw: object, expected: dict, plan: object) -> dict:
    if not isinstance(raw, bytes) or len(raw) > nats_contract_runner.MAX_INPUT_BYTES:
        _fail('input_size')
    try:
        checked = nats_contract_runner.validate_envelope(raw, expected=expected)
        validated = nats_contract_runner.validate_plan(plan)
    except Exception:
        _fail('input_or_plan_invalid')
    if (checked['plan'] != validated or checked['plan_sha256'] != nats_contract_plan.digest(validated)
        or expected['plan_sha256'] != nats_contract_plan.digest(validated)):
        _fail('plan_binding')
    return checked


def _input_receipt(expected: dict) -> dict:
    return {
        'schema': nats_contract_runner.INPUT_RECEIPT_SCHEMA,
        'phase': 'contract-inputs', 'purpose': 'nats-contract-migration',
        'operation': expected['operation'], 'nonce': expected['nonce'],
        'pod_uid': expected['pod_uid'], 'contract_attempt': expected['contract_attempt'],
        'source_pvc_uid': expected['source_pvc_uid'], 'source_pv_uid': expected['source_pv_uid'],
        'verification_pvc_uid': expected['verification_pvc_uid'],
        'verification_pv_uid': expected['verification_pv_uid'],
        'archive_sha256': expected['archive_sha256'], 'archive_bytes': expected['archive_bytes'],
        'input_bundle_sha256': expected['input_bundle_sha256'],
        'input_bundle_bytes': expected['input_bundle_bytes'], 'status': 'COMPLETE',
        'plan_sha256': expected['plan_sha256'], 'plan_bytes': expected['plan_bytes'],
        'server_config_sha256': expected['server_config_sha256'],
        'server_config_bytes': expected['server_config_bytes'],
        'bootstrap_creds_sha256': expected['bootstrap_creds_sha256'],
        'bootstrap_creds_bytes': expected['bootstrap_creds_bytes'], 'file_count': 2,
    }


def _expect_input_receipt(row: dict, expected: dict) -> dict:
    wanted = _input_receipt(expected)
    if row != wanted or set(row) != set(wanted):
        _fail('input_receipt_binding')
    return row


def _broker_receipt(row: dict, expected: dict, action: str) -> dict:
    required = {'schema': nats_contract_runner.RECEIPT_SCHEMA, **expected,
        'status': 'COMPLETE', 'action': 'broker-' + action, 'broker': None}
    if set(row) != set(required) or any(row.get(key) != value for key, value in required.items()
        if key != 'broker'):
        _fail('broker_receipt_binding')
    broker = row.get('broker')
    if not isinstance(broker, dict):
        _fail('broker_receipt_shape')
    if action == 'start':
        if (set(broker) != {'schema', 'ok', 'running', 'pid', 'started_at_ns'}
            or broker.get('schema') != 'voice-nats-supervisor-v1' or broker.get('ok') is not True
            or broker.get('running') is not True or type(broker.get('pid')) is not int
            or broker['pid'] <= 0 or type(broker.get('started_at_ns')) is not int
            or broker['started_at_ns'] <= 0):
            _fail('broker_start_identity')
    elif action == 'stop':
        if (set(broker) != {'schema', 'ok', 'running', 'pid', 'started_at_ns', 'finished_at_ns'}
            or broker.get('schema') != 'voice-nats-supervisor-v1' or broker.get('ok') is not True
            or broker.get('running') is not False or type(broker.get('pid')) is not int
            or broker['pid'] <= 0 or type(broker.get('started_at_ns')) is not int
            or broker['started_at_ns'] <= 0 or type(broker.get('finished_at_ns')) is not int
            or broker['finished_at_ns'] < broker['started_at_ns']):
            _fail('broker_stop_identity')
    else:
        _fail('broker_action_invalid')
    return row


def _broker_status(raw: object, expected: dict) -> dict:
    row = _parse_line(raw) if isinstance(raw, bytes) else raw
    if not isinstance(row, dict):
        _fail('broker_status_shape')
    required = {'schema': nats_contract_runner.RECEIPT_SCHEMA, **expected,
        'status': 'READ_ONLY', 'action': 'broker-status', 'broker': None}
    if set(row) != set(required) or any(row.get(key) != value for key, value in required.items()
        if key != 'broker'):
        _fail('broker_status_binding')
    broker = row.get('broker')
    if (not isinstance(broker, dict) or set(broker) != {'schema', 'ok', 'running', 'pid', 'started_at_ns'}
        or broker.get('schema') != 'voice-nats-supervisor-v1' or broker.get('ok') is not True
        or type(broker.get('running')) is not bool):
        _fail('broker_status_shape')
    if broker['running']:
        if type(broker.get('pid')) is not int or broker['pid'] <= 0 or type(broker.get('started_at_ns')) is not int or broker['started_at_ns'] <= 0:
            _fail('broker_status_identity')
    elif broker.get('pid') is not None or broker.get('started_at_ns') is not None:
        _fail('broker_status_identity')
    return broker


def _now() -> str:
    return datetime.now(timezone.utc).isoformat()


def _matching_action_events(events: list, action: dict, attempt: str) -> tuple[dict | None, dict | None, dict | None]:
    by_kind = {'nats_contract_intent': [], 'nats_contract_mutation_issued': [], 'nats_contract_applied': []}
    for row in events:
        if not isinstance(row, dict) or row.get('kind') not in by_kind:
            continue
        if row.get('object') != action['object']:
            continue
        if row.get('migration_attempt') != attempt or row.get('api') not in (None, action['api']):
            _fail('prior_action_attempt_conflict')
        by_kind[row['kind']].append(row)
    if any(len(rows) > 1 for rows in by_kind.values()):
        _fail('prior_action_duplicate_event')
    return tuple(rows[0] if rows else None for rows in (
        by_kind['nats_contract_intent'], by_kind['nats_contract_mutation_issued'],
        by_kind['nats_contract_applied']))


def _validate_history_event(event: dict, wanted: dict, *, kind: str, attempt: str) -> None:
    expected = {'kind': kind, **wanted, 'migration_attempt': attempt}
    if event != expected or set(event) != set(expected):
        _fail('prior_action_event_changed')


def _inspection(raw: object, action: dict, index: int) -> dict:
    row = _parse_line(raw) if isinstance(raw, bytes) else raw
    if not isinstance(row, dict):
        _fail('inspection_shape')
    target = action.get('normalized_config_sha256', nats_contract_plan.digest(action['after']))
    required = {'schema', 'action_index', 'object', 'api', 'before_sha256', 'target_sha256', 'current', 'state'}
    if (set(row) != required or row.get('schema') != 'voice-nats-contract-inspection-v1'
        or row.get('action_index') != index or row.get('object') != action['object']
        or row.get('api') != action['api'] or row.get('target_sha256') != target):
        _fail('inspection_binding')
    current = row.get('current')
    if current is not None and not isinstance(current, dict):
        _fail('inspection_current_shape')
    if current is None:
        before_hash = None
    else:
        before_hash = nats_contract_plan.digest(current)
    if row.get('before_sha256') != before_hash:
        _fail('inspection_digest')
    if current is not None and before_hash == target:
        expected_state = 'TARGET'
    elif current == action['before']:
        expected_state = 'BEFORE'
    else:
        expected_state = 'DRIFT'
    if row.get('state') != expected_state:
        _fail('inspection_state')
    return row


def _action_receipt(raw: object, expected: dict, action: dict, index: int,
        intent_sha256: str, issued_sha256: str) -> dict:
    row = _parse_line(raw) if isinstance(raw, bytes) else raw
    if not isinstance(row, dict):
        _fail('action_receipt_shape')
    target = action.get('normalized_config_sha256', nats_contract_plan.digest(action['after']))
    required = {'schema': nats_contract_runner.RECEIPT_SCHEMA, **expected,
        'status': 'COMPLETE', 'action_index': index, 'object': action['object'], 'api': action['api'],
        'before_event_sha256': intent_sha256, 'issued_event_sha256': issued_sha256,
        'target_sha256': target, 'config_sha256': target, 'recovered_exact_target': False}
    if row != required or set(row) != set(required):
        _fail('action_receipt_binding')
    return row


def _public_action_receipt(action: dict, target_sha256: str) -> dict:
    return {'object': action['object'], 'config_sha256': target_sha256}


def apply_contract(api, request: dict, binding: dict, *, pod_name: str, pod_uid: str,
        job_uid: str, runner_image: str, validated_plan: dict, plan_envelope_bytes: bytes,
        contract_attempt: str, journal_event, revalidate, journal_events=()) -> list[dict]:
    """Apply only the already-authorized contract plan through fixed typed API calls.

    `journal_event` must durably save before returning. `journal_events` is the
    current durable host journal snapshot; issued actions are reconciliation-only.
    `revalidate` rechecks the source/fence/job/Pod/PVC binding at every boundary.
    """
    row, storage, identity = _identity(request, binding, pod_name=pod_name,
        pod_uid=pod_uid, job_uid=job_uid, contract_attempt=contract_attempt,
        runner_image=runner_image)
    if row['operation'] != request['operation'] or row['nonce'] != request['nonce']:
        _fail('request_changed')
    if not callable(journal_event) or not callable(revalidate):
        _fail('callbacks_invalid')
    if not isinstance(journal_events, (list, tuple)) or len(journal_events) > MAX_EVENTS:
        _fail('journal_snapshot_invalid')
    history = [copy.deepcopy(event) for event in journal_events]
    expected = _runner_binding(row, storage, pod_uid)
    checked_input = _validate_input(plan_envelope_bytes, expected, validated_plan)
    plan = checked_input['plan']
    attempt = storage['contract_attempt']
    plan_digest = nats_contract_plan.digest(plan)

    def append(event: dict) -> None:
        try:
            journal_event(copy.deepcopy(event))
        except Exception:
            _fail('host_journal_write_failed')
        history.append(copy.deepcopy(event))

    def call_read(receipt_kind: str, action_index: int | None = None) -> dict:
        try:
            revalidate()
            raw = api.read_contract_receipt(identity['namespace'], pod_name, pod_uid,
                row, storage, attempt, receipt_kind, action_index=action_index)
        except Exception:
            _fail('receipt_read_unknown')
        return _parse_line(raw)

    def call_action(action_name: str, *, action_index: int | None = None,
            journal_hashes: dict | None = None) -> dict:
        try:
            revalidate()
            raw = api.exec_pod_contract(identity['namespace'], pod_name, pod_uid,
                row, storage, action_name, action_index=action_index,
                journal_hashes=journal_hashes)
        except Exception:
            _fail('exec_outcome_unknown')
        return _parse_line(raw)

    def plan_receipt() -> dict:
        return _expect_input_receipt(call_read('input'), expected)

    def exact_input_event(kind: str, *, include_size: bool = False) -> dict:
        event = {'kind': kind, 'contract_attempt': attempt, 'pod_uid': pod_uid,
            'plan_sha256': plan_digest, 'input_bundle_sha256': storage['input_bundle_sha256']}
        if include_size:
            event['input_bundle_bytes'] = storage['input_bundle_bytes']
        return event

    # The one-shot input action has its own durable host intent/issued fence.
    input_events = [event for event in history if isinstance(event, dict)
        and event.get('kind') in ('nats_contract_input_load_intent',
            'nats_contract_input_load_issued', 'nats_contract_input_loaded')]
    if len([e for e in input_events if e['kind'] == 'nats_contract_input_load_intent']) > 1 or \
        len([e for e in input_events if e['kind'] == 'nats_contract_input_load_issued']) > 1 or \
        len([e for e in input_events if e['kind'] == 'nats_contract_input_loaded']) > 1:
        _fail('input_journal_duplicate')
    input_issued = any(e['kind'] == 'nats_contract_input_load_issued' for e in input_events)
    input_loaded = any(e['kind'] == 'nats_contract_input_loaded' for e in input_events)
    prior_input_intent = next((e for e in input_events
        if e['kind'] == 'nats_contract_input_load_intent'), None)
    prior_input_issued = next((e for e in input_events
        if e['kind'] == 'nats_contract_input_load_issued'), None)
    prior_input_loaded = next((e for e in input_events
        if e['kind'] == 'nats_contract_input_loaded'), None)
    if input_loaded and (not input_issued or prior_input_intent is None):
        _fail('input_journal_order_invalid')
    if prior_input_intent is not None and prior_input_intent != exact_input_event(
        'nats_contract_input_load_intent', include_size=True):
        _fail('input_journal_intent_changed')
    if prior_input_issued is not None and prior_input_issued != exact_input_event(
        'nats_contract_input_load_issued'):
        _fail('input_journal_issued_changed')
    if input_loaded or input_issued:
        receipt = plan_receipt()
        if input_loaded:
            expected_loaded = {**exact_input_event('nats_contract_input_loaded'),
                'receipt_sha256': hashlib.sha256(_canonical(receipt)).hexdigest()}
            if prior_input_loaded != expected_loaded:
                _fail('input_journal_receipt_changed')
        else:
            append({**exact_input_event('nats_contract_input_loaded'),
                'receipt_sha256': hashlib.sha256(_canonical(receipt)).hexdigest()})
    else:
        intent = exact_input_event('nats_contract_input_load_intent', include_size=True)
        prior_intent = next((e for e in input_events if e['kind'] == 'nats_contract_input_load_intent'), None)
        if prior_intent is not None and prior_intent != intent:
            _fail('input_journal_intent_changed')
        if prior_intent is None:
            revalidate()
            append(intent)
        revalidate()
        append(exact_input_event('nats_contract_input_load_issued'))
        try:
            revalidate()
            raw = api.exec_pod_contract_input(identity['namespace'], pod_name, pod_uid,
                row, storage, plan_envelope_bytes)
            load_receipt = _expect_input_receipt(_parse_line(raw), expected)
        except Exception:
            # A load may have created private state before transport failed.
            load_receipt = plan_receipt()
        durable_receipt = plan_receipt()
        if load_receipt != durable_receipt:
            _fail('input_receipt_disagrees')
        append({**exact_input_event('nats_contract_input_loaded'),
            'receipt_sha256': hashlib.sha256(_canonical(durable_receipt)).hexdigest()})

    # Bind the immutable input receipt to this original Job/Pod before starting
    # the NATS child. The helper starts idle and exposes only typed start/stop.
    if not plan['actions']:
        return []

    def broker_event(kind: str, broker: dict | None = None, *, finished_at: str | None = None) -> dict:
        if broker is None:
            broker = {}
        event = {'kind': kind, 'migration_attempt': attempt, 'container_id': attempt,
            'server_image': identity['runner_image'], 'job_uid': job_uid, 'pod_uid': pod_uid,
            'server_pid': broker.get('pid'), 'server_started_at_ns': broker.get('started_at_ns')}
        if 'started_at' in broker:
            event['started_at'] = broker['started_at']
        if finished_at is not None:
            event['finished_at'] = finished_at
        return event

    opened = [event for event in history if isinstance(event, dict)
        and event.get('kind') == 'nats_contract_broker_opened'
        and event.get('migration_attempt') == attempt]
    closed = [event for event in history if isinstance(event, dict)
        and event.get('kind') == 'nats_contract_broker_closed'
        and event.get('migration_attempt') == attempt]
    open_intent = [event for event in history if isinstance(event, dict)
        and event.get('kind') == 'nats_contract_broker_open_intent'
        and event.get('contract_attempt') == attempt]
    open_issued = [event for event in history if isinstance(event, dict)
        and event.get('kind') == 'nats_contract_broker_open_issued'
        and event.get('contract_attempt') == attempt]
    close_intent = [event for event in history if isinstance(event, dict)
        and event.get('kind') == 'nats_contract_broker_close_intent'
        and event.get('contract_attempt') == attempt]
    close_issued = [event for event in history if isinstance(event, dict)
        and event.get('kind') == 'nats_contract_broker_close_issued'
        and event.get('contract_attempt') == attempt]
    if any(len(events) > 1 for events in (opened, closed, open_intent, open_issued, close_intent, close_issued)):
        _fail('broker_journal_duplicate')
    if closed:
        _validate_closed(closed[0], attempt, job_uid, pod_uid, identity['runner_image'])
        _validate_broker_host_event(open_intent[0] if open_intent else None,
            'nats_contract_broker_open_intent', attempt, job_uid, pod_uid,
            identity['runner_image'], plan_digest)
        _validate_broker_host_event(open_issued[0] if open_issued else None,
            'nats_contract_broker_open_issued', attempt, job_uid, pod_uid,
            identity['runner_image'], plan_digest)
        close_base = {'contract_attempt': attempt, 'job_uid': job_uid, 'pod_uid': pod_uid,
            'runner_image': identity['runner_image'], 'server_pid': closed[0]['server_pid'],
            'server_started_at_ns': closed[0]['server_started_at_ns']}
        if close_intent:
            _validate_broker_host_event(close_intent[0], 'nats_contract_broker_close_intent',
                attempt, job_uid, pod_uid, identity['runner_image'], plan_digest, close_base)
        if close_issued:
            _validate_broker_host_event(close_issued[0], 'nats_contract_broker_close_issued',
                attempt, job_uid, pod_uid, identity['runner_image'], plan_digest, close_base)
        if not open_intent or not open_issued or not close_intent or not close_issued:
            _fail('closed_broker_history_incomplete')
        return _closed_receipts(api, row, storage, identity, plan, history, expected, revalidate)
    if opened:
        if not open_intent or not open_issued:
            _fail('broker_open_history_incomplete')
        _validate_broker_host_event(open_intent[0], 'nats_contract_broker_open_intent',
            attempt, job_uid, pod_uid, identity['runner_image'], plan_digest)
        _validate_broker_host_event(open_issued[0], 'nats_contract_broker_open_issued',
            attempt, job_uid, pod_uid, identity['runner_image'], plan_digest)
        _validate_opened(opened[0], attempt, job_uid, pod_uid, identity['runner_image'])
        started_receipt = _broker_receipt(call_read('broker-start'), expected, 'start')['broker']
        if (opened[0]['server_pid'] != started_receipt['pid']
            or opened[0]['server_started_at_ns'] != started_receipt['started_at_ns']
            or opened[0]['started_at'] != _time_from_ns(started_receipt['started_at_ns'])):
            _fail('broker_open_receipt_changed')
        broker_identity = {'pid': opened[0]['server_pid'], 'started_at_ns': opened[0]['server_started_at_ns']}
        revalidate()
        status = _broker_status(call_action('broker-status'), expected)
        if (not status['running'] or status['pid'] != broker_identity['pid']
            or status['started_at_ns'] != broker_identity['started_at_ns']):
            _fail('broker_identity_changed')
    else:
        if open_intent:
            _validate_broker_host_event(open_intent[0], 'nats_contract_broker_open_intent',
                attempt, job_uid, pod_uid, identity['runner_image'], plan_digest)
        if open_issued:
            if not open_intent:
                _fail('broker_open_history_incomplete')
            _validate_broker_host_event(open_issued[0], 'nats_contract_broker_open_issued',
                attempt, job_uid, pod_uid, identity['runner_image'], plan_digest)
            # Already issued: adopt only a complete same-Pod receipt and exact
            # running process identity. Never invoke broker-start a second time.
            started = _broker_receipt(call_read('broker-start'), expected, 'start')
            broker = started['broker']
            status = _broker_status(call_action('broker-status'), expected)
            if (not status['running'] or status['pid'] != broker['pid']
                or status['started_at_ns'] != broker['started_at_ns']):
                _fail('broker_start_unknown')
        else:
            if not open_intent:
                revalidate()
                append({'kind': 'nats_contract_broker_open_intent', 'contract_attempt': attempt,
                    'job_uid': job_uid, 'pod_uid': pod_uid, 'runner_image': identity['runner_image'],
                    'plan_sha256': plan_digest})
            revalidate()
            append({'kind': 'nats_contract_broker_open_issued', 'contract_attempt': attempt,
                'job_uid': job_uid, 'pod_uid': pod_uid, 'runner_image': identity['runner_image'],
                'plan_sha256': plan_digest})
            try:
                revalidate()
                _parse_line(api.exec_pod_contract(identity['namespace'], pod_name, pod_uid,
                    row, storage, 'broker-start'))
            except Exception:
                pass
            started = _broker_receipt(call_read('broker-start'), expected, 'start')
            broker = started['broker']
            status = _broker_status(call_action('broker-status'), expected)
            if (not status['running'] or status['pid'] != broker['pid']
                or status['started_at_ns'] != broker['started_at_ns']):
                _fail('broker_start_unknown')
        started_at = _time_from_ns(broker['started_at_ns'])
        opened_event = {**broker_event('nats_contract_broker_opened', broker),
            'started_at': started_at, 'readiness': 'healthz-200'}
        append(opened_event)
        broker_identity = {'pid': broker['pid'], 'started_at_ns': broker['started_at_ns']}

    applied = []
    for index, action in enumerate(plan['actions']):
        target = action.get('normalized_config_sha256', nats_contract_plan.digest(action['after']))
        intent_base = {'kind': 'nats_contract_intent', 'object': action['object'], 'api': action['api'],
            'before_sha256': nats_contract_plan.digest(action['before']), 'target_sha256': target}
        issued_base = {'kind': 'nats_contract_mutation_issued', 'object': action['object'],
            'api': action['api'], 'target_sha256': target}
        intent_event = {**intent_base, 'migration_attempt': attempt}
        issued_event = {**issued_base, 'migration_attempt': attempt}
        applied_event = {'kind': 'nats_contract_applied', 'object': action['object'],
            'config_sha256': target, 'recovered_exact_target': False,
            'migration_attempt': attempt}
        # The runner verifies hashes of the exact durable host-journal rows,
        # including migration_attempt, before it accepts an execute request.
        intent_sha = hashlib.sha256(nats_contract_runner._canonical(intent_event)).hexdigest()
        issued_sha = hashlib.sha256(nats_contract_runner._canonical(issued_event)).hexdigest()
        prior_intent, prior_issued, prior_applied = _matching_action_events(history, action, attempt)
        if prior_intent is not None:
            _validate_history_event(prior_intent, intent_base, kind='nats_contract_intent', attempt=attempt)
        if prior_issued is not None:
            _validate_history_event(prior_issued, issued_base, kind='nats_contract_mutation_issued', attempt=attempt)
        if prior_applied is not None:
            if (set(prior_applied) != {'kind', 'object', 'config_sha256',
                    'recovered_exact_target', 'migration_attempt'}
                or prior_applied.get('kind') != 'nats_contract_applied'
                or prior_applied.get('config_sha256') != target
                or type(prior_applied.get('recovered_exact_target')) is not bool
                or prior_applied.get('migration_attempt') != attempt):
                _fail('prior_action_applied_changed')
            if prior_issued is not None:
                if (prior_intent is None or history.index(prior_intent) >= history.index(prior_issued)
                    or history.index(prior_issued) >= history.index(prior_applied)):
                    _fail('prior_action_order_invalid')
            elif prior_intent is not None or not prior_applied.get('recovered_exact_target'):
                _fail('prior_action_order_invalid')
        revalidate()
        try:
            observation = _inspection(call_action('inspect-action', action_index=index), action, index)
        except Exception:
            _fail('action_inspection_unknown')
        if prior_applied is not None:
            if observation['state'] != 'TARGET':
                _fail('applied_action_drift')
            applied.append(_public_action_receipt(action, target))
            continue
        if observation['state'] == 'DRIFT':
            _fail('action_before_drift')
        if prior_issued is not None:
            # An issued event means the write may have happened. Receipt and
            # INFO reconciliation are read-only; the action is never replayed.
            try:
                receipt = _action_receipt(call_read('action', index), expected,
                    action, index, intent_sha, issued_sha)
            except Exception:
                receipt = None
            if observation['state'] != 'TARGET':
                _fail('action_outcome_unknown')
            recovered = receipt is None or receipt['recovered_exact_target']
            applied_event['recovered_exact_target'] = recovered
            append(applied_event)
            applied.append(_public_action_receipt(action, target))
            continue
        if observation['state'] == 'TARGET':
            if prior_intent is not None:
                _fail('target_after_unissued_intent')
            applied_event['recovered_exact_target'] = True
            append(applied_event)
            applied.append(_public_action_receipt(action, target))
            continue
        if prior_intent is None:
            revalidate()
            append(intent_event)
        revalidate()
        append(issued_event)
        revalidate()
        try:
            response = call_action('execute-action', action_index=index,
                journal_hashes={'intent_sha256': intent_sha, 'issued_sha256': issued_sha})
            receipt = _action_receipt(_canonical(response) + b'\n', expected,
                action, index, intent_sha, issued_sha)
        except Exception:
            # On any ambiguous transport/result, reconcile only this Pod's
            # durable action receipt and current INFO. No second execute call.
            try:
                receipt = _action_receipt(call_read('action', index), expected,
                    action, index, intent_sha, issued_sha)
            except Exception:
                receipt = None
            revalidate()
            observation = _inspection(call_action('inspect-action', action_index=index), action, index)
            if observation['state'] != 'TARGET':
                _fail('action_outcome_unknown')
            applied_event['recovered_exact_target'] = receipt is None or receipt['recovered_exact_target']
        else:
            durable = _action_receipt(call_read('action', index), expected,
                action, index, intent_sha, issued_sha)
            if durable != receipt:
                _fail('action_receipt_disagrees')
            revalidate()
            status = _broker_status(call_action('broker-status'), expected)
            if (not status['running'] or status['pid'] != broker_identity['pid']
                or status['started_at_ns'] != broker_identity['started_at_ns']):
                _fail('broker_identity_changed')
            observation = _inspection(call_action('inspect-action', action_index=index), action, index)
            if observation['state'] != 'TARGET':
                _fail('action_target_not_observed')
        append(applied_event)
        applied.append(_public_action_receipt(action, target))

    # Close the exact broker once after the canonical plan completes. If a
    # close was issued by an earlier invocation, only same-Pod receipt/status
    # readback is permitted.
    close_intent = [event for event in history if isinstance(event, dict)
        and event.get('kind') == 'nats_contract_broker_close_intent'
        and event.get('contract_attempt') == attempt]
    close_issued = [event for event in history if isinstance(event, dict)
        and event.get('kind') == 'nats_contract_broker_close_issued'
        and event.get('contract_attempt') == attempt]
    if len(close_intent) > 1 or len(close_issued) > 1:
        _fail('broker_close_journal_duplicate')
    if close_issued:
        if not close_intent:
            _fail('broker_close_history_incomplete')
        close_base = {'contract_attempt': attempt, 'job_uid': job_uid, 'pod_uid': pod_uid,
            'runner_image': identity['runner_image'], 'server_pid': broker_identity['pid'],
            'server_started_at_ns': broker_identity['started_at_ns']}
        _validate_broker_host_event(close_intent[0], 'nats_contract_broker_close_intent',
            attempt, job_uid, pod_uid, identity['runner_image'], plan_digest, close_base)
        _validate_broker_host_event(close_issued[0], 'nats_contract_broker_close_issued',
            attempt, job_uid, pod_uid, identity['runner_image'], plan_digest, close_base)
        stopped = _broker_receipt(call_read('broker-stop'), expected, 'stop')
        broker = stopped['broker']
        if (broker['pid'] != broker_identity['pid']
            or broker['started_at_ns'] != broker_identity['started_at_ns']):
            _fail('broker_stop_identity_changed')
        status = _broker_status(call_action('broker-status'), expected)
        if status['running']:
            _fail('broker_stop_unknown')
    else:
        if not close_intent:
            revalidate()
            close_intent_event = {'kind': 'nats_contract_broker_close_intent', 'contract_attempt': attempt,
                'job_uid': job_uid, 'pod_uid': pod_uid, 'runner_image': identity['runner_image'],
                'plan_sha256': plan_digest, 'server_pid': broker_identity['pid'],
                'server_started_at_ns': broker_identity['started_at_ns']}
            append(close_intent_event)
        else:
            close_intent_event = close_intent[0]
        revalidate()
        close_issued_event = {'kind': 'nats_contract_broker_close_issued', 'contract_attempt': attempt,
            'job_uid': job_uid, 'pod_uid': pod_uid, 'runner_image': identity['runner_image'],
            'plan_sha256': plan_digest, 'server_pid': broker_identity['pid'],
            'server_started_at_ns': broker_identity['started_at_ns']}
        _validate_broker_host_event(close_intent_event, 'nats_contract_broker_close_intent',
            attempt, job_uid, pod_uid, identity['runner_image'], plan_digest,
            {'contract_attempt': attempt, 'job_uid': job_uid, 'pod_uid': pod_uid,
                'runner_image': identity['runner_image'], 'plan_sha256': plan_digest,
                'server_pid': broker_identity['pid'], 'server_started_at_ns': broker_identity['started_at_ns']})
        append(close_issued_event)
        try:
            revalidate()
            _parse_line(api.exec_pod_contract(identity['namespace'], pod_name, pod_uid,
                row, storage, 'broker-stop'))
        except Exception:
            pass
        stopped = _broker_receipt(call_read('broker-stop'), expected, 'stop')
        broker = stopped['broker']
        if (broker['pid'] != broker_identity['pid']
            or broker['started_at_ns'] != broker_identity['started_at_ns']):
            _fail('broker_stop_identity_changed')
        status = _broker_status(call_action('broker-status'), expected)
        if status['running']:
            _fail('broker_stop_unknown')
    closed_event = {**broker_event('nats_contract_broker_closed', broker,
        finished_at=_time_from_ns(broker['finished_at_ns'])), 'started_at': _time_from_ns(broker['started_at_ns'])}
    append(closed_event)
    return applied


def _time_from_ns(value: int) -> str:
    if type(value) is not int or value <= 0:
        _fail('broker_time_invalid')
    try:
        return datetime.fromtimestamp(value / 1_000_000_000, timezone.utc).isoformat()
    except (OverflowError, OSError, ValueError):
        _fail('broker_time_invalid')


def _validate_opened(event: dict, attempt: str, job_uid: str, pod_uid: str, image: str) -> None:
    if (set(event) != {'kind', 'migration_attempt', 'container_id', 'server_image', 'job_uid',
        'pod_uid', 'server_pid', 'server_started_at_ns', 'started_at', 'readiness'}
        or event.get('kind') != 'nats_contract_broker_opened'
        or event.get('migration_attempt') != attempt
        or event.get('container_id') != attempt or event.get('job_uid') != job_uid
        or event.get('pod_uid') != pod_uid or event.get('server_image') != image
        or type(event.get('server_pid')) is not int or event['server_pid'] <= 0
        or type(event.get('server_started_at_ns')) is not int or event['server_started_at_ns'] <= 0
        or event.get('readiness') != 'healthz-200' or not isinstance(event.get('started_at'), str)):
        _fail('prior_broker_identity_changed')


def _validate_broker_host_event(event: object, kind: str, attempt: str, job_uid: str,
        pod_uid: str, image: str, plan_sha256: str, extra: dict | None = None) -> None:
    wanted = {'kind': kind, 'contract_attempt': attempt, 'job_uid': job_uid,
        'pod_uid': pod_uid, 'runner_image': image, 'plan_sha256': plan_sha256}
    if extra:
        wanted.update(extra)
    if not isinstance(event, dict) or set(event) != set(wanted) or event != wanted:
        _fail('broker_host_event_changed')


def _validate_closed(event: dict, attempt: str, job_uid: str, pod_uid: str, image: str) -> None:
    if (set(event) != {'kind', 'migration_attempt', 'container_id', 'server_image', 'job_uid',
        'pod_uid', 'server_pid', 'server_started_at_ns', 'started_at', 'finished_at'}
        or event.get('kind') != 'nats_contract_broker_closed'
        or event.get('migration_attempt') != attempt or event.get('container_id') != attempt
        or event.get('job_uid') != job_uid or event.get('pod_uid') != pod_uid
        or event.get('server_image') != image or type(event.get('server_pid')) is not int
        or event['server_pid'] <= 0 or type(event.get('server_started_at_ns')) is not int
        or event['server_started_at_ns'] <= 0 or not isinstance(event.get('started_at'), str)
        or not isinstance(event.get('finished_at'), str)):
        _fail('prior_broker_close_identity_changed')


def _closed_receipts(api, request: dict, storage: dict, identity: dict,
        plan: dict, history: list, expected: dict, revalidate) -> list[dict]:
    """A completed broker session can only be reconciled, never reopened."""
    attempt = storage['contract_attempt']
    try:
        def read(kind: str, index: int | None = None) -> dict:
            revalidate()
            return _parse_line(api.read_contract_receipt(identity['namespace'],
                identity['pod_name'], identity['pod_uid'], request, storage, attempt,
                kind, action_index=index))
        input_receipt = read('input')
        _expect_input_receipt(input_receipt, expected)
        start_receipt = _broker_receipt(read('broker-start'), expected, 'start')
        stop_receipt = _broker_receipt(read('broker-stop'), expected, 'stop')
    except Exception:
        _fail('closed_session_receipt_unknown')
    started, stopped = start_receipt['broker'], stop_receipt['broker']
    closed_event = next(event for event in history if isinstance(event, dict)
        and event.get('kind') == 'nats_contract_broker_closed'
        and event.get('migration_attempt') == attempt)
    if (stopped['pid'] != started['pid'] or stopped['started_at_ns'] != started['started_at_ns']
        or closed_event.get('server_pid') != started['pid']
        or closed_event.get('server_started_at_ns') != started['started_at_ns']
        or closed_event.get('started_at') != _time_from_ns(started['started_at_ns'])
        or closed_event.get('finished_at') != _time_from_ns(stopped['finished_at_ns'])):
        _fail('closed_session_broker_identity')
    applied = []
    for index, action in enumerate(plan['actions']):
        target = action.get('normalized_config_sha256', nats_contract_plan.digest(action['after']))
        intents = [event for event in history if isinstance(event, dict)
            and event.get('kind') == 'nats_contract_intent' and event.get('object') == action['object']]
        issued = [event for event in history if isinstance(event, dict)
            and event.get('kind') == 'nats_contract_mutation_issued' and event.get('object') == action['object']]
        done = [event for event in history if isinstance(event, dict)
            and event.get('kind') == 'nats_contract_applied' and event.get('object') == action['object']]
        if (len(done) != 1 or done[0].get('migration_attempt') != attempt
            or done[0].get('config_sha256') != target):
            _fail('closed_session_action_incomplete')
        if issued:
            if len(intents) != 1 or len(issued) != 1:
                _fail('closed_session_action_history')
            base_intent = {'kind': 'nats_contract_intent', 'object': action['object'], 'api': action['api'],
                'before_sha256': nats_contract_plan.digest(action['before']), 'target_sha256': target}
            base_issued = {'kind': 'nats_contract_mutation_issued', 'object': action['object'],
                'api': action['api'], 'target_sha256': target}
            _validate_history_event(intents[0], base_intent, kind='nats_contract_intent', attempt=attempt)
            _validate_history_event(issued[0], base_issued, kind='nats_contract_mutation_issued', attempt=attempt)
            base_intent['migration_attempt'] = attempt
            base_issued['migration_attempt'] = attempt
            _action_receipt(read('action', index),
                expected, action, index, hashlib.sha256(nats_contract_runner._canonical(base_intent)).hexdigest(),
                hashlib.sha256(nats_contract_runner._canonical(base_issued)).hexdigest())
        elif not done[0].get('recovered_exact_target'):
            _fail('closed_session_action_unowned')
        applied.append(_public_action_receipt(action, target))
    return applied
