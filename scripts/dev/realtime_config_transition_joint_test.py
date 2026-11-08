"""Real mixed WriterCut/Journals with synthetic producer and Kubernetes inputs."""
import copy
import sys
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import backend_config_transition as backend_service
import backend_config_transition_deployment as backend
import backend_config_transition_probe as probe
import media_operation_journal as journal
import realtime_config_transition_deployment as p3
import realtime_config_transition_service as chat_service
import realtime_config_transition_space_service as space_service
import realtime_config_transition_authority as authority
import realtime_config_transition_authority_test as authority_fixture
import realtime_p3_create_only as creation
import realtime_migration_job as migration
import realtime_config_transition_space_plan_phase as space
import realtime_jwks_publisher as publisher
import writer_cut
import writer_cut_test as fixture


class Session: pass
class Migration: pass
class Space: pass
class Publisher: pass


class MixedTest(unittest.TestCase):
    def setUp(self):
        self.base=fixture.WriterCutTest('test_drain_and_resume_use_current_template_not_old')
        self.addCleanup(self.cleanup,self.base); self.base.setUp()
        self.names=['chat','notification','realtime','space','user']
        sources=[]
        for name in self.names:
            item=fixture.workload('Deployment','voice-'+name,'uid-'+name,1,'old-'+name)
            item['spec']['template']['metadata']={'labels':{'app':'voice-'+name}}
            item['spec']['template']['spec']['containers'][0]['name']=name
            sources.append(item)
        self.base.admission['controllers']=copy.deepcopy(sources)
        self.base.client.workloads={(x['kind'],x['metadata']['name']):copy.deepcopy(x) for x in sources}
        self.base.journal.intent['writer_cut']=copy.deepcopy(self.base.admission)
        realtime_service={'apiVersion':'v1','kind':'Service','metadata':{'name':'voice-realtime','namespace':'voice-dev','uid':'rs','resourceVersion':'1'},
            'spec':{'type':'ClusterIP','clusterIP':'10.0.0.1','selector':{'app':'voice-realtime'},'ports':[{'name':'http','port':8080}]}}
        chat={'apiVersion':'v1','kind':'Service','metadata':{'name':'voice-chat','namespace':'voice-dev','uid':'cs','resourceVersion':'1'},
            'spec':{'type':'ClusterIP','clusterIP':'10.0.0.2','selector':{'app':'voice-chat'},'ports':[{'name':'http','port':8080}]}}
        space_svc={'apiVersion':'v1','kind':'Service','metadata':{'name':'voice-space','namespace':'voice-dev','uid':'ss','resourceVersion':'1'},
            'spec':{'type':'ClusterIP','clusterIP':'10.0.0.3','selector':{'app':'voice-space'},'ports':[{'name':'http','port':8080}]}}
        self.base.journal.intent['backend_config_service']=copy.deepcopy(realtime_service)
        self.base.journal.state['intent']=copy.deepcopy(self.base.journal.intent)
        self.base.cut.admission=copy.deepcopy(self.base.admission); self.base.cut.intent=copy.deepcopy(self.base.journal.intent)
        self.base.cut.drain()
        self.services={'voice-chat':chat,'voice-realtime':realtime_service,'voice-space':space_svc}; self.old_call=self.base.client.call
        self.authority=authority_fixture.AuthorityTest('test_derives_renderer_input_without_private_values')
        self.addCleanup(self.cleanup,self.authority); self.authority.setUp()
        module=sys.modules['realtime_jwks_publisher']; original_class=getattr(module,'PublisherPhase',None)
        def restore_class():
            if original_class is None: delattr(module,'PublisherPhase')
            else: module.PublisherPhase=original_class
        self.addCleanup(restore_class); module.PublisherPhase=Publisher
        sys.modules['realtime_config_transition_service']=chat_service
        self.authority.cut=self.base.cut; self.authority.publisher.fence=self.base.fence
        self.authority.publisher.lock=self.base.lock; self.authority.publisher.namespace_uid='ns'
        self.authority.published['operation_id']=self.base.context['operation_id']
        self.authority.p3['public_sources']['voice-principal-ca']={'uid':'principal-ca','resourceVersion':'1','der_sha256':'a'*64}
        self.authority.config['data']['REDIS_ADDR']='voice-redis:6379'
        original_call=self.old_call
        def call(*args):
            if args[1]=='service': return copy.deepcopy(self.services[args[2]])
            if args[1]=='configmap' and args[2]=='voice-app-config': return copy.deepcopy(self.authority.config)
            if args[1]=='secret': return copy.deepcopy(self.authority.values['Secret',args[2]])
            return original_call(*args)
        self.call_patch=patch.object(self.base.client,'call',side_effect=call); self.call_patch.start(); self.addCleanup(self.call_patch.stop)
        self.base.effect_patch.stop()
        self.calls=[]
        def apply(kind,name,operations):
            self.calls.append((kind,name))
            if kind=='Service':
                current=self.services[name]
            else: current=self.base.client.workloads[kind,name]
            if (operations[0]['value']!=current['metadata']['uid'] or operations[1]['value']!=current['metadata']['resourceVersion']
                    or operations[2]['value']!=current['spec']): raise ValueError('CAS')
            current['spec']=copy.deepcopy(operations[3]['value']); current['metadata']['resourceVersion']=str(int(current['metadata']['resourceVersion'])+1)
            if kind=='Deployment':
                current['metadata']['generation']+=1
                current['status']={'observedGeneration':current['metadata']['generation'],'replicas':0,'readyReplicas':0}
            return copy.deepcopy(current)
        self.effect=patch.object(self.base.cut,'_patch',side_effect=apply); self.effect.start(); self.addCleanup(self.effect.stop)
        backend_service.ServiceTransition(self.base.journal,self.base.fence,self.base.client,self.base.lock,'ns',self.base.cut).apply_realtime_service()
        targets=[{'workload':n,'pod_name':'voice-'+n+'-123','pod_uid':n+'-pod','deployment_uid':'uid-'+n,
                  'container_id':n+'-container','restart_count':0} for n in backend.SERVICES]
        observations={}
        for item in targets:
            n=item['workload']; observations[n]={'status':'observed','identity':{**{k:item[k] for k in ('pod_name','pod_uid','deployment_uid','container_id','restart_count')},
                'namespace_uid':'ns','deployment_rv':'1'},'fields':{k:{'state':'absent','origin':None} for k in probe.FIELDS[n]},
                'env_sources':[],'explicit_sources':[],'scope':'runtime-config-metadata-only-no-provider-validation','template_binding':'pod-env-and-deployment-template-v1'}
        observation={'operation_id':self.base.context['operation_id'],'source_sha':self.base.context['commit'],'nonce':'c'*32,'observations':observations}
        self.base.ledger.row['backend_config_observation']={'status':'done','nonce':'c'*32,'operation_id':observation['operation_id'],
            'source_sha':observation['source_sha'],'targets_sha256':writer_cut.digest(targets),'receipt_sha256':writer_cut.digest(observation)}
        intent={'resource_context':copy.deepcopy(self.base.context),'namespace_uid':'ns',
            'backend_config_deployments':{'source_sha':self.base.context['commit'],'cut_invocation':self.base.cut.invocation,
                'services':self.names,'sources':{n:copy.deepcopy(self.base.client.workloads['Deployment','voice-'+n]) for n in self.names},
                'observation':observation,'observation_plan':{'source_sha':self.base.context['commit'],'targets':targets}},
            'realtime_chat_service':{'source':copy.deepcopy(chat),'source_sha':self.base.context['commit'],'cut_invocation':self.base.cut.invocation},
            'realtime_space_service':{'source':copy.deepcopy(space_svc),'source_sha':self.base.context['commit'],'cut_invocation':self.base.cut.invocation},
            'realtime_config_deployments':{'source_root':'/source','source_sha':self.base.context['commit'],'cut_invocation':self.base.cut.invocation}}
        self.config=journal.Journal(Path(self.base.temp.name)/'mixed-config',intent); self.addCleanup(self.config.close)
        self.base.cut.bind_config_plan(self.config)
        chat_service.apply_chat_service(self.base.cut,self.config)
        space_service.apply_space_service(self.base.cut,self.config)
        session=Session(); migration_phase=Migration(); migration_phase.intent=self.authority.migration.intent
        space_phase=Space(); publisher_phase=Publisher(); publisher_phase.fence=self.base.fence; publisher_phase.lock=self.base.lock; publisher_phase.namespace_uid='ns'
        self.types=[patch.object(creation,'Session',Session),patch.object(migration,'Migration',Migration),
                    patch.object(space,'SpacePlanPhase',Space),patch.object(publisher,'PublisherPhase',Publisher)]
        for p in self.types: p.start(); self.addCleanup(p.stop)
        # The helper's values are synthetic; the collector and both config producers remain real.
        self.authority.patches[-3].stop(); self.authority.patches[-2].stop(); self.authority.patches[-1].stop()
        self.transport=[patch.object(authority.transport,'get',side_effect=lambda k,n:copy.deepcopy(self.authority.values[k,n])),
                        patch.object(authority.transport,'public_certificate',return_value=b'space-cert\n7\ncHVibGlj')]
        for p in self.transport: p.start(); self.addCleanup(p.stop)
        p3.bind_dependencies(self.base.cut,self.config,session,migration_phase,space_phase,publisher_phase,'/source','token')

    def cleanup(self,case):
        if not case.doCleanups(): raise RuntimeError('fixture cleanup failed')

    def test_ordered_real_producers_with_shared_cut(self):
        p3.apply(self.base.cut,self.config,'chat')
        backend.apply_development(self.base.cut,self.config,'notification')
        p3.apply(self.base.cut,self.config,'realtime')
        p3.apply(self.base.cut,self.config,'space')
        backend.apply_development(self.base.cut,self.config,'user')
        self.assertEqual(self.base.cut.config_snapshot['services'],self.names)
        self.assertEqual(space_service.verify_space_service(self.base.cut,self.config)['receipt']['uid'],'ss')
        self.assertEqual(self.base.cut.verify_drained()['active_sessions'],0)

    def test_source_drift_before_chat_patch_holds_without_mutation(self):
        calls=0
        def observe(*args):
            nonlocal calls
            calls+=1
            if calls==3: self.authority.p3['public_sources']['voice-space-media-server-tls']['uid']='foreign'
            return copy.deepcopy(self.authority.p3)
        initial=len(self.calls)
        with patch.object(authority.inputs,'observe',side_effect=observe):
            with self.assertRaises(p3.DeploymentError): p3.apply(self.base.cut,self.config,'chat')
        self.assertEqual(len(self.calls),initial); self.assertEqual(self.base.ledger.row['status'],'uncertain')

    def test_lost_chat_patch_retains_uncertainty_and_no_replay(self):
        apply=self.base.cut._patch
        def lost(kind,name,operations):
            result=apply(kind,name,operations)
            if kind=='Deployment' and name=='voice-chat': raise OSError('private-sentinel')
            return result
        initial=len(self.calls)
        with patch.object(self.base.cut,'_patch',side_effect=lost):
            with self.assertRaises(p3.DeploymentError): p3.apply(self.base.cut,self.config,'chat')
            with self.assertRaises(p3.DeploymentError): p3.apply(self.base.cut,self.config,'chat')
        self.assertEqual(len(self.calls),initial+1); self.assertEqual(self.base.ledger.row['status'],'uncertain')


if __name__=='__main__': unittest.main()
