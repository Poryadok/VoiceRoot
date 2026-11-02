"""Actual phase/socket/root issuer composition; cluster and TT are fixtures."""
import hashlib
import json
import os
from pathlib import Path
import socket
import tempfile
import threading
import unittest
from unittest.mock import patch

import reset_credentials as reset
import space_p3_issuer_preflight as preflight
import space_p3_issuer_preflight_test as fixture
import space_p3_issuer_preflight_metadata_root as metadata
import space_creator_runtime_profile_issuer as issuer
import space_creator_runtime_profile_issuer_export as exporter
import space_creator_runtime_profile_issuer_packet as packet

@unittest.skipUnless(os.name=='posix' and os.geteuid()==0,'isolated root Linux fixture')
class JointTest(unittest.TestCase):
    def test_actual_preflight_exchange_and_crypto_stage(self):
        case=fixture.PreflightTest('test_fenced_root_response_and_live_ref_drift'); case.setUp()
        self.addCleanup(case.doCleanups)
        with tempfile.TemporaryDirectory(dir='/root',prefix='creator-joint-') as parent:
            root=Path(parent); archive=root/'archive'; archive.mkdir(mode=0o700)
            key,cert=reset.ResetMaterial(root).ca('fixture')
            (archive/'media-ca.crt').write_bytes(cert.read_bytes()); (archive/'media-ca.key').write_bytes(key.read_bytes())
            os.chmod(archive/'media-ca.key',0o600)
            case.ca['der_sha256']=hashlib.sha256(packet._openssl(['x509','-in','{0}','-outform','DER'],[cert.read_bytes()])).hexdigest()
            for name in packet.FAMILIES:
                if name!='voice-space-media-client-tls':
                    keys=['active-kid','current.pem','next.pem'] if packet.FAMILIES[name] is None else ['tls.crt','tls.key']
                    case.creator[name]={'state':'present','uid':'old-'+name,'resourceVersion':'5','keys':keys}
            receipt=case._completed(); sock=root/'packet.sock'; errors=[]
            server=socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET); server.bind(str(sock)); server.listen(1)
            def serve():
                try:
                    conn,_=server.accept()
                    with conn:
                        for _ in range(2):
                            if conn.recv(64)!=b'PREFLIGHT-REQUEST\n': raise ValueError('unexpected request')
                            preflight.exchange_root_request(case.phase,'token',conn)
                except Exception as error: errors.append(type(error).__name__)
            thread=threading.Thread(target=serve,daemon=True); thread.start()
            with patch.object(exporter,'ROOT',root),patch.object(exporter,'_socket_path',return_value=sock), \
                 patch.object(exporter.existing,'_pmd_uid',return_value=0),patch.object(issuer.ca_custody,'ARCHIVE',archive), \
                 patch.object(metadata,'observe',side_effect=lambda *args:json.loads(json.dumps(case.creator))), \
                 patch.dict(os.environ,{'PATH':'/usr/bin:/bin','LANG':'C','LC_ALL':'C'},clear=True):
                result=issuer.stage(case.base.context['operation_id'],case.base.context['commit'],os.getpid(),exporter.existing._start_ticks(os.getpid()))
            thread.join(timeout=10); server.close()
            self.assertFalse(thread.is_alive()); self.assertFalse(errors)
            self.assertEqual(result['object_names'],['voice-space-media-client-tls'])
            dest=root/('space-creator-'+case.base.context['operation_id'][-32:])
            envelope=json.loads((dest/'objects.json').read_bytes()); digest=preflight.writer_cut.digest(receipt)
            self.assertEqual(set(packet.validate(envelope,case.base.context['operation_id'],case.base.context['commit'],
                                                  'ns',digest,result['object_names'],cert.read_bytes())),set(result['object_names']))

if __name__=='__main__': unittest.main()
