"""Real Journal/Fence; synthetic Kube and root handoff boundaries."""
import base64
import copy
import os
from pathlib import Path
import stat
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import media_backup_phase_test as fixture
import media_operation_journal
import realtime_jwks_publisher_phase_test as shared
import resource_fencing
import space_p3_issuer_create as subject


class PhaseTest(unittest.TestCase):
    def setUp(self):
        self.base=fixture.PhaseTest('test_exact_terminal_receipt_and_no_replay')
        self.addCleanup(self.checked_base_cleanup); self.base.setUp()
        self.client=fixture.Client(self.base.lock)
        roots={name:{'uid':name+'-uid','resourceVersion':'1','der_sha256':'a'*64} for name,_ in subject.ROOTS}
        intent={'resource_context':self.base.context,'namespace_uid':'ns','space_p3_create':
                {'source_repo':'/source','source_sha':self.base.context['commit'],'roots':roots}}
        self.journal=media_operation_journal.Journal(Path(self.base.temp.name)/'space',intent)
        self.addCleanup(self.journal.close); self.client.journal=self.journal
        self.fence=resource_fencing.Fence(self.base.context,lambda:copy.deepcopy(self.base.card),shared.View(self.base.ledger,self.client))
        self.typepatch=patch.object(subject.frontend_runtime_executor,'KubectlTransport',fixture.Client)
        self.typepatch.start(); self.addCleanup(self.typepatch.stop)
        self.sourcepatch=patch.object(subject.source,'verify'); self.sourcepatch.start(); self.addCleanup(self.sourcepatch.stop)
        self.phase=subject.Session(self.journal,self.fence,self.client,self.base.lock,'ns')
        self.phase.roots={name:{'identity':roots[name],'certificate':b'public-ca'} for name,_ in subject.ROOTS}
        self.phase.workload={'uid':'space','generation':1,'spec_sha256':'f'*64}
        self.rootpatch=patch.object(self.phase,'_public',return_value=copy.deepcopy(self.phase.roots))
        self.workpatch=patch.object(self.phase,'_workload',return_value=copy.deepcopy(self.phase.workload))
        self.rootpatch.start(); self.workmock=self.workpatch.start(); self.addCleanup(self.rootpatch.stop); self.addCleanup(self.workpatch.stop)
        self.objects={name:{'apiVersion':'v1','kind':'Secret','metadata':{'name':name,'namespace':'voice-dev'},'type':'Opaque',
                           'data':{key:base64.b64encode(('fixture-'+key).encode()).decode() for key in keys}}
                      for name,keys in subject.packet.MEMBERS.items()}
        self.actual={}; self.calls=[]; self.lost=False
        self.getpatch=patch.object(subject,'_get',side_effect=lambda name:copy.deepcopy(self.actual.get(name)))
        self.createpatch=patch.object(subject,'_create',side_effect=self.create)
        self.getpatch.start(); self.createpatch.start(); self.addCleanup(self.getpatch.stop); self.addCleanup(self.createpatch.stop)

    def checked_base_cleanup(self):
        if not self.base.doCleanups(): raise AssertionError('nested fixture cleanup failed')

    def create(self,obj,dry):
        name=obj['metadata']['name']; self.calls.append((name,dry)); value=copy.deepcopy(obj)
        if not dry:
            value['metadata'].update(uid='uid-'+name,resourceVersion='7'); self.actual[name]=copy.deepcopy(value)
            if self.lost: raise OSError('lost response')
        return value

    def ready(self):
        ready={'status':'ready','nonce':'a'*32,'operation_id':self.base.context['operation_id'],
               'source_sha':self.base.context['commit'],'namespace_uid':'ns','pid':subject.os.getpid(),
               'start_ticks':subject.common._ticks(),'socket_dev':1,'socket_ino':2,'workload':self.phase.workload}
        self.phase.handoff=copy.deepcopy(ready); self.phase.pending=True; self.phase.listener=object(); self.phase.dirfd=0; self.phase.inode=(1,2)
        self.journal.state['steps'][subject.STEP]=copy.deepcopy(ready); self.journal.save(); self.base.ledger.row[subject.LEDGER]=copy.deepcopy(ready)

    def run_finish(self):
        self.ready()
        info=SimpleNamespace(st_dev=1,st_ino=2,st_mode=stat.S_IFSOCK)
        with patch.object(subject.os,'stat',return_value=info), patch.object(subject.channel,'receive',return_value={}), \
             patch.object(subject.packet,'validate',return_value=(copy.deepcopy(self.objects),{'keys':['public']})), \
             patch.object(subject.crypto,'der',return_value=b'fixture-der'), patch.object(self.phase,'close'):
            return self.phase.finish()

    def test_create_only_receipt_rechecks_live_exact_objects(self):
        receipt=self.run_finish()
        self.assertEqual(receipt['scope'],'space-two-secret-create-only')
        self.assertEqual([dry for _,dry in self.calls],[True,True,False,False])
        with patch.object(subject.packet,'validate',return_value=(copy.deepcopy(self.objects),{'keys':['public']})), \
             patch.object(subject.crypto,'der',return_value=b'fixture-der'):
            self.assertEqual(subject.verify_completed(self.phase),receipt)
        self.assertNotIn('fixture-current.pem',str(self.journal.state)); self.assertNotIn('fixture-current.pem',str(self.base.ledger.row))
        self.actual['voice-space-principal-signing']['metadata']['uid']='other'
        with self.assertRaises(subject.CreateError): subject.verify_completed(self.phase)
        self.assertEqual(self.base.ledger.row['status'],'uncertain')

    def test_lost_create_is_uncertain_without_replay(self):
        self.lost=True
        with self.assertRaises(subject.CreateError): self.run_finish()
        self.assertEqual(len(self.actual),1); self.assertEqual(self.base.ledger.row['status'],'uncertain')
        self.assertEqual(sum(not dry for _,dry in self.calls),1)
        with self.assertRaises(subject.CreateError): self.run_finish()
        self.assertEqual(sum(not dry for _,dry in self.calls),1)

    def test_collision_never_adopted(self):
        self.actual['voice-space-media-server-tls']={'foreign':True}
        with self.assertRaises(subject.CreateError): self.run_finish()
        self.assertFalse(any(not dry for _,dry in self.calls))

    def test_workload_changes_after_done_never_return_success(self):
        write=self.phase._write
        def changed(**kwargs):
            write(**kwargs)
            if kwargs.get('handoff',{}).get('status')=='done':
                self.workmock.return_value=dict(self.phase.workload,generation=2)
        with patch.object(self.phase,'_write',side_effect=changed):
            with self.assertRaises(subject.CreateError): self.run_finish()
        self.assertFalse(self.phase.completed); self.assertEqual(self.base.ledger.row['status'],'uncertain')

    @unittest.skipUnless(os.name=='posix' and os.geteuid()==1000,'fixed pmd UID fixture')
    def test_prepare_owns_real_private_socket_and_is_one_shot(self):
        parent=Path(self.base.temp.name)/'operations'; parent.mkdir(mode=0o700)
        path=parent/self.phase.suffix; path.mkdir(mode=0o700)
        with patch.object(subject,'SOCKET_PARENT',parent):
            result=self.phase.prepare()
            self.assertEqual(result['status'],'ready'); self.assertTrue((path/'packet.sock').is_socket())
            self.assertEqual(self.base.ledger.row[subject.LEDGER],result)
            with self.assertRaises(subject.CreateError): self.phase.prepare()
        self.assertFalse((path/'packet.sock').exists())


class CleanupTest(unittest.TestCase):
    def test_nested_failure_is_reported_and_remaining_cleanups_run(self):
        events=[]
        class Nested(PhaseTest):
            def runTest(self):
                self.base.addCleanup(lambda:events.append('after-failure'))
                def fail(): raise RuntimeError('fixture cleanup failure')
                self.base.addCleanup(fail)
        result=unittest.TestResult(); Nested().run(result)
        self.assertFalse(result.wasSuccessful()); self.assertTrue(result.errors or result.failures)
        self.assertEqual(events,['after-failure'])


if __name__ == '__main__': unittest.main()
