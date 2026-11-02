"""Actual Journal/Fence and crypto, synthetic Kubernetes and root channel."""
import base64
import copy
from pathlib import Path
from types import SimpleNamespace
import sys
import unittest
from unittest.mock import patch

import media_backup_phase_test as fixtures
import media_operation_journal as journal
import realtime_p3_create_only as create
import realtime_p3_create_only_packet as packet
import realtime_p3_create_only_packet_test as packet_fixture


class CreateOnlyTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        packet_fixture.PacketTest.setUpClass()
        cls.addClassCleanup(packet_fixture.PacketTest.doClassCleanups)

    def setUp(self):
        self.base = fixtures.PhaseTest('test_exact_terminal_receipt_and_no_replay')
        self.addCleanup(self.cleanup,self.base); self.base.setUp()
        self.suffix = self.base.context['operation_id'].rsplit('-',1)[1]
        self.parent = Path(self.base.temp.name)/'operations'; self.parent.mkdir(mode=0o700)
        (self.parent/self.suffix).mkdir(mode=0o700)
        intent = copy.deepcopy(self.base.journal.intent)
        intent['realtime_p3_create'] = {'issuer_source_sha':'b'*40,'source_repo':'/source','issuer_suffix':self.suffix}
        self.journal = journal.Journal(Path(self.base.temp.name)/'p3-journal',intent); self.addCleanup(self.journal.close)
        self.client = fixtures.Client(self.base.lock); self.client.journal = self.journal
        self.base.ledger.client = self.client
        self.client_type = patch.object(create.frontend_runtime_executor,'KubectlTransport',fixtures.Client)
        self.client_type.start(); self.addCleanup(self.client_type.stop)
        self.roots = {'voice-principal-ca':packet_fixture.PacketTest.principal,'voice-media-ca':packet_fixture.PacketTest.media,
                      'voice-space-media-server-tls':packet_fixture.PacketTest.space}
        self.created = {}; self.calls = []; self.lost = False; self.fault = None
        self.envelope = copy.deepcopy(packet_fixture.PacketTest.envelope)
        self.envelope['operation_id'] = self.base.context['operation_id']
        self.session = create.Session(self.journal,self.base.fence,self.client,self.base.lock,'ns')
        self.addCleanup(self.session.close)

    def cleanup(self,base):
        if not base.doCleanups(): raise RuntimeError('fixture cleanup failed')

    def public(self,name,key):
        version = '10' if self.fault == 'root-drift' and self.created else '9'
        return ('uid-'+name+'\n'+version+'\n'+base64.b64encode(self.roots[name]).decode()).encode()

    def get(self,kind,name): return copy.deepcopy(self.created.get((kind,name)))

    def create(self,obj,dry_run):
        self.calls.append(('dry-run' if dry_run else 'create',obj['metadata']['name']))
        if self.fault == 'dry-run' and dry_run and len(self.calls) == 3: raise OSError('private sentinel')
        result = copy.deepcopy(obj)
        if not dry_run:
            result['metadata'].update(uid='uid-'+obj['metadata']['name'],resourceVersion='10')
            self.created[obj['kind'],obj['metadata']['name']] = copy.deepcopy(result)
            if self.lost: raise OSError('private sentinel')
        return result

    def execute(self):
        fake_source = SimpleNamespace(verify=lambda *args:None)
        with patch.dict(sys.modules,{'realtime_p3_create_only_source':fake_source}), \
             patch.object(create,'SOCKET_PARENT',self.parent), patch.object(create.transport,'public_certificate',side_effect=self.public), \
             patch.object(create.transport,'get',side_effect=self.get), patch.object(create.transport,'create',side_effect=self.create), \
             patch.object(create.channel,'receive',return_value=self.envelope):
            ready = self.session.prepare(); self.assertEqual(ready['status'],'ready')
            return self.session.finish()

    def test_actual_crypto_dryrun_then_seven_create_and_exact_readback(self):
        receipt = self.execute()
        self.assertEqual(len(receipt['objects']),7); self.assertEqual(len(self.created),7)
        self.assertEqual([kind for kind,_ in self.calls],['dry-run']*7+['create']*7)
        self.assertEqual(self.base.ledger.row['realtime_p3_handoff']['status'],'done')
        self.assertNotIn('PRIVATE KEY',str(self.journal.state)); self.assertFalse((self.parent/self.suffix/'packet.sock').exists())
        self.assertNotIn('realtime-p3-handoff',self.base.journal.state['steps'])

    def test_lost_create_response_retains_effect_and_never_replays(self):
        self.lost = True
        with self.assertRaises(create.CreateError) as error: self.execute()
        self.assertNotIn('sentinel',str(error.exception)); self.assertEqual(len(self.created),1)
        self.assertEqual(self.base.ledger.row['status'],'uncertain')
        self.assertTrue(any(x['status']=='pending' for x in self.base.ledger.row['realtime_p3_objects'].values()))
        with self.assertRaises(create.CreateError): self.execute()
        self.assertEqual(len(self.created),1)

    def test_closed_journal_after_pending_refuses_create(self):
        original=self.session._reserve
        def close_after_pending(*args):
            original(*args); self.journal.close()
        with patch.object(self.session,'_reserve',side_effect=close_after_pending):
            with self.assertRaises(create.CreateError):self.execute()
        self.assertEqual(self.created,{}); self.assertFalse(any(kind=='create' for kind,_ in self.calls))
        self.assertEqual(self.base.ledger.row['status'],'uncertain')

    def test_existing_object_never_adopted(self):
        identity = sorted(packet.MEMBERS)[0]; self.created[identity] = {'foreign':True}
        with self.assertRaises(create.CreateError): self.execute()
        self.assertEqual(self.calls,[]); self.assertEqual(self.created[identity],{'foreign':True})

    def test_dryrun_failure_never_creates(self):
        self.fault = 'dry-run'
        with self.assertRaises(create.CreateError): self.execute()
        self.assertEqual(self.created,{})
        self.assertEqual(self.base.ledger.row['status'],'uncertain')

    def test_delegated_failure_never_falls_back_or_dry_runs(self):
        fake_source = SimpleNamespace(verify=lambda *args:None)
        fake_receiver = SimpleNamespace(receive=lambda *args:(_ for _ in ()).throw(ValueError('not ready')))
        with patch.dict(sys.modules,{'realtime_p3_create_only_source':fake_source,'realtime_p3_delegated_receiver':fake_receiver}), \
             patch.object(create,'SOCKET_PARENT',self.parent), patch.object(create.transport,'public_certificate',side_effect=self.public), \
             patch.object(create.transport,'get',side_effect=self.get), patch.object(create.transport,'create',side_effect=self.create), \
             patch.object(create.channel,'receive',side_effect=AssertionError('must not fall back')):
            self.session.prepare()
            with self.assertRaises(create.CreateError):self.session.finish_delegated(None,None)
        self.assertEqual(self.calls,[]); self.assertEqual(self.base.ledger.row['status'],'uncertain')

    def test_delegated_drain_loss_refuses_first_dry_run(self):
        fake_source = SimpleNamespace(verify=lambda *args:None)
        cut=SimpleNamespace(journal=self.base.journal,client=object(),fence=SimpleNamespace(ledger=self.base.ledger),
                            verify_drained=lambda:(_ for _ in ()).throw(ValueError('drain lost')))
        cut.fence.context=self.base.context; cut.lock=self.base.lock; cut.namespace_uid='ns'
        def received(session,*_):
            session._delegated_cut=cut; session._delegated_refs=(cut,cut.journal,cut.fence,cut.client,cut.fence.ledger)
            return self.envelope
        fake_receiver=SimpleNamespace(receive=received)
        with patch.dict(sys.modules,{'realtime_p3_create_only_source':fake_source,'realtime_p3_delegated_receiver':fake_receiver}), \
             patch.object(create,'SOCKET_PARENT',self.parent), patch.object(create.transport,'public_certificate',side_effect=self.public), \
             patch.object(create.transport,'get',side_effect=self.get), patch.object(create.transport,'create',side_effect=self.create):
            self.session.prepare()
            with self.assertRaises(create.CreateError):self.session.finish_delegated(object(),'token')
        self.assertEqual(self.calls,[]); self.assertEqual(self.base.ledger.row['status'],'uncertain')

    def test_delegated_terminal_cut_journal_drift_refuses_receipt(self):
        fake_source=SimpleNamespace(verify=lambda *args:None)
        armed=[]
        def drained():
            if self.base.ledger.row.get('step')=='realtime-p3-handoff-done':armed.append(True)
        cut=SimpleNamespace(journal=self.base.journal,client=object(),fence=SimpleNamespace(ledger=self.base.ledger),
                            verify_drained=drained,lock=self.base.lock,namespace_uid='ns')
        cut.fence.context=self.base.context
        def received(session,*_):
            session._delegated_cut=cut; session._delegated_refs=(cut,cut.journal,cut.fence,cut.client,cut.fence.ledger)
            return self.envelope
        fake_receiver=SimpleNamespace(receive=received); original=self.base.ledger.read; mutated=[]
        def drift():
            result=original()
            if armed and result[0].get('step')=='realtime-p3-handoff-done' and not mutated:
                cut.journal.state['steps']['late-drift']={'status':'changed'}; mutated.append(True)
            return result
        with patch.dict(sys.modules,{'realtime_p3_create_only_source':fake_source,'realtime_p3_delegated_receiver':fake_receiver}), \
             patch.object(create,'SOCKET_PARENT',self.parent),patch.object(create.transport,'public_certificate',side_effect=self.public), \
             patch.object(create.transport,'get',side_effect=self.get),patch.object(create.transport,'create',side_effect=self.create), \
             patch.object(self.base.ledger,'read',side_effect=drift):
            self.session.prepare()
            with self.assertRaises(create.CreateError):self.session.finish_delegated(object(),'token')
        self.assertTrue(mutated); self.assertEqual(len(self.created),7)
        self.assertFalse(self.session.completed); self.assertEqual(self.base.ledger.row['status'],'uncertain')

    def test_delegated_final_ledger_callback_cannot_change_cut_identity(self):
        armed=[]
        cut=SimpleNamespace(journal=self.base.journal,client=object(),fence=SimpleNamespace(ledger=self.base.ledger),
                            verify_drained=lambda:armed.append(True),lock=self.base.lock,namespace_uid='ns')
        cut.fence.context=self.base.context
        self.session._delegated_cut=cut; self.session._delegated_refs=(cut,cut.journal,cut.fence,cut.client,cut.fence.ledger)
        original=self.base.ledger.read
        def drift():
            result=original()
            if armed:cut.lock={'uid':'foreign'}
            return result
        with patch.object(self.base.ledger,'read',side_effect=drift):
            with self.assertRaises(ValueError):self.session._guard()

    def test_live_ca_drift_retains_created_resources_and_holds(self):
        self.fault = 'root-drift'
        with self.assertRaises(create.CreateError): self.execute()
        self.assertEqual(len(self.created),1)
        self.assertEqual(self.base.ledger.row['status'],'uncertain')

    def test_terminal_cas_object_drift_holds_without_receipt(self):
        write = self.base.ledger.write
        def mutate_after_terminal(rev, row):
            write(rev, row)
            if row.get('step') == 'realtime-p3-handoff-done':
                self.created[sorted(self.created)[0]]['metadata']['uid'] = 'foreign-uid'
        with patch.object(self.base.ledger, 'write', side_effect=mutate_after_terminal):
            with self.assertRaises(create.CreateError): self.execute()
        self.assertEqual(len(self.created), 7)
        self.assertEqual(self.base.ledger.row['status'], 'uncertain')


if __name__ == '__main__': unittest.main()
