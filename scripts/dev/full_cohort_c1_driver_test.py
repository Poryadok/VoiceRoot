import os
import copy
from contextlib import ExitStack
from pathlib import Path
import socket
import tempfile
import threading
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import full_cohort_c1_driver as driver


class PrivateDirectoryTest(unittest.TestCase):
    def test_create_and_reuse_private_directory(self):
        with tempfile.TemporaryDirectory() as tmp:
            parent=Path(tmp); driver._ensure_private(parent,'preflight')
            self.assertEqual((parent/'preflight').stat().st_mode & 0o777,0o700)
            inode=(parent/'preflight').stat().st_ino
            driver._ensure_private(parent,'preflight')
            self.assertEqual((parent/'preflight').stat().st_ino,inode)

    def test_rejects_symlink_and_wrong_mode(self):
        with tempfile.TemporaryDirectory() as tmp:
            parent=Path(tmp); (parent/'other').mkdir(mode=0o700)
            (parent/'preflight').symlink_to(parent/'other',target_is_directory=True)
            with self.assertRaises((OSError,driver.DriverError,driver.story_media_backup.BackupError)):
                driver._ensure_private(parent,'preflight')
            (parent/'preflight').unlink(); (parent/'preflight').mkdir(mode=0o755)
            with self.assertRaises(driver.DriverError):driver._ensure_private(parent,'preflight')

    def test_refuses_name_and_inode_swap(self):
        with tempfile.TemporaryDirectory() as tmp:
            parent=Path(tmp); (parent/'preflight').mkdir(mode=0o700); (parent/'other').mkdir(mode=0o700)
            with self.assertRaises(driver.DriverError):driver._ensure_private(parent,'foreign')
            original=os.stat; calls=0
            def changed(path,*args,**kwargs):
                nonlocal calls
                result=original(path,*args,**kwargs)
                if path=='preflight':
                    calls+=1
                    if calls==2:return original(parent/'other')
                return result
            with patch.object(driver.os,'stat',side_effect=changed):
                with self.assertRaises(driver.DriverError):driver._ensure_private(parent,'preflight')


