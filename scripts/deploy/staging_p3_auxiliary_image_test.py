"""Fixed Docker Hub nginx OCI qualification; no cluster or rollout authority."""
import hashlib
import json
import os
import socket
import threading
import time
import unittest
from unittest.mock import patch

import staging_p3_auxiliary_image as image


def encoded(value): return json.dumps(value,separators=(',',':')).encode()
def sha(raw): return 'sha256:'+hashlib.sha256(raw).hexdigest()


class MetadataTest(unittest.TestCase):
    def fixture(self):
        config=encoded({'os':'linux','architecture':'amd64'})
        manifest=encoded({'schemaVersion':2,'mediaType':'application/vnd.oci.image.manifest.v1+json',
            'config':{'mediaType':'application/vnd.oci.image.config.v1+json','digest':sha(config),'size':len(config)},'layers':[]})
        index=encoded({'schemaVersion':2,'mediaType':'application/vnd.oci.image.index.v1+json','manifests':[
            {'mediaType':'application/vnd.oci.image.manifest.v1+json','digest':sha(manifest),'size':len(manifest),
             'platform':{'os':'linux','architecture':'amd64'}}]})
        return index,manifest,config

    def test_complete_digest_chain_and_platform(self):
        index,manifest,config=self.fixture()
        with patch.object(image,'INDEX',sha(index)), patch.object(image,'MANIFEST',sha(manifest)), patch.object(image,'CONFIG',sha(config)):
            proof=image.validate(index,manifest,config)
            self.assertEqual(proof['image_ref'],'docker.io/library/nginx@'+sha(manifest))
            self.assertEqual(proof['platform'],{'os':'linux','architecture':'amd64','variant':None})
            with self.assertRaises(image.ImageError): image.validate(index,manifest,config+b' ')

    def test_ambiguous_platform_and_duplicate_json_are_rejected(self):
        index,manifest,config=self.fixture(); obj=json.loads(index); obj['manifests'].append(obj['manifests'][0]); index=encoded(obj)
        with patch.object(image,'INDEX',sha(index)), patch.object(image,'MANIFEST',sha(manifest)), patch.object(image,'CONFIG',sha(config)):
            with self.assertRaises(image.ImageError): image.validate(index,manifest,config)
        duplicate=b'{"schemaVersion":2,"schemaVersion":2}'
        with patch.object(image,'INDEX',sha(duplicate)):
            with self.assertRaises(image.ImageError): image.validate(duplicate,manifest,config)

    def test_wrong_config_platform_and_size_are_rejected(self):
        index,manifest,config=self.fixture(); config=encoded({'os':'linux','architecture':'arm64'})
        obj=json.loads(manifest); obj['config'].update(digest=sha(config),size=len(config)); manifest=encoded(obj)
        root=json.loads(index); root['manifests'][0].update(digest=sha(manifest),size=len(manifest)); index=encoded(root)
        with patch.object(image,'INDEX',sha(index)), patch.object(image,'MANIFEST',sha(manifest)), patch.object(image,'CONFIG',sha(config)):
            with self.assertRaises(image.ImageError): image.validate(index,manifest,config)
        obj['config']['size']+=1; manifest=encoded(obj)
        root['manifests'][0].update(digest=sha(manifest),size=len(manifest)); index=encoded(root)
        with patch.object(image,'INDEX',sha(index)), patch.object(image,'MANIFEST',sha(manifest)), patch.object(image,'CONFIG',sha(config)):
            with self.assertRaises(image.ImageError): image.validate(index,manifest,config)

    def test_redirects_are_strictly_fixed_and_drop_authorization(self):
        with self.assertRaises(image.ImageError): image._redirect('https://attacker.invalid/config')
        with self.assertRaises(image.ImageError): image._redirect('https://production.cloudfront.docker.com:444/x')
        self.assertEqual(image._redirect('https://production.cloudfront.docker.com/x?a=b'),
                         ('production.cloudfront.docker.com','/x?a=b'))

    def test_fixed_requests_drop_auth_on_blob_redirect(self):
        index,manifest,config=self.fixture(); calls=[]
        def fetch(host,path,headers,deadline,limit):
            calls.append((host,path,dict(headers)))
            if host=='auth.docker.io': return 200,None,b'{"token":"fixture-token"}'
            if path.endswith(image.INDEX): return 200,None,index
            if path.endswith(image.MANIFEST): return 200,None,manifest
            if host=='registry-1.docker.io': return 307,'https://production.cloudfront.docker.com/blob?signature=fixture',b''
            return 200,None,config
        with patch.object(image,'INDEX',sha(index)), patch.object(image,'MANIFEST',sha(manifest)), \
             patch.object(image,'CONFIG',sha(config)), patch.object(image,'_get',side_effect=fetch):
            self.assertEqual(image.collect(),{'index':index,'manifest':manifest,'config':config})
        self.assertEqual(calls[-1][0],'production.cloudfront.docker.com'); self.assertEqual(calls[-1][2],{})
        self.assertEqual(sum('Authorization' in c[2] for c in calls),3)

    def test_manifest_redirect_or_lost_response_never_produces_proof(self):
        def redirect(host,path,headers,deadline,limit):
            if host=='auth.docker.io': return 200,None,b'{"token":"fixture-token"}'
            return 307,'https://production.cloudfront.docker.com/blob',b''
        with patch.object(image,'_get',side_effect=redirect):
            with self.assertRaises(image.ImageError): image.collect()
        with patch.object(image,'_get',side_effect=TimeoutError('sensitive diagnostics')):
            with self.assertRaises(image.ImageError) as error: image.collect()
        self.assertNotIn('sensitive diagnostics',str(error.exception))

    @unittest.skipUnless(hasattr(os,'fork'),'Linux deadline worker required')
    def test_slow_drip_cannot_exceed_absolute_deadline(self):
        listener=socket.socket(); listener.bind(('127.0.0.1',0)); listener.listen(1); listener.settimeout(2)
        address=listener.getsockname(); stopped=threading.Event()
        def drip():
            try:
                writer,_=listener.accept()
                with writer:
                    while not stopped.wait(.02): writer.sendall(b'x')
            except OSError: pass
        thread=threading.Thread(target=drip); thread.start()
        def slow(*args):
            reader=socket.create_connection(address,timeout=2)
            stream=reader.makefile('rb'); stream.read(65536)
            return 200,None,b''
        owned=[]; fork=os.fork
        def start():
            pid=fork()
            if pid: owned.append(pid)
            return pid
        try:
            before=time.monotonic()
            with patch.object(image,'_get_network',side_effect=slow), patch.object(image.os,'fork',side_effect=start):
                with self.assertRaises(Exception): image._get('fixed','/fixed',{},time.monotonic()+.3,1024)
            elapsed=time.monotonic()-before; self.assertGreaterEqual(elapsed,.2); self.assertLess(elapsed,2)
            self.assertEqual(len(owned),1)
            with self.assertRaises(ChildProcessError): os.waitpid(owned[0],os.WNOHANG)
        finally:
            stopped.set(); thread.join(timeout=2); listener.close()
        self.assertFalse(thread.is_alive())


if __name__=='__main__': unittest.main()
