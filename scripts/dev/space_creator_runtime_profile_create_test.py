import array
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
                marker={'status':'pending'}; phase._write(record=marker)
                phase._write(effects={'voice-auth-media-server-tls':{'status':'pending'}})
                self.assertEqual(backing.row[create.LEDGER],marker)
                self.assertEqual(backing.row[create.EFFECTS]['voice-auth-media-server-tls']['status'],'pending')
                backing.row[create.EFFECTS]={}
                with self.assertRaises(create.schema.GuardLost):phase._guard()

    def test_real_crypto_rejects_cross_family_key_reuse(self):
        fixture_class=crypto_fixture.PacketTest; fixture_class.setUpClass(); self.addCleanup(fixture_class.tearDownClass)
        phase=create.Session.__new__(create.Session); phase.objects={obj['metadata']['name']:obj for obj in fixture_class.objects}
        phase._distinct_keys()
        phase.objects=copy.deepcopy(phase.objects)
        phase.objects['voice-space-media-client-tls']['data']['tls.key']=phase.objects['voice-gateway-media-client-tls']['data']['tls.key']
        with self.assertRaises(ValueError):phase._distinct_keys()

    def test_server_metadata_injection_rejected(self):
        base=desired(); actual=copy.deepcopy(base); actual['metadata'].update(uid='uid',resourceVersion='1')
        self.assertTrue(create._closed(actual,base))
        for key,value in (('ownerReferences',[{'uid':'unexpected'}]),('finalizers',['unexpected']),('labels',{'other':'x'})):
            bad=copy.deepcopy(actual); bad['metadata'][key]=value
            with self.subTest(key=key):self.assertFalse(create._closed(bad,base))

    def test_bad_dry_run_stops_before_any_create(self):
        phase=create.Session.__new__(create.Session); phase.exchanged=True; phase.listener=object(); phase.record={'status':'ready'}
        phase.pending=True; phase.completed=False; phase.initial={}; phase.media=b'ca'; phase.missing=['voice-auth-media-server-tls']
        phase.fence=SimpleNamespace(context={'operation_id':'op','commit':'sha'}); phase.namespace_uid='ns'; phase.effects={}
        phase._socket=mock.Mock(); phase._check_current=mock.Mock(); phase._drained=mock.Mock(); phase._distinct_keys=mock.Mock(); phase._uncertain=mock.Mock(); phase.close=mock.Mock()
        obj=desired(); bad=copy.deepcopy(obj); bad['metadata']['ownerReferences']=[{'uid':'unexpected'}]
        with mock.patch.object(create,'_receive',return_value={}), mock.patch.object(create.packet,'validate',return_value={phase.missing[0]:obj}), \
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
        with mock.patch.object(create,'_receive',return_value={}), mock.patch.object(create.packet,'validate',return_value={phase.missing[0]:obj}), \
             mock.patch.object(create,'_create') as write:
            with self.assertRaises(create.schema.GuardLost):phase.finish()
            write.assert_not_called(); phase._uncertain.assert_called_once()

    def test_done_requires_owned_uid_and_exact_readback(self):
        phase=create.Session.__new__(create.Session); phase.exchanged=True; phase.listener=object(); phase.record={'status':'ready'}
        phase.pending=True; phase.completed=False; phase.initial={}; phase.media=b'ca'; phase.missing=['voice-auth-media-server-tls']
        phase.fence=SimpleNamespace(context={'operation_id':'op','commit':'sha'}); phase.namespace_uid='ns'; phase.effects={}
        phase._socket=mock.Mock(); phase._check_current=mock.Mock(); phase._drained=mock.Mock(); phase._distinct_keys=mock.Mock(); phase._uncertain=mock.Mock(); phase.close=mock.Mock()
        obj=desired(); actual=copy.deepcopy(obj); actual['metadata'].update(uid='uid',resourceVersion='1')
        def save(record=None,effects=None):
            if record is not None: phase.record=copy.deepcopy(record)
            if effects is not None: phase.effects=copy.deepcopy(effects)
        phase._write=save
        with mock.patch.object(create,'_receive',return_value={}), mock.patch.object(create.packet,'validate',return_value={phase.missing[0]:obj}), \
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
        with mock.patch.object(create,'_receive',return_value={}), mock.patch.object(create.packet,'validate',return_value={phase.missing[0]:obj}), \
             mock.patch.object(create,'_get',return_value=None), mock.patch.object(create,'_create',side_effect=[obj,RuntimeError('PRIVATE-SENTINEL')]):
            with self.assertRaises(create.CreateError) as failure:phase.finish()
        self.assertNotIn('PRIVATE-SENTINEL',str(failure.exception))
        self.assertEqual(phase.effects[phase.missing[0]]['status'],'pending'); phase._uncertain.assert_called_once()

    def test_transient_preflight_observer_failure_is_guard_lost(self):
        phase=create.Session.__new__(create.Session); phase._guard=mock.Mock()
        phase.initial={'ca':{'uid':'ca'},'creator_secrets':{}}; phase.effects={}; phase.objects=None
        phase.preflight=SimpleNamespace(_ca=mock.Mock(side_effect=[ValueError('private'),{'uid':'ca'}]),_creator=mock.Mock(return_value={}))
        with self.assertRaises(create.schema.GuardLost):phase._check_current()
        phase._check_current()

    @unittest.skipUnless(os.name=='posix' and os.geteuid()==0 and hasattr(os,'memfd_create'),'requires disposable Linux root fixture')
    def test_sealed_fd_and_multi_fd_cleanup(self):
        with tempfile.TemporaryDirectory() as directory:
            path=str(Path(directory)/'packet.sock'); listener=socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET)
            listener.bind(path); listener.listen(1); self.addCleanup(listener.close)
            def send(count):
                fds=[]
                try:
                    for _ in range(count):
                        fd=os.memfd_create('fixture',os.MFD_ALLOW_SEALING); fds.append(fd); os.fchmod(fd,0o600)
                        os.write(fd,json.dumps({'safe':True}).encode()); fcntl.fcntl(fd,fcntl.F_ADD_SEALS,create.channel.REQUIRED_SEALS)
                    with socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET) as peer:
                        peer.connect(path); peer.sendmsg([create.FRAME],[(socket.SOL_SOCKET,socket.SCM_RIGHTS,array.array('i',fds))])
                finally:
                    for fd in fds:os.close(fd)
            thread=threading.Thread(target=send,args=(1,)); thread.start(); self.assertEqual(create._receive(listener),{'safe':True}); thread.join()
            before=len(os.listdir('/proc/self/fd')); thread=threading.Thread(target=send,args=(2,)); thread.start()
            with self.assertRaises(ValueError):create._receive(listener)
            thread.join(); self.assertEqual(len(os.listdir('/proc/self/fd')),before)
