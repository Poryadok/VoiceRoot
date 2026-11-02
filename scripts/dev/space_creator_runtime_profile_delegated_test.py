import base64
import copy
import hashlib
from pathlib import Path
import tempfile
import threading
from types import SimpleNamespace
import unittest
from unittest.mock import patch,Mock

import dev_peer_issuer_metadata as metadata
import dev_peer_issuer_packet as packet
import reset_credentials as reset
import space_creator_runtime_profile_delegated as subject
import full_cohort_package_phase as package
import media_operation_journal as journals
import resource_fencing
import writer_cut_test as cut_fixture


class CryptoTest(unittest.TestCase):
    def test_existing_space_pair_is_qualified_and_next_kid_preserved(self):
        with tempfile.TemporaryDirectory(prefix='peer-receiver-test-') as tmp:
            material=reset.ResetMaterial(Path(tmp)); key,cert=material.ca('fixture'); media=cert.read_bytes()
            material.signer('voice-space-principal-signing',active=True)
            material.tls('voice-space-media-server-tls',(key,cert),*packet.TLS['voice-space-media-server-tls'])
            objects={item['metadata']['name']:copy.deepcopy(item) for item in material.items}
            objects['voice-space-principal-signing']['data']['active-kid']=base64.b64encode(b'next').decode()
            identities={}
            for name,obj in objects.items():
                obj['metadata'].update(uid='uid-'+name,resourceVersion='1')
                identities[name]={'state':'present','uid':'uid-'+name,'resourceVersion':'1','keys':sorted(metadata.MEMBERS[name])}
            phase=subject.SpaceReceiver.__new__(subject.SpaceReceiver)
            phase.initial={'ca':{'der_sha256':hashlib.sha256(packet.crypto._openssl(['x509','-in','{0}','-outform','DER'],[media])).hexdigest()}}
            phase.preflight=SimpleNamespace(_creator=lambda:copy.deepcopy(identities))
            phase.fence=SimpleNamespace(context={'operation_id':'media-c1-'+'a'*32,'commit':'b'*40}); phase.namespace_uid='ns'
            phase.media=media; phase._current=lambda:None
            names=sorted(metadata.PROFILES['space'])
            with patch.object(subject,'_get',side_effect=lambda name,profile:copy.deepcopy(objects[name])):
                jwks=phase._crypto(names)
                self.assertEqual(len(jwks['voice-space-principal-signing']['keys']),2)
                self.assertEqual(base64.b64decode(objects['voice-space-principal-signing']['data']['active-kid']),b'next')
                objects['voice-space-media-server-tls']['data']['tls.key']=objects['voice-space-principal-signing']['data']['current.pem']
                with self.assertRaises(ValueError): phase._crypto(names)