class HandshakeTest(unittest.TestCase):
    """Socket coordination only; issuer authority is exercised by its own fixture."""
    @staticmethod
    def handle(run): return SimpleNamespace(run=run,cancel=lambda:None)

    def test_trigger_failure_before_connection_is_uncertain(self):
        class Phase:
            def __init__(self): self.marks=0; self.listener=None
            def prepare(self):
                self.listener=socket.socket(socket.AF_INET,socket.SOCK_STREAM)
                self.listener.bind(('127.0.0.1',0)); self.listener.listen(1)
            def _uncertain(self): self.marks+=1
            def close(self): self.listener.close()
        phase=Phase()
        trigger=self.handle(lambda:(_ for _ in ()).throw(OSError('unavailable')))
        with patch.object(driver.issuer_client,'new_preflight_trigger',return_value=trigger,create=True):
            with self.assertRaises(driver.DriverError):driver._handshake(phase,'token')
        self.assertEqual(phase.marks,1)

    def test_ack_and_verified_receipt_are_required(self):
        class Phase:
            def __init__(self): self.listener=None
            def prepare(self):
                self.listener=socket.socket(socket.AF_INET,socket.SOCK_STREAM)
                self.listener.bind(('127.0.0.1',0)); self.listener.listen(1)
            def finish(self):
                connection,_=self.listener.accept(); connection.close(); self.listener.close()
                return {'status':'done'}
        phase=Phase()
        def trigger(current,token):
            with socket.create_connection(current.listener.getsockname(),timeout=5): pass
        with patch.object(driver.issuer_client,'new_preflight_trigger',return_value=self.handle(lambda:trigger(phase,'token')),create=True), \
             patch.object(driver.issuer,'verify_current',return_value={'status':'done'}):
            self.assertEqual(driver._handshake(phase,'token'),{'status':'done'})

    def test_lost_response_marks_uncertain_once_and_stops(self):
        class Phase:
            def __init__(self): self.marks=0; self.closed=False; self.listener=None
            def prepare(self):
                self.listener=socket.socket(socket.AF_INET,socket.SOCK_STREAM)
                self.listener.bind(('127.0.0.1',0)); self.listener.listen(1)
            def finish(self):
                connection,_=self.listener.accept(); connection.close(); self._uncertain()
                raise RuntimeError('response unknown')
            def _uncertain(self): self.marks+=1
            def close(self): self.closed=True; self.listener.close()
        phase=Phase()
        def trigger(current,token):
            with socket.create_connection(current.listener.getsockname(),timeout=5): pass
        with patch.object(driver.issuer_client,'new_preflight_trigger',return_value=self.handle(lambda:trigger(phase,'token')),create=True):
            with self.assertRaises(RuntimeError):driver._handshake(phase,'token')
        self.assertEqual(phase.marks,1); self.assertTrue(phase.closed)

    def test_lost_finish_cancels_and_joins_trigger_before_return(self):
        class Phase:
            def __init__(self): self.listener=None; self.marks=0
            def prepare(self):
                self.listener=socket.socket(socket.AF_INET,socket.SOCK_STREAM)
                self.listener.bind(('127.0.0.1',0)); self.listener.listen(1)
            def finish(self):
                connection,_=self.listener.accept(); connection.close(); self._uncertain()
                raise RuntimeError('response unknown')
            def _uncertain(self): self.marks+=1
            def close(self): self.listener.close()
        phase=Phase(); cancelled=threading.Event(); release=threading.Event(); exited=threading.Event(); returned=threading.Event()
        def run():
            with socket.create_connection(phase.listener.getsockname(),timeout=5): pass
            if not cancelled.wait(5): raise RuntimeError('trigger not cancelled')
            if not release.wait(5): raise RuntimeError('fixture worker not released')
            exited.set()
        handle=SimpleNamespace(run=run,cancel=cancelled.set)
        def call():
            try: driver._handshake(phase,'token')
            except RuntimeError: pass
            finally: returned.set()
        with patch.object(driver.issuer_client,'new_preflight_trigger',return_value=handle,create=True):
            caller=threading.Thread(target=call); caller.start()
            try:
                self.assertTrue(cancelled.wait(5)); self.assertFalse(returned.wait(.1))
            finally:
                release.set(); caller.join(5)
        self.assertFalse(caller.is_alive()); self.assertTrue(returned.is_set())
        self.assertTrue(cancelled.is_set()); self.assertTrue(exited.is_set()); self.assertEqual(phase.marks,1)


