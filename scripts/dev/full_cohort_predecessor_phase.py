"""Validate the source-produced P3 portion of a completed predecessor row.

This is not admission: callers must bind the row to the old ledger revision,
the actual TT deployment and the new operation's claim and lock before use.
"""
import copy
import hashlib
import json
import re
import urllib.request


MEMBERS = {
    ('Secret', 'voice-realtime-principal-signing'): {'current.pem', 'next.pem'},
    ('Secret', 'voice-realtime-p3-tls'): {'tls.crt', 'tls.key'},
    ('Secret', 'voice-chat-realtime-p3-tls'): {'tls.crt', 'tls.key'},
    ('Secret', 'voice-p3-principal-jwks-tls'): {'tls.crt', 'tls.key'},
    ('Secret', 'voice-p3-principal-replay'): {'password'},
    ('Secret', 'voice-p3-principal-trust'): {'chat-server-ca.crt', 'space-client-ca.crt',
        'space-jwks-ca.crt', 'realtime-client-ca.crt', 'realtime-jwks-ca.crt'},
    ('ConfigMap', 'voice-p3-principal-public'): {'realtime.json'},
}
PUBLIC = {'voice-principal-ca', 'voice-media-ca', 'voice-space-media-server-tls'}
TT_ENDPOINT = 'http://192.168.0.109:30990'
PUBLISHER_IMAGE = {
    'index':'sha256:0e52bd054126cdc040b1276c0af347aeb02a41b9cfe305d6094767ac30e97949',
    'platform':'sha256:84ed8c0dd296c188bfc24ed506f46589e9285fb0b11cf3a04d91d524d3067e11',
    'config':'sha256:88f52f3d58a505b7e1ad96ca6969478517bcccb58418e75a54223271a64da7df',
}


class PredecessorError(RuntimeError):
    pass


def digest(value):
    raw = json.dumps(value, sort_keys=True, separators=(',', ':'), allow_nan=False).encode()
    return hashlib.sha256(raw).hexdigest()


def _text(value):
    return isinstance(value, str) and bool(value) and len(value) <= 256


def _no_tt_proxy():
    if urllib.request.getproxies().get('http') and not urllib.request.proxy_bypass('192.168.0.109'):
        raise PredecessorError('TT authority proxy is not permitted')


def deployment_event(card, previous):
    """Select a historical C1 deployment; unknown or ambiguous history holds."""
    try:
        if not isinstance(card, dict) or card.get('code') != 'DEV' or card.get('environment') != 'dev':
            raise ValueError()
        events = card['events']
        if not isinstance(events, list) or len(events) > 20000: raise ValueError()
        matches = []
        for event in events:
            if not isinstance(event, dict) or event.get('kind') != 'deployment': continue
            before, after = event.get('before'), event.get('after')
            if not isinstance(before, dict) or not isinstance(after, dict): continue
            observed, candidate = after.get('observed'), after.get('candidate')
            if not isinstance(observed, dict) or not isinstance(candidate, dict): continue
            if (event.get('actor') != previous['actor'] or before.get('owner') != previous['actor'] or
                    before.get('claim') != previous['claim'] or before.get('window') != previous['window'] or
                    before.get('generation') != previous['generation'] or
                    after.get('claim') != previous['claim'] or after.get('window') != previous['window'] or
                    after.get('generation') != previous['generation'] or
                    observed.get('state') != 'known' or observed.get('actor') != previous['actor'] or
                    observed.get('claim') != previous['claim'] or observed.get('window') != previous['window'] or
                    observed.get('commit') != previous['commit'] or observed.get('images') != previous['actual'] or
                    candidate.get('state') != 'known' or candidate.get('commit') != previous['commit'] or
                    candidate.get('images') != previous['images']): continue
            event_id = event.get('id')
            if not _text(event_id): raise ValueError()
            deployment = observed.get('deployment')
            if not _text(deployment): raise ValueError()
            if (type(observed.get('operation')) is not int or observed['operation'] < 1 or
                    observed['operation'] != before.get('operation')): raise ValueError()
            matches.append({'id':event_id, 'kind':'deployment', 'actor':previous['actor'],
                            'claim':previous['claim'], 'window':previous['window'],
                            'generation':previous['generation'], 'commit':previous['commit'],
                            'operation':observed['operation'], 'deployment':deployment,
                            'images':copy.deepcopy(previous['actual'])})
        if len(matches) != 1: raise ValueError()
        return matches[0]
    except (KeyError, TypeError, ValueError):
        raise PredecessorError('C1 deployment history unavailable or ambiguous') from None


