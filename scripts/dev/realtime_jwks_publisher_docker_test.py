"""Optional isolated test of the exact pinned nginx image and public route."""
import os
from pathlib import Path
import subprocess
import tempfile
import time
import unittest
import uuid

import realtime_jwks_publisher as publisher


@unittest.skipUnless(os.environ.get('VOICE_REALTIME_JWKS_DOCKER') == '1', 'requires isolated Docker fixture')
class DockerRuntimeTest(unittest.TestCase):
    def test_pinned_nginx_serves_only_exact_public_document(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory); key = root/'tls.key'; cert = root/'tls.crt'
            subprocess.run(['openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '2',
                            '-subj', '/CN=fixture', '-keyout', str(key), '-out', str(cert)],
                           stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=True, timeout=30)
            (root/'default.conf').write_text(publisher.CONFIG_TEXT)
            (root/'realtime.json').write_text('{"keys":[]}')
            name = 'voice-jwks-fixture-' + uuid.uuid4().hex[:16]
            created = subprocess.run(['docker','create','--name',name,'--network','none',
                                      '--label','voice.test.owner=realtime-jwks-publisher',
                                      '--entrypoint','sh',publisher.IMAGE,'-c','sleep 180'],
                                     capture_output=True, check=True, timeout=30)
            cid = created.stdout.decode().strip()
            def cleanup():
                subprocess.run(['docker','rm','-f','-v',cid], stdout=subprocess.DEVNULL,
                               stderr=subprocess.DEVNULL, check=True, timeout=30)
            self.addCleanup(cleanup)
            subprocess.run(['docker','start',cid], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=True, timeout=30)
            subprocess.run(['docker','exec',cid,'mkdir','-p','/srv/jwks','/etc/voice/tls'],
                           stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=True, timeout=30)
            for source, target in ((root/'default.conf','/etc/nginx/conf.d/default.conf'), (cert,'/etc/voice/tls/tls.crt'),
                                   (key,'/etc/voice/tls/tls.key'), (root/'realtime.json','/srv/jwks/realtime.json')):
                subprocess.run(['docker','cp',str(source),cid+':'+target], stdout=subprocess.DEVNULL,
                               stderr=subprocess.DEVNULL, check=True, timeout=30)
            subprocess.run(['docker','exec','-d',cid,'nginx','-g','daemon off;'], stdout=subprocess.DEVNULL,
                           stderr=subprocess.DEVNULL, check=True, timeout=30)
            def get(path):
                return subprocess.run(['docker','exec',cid,'wget','-qO-','--no-check-certificate',
                                       'https://127.0.0.1:8443'+path], capture_output=True, timeout=10)
            for _ in range(30):
                response = get('/realtime.json')
                if response.returncode == 0: break
                time.sleep(.2)
            else: self.fail('isolated publisher did not become ready')
            self.assertEqual(response.stdout, b'{"keys":[]}')
            self.assertNotEqual(get('/other').returncode, 0)


if __name__ == '__main__': unittest.main()