class PhaseTest(unittest.TestCase):
    def cleanup(self,case):
        if not case.doCleanups(): raise RuntimeError('nested cleanup failed')

    def setUp(self):
        self.profile=getattr(self,'profile','space')
        self.base=cut_fixture.WriterCutTest('test_drain_and_resume_use_current_template_not_old')
        self.addCleanup(self.cleanup,self.base); self.base.setUp(); self.base.cut.drain()
        self.root=Path(self.base.temp.name); material=reset.ResetMaterial(self.root)
        key,cert=material.ca('fixture'); self.media=cert.read_bytes()
        for name in metadata.PROFILES[self.profile]:
            if name.endswith('-principal-signing'): material.signer(name,active=True)
            else: material.tls(name,(key,cert),*packet.TLS[name])
        self.desired={x['metadata']['name']:copy.deepcopy(x) for x in material.items}
        self.actual={}; self.identities={name:{'state':'absent'} for name in metadata.MEMBERS}
        self.ca={'uid':'ca','resourceVersion':'1','der_sha256':hashlib.sha256(packet.crypto._openssl(['x509','-in','{0}','-outform','DER'],[self.media])).hexdigest()}
        client_type=type(self.base.client); backing=self.base.ledger
        class Ledger:
            def __init__(self,client): self.client=client
            def read(self): return backing.read()
            def write(self,revision,row): return backing.write(revision,row)
        def attach(name,intent):
            journal=journals.Journal(self.root/name,intent); self.addCleanup(journal.close)
            client=client_type(self.base.lock,journal); client.workloads=self.base.client.workloads
            fence=resource_fencing.Fence(self.base.context,lambda:copy.deepcopy(self.base.card),Ledger(client))
            return journal,client,fence
        base_intent={'resource_context':copy.deepcopy(self.base.context),'namespace_uid':'ns'}
        pj,pc,pf=attach('package',base_intent); self.package=package.PackagePhase.__new__(package.PackagePhase)
        self.package.journal,self.package.client,self.package.fence=pj,pc,pf
        self.package.lock,self.package.namespace_uid=self.base.lock,'ns'; self.package.verify_current=Mock(return_value={})
        pre_intent=base_intent|{'dev_peer_issuer_preflight':{'source_root':str(self.root),'source_sha':self.base.context['commit'],'profile':self.profile}}
        j,c,f=attach('peer-preflight',pre_intent)
        self.preflight=subject.preflight.PeerIssuerPreflightPhase(j,f,c,self.base.lock,'ns',self.package,self.base.cut,str(self.root),'token',self.profile)
        self.preflight._ca=Mock(side_effect=lambda:copy.deepcopy(self.ca)); self.preflight._creator=Mock(side_effect=lambda:copy.deepcopy(self.identities))
        self.initial={'profile':self.profile,'ca':copy.deepcopy(self.ca),'creator_secrets':copy.deepcopy(self.identities)}
        self.preflight.record={'status':'done','ca':copy.deepcopy(self.ca),'creator_secrets':copy.deepcopy(self.identities),
                               'receipt_sha256':subject.writer_cut.digest(self.initial)}
        j.state['steps'][self.preflight.step]=copy.deepcopy(self.preflight.record)|{'receipt':copy.deepcopy(self.initial)}; j.save()
        backing.row[self.preflight.ledger]=copy.deepcopy(self.preflight.record)
        receiver_intent=base_intent|{'peer_receiver':{'profile':self.profile,'source_sha':self.base.context['commit']}}
        j,c,f=attach('receiver',receiver_intent)
        self.writes=[]
        patches=[patch.object(subject.preflight.source,'verify'),patch.object(subject.canonical,'ROOT',self.root),
                 patch.object(subject.preflight.public,'_value',return_value=self.media),
                 patch.object(subject,'_get',side_effect=lambda name,profile:copy.deepcopy(self.actual.get(name))),
                 patch.object(subject,'_create',side_effect=self.create)]
        for item in patches: item.start(); self.addCleanup(item.stop)
        receiver_type=subject.SpaceReceiver if self.profile=='space' else subject.CreatorReceiver
        self.phase=receiver_type(j,f,c,self.base.lock,'ns',self.package,self.base.cut,self.preflight,self.root,'token')

    def create(self,obj,profile,dry):
        self.writes.append((obj['metadata']['name'],dry)); result=copy.deepcopy(obj)
        if not dry:
            name=result['metadata']['name']; result['metadata'].update(uid='uid-'+name,resourceVersion='1')
            self.actual[name]=copy.deepcopy(result)
            self.identities[name]={'state':'present','uid':'uid-'+name,'resourceVersion':'1','keys':sorted(metadata.MEMBERS[name])}
        return result

    def test_unknown_issue_never_submits_kubernetes_write(self):
        with patch.object(self.phase,'_obtain',side_effect=OSError('lost issue response')):
            with self.assertRaises(subject.DelegatedError):self.phase.run()
            self.assertEqual(self.writes,[]); self.assertEqual(self.base.ledger.row['status'],'uncertain')
            with self.assertRaises(subject.DelegatedError):self.phase.run()

    def test_exact_existing_pair_is_read_only_and_remains_revalidated(self):
        for name,obj in self.desired.items():
            current=copy.deepcopy(obj); current['metadata'].update(uid='uid-'+name,resourceVersion='1'); self.actual[name]=current
            self.identities[name]={'state':'present','uid':'uid-'+name,'resourceVersion':'1','keys':sorted(metadata.MEMBERS[name])}
        self.initial['creator_secrets']=copy.deepcopy(self.identities)
        self.preflight.record['creator_secrets']=copy.deepcopy(self.identities)
        self.preflight.record['receipt_sha256']=subject.writer_cut.digest(self.initial)
        self.preflight.journal.state['steps'][self.preflight.step]=copy.deepcopy(self.preflight.record)|{'receipt':copy.deepcopy(self.initial)}
        self.base.ledger.row[self.preflight.ledger]=copy.deepcopy(self.preflight.record)
        with patch.object(self.phase,'_obtain',side_effect=AssertionError('no-op must not issue')):
            receipt=self.phase.run(); self.assertEqual(self.writes,[])
            self.assertEqual(subject.verify_completed(self.phase,'token'),receipt)
        self.identities['voice-space-principal-signing']['resourceVersion']='2'
        with self.assertRaises(subject.DelegatedError):subject.verify_completed(self.phase,'token')

    def test_predrain_existing_observation_has_no_effect_or_drain_dependency(self):
        name='voice-space-principal-signing'; current=copy.deepcopy(self.desired[name])
        current['metadata'].update(uid='existing',resourceVersion='3'); self.actual[name]=current
        self.identities[name]={'state':'present','uid':'existing','resourceVersion':'3','keys':sorted(metadata.MEMBERS[name])}
        self.initial['creator_secrets']=copy.deepcopy(self.identities)
        self.preflight.record['creator_secrets']=copy.deepcopy(self.identities)
        self.preflight.record['receipt_sha256']=subject.writer_cut.digest(self.initial)
        self.preflight.journal.state['steps'][self.preflight.step]=copy.deepcopy(self.preflight.record)|{'receipt':copy.deepcopy(self.initial)}
        self.base.ledger.row[self.preflight.ledger]=copy.deepcopy(self.preflight.record); before=self.base.ledger.read()
        with patch.object(self.base.cut,'verify_drained',side_effect=AssertionError('must not require drain')):
            result=subject.inspect_existing(self.phase,'token')
        self.assertEqual(result['missing'],['voice-space-media-server-tls']); self.assertEqual(set(result['present']),{name})
        self.assertEqual(self.writes,[]); self.assertEqual(self.base.ledger.read(),before)

    def test_package_record_changed_during_last_ledger_read_is_refused(self):
        self.package.journal.state['steps'][package.STEP]={'status':'done'}
        original=self.package.fence.ledger.read
        def drift():
            result=original(); self.package.journal.state['steps'].pop(package.STEP,None); return result
        with patch.object(self.package.fence.ledger,'read',side_effect=drift):
            with self.assertRaises(ValueError): self.phase._guard()

    def test_terminal_drain_drift_is_uncertain(self):
        for name,obj in self.desired.items():
            current=copy.deepcopy(obj); current['metadata'].update(uid='uid-'+name,resourceVersion='1'); self.actual[name]=current
            self.identities[name]={'state':'present','uid':'uid-'+name,'resourceVersion':'1','keys':sorted(metadata.MEMBERS[name])}
        self.initial['creator_secrets']=copy.deepcopy(self.identities)
        self.preflight.record['creator_secrets']=copy.deepcopy(self.identities)
        self.preflight.record['receipt_sha256']=subject.writer_cut.digest(self.initial)
        self.preflight.journal.state['steps'][self.preflight.step]=copy.deepcopy(self.preflight.record)|{'receipt':copy.deepcopy(self.initial)}
        self.base.ledger.row[self.preflight.ledger]=copy.deepcopy(self.preflight.record)
        original=self.base.cut.verify_drained
        def drift():
            result=original()
            if self.phase.record and self.phase.record.get('status')=='done':
                self.identities['voice-space-principal-signing']['resourceVersion']='2'
            return result
        with patch.object(self.base.cut,'verify_drained',side_effect=drift):
            with self.assertRaises(subject.DelegatedError): self.phase.run()
        self.assertEqual(self.base.ledger.row['status'],'uncertain'); self.assertFalse(self.phase.completed)

    def test_failed_handoff_waits_for_cancelled_worker(self):
        root=self.root/'operations'; root.mkdir(mode=0o700); (root/self.phase.suffix).mkdir(mode=0o700)
        cancelled=threading.Event(); release=threading.Event(); finished=threading.Event()
        class Handle:
            def run(self):cancelled.wait(5); release.wait(20); finished.set()
            def cancel(self):cancelled.set()
        outcome=[]
        def call():
            try:self.phase._obtain()
            except BaseException:outcome.append('hold')
        with patch.object(subject,'ROOT',root),patch.object(subject.issuer_client,'new_issue_trigger',return_value=Handle()), \
             patch.object(self.phase,'_socket'),patch.object(self.phase,'_drained'), \
             patch.object(self.phase,'_current',side_effect=[None,ValueError('lost')]):
            caller=threading.Thread(target=call); caller.start()
            try:
                self.assertTrue(cancelled.wait(5)); caller.join(5.5); self.assertTrue(caller.is_alive()); self.assertEqual(outcome,[])
            finally:release.set(); caller.join(5); self.phase.close()
        self.assertFalse(caller.is_alive()); self.assertTrue(finished.is_set()); self.assertEqual(outcome,['hold'])


