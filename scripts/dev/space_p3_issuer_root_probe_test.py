"""Exercise literal root probe against disposable local CA fixtures."""
import hashlib
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


@unittest.skipUnless(os.name=='posix' and os.geteuid()==0 and Path('/root').is_dir(),'isolated root Linux fixture')
class RootProbeTest(unittest.TestCase):
    def test_match_mismatch_missing_and_no_private_output(self):
        with tempfile.TemporaryDirectory(prefix='space-root-probe-',dir='/root') as temporary:
            root=Path(temporary); archive=root/'generation1'; archive.mkdir(mode=0o700)
            def create(name):
                subprocess.run(['openssl','req','-x509','-newkey','rsa:2048','-nodes','-days','2',
                    '-subj','/CN='+name,'-keyout',str(root/(name+'.key')),'-out',str(root/(name+'.crt'))],
                    check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
            create('media'); create('foreign')
            (archive/'media-ca.key').write_bytes((root/'media.key').read_bytes()); (archive/'media-ca.key').chmod(0o600)
            (archive/'media-ca.crt').write_bytes((root/'media.crt').read_bytes()); (archive/'media-ca.crt').chmod(0o600)
            public=root/'public.pem'; public.write_bytes((root/'media.crt').read_bytes()); public.chmod(0o600)
            der=subprocess.run(['openssl','x509','-in',str(public),'-outform','DER'],check=True,capture_output=True).stdout
            doc=Path(__file__).with_name('space_p3_issuer_root_probe.md').read_text(); command=doc.split('```sh\n',1)[1].split('\n```',1)[0]
            script=command.split("<<'PY'\n",1)[1].rsplit('\nPY',1)[0]
            replacements={"'/home/voice-dev/issuance/generation1'":repr(str(archive)),
                "'/home/pmd/voice-prepare-TKKQWcSr/p3-ca-public-4b148349b66742b783ea59a662388b2a/media-ca.pem'":repr(str(public)),
                "'9f181873e80e269b5adfe9b9991a7b6f8ffd8060fa3f38c6116675e9a877b828'":repr(hashlib.sha256(der).hexdigest()),
                "('/', '/home', '/home/voice-dev', '/home/voice-dev/issuance', archive)":repr(('/', '/root', str(root), str(archive))),
                "pwd.getpwnam('pmd').pw_uid":'os.getuid()'}
            for old,new in replacements.items(): self.assertEqual(script.count(old),1); script=script.replace(old,new)
            def run():
                result=subprocess.run(['/usr/bin/python3','-I','-S','-'],input=script.encode(),capture_output=True,check=True,timeout=15)
                self.assertEqual(result.stderr,b''); self.assertNotIn(b'PRIVATE KEY',result.stdout)
                return result.stdout.decode().strip()
            self.assertEqual(run(),'MEDIA_CA_CUSTODY=MATCH')
            (archive/'media-ca.key').write_bytes((root/'foreign.key').read_bytes()); (archive/'media-ca.key').chmod(0o600)
            self.assertEqual(run(),'MEDIA_CA_CUSTODY=MISMATCH')
            (archive/'media-ca.key').unlink(); self.assertEqual(run(),'MEDIA_CA_CUSTODY=MISSING')


if __name__ == '__main__': unittest.main()
