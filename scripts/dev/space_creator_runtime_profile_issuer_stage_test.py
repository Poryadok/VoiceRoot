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
import space_creator_runtime_profile_issuer as issuer
import space_creator_runtime_profile_issuer_export as exporter
import space_creator_runtime_profile_issuer_packet as packet

@unittest.skipUnless(os.geteuid()==0,'isolated root fixture required')
class StageTest(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory(); self.addCleanup(self.tmp.cleanup); self.root=Path(self.tmp.name)
        self.op='media-c1-'+'a'*32; self.sock=self.root/'packet.sock'
        self.server=socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET); self.server.bind(str(self.sock)); self.server.listen(1)
        self.addCleanup(self.server.close)
        self.request={'context':{'operation_id':self.op},'namespace_uid':'namespace','lock':{},'ca':{},
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
        self.assertTrue((self.root/('space-creator-'+'a'*32+'.once')).is_file())
        self.assertTrue((self.root/('space-creator-'+'a'*32)/'complete').is_file())

    def test_no_ready_prevents_any_generation(self):
        thread=self.serve('no-ready')
        with self.assertRaises(issuer.IssuerError): issuer.stage(self.op,'b'*40,os.getpid(),exporter.existing._start_ticks(os.getpid()))
        thread.join(timeout=5); issuer._prepare.assert_not_called()
        self.assertFalse((self.root/('space-creator-'+'a'*32+'.once')).exists())

    def test_post_generation_drift_is_durable_hold(self):
        thread=self.serve('drift')
        with self.assertRaises(issuer.IssuerError): issuer.stage(self.op,'b'*40,os.getpid(),exporter.existing._start_ticks(os.getpid()))
        thread.join(timeout=5)
        self.assertTrue((self.root/('space-creator-'+'a'*32+'.once')).exists())
        self.assertFalse((self.root/('space-creator-'+'a'*32)/'complete').exists())

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
