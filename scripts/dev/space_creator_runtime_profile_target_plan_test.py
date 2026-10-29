import copy
from pathlib import Path
import tempfile
import types
import unittest
from unittest import mock

import space_creator_runtime_profile_target_plan as subject
import frontend_runtime_executor as transport
import full_cohort_inventory_phase as inventory
import full_cohort_package_phase as package
import media_backup_phase_test as fixture
import media_operation_journal as journals
import resource_fencing
import space_creator_runtime_profile_trust_effect as trust_effect


def workload(name):
    return {'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':'voice-'+name,'namespace':'voice-dev',
            'uid':'uid-'+name,'resourceVersion':'1','generation':1},'spec':{'replicas':2,'template':{'spec':{
            'containers':[{'name':name,'image':'old','env':[],'volumeMounts':[]}],'volumes':[]}}}}


class RenderTest(unittest.TestCase):
    def auth(self):
        source=workload('auth'); source['spec']['template']['spec']['containers'][0]['env']=[
            {'name':'AUTH_FLYWAY_ENABLED','value':'false'},
            {'name':'SPRING_DATASOURCE_URL','value':'jdbc:postgresql://voice-postgres:5432/auth_db'}]
        return source

    def test_auth_target_only_fixed_creator_and_preserves_source(self):
        source=self.auth(); original=copy.deepcopy(source)
        target,route=subject.render_auth(source,'ghcr.io/poryadok/voiceroot/auth@sha256:'+'a'*64,{}, {},[])
        self.assertEqual(source,original); self.assertEqual(target['replicas'],0); self.assertEqual(route['database'],'auth_db')
        container=target['template']['spec']['containers'][0]
        self.assertEqual(container['image'],'ghcr.io/poryadok/voiceroot/auth@sha256:'+'a'*64)
        self.assertIn({'name':'session-floor','containerPort':9092,'protocol':'TCP'},container['ports'])
        names={v['name']:v for v in container['env']}
        self.assertEqual(names['AUTH_SESSION_FLOOR_GRPC_PORT']['value'],'9092')
        self.assertIn(subject.TRUST,names['JAVA_TOOL_OPTIONS']['value'])
        self.assertEqual({x['name'] for x in target['template']['spec']['volumes']},{'media-server','media-ca','media-trust'})

    def test_auth_inherited_override_and_jvm_injection_hold(self):
        for name,value in (('_JAVA_OPTIONS','-Dfoo=bar'),('SPRING_CONFIG_IMPORT','file:/tmp/x'),
                           ('JAVA_TOOL_OPTIONS','-Djavax.net.ssl.trustStore=/foreign'),
                           ('AUTH_SESSION_FLOOR_GRPC_PORT','9999'),('PATH','/foreign'),('S2S_SIGNING_KID','foreign'),
                           ('SPRING_DATA_REDIS_URL','redis://foreign:6379'),('SPRING_DATA_REDIS_DATABASE','1'),
                           ('SPRING_DATA_REDIS_SENTINEL_MASTER','foreign'),('SPRING_DATA_REDIS_PASSWORD','private-sentinel')):
            source=self.auth(); source['spec']['template']['spec']['containers'][0]['env'].append({'name':name,'value':value})
            with self.subTest(name=name),self.assertRaises((ValueError,subject.auth.EligibilityError)):
                subject.render_auth(source,'ghcr.io/poryadok/voiceroot/auth@sha256:'+'a'*64,{}, {},[])

    def test_auth_redis_password_reference_is_preserved_without_disclosure(self):
        source=self.auth(); ref={'name':'SPRING_DATA_REDIS_PASSWORD','valueFrom':{'secretKeyRef':{'name':'voice-app-secrets','key':'REDIS_PASSWORD'}}}
        source['spec']['template']['spec']['containers'][0]['env'].append(copy.deepcopy(ref))
        target,_=subject.render_auth(source,'ghcr.io/poryadok/voiceroot/auth@sha256:'+'a'*64,
                                    {('secret','voice-app-secrets'):{'REDIS_PASSWORD':'private-sentinel'}},{},[])
        self.assertIn(ref,target['template']['spec']['containers'][0]['env']); self.assertNotIn('private-sentinel',str(target))

    def test_existing_exact_config_refs_are_preserved_without_values(self):
        source=self.auth(); entries=source['spec']['template']['spec']['containers'][0]['env']
        for name in ('JAVA_TOOL_OPTIONS','S2S_JWKS_URLS_JSON'):
            entries.append({'name':name,'valueFrom':{'configMapKeyRef':{'name':'voice-app-config','key':name}}})
        values={'JAVA_TOOL_OPTIONS':' '.join(subject.TRUST_OPTIONS),
                'S2S_JWKS_URLS_JSON':'{"gateway":"'+subject.GATEWAY_JWKS+'","space":"'+subject.SPACE_JWKS+'"}'}
        target,_=subject.render_auth(source,'ghcr.io/poryadok/voiceroot/auth@sha256:'+'a'*64,
                                     {('configmap','voice-app-config'):values},{},list(values))
        selected=[x for x in target['template']['spec']['containers'][0]['env'] if x['name'] in values]
        self.assertEqual(selected,entries[-2:]); self.assertNotIn(subject.GATEWAY_JWKS,str(selected))
        values['JAVA_TOOL_OPTIONS']='-XX:MaxRAMPercentage=75.0'
        with self.assertRaises(ValueError):
            subject.render_auth(source,'ghcr.io/poryadok/voiceroot/auth@sha256:'+'a'*64,
                                {('configmap','voice-app-config'):values},{},list(values))

    def test_gateway_target_and_command_override_hold(self):
        source=workload('gateway'); original=copy.deepcopy(source)
        target=subject.render_gateway(source,'ghcr.io/poryadok/voiceroot/gateway@sha256:'+'b'*64,[])
        self.assertEqual(source,original); self.assertEqual(target['replicas'],0)
        self.assertEqual(target['template']['spec']['containers'][0]['image'],'ghcr.io/poryadok/voiceroot/gateway@sha256:'+'b'*64)
        source['spec']['template']['spec']['containers'][0]['command']=['/bin/sh']
        with self.assertRaises(ValueError): subject.render_gateway(source,'ghcr.io/poryadok/voiceroot/gateway@sha256:'+'b'*64,[])

    def test_gateway_source_requires_jwks_and_no_static_bypass(self):
        values={key:subject.observed_config.EXPECTED[key] for key in ('GATEWAY_JWKS_URL','GATEWAY_JWT_ISSUER','GATEWAY_JWT_AUDIENCE')}
        values['GATEWAY_REDIS_ADDR']='voice-redis:6379'; subject._check_gateway_environment(values)
        for field,value in (('GATEWAY_AUTH_MODE','static'),('GATEWAY_STATIC_TOKENS_JSON','{"token":"user"}'),('GATEWAY_JWKS_URL',''),('GATEWAY_REDIS_ADDR','foreign:6379')):
            bad=dict(values); bad[field]=value
            with self.subTest(field=field),self.assertRaises(ValueError): subject._check_gateway_environment(bad)


