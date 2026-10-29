"""Controlled registry and Kubernetes boundaries; no live qualification."""
import hashlib
import copy
import json
import tempfile
import unittest
from unittest.mock import patch
from types import SimpleNamespace

import realtime_jwks_publisher_oci as admission
import frontend_runtime_executor
import media_operation_guard as guard
import media_operation_journal
import resource_fencing


class SourceTests(unittest.TestCase):
    def test_fixed_source_requires_candidate_disk_and_reviewed_hash(self):
        raw = b'def collect():\n    return {"index": b"i", "manifest": b"m", "config": b"c"}\n'
        sha = hashlib.sha256(raw).hexdigest()
        with patch.object(admission, 'COLLECTOR_SHA256', sha), \
             patch.object(admission.source, '_blob', return_value=raw) as blob, \
             patch.object(admission.source, '_disk', return_value=raw) as disk:
            module = admission._source_module('/protected/source', 'a'*40)
            self.assertEqual(module.collect()['index'], b'i')
            blob.assert_called_once_with('/protected/source', 'a'*40, admission.COLLECTOR_PATH)
            disk.assert_called_once_with('/protected/source', admission.COLLECTOR_PATH)
            disk.return_value = raw + b'changed'
            with self.assertRaises(admission.OCIError): admission._source_module('/protected/source', 'a'*40)
            disk.return_value = raw
            blob.return_value = raw + b'changed'
            with self.assertRaises(admission.OCIError): admission._source_module('/protected/source', 'a'*40)


