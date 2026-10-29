import array
import base64
import copy
import fcntl
import json
import os
from pathlib import Path
import socket
import tempfile
import threading
from types import SimpleNamespace
import unittest
from unittest import mock

import canonical_application_package as canonical
import frontend_runtime_executor as transport
import full_cohort_package_phase as package
import media_backup_phase_test as fixture
import media_operation_guard as guard
import media_operation_journal as journals
import resource_fencing
import space_creator_runtime_profile_create as create
import space_creator_runtime_profile_issuer_packet_test as crypto_fixture


def desired():
    return {'apiVersion':'v1','kind':'Secret','metadata':{'name':'voice-auth-media-server-tls','namespace':'voice-dev'},
            'type':'Opaque','data':{'tls.crt':'YQ==','tls.key':'Yg=='}}


class CreateTest(unittest.TestCase):
    def test_guarded_real_journal_ledger_effect_marker_rejects_drift(self):
        with tempfile.TemporaryDirectory() as temp:
            context={'project':'voice','resource':'DEV','actor':'scheduled_devops',
              'claim':'11111111-1111-4111-8111-111111111111','window':'22222222-2222-4222-8222-222222222222',
              'revision':4,'generation':3,'operation_id':'media-c1-'+'a'*32,'commit':'b'*40,
              'images':{'voice-space/space':'ghcr.io/x@sha256:'+'c'*64}}
            journal=journals.Journal(Path(temp)/'journal',{'resource_context':context,'namespace_uid':'ns'}); self.addCleanup(journal.close)
            backing=fixture.Ledger(context); client=transport.KubectlTransport(journal); ledger=resource_fencing.KubeLedger(client)
            ledger.read=backing.read; ledger.write=backing.write; fence=resource_fencing.Fence(context,lambda:{},ledger)
            pkg=package.PackagePhase.__new__(package.PackagePhase); pre=create.preflight.IssuerPreflightPhase.__new__(create.preflight.IssuerPreflightPhase)
            lock={'metadata':{'uid':'lock'}}
            for obj in (pkg,pre):obj.fence=fence; obj.lock=lock; obj.namespace_uid='ns'
            pre.package=pkg; pre._local=mock.Mock(); pre._authority=mock.Mock()
            phase=create.Session(journal,fence,client,lock,'ns',pkg,pre,canonical.ROOT,'token')
            with mock.patch.object(guard,'verify'),mock.patch.object(create.source,'verify'):
                phase.missing=['voice-auth-media-server-tls']
                marker={'status':'pending','missing':phase.missing,'issuer_source_sha':create.source.REVIEWED_SOURCE}; phase._write(record=marker)
                phase._write(effects={'voice-auth-media-server-tls':{'status':'pending'}})
                self.assertEqual(backing.row[create.LEDGER],marker)
                self.assertEqual(backing.row[create.EFFECTS]['voice-auth-media-server-tls']['status'],'pending')
                phase.token='other'
                with self.assertRaises(create.schema.GuardLost):phase._guard()
                phase.token='token'; phase.missing=[]
                with self.assertRaises(create.schema.GuardLost):phase._guard()
                phase.missing=['voice-auth-media-server-tls']
                backing.row[create.EFFECTS]={}
                with self.assertRaises(create.schema.GuardLost):phase._guard()

    def test_real_crypto_rejects_cross_family_key_reuse(self):
        fixture_class=crypto_fixture.PacketTest; fixture_class.setUpClass(); self.addCleanup(fixture_class.tearDownClass)
        phase=create.Session.__new__(create.Session); phase.objects={obj['metadata']['name']:obj for obj in fixture_class.objects}
        phase._distinct_keys()
        phase.objects=copy.deepcopy(phase.objects)
        phase.objects['voice-space-media-client-tls']['data']['tls.key']=phase.objects['voice-gateway-media-client-tls']['data']['tls.key']
        with self.assertRaises(ValueError):phase._distinct_keys()
        phase.objects={obj['metadata']['name']:copy.deepcopy(obj) for obj in fixture_class.objects}
        data=phase.objects['voice-space-media-client-tls']['data']
        data['tls.crt']=base64.b64encode(base64.b64decode(data['tls.crt'])+b'PRIVATE-SENTINEL').decode()
        with self.assertRaises(ValueError):phase._distinct_keys()

    def test_server_metadata_injection_rejected(self):
        base=desired(); actual=copy.deepcopy(base); actual['metadata'].update(uid='uid',resourceVersion='1')
        self.assertTrue(create._closed(actual,base))
        for key,value in (('ownerReferences',[{'uid':'unexpected'}]),('finalizers',['unexpected']),('labels',{'other':'x'})):
            bad=copy.deepcopy(actual); bad['metadata'][key]=value
            with self.subTest(key=key):self.assertFalse(create._closed(bad,base))

    @unittest.skipUnless(os.name=='posix','Linux Unix-socket fixture')
    def test_socket_mode_and_directory_identity_are_bound(self):
        with tempfile.TemporaryDirectory() as parent:
            root=Path(parent); suffix='a'*32; directory=root/suffix; directory.mkdir(mode=0o700)
            phase=create.Session.__new__(create.Session); phase.suffix=suffix; phase.dirfd=os.open(directory,os.O_RDONLY|os.O_DIRECTORY)
            self.addCleanup(os.close,phase.dirfd); phase.listener=socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET); self.addCleanup(phase.listener.close)
            phase.listener.bind(str(directory/'packet.sock')); os.chmod(directory/'packet.sock',0o600)
            info=os.stat(directory/'packet.sock'); phase.inode=(info.st_dev,info.st_ino)
            phase.record={'pid':os.getpid(),'start_ticks':create.common._ticks()}
            with mock.patch.object(create,'ROOT',root):
                phase._socket(); os.chmod(directory/'packet.sock',0o644)
                with self.assertRaises(ValueError):phase._socket()

    def test_bad_dry_run_stops_before_any_create(self):
        phase=create.Session.__new__(create.Session); phase.exchanged=True; phase.listener=object(); phase.record={'status':'ready'}
        phase.pending=True; phase.completed=False; phase.initial={}; phase.media=b'ca'; phase.missing=['voice-auth-media-server-tls']
        phase.fence=SimpleNamespace(context={'operation_id':'op','commit':'sha'}); phase.namespace_uid='ns'; phase.effects={}
        phase._socket=mock.Mock(); phase._check_current=mock.Mock(); phase._drained=mock.Mock(); phase._distinct_keys=mock.Mock(); phase._uncertain=mock.Mock(); phase.close=mock.Mock()
        obj=desired(); bad=copy.deepcopy(obj); bad['metadata']['ownerReferences']=[{'uid':'unexpected'}]
        with mock.patch.object(create,'_receive',side_effect=lambda _,p:(setattr(p,'objects',{p.missing[0]:obj}) or {})), \
             mock.patch.object(create,'_get',return_value=None), mock.patch.object(create,'_create',return_value=bad) as call:
            with self.assertRaises(create.CreateError):phase.finish()
            self.assertEqual(call.call_count,1); self.assertTrue(call.call_args.args[1]); phase._uncertain.assert_called_once()

    def test_guard_loss_is_not_downgraded(self):
        phase=create.Session.__new__(create.Session); phase.exchanged=True; phase.listener=object(); phase.record={'status':'ready'}
        phase.pending=True; phase.completed=False; phase._socket=mock.Mock(); phase._check_current=mock.Mock(side_effect=create.schema.GuardLost('lost'))
        phase._uncertain=mock.Mock(); phase.close=mock.Mock()
        with self.assertRaises(create.schema.GuardLost):phase.finish()

    def test_writer_drain_loss_stops_before_secret_effect(self):
        phase=create.Session.__new__(create.Session); phase.exchanged=True; phase.listener=object(); phase.record={'status':'ready'}
        phase.pending=True; phase.completed=False; phase.initial={}; phase.media=b'ca'; phase.missing=['voice-auth-media-server-tls']
        phase.fence=SimpleNamespace(context={'operation_id':'op','commit':'sha'}); phase.namespace_uid='ns'; phase.effects={}
        phase._socket=mock.Mock(); phase._check_current=mock.Mock(); phase._distinct_keys=mock.Mock(); phase._uncertain=mock.Mock(); phase.close=mock.Mock()
        phase._drained=mock.Mock(side_effect=create.schema.GuardLost('drain lost')); obj=desired()
        with mock.patch.object(create,'_receive',side_effect=lambda _,p:(setattr(p,'objects',{p.missing[0]:obj}) or {})), \
             mock.patch.object(create,'_create') as write:
            with self.assertRaises(create.schema.GuardLost):phase.finish()
            write.assert_not_called(); phase._uncertain.assert_called_once()

    def test_done_requires_owned_uid_and_exact_readback(self):
        phase=create.Session.__new__(create.Session); phase.exchanged=True; phase.listener=object(); phase.record={'status':'ready'}
        phase.pending=True; phase.completed=False; phase.initial={}; phase.media=b'ca'; phase.missing=['voice-auth-media-server-tls']
        phase.fence=SimpleNamespace(context={'operation_id':'op','commit':'sha'}); phase.namespace_uid='ns'; phase.effects={}
        phase._socket=mock.Mock(); phase._check_current=mock.Mock(); phase._drained=mock.Mock(); phase._distinct_keys=mock.Mock(); phase._uncertain=mock.Mock(); phase.close=mock.Mock()
        phase._local=mock.Mock(); phase.journal=SimpleNamespace(state={'steps':{}})
        obj=desired(); actual=copy.deepcopy(obj); actual['metadata'].update(uid='uid',resourceVersion='1')
        def save(record=None,effects=None):
            if record is not None: phase.record=copy.deepcopy(record); phase.journal.state['steps'][create.STEP]=copy.deepcopy(record)
            if effects is not None: phase.effects=copy.deepcopy(effects)
        phase._write=save
        with mock.patch.object(create,'_receive',side_effect=lambda _,p:(setattr(p,'objects',{p.missing[0]:obj}) or {})), \
             mock.patch.object(create,'_get',side_effect=[None,None,actual]), \
             mock.patch.object(create,'_create',side_effect=[obj,actual]):
            receipt=phase.finish()
        self.assertTrue(phase.completed); self.assertEqual(receipt['objects'][phase.missing[0]]['uid'],'uid')
        self.assertNotIn('Yg==',json.dumps(receipt))

    def test_lost_create_response_keeps_pending_and_never_logs_payload(self):
        phase=create.Session.__new__(create.Session); phase.exchanged=True; phase.listener=object(); phase.record={'status':'ready'}
        phase.pending=True; phase.completed=False; phase.initial={}; phase.media=b'ca'; phase.missing=['voice-auth-media-server-tls']
        phase.fence=SimpleNamespace(context={'operation_id':'op','commit':'sha'}); phase.namespace_uid='ns'; phase.effects={}
        phase._socket=mock.Mock(); phase._check_current=mock.Mock(); phase._drained=mock.Mock(); phase._distinct_keys=mock.Mock(); phase._uncertain=mock.Mock(); phase.close=mock.Mock()
        def save(record=None,effects=None):
            if record is not None:phase.record=copy.deepcopy(record)
            if effects is not None:phase.effects=copy.deepcopy(effects)
        phase._write=save; obj=desired()
        with mock.patch.object(create,'_receive',side_effect=lambda _,p:(setattr(p,'objects',{p.missing[0]:obj}) or {})), \
             mock.patch.object(create,'_get',return_value=None), mock.patch.object(create,'_create',side_effect=[obj,RuntimeError('PRIVATE-SENTINEL')]):
            with self.assertRaises(create.CreateError) as failure:phase.finish()
        self.assertNotIn('PRIVATE-SENTINEL',str(failure.exception))
        self.assertEqual(phase.effects[phase.missing[0]]['status'],'pending'); phase._uncertain.assert_called_once()

    def test_finish_rejects_proof_changed_during_last_guard(self):
        phase=create.Session.__new__(create.Session); phase.exchanged=True; phase.listener=object(); phase.record={'status':'ready'}
        phase.pending=True; phase.completed=False; phase.initial={}; phase.media=b'ca'; phase.missing=['voice-auth-media-server-tls']
        phase.fence=SimpleNamespace(context={'operation_id':'op','commit':'sha'}); phase.namespace_uid='ns'; phase.effects={}
        phase.journal=SimpleNamespace(state={'steps':{}}); phase._socket=mock.Mock(); phase._drained=mock.Mock(); phase._local=mock.Mock()
        phase._distinct_keys=mock.Mock(); phase._uncertain=mock.Mock(); phase.close=mock.Mock()
        def check():
            if phase.record.get('status')=='done':phase.proof={'scope':'tampered'}
        phase._check_current=check
        def save(record=None,effects=None):
            if record is not None:phase.record=copy.deepcopy(record); phase.journal.state['steps'][create.STEP]=copy.deepcopy(record)
            if effects is not None:phase.effects=copy.deepcopy(effects)
        phase._write=save; obj=desired(); actual=copy.deepcopy(obj); actual['metadata'].update(uid='uid',resourceVersion='1')
        with mock.patch.object(create,'_receive',side_effect=lambda _,p:(setattr(p,'objects',{p.missing[0]:obj}) or {})), \
             mock.patch.object(create,'_get',side_effect=[None,None,actual]), mock.patch.object(create,'_create',side_effect=[obj,actual]):
            with self.assertRaises(create.schema.GuardLost):phase.finish()
        self.assertFalse(phase.completed); phase._uncertain.assert_called_once()

    def test_transient_preflight_observer_failure_is_guard_lost(self):
        phase=create.Session.__new__(create.Session); phase._guard=mock.Mock()
        phase.initial={'ca':{'uid':'ca'},'creator_secrets':{}}; phase.effects={}; phase.objects=None
        phase.preflight=SimpleNamespace(_ca=mock.Mock(side_effect=[ValueError('private'),{'uid':'ca'}]),_creator=mock.Mock(return_value={}))
        with self.assertRaises(create.schema.GuardLost):phase._check_current()
        phase._check_current()

    def test_terminal_callback_cannot_replace_receipt(self):
        phase=create.Session.__new__(create.Session); phase.token='token'; phase.completed=True; phase.pending=True
        phase.proof={'scope':'original'}; phase.record={'status':'done','receipt':copy.deepcopy(phase.proof)}
        phase.initial={}; phase.envelope={}; phase.objects={}; phase.missing=[]; phase.media=b'ca'; phase.namespace_uid='ns'; phase.effects={}
        phase.fence=SimpleNamespace(context={'operation_id':'op','commit':'sha'}); phase.journal=SimpleNamespace(state={'steps':{create.STEP:phase.record}})
        phase._check_current=mock.Mock(); phase._distinct_keys=mock.Mock(); phase._local=mock.Mock(); phase._uncertain=mock.Mock()
        row={create.LEDGER:phase.record,create.EFFECTS:{}}
        def last_guard():phase.proof={'scope':'replaced'}; return row,('uid','rv')
        phase._guard=last_guard
        with mock.patch.object(create.packet,'validate',return_value={}):
            with self.assertRaises(create.CreateError):create.verify_completed(phase,'token')

    @unittest.skipUnless(os.name=='posix' and os.geteuid()==0 and hasattr(os,'memfd_create'),'requires disposable Linux root fixture')
    def test_sealed_fd_and_multi_fd_cleanup(self):
        with tempfile.TemporaryDirectory() as directory:
            path=str(Path(directory)/'packet.sock'); listener=socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET)
            listener.bind(path); listener.listen(1); self.addCleanup(listener.close)
            phase=SimpleNamespace(fence=SimpleNamespace(context={'operation_id':'op','commit':'sha'}),namespace_uid='ns',initial={},missing=[],media=b'ca',
                                  _check_current=mock.Mock(),_distinct_keys=mock.Mock()); replies=[]
            def send(count):
                fds=[]
                try:
                    for _ in range(count):
                        fd=os.memfd_create('fixture',os.MFD_ALLOW_SEALING); fds.append(fd); os.fchmod(fd,0o600)
                        os.write(fd,json.dumps({'safe':True}).encode()); fcntl.fcntl(fd,fcntl.F_ADD_SEALS,create.channel.REQUIRED_SEALS)
                    with socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET) as peer:
                        peer.connect(path); peer.sendmsg([create.FRAME],[(socket.SOL_SOCKET,socket.SCM_RIGHTS,array.array('i',fds))])
                        if count==1:replies.append(peer.recv(64))
                finally:
                    for fd in fds:os.close(fd)
            thread=threading.Thread(target=send,args=(1,)); thread.start()
            with mock.patch.object(create.packet,'validate',return_value={}):self.assertEqual(create._receive(listener,phase),{'safe':True})
            thread.join(); self.assertEqual(replies,[create.ACK])
            before=len(os.listdir('/proc/self/fd')); thread=threading.Thread(target=send,args=(2,)); thread.start()
            with self.assertRaises(ValueError):create._receive(listener,phase)
            thread.join(); self.assertEqual(len(os.listdir('/proc/self/fd')),before)
            replies.clear(); thread=threading.Thread(target=send,args=(1,)); thread.start()
            with mock.patch.object(create.packet,'validate',side_effect=ValueError('invalid packet')):
                with self.assertRaises(ValueError):create._receive(listener,phase)
            thread.join(); self.assertEqual(replies,[b''])

    @unittest.skipUnless(os.name=='posix' and os.geteuid()==0 and hasattr(os,'memfd_create'),'requires disposable Linux root fixture')
    def test_distinct_short_lived_root_sender_waits_for_ack(self):
        with tempfile.TemporaryDirectory() as directory:
            path=str(Path(directory)/'packet.sock'); listener=socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET)
            listener.bind(path); listener.listen(1); self.addCleanup(listener.close)
            phase=SimpleNamespace(fence=SimpleNamespace(context={'operation_id':'op','commit':'sha'}),namespace_uid='ns',initial={},missing=[],media=b'ca',
                                  _check_current=mock.Mock(),_distinct_keys=mock.Mock())
            for valid in (True,False):
                pid=os.fork()
                if pid==0:
                    try:
                        fd=os.memfd_create('fixture',os.MFD_ALLOW_SEALING); os.fchmod(fd,0o600); os.write(fd,b'{"safe":true}')
                        fcntl.fcntl(fd,fcntl.F_ADD_SEALS,create.channel.REQUIRED_SEALS)
                        with socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET) as peer:
                            peer.settimeout(10); peer.connect(path); peer.sendmsg([create.FRAME],[(socket.SOL_SOCKET,socket.SCM_RIGHTS,array.array('i',[fd]))])
                            answer=peer.recv(64)
                        os.close(fd); os._exit(0 if answer==(create.ACK if valid else b'') else 1)
                    except BaseException:os._exit(2)
                try:
                    with mock.patch.object(create.packet,'validate',side_effect=(lambda *a:{} if valid else (_ for _ in ()).throw(ValueError()))):
                        if valid:self.assertEqual(create._receive(listener,phase),{'safe':True})
                        else:
                            with self.assertRaises(ValueError):create._receive(listener,phase)
                finally:
                    _,status=os.waitpid(pid,0); self.assertEqual(os.waitstatus_to_exitcode(status),0)