def _read_history(check):
    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, *args): return None
    try:
        with urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect()).open(
                check.endpoint + '/api/projects/voice/resources/DEV', timeout=10) as response:
            raw = response.read((8 << 20) + 1)
        if len(raw) > 8 << 20: raise ValueError()
        return json.loads(raw)
    except (OSError, ValueError):
        raise PredecessorError('C1 deployment history unavailable') from None


def check_pre_begin(fence, client, expected_lock, namespace_uid, previous, revision):
    """Recheck C2 authority while the ledger still contains the complete C1 row."""
    import media_operation_guard as guard
    import resource_fencing
    import frontend_runtime_executor

    if (type(fence) is not resource_fencing.Fence or type(fence.ledger) is not resource_fencing.KubeLedger or
            type(fence.check) is not resource_fencing.ResourceCheck or
            type(client) is not frontend_runtime_executor.KubectlTransport or fence.ledger.client is not client or
            fence.check.context != fence.context or fence.check.endpoint != TT_ENDPOINT or
            not isinstance(expected_lock, dict)):
        raise PredecessorError('pre-begin executor invalid')
    _no_tt_proxy()
    context = fence.context
    resource_fencing.validate_context(context)
    if not isinstance(previous, dict): raise PredecessorError('completed predecessor required')
    keys = {'project', 'resource', 'actor', 'claim', 'generation', 'window', 'revision',
            'operation_id', 'commit', 'images'}
    try:
        resource_fencing.validate_context({key: previous[key] for key in keys})
    except (KeyError, TypeError, ValueError):
        raise PredecessorError('completed predecessor context invalid') from None
    if (previous.get('status') != 'complete' or previous.get('actor') != 'scheduled_devops' or
            not re.fullmatch(r'media-c1-[0-9a-f]{32}', previous['operation_id']) or
            not re.fullmatch(r'media-c2-[0-9a-f]{32}', context['operation_id']) or
            previous['generation'] >= context['generation'] or previous.get('actual') != previous['images']):
        raise PredecessorError('completed predecessor identity invalid')
    meta = expected_lock.get('metadata', {})
    data = expected_lock.get('data', {})
    if (context['actor'] != 'scheduled_devops' or not _text(namespace_uid) or
            meta.get('name') != guard.LOCK or meta.get('namespace') != 'voice-dev' or
            any(meta.get('labels', {}).get(k) != v for k, v in guard.LABELS.items()) or
            not _text(meta.get('uid')) or not _text(meta.get('resourceVersion')) or
            data.get('resource_operation') != context['operation_id'] or
            data.get('resource_context') != json.dumps(context, sort_keys=True, separators=(',', ':')) or
            not data.get('operation') or not isinstance(revision, tuple) or len(revision) != 2 or
            any(not _text(value) for value in revision)):
        raise PredecessorError('pre-begin authority invalid')
    fence.authority()
    first, first_revision = fence.ledger.read()
    if first != previous or first_revision != revision: raise PredecessorError('predecessor changed')
    ns = client.call('get', 'namespace', 'voice-dev', '-o', 'json').get('metadata', {})
    lock = client.call('get', 'configmap', guard.LOCK, '-o', 'json')
    lock_meta = lock.get('metadata', {})
    if (ns.get('name') != 'voice-dev' or ns.get('uid') != namespace_uid or ns.get('deletionTimestamp') or
            any(ns.get('labels', {}).get(k) != v for k, v in guard.LABELS.items()) or
            lock_meta.get('name') != guard.LOCK or lock_meta.get('namespace') != 'voice-dev' or
            lock_meta.get('uid') != meta['uid'] or lock_meta.get('resourceVersion') != meta['resourceVersion'] or
            lock_meta.get('deletionTimestamp') or
            any(lock_meta.get('labels', {}).get(k) != v for k, v in guard.LABELS.items()) or
            lock.get('data') != data):
        raise PredecessorError('namespace or updater lock changed')
    fence.authority()
    if fence.ledger.read() != (previous, revision): raise PredecessorError('predecessor changed')


