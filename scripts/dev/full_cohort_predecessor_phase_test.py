import copy
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
from types import SimpleNamespace

import full_cohort_predecessor_phase as phase


def completed():
    operation = 'media-c1-' + 'a' * 32
    effects = {}
    for (kind, name), keys in phase.MEMBERS.items():
        effects[kind.lower() + '-' + name] = dict(status='done', nonce='b' * 32,
            operation_id=operation, kind=kind, name=name, keys=sorted(keys), uid='uid-' + name,
            resourceVersion='19')
    public = {name: dict(uid=name, resourceVersion='12', der_sha256='c' * 64) for name in phase.PUBLIC}
    receipt = dict(operation_id=operation, issuer_source_sha='d' * 40, candidate_source_sha='e' * 40,
                   namespace_uid='namespace', objects=copy.deepcopy(effects), public_sources=public,
                   scope='p3-create-only')
    row = dict(operation_id=operation, commit='e' * 40, realtime_p3_objects=effects,
                realtime_p3_handoff=dict(status='done', nonce='f' * 32, operation_id=operation,
                    issuer_suffix='a' * 32, pid=123, start_ticks='12345', socket_dev=1, socket_ino=2, namespace_uid='namespace',
                    candidate_source_sha='e' * 40, issuer_source_sha='d' * 40, receipt=receipt))
    objects = {key:dict(status='done', nonce='1'*32, name=name, uid='uid-'+key,
                        resourceVersion='22', spec_sha256='2'*64)
               for key, name in {'configmap':'voice-p3-principal-publisher-config',
                   'deployment':'voice-p3-principal-jwks', 'service':'voice-p3-principal-jwks'}.items()}
    objects['deployment']['generation'] = 1
    row['realtime_jwks_publisher'] = dict(status='done', nonce='3'*32,
        operation_id=operation, source_sha=row['commit'],
        node=dict(name='node', uid='node-uid', resourceVersion='4', hostname='node'),
        image=copy.deepcopy(phase.PUBLISHER_IMAGE),
        p3_handoff_sha256=phase.digest(row['realtime_p3_handoff']), objects=objects,
        runtime=dict(replicaset_uid='rs', replicaset_resourceVersion='7', pod_name='pod', pod_uid='pod-uid',
                     pod_resourceVersion='8', node_name='node', container_id='container', image_id=phase.PUBLISHER_IMAGE['platform'],
                     restart_count=0, deployment_resourceVersion='22'),
        public=dict(uid=effects['configmap-voice-p3-principal-public']['uid'], resourceVersion='19', sha256='5'*64),
        tls=dict(uid=effects['secret-voice-p3-principal-jwks-tls']['uid'], resourceVersion='19', certificate_sha256='6'*64))
    return row


class P3ReceiptTests(unittest.TestCase):
    def test_valid_receipt_is_copied(self):
        row = completed()
        result = phase.validate_p3(row, 'namespace')
        result['objects'].clear()
        self.assertEqual(len(row['realtime_p3_objects']), 7)

    def test_incomplete_or_changed_receipt_fails(self):
        for mutation in ('pending', 'effect', 'namespace', 'source', 'public', 'missing', 'suffix', 'extra', 'pid', 'duplicate_uid'):
            with self.subTest(mutation=mutation):
                row = completed()
                receipt = row['realtime_p3_handoff']['receipt']
                if mutation == 'pending': row['realtime_p3_handoff']['status'] = 'pending'
                if mutation == 'effect': row['realtime_p3_objects'].pop(next(iter(row['realtime_p3_objects'])))
                if mutation == 'namespace': receipt['namespace_uid'] = 'other'
                if mutation == 'source': receipt['candidate_source_sha'] = 'f' * 40
                if mutation == 'public': receipt['public_sources'].clear()
                if mutation == 'missing': del row['realtime_p3_handoff']
                if mutation == 'suffix': row['realtime_p3_handoff']['issuer_suffix'] = '0' * 32
                if mutation == 'extra': row['realtime_p3_handoff']['extra'] = 'unexpected'
                if mutation == 'pid': row['realtime_p3_handoff']['pid'] = True
                if mutation == 'duplicate_uid':
                    values = list(row['realtime_p3_objects'].values())
                    values[1]['uid'] = values[0]['uid']
                    receipt['objects'] = copy.deepcopy(row['realtime_p3_objects'])
                with self.assertRaises(phase.PredecessorError): phase.validate_p3(row, 'namespace')

    def test_publisher_binds_actual_handoff_and_objects(self):
        row = completed()
        phase.validate_publisher(row, 'namespace')
        row['realtime_jwks_publisher']['runtime']['image_id'] = 'sha256:' + 'f'*64
        with self.assertRaises(phase.PredecessorError): phase.validate_publisher(row, 'namespace')
        row['realtime_jwks_publisher']['runtime']['image_id'] = phase.PUBLISHER_IMAGE['platform']
        row['realtime_jwks_publisher']['public']['uid'] = 'different'
        with self.assertRaises(phase.PredecessorError): phase.validate_publisher(row, 'namespace')


