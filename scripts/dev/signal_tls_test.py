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

    def test_https_file_bucket_preserves_signed_path_and_gateway_fallback(self):
        obj=tls.ingress(True,'api-dev.comrade.click')
        paths=obj['spec']['rules'][0]['http']['paths']
        self.assertEqual(paths,[
            {'path':'/','pathType':'Prefix','backend':{'service':{'name':'voice-gateway','port':{'number':8080}}}},
            {'path':'/voice-dev-files','pathType':'Prefix','backend':{'service':{'name':'voice-minio','port':{'number':9000}}}},
        ])
        self.assertEqual(obj['spec']['tls'],[{'hosts':['api-dev.comrade.click'],'secretName':tls.SECRET}])
        self.assertEqual(obj['metadata']['annotations'],{'traefik.ingress.kubernetes.io/router.entrypoints':'websecure'})
        def backend(path):
            matching=[row for row in paths if row['path']=='/' or path==row['path'] or path.startswith(row['path']+'/')]
            return max(matching,key=lambda row:len(row['path']))['backend']['service']['name']
        for path in ('/voice-dev-files','/voice-dev-files/attachments/owned/file.png'):
            self.assertEqual(backend(path),'voice-minio')
        for path in ('/api/v1/files/upload','/health','/voice-dev-files-other/owned'):
            self.assertEqual(backend(path),'voice-gateway')

    def test_file_bucket_route_does_not_change_acme_or_other_https_hosts(self):
        for host in tls.ROUTES:
            challenge=tls.ingress(False,host)
            self.assertEqual(len(challenge['spec']['rules'][0]['http']['paths']),1)
            self.assertEqual(challenge['spec']['rules'][0]['http']['paths'][0]['path'],'/.well-known/acme-challenge/')
            if host!='api-dev.comrade.click':
                paths=tls.ingress(True,host)['spec']['rules'][0]['http']['paths']
                self.assertEqual(len(paths),1)
                self.assertEqual(paths[0]['path'],'/')

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

    def test_exact_five_hosts_route_https_and_all_challenges_to_web(self):
        expected={'livekit-dev.comrade.click':('voice-livekit',7880),
                  'dev.comrade.click':('voice-web',80),'api-dev.comrade.click':('voice-gateway',8080),
                  'admin-dev.comrade.click':('voice-admin',80),
                  'developers-dev.comrade.click':('voice-developer-portal',80)}
        names=set()
        for host,(service,port) in expected.items():
            self.assertEqual(tls.challenge({'CERTBOT_DOMAIN':host,'CERTBOT_TOKEN':'safe'}),'safe')
            for secure in (False,True):
                obj=tls.ingress(secure,host=host)
                names.add(obj['metadata']['name'])
                self.assertEqual(obj['metadata']['namespace'],'voice-dev')
                self.assertEqual(obj['spec']['rules'][0]['host'],host)
                backend=obj['spec']['rules'][0]['http']['paths'][0]['backend']['service']
                self.assertEqual(backend,{'name':service if secure else 'voice-web','port':{'number':port if secure else 80}})
                if secure:self.assertEqual(obj['spec']['tls'],[{'hosts':[host],'secretName':tls.SECRET}])
        self.assertEqual(len(names),10)
        self.assertEqual(tls.ingress(True)['metadata']['name'],'voice-dev-signaling-https')
        for host in ('livekit.comrade.click','voice.comrade.click','app.comrade.click','other.example.org','*.comrade.click'):
            with self.assertRaises(ValueError):tls.ingress(True,host)
            with self.assertRaises(ValueError):tls.challenge({'CERTBOT_DOMAIN':host,'CERTBOT_TOKEN':'safe'})

    def test_expand_requires_owned_existing_secret_and_requests_exact_sans(self):
        with self.assertRaises(ValueError):tls.validate_action('expand',None,False)
        with self.assertRaises(ValueError):tls.validate_action('expand',{'metadata':{'namespace':tls.NS}},False)
        tls.validate_action('expand',tls.secret(b'cert',b'key'),False)
        for action in ('issue','expand'):
            args=tls.certbot_arguments(action,agree_tos=True)
            self.assertEqual(args[0],'certonly')
            domains=[args[i+1] for i,value in enumerate(args[:-1]) if value=='-d']
            self.assertEqual(set(domains),set(tls.ROUTES))
            self.assertEqual(len(domains),5)
            self.assertEqual(args[args.index('--cert-name')+1],tls.HOST)
            self.assertEqual('--expand' in args,action=='expand')
        renewal=tls.certbot_arguments('renew')
        self.assertEqual(renewal[0],'renew')
        self.assertNotIn('-d',renewal)
        self.assertEqual(renewal[renewal.index('--cert-name')+1],tls.HOST)

    def test_certificate_checks_cover_all_five_hosts_before_publication(self):
        with patch.object(tls,'run',return_value=b'public-key') as command:
            tls.validate_certificate(Path('/private/fullchain.pem'),Path('/private/key.pem'))
        hosts=[args.args[0][args.args[0].index('-checkhost')+1] for args in command.call_args_list if '-checkhost' in args.args[0]]
        self.assertEqual(set(hosts),set(tls.ROUTES))


if __name__=='__main__': unittest.main()
