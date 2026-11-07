import copy
import array
import fcntl
import hashlib
import os
from pathlib import Path
import socket
import tempfile
import threading
from types import SimpleNamespace
import unittest
from unittest import mock

import space_role_peer_create as create
import canonical_application_package as canonical
import frontend_runtime_executor as transport
import full_cohort_package_phase as package
import media_backup_phase_test as fixture
import media_operation_guard as guard
import media_operation_journal as journals
import resource_fencing


def desired():
    return {'apiVersion':'v1','kind':'Secret','metadata':{'name':'voice-role-media-client-tls','namespace':'voice-dev'},
            'type':'Opaque','data':{'tls.crt':'YQ==','tls.key':'Yg=='}}


class RoleCreateTest(unittest.TestCase):
    @unittest.skipUnless(os.name=='posix' and os.geteuid()==0 and hasattr(os,'memfd_create'),'requires disposable Linux root fixture')
    def test_separate_signer_ack_only_after_packet_validation(self):
        with tempfile.TemporaryDirectory() as directory:
            path=str(Path(directory)/'role.sock'); listener=socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET)
            listener.bind(path); listener.listen(1); self.addCleanup(listener.close)
            phase=SimpleNamespace(fence=SimpleNamespace(context={'operation_id':'op','commit':'sha'}),namespace_uid='ns',
                                  initial={},missing=[],media=b'ca',_check_current=mock.Mock(),_distinct_keys=mock.Mock())
            for valid in (True,False):
                pid=os.fork()
                if pid==0:
                    try:
                        fd=os.memfd_create('fixture',os.MFD_ALLOW_SEALING); os.fchmod(fd,0o600); os.write(fd,b'{"safe":true}')
                        fcntl.fcntl(fd,fcntl.F_ADD_SEALS,create.channel.REQUIRED_SEALS)
                        with socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET) as peer:
                            peer.settimeout(10); peer.connect(path)
                            peer.sendmsg([create.FRAME],[(socket.SOL_SOCKET,socket.SCM_RIGHTS,array.array('i',[fd]))])
                            answer=peer.recv(64)
                        os.close(fd); os._exit(0 if answer==(create.ACK if valid else b'') else 1)
                    except BaseException: os._exit(2)
                try:
                    with mock.patch.object(create.packet,'validate',side_effect=(lambda *a:{} if valid else (_ for _ in ()).throw(ValueError()))):
                        if valid: self.assertEqual(create._receive(listener,phase),{'safe':True})
                        else:
                            with self.assertRaises(ValueError): create._receive(listener,phase)
                finally:
                    _,status=os.waitpid(pid,0); self.assertEqual(os.waitstatus_to_exitcode(status),0)

    def test_real_journal_and_fence_refuse_role_effect_drift(self):
        with tempfile.TemporaryDirectory() as temp:
            context={'project':'voice','resource':'DEV','actor':'scheduled_devops',
                     'claim':'11111111-1111-4111-8111-111111111111','window':'22222222-2222-4222-8222-222222222222',
                     'revision':4,'generation':3,'operation_id':'media-c1-'+'a'*32,'commit':'b'*40,
                     'images':{'voice-space/space':'ghcr.io/x@sha256:'+'c'*64}}
            journal=journals.Journal(Path(temp)/'journal',{'resource_context':context,'namespace_uid':'ns'}); self.addCleanup(journal.close)
            backing=fixture.Ledger(context); client=transport.KubectlTransport(journal); ledger=resource_fencing.KubeLedger(client)
            ledger.read=backing.read; ledger.write=backing.write; fence=resource_fencing.Fence(context,lambda:{},ledger)
            pkg=package.PackagePhase.__new__(package.PackagePhase); pre=create.preflight.PeerIssuerPreflightPhase.__new__(create.preflight.PeerIssuerPreflightPhase)
            pkg.journal=SimpleNamespace(state={'steps':{package.STEP:{'status':'done','receipt':{'source':'synthetic'}}}})
            lock={'metadata':{'uid':'lock'}}
            for obj in (pkg,pre):obj.fence=fence; obj.lock=lock; obj.namespace_uid='ns'
            pre.package=pkg; pre.profile='role'; pre._local=mock.Mock(); pre._authority=mock.Mock()
            phase=create.Session(journal,fence,client,lock,'ns',pkg,pre,canonical.ROOT,'token')
            with mock.patch.object(guard,'verify'),mock.patch.object(create.source,'verify'):
                phase.missing=['voice-role-media-client-tls']
                marker={'status':'pending','missing':phase.missing,'issuer_source_sha':create.source.REVIEWED_SOURCE}
                phase._write(record=marker); phase._write(effects={'voice-role-media-client-tls':{'status':'pending'}})
                reads=[]
                def drop_package_on_last_read():
                    reads.append(True); result=backing.read()
                    if len(reads)==2:pkg.journal.state['steps'].pop(package.STEP)
                    return result
                with mock.patch.object(ledger,'read',side_effect=drop_package_on_last_read):
                    with self.assertRaises(create.schema.GuardLost):phase._guard()
                pkg.journal.state['steps'][package.STEP]={'status':'done','receipt':{'source':'synthetic'}}
                backing.row[create.EFFECTS]={}
                with self.assertRaises(create.schema.GuardLost): phase._guard()

    def phase(self):
        phase=create.Session.__new__(create.Session)
        phase.exchanged=True; phase.listener=object(); phase.record={'status':'ready'}
        phase.pending=True; phase.completed=False; phase.initial={}; phase.media=b'ca'
        phase.missing=['voice-role-media-client-tls']; phase.effects={}
        phase.fence=SimpleNamespace(context={'operation_id':'op','commit':'sha'}); phase.namespace_uid='ns'
        phase._socket=mock.Mock(); phase._check_current=mock.Mock(); phase._drained=mock.Mock()
        phase._await_issue=mock.Mock()
        phase._uncertain=mock.Mock(); phase.close=mock.Mock()
        return phase

    def test_drain_loss_prevents_any_create(self):
        phase=self.phase(); phase._drained.side_effect=create.schema.GuardLost('drain lost')
        with mock.patch.object(create,'_receive',side_effect=lambda _,p:(setattr(p,'objects',{p.missing[0]:desired()}) or {})), \
             mock.patch.object(create,'_create') as write:
            with self.assertRaises(create.schema.GuardLost): phase.finish()
            write.assert_not_called(); phase._uncertain.assert_called_once()

    def test_missing_signer_acknowledgement_prevents_any_create(self):
        phase=self.phase(); phase._await_issue.side_effect=ValueError()
        with mock.patch.object(create,'_receive',side_effect=lambda _,p:(setattr(p,'objects',{p.missing[0]:desired()}) or {})), \
             mock.patch.object(create,'_create') as write:
            with self.assertRaises(create.CreateError): phase.finish()
            write.assert_not_called(); phase._uncertain.assert_called_once()

    def test_guard_loss_cancels_trigger_before_delayed_worker_can_connect(self):
        import dev_peer_issuer_client as client
        phase=self.phase(); phase.issue_thread=phase.issue_done=phase.issue_result=phase.issue_control=None
        phase.preflight=create.preflight.PeerIssuerPreflightPhase.__new__(create.preflight.PeerIssuerPreflightPhase)
        phase.token='token'; entered=threading.Event(); release=threading.Event()
        def stalled(*_):
            entered.set()
            if not release.wait(10): raise RuntimeError('fixture timeout')
        def lost():
            if not entered.wait(5): raise RuntimeError('fixture did not start')
            raise create.schema.GuardLost('lost')
        phase.serve_preflight=lost
        try:
            with mock.patch.object(client.preflight,'verify_current',side_effect=stalled),mock.patch.object(client.socket,'socket') as connect:
                with self.assertRaises(create.schema.GuardLost):phase.run()
                release.set(); phase.issue_thread.join(5)
                self.assertFalse(phase.issue_thread.is_alive()); connect.assert_not_called()
        finally:release.set()

    def test_trigger_setup_failure_is_uncertain_and_redacted(self):
        import dev_peer_issuer_client as client
        phase=self.phase(); phase.issue_thread=phase.issue_done=phase.issue_result=phase.issue_control=None
        phase.preflight=object(); phase.token='token'
        with mock.patch.object(client,'new_issue_trigger',side_effect=ValueError('PRIVATE-SENTINEL')):
            with self.assertRaises(create.CreateError) as failure:phase.run()
        self.assertNotIn('PRIVATE-SENTINEL',str(failure.exception))
        phase._uncertain.assert_called_once(); phase.close.assert_called_once()

    def test_lost_create_response_keeps_pending_without_secret_output(self):
        phase=self.phase(); obj=desired()
        def save(record=None,effects=None):
            if record is not None: phase.record=copy.deepcopy(record)
            if effects is not None: phase.effects=copy.deepcopy(effects)
        phase._write=save
        with mock.patch.object(create,'_receive',side_effect=lambda _,p:(setattr(p,'objects',{p.missing[0]:obj}) or {})), \
             mock.patch.object(create,'_get',return_value=None), \
             mock.patch.object(create,'_create',side_effect=[obj,RuntimeError('PRIVATE-SENTINEL')]):
            with self.assertRaises(create.CreateError) as failure: phase.finish()
        self.assertNotIn('PRIVATE-SENTINEL',str(failure.exception))
        self.assertEqual(phase.effects[phase.missing[0]]['status'],'pending')
        phase._uncertain.assert_called_once()

    def test_metadata_injection_and_wrong_name_rejected(self):
        obj=desired(); actual=copy.deepcopy(obj); actual['metadata'].update(uid='uid',resourceVersion='1')
        self.assertTrue(create._closed(actual,obj))
        actual['metadata']['ownerReferences']=[{'uid':'other'}]
        self.assertFalse(create._closed(actual,obj))
        with self.assertRaises(ValueError): create._get('voice-gateway-media-client-tls')

    def test_preserved_material_never_calls_signer_or_secret_create(self):
        phase=create.Session.__new__(create.Session); phase.pending=False; phase.completed=False
        phase.fence=SimpleNamespace(context={'operation_id':'op','commit':'sha'}); phase.namespace_uid='ns'; phase.token='token'
        phase.preflight=SimpleNamespace(); phase.source_root='/source'; phase.effects={}; phase.journal=SimpleNamespace(state={'steps':{}})
        initial={'profile':'role','ca':{'der_sha256':hashlib.sha256(b'der').hexdigest()},
                 'creator_secrets':{name:{'state':'present','uid':'uid','resourceVersion':'1'} for name in create.NAMES}}
        objects={name:desired() for name in create.NAMES}
        for name,obj in objects.items():obj['metadata'].update(name=name,uid='uid',resourceVersion='1')
        phase._guard=mock.Mock(return_value=({},1)); phase._check_current=mock.Mock(); phase._distinct_keys=mock.Mock()
        phase._local=mock.Mock(); phase._uncertain=mock.Mock(); phase.close=mock.Mock()
        def save(record=None,effects=None):
            if record is not None: phase.record=copy.deepcopy(record); phase.journal.state['steps'][create.STEP]=copy.deepcopy(record)
        phase._write=save
        with mock.patch.object(create.preflight,'verify_current',return_value=initial), mock.patch.object(create.source,'verify'), \
             mock.patch.object(create.preflight.public,'_value',return_value=b'ca'), mock.patch.object(create.preflight.crypto,'der',return_value=b'der'), \
             mock.patch.object(create,'_get',side_effect=lambda name:objects[name]), mock.patch.object(create,'_values'), \
             mock.patch.object(create.role_material,'qualify',side_effect=lambda name,*a: {'keys':[]} if name.endswith('signing') else None), \
             mock.patch.object(create,'_create') as write:
            result=phase.prepare()
            phase._guard=mock.Mock(return_value=({create.LEDGER:phase.record},1))
            self.assertEqual(create.verify_completed(phase,'token'),phase.proof)
        self.assertEqual(result['status'],'done'); self.assertTrue(phase.completed); write.assert_not_called()

    def test_inspection_is_read_only_and_rejects_snapshot_drift(self):
        phase=create.Session.__new__(create.Session); phase.token='token'; phase.record=None; phase.pending=False
        phase.preflight=SimpleNamespace(); phase.fence=SimpleNamespace(context={'operation_id':'op','commit':'sha'}); phase.namespace_uid='ns'
        phase.package=SimpleNamespace(journal=SimpleNamespace(state={'steps':{package.STEP:{'status':'done','receipt':{'safe':True}}}}))
        phase._guard=mock.Mock(return_value=({'status':'active'},('uid','rv'))); phase._local=mock.Mock(); tls='voice-role-media-client-tls'; signer='voice-role-principal-signing'
        initial={'profile':'role','ca':{'der_sha256':hashlib.sha256(b'der').hexdigest()},
                 'creator_secrets':{tls:{'state':'present','uid':'uid','resourceVersion':'1','keys':['tls.crt','tls.key']},signer:{'state':'absent'}}}
        obj=desired(); obj['metadata'].update(uid='uid',resourceVersion='1')
        with mock.patch.object(create.preflight,'verify_current',side_effect=[initial,initial]), \
             mock.patch.object(create.preflight.public,'_value',return_value=b'ca'), mock.patch.object(create.preflight.crypto,'der',return_value=b'der'), \
             mock.patch.object(create,'_get',return_value=obj), mock.patch.object(create,'_values',return_value={'tls.key':b'key'}), \
             mock.patch.object(create.role_material,'qualify',return_value=None), mock.patch.object(create.role_material.crypto,'_openssl',return_value=b'pub'), \
             mock.patch.object(create,'_create') as write:
            result=create.inspect_existing(phase,'token')
            self.assertEqual(result['schema'],'voice-dev-peer-existing-v1'); self.assertEqual(result['missing'],[signer]); self.assertEqual(set(result['present']),{tls}); write.assert_not_called()
        changed=copy.deepcopy(initial); changed['creator_secrets'][signer]={'state':'present','uid':'other','resourceVersion':'2'}
        with mock.patch.object(create.preflight,'verify_current',side_effect=[initial,changed]), \
             mock.patch.object(create.preflight.public,'_value',return_value=b'ca'), mock.patch.object(create.preflight.crypto,'der',return_value=b'der'), \
             mock.patch.object(create,'_get',return_value=obj), mock.patch.object(create,'_values',return_value={'tls.key':b'key'}), \
             mock.patch.object(create.role_material,'qualify',return_value=None), mock.patch.object(create.role_material.crypto,'_openssl',return_value=b'pub'):
            with self.assertRaises(create.CreateError):create.inspect_existing(phase,'token')
        def mutate_on_final_guard():
            if phase._guard.call_count>=2:phase.record={'status':'unexpected'}
            return {'status':'active'},('uid','rv')
        phase._guard=mock.Mock(side_effect=mutate_on_final_guard)
        with mock.patch.object(create.preflight,'verify_current',side_effect=[initial,initial]), \
             mock.patch.object(create.preflight.public,'_value',return_value=b'ca'), mock.patch.object(create.preflight.crypto,'der',return_value=b'der'), \
             mock.patch.object(create,'_get',return_value=obj), mock.patch.object(create,'_values',return_value={'tls.key':b'key'}), \
             mock.patch.object(create.role_material,'qualify',return_value=None), mock.patch.object(create.role_material.crypto,'_openssl',return_value=b'pub'):
            with self.assertRaises(create.CreateError):create.inspect_existing(phase,'token')


if __name__ == '__main__':
    unittest.main()
