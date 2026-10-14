"""Exact failed prepare admission; all workspace bytes are disposable fixtures."""
import contextlib
import copy
import hashlib
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import operator_retirement as retirement
from controller import Blocked


class Tests(unittest.TestCase):
    def fixture(self,root,operation):
        spec=copy.deepcopy(retirement.SEARCH_BOOTSTRAP_REJECTIONS[operation])
        installed=root/'installed';installed.mkdir(exist_ok=True)
        for name in ('journal','responses','sources'):(installed/name).mkdir(mode=0o700,exist_ok=True)
        source=installed/'sources'/operation;source.mkdir(mode=0o700)
        raw=b'public canonical source fixture'
        (source/'input.txt').write_bytes(raw)
        tree=[{'path':'input.txt','kind':'file','bytes':len(raw),'sha256':hashlib.sha256(raw).hexdigest()}]
        spec['source_tree']={'files':1,'bytes':len(raw),'sha256':retirement.canonical(tree)}
        if spec['build_tree'] is not None:
            build=installed/'sources'/(operation+'-build');build.mkdir(mode=0o700)
            (build/'input.txt').write_bytes(raw)
            spec['build_tree']=copy.deepcopy(spec['source_tree'])
        request={'action':'prepare','nonce':spec['nonce'],'source_sha':spec['source_sha'],
            'run_id':spec['run_id'],'mode':'full','changed_services':[]}
        row={'schema':'voice-nats-bridge-request-v1','request_sha256':retirement.canonical(request),
            'request':request,'phase':'STARTED','prepare_stage':{'name':spec['stage'],'status':'STARTED'},
            'prepare_error':spec['prepare_error']}
        raw=json.dumps(row,sort_keys=True,separators=(',',':')).encode()
        response=json.dumps({'status':'BLOCKED','error':spec['response_error']},sort_keys=True,separators=(',',':')).encode()
        # Real captured hashes are not mocked: synthesized public metadata must
        # exactly reproduce both authoritative journal/response byte hashes.
        self.assertEqual(retirement.canonical(request),spec['request_sha256'])
        self.assertEqual(hashlib.sha256(raw).hexdigest(),spec['journal_sha256'])
        self.assertEqual(hashlib.sha256(response).hexdigest(),spec['response_sha256'])
        path=installed/'journal'/(spec['nonce']+'.json');path.write_bytes(raw);path.chmod(0o600)
        reply=installed/'responses'/path.name;reply.write_bytes(response);reply.chmod(0o440)
        return installed,path,spec,source

    def test_exact_failures_admitted_as_abandoned_without_history_writes(self):
        for operation in retirement.SEARCH_BOOTSTRAP_REJECTIONS:
            with self.subTest(operation=operation),tempfile.TemporaryDirectory() as td:
                root=Path(td);installed,path,spec,source=self.fixture(root,operation)
                before={p.relative_to(root).as_posix():p.read_bytes() for p in root.rglob('*') if p.is_file()}
                with patch.object(retirement.guard,'ROOT',root),patch.dict(retirement.SEARCH_BOOTSTRAP_REJECTIONS,{operation:spec}):
                    result=retirement.pending_request_accepted(installed,path,{'journals':{}})
                    self.assertEqual(result['disposition'],'ABANDONED_PRECHECKPOINT')
                    self.assertEqual(result['operation'],operation)
                    self.assertEqual(retirement.pending_abandonment_names(installed),set())
                after={p.relative_to(root).as_posix():p.read_bytes() for p in root.rglob('*') if p.is_file()}
                self.assertEqual(before,after)

    def test_drifted_journal_response_source_build_or_operation_rejected(self):
        for operation in retirement.SEARCH_BOOTSTRAP_REJECTIONS:
            for drift in ('journal','response','source','build','operation','source_symlink'):
                with self.subTest(operation=operation,drift=drift),tempfile.TemporaryDirectory() as td:
                    root=Path(td);installed,path,spec,source=self.fixture(root,operation)
                    if drift=='journal':path.write_bytes(path.read_bytes()+b' ')
                    elif drift=='response':
                        reply=installed/'responses'/path.name;reply.chmod(0o600);reply.write_bytes(b'{}')
                    elif drift=='source':(source/'input.txt').write_bytes(b'changed')
                    elif drift=='build':
                        build=installed/'sources'/(operation+'-build')
                        if build.exists():(build/'input.txt').write_bytes(b'changed')
                        else:build.mkdir()
                    elif drift=='operation':(root/('rollout-'+operation)).mkdir()
                    else:(source/'link').symlink_to(source/'input.txt')
                    with patch.object(retirement.guard,'ROOT',root),patch.dict(retirement.SEARCH_BOOTSTRAP_REJECTIONS,{operation:spec}):
                        with self.assertRaises(Blocked):retirement.pending_request_accepted(installed,path,{'journals':{}})

    def test_same_bytes_replaced_workspace_changes_custody_witness(self):
        operation=next(iter(retirement.SEARCH_BOOTSTRAP_REJECTIONS))
        with tempfile.TemporaryDirectory() as td:
            root=Path(td);installed,path,spec,source=self.fixture(root,operation)
            with patch.object(retirement.guard,'ROOT',root),patch.dict(retirement.SEARCH_BOOTSTRAP_REJECTIONS,{operation:spec}):
                before=retirement.pending_request_accepted(installed,path,{'journals':{}})
                data=(source/'input.txt').read_bytes();(source/'input.txt').rename(source/'old')
                (source/'input.txt').write_bytes(data);(source/'old').unlink()
                after=retirement.pending_request_accepted(installed,path,{'journals':{}})
                self.assertNotEqual(before,after)

    def test_actual_idle_upgrade_rechecks_both_dispositions_and_preserves_ingress(self):
        import installer
        from types import SimpleNamespace
        from operator_retirement_test import RetirementActionTests,FakeKube
        helper=RetirementActionTests()
        for failure in (None,'unknown','workspace'):
            with self.subTest(failure=failure),tempfile.TemporaryDirectory() as td:
                fixture=helper.fixture(td);kube=FakeKube(fixture[4]);helper.action(fixture,kube)
                root,base,_,_,_,binding,_,_=fixture;installed=root/'installed'
                (installed/'recovery').mkdir()
                for name in ('recovery-key.pem','recovery-cert.pem'):
                    path=installed/'recovery'/name;path.write_bytes(b'protected fixture');path.chmod(0o600)
                (installed/'policy.json').write_bytes(b'{}');(installed/'policy.json').chmod(0o600)
                specs={}
                for operation in retirement.SEARCH_BOOTSTRAP_REJECTIONS:
                    _,_,spec,source=self.fixture(root,operation);specs[operation]=spec
                if failure=='unknown':
                    path=installed/'journal'/('e'*64+'.json');path.write_bytes(b'{"phase":"STARTED"}');path.chmod(0o600)
                def swap(*args):
                    if failure=='workspace':(source/'input.txt').write_bytes(b'changed while swapping')
                with patch.object(retirement.guard,'ROOT',root),\
                     patch.object(retirement,'SNAPSHOT_SHA',hashlib.sha256(fixture[3]).hexdigest()),\
                     patch.object(retirement.root_cli,'code_binding',return_value=binding),\
                     patch.dict(retirement.SEARCH_BOOTSTRAP_REJECTIONS,specs),\
                     patch.object(installer,'operation_lock',return_value=contextlib.nullcontext()),\
                     patch.object(installer,'Kube',return_value=kube),\
                     patch.object(installer.grp,'getgrnam',return_value=SimpleNamespace(gr_gid=1000)),\
                     patch('story_witness.registry_snapshot',return_value={'fixed':'witness'}),\
                     patch.object(installer,'_upgrade_code',side_effect=swap) as upgrade:
                    if failure is None:
                        installer.upgrade_idle_search_bootstrap_v1(root/'candidate')
                        upgrade.assert_called_once_with(root/'candidate',installed,1000,
                            installer.SEARCH_BOOTSTRAP_INSTALLED_BINDING,'idle-search-bootstrap-v1','search-a025')
                    else:
                        with self.assertRaises(Blocked):installer.upgrade_idle_search_bootstrap_v1(root/'candidate')
                        if failure=='unknown':upgrade.assert_not_called()
                    self.assertEqual((installed/'inbox').stat().st_mode&0o7777,0o1730)


if __name__=='__main__':unittest.main()
