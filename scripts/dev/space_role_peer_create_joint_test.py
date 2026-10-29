"""Real Role signer, socket, crypto and receiver; authority/Kube are synthetic."""
import copy
import hashlib
import json
import os
from pathlib import Path
import socket
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import dev_peer_issuer_client as client
import dev_peer_issuer_metadata as metadata
import dev_peer_issuer_service as service
import reset_credentials as reset
import space_role_peer_create as role


@unittest.skipUnless(os.name=='posix' and os.geteuid()==0,'isolated Linux root fixture')
class RoleJointTest(unittest.TestCase):
    def test_actual_signer_callback_and_missing_role_pair(self):
        with tempfile.TemporaryDirectory(dir='/root',prefix='role-peer-joint-') as temporary:
            root=Path(temporary); archive=root/'archive'; archive.mkdir(mode=0o700)
            operations=root/'operations'; operations.mkdir(mode=0o700)
            key,cert=reset.ResetMaterial(root).ca('fixture')
            (archive/'media-ca.crt').write_bytes(cert.read_bytes()); (archive/'media-ca.key').write_bytes(key.read_bytes())
            (archive/'media-ca.key').chmod(0o600); media=cert.read_bytes()
            op='media-c1-'+'a'*32; source='b'*40; path=operations/'role.sock'
            listener=socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET); listener.bind(str(path)); listener.listen(1); self.addCleanup(listener.close)
            phase=role.Session.__new__(role.Session); phase.listener=listener; phase.connection=None; phase.issue_thread=None
            phase.issue_done=phase.issue_result=None; phase.pending=True; phase.completed=False; phase.exchanged=False
            phase.record={'status':'ready'}; phase.effects={}; phase.missing=sorted(role.NAMES); phase.media=media
            phase.fence=SimpleNamespace(context={'operation_id':op,'commit':source}); phase.namespace_uid='ns'; phase.token='token'
            phase.preflight=SimpleNamespace(); phase.objects=None; phase.journal=SimpleNamespace(state={'steps':{}})
            phase.initial={'profile':'role','ca':{'uid':'ca','resourceVersion':'1',
                'der_sha256':hashlib.sha256(role.role_material.crypto._openssl(['x509','-in','{0}','-outform','DER'],[media])).hexdigest()},
                'creator_secrets':{name:{'state':'absent'} for name in metadata.MEMBERS}}
            objects={}; phase._socket=lambda:None; phase._check_current=lambda:None; phase._drained=lambda:None
            phase._local=lambda:None; phase._uncertain=lambda:None; phase.close=lambda:None
            phase._guard=lambda:({role.LEDGER:phase.record,role.EFFECTS:phase.effects},1)
            def save(record=None,effects=None):
                if record is not None:phase.record=copy.deepcopy(record); phase.journal.state['steps'][role.STEP]=copy.deepcopy(record)
                if effects is not None:phase.effects=copy.deepcopy(effects)
            phase._write=save
            def get(name):return copy.deepcopy(objects.get(name))
            def apply(obj,dry):
                value=copy.deepcopy(obj)
                if not dry:
                    name=value['metadata']['name']; value['metadata'].update(uid='uid-'+name,resourceVersion='1'); objects[name]=copy.deepcopy(value)
                return value
            def exchange(_phase,_token,channel):
                request={'context':phase.fence.context,'namespace_uid':'ns','lock':{},'ca':phase.initial['ca'],
                         'creator_secrets':phase.initial['creator_secrets'],'profile':'role',
                         'preflight_sha256':role.writer_cut.digest(phase.initial)}
                channel.send(json.dumps(request,sort_keys=True,separators=(',',':')).encode()); channel.send(b'PREFLIGHT-READY\n')
                return copy.deepcopy(phase.initial)
            def connect(*_args):
                peer=socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET); peer.connect(str(path)); return peer
            with patch.object(service,'ROOT',root), patch.object(service,'_connect',side_effect=connect), patch.object(service,'_guard'), \
                 patch.object(service.root_export,'_check_root_custody'), patch.object(service.custody,'ARCHIVE',archive), \
                 patch.object(role.preflight,'exchange_root_request',side_effect=exchange), \
                 patch.object(role,'_get',side_effect=get), patch.object(role,'_create',side_effect=apply), \
                 patch.object(client,'new_issue_trigger',side_effect=lambda *_:SimpleNamespace(
                     run=lambda:service.issue(op,source,'role',os.getpid(),service.root_export._start_ticks(os.getpid())),cancel=lambda:None)):
                receipt=phase.run()
                self.assertEqual(role.verify_completed(phase,'token'),receipt)
            self.assertEqual(set(receipt['objects']),role.NAMES)
            self.assertEqual(set(receipt['public_jwks']),{'voice-role-principal-signing'})
            self.assertNotIn('PRIVATE KEY',str(receipt))


if __name__ == '__main__': unittest.main()