def validate_p3(row, namespace_uid):
    """Validate fixed receipt shape and ledger effects; does not establish authority."""
    try:
        operation = row['operation_id']
        if not re.fullmatch(r'media-c1-[0-9a-f]{32}', operation): raise ValueError()
        handoff = row['realtime_p3_handoff']
        receipt = handoff['receipt']
        if (set(handoff) != {'status', 'nonce', 'operation_id', 'issuer_suffix', 'issuer_source_sha',
                             'candidate_source_sha', 'namespace_uid', 'pid', 'start_ticks',
                             'socket_dev', 'socket_ino', 'receipt'} or
                not _text(namespace_uid) or not re.fullmatch(r'[0-9a-f]{40}', row['commit']) or
                not re.fullmatch(r'[0-9a-f]{32}', handoff['nonce']) or
                handoff['issuer_suffix'] != operation.removeprefix('media-c1-') or
                type(handoff['pid']) is not int or handoff['pid'] < 1 or
                not re.fullmatch(r'[0-9]+', handoff['start_ticks']) or
                type(handoff['socket_dev']) is not int or handoff['socket_dev'] < 0 or
                type(handoff['socket_ino']) is not int or handoff['socket_ino'] < 1 or
                handoff['status'] != 'done' or handoff['operation_id'] != operation or
                handoff['namespace_uid'] != namespace_uid or
                handoff['candidate_source_sha'] != row['commit'] or
                not re.fullmatch(r'[0-9a-f]{40}', handoff['issuer_source_sha']) or
                set(receipt) != {'operation_id', 'issuer_source_sha', 'candidate_source_sha',
                                 'namespace_uid', 'objects', 'public_sources', 'scope'} or
                receipt['operation_id'] != operation or receipt['namespace_uid'] != namespace_uid or
                receipt['candidate_source_sha'] != row['commit'] or
                receipt['issuer_source_sha'] != handoff['issuer_source_sha'] or
                receipt['scope'] != 'p3-create-only' or receipt['objects'] != row['realtime_p3_objects']):
            raise ValueError()
        effects = receipt['objects']
        if set(effects) != {kind.lower() + '-' + name for kind, name in MEMBERS}: raise ValueError()
        identities = set()
        for (kind, name), keys in MEMBERS.items():
            effect = effects[kind.lower() + '-' + name]
            if (set(effect) != {'status', 'nonce', 'operation_id', 'kind', 'name', 'keys', 'uid', 'resourceVersion'} or
                    effect['status'] != 'done' or effect['operation_id'] != operation or effect['kind'] != kind or
                    effect['name'] != name or effect['keys'] != sorted(keys) or
                    not re.fullmatch(r'[0-9a-f]{32}', effect['nonce']) or
                    not _text(effect['uid']) or not _text(effect['resourceVersion'])): raise ValueError()
            if effect['uid'] in identities: raise ValueError()
            identities.add(effect['uid'])
        public = receipt['public_sources']
        if set(public) != PUBLIC: raise ValueError()
        for item in public.values():
            if (set(item) != {'uid', 'resourceVersion', 'der_sha256'} or not _text(item['uid']) or
                    not _text(item['resourceVersion']) or not re.fullmatch(r'[0-9a-f]{64}', item['der_sha256'])):
                raise ValueError()
        return copy.deepcopy(receipt)
    except (KeyError, TypeError, ValueError, AttributeError):
        raise PredecessorError('completed predecessor P3 receipt invalid') from None


def validate_publisher(row, namespace_uid):
    """Check the publisher producer's provisional record; not admission."""
    validate_p3(row, namespace_uid)
    try:
        item = row['realtime_jwks_publisher']
        if (set(item) != {'status', 'nonce', 'operation_id', 'source_sha', 'node', 'image',
                          'p3_handoff_sha256', 'public', 'tls', 'objects', 'runtime'} or
                item['status'] != 'done' or item['operation_id'] != row['operation_id'] or
                item['source_sha'] != row['commit'] or not re.fullmatch(r'[0-9a-f]{32}', item['nonce']) or
                item['p3_handoff_sha256'] != digest(row['realtime_p3_handoff'])): raise ValueError()
        node = item['node']
        if (set(node) != {'name', 'uid', 'resourceVersion', 'hostname'} or
                any(not _text(value) for value in node.values())): raise ValueError()
        image = item['image']
        if image != PUBLISHER_IMAGE: raise ValueError()
        for key, hash_key in (('public', 'sha256'), ('tls', 'certificate_sha256')):
            entry = item[key]
            if (set(entry) != {'uid', 'resourceVersion', hash_key} or not _text(entry['uid']) or
                    not _text(entry['resourceVersion']) or not re.fullmatch(r'[0-9a-f]{64}', entry[hash_key])):
                raise ValueError()
        for key, identity in (('public', 'configmap-voice-p3-principal-public'),
                              ('tls', 'secret-voice-p3-principal-jwks-tls')):
            effect = row['realtime_p3_objects'][identity]
            if any(item[key][field] != effect[field] for field in ('uid', 'resourceVersion')):
                raise ValueError()
        objects = item['objects']
        names = {'configmap':'voice-p3-principal-publisher-config',
                 'deployment':'voice-p3-principal-jwks', 'service':'voice-p3-principal-jwks'}
        if set(objects) != set(names): raise ValueError()
        uids = set()
        for key, name in names.items():
            obj = objects[key]
            fields = {'status', 'nonce', 'name', 'uid', 'resourceVersion', 'spec_sha256'}
            if key == 'deployment': fields.add('generation')
            if (set(obj) != fields or
                    obj['status'] != 'done' or obj['name'] != name or
                    not re.fullmatch(r'[0-9a-f]{32}', obj['nonce']) or not _text(obj['uid']) or
                    not _text(obj['resourceVersion']) or not re.fullmatch(r'[0-9a-f]{64}', obj['spec_sha256']) or
                    obj['uid'] in uids): raise ValueError()
            if key == 'deployment' and (type(obj['generation']) is not int or obj['generation'] < 1): raise ValueError()
            uids.add(obj['uid'])
        runtime = item['runtime']
        fields = {'replicaset_uid', 'replicaset_resourceVersion', 'pod_name', 'pod_uid', 'pod_resourceVersion',
                  'node_name', 'container_id', 'image_id', 'restart_count', 'deployment_resourceVersion'}
        if (set(runtime) != fields or any(not _text(runtime[k]) for k in fields - {'restart_count'}) or
                type(runtime['restart_count']) is not int or runtime['restart_count'] < 0 or
                runtime['node_name'] != node['name'] or
                not runtime['image_id'].endswith(PUBLISHER_IMAGE['platform']) or
                runtime['deployment_resourceVersion'] != objects['deployment']['resourceVersion']): raise ValueError()
        return copy.deepcopy(item)
    except (KeyError, TypeError, ValueError, AttributeError):
        raise PredecessorError('completed predecessor publisher receipt invalid') from None


