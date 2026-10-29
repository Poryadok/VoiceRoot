import copy
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import full_cohort_config_custody as custody


def facts():
    operation = 'media-c1-' + 'a' * 32
    source = {'apiVersion':'apps/v1', 'kind':'Deployment', 'metadata': {'name':'voice-chat', 'namespace':'voice-dev', 'uid':'uid', 'generation':2},
              'spec': {'replicas':0, 'template':{'spec':{'containers':[{'name':'chat','image':'old'}]}}}}
    target = copy.deepcopy(source['spec']); target['template']['spec']['containers'][0]['env'] = [{'name':'x','value':'y'}]
    details = {'operation_id':operation, 'invocation_nonce':'b'*32, 'source':source, 'target_spec':target, 'patch':[]}
    receipt = {'uid':'uid', 'spec_sha256':custody.digest(target), 'responseResourceVersion':'3', 'resourceVersion':'4', 'generation':3}
    local = {'status':'done', 'nonce':'c'*32, 'details':details, 'details_sha256':custody.digest(details), 'receipt':receipt}
    ledger = {'status':'done', 'nonce':local['nonce'], 'details_sha256':local['details_sha256'], 'receipt_sha256':custody.digest(receipt)}
    return operation, {'backend-config-deployment-voice-chat':local}, {'backend-config-deployment-voice-chat':ledger}


