import hashlib
import base64
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import space_p3_issuer_preflight_metadata_root as subject
import media_backup_phase_test as fixture


@unittest.skipUnless(os.name=='posix' and os.geteuid()==0,'isolated root Linux fixture')
class MetadataTest(unittest.TestCase):
    def test_tt_guard_rejects_candidate_and_uses_fixed_endpoint(self):
        base=fixture.PhaseTest('test_exact_terminal_receipt_and_no_replay')
        def cleanup():
            if not base.doCleanups(): raise RuntimeError('nested cleanup failed')
        self.addCleanup(cleanup); base.setUp()
        class Response:
            def __enter__(self): return self
            def __exit__(self,*args): pass
            def read(self,limit): return json.dumps({'valid':True,'resource':base.card}).encode()
        class Opener:
            def open(self,url,timeout):
                self_url.append(url); return Response()
        self_url=[]
        with patch.object(subject.urllib.request,'build_opener',return_value=Opener()):
            subject._tt(base.context)
            self.assertTrue(self_url[0].startswith(subject.TT+'/api/projects/voice/resources/DEV/check?'))
            base.card['candidate']['commit']='f'*40
            with self.assertRaises(ValueError): subject._tt(base.context)

    def test_fixed_subprocess_names_absence_and_transport_failure(self):
        with tempfile.TemporaryDirectory(dir='/root',prefix='space-metadata-') as tmp:
            directory=Path(tmp); binary=directory/'k3s'; config=directory/'config'
            binary.write_text('#!/bin/sh\nexit 0\n')
            binary.chmod(0o755); config.write_text('private-sentinel'); config.chmod(0o644)
            binary_fd=os.open(binary,os.O_RDONLY); config_fd=os.open(config,os.O_RDONLY)
            try: self.assertEqual(subject._read(binary_fd,config_fd,['get','namespace','voice-dev']),b'')
            finally: os.close(binary_fd); os.close(config_fd)
            digest=hashlib.sha256(binary.read_bytes()).hexdigest()
            subprocess.run(['openssl','req','-x509','-newkey','rsa:2048','-nodes','-days','2','-subj','/CN=fixture',
                '-keyout',str(directory/'key'),'-out',str(directory/'ca')],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
            pem=(directory/'ca').read_bytes(); der=subprocess.run(['openssl','x509','-in',str(directory/'ca'),'-outform','DER'],check=True,capture_output=True).stdout
            ca={'uid':'ca','resourceVersion':'1','der_sha256':hashlib.sha256(der).hexdigest()}
            context={'operation_id':'op'}; lock={'metadata':{'uid':'lock','resourceVersion':'2'},
                'data':{'operation':'x','resource_operation':'op','resource_context':json.dumps(context,sort_keys=True,separators=(',',':'))}}
            def read(binary_fd,config_fd,args):
                if 'namespace' in args: return json.dumps({'metadata':{'name':'voice-dev','uid':'ns','labels':subject.LABELS}}).encode()
                if 'configmap' in args: return json.dumps({'metadata':lock['metadata']|{'name':'voice-dev-image-update-lock','namespace':'voice-dev','labels':subject.LABELS},'data':lock['data']}).encode()
                if 'voice-media-ca' in args: return b'ca\n1\n'+base64.b64encode(pem)
                if 'voice-space-media-client-tls' in args: return b'uid\nrv\n\ntls.crt\ntls.key\n'
                return b''
            with patch.object(subject,'K3S',binary),patch.object(subject,'KUBECONFIG',config),patch.object(subject,'K3S_SHA256',digest), \
                 patch.object(subject,'_tt'),patch.object(subject,'_read',side_effect=read):
                result=subject.observe('ns',context,lock,ca)
                self.assertEqual(result['voice-space-media-client-tls']['keys'],['tls.crt','tls.key'])
                self.assertEqual(result['voice-gateway-principal-signing'],{'state':'absent'})
                self.assertNotIn('private-sentinel',str(result))
                binary.write_text('#!/bin/sh\nexit 1\n'); binary.chmod(0o755)
                with self.assertRaises(ValueError): subject.observe('ns',context,lock,ca)


if __name__=='__main__': unittest.main()