class CreatorSpaceInspectionTest(unittest.TestCase):
    def test_creator_preflight_qualifies_existing_space_without_effects(self):
        case=PhaseTest('test_unknown_issue_never_submits_kubernetes_write'); case.profile='creator'
        self.addCleanup(lambda:self.assertTrue(case.doCleanups())); case.setUp()
        material=reset.ResetMaterial(case.root); material.signer('voice-space-principal-signing',active=True)
        obj=copy.deepcopy(material.items[-1]); name=obj['metadata']['name']
        obj['metadata'].update(uid='space-signing',resourceVersion='3'); case.actual[name]=obj
        case.identities[name]={'state':'present','uid':'space-signing','resourceVersion':'3','keys':sorted(metadata.MEMBERS[name])}
        case.initial['creator_secrets']=copy.deepcopy(case.identities)
        case.preflight.record['creator_secrets']=copy.deepcopy(case.identities)
        case.preflight.record['receipt_sha256']=subject.writer_cut.digest(case.initial)
        case.preflight.journal.state['steps'][case.preflight.step]=copy.deepcopy(case.preflight.record)|{'receipt':copy.deepcopy(case.initial)}
        case.base.ledger.row[case.preflight.ledger]=copy.deepcopy(case.preflight.record); before=case.base.ledger.read()
        with patch.object(case.base.cut,'verify_drained',side_effect=AssertionError('no drain')):
            result=subject.inspect_space_from_creator(case.phase,'token')
        self.assertEqual(result['profile'],'space'); self.assertEqual(set(result['present']),{name})
        self.assertEqual(result['missing'],['voice-space-media-server-tls'])
        self.assertEqual(case.base.ledger.read(),before); self.assertEqual(case.writes,[])
        with self.assertRaises(subject.DelegatedError):subject.inspect_space_from_creator(case.phase,'wrong-token')
        crypto=case.phase._crypto
        def drift(*args,**kwargs):
            result=crypto(*args,**kwargs); case.identities[name]['resourceVersion']='4'; return result
        with patch.object(case.phase,'_crypto',side_effect=drift):
            with self.assertRaises(subject.DelegatedError):subject.inspect_space_from_creator(case.phase,'token')
        self.assertEqual(case.writes,[])


if __name__=='__main__': unittest.main()
