import unittest
import copy
import hashlib
import json
import base64
from unittest.mock import patch
from types import SimpleNamespace
import auth_live_migration_phase_eligibility as eligibility
from auth_live_migration_phase_eligibility import check_environment, check_database_route, resolve_environment, read_image_config, read_references, check_container, candidate_target, EligibilityError


class EligibilityTests(unittest.TestCase):
    def test_candidate_route_requires_exact_auth_database_and_role(self):
        env={'SPRING_DATASOURCE_URL':'jdbc:postgresql://voice-postgres:5432/auth_db'}
        self.assertEqual(check_database_route(env)['role'],'voice')
        for value in ('jdbc:postgresql://elsewhere:5432/auth_db','jdbc:postgresql://voice-postgres/user_db'):
            with self.assertRaises(EligibilityError): check_database_route({**env,'SPRING_DATASOURCE_URL':value})
        with self.assertRaises(EligibilityError): check_database_route({**env,'SPRING_DATASOURCE_USERNAME':'other'})
    def test_candidate_snapshot_rejects_deployment_drift(self):
        image='ghcr.io/poryadok/voiceroot/auth@sha256:'+'a'*64
        spec={'replicas':0,'template':{'spec':{'containers':[{'name':'auth','image':'old',
              'env':[{'name':'AUTH_FLYWAY_ENABLED','value':'false'},
                     {'name':'SPRING_DATASOURCE_URL','value':'jdbc:postgresql://voice-postgres:5432/auth_db'}]}]}}}
        meta={'name':'voice-auth','namespace':'voice-dev','uid':'uid','generation':3,'resourceVersion':'9'}
        plan={'deployment':{'uid':'uid','generation':3,'resourceVersion':'8','spec':spec},'platform_digest':'sha256:'+'b'*64}
        class Client:
            def __init__(self): self.journal=SimpleNamespace(intent={'auth_live_candidate':plan}); self.changed=False
            def call(self,*args):
                result={'metadata':copy.deepcopy(meta),'spec':copy.deepcopy(spec)}
                if self.changed: result['spec']['replicas']=1
                return result
        client=Client(); p=SimpleNamespace(journal=client.journal,client=client,
            fence=SimpleNamespace(context={'commit':'c'*40,'images':{'voice-auth/auth':image}},ledger=SimpleNamespace(client=client)))
        p.intent=copy.deepcopy(p.journal.intent)
        phase=SimpleNamespace(phases=p)
        with patch.object(eligibility,'KubectlTransport',Client),patch.object(eligibility,'_source_tree',return_value='tree'), \
             patch.object(eligibility.readback,'_guard'),patch.object(eligibility.publish_candidate,'Registry'), \
             patch.object(eligibility,'read_image_config',return_value={'env':{},'config_digest':'sha256:'+'d'*64}):
            result=eligibility.observe_candidate(phase,lambda:None,'.','actor','token')
            self.assertEqual(result['deployment']['resourceVersion'],'9')
            client.changed=True
            with self.assertRaises(EligibilityError): eligibility.observe_candidate(phase,lambda:None,'.','actor','token')
    def test_target_substitutes_only_auth_image(self):
        spec={'replicas':0,'template':{'spec':{'containers':[{'name':'auth','image':'old'},
               {'name':'sidecar','image':'sidecar'}]}}}
        target=candidate_target(spec,'ghcr.io/poryadok/voiceroot/auth@sha256:'+'a'*64)
        self.assertEqual(spec['template']['spec']['containers'][0]['image'],'old')
        self.assertEqual(target['template']['spec']['containers'][1]['image'],'sidecar')
        with self.assertRaises(EligibilityError): candidate_target({**spec,'replicas':1},'invalid')
    def test_candidate_rejects_external_spring_configuration_mounts_and_commands(self):
        container={'name':'auth','image':'ghcr.io/poryadok/voiceroot/auth@sha256:'+'a'*64}
        check_container(container)
        for extra in ({'command':['java']},{'args':['--spring.flyway.enabled=true']},
                      {'workingDir':'/tmp'},{'volumeMounts':[{'name':'x','mountPath':'/config/x'}]}):
            with self.assertRaises(EligibilityError): check_container({**container,**extra})
    def test_private_reference_snapshot_detects_race(self):
        container={'env':[{'name':'AUTH_FLYWAY_ENABLED','valueFrom':{'secretKeyRef':{'name':'s','key':'enabled'}}}]}
        obj={'metadata':{'name':'s','namespace':'voice-dev','uid':'uid','resourceVersion':'1'},
             'data':{'enabled':base64.b64encode(b'false').decode()}}
        class Client:
            def __init__(self): self.calls=0
            def call(self,*args): self.calls+=1; return obj
        client=Client(); snapshot,refs=read_references(client,container,lambda:None)
        self.assertEqual(snapshot[('secret','s')]['enabled'],'false')
        self.assertEqual(refs[0]['uid'],'uid')
        self.assertNotIn('false',json.dumps(refs))
        self.assertEqual(client.calls,2)
        class RacingClient:
            def __init__(self): self.calls=0
            def call(self,*args):
                self.calls+=1; changed=copy.deepcopy(obj)
                if self.calls==2: changed['metadata']['resourceVersion']='2'
                return changed
        with self.assertRaises(EligibilityError): read_references(RacingClient(),container,lambda:None)

    def test_registry_digest_binds_platform_and_image_configuration(self):
        encode=lambda x:json.dumps(x,separators=(',',':')).encode()
        digest=lambda x:'sha256:'+hashlib.sha256(x).hexdigest()
        config=encode({'os':'linux','architecture':'amd64','config':{'Env':['SERVER_PORT=8080'],
                      'Entrypoint':['java','-jar','/auth.jar'],'Cmd':None,'WorkingDir':'/'}})
        platform=encode({'schemaVersion':2,'config':{'digest':digest(config),'size':len(config)}})
        index=encode({'manifests':[{'digest':digest(platform),'size':len(platform),'platform':{'os':'linux','architecture':'amd64'}}]})
        blobs={'manifests/'+digest(index):index,'manifests/'+digest(platform):platform,'blobs/'+digest(config):config}
        class Registry:
            def get(self,repo,path): return blobs[path]
        result=read_image_config(Registry(),digest(index),digest(platform))
        self.assertEqual(result['env'],{'SERVER_PORT':'8080'})
        blobs['blobs/'+digest(config)]=config+b' '
        with self.assertRaises(EligibilityError): read_image_config(Registry(),digest(index),digest(platform))
    def test_effective_environment_kubernetes_precedence(self):
        container={'envFrom':[{'configMapRef':{'name':'one'}},{'configMapRef':{'name':'two'}}],
                   'env':[{'name':'AUTH_FLYWAY_ENABLED','value':'false'}]}
        sources={('configmap','one'):{'AUTH_FLYWAY_ENABLED':'true'},
                 ('configmap','two'):{'AUTH_PERSISTENCE':'jdbc'}}
        env=resolve_environment(container,sources)
        self.assertEqual(check_environment(env)['flyway'],'disabled')
        sources[('configmap','two')]['AUTH_ROLE_IDENTITY_SELECTOR_KEYS_DIR']='/keys'
        with self.assertRaises(EligibilityError): check_environment(resolve_environment(container,sources))
        sources[('secret','s')]={}
        container['env'].append({'name':'AUTH_ROLE_IDENTITY_SELECTOR_KEYS_DIR','valueFrom':
                                 {'secretKeyRef':{'name':'s','key':'missing','optional':True}}})
        with self.assertRaises(EligibilityError): check_environment(resolve_environment(container,sources))
        container['env'].append({'name':'AUTH_FLYWAY_ENABLED','value':'true'})
        with self.assertRaises(EligibilityError): resolve_environment(container,sources)

    def test_missing_or_unknown_reference_is_not_absence(self):
        for env in ([{'name':'AUTH_FLYWAY_ENABLED','valueFrom':{'secretKeyRef':{'name':'s','key':'x'}}}],
                    [{'name':'AUTH_FLYWAY_ENABLED','valueFrom':{'fieldRef':{'fieldPath':'metadata.name'}}}]):
            with self.assertRaises(EligibilityError): resolve_environment({'env':env},{})

    def test_dotted_envfrom_overrides_are_never_silently_dropped(self):
        for prefix,key in (('', 'spring.flyway.enabled'), ('spring.', 'config.location'),
                           ('auth.', 'role.identity.selector.keys.dir')):
            container={'envFrom':[{'prefix':prefix,'configMapRef':{'name':'c'}}],
                       'env':[{'name':'AUTH_FLYWAY_ENABLED','value':'false'}]}
            with self.subTest(prefix=prefix,key=key),self.assertRaises(EligibilityError):
                check_environment(resolve_environment(container,{('configmap','c'):{key:'true'}}))
    def test_explicit_disabled_flyway_and_default_jdbc(self):
        self.assertEqual(check_environment({'AUTH_FLYWAY_ENABLED':'false'}),
                         {'flyway':'disabled', 'persistence':'jdbc', 'selector':'absent'})

    def test_refuses_selector_and_external_override_channels(self):
        for extra in ({'AUTH_ROLE_IDENTITY_SELECTOR_KEYS_DIR':'/keys'},
                      {'spring.config.location':'file:/tmp/a'},
                      {'JDK_JAVA_OPTIONS':'-Dspring.flyway.enabled=true'},
                      {'SPRING_APPLICATION_JSON':'{}'}, {'AUTH_PERSISTENCE':'memory'},
                      {'SPRING_FLYWAY_ENABLED':'true'}, {'JAVA_TOOL_OPTIONS':'-Dspring.flyway.enabled=true'}):
            with self.subTest(extra=extra), self.assertRaises(EligibilityError):
                check_environment({'AUTH_FLYWAY_ENABLED':'false', **extra})

    def test_requires_explicit_disabled_setting_and_safe_java_options(self):
        for env in ({}, {'AUTH_FLYWAY_ENABLED':'true'}, {'AUTH_FLYWAY_ENABLED':'false',
                     'JAVA_TOOL_OPTIONS':'-XX:MaxRAMPercentage=75.0 -Djavax.net.ssl.trustStore=/trusted'}):
            if env.get('JAVA_TOOL_OPTIONS'):
                self.assertEqual(check_environment(env)['selector'], 'absent')
            else:
                with self.assertRaises(EligibilityError): check_environment(env)


if __name__=='__main__': unittest.main()