class PreBeginTests(unittest.TestCase):
    def test_history_transport_uses_fixed_resource_endpoint_and_bounds(self):
        class Response:
            def __init__(self, raw): self.raw = raw
            def __enter__(self): return self
            def __exit__(self, *args): return False
            def read(self, limit): return self.raw[:limit]
        class Opener:
            def __init__(self, raw): self.raw = raw
            def open(self, url, timeout):
                self_url.append((url, timeout))
                return Response(self.raw)
        self_url = []
        with patch.object(phase.urllib.request, 'build_opener', return_value=Opener(b'{"events":[]}')):
            self.assertEqual(phase._read_history(SimpleNamespace(endpoint=phase.TT_ENDPOINT)), {'events':[]})
        self.assertEqual(self_url, [(phase.TT_ENDPOINT + '/api/projects/voice/resources/DEV', 10)])
        large=json.dumps({'events':[], 'padding':'x'*(3<<20)}).encode()
        with patch.object(phase.urllib.request, 'build_opener', return_value=Opener(large)):
            self.assertEqual(phase._read_history(SimpleNamespace(endpoint=phase.TT_ENDPOINT))['padding'], 'x'*(3<<20))
        with patch.object(phase.urllib.request, 'build_opener', return_value=Opener(b'x' * ((8 << 20) + 1))):
            with self.assertRaises(phase.PredecessorError):
                phase._read_history(SimpleNamespace(endpoint=phase.TT_ENDPOINT))

    def test_historical_deployment_matches_c1_not_c2(self):
        old = dict(actor='scheduled_devops', claim='old-claim', window='old-window', generation=3,
                   commit='a'*40, images={'app':'image'}, actual={'app':'image'})
        event = dict(id='r42', kind='deployment', actor=old['actor'],
                     before=dict(owner=old['actor'], claim=old['claim'], window=old['window'], generation=3, operation=12),
                     after=dict(claim=old['claim'], window=old['window'], generation=3,
                         observed=dict(state='known', actor=old['actor'], claim=old['claim'],
                         window=old['window'], commit=old['commit'], images=old['actual'], operation=12, deployment='rollout'),
                         candidate=dict(state='known', commit=old['commit'], images=old['images'])))
        card = dict(code='DEV', environment='dev', events=[event])
        self.assertEqual(phase.deployment_event(card, old)['id'], 'r42')
        card['events'].append(copy.deepcopy(event))
        with self.assertRaises(phase.PredecessorError): phase.deployment_event(card, old)
        card['events'].pop()
        event['after']['observed']['claim'] = 'new-claim'
        with self.assertRaises(phase.PredecessorError): phase.deployment_event(card, old)

    def test_real_fence_and_ledger_recheck(self):
        import frontend_runtime_executor
        import media_operation_guard
        import resource_fencing
        ctx = dict(project='voice', resource='DEV', actor='scheduled_devops',
            claim='11111111-1111-4111-8111-111111111111', window='22222222-2222-4222-8222-222222222222',
            generation=2, revision=3, operation_id='media-c2-' + 'a' * 32, commit='b' * 40,
            images={'voice-auth/auth':'registry/auth@sha256:' + 'c' * 64})
        client = frontend_runtime_executor.KubectlTransport.__new__(frontend_runtime_executor.KubectlTransport)
        ledger = resource_fencing.KubeLedger(client)
        check = resource_fencing.ResourceCheck(phase.TT_ENDPOINT, ctx)
        fence = resource_fencing.Fence(ctx, check, ledger, on_uncertain=check.mark_uncertain)
        old = dict(ctx, status='complete', generation=1, actual=ctx['images']) | completed()
        old['claim'] = '33333333-3333-4333-8333-333333333333'
        old['window'] = '44444444-4444-4444-8444-444444444444'
        labels = dict(media_operation_guard.LABELS)
        lock = {'metadata':dict(name=media_operation_guard.LOCK, namespace='voice-dev', uid='lock',
                                  resourceVersion='4', labels=labels),
                'data':dict(resource_operation=ctx['operation_id'],
                            resource_context=json.dumps(ctx, sort_keys=True, separators=(',', ':')), operation='run')}
        namespace = {'metadata':dict(name='voice-dev', uid='namespace', labels=labels)}
        document = {'metadata':dict(name=resource_fencing.LEDGER, namespace='voice-dev', uid='ledger',
                                    resourceVersion='5'), 'data':{'executor':json.dumps(old)}}
        def call(*args, **kwargs):
            if args[0] == 'patch':
                body = kwargs['body']
                if body[0]['value'] != document['metadata']['uid'] or body[1]['value'] != document['metadata']['resourceVersion']:
                    raise RuntimeError('CAS failed')
                document['data'] = body[2]['value']
                document['metadata']['resourceVersion'] = '6'
                return document
            if args[1] == 'namespace': return namespace
            if args[2] == media_operation_guard.LOCK: return lock
            return document
        client.call = call
        card = dict(code='DEV', environment='dev', owner=ctx['actor'], claim=ctx['claim'],
                    generation=2, window=ctx['window'], revision=3, state='held', phase='deploy',
                    operation=9, observed=dict(state='known', claim=ctx['claim'], window=ctx['window'],
                                               operation=8, commit=old['commit'], images=old['actual']),
                    candidate=dict(state='known', commit=ctx['commit'], images=ctx['images']))
        with patch.object(resource_fencing.ResourceCheck, '__call__', return_value=card):
            phase.check_pre_begin(fence, client, copy.deepcopy(lock), 'namespace', old, ('ledger', '5'))
            with patch.object(phase.urllib.request, 'getproxies', return_value={'http':'http://untrusted'}), \
                 patch.object(phase.urllib.request, 'proxy_bypass', return_value=False):
                with self.assertRaises(phase.PredecessorError):
                    phase.check_pre_begin(fence, client, lock, 'namespace', old, ('ledger', '5'))
            check.endpoint = 'http://example.test'
            with self.assertRaises(phase.PredecessorError):
                phase.check_pre_begin(fence, client, lock, 'namespace', old, ('ledger', '5'))
            check.endpoint = phase.TT_ENDPOINT
            expected = copy.deepcopy(lock)
            lock['metadata']['resourceVersion'] = 'changed'
            with self.assertRaises(phase.PredecessorError):
                phase.check_pre_begin(fence, client, expected, 'namespace', old, ('ledger', '5'))
            lock['metadata']['resourceVersion'] = '4'
            card['generation'] = 3
            with self.assertRaises(RuntimeError):
                phase.check_pre_begin(fence, client, lock, 'namespace', old, ('ledger', '5'))
            card['generation'] = 2
            document['metadata']['resourceVersion'] = '6'
            with self.assertRaises(phase.PredecessorError):
                phase.check_pre_begin(fence, client, lock, 'namespace', old, ('ledger', '5'))
            document['metadata']['resourceVersion'] = '5'
            import media_operation_journal
            with tempfile.TemporaryDirectory() as parent:
                import full_cohort_config_custody as custody
                import full_cohort_config_custody_test as config_fixture
                import media_backup_upload_phase
                operation, entries, effects = config_fixture.facts()
                self.assertEqual(operation, old['operation_id'])
                context_keys = {'project','resource','actor','claim','generation','window','revision','operation_id','commit','images'}
                config = {'schema':'voice-full-cohort-config-snapshot-v1',
                          'resource_context':{k:old[k] for k in context_keys}, 'namespace_uid':'namespace',
                          'cut_invocation':'b'*32, 'services':['chat'], 'effects':entries}
                marker = {'schema':'voice-full-cohort-config-marker-v1','status':'done','operation_id':operation,
                          'source_sha':old['commit'],'namespace_uid':'namespace','cut_invocation':'b'*32,
                          'snapshot_sha256':custody.digest(config), 'effects_sha256':custody.digest(effects)}
                old[custody.MARKER] = marker; old['backend_config_effects'] = effects
                document['data']['executor'] = json.dumps(old)
                root = Path(parent)/operation; root.mkdir(mode=0o700)
                filename = root/custody.FILENAME
                filename.write_text(json.dumps(config,sort_keys=True,separators=(',',':'))); os.chmod(filename,0o400)
                import space_native_tool_identity_custody as tool_custody
                inventory={'source_sha':old['commit'], 'build_source_sha256':'1'*64, 'sql_inventory_sha256':'2'*64,
                           'dockerfile_sha256':'3'*64, 'source_manifest_sha256':'4'*64}
                contract={'schema':'voice-space-native-local-closure-v1','packages':{},'files':{},'raw_sha256':'9'*64}
                receipt={'operation_id':operation,'namespace_uid':'namespace','source_sha':old['commit'],
                         **{k:inventory[k] for k in ('build_source_sha256','sql_inventory_sha256','dockerfile_sha256','source_manifest_sha256')},
                         'runtime_receipt_verified':False,'compiled_gate_probe':'native51-source-gates-reached-v1',
                         'closure_sha256':tool_custody.digest(contract)}
                old['space_native_tool']={'status':'done','receipt_sha256':tool_custody.digest(receipt)}
                tool_snapshot={'schema':'voice-space-native-tool-completion-v1','resource_context':{k:old[k] for k in context_keys},
                               'namespace_uid':'namespace','receipt':receipt,'inventory':inventory,'contract':contract,
                               'tool_marker':copy.deepcopy(old['space_native_tool'])}
                old[tool_custody.MARKER]={'schema':'voice-space-native-tool-completion-marker-v1','status':'done',
                    'operation_id':operation,'source_sha':old['commit'],'namespace_uid':'namespace',
                    'snapshot_sha256':tool_custody.digest(tool_snapshot)}
                toolfile=root/tool_custody.FILENAME
                toolfile.write_text(json.dumps(tool_snapshot,sort_keys=True,separators=(',',':'))); os.chmod(toolfile,0o400)
                document['data']['executor'] = json.dumps(old)
                root_patch = patch.object(media_backup_upload_phase, 'ROOT', Path(parent)); root_patch.start()
                self.addCleanup(root_patch.stop)
                intent = {'resource_context':ctx, 'namespace_uid':'namespace'}
                history = dict(code='DEV', environment='dev', events=[dict(id='r11', kind='deployment',
                    actor=old['actor'], before=dict(owner=old['actor'], claim=old['claim'],
                        window=old['window'], generation=old['generation'], operation=8),
                    after=dict(claim=old['claim'], window=old['window'], generation=old['generation'],
                        candidate=dict(state='known', commit=old['commit'], images=old['images']),
                        observed=dict(state='known', actor=old['actor'], claim=old['claim'], window=old['window'],
                            commit=old['commit'], images=old['actual'], operation=8, deployment='rollout')))])
                with media_operation_journal.Journal(parent+'/drift', intent) as journal:
                    client.journal = journal
                    changed = copy.deepcopy(history); changed['events'][0]['id'] = 'r12'
                    with patch.object(phase, '_read_history', side_effect=[history, changed]):
                        with self.assertRaises(phase.PredecessorError):
                            phase.capture(fence, client, lock, 'namespace', journal, old, ('ledger', '5'))
                    self.assertIn('full-cohort-predecessor', journal.state['steps'])
                with media_operation_journal.Journal(parent+'/journal', intent) as journal:
                    client.journal = journal
                    with patch.object(phase, '_read_history', return_value=history):
                        fence.begin_with_predecessor(client,lock,'namespace',journal)
                    snapshot = phase.load(fence, client, lock, 'namespace', journal)
                    self.assertEqual(snapshot['previous_operation_id'], old['operation_id'])
                    detached = custody.load_predecessor(fence,client,lock,'namespace',journal)
                    self.assertEqual(detached['source_sha'], old['commit'])
                    self.assertNotEqual(detached['source_sha'], ctx['commit'])
                    detached['effects'].clear()
                    self.assertTrue(custody.load_predecessor(fence,client,lock,'namespace',journal)['effects'])
                    journal.state['steps']['full-cohort-predecessor']['snapshot']['previous_commit'] = 'f'*40
                    with self.assertRaises(phase.PredecessorError): phase.load(fence, client, lock, 'namespace', journal)


if __name__ == '__main__': unittest.main()
