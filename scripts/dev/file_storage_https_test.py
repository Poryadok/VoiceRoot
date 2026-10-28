import copy
import importlib.util
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch


def load(name):
    spec=importlib.util.spec_from_file_location(name,Path(__file__).with_name(name+'.py'))
    module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
    return module


storage=load('file_storage_https');tls=load('signal_tls')


class FileStorageHTTPSTests(unittest.TestCase):
    def test_companion_uses_existing_dev_host_tls_and_owned_fixed_target(self):
        obj=storage.ingress();root=tls.ingress(True,'api-dev.comrade.click')
        self.assertEqual(obj['metadata']['name'],'voice-dev-file-storage-https')
        self.assertNotEqual(obj['metadata']['name'],root['metadata']['name'])
        self.assertEqual(obj['metadata']['namespace'],'voice-dev')
        self.assertEqual(obj['metadata']['labels']['voice.environment'],'dev')
        self.assertEqual(obj['metadata']['labels']['app.kubernetes.io/managed-by'],'voice-dev')
        self.assertEqual(obj['spec']['tls'],root['spec']['tls'])
        self.assertEqual(obj['spec']['rules'][0]['host'],'api-dev.comrade.click')
        self.assertEqual(obj['metadata']['annotations'],{'traefik.ingress.kubernetes.io/router.entrypoints':'websecure'})
        self.assertEqual(obj['spec']['ingressClassName'],'traefik')

    def test_actual_non_strict_prefix_semantics_exclude_sibling_buckets(self):
        paths=storage.ingress()['spec']['rules'][0]['http']['paths']
        self.assertEqual([(r['path'],r['pathType']) for r in paths],
                         [('/voice-dev-files','Exact'),('/voice-dev-files/','Prefix')])
        for row in paths:
            self.assertEqual(row['backend']['service'],{'name':'voice-minio','port':{'number':9000}})
        # Traefik3.6.6 with strictPrefixMatching=false maps these to Path and
        # character-wise PathPrefix. A bare bucket prefix would fail this test.
        def selected(value):
            return any(value==r['path'] if r['pathType']=='Exact' else value.startswith(r['path']) for r in paths)
        for value in ('/voice-dev-files','/voice-dev-files/','/voice-dev-files/attachments/owned/file.png'):
            self.assertTrue(selected(value),value)
        for value in ('/','/health','/api/v1/files/upload','/voice-dev-files-other/owned','/voice-dev-files2'):
            self.assertFalse(selected(value),value)

    def test_canonical_renewal_leaves_companion_outside_its_object_selection(self):
        companion=storage.ingress();snapshot=copy.deepcopy(companion)
        known={tls.ingress(secure,host)['metadata']['name']:tls.ingress(secure,host)
               for host in tls.ROUTES for secure in (False,True)}
        class Client:
            def __init__(self):self.reads=[];self.writes=[]
            def get(self,kind,name):
                if kind=='secret':return tls.secret(b'cert-fixture',b'key-fixture')
                if kind!='ingress' or name not in known:
                    raise AssertionError('renewal attempted an unowned object selection')
                self.reads.append(name);return copy.deepcopy(known[name])
            def call(self,*args,body=None):self.writes.append((args,body))
        client=Client()
        with tempfile.TemporaryDirectory() as directory:
            state=Path(directory);lineage=state/'live'/tls.HOST;lineage.mkdir(parents=True)
            (lineage/'fullchain.pem').write_bytes(b'cert-fixture')
            (lineage/'privkey.pem').write_bytes(b'key-fixture')
            kubeconfig=state/'fixture.kubeconfig';kubeconfig.write_text('{}')
            with patch.object(tls,'STATE',state),patch.object(tls,'private_state'),patch.object(tls,'guard'), \
                 patch.object(tls,'Client',return_value=client),patch.object(tls,'validate_certificate'), \
                 patch.object(tls,'run',return_value=b'notAfter=fixture'):
                tls.operator('renew',kubeconfig)
        self.assertEqual(set(client.reads),set(known))
        self.assertNotIn(companion['metadata']['name'],client.reads)
        self.assertEqual(client.writes,[])
        self.assertEqual(companion,snapshot)

    def test_cli_is_render_only(self):
        tool=Path(__file__).with_name('file_storage_https.py')
        rendered=subprocess.run([sys.executable,str(tool),'render'],capture_output=True,text=True)
        self.assertEqual(rendered.returncode,0)
        rejected=subprocess.run([sys.executable,str(tool),'apply'],capture_output=True,text=True)
        self.assertNotEqual(rejected.returncode,0)


if __name__=='__main__':unittest.main()