class PhaseTest(unittest.TestCase):
    def test_capture_derives_two_targets_from_inventory_and_bound_images(self):
        phase=subject.TargetPlanPhase.__new__(subject.TargetPlanPhase)
        auth_obj=RenderTest().auth(); gateway_obj=workload('gateway')
        gateway_obj['spec']['template']['spec']['containers'][0]['env']=[
            {'name':name,'value':subject.observed_config.EXPECTED[name]} for name in
            ('GATEWAY_JWKS_URL','GATEWAY_JWT_ISSUER','GATEWAY_JWT_AUDIENCE')]+[{'name':'GATEWAY_REDIS_ADDR','value':'voice-redis:6379'}]
        phase.fence=types.SimpleNamespace(context={'commit':'a'*40,'operation_id':'op'}); phase.namespace_uid='ns'; phase.source_root='/source'
        phase._guard=mock.Mock(); phase.client=types.SimpleNamespace(call=mock.Mock(return_value={'metadata':{'name':'voice-app-config',
            'namespace':'voice-dev','uid':'cm','resourceVersion':'1'},'data':{}}))
        pkg={'images':{'voice-auth/auth':'ghcr.io/poryadok/voiceroot/auth@sha256:'+'b'*64,
                       'voice-gateway/gateway':'ghcr.io/poryadok/voiceroot/gateway@sha256:'+'c'*64},
             'platform_digests':{'voice-auth/auth':'sha256:'+'d'*64,'voice-gateway/gateway':'sha256:'+'e'*64}}
        observed={'projection':{'auth':{'uid':'a'},'gateway':{'uid':'g'}}}; evidence=(pkg,observed,{}, {'config_digest':'sha256:'+'f'*64},{},{})
        phase._evidence=mock.Mock(return_value=evidence)
        with mock.patch.object(subject.publish_candidate,'source_snapshot',return_value={'git_tree_sha256':'t'}), \
             mock.patch.object(subject.inventory,'inventory',return_value=({}, {'voice-auth':auth_obj,'voice-gateway':gateway_obj},observed['projection'])), \
             mock.patch.object(subject.publish_candidate,'Registry'), \
             mock.patch.object(subject.auth,'read_image_config',return_value={'config_digest':'sha256:'+'f'*64,'env':{}}), \
             mock.patch.object(subject,'_gateway_image',return_value={'config_digest':'sha256:'+'1'*64,'env':{}}), \
             mock.patch.object(subject.auth,'read_references',return_value=({},[])):
            receipt,targets=phase._capture('token','actor')
        for service in ('auth','gateway'):
            config=targets[service]['config_target']; image=targets[service]['image_target']
            self.assertEqual(config['template']['spec']['containers'][0]['image'],'old')
            config['template']['spec']['containers'][0]['image']=pkg['images']['voice-'+service+'/'+service]
            self.assertEqual(config,image)
        self.assertNotIn('SPRING_DATASOURCE_URL',str(receipt)); self.assertEqual(receipt['auth_database']['database'],'auth_db')

    def test_private_plan_journal_and_ref_drift(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp); base=fixture.PhaseTest('test_exact_terminal_receipt_and_no_replay')
            self.addCleanup(lambda:self.assertTrue(base.doCleanups())); base.setUp(); context=base.context; lock=base.lock
            intent={'resource_context':context,'namespace_uid':'ns','creator_target_plan':{'source_root':'/source','source_sha':context['commit']}}
            backing=base.ledger
            def attach(obj,name):
                obj.journal=journals.Journal(root/name,intent); self.addCleanup(obj.journal.close)
                obj.client=transport.KubectlTransport(obj.journal); ledger=resource_fencing.KubeLedger(obj.client)
                ledger.read=backing.read; ledger.write=backing.write
                obj.fence=resource_fencing.Fence(context,lambda:copy.deepcopy(base.card),ledger)
                obj.lock=lock; obj.namespace_uid='ns'; return obj
            own=attach(types.SimpleNamespace(),'own'); pkg=attach(package.PackagePhase.__new__(package.PackagePhase),'package')
            inv=attach(inventory.InventoryPhase.__new__(inventory.InventoryPhase),'inventory'); inv.package=pkg
            config=attach(subject.observed_config.CreatorConfigPhase.__new__(subject.observed_config.CreatorConfigPhase),'config')
            roots=attach(subject.jdk.TargetJdkPhase.__new__(subject.jdk.TargetJdkPhase),'jdk')
            issuer=attach(subject.preflight.PeerIssuerPreflightPhase.__new__(subject.preflight.PeerIssuerPreflightPhase),'issuer'); issuer.profile='creator'
            trust_refs=attach(types.SimpleNamespace(),'trust')
            target_refs=attach(types.SimpleNamespace(),'target-trust')
            for item in (config,roots,issuer): item.package=pkg
            config.inventory=inv; roots.inventory=inv
            issuer.step='dev-peer-issuer-preflight-creator'; issuer.ledger='dev_peer_issuer_preflight_creator'
            with mock.patch.object(subject.shared.media_operation_guard,'verify'):
                trust=trust_effect.TrustEffectPhase(trust_refs.journal,trust_refs.fence,trust_refs.client,lock,'ns',pkg,inv,roots,issuer)
                from space_creator_runtime_profile_target_trust import TargetTrustPhase
                target_trust=TargetTrustPhase(target_refs.journal,target_refs.fence,target_refs.client,lock,'ns',pkg,inv,roots,issuer)
                self.assertIs(target_trust.preflight,issuer)
                phase=subject.TargetPlanPhase(own.journal,own.fence,own.client,lock,'ns',pkg,inv,config,roots,issuer,trust,'/source')
                issuer_receipt={'ca':{'uid':'ca','resourceVersion':'1','der_sha256':'a'*64},'creator_secrets':{
                    name:{'state':'absent'} for name in ('voice-auth-media-server-tls','voice-gateway-media-client-tls','voice-gateway-principal-signing')}}
                jdk_receipt={'image':'image','root_fingerprints':['b'*64]}
                prepared={'mode':'create','old':None,'media_ca':issuer_receipt['ca'],
                    'jdk_receipt_sha256':subject.writer_cut.digest(jdk_receipt),'issuer_receipt_sha256':subject.writer_cut.digest(issuer_receipt),
                    'root_count':1,'operation_id':context['operation_id'],'source_sha':context['commit'],'namespace_uid':'ns'}
                trust.record={'status':'prepared','plan_sha256':subject.writer_cut.digest(prepared)}
                trust.journal.state['steps'][trust_effect.STEP]=copy.deepcopy(trust.record)
                trust.journal.state['steps'][trust_effect.PLAN_STEP]=copy.deepcopy(prepared)
                backing.row[trust_effect.LEDGER]=copy.deepcopy(trust.record)
                trust._guard=mock.Mock(); trust._ledger=backing.read; trust._dependencies=mock.Mock(return_value=(jdk_receipt,issuer_receipt))
                trust._ca=mock.Mock(); trust._read_store=mock.Mock(return_value=(None,None)); trust._key=mock.Mock(return_value=b'key')
                trust._locals=mock.Mock(); phase._inventory=mock.Mock(return_value={'projection':{}})
                phase._package=mock.Mock(return_value={'images':{'voice-auth/auth':'image'}})
                with mock.patch.object(subject.observed_config,'verify_completed',return_value={}), \
                     mock.patch.object(subject.jdk,'verify_completed',return_value=jdk_receipt), \
                     mock.patch.object(subject.preflight,'verify_current',return_value=issuer_receipt):
                    self.assertEqual(phase._evidence('token')[-1],prepared)
                    trust.journal.state['steps'][trust_effect.PLAN_STEP]['mode']='replace'
                    with self.assertRaises(trust_effect.TrustEffectError): phase._evidence('token')
                    trust.journal.state['steps'][trust_effect.PLAN_STEP]=copy.deepcopy(prepared)
                receipt={'config_sha256':subject.writer_cut.digest({'fixed':'config'}),'scope':'fixture'}
                targets={'auth':{'source':{},'config_target':{},'image_target':{}}}
                phase._evidence=mock.Mock(return_value=()); phase._capture=mock.Mock(return_value=(copy.deepcopy(receipt),copy.deepcopy(targets)))
                phase._package=mock.Mock(return_value={})
                self.assertEqual(phase.run('token','actor'),receipt)
                self.assertEqual(subject.verify_current(phase,'token','actor')['targets'],targets)
                with mock.patch.object(subject.observed_config,'verify_receipt',return_value={'fixed':'config'}):
                    self.assertEqual(subject.verify_receipt(phase,'token')['targets'],targets)
                def tamper(*args):
                    phase.journal.state['steps'][subject.STEP]['private_targets']['auth']['config_target']['drift']=True
                    return copy.deepcopy(receipt),copy.deepcopy(targets)
                phase._capture.side_effect=tamper
                with self.assertRaises(subject.TargetPlanError): subject.verify_current(phase,'token','actor')
                phase._capture.side_effect=None
                phase.journal.state['steps'][subject.STEP]['private_targets']['auth']['config_target']['drift']=True
                with self.assertRaises(subject.TargetPlanError): subject.verify_current(phase,'token','actor')


if __name__=='__main__': unittest.main()
