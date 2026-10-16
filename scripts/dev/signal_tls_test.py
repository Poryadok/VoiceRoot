import base64
import importlib.util
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest.mock import patch

spec=importlib.util.spec_from_file_location('signal_tls',Path(__file__).with_name('signal_tls.py'))
tls=importlib.util.module_from_spec(spec)
spec.loader.exec_module(tls)


class SignalTLSTests(unittest.TestCase):
    def test_hook_rejects_foreign_host_and_unsafe_token(self):
        for token in ('../secret','a/b','', '.', 'x;rm', 'a'*129):
            with self.assertRaises(ValueError):
                tls.challenge({'CERTBOT_DOMAIN':tls.HOST,'CERTBOT_TOKEN':token})
        with self.assertRaises(ValueError):
            tls.challenge({'CERTBOT_DOMAIN':'livekit.comrade.click','CERTBOT_TOKEN':'safe'})
        self.assertEqual(tls.challenge({'CERTBOT_DOMAIN':tls.HOST,'CERTBOT_TOKEN':'safe_123-'}),'safe_123-')

    def test_routes_are_fixed_to_dev_host_and_selected_backends(self):
        http=tls.ingress(False);https=tls.ingress(True)
        for obj in (http,https):
            self.assertEqual(obj['metadata']['namespace'],'voice-dev')
            self.assertEqual(obj['spec']['rules'][0]['host'],'livekit-dev.comrade.click')
        self.assertEqual(http['spec']['rules'][0]['http']['paths'][0]['path'],'/.well-known/acme-challenge/')
        self.assertEqual(http['spec']['rules'][0]['http']['paths'][0]['backend']['service'],{'name':'voice-web','port':{'number':80}})
        self.assertEqual(https['spec']['tls'],[{'hosts':[tls.HOST],'secretName':tls.SECRET}])
        self.assertEqual(https['spec']['rules'][0]['http']['paths'][0]['backend']['service'],{'name':'voice-livekit','port':{'number':7880}})

    def test_existing_foreign_secret_is_not_adopted(self):
        class Client:
            def __init__(self): self.writes=[]
            def get(self,kind,name): return {'metadata':{'namespace':tls.NS,'name':name,'labels':{}}}
            def call(self,*args,body=None): self.writes.append(args)
        client=Client()
        with self.assertRaises(ValueError): tls.publish(client,tls.secret(b'cert',b'key'))
        self.assertEqual(client.writes,[])
        with self.assertRaises(ValueError): tls.owned({'metadata':{'name':tls.NS,'labels':{}}},namespace=True)

    def test_subprocess_failure_never_exports_secret_stdout_or_stderr(self):
        with patch.object(tls.subprocess,'run',return_value=SimpleNamespace(returncode=1,stdout=b'PRIVATE-SECRET',stderr=b'PRIVATE-SECRET')):
            with self.assertRaises(RuntimeError) as caught:
                tls.run(['kubectl','get','secret',tls.SECRET])
        self.assertNotIn('PRIVATE-SECRET',str(caught.exception))

    def test_owned_replace_binds_uid_and_resource_version(self):
        class Client:
            def get(self,kind,name): return {'metadata':{'name':name,'namespace':tls.NS,'labels':dict(tls.LABELS),'uid':'owned','resourceVersion':'8'}}
            def call(self,*args,body=None): self.args=args;self.body=body
        client=Client();tls.publish(client,tls.secret(b'cert',b'key'))
        self.assertEqual(client.args[0],'replace')
        self.assertEqual(client.body['metadata']['uid'],'owned')
        self.assertEqual(client.body['metadata']['resourceVersion'],'8')
        self.assertEqual(base64.b64decode(client.body['data']['tls.key']),b'key')

    def test_hook_cleanup_removes_only_exact_validated_public_file(self):
        class Client:
            def __init__(self): self.calls=[]
            def get(self,kind,name):
                return {'metadata':{'name':name,'namespace':tls.NS,'labels':dict(tls.LABELS)}}
            def call(self,*args,body=None): self.calls.append(args)
        client=Client()
        with patch.dict(tls.os.environ,{'CERTBOT_DOMAIN':tls.HOST,'CERTBOT_TOKEN':'safe_token'},clear=True):
            tls.hook(client,cleanup=True)
        self.assertEqual(client.calls,[('exec','deployment/voice-web','-c','web','--','rm','-f','--',tls.WEBROOT+'/safe_token')])

    def test_client_binds_explicit_kubeconfig_and_captures_secret_output(self):
        with patch.object(tls.subprocess,'run',return_value=SimpleNamespace(returncode=0,stdout=b'{}',stderr=b'')) as proc:
            tls.Client('/private/operator.kubeconfig').get('secret',tls.SECRET)
        command=proc.call_args.args[0]
        self.assertEqual(command[:5],['kubectl','--kubeconfig','/private/operator.kubeconfig','--namespace','voice-dev'])
        self.assertTrue(proc.call_args.kwargs['capture_output'])

    def test_unchanged_certificate_is_not_replaced_on_renew(self):
        obj=tls.secret(b'cert',b'key')
        class Client:
            def __init__(self): self.calls=[]
            def get(self,kind,name): return obj
            def call(self,*args,body=None): self.calls.append(args)
        client=Client();tls.publish(client,tls.secret(b'cert',b'key'))
        self.assertEqual(client.calls,[])


if __name__=='__main__': unittest.main()