def capture(fence, client, expected_lock, namespace_uid, journal, previous, revision):
    """Persist C1 evidence before the caller's single old-revision begin CAS.

    A saved snapshot alone is not authority.  The caller must use the returned
    descriptor in the very same CAS replacing `previous` at `revision`.
    """
    import full_cohort_config_custody as config_custody
    import space_native_tool_identity_custody as tool_custody
    import media_operation_journal
    if (type(journal) is not media_operation_journal.Journal or client.journal is not journal or
            journal.poisoned or journal.dirfd is None or journal.lockfd is None or
            journal.state.get('intent') != journal.intent or
            journal.intent.get('resource_context') != fence.context or
            journal.intent.get('namespace_uid') != namespace_uid or
            'full-cohort-predecessor' in journal.state['steps']):
        raise PredecessorError('current private journal invalid or already captured')
    check_pre_begin(fence, client, expected_lock, namespace_uid, previous, revision)
    p3 = validate_p3(previous, namespace_uid)
    publisher = validate_publisher(previous, namespace_uid)
    event = deployment_event(_read_history(fence.check), previous)
    config = config_custody.read_completed(previous, namespace_uid)
    tool_snapshot = tool_custody.read_completed(previous, namespace_uid)
    check_pre_begin(fence, client, expected_lock, namespace_uid, previous, revision)
    snapshot = {'schema':'voice-full-cohort-predecessor-snapshot-v2',
                'operation_id':fence.context['operation_id'], 'namespace_uid':namespace_uid,
                'ledger_uid':revision[0], 'ledger_resourceVersion':revision[1],
                'old_row_sha256':digest(previous), 'previous_operation_id':previous['operation_id'],
                'previous_commit':previous['commit'], 'deployment':event,
                'config_completion':{'snapshot':config, 'marker':copy.deepcopy(previous[config_custody.MARKER]),
                                     'ledger_effects':copy.deepcopy(previous['backend_config_effects'])},
                'tool_completion':{'snapshot':tool_snapshot, 'marker':copy.deepcopy(previous[tool_custody.MARKER]),
                                   'ledger_tool':copy.deepcopy(previous['space_native_tool'])},
                'p3_receipt':p3, 'p3_handoff':copy.deepcopy(previous['realtime_p3_handoff']),
                'p3_objects':copy.deepcopy(previous['realtime_p3_objects']), 'publisher':publisher}
    descriptor = {'schema':'voice-full-cohort-predecessor-v1',
                  'snapshot_sha256':digest(snapshot), 'old_row_sha256':snapshot['old_row_sha256'],
                  'ledger_uid':revision[0], 'ledger_resourceVersion':revision[1],
                  'previous_operation_id':previous['operation_id']}
    journal.state['steps']['full-cohort-predecessor'] = {'status':'captured',
        'descriptor':copy.deepcopy(descriptor), 'snapshot':snapshot}
    journal.save()
    check_pre_begin(fence, client, expected_lock, namespace_uid, previous, revision)
    if deployment_event(_read_history(fence.check), previous) != event:
        raise PredecessorError('C1 deployment history changed')
    if config_custody.read_completed(previous, namespace_uid) != config:
        raise PredecessorError('C1 config custody changed')
    if tool_custody.read_completed(previous, namespace_uid) != tool_snapshot:
        raise PredecessorError('C1 tool custody changed')
    check_pre_begin(fence, client, expected_lock, namespace_uid, previous, revision)
    return descriptor


