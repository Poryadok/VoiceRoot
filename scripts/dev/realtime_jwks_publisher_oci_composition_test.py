"""Real collector and source qualification over controlled external I/O.

Set VOICE_OCI_SOURCE_REPO and VOICE_OCI_SOURCE_SHA to the immutable selected
source checkout and VOICE_OCI_RAW_DIR to previously fetched public metadata. The
registry transport is simulated; no cluster, TT or rollout is contacted.
"""
import copy
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import frontend_runtime_executor
import full_cohort_package_phase as package_module
import media_operation_guard as guard
import media_operation_journal
import realtime_jwks_publisher as publisher_module
import realtime_jwks_publisher_oci as oci
import realtime_p3_create_only as p3_module
import resource_fencing

@unittest.skipUnless(os.environ.get('VOICE_OCI_SOURCE_REPO') and os.environ.get('VOICE_OCI_SOURCE_SHA') and
                     os.environ.get('VOICE_OCI_RAW_DIR'),
                     'controlled immutable source and public metadata fixtures required')
class CompositionTests(unittest.TestCase):
    def test_real_collector_and_publisher_pin_inside_fenced_phase(self):
        repo = os.environ['VOICE_OCI_SOURCE_REPO']
        candidate = os.environ['VOICE_OCI_SOURCE_SHA']
        self.assertRegex(candidate, r'^[a-f0-9]{40}$')
        self.assertEqual(subprocess.run(['git','-C',repo,'rev-parse','HEAD'],check=True,stdout=subprocess.PIPE,
                                        stderr=subprocess.DEVNULL,text=True).stdout.strip(), candidate)
        raw_dir = Path(os.environ['VOICE_OCI_RAW_DIR'])
        raw = tuple((raw_dir / name).read_bytes() for name in ('index','manifest','config'))
        self.assertEqual([hashlib.sha256(value).hexdigest() for value in raw],
                         [publisher_module.INDEX[7:], publisher_module.PLATFORM[7:], publisher_module.CONFIG[7:]])
        context = dict(project='voice', resource='DEV', actor='scheduled_devops',
            claim='11111111-1111-4111-8111-111111111111', window='22222222-2222-4222-8222-222222222222',
            generation=2, revision=3, operation_id='media-c1-'+'a'*32, commit=candidate,
            images={'voice-auth/auth':'registry/auth@sha256:'+'c'*64})
        node = {'name':'node','uid':'node-uid','resourceVersion':'4','hostname':'node'}
        package_node = {'name':'node','uid':'node-uid','hostname':'node','architecture':'amd64'}
        plan = {'source_repo':repo,'source_sha':candidate,'node':node}
        base_intent = {'resource_context':context,'namespace_uid':'ns'}
        publisher_intent = base_intent | {'realtime_jwks_publisher':plan}
        package_intent = base_intent | {'full_cohort_package':{'run_id':1,'run_attempt':1,'nodes':[package_node]}}
        p3_intent = base_intent | {'realtime_p3_create':{'issuer_source_sha':'d'*40,
                              'source_repo':repo,'issuer_suffix':'a'*32}}
        lock = {'metadata':{'name':guard.LOCK,'namespace':'voice-dev','uid':'lock','resourceVersion':'5',
                            'labels':dict(guard.LABELS)},
                'data':{'resource_operation':context['operation_id'],'operation':'run',
                        'resource_context':json.dumps(context,sort_keys=True,separators=(',',':'))}}
        namespace = {'metadata':{'name':'voice-dev','uid':'ns','labels':dict(guard.LABELS)}}
        node_doc = {'metadata':{'name':'node','uid':'node-uid','resourceVersion':'4',
                    'labels':{'kubernetes.io/hostname':'node','kubernetes.io/os':'linux','kubernetes.io/arch':'amd64'}},
                    'spec':{},'status':{'nodeInfo':{'operatingSystem':'linux','architecture':'amd64'},
                                        'conditions':[{'type':'Ready','status':'True'}]}}
        ledger_doc = {'metadata':{'name':resource_fencing.LEDGER,'namespace':'voice-dev','uid':'ledger','resourceVersion':'1'},
                      'data':{'executor':json.dumps(dict(context,status='active',step='admitted'))}}
        def call(*args, **kwargs):
            if args[0] == 'patch':
                body = kwargs['body']
                if body[0]['value'] != 'ledger' or body[1]['value'] != ledger_doc['metadata']['resourceVersion']:
                    raise RuntimeError('CAS failed')
                ledger_doc['data'] = copy.deepcopy(body[2]['value'])
                ledger_doc['metadata']['resourceVersion'] = str(int(ledger_doc['metadata']['resourceVersion'])+1)
                return copy.deepcopy(ledger_doc)
            if args[1] == 'namespace': return copy.deepcopy(namespace)
            if args[1] == 'node': return copy.deepcopy(node_doc)
            if args[1] == 'nodes': return {'items':[copy.deepcopy(node_doc)]}
            if args[2] == guard.LOCK: return copy.deepcopy(lock)
            return copy.deepcopy(ledger_doc)
        card = dict(code='DEV',environment='dev',owner=context['actor'],claim=context['claim'],generation=2,
                    window=context['window'],revision=3,state='held',phase='deploy',
                    candidate={'state':'known','commit':candidate,'images':context['images']})

        class Response:
            status = 200
            def __init__(self, body): self.body = body
            def getheader(self, name): return None
            def read(self, size):
                result,self.body = self.body[:size],self.body[size:]
                return result
        class Connection:
            sock = None
            def __init__(self, host, **kwargs): self.host = host
            def request(self, method, path, headers): self.path = path
            def getresponse(self):
                if self.host == 'auth.docker.io': return Response(b'{"token":"fixture-token"}')
                if self.host != 'registry-1.docker.io': raise AssertionError('unexpected host')
                for digest, body in zip((publisher_module.INDEX,publisher_module.PLATFORM,publisher_module.CONFIG),raw):
                    if self.path.endswith(digest): return Response(body)
                raise AssertionError('unexpected registry path')
            def close(self): pass

        with tempfile.TemporaryDirectory() as root, \
             media_operation_journal.Journal(root+'/oci',publisher_intent) as journal, \
             media_operation_journal.Journal(root+'/package',package_intent) as package_journal, \
             media_operation_journal.Journal(root+'/publisher',publisher_intent) as publisher_journal, \
             media_operation_journal.Journal(root+'/p3',p3_intent) as p3_journal:
            def facade(j):
                client = frontend_runtime_executor.KubectlTransport.__new__(frontend_runtime_executor.KubectlTransport)
                client.journal=j; client.call=call
                check=resource_fencing.ResourceCheck(oci.full_cohort_predecessor_phase.TT_ENDPOINT,context)
                return client,resource_fencing.Fence(context,check,resource_fencing.KubeLedger(client))
            client,fence=facade(journal); pc,pf=facade(package_journal)
            wc,wf=facade(publisher_journal); p3c,p3f=facade(p3_journal)
            p3=p3_module.Session(p3_journal,p3f,p3c,lock,'ns')
            p3.completed=True; p3.handoff={'status':'done','fixture':'controlled'}; p3.effects={}
            p3_journal.state['steps']['realtime-p3-handoff']=copy.deepcopy(p3.handoff); p3_journal.save()
            root_path=Path(root)/context['operation_id']; root_path.mkdir(mode=0o700)
            package_file=root_path/'canonical-application-package.json'; package_file.write_bytes(b'{}'); package_file.chmod(0o600)
            receipt={'operation_id':context['operation_id'],'source_sha':candidate,'images':context['images'],
                     'nodes':[package_node],'run_id':1,'run_attempt':1,'package_sha256':hashlib.sha256(b'{}').hexdigest()}
            receipt_sha=hashlib.sha256(json.dumps(receipt,sort_keys=True,separators=(',',':')).encode()).hexdigest()
            done={'status':'done','receipt_sha256':receipt_sha}
            package_journal.state['steps'][package_module.STEP]=done | {'receipt':copy.deepcopy(receipt)}; package_journal.save()
            current,revision=fence.ledger.read(); current['full_cohort_package']=done; current['realtime_p3_handoff']=copy.deepcopy(p3.handoff)
            current['realtime_p3_objects']={}; fence.ledger.write(revision,current)
            package=package_module.PackagePhase(package_journal,pf,pc,lock,'ns')
            publisher=publisher_module.PublisherPhase(publisher_journal,wf,wc,lock,'ns',p3)
            with patch.object(resource_fencing.ResourceCheck,'__call__',return_value=card), \
                 patch.object(oci.full_cohort_predecessor_phase,'_no_tt_proxy'), \
                 patch.object(package_module,'ROOT',Path(root)), \
                 patch('http.client.HTTPSConnection',Connection):
                phase=oci.PublisherOCIPhase(journal,fence,client,lock,'ns',package,publisher)
                self.assertEqual(phase.run('fixture-token'),raw)
                self.assertEqual(phase.verify_current('fixture-token'),raw)
                self.assertEqual(publisher_module.qualify_oci(*raw),
                                 {'index':publisher_module.INDEX,'platform':publisher_module.PLATFORM,'config':publisher_module.CONFIG})


if __name__ == '__main__': unittest.main()
