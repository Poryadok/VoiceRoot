"""Isolated synthetic root custody and socket protocol checks."""
import hashlib
import json
import os
from pathlib import Path
import socket
import subprocess
import tempfile
import threading
import unittest
from unittest.mock import patch

import space_p3_issuer_preflight_root as subject
import media_backup_phase_test as fixture


@unittest.skipUnless(os.name=='posix' and os.geteuid()==0,'isolated root Linux fixture')
class RootTest(unittest.TestCase):
    def test_actual_ca_key_and_root_socket_with_wrong_digest_refused(self):
        with tempfile.TemporaryDirectory(prefix='space-preflight-',dir='/root') as tmp:
            root=Path(tmp); archive=root/'archive'; archive.mkdir(mode=0o700)
            subprocess.run(['openssl','req','-x509','-newkey','rsa:2048','-nodes','-days','2','-subj','/CN=fixture',
                '-addext','basicConstraints=critical,CA:TRUE','-keyout',str(archive/'media-ca.key'),
                '-out',str(archive/'media-ca.crt')],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
            (archive/'media-ca.key').chmod(0o600)
            der=subprocess.run(['openssl','x509','-in',str(archive/'media-ca.crt'),'-outform','DER'],check=True,capture_output=True).stdout
            digest=hashlib.sha256(der).hexdigest(); path=root/'socket'; listener=socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET)
            self.addCleanup(listener.close); listener.bind(str(path)); listener.listen(1)
            base=fixture.PhaseTest('test_exact_terminal_receipt_and_no_replay')
            def cleanup():
                if not base.doCleanups(): raise RuntimeError('nested cleanup failed')
            self.addCleanup(cleanup); base.setUp()
            request={'context':base.context,'namespace_uid':'ns','nonce':'c'*32,
                     'ca':{'uid':'uid','resourceVersion':'rv','der_sha256':digest},'lock':base.lock}
            args=(base.context['operation_id'],os.getpid(),subject.root_export._start_ticks(os.getpid()))
            result=[]; errors=[]
            def receive():
                try:
                    listener.settimeout(10); channel,_=listener.accept()
                    with channel:
                        channel.send(json.dumps(request).encode()); result.append(json.loads(channel.recv(4096)))
                except Exception as error: errors.append(error)
            with patch.object(subject.issuer,'ARCHIVE',archive), patch.object(subject,'_socket',return_value=path), \
                 patch.object(subject.metadata,'observe',return_value={}), \
                 patch.object(subject.root_export,'_pmd_uid',return_value=0):
                with self.assertRaises(ValueError): subject._check_ca({'der_sha256':'f'*64})
                thread=threading.Thread(target=receive); thread.start()
                try: subject.check(*args)
                finally: thread.join(timeout=10)
            self.assertFalse(thread.is_alive()); self.assertEqual(errors,[])
            self.assertEqual(result[0]['ca']['der_sha256'],digest)
            self.assertNotIn('PRIVATE KEY',str(result)); self.assertEqual(result[0]['scope'],'root-custody-available-not-issued')


if __name__=='__main__': unittest.main()