def load(fence, client, expected_lock, namespace_uid, journal):
    """Return the private snapshot only while the matching C2 fence is active."""
    import media_operation_guard
    import media_operation_journal
    import full_cohort_config_custody as config_custody
    import space_native_tool_identity_custody as tool_custody
    import frontend_runtime_executor
    import resource_fencing
    try:
        if (type(fence) is not resource_fencing.Fence or type(fence.ledger) is not resource_fencing.KubeLedger or
                type(fence.check) is not resource_fencing.ResourceCheck or fence.check.endpoint != TT_ENDPOINT or
                fence.check.context != fence.context or type(client) is not frontend_runtime_executor.KubectlTransport or
                fence.ledger.client is not client or type(journal) is not media_operation_journal.Journal or
                client.journal is not journal or journal.poisoned or journal.dirfd is None or journal.lockfd is None or
                journal.state.get('intent') != journal.intent or journal.intent.get('resource_context') != fence.context or
                journal.intent.get('namespace_uid') != namespace_uid): raise ValueError()
        _no_tt_proxy()
        media_operation_guard.verify(fence, client, expected_lock, namespace_uid)
        row, revision = fence.ledger.read()
        entry = journal.state['steps']['full-cohort-predecessor']
        if set(entry) != {'status', 'descriptor', 'snapshot'} or entry['status'] != 'captured': raise ValueError()
        descriptor, snapshot = entry['descriptor'], entry['snapshot']
        if (not fence.owned(row) or row.get('status') != 'active' or row.get('predecessor') != descriptor or
                descriptor.get('schema') != 'voice-full-cohort-predecessor-v1' or
                descriptor.get('snapshot_sha256') != digest(snapshot) or
                descriptor.get('old_row_sha256') != snapshot['old_row_sha256'] or
                descriptor.get('ledger_uid') != snapshot['ledger_uid'] or
                descriptor.get('ledger_resourceVersion') != snapshot['ledger_resourceVersion'] or
                descriptor.get('previous_operation_id') != snapshot['previous_operation_id'] or
                snapshot['schema'] != 'voice-full-cohort-predecessor-snapshot-v2' or
                snapshot['operation_id'] != fence.context['operation_id'] or snapshot['namespace_uid'] != namespace_uid):
            raise ValueError()
        old = {'operation_id':snapshot['previous_operation_id'], 'commit':snapshot['previous_commit'],
               'realtime_p3_handoff':snapshot['p3_handoff'], 'realtime_p3_objects':snapshot['p3_objects'],
               'realtime_jwks_publisher':snapshot['publisher']}
        if validate_p3(old, namespace_uid) != snapshot['p3_receipt']: raise ValueError()
        validate_publisher(old, namespace_uid)
        config = snapshot['config_completion']
        if not isinstance(config,dict) or set(config) != {'snapshot','marker','ledger_effects'}: raise ValueError()
        previous_config = dict(config['snapshot']['resource_context'], backend_config_effects=config['ledger_effects'])
        if (previous_config.get('operation_id') != snapshot['previous_operation_id'] or
                previous_config.get('commit') != snapshot['previous_commit']): raise ValueError()
        config_custody.validate_snapshot(config['snapshot'],config['marker'],previous_config,namespace_uid)
        completion=snapshot['tool_completion']
        if not isinstance(completion,dict) or set(completion)!={'snapshot','marker','ledger_tool'}: raise ValueError()
        previous_tool=dict(completion['snapshot']['resource_context'],space_native_tool=completion['ledger_tool'])
        if (previous_tool.get('operation_id')!=snapshot['previous_operation_id'] or
            previous_tool.get('commit')!=snapshot['previous_commit']): raise ValueError()
        tool_custody.validate_snapshot(completion['snapshot'],completion['marker'],previous_tool,namespace_uid)
        media_operation_guard.verify(fence, client, expected_lock, namespace_uid)
        if fence.ledger.read() != (row, revision): raise ValueError()
        return copy.deepcopy(snapshot)
    except (KeyError, TypeError, ValueError, AttributeError):
        raise PredecessorError('active predecessor snapshot unavailable') from None
