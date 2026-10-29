import copy
import hashlib
import sys
import threading
from types import SimpleNamespace
import unittest
from unittest.mock import Mock, patch

import realtime_p3_create_only as creation
import realtime_p3_create_only_packet as packet
import realtime_p3_delegated_receiver as receiver


class RequestTest(unittest.TestCase):
    def test_closed_request_binds_all_seven_and_three_public_roots(self):
        roots={identity:{'uid':'uid-'+identity[0],'resourceVersion':'1','certificate':identity[0].encode()} for identity in creation.PUBLIC}
        session=SimpleNamespace(roots=roots,fence=SimpleNamespace(context={'operation_id':'op'}),namespace_uid='ns',
                                lock={'uid':'lock'},plan={'issuer_source_sha':'b'*40})
        request={'context':{'operation_id':'op'},'namespace_uid':'ns','lock':{'uid':'lock'},'issuer_source_sha':'b'*40,
                 'capability_sha256':'a'*64,'objects':{kind.lower()+'/'+name:{'state':'absent'} for kind,name in packet.MEMBERS}}
        for name,identity in zip(('principal','media','space'),creation.PUBLIC):
            value=roots[identity]
            request[name]={'uid':value['uid'],'resourceVersion':'1','der_sha256':hashlib.sha256(value['certificate']).hexdigest()}
        with patch.object(receiver.crypto,'der',side_effect=lambda value:value):
            receiver._request_matches(session,request)
            for change in ('object','root','source','extra'):
                altered=copy.deepcopy(request)
                if change=='object':altered['objects'][next(iter(altered['objects']))]={'state':'present'}
                elif change=='root':altered['space']['resourceVersion']='2'
                elif change=='source':altered['issuer_source_sha']='c'*40
                else:altered['unexpected']=True
                with self.subTest(change=change),self.assertRaises(ValueError):receiver._request_matches(session,altered)

    def test_failed_accept_cancels_and_joins_trigger_before_return(self):
        class Cut:
            def __init__(self):
                self.fence=SimpleNamespace(context={'op':'same'},ledger=SimpleNamespace(read=lambda:({},1)))
                self.lock={'uid':'lock'}; self.namespace_uid='ns'; self.journal=object(); self.client=object()
        class Peer:pass
        class Session:pass
        class Capability:pass
        cut=Cut(); pre=Peer(); pre.cut=cut; pre.fence=cut.fence; pre.lock=cut.lock; pre.namespace_uid='ns'
        cap=Capability(); cap.creator_preflight=pre; cap.token='token'
        session=Session(); session.fence=cut.fence; session.lock=cut.lock; session.namespace_uid='ns'
        session.listener=Mock(); session.listener.accept.side_effect=OSError('fixture'); session.handoff={'status':'ready'}
        session._delegated_cut=None; session.journal=object()
        cancelled=threading.Event(); release=threading.Event(); finished=threading.Event()
        class Handle:
            def run(self):cancelled.wait(5); release.wait(20); finished.set()
            def cancel(self):cancelled.set()
        authority=SimpleNamespace(RealtimeP3Capability=Capability,verify_current=lambda *_:None)
        client=SimpleNamespace(new_issue_trigger=lambda *_:Handle())
        with patch.dict(sys.modules,{'dev_realtime_p3_issuer_capability':authority,'dev_realtime_p3_issuer_client':client}), \
             patch.object(receiver.creation,'Session',Session),patch.object(receiver.peer,'PeerIssuerPreflightPhase',Peer), \
             patch.object(receiver.writer_cut,'WriterCut',Cut),patch.object(receiver.native.SpaceNativeToolPhase,'_custody',return_value='custody'), \
             patch.object(receiver,'_current'):
            outcome=[]
            def call():
                try:receiver.receive(session,cap,'token')
                except receiver.creation.CreateError:outcome.append('hold')
            caller=threading.Thread(target=call); caller.start()
            try:
                self.assertTrue(cancelled.wait(5)); caller.join(5.5); self.assertTrue(caller.is_alive()); self.assertEqual(outcome,[])
            finally:release.set(); caller.join(5)
        self.assertFalse(caller.is_alive()); self.assertTrue(finished.is_set()); self.assertEqual(outcome,['hold'])


if __name__=='__main__':unittest.main()