class ConfigRecordsTest(unittest.TestCase):
    def test_factory_and_committed_lost_reply_remain_uncertain(self):
        import frontend_runtime_executor
        import media_backup_upload_phase
        import media_operation_journal
        import resource_fencing
        import update_images
        import writer_cut
        operation, entries, effects = facts()
        context = {'project':'voice','resource':'DEV','actor':'scheduled_devops',
            'claim':'11111111-1111-4111-8111-111111111111','window':'22222222-2222-4222-8222-222222222222',
            'generation':1,'revision':1,'operation_id':operation,'commit':'a'*40,
            'images':{'voice-chat/chat':'registry/chat@sha256:'+'b'*64}}
        card = {'code':'DEV','environment':'dev','owner':context['actor'],'claim':context['claim'],
                'window':context['window'],'generation':1,'revision':1,'state':'held','phase':'deploy',
                'candidate':{'state':'known','commit':context['commit'],'images':context['images']}}
        lock = {'metadata':{'name':update_images.LOCK,'namespace':'voice-dev','uid':'lock','resourceVersion':'1',
                            'labels':update_images.LABELS},
                'data':{'resource_context':json.dumps(context,sort_keys=True,separators=(',',':')),
                        'resource_operation':operation,'operation':'run'}}
        namespace = {'metadata':{'name':'voice-dev','uid':'ns','labels':update_images.LABELS}}
        doc = {'metadata':{'name':resource_fencing.LEDGER,'namespace':'voice-dev','uid':'ledger','resourceVersion':'1'},
               'data':{'executor':json.dumps(context | {'status':'active','backend_config_effects':effects})}}
        client = frontend_runtime_executor.KubectlTransport.__new__(frontend_runtime_executor.KubectlTransport)
        lost = []
        def call(*args, **kwargs):
            if args[:3] == ('get','configmap',resource_fencing.LEDGER): return copy.deepcopy(doc)
            if args[:3] == ('get','configmap',update_images.LOCK): return copy.deepcopy(lock)
            if args[:3] == ('get','namespace','voice-dev'): return copy.deepcopy(namespace)
            if args[:3] == ('patch','configmap',resource_fencing.LEDGER):
                ops = kwargs['body']
                if ops[1]['value'] != doc['metadata']['resourceVersion']: raise ValueError()
                doc['data'] = copy.deepcopy(ops[2]['value'])
                doc['metadata']['resourceVersion'] = str(int(doc['metadata']['resourceVersion'])+1)
                if not lost:
                    lost.append(True); raise OSError('synthetic committed reply lost')
                return copy.deepcopy(doc)
            raise ValueError()
        client.call = call
        fence = resource_fencing.Fence(context, lambda:copy.deepcopy(card), resource_fencing.KubeLedger(client))
        intent = {'resource_context':context,'namespace_uid':'ns','backend_config_deployments':
                  {'source_sha':context['commit'],'cut_invocation':'b'*32,'services':['chat']}}
        with tempfile.TemporaryDirectory() as temp, patch.object(media_backup_upload_phase,'ROOT',Path(temp)):
            root = Path(temp)/operation; root.mkdir(mode=0o700)
            with media_operation_journal.Journal(root/'cut-journal', {'resource_context':context,'namespace_uid':'ns'}) as cut_journal:
                client.journal = cut_journal
                with custody.open_config_journal(fence,client,lock,'ns',intent) as config:
                    config.state['steps'] = copy.deepcopy(entries); config.save()
                    cut = object.__new__(writer_cut.WriterCut)
                    cut.journal,cut.fence,cut.client,cut.lock,cut.namespace_uid = cut_journal,fence,client,lock,'ns'
                    cut.invocation,cut.config_journal = 'b'*32,config
                    cut.config_snapshot = {'services':['chat']}
                    with patch.object(writer_cut.WriterCut,'_guard'), patch.object(writer_cut.WriterCut,'_recheck_config_successors'):
                        with self.assertRaises(custody.CustodyError): custody.seal(fence,client,lock,'ns',config,cut)
                        observed,_ = fence.ledger.read()
                        self.assertEqual(observed['status'],'uncertain')
                        self.assertIn(custody.MARKER, observed)
                        self.assertTrue((root/custody.FILENAME).exists())
                        with self.assertRaises(custody.CustodyError): custody.seal(fence,client,lock,'ns',config,cut)

    def test_ledger_bound_exact_source_and_target(self):
        operation, local, ledger = facts()
        result = custody.checked_records(operation, 'b'*32, ['chat'], local, ledger)
        self.assertEqual(result['backend-config-deployment-voice-chat']['details'], local['backend-config-deployment-voice-chat']['details'])
        result.clear(); self.assertTrue(local)

    def test_tamper_missing_pending_or_extra_entry_holds(self):
        for kind in ('source', 'receipt', 'pending', 'missing', 'extra'):
            operation, local, ledger = facts()
            if kind == 'source': local[next(iter(local))]['details']['source']['spec']['replicas'] = 1
            if kind == 'receipt': local[next(iter(local))]['receipt']['uid'] = 'foreign'
            if kind == 'pending': ledger[next(iter(ledger))]['status'] = 'pending'
            if kind == 'missing': ledger.clear()
            if kind == 'extra': local['backend-config-deployment-voice-other'] = copy.deepcopy(next(iter(local.values())))
            with self.subTest(kind=kind), self.assertRaises(custody.CustodyError):
                custody.checked_records(operation, 'b'*32, ['chat'], local, ledger)

    def test_real_cut_journals_and_ledger_seal_then_reopen(self):
        import media_backup_upload_phase
        import media_operation_journal
        import realtime_config_transition_joint_test
        import realtime_jwks_publisher
        import resource_fencing
        original = realtime_jwks_publisher.PublisherPhase
        self.addCleanup(setattr, realtime_jwks_publisher, 'PublisherPhase', original)
        fixture = realtime_config_transition_joint_test.MixedTest('test_ordered_real_producers_with_shared_cut')
        fixture.setUp(); self.addCleanup(fixture.doCleanups)
        fixture.test_ordered_real_producers_with_shared_cut()
        base, cut, old_config = fixture.base, fixture.base.cut, fixture.config
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp); operation_root = root / base.context['operation_id']; operation_root.mkdir(mode=0o700)
            with patch.object(media_backup_upload_phase, 'ROOT', root):
                with media_operation_journal.Journal(operation_root/'config-journal', old_config.intent) as config:
                    config.state = copy.deepcopy(old_config.state); config.save()
                    config._realtime_config_dependencies = copy.copy(old_config._realtime_config_dependencies)
                    config._realtime_config_dependencies.journal = config
                    cut.config_journal = config
                    row = copy.deepcopy(base.ledger.row)
                    document = {'metadata':{'name':resource_fencing.LEDGER,'namespace':'voice-dev','uid':'ledger','resourceVersion':'1'},
                                'data':{'executor':json.dumps(row)}}
                    original_call = base.client.call
                    def call(*args, **kwargs):
                        if args[:3] == ('get','configmap',resource_fencing.LEDGER): return copy.deepcopy(document)
                        if args[:3] == ('patch','configmap',resource_fencing.LEDGER):
                            ops = kwargs['body']
                            if ops[0]['value'] != document['metadata']['uid'] or ops[1]['value'] != document['metadata']['resourceVersion']:
                                raise ValueError()
                            document['data'] = copy.deepcopy(ops[2]['value'])
                            document['metadata']['resourceVersion'] = str(int(document['metadata']['resourceVersion'])+1)
                            return copy.deepcopy(document)
                        return original_call(*args, **kwargs)
                    base.fence.ledger = resource_fencing.KubeLedger(base.client)
                    with patch.object(base.client, 'call', side_effect=call):
                        marker = custody.seal(base.fence,base.client,base.lock,'ns',config,cut)
                        self.assertEqual(marker['status'],'done')
                        previous,_ = base.fence.ledger.read(); previous['status'] = 'complete'
                        restored = custody.read_completed(previous,'ns')
                        self.assertEqual(restored['services'], fixture.names)
                        self.assertIn('backend-config-service-voice-realtime',restored['effects'])
                        self.assertIn('backend-config-service-voice-chat',restored['effects'])
                        with self.assertRaises(custody.CustodyError):
                            custody.seal(base.fence,base.client,base.lock,'ns',config,cut)
                        snapshot_file = operation_root/custody.FILENAME
                        original_bytes = snapshot_file.read_bytes()
                        os.chmod(snapshot_file,0o600); snapshot_file.write_bytes(original_bytes + b' '); os.chmod(snapshot_file,0o400)
                        with self.assertRaises(custody.CustodyError): custody.read_completed(previous,'ns')
                        os.chmod(snapshot_file,0o600); snapshot_file.write_bytes(original_bytes); os.chmod(snapshot_file,0o400)


if __name__ == '__main__': unittest.main()
