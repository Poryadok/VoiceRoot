import json
import os
from pathlib import Path
import socket
import tempfile
import threading
import types
import unittest
from unittest.mock import patch

import credentials
import reset_credentials as reset
import space_creator_runtime_profile_issuer as issuer
import space_creator_runtime_profile_issuer_export as exporter
import space_creator_runtime_profile_issuer_packet as packet

REAL_PREPARE=issuer._prepare
REAL_OPEN=issuer.ca_custody._open

@unittest.skipUnless(os.geteuid()==0,'isolated root fixture required')
class StageTest(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory(); self.addCleanup(self.tmp.cleanup); self.root=Path(self.tmp.name)
        self.op='media-c1-'+'a'*32; self.sock=self.root/'packet.sock'
        self.server=socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET); self.server.bind(str(self.sock)); self.server.listen(1)
        self.addCleanup(self.server.close)
        self.request={'context':{'operation_id':self.op,'commit':'b'*40},'namespace_uid':'namespace','lock':{},'ca':{},
                      'creator_secrets':{name:{'state':'absent'} for name in packet.FAMILIES},'preflight_sha256':'c'*64}
        self.cert=self.root/'cert'; self.cert.write_bytes(b'fixture')
        metadata=types.ModuleType('space_p3_issuer_preflight_metadata_root')
        metadata.MEMBERS={name:({'current.pem','next.pem','active-kid'} if spec is None else {'tls.crt','tls.key'})
                          for name,spec in packet.FAMILIES.items()}
        def open_cert(*args):
            fd=os.open(self.cert,os.O_RDONLY); s=os.fstat(fd)
            return fd,(s.st_dev,s.st_ino,s.st_size,s.st_mtime_ns,s.st_ctime_ns)
        def prepare(dest,media,missing,*args):
            credentials.write_private(dest/'objects.json',b'private fixture'); credentials.write_private(dest/'media-public.crt',media)
            return {'object_names':missing}
        patches=(patch.dict('sys.modules',{'space_p3_issuer_preflight_metadata_root':metadata}),
                 patch.dict(os.environ,{'PATH':'/usr/bin:/bin','LANG':'C','LC_ALL':'C'},clear=True),
                 patch.object(exporter,'ROOT',self.root),patch.object(exporter,'_socket_path',return_value=self.sock),
                 patch.object(exporter.existing,'_check_root_custody'),patch.object(exporter.existing,'_pmd_uid',return_value=0),
                 patch.object(issuer.ca_custody,'_open',side_effect=open_cert),patch.object(issuer,'_guard'),
                 patch.object(issuer,'_prepare',side_effect=prepare))
        for item in patches: item.start(); self.addCleanup(item.stop)

    def serve(self,mode):
        def run():
            conn,_=self.server.accept()
            with conn:
                for index in range(2):
                    if conn.recv(64)!=b'PREFLIGHT-REQUEST\n': return
                    request=dict(self.request)
                    if mode=='drift' and index==1: request['preflight_sha256']='d'*64
                    conn.send(json.dumps(request).encode())
                    conn.send(b'NO-READY\n' if mode=='no-ready' else b'PREFLIGHT-READY\n')
                    if mode=='no-ready': return
        thread=threading.Thread(target=run,daemon=True); thread.start(); return thread

    def test_two_authenticated_exchanges_complete_and_reserve(self):
        thread=self.serve('ok'); result=issuer.stage(self.op,'b'*40,os.getpid(),exporter.existing._start_ticks(os.getpid()))
        thread.join(timeout=5)
        self.assertEqual(result['object_names'],sorted(packet.FAMILIES))
        self.assertEqual(issuer._guard.call_count,3)
        self.assertTrue((self.root/'space-creator-issuance.once').is_file())
        self.assertTrue((self.root/('space-creator-'+'a'*32)/'complete').is_file())

    def test_no_ready_prevents_any_generation(self):
        thread=self.serve('no-ready')
        with self.assertRaises(issuer.IssuerError): issuer.stage(self.op,'b'*40,os.getpid(),exporter.existing._start_ticks(os.getpid()))
        thread.join(timeout=5); issuer._prepare.assert_not_called()
        self.assertFalse((self.root/'space-creator-issuance.once').exists())

    def test_post_generation_drift_is_durable_hold(self):
        thread=self.serve('drift')
        with self.assertRaises(issuer.IssuerError): issuer.stage(self.op,'b'*40,os.getpid(),exporter.existing._start_ticks(os.getpid()))
        thread.join(timeout=5)
        self.assertTrue((self.root/'space-creator-issuance.once').exists())
        self.assertFalse((self.root/('space-creator-'+'a'*32)/'complete').exists())
        self.op='media-c1-'+'b'*32; self.request['context']['operation_id']=self.op
        thread=self.serve('ok')
        with self.assertRaises(issuer.IssuerError): issuer.stage(self.op,'b'*40,os.getpid(),exporter.existing._start_ticks(os.getpid()))
        thread.join(timeout=5); self.assertEqual(issuer._prepare.call_count,1)

    def test_existing_complete_secret_is_not_reissued(self):
        name='voice-gateway-principal-signing'
        self.request['creator_secrets'][name]={'state':'present','uid':'old','resourceVersion':'5',
                                              'keys':['active-kid','current.pem','next.pem']}
        thread=self.serve('ok'); result=issuer.stage(self.op,'b'*40,os.getpid(),exporter.existing._start_ticks(os.getpid()))
        thread.join(timeout=5)
        self.assertNotIn(name,result['object_names']); self.assertEqual(len(result['object_names']),4)

    def test_partial_existing_secret_never_starts_generation(self):
        self.request['creator_secrets']['voice-role-media-server-tls']={'state':'present','uid':'old','resourceVersion':'5','keys':['tls.crt']}
        thread=self.serve('ok')
        with self.assertRaises(issuer.IssuerError): issuer.stage(self.op,'b'*40,os.getpid(),exporter.existing._start_ticks(os.getpid()))
        thread.join(timeout=5); issuer._prepare.assert_not_called()
        self.assertFalse((self.root/'space-creator-issuance.once').exists())

    def test_foreign_candidate_never_starts_generation(self):
        self.request['context']['commit']='d'*40
        thread=self.serve('ok')
        with self.assertRaises(issuer.IssuerError): issuer.stage(self.op,'b'*40,os.getpid(),exporter.existing._start_ticks(os.getpid()))
        thread.join(timeout=5); issuer._prepare.assert_not_called()
        self.assertFalse((self.root/'space-creator-issuance.once').exists())

    def test_guarded_stage_with_real_ca_and_real_generated_leaf(self):
        archive=self.root/'archive'; archive.mkdir(mode=0o700)
        key,cert=reset.ResetMaterial(self.root).ca('fixture')
        (archive/'media-ca.crt').write_bytes(cert.read_bytes()); (archive/'media-ca.key').write_bytes(key.read_bytes())
        os.chmod(archive/'media-ca.key',0o600)
        for name in packet.FAMILIES:
            if name!='voice-space-media-client-tls':
                keys=['active-kid','current.pem','next.pem'] if packet.FAMILIES[name] is None else ['tls.crt','tls.key']
                self.request['creator_secrets'][name]={'state':'present','uid':'old-'+name,'resourceVersion':'5','keys':keys}
        with patch.object(issuer.ca_custody,'ARCHIVE',archive),patch.object(issuer.ca_custody,'_check_ancestors'), \
             patch.object(issuer.ca_custody,'_open',side_effect=REAL_OPEN),patch.object(issuer,'_prepare',side_effect=REAL_PREPARE):
            thread=self.serve('ok'); result=issuer.stage(self.op,'b'*40,os.getpid(),exporter.existing._start_ticks(os.getpid()))
            thread.join(timeout=5)
        self.assertEqual(result['object_names'],['voice-space-media-client-tls'])
        dest=self.root/('space-creator-'+'a'*32)
        envelope=json.loads((dest/'objects.json').read_bytes())
        self.assertEqual(set(packet.validate(envelope,self.op,'b'*40,'namespace','c'*64,result['object_names'],cert.read_bytes())),set(result['object_names']))

class GuardTest(unittest.TestCase):
    def test_actual_guard_rechecks_ca_and_exact_observation(self):
        metadata=types.ModuleType('space_p3_issuer_preflight_metadata_root')
        metadata.observe=unittest.mock.Mock(return_value={'leaf':{'state':'absent'}})
        preflight=types.ModuleType('space_p3_issuer_preflight_root'); preflight._check_ca=unittest.mock.Mock()
        request={'ca':{},'namespace_uid':'ns','context':{},'lock':{},'creator_secrets':{'leaf':{'state':'absent'}}}
        with patch.dict('sys.modules',{'space_p3_issuer_preflight_metadata_root':metadata,'space_p3_issuer_preflight_root':preflight}):
            issuer._guard(request); self.assertEqual(preflight._check_ca.call_count,2)
            metadata.observe.return_value={'leaf':{'state':'present'}}
            with self.assertRaises(ValueError): issuer._guard(request)

if __name__=='__main__': unittest.main()