class SharedLedgerTest(unittest.TestCase):
    def test_phase_marker_and_receiver_absence_are_required(self):
        fence=SimpleNamespace(owned=lambda row:row.get('operation')=='owned')
        receiver=SimpleNamespace(ledger='receiver',effects_key='receiver_effects')
        row={'operation':'owned','status':'active','preflight':{'status':'done'}}
        revision=('uid','4'); reads=[(row.copy(),revision)]
        driver._check_shared_row(fence,row,revision,reads,{'preflight':{'status':'done'}},receiver)
        for changed in (dict(row,preflight={'status':'pending'}),dict(row,receiver=None),dict(row,receiver_effects={})):
            with self.subTest(changed=changed),self.assertRaises(driver.DriverError):
                driver._check_shared_row(fence,changed,revision,[(changed.copy(),revision)],{'preflight':{'status':'done'}},receiver)
        with self.assertRaises(driver.DriverError):
            driver._check_shared_row(fence,row,revision,[(row.copy(),('uid','5'))],{'preflight':{'status':'done'}},receiver)

    def test_final_guard_marker_drift_refuses_creator_result(self):
        """Diagnostic callback seam; real issuer/phase composition is tested separately."""
        from media_operation_journal import Journal
        with tempfile.TemporaryDirectory() as tmp, ExitStack() as stack:
            context={'operation_id':'media-c1-'+'a'*32,'commit':'b'*40}; lock={'fixed':'lock'}
            row={'status':'active','config':{'status':'done'},'jdk':{'status':'done'},
                 'peer':{'status':'done'},'issuer':{'status':'done'},'trust':{'status':'prepared'},'target':{'status':'done'}}
            def ledger(client): return SimpleNamespace(client=client,read=lambda:(copy.deepcopy(row),('uid','1')))
            def phase(name,record):
                j=Journal(Path(tmp)/name,{'resource_context':context,'namespace_uid':'ns'}); stack.callback(j.close)
                c=SimpleNamespace(journal=j); f=SimpleNamespace(context=context,ledger=ledger(c))
                return SimpleNamespace(journal=j,client=c,fence=f,lock=lock,namespace_uid='ns',record=copy.deepcopy(record))
            config,jdk,peer=(phase(name,row[name]) for name in ('config','jdk','peer'))
            issuer,receiver,trust,target=(phase(name,row.get(name)) for name in ('issuer','receiver','trust','target'))
            issuer.ledger='issuer'; receiver.ledger='receiver'; receiver.effects_key='receiver_effects'
            receiver.initial={'snapshot':'fixed'}; receiver.media=b'public-ca'
            root_client=SimpleNamespace(); root_fence=SimpleNamespace(ledger=ledger(root_client),owned=lambda value:value.get('status')=='active')
            s=SimpleNamespace(context=context,lock=lock,namespace_uid='ns',fence=root_fence,
                package=SimpleNamespace(journal=SimpleNamespace(state={'steps':{}})),
                inventory=SimpleNamespace(journal=SimpleNamespace(state={'steps':{}})),cut=object(),
                creator_config=config,target_jdk=jdk,peer_target=peer,_execution_actor='actor')
            s._creator_authority=lambda:object(); s.prepare_rootless_creator_base=lambda token:None
            s.prepare_peer_targets=lambda token:None; s._execution_final=lambda token,authority:None
            calls=[]
            def guard(token):
                calls.append(token)
                if len(calls)==2: row['issuer']={'status':'changed'}
            s._creator_guard=guard; s._open=lambda *args:(None,None,None)
            subject=driver.C1Driver.__new__(driver.C1Driver); subject.supervisor=s; subject.invoked=True; subject.started=False
            subject.preflight=subject.receiver=subject.trust=subject.target=None
            subject.auth_preparation=SimpleNamespace(supervisor=s,package=s.package,context=copy.deepcopy(context),
                actor='actor',_receipt={'auth':'fixture'},verify_current=lambda token:{'auth':'fixture'})
            for owner,name,value in ((driver.issuer,'SOCKET_ROOT',Path(tmp)),(driver.delegated,'ROOT',Path(tmp)),
                (driver.canonical,'ROOT',Path(tmp)),(driver.creator_config,'LEDGER','config'),(driver.target_jdk,'LEDGER','jdk'),
                (driver.peer_phase,'LEDGER','peer'),(driver.trust_effect,'LEDGER','trust'),(driver.target_plan,'LEDGER','target'),
                (driver.backup,'AuthToolPreparation',SimpleNamespace)):
                stack.enter_context(patch.object(owner,name,value))
            for owner,name,value in ((driver,'_bases',None),(driver,'_directory',None),(driver,'_handshake',{'proof':'issuer'}),
                (driver.issuer,'PeerIssuerPreflightPhase',issuer),(driver.issuer,'verify_current',{'proof':'issuer'}),
                (driver.delegated,'CreatorReceiver',receiver),(driver.delegated,'inspect_existing',{'profile':'creator'}),
                (driver.delegated,'inspect_space_from_creator',{'profile':'space'}),(driver.role,'inspect_role_from_creator',{'profile':'role'}),
                (driver.trust_effect,'TrustEffectPhase',trust),(driver.trust_effect,'verify_prepared',{'proof':'trust'}),
                (driver.target_plan,'TargetPlanPhase',target),(driver.target_plan,'verify_current',{'receipt':{'proof':'target'}})):
                stack.enter_context(patch.object(owner,name,return_value=value))
            trust.prepare=lambda token:{'proof':'trust'}; target.run=lambda token,actor:{'proof':'target'}
            with self.assertRaises(driver.DriverError):subject._prepare_creator('token')
            self.assertEqual(len(calls),2)


if __name__=='__main__':unittest.main()