class PhaseTests(unittest.TestCase):
    def test_three_real_journals_ledger_loss_and_node_mismatch(self):
        context = dict(project='voice', resource='DEV', actor='scheduled_devops',
            claim='11111111-1111-4111-8111-111111111111', window='22222222-2222-4222-8222-222222222222',
            generation=2, revision=3, operation_id='media-c1-'+'a'*32, commit='b'*40,
            images={'voice-auth/auth':'registry/auth@sha256:'+'c'*64})
        plan = {'source_repo':'/protected/source', 'source_sha':context['commit'],
                'node':{'name':'node','uid':'node-uid','resourceVersion':'4','hostname':'node'}}
        package_node = {'name':'node','uid':'node-uid','hostname':'node','architecture':'amd64'}
        intent = {'resource_context':context, 'namespace_uid':'ns', 'realtime_jwks_publisher':plan}
        lock = {'metadata':{'name':guard.LOCK,'namespace':'voice-dev','uid':'lock','resourceVersion':'5',
                            'labels':dict(guard.LABELS)},
                'data':{'resource_operation':context['operation_id'], 'operation':'run',
                        'resource_context':json.dumps(context,sort_keys=True,separators=(',',':'))}}
        namespace = {'metadata':{'name':'voice-dev','uid':'ns','labels':dict(guard.LABELS)}}
        row = dict(context, status='active', step='admitted')
        document = {'metadata':{'name':resource_fencing.LEDGER,'namespace':'voice-dev','uid':'ledger','resourceVersion':'1'},
                    'data':{'executor':json.dumps(row)}}
        writes = []
        def call(*args, **kwargs):
            if args[0] == 'patch':
                patch_body = kwargs['body']; writes.append(copy.deepcopy(patch_body))
                if patch_body[0]['value'] != 'ledger' or patch_body[1]['value'] != document['metadata']['resourceVersion']:
                    raise RuntimeError('CAS failed')
                document['data'] = patch_body[2]['value']
                document['metadata']['resourceVersion'] = str(int(document['metadata']['resourceVersion']) + 1)
                return copy.deepcopy(document)
            if args[1] == 'namespace': return copy.deepcopy(namespace)
            if args[2] == guard.LOCK: return copy.deepcopy(lock)
            return copy.deepcopy(document)
        card = dict(code='DEV', environment='dev', owner=context['actor'], claim=context['claim'],
                    generation=2, window=context['window'], revision=3, state='held', phase='deploy',
                    candidate={'state':'known','commit':context['commit'],'images':context['images']})
        raw = (b'synthetic index', b'synthetic manifest', b'synthetic config')
        digest = lambda value: 'sha256:' + hashlib.sha256(value).hexdigest()
        module = SimpleNamespace(collect=lambda:dict(index=raw[0],manifest=raw[1],config=raw[2]),
                                 validate=lambda *args:None)
        with tempfile.TemporaryDirectory() as root, \
             media_operation_journal.Journal(root+'/oci',intent) as journal, \
             media_operation_journal.Journal(root+'/package',intent) as package_journal, \
             media_operation_journal.Journal(root+'/publisher',intent) as publisher_journal, \
             media_operation_journal.Journal(root+'/p3',intent) as p3_journal:
            def facades(private_journal):
                client = frontend_runtime_executor.KubectlTransport.__new__(frontend_runtime_executor.KubectlTransport)
                client.journal = private_journal; client.call = call
                check = resource_fencing.ResourceCheck(admission.full_cohort_predecessor_phase.TT_ENDPOINT,context)
                return client, resource_fencing.Fence(context,check,resource_fencing.KubeLedger(client))
            client, fence = facades(journal); pc, pf = facades(package_journal); wc, wf = facades(publisher_journal)
            p3c,p3f = facades(p3_journal)
            package = admission.full_cohort_package_phase.PackagePhase.__new__(admission.full_cohort_package_phase.PackagePhase)
            package.journal,package.fence,package.client = package_journal,pf,pc
            package.lock,package.namespace_uid = copy.deepcopy(lock),'ns'
            publisher = admission.publisher_module.PublisherPhase.__new__(admission.publisher_module.PublisherPhase)
            publisher.journal,publisher.fence,publisher.client = publisher_journal,wf,wc
            publisher.lock,publisher.namespace_uid,publisher.plan = copy.deepcopy(lock),'ns',copy.deepcopy(plan)
            publisher.p3 = admission.publisher_module.p3.Session.__new__(admission.publisher_module.p3.Session)
            publisher.p3.journal,publisher.p3.fence,publisher.p3.client = p3_journal,p3f,p3c
            with patch.object(resource_fencing.ResourceCheck,'__call__',return_value=card), \
                 patch.object(admission.full_cohort_predecessor_phase,'_no_tt_proxy'), \
                 patch.object(admission.publisher_module.PublisherPhase,'_guard'), \
                 patch.object(admission.publisher_module.PublisherPhase,'_node'), \
                 patch.object(admission.full_cohort_package_phase.PackagePhase,'verify_current',
                              return_value={'nodes':[package_node]}), \
                 patch.object(admission,'_source_module',return_value=module), \
                 patch.object(admission.publisher_module,'INDEX',digest(raw[0])), \
                 patch.object(admission.publisher_module,'PLATFORM',digest(raw[1])), \
                 patch.object(admission.publisher_module,'CONFIG',digest(raw[2])), \
                 patch.object(admission.publisher_module,'qualify_oci',return_value={
                     'index':digest(raw[0]),'platform':digest(raw[1]),'config':digest(raw[2])}):
                phase = admission.PublisherOCIPhase(journal,fence,client,lock,'ns',package,publisher)
                with patch.object(admission.full_cohort_package_phase.PackagePhase,'verify_current',return_value={'nodes':[]}):
                    with self.assertRaises(admission.OCIError): phase.run('fixture-token')
                namespace['metadata']['uid'] = 'foreign'
                with self.assertRaises(ValueError): phase.run('fixture-token')
                namespace['metadata']['uid'] = 'ns'
                with patch.object(admission,'_source_module',side_effect=admission.OCIError('source absent')):
                    with self.assertRaisesRegex(admission.OCIError,'source absent'): phase.run('fixture-token')
                self.assertEqual(writes,[])
                self.assertEqual(phase.run('fixture-token'),raw)
                self.assertEqual(phase.verify_current('fixture-token'),raw)
                saved = copy.deepcopy(journal.state['steps'][admission.STEP]); source_reads = [0]
                def remove_local(*args):
                    source_reads[0] += 1
                    if source_reads[0] == 3: del journal.state['steps'][admission.STEP]
                    return module
                with patch.object(admission,'_source_module',side_effect=remove_local):
                    with self.assertRaises(admission.OCIError): phase.verify_current('fixture-token')
                journal.state['steps'][admission.STEP] = saved; journal.save()
                current, revision = fence.ledger.read()
                del current[admission.MARKER]
                fence.ledger.write(revision,current)
                del journal.state['steps'][admission.STEP]; journal.save()
                writer = fence.ledger.write
                def lost_done(revision, value):
                    writer(revision,value)
                    if value.get(admission.MARKER,{}).get('status') == 'done':
                        raise OSError('lost completion response')
                retry = admission.PublisherOCIPhase(journal,fence,client,lock,'ns',package,publisher)
                with patch.object(fence.ledger,'write',side_effect=lost_done), \
                     patch.object(fence,'uncertain',side_effect=OSError('TT unavailable')):
                    with self.assertRaisesRegex(OSError,'lost completion response'): retry.run('fixture-token')
                self.assertTrue(retry.poisoned)
                self.assertEqual(journal.state['steps'][admission.STEP]['status'],'pending')
                reopened = admission.PublisherOCIPhase(journal,fence,client,lock,'ns',package,publisher)
                with self.assertRaises(admission.OCIError): reopened.verify_current('fixture-token')
                p3_read = p3f.ledger.read
                def close_during_final_read():
                    result = p3_read(); package_journal.close(); return result
                with patch.object(p3f.ledger,'read',side_effect=close_during_final_read):
                    with self.assertRaises(admission.OCIError): reopened._guard()
if __name__ == '__main__': unittest.main()
