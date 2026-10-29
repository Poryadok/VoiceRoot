"""Root launcher round-trip with isolated fixture paths and synthetic UID custody."""
import array
import fcntl
import json
import os
from pathlib import Path
import socket
import subprocess
import tempfile
import threading
import unittest
from unittest.mock import patch

import realtime_p3_root_bundle as bundle
import space_p3_issuer_bundle as subject
import space_p3_issuer_source as source
import realtime_p3_root_export as export
import realtime_p3_create_only_socket as receiver
import space_p3_issuer_packet as packet


@unittest.skipUnless(os.name=='posix' and os.geteuid()==0 and Path('/root').is_dir(),'isolated root Linux fixture')
class LauncherTest(unittest.TestCase):
    def test_source_capture_stage_and_sealed_export(self):
        with tempfile.TemporaryDirectory(prefix='space-issuer-fixture-',dir='/root') as scratch:
            root=Path(scratch); issuance=root/'issuance'; issuance.mkdir(mode=0o700)
            archive=issuance/'generation1'; archive.mkdir(mode=0o700)
            subprocess.run(['openssl','req','-x509','-newkey','rsa:2048','-nodes','-days','120',
                '-subj','/CN=fixture-media','-addext','basicConstraints=critical,CA:TRUE',
                '-keyout',str(archive/'media-ca.key'),'-out',str(archive/'media-ca.crt')],
                check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
            (archive/'media-ca.key').chmod(0o600); public=root/'media.crt'; public.write_bytes((archive/'media-ca.crt').read_bytes()); public.chmod(0o600)
            sources=Path(__file__).parent; contents={name:(sources/name).read_bytes() for name in source.ISSUER_FILES}
            def replace(name,before,after):
                raw=contents[name]; old=before.encode(); self.assertEqual(raw.count(old),1)
                contents[name]=raw.replace(old,after.encode())
            replace('space_p3_issuer.py',"Path('/home/voice-dev/issuance/generation1')",f'Path({str(archive)!r})')
            replace('space_p3_issuer.py',"Path('/home/voice-dev/issuance')",f'Path({str(issuance)!r})')
            replace('space_p3_issuer_export.py',"Path('/home/voice-dev/issuance')",f'Path({str(issuance)!r})')
            sockets=root/'sockets'; sockets.mkdir(mode=0o700); operations=sockets/'operations'; operations.mkdir(mode=0o700)
            suffix='a'*32; directory=operations/suffix; directory.mkdir(mode=0o700)
            replace('space_p3_issuer_export.py',"Path('/home/pmd/voice-space-p3/operations')",f'Path({str(operations)!r})')
            replace('realtime_p3_root_export.py',"pwd.getpwnam('pmd').pw_uid",'os.getuid()')
            replace('realtime_p3_missing_issuer.py',"pwd.getpwnam('pmd').pw_uid",'os.getuid()')
            original=(sources/'realtime_p3_root_launch.template.sh').read_bytes(); actual_build=bundle._build
            def fixture_build(items,template,output):
                replacements={'/home/pmd/voice-space-p3/p3-source.tar':str(root/'p3-source.tar'),
                    "pathlib.Path('/home/voice-dev/issuance')":f'pathlib.Path({str(issuance)!r})',
                    "pwd.getpwnam('pmd').pw_uid":'os.getuid()'}
                for old,new in replacements.items(): self.assertEqual(template.count(old),1); template=template.replace(old,new)
                return actual_build(items,template,output)
            def blob(repo,commit,path):
                return original if path.endswith('realtime_p3_root_launch.template.sh') else contents[path.rsplit('/',1)[1]]
            with patch.object(subject.contract,'verify'), patch.object(subject.source,'_blob',side_effect=blob), \
                 patch.object(subject.source,'_disk',return_value=original), patch.object(bundle,'_build',side_effect=fixture_build):
                subject.build_git('/fixture','a'*40,root)
            (root/'p3-source.tar').chmod(0o600)
            op='media-c1-'+suffix; launcher=root/'p3-root-launch.sh'
            command=['bash',str(launcher),'--operation-id',op,'--media-public',str(public)]
            result=subprocess.run(command,check=True,capture_output=True,timeout=60)
            receipt=json.loads(result.stdout); self.assertEqual(receipt['status'],'staged')
            sock=directory/'packet.sock'; listener=socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET); listener.bind(str(sock)); os.chmod(sock,0o600); listener.listen(1)
            received=[]; errors=[]
            def receive():
                try:
                    received.append(receiver.receive(listener,op,'b'*40))
                except Exception as error: errors.append(error)
            thread=threading.Thread(target=receive); thread.start()
            try:
                subprocess.run(['bash',str(launcher),'--export','--operation-id',op,'--source-sha','b'*40,
                    '--consumer-pid',str(os.getpid()),'--consumer-start-ticks',str(export._start_ticks(os.getpid()))],
                    check=True,capture_output=True,timeout=60)
            finally: thread.join(timeout=5); listener.close()
            self.assertEqual(errors,[]); self.assertFalse(thread.is_alive())
            self.assertEqual(received[0]['issuer_receipt'],receipt)
            objects,jwks=packet.validate(received[0],op,'b'*40,public.read_bytes())
            self.assertEqual(jwks,receipt['public_jwks']); self.assertEqual(len(objects),2)


if __name__ == '__main__': unittest.main()
