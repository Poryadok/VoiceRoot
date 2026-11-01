"""Normal service umask must retain group-only scratch credential access."""
import os
import hashlib
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import actor_auth
import bootstrap_auth


class RuntimeBoundary(Exception):
    pass


class ScratchInputTests(unittest.TestCase):
    def test_successful_prove_removes_scratch_after_confirmed_cleanup(self):
        from resource_claim import InstallationAuthority
        from nats_contract_plan import DECLARATIONS
        installation=InstallationAuthority('owned-isolated-v1','734fe817-dc75-4732-89ac-6aecb83f0a32',
            'r20260930a4',hashlib.sha256(b'public-fixture').hexdigest(),'b'*64,'c'*64)
        for module in (actor_auth,bootstrap_auth):
            with self.subTest(helper=module.__name__),tempfile.TemporaryDirectory() as td:
                root=Path(td);binary=root/'dummy-helper';binary.write_bytes(b'not executed');instances=[]
                class Runtime:
                    def __init__(self,base,operation):self.base=base;self.owned={};self.removed=set();instances.append(self)
                    def create(self,suffix,*args,**kwargs):
                        name='broker' if suffix=='broker' else 'client';self.owned[name]={'id':name};return name
                    def start_broker(self,*args):self.owned['broker']={'id':'broker'};return 'broker'
                    def inspect(self,name):return {'Id':name}
                    def stop(self,name):pass
                    def run(self,args,**kwargs):
                        if args[0]=='wait':return '0'
                        if args[0]=='rm':self.removed.add(args[2]);return ''
                        if args[0] in ('start','ps'):return ''
                        raise AssertionError(args)
                    def no_operation_containers(self):self_test.assertEqual(self.removed,set(self.owned))
                    def monitor_jsz(self,broker):
                        return {'messages':0,'streams':len({stream for stream,_ in DECLARATIONS}),
                            'consumers':len(DECLARATIONS),'account_details':[{'id':'public-fixture','stream_detail':[{}]}]}
                self_test=self
                class Actor:
                    def __init__(self,*args,**kwargs):pass
                    def _request(self,*args,**kwargs):pass
                    def mutate(self,*args,**kwargs):pass
                    def info_config(self,name):
                        return next(dict(config) for stream,config in DECLARATIONS if name==stream+'/'+config['durable_name'])
                with patch.object(module.guard,'ROOT',root),patch.object(module,'DockerRuntime',Runtime), \
                     patch.object(module,'secret_bytes',return_value=b'public-fixture'),patch.object(module.os,'chown'), \
                     patch.object(bootstrap_auth,'Actor',Actor),patch.object(bootstrap_auth,'ready'):
                    if module is actor_auth:result=module.prove(b'harmless-fixture',{},binary,lambda:None)
                    else:result=module.prove(b'harmless-fixture',{},lambda:None,installation=installation)
                self.assertTrue(result['server_authentication_verified'] if module is actor_auth else result['verified'])
                self.assertFalse(instances[0].base.exists())
                self.assertEqual([path.name for path in root.iterdir()],['dummy-helper'])

    def test_real_prove_paths_retain_inputs_after_runtime_failure(self):
        from resource_claim import InstallationAuthority
        installation=InstallationAuthority('owned-isolated-v1','734fe817-dc75-4732-89ac-6aecb83f0a32',
            'r20260930a4',hashlib.sha256(b'public-fixture').hexdigest(),'b'*64,'c'*64)
        for module in (actor_auth,bootstrap_auth):
            for cleanup_fails in (False,True):
                with self.subTest(helper=module.__name__,cleanup_fails=cleanup_fails),tempfile.TemporaryDirectory() as td:
                    root=Path(td);binary=root/'dummy-helper';binary.write_bytes(b'not executed');observed=[]
                    class Runtime:
                        def __init__(self,base,operation):self.base=base;self.owned={'broker':{'id':'a'*64}}
                        def create(self,*args,**kwargs):return 'broker'
                        def start_broker(self,*args):raise RuntimeBoundary('start failed')
                        def inspect(self,name):
                            name='actor.creds' if module is actor_auth else 'bootstrap.creds'
                            observed.append((self.base/'inputs'/name).is_file())
                            if cleanup_fails:raise RuntimeError('cleanup unconfirmed')
                            return {'Id':'a'*64}
                        def run(self,args,**kwargs):
                            if args[0]=='start':raise RuntimeBoundary('start failed')
                            if args[0] in ('rm','ps'):return ''
                            raise AssertionError(args)
                        def no_operation_containers(self):pass
                    with patch.object(module.guard,'ROOT',root),patch.object(module,'DockerRuntime',Runtime), \
                         patch.object(module,'secret_bytes',return_value=b'public-fixture'),patch.object(module.os,'chown'):
                        expected=RuntimeError if cleanup_fails else RuntimeBoundary
                        with self.assertRaises(expected):
                            if module is actor_auth:module.prove(b'harmless-fixture',{},binary,lambda:None)
                            else:module.prove(b'harmless-fixture',{},lambda:None,installation=installation)
                    self.assertEqual(observed,[True])
                    retained=[p for p in root.iterdir() if p.is_dir()]
                    self.assertEqual(len(retained),1)
                    if retained:
                        name='actor.creds' if module is actor_auth else 'bootstrap.creds'
                        self.assertEqual((retained[0]/'inputs'/name).read_bytes(),b'harmless-fixture')

    def test_lost_create_reply_keeps_inputs_when_operation_has_orphan(self):
        from resource_claim import InstallationAuthority
        from docker_runtime import DockerRuntime as RealRuntime
        from controller import Blocked
        installation=InstallationAuthority('owned-isolated-v1','734fe817-dc75-4732-89ac-6aecb83f0a32',
            'r20260930a4',hashlib.sha256(b'public-fixture').hexdigest(),'b'*64,'c'*64)
        for module in (actor_auth,bootstrap_auth):
            with self.subTest(helper=module.__name__),tempfile.TemporaryDirectory() as td:
                root=Path(td);binary=root/'dummy-helper';binary.write_bytes(b'not executed');observed=[]
                class Runtime(RealRuntime):
                    def __init__(self,base,operation):
                        super().__init__(base,operation,run=self.command);self.orphan=False
                    def create(self,*args,**kwargs):self.orphan=True;raise RuntimeBoundary('lost create reply')
                    def start_broker(self,*args):self.orphan=True;raise RuntimeBoundary('lost create reply')
                    def command(self,args,**kwargs):
                        name='actor.creds' if module is actor_auth else 'bootstrap.creds'
                        observed.append((self.base/'inputs'/name).is_file())
                        self_test.assertEqual(args,['ps','-a','--filter','label=voice.known-nats.operation='+self.operation,'--format','{{.ID}}'])
                        return 'a'*64 if self.orphan else ''
                self_test=self
                with patch.object(module.guard,'ROOT',root),patch.object(module,'DockerRuntime',Runtime), \
                     patch.object(module,'secret_bytes',return_value=b'public-fixture'),patch.object(module.os,'chown'):
                    with self.assertRaisesRegex(Blocked,'operation_container_writer_present'):
                        if module is actor_auth:module.prove(b'harmless-fixture',{},binary,lambda:None)
                        else:module.prove(b'harmless-fixture',{},lambda:None,installation=installation)
                self.assertEqual(observed,[True])
                dirs=[p for p in root.iterdir() if p.is_dir()];self.assertEqual(len(dirs),1)
                name='actor.creds' if module is actor_auth else 'bootstrap.creds'
                self.assertEqual((dirs[0]/'inputs'/name).read_bytes(),b'harmless-fixture')

    def test_ambiguous_create_keeps_inputs_even_if_orphan_appears_after_empty_inventory(self):
        from resource_claim import InstallationAuthority
        from docker_runtime import DockerRuntime as RealRuntime
        installation=InstallationAuthority('owned-isolated-v1','734fe817-dc75-4732-89ac-6aecb83f0a32',
            'r20260930a4',hashlib.sha256(b'public-fixture').hexdigest(),'b'*64,'c'*64)
        for module in (actor_auth,bootstrap_auth):
            with self.subTest(helper=module.__name__),tempfile.TemporaryDirectory() as td:
                root=Path(td);binary=root/'dummy-helper';binary.write_bytes(b'not executed');instances=[]
                class Runtime(RealRuntime):
                    def __init__(self,base,operation):
                        super().__init__(base,operation,run=self.command);self.late_orphan=False;instances.append(self)
                    def create(self,*args,**kwargs):raise RuntimeBoundary('lost create reply')
                    def start_broker(self,*args):raise RuntimeBoundary('lost create reply')
                    def command(self,args,**kwargs):
                        self.late_orphan=True # Empty inventory is observed immediately before late creation.
                        return ''
                with patch.object(module.guard,'ROOT',root),patch.object(module,'DockerRuntime',Runtime), \
                     patch.object(module,'secret_bytes',return_value=b'public-fixture'),patch.object(module.os,'chown'):
                    with self.assertRaisesRegex(RuntimeBoundary,'lost create reply'):
                        if module is actor_auth:module.prove(b'harmless-fixture',{},binary,lambda:None)
                        else:module.prove(b'harmless-fixture',{},lambda:None,installation=installation)
                self.assertTrue(instances[0].late_orphan)
                name='actor.creds' if module is actor_auth else 'bootstrap.creds'
                self.assertEqual((instances[0].base/'inputs'/name).read_bytes(),b'harmless-fixture')

    def test_actual_helper_setup_preserves_private_modes_under_service_umask(self):
        for mask in (0o022, 0o077):
            for module in (actor_auth, bootstrap_auth):
                with self.subTest(mask=oct(mask), helper=module.__name__):
                    self.check_setup(module, mask)

    def check_setup(self, module, mask):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            binary = root / 'dummy-helper'
            binary.write_bytes(b'not executed')
            calls = []
            observed = []

            def boundary(base, operation):
                inputs = base / 'inputs'
                name = 'actor.creds' if module is actor_auth else 'bootstrap.creds'
                credential = inputs / name
                observed.append((inputs.stat().st_mode & 0o777,
                                 credential.stat().st_mode & 0o777))
                self.assertIn((inputs, 0, 65532), calls)
                self.assertIn((credential, 0, 65532), calls)
                raise RuntimeBoundary()

            previous = os.umask(mask)
            try:
                with patch.object(module.guard, 'ROOT', root), \
                        patch.object(module, 'DockerRuntime', boundary), \
                        patch.object(module, 'secret_bytes', return_value=b'public-fixture'), \
                        patch.object(module.os, 'chown', side_effect=lambda p, u, g: calls.append((p, u, g))):
                    with self.assertRaises(RuntimeBoundary):
                        if module is actor_auth:
                            module.prove(b'harmless-fixture', {}, binary, lambda: None)
                        else:
                            with patch.object(module, 'hash_bytes', return_value=module.ACCOUNT_SHA):
                                module.prove(b'harmless-fixture', {}, lambda: None)
                self.assertEqual(observed, [(0o750, 0o440)])
                self.assertEqual([p.name for p in root.iterdir()], ['dummy-helper'])
            finally:
                os.umask(previous)


if __name__ == '__main__':
    unittest.main()
