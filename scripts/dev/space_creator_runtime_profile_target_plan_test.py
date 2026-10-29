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
                           ('AUTH_SESSION_FLOOR_GRPC_PORT','9999'),('PATH','/foreign'),('S2S_SIGNING_KID','foreign')):
            source=self.auth(); source['spec']['template']['spec']['containers'][0]['env'].append({'name':name,'value':value})
            with self.subTest(name=name),self.assertRaises((ValueError,subject.auth.EligibilityError)):
                subject.render_auth(source,'ghcr.io/poryadok/voiceroot/auth@sha256:'+'a'*64,{}, {},[])

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
        for field,value in (('GATEWAY_AUTH_MODE','static'),('GATEWAY_STATIC_TOKENS_JSON','{"token":"user"}'),('GATEWAY_JWKS_URL','')):
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
            issuer=attach(subject.preflight.IssuerPreflightPhase.__new__(subject.preflight.IssuerPreflightPhase),'issuer')
            trust=attach(trust_effect.TrustEffectPhase.__new__(trust_effect.TrustEffectPhase),'trust')
            for item in (config,roots,issuer,trust): item.package=pkg
            config.inventory=inv; roots.inventory=inv; trust.inventory=inv; trust.jdk=roots; trust.preflight=issuer
            with mock.patch.object(subject.shared.media_operation_guard,'verify'):
                phase=subject.TargetPlanPhase(own.journal,own.fence,own.client,lock,'ns',pkg,inv,config,roots,issuer,trust,'/source')
                receipt={'config_sha256':subject.writer_cut.digest({'fixed':'config'}),'scope':'fixture'}
                targets={'auth':{'source':{},'config_target':{},'image_target':{}}}
                phase._evidence=mock.Mock(return_value=()); phase._capture=mock.Mock(return_value=(copy.deepcopy(receipt),copy.deepcopy(targets)))
                phase._package=mock.Mock(return_value={})
                self.assertEqual(phase.run('token','actor'),receipt)
                self.assertEqual(subject.verify_current(phase,'token','actor')['targets'],targets)
                with mock.patch.object(subject.observed_config,'verify_receipt',return_value={'fixed':'config'}):
                    self.assertEqual(subject.verify_receipt(phase,'token')['targets'],targets)
                phase.journal.state['steps'][subject.STEP]['private_targets']['auth']['config_target']['drift']=True
                with self.assertRaises(subject.TargetPlanError): subject.verify_current(phase,'token','actor')


if __name__=='__main__': unittest.main()
