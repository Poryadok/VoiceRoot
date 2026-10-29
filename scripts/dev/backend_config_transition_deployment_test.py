import copy
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import backend_config_transition_deployment as deployment
import backend_config_transition_probe as probe
import media_backup_phase_test as fixtures
import writer_cut


class DeploymentVerifierTest(unittest.TestCase):
    def setUp(self):
        self.base = fixtures.PhaseTest('test_exact_terminal_receipt_and_no_replay')
        self.addCleanup(self.cleanup,self.base); self.base.setUp()
        self.context = self.base.context; self.invocation = 'b'*32
        self.source = {'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':'voice-user','namespace':'voice-dev',
                       'uid':'user-dep','resourceVersion':'7','generation':2},
            'spec':{'replicas':0,'template':{'spec':{'containers':[{'name':'user','image':'old',
                'env':[{'name':'DATABASE_URL','valueFrom':{'secretKeyRef':{'name':'s','key':'db'}}}]}]}}}}
        original = copy.deepcopy(self.source); original['spec']['replicas'] = 1; original['metadata']['resourceVersion'] = '2'
        cut_step = {'status':'done','details':{'source':original,'target_spec':self.source['spec']},
                    'receipt':{'uid':'user-dep','generation':2}}
        self.base.ledger.row['writer_cut_effects'] = {'writer-drain-deployment-voice-user':
            {'status':'done','receipt_sha256':writer_cut.digest(cut_step['receipt'])}}
        targets = [{'workload':name,'pod_name':'voice-'+name+'-123','pod_uid':name+'-pod',
            'deployment_uid':name+'-dep','container_id':name+'-container','restart_count':0} for name in deployment.SERVICES]
        observations = {}
        for target in targets:
            name = target['workload']; observations[name] = {'status':'observed',
                'identity':{**{k:target[k] for k in ('pod_name','pod_uid','deployment_uid','container_id','restart_count')},'namespace_uid':'ns','deployment_rv':'2'},
                'fields':{key:{'state':'absent','origin':None} for key in probe.FIELDS[name]},'env_sources':[],
                'explicit_sources':[],
                'scope':'runtime-config-metadata-only-no-provider-validation',
                'template_binding':'pod-env-and-deployment-template-v1'}
        observations['user']['fields']['DATABASE_URL'] = {'state':'present','origin':{
            'kind':'secret','name':'s','uid':'secret-uid','resource_version':'9','key':'db','source':'explicit'}}
        observations['user']['explicit_sources'] = [{'kind':'secret','name':'s','uid':'secret-uid',
            'resource_version':'9','key':'db','optional':False,'present':True,'field':'DATABASE_URL','object_present':True}]
        receipt = {'operation_id':self.context['operation_id'],'source_sha':self.context['commit'],
                   'nonce':'c'*32,'observations':observations}
        prior = {'source_sha':self.context['commit'],'targets':targets}
        self.base.ledger.row['backend_config_observation'] = {'status':'done','nonce':receipt['nonce'],
            'operation_id':receipt['operation_id'],'source_sha':receipt['source_sha'],
            'targets_sha256':writer_cut.digest(targets),'receipt_sha256':writer_cut.digest(receipt)}
        self.cut = SimpleNamespace(fence=self.base.fence,namespace_uid='ns',invocation=self.invocation,
            admission={'controllers':[original]}, journal=SimpleNamespace(state={'steps':{'writer-drain-deployment-voice-user':cut_step}}),
            client=fixtures.Client(self.base.lock),lock=self.base.lock)
        self.intent = {'resource_context':self.context,'namespace_uid':'ns','backend_config_deployments':{
            'source_sha':self.context['commit'],'cut_invocation':self.invocation,'services':['user'],
            'sources':{'user':self.source},'observation':receipt,'observation_plan':prior}}
        self.journal = SimpleNamespace(intent=self.intent,state={'intent':self.intent,'steps':{}},poisoned=False)
        bound_details = {'intent':copy.deepcopy(self.intent),'steps':{},'effects':{},'services':[]}
        bound_receipt = {'operation_id':self.context['operation_id'],'cut_invocation':self.invocation,
                         'services':['user'],'config_intent_sha256':writer_cut.digest(self.intent)}
        bound = {'status':'done','nonce':'a'*32,'details_sha256':writer_cut.digest(bound_details),
                 'details':bound_details,'receipt':bound_receipt}
        self.cut.journal.state['steps']['writer-cut-config-plan'] = bound
        self.base.ledger.row['writer_cut_effects']['writer-cut-config-plan'] = {
            'status':'done','nonce':bound['nonce'],'details_sha256':bound['details_sha256'],
            'receipt_sha256':writer_cut.digest(bound_receipt)}
        source,target,_ = deployment._plan(self.cut,self.journal,'user')
        self.target = target; m = source['metadata']
        patch_list = [{'op':'test','path':'/metadata/uid','value':m['uid']},
                      {'op':'test','path':'/metadata/resourceVersion','value':m['resourceVersion']},
                      {'op':'test','path':'/spec','value':source['spec']},{'op':'replace','path':'/spec','value':target}]
        details = {'operation_id':self.context['operation_id'],'invocation_nonce':self.invocation,
                   'source':source,'target_spec':target,'patch':patch_list}
        output = {'uid':m['uid'],'spec_sha256':writer_cut.digest(target),'responseResourceVersion':'8',
                  'resourceVersion':'8','generation':3}
        self.step = 'backend-config-deployment-voice-user'
        entry = {'status':'done','nonce':'d'*32,'details_sha256':writer_cut.digest(details),'details':details,'receipt':output}
        self.journal.state['steps'][self.step] = entry
        self.base.ledger.row['backend_config_effects'] = {self.step:{'status':'done','nonce':entry['nonce'],
            'details_sha256':entry['details_sha256'],'receipt_sha256':writer_cut.digest(output)}}

    def cleanup(self, base):
        if not base.doCleanups(): raise RuntimeError('fixture cleanup failed')

    def verify(self):
        with patch.object(deployment.media_operation_guard,'verify'), \
             patch.object(deployment.metadata,'linux_private_bytes',return_value=b'secret-uid\n9'):
            return deployment.verify_deployment_successor(self.cut,self.journal,'user')

    def test_fixed_profile_preserves_old_image_and_full_spec(self):
        result = self.verify(); before = self.source['spec']['template']['spec']['containers'][0]
        after = result['target_spec']['template']['spec']['containers'][0]
        self.assertEqual(after['image'],before['image']); self.assertEqual(result['target_spec']['replicas'],0)
        self.assertEqual(after['env'],before['env']+[{'name':'VOICE_RELEASE_PROFILE','value':'development'}])

    def test_foreign_observation_or_effect_cannot_become_successor(self):
        for fault in ('receipt','source','target','ledger','claim'):
            with self.subTest(fault=fault):
                self.setUp()
                if fault == 'receipt': self.intent['backend_config_deployments']['observation']['nonce'] = 'e'*32
                if fault == 'source': self.source['spec']['replicas'] = 1
                if fault == 'target': self.journal.state['steps'][self.step]['details']['target_spec']['replicas'] = 1
                if fault == 'ledger': self.base.ledger.row['backend_config_effects'][self.step]['nonce'] = 'f'*32
                if fault == 'claim': self.intent['resource_context'] = dict(self.context, claim='foreign')
                with self.assertRaises(deployment.DeploymentError): self.verify()

    def test_cut_source_revision_must_match_authenticated_observation(self):
        obs = self.intent['backend_config_deployments']['observation']
        obs['observations']['user']['identity']['deployment_rv'] = 'different'
        self.base.ledger.row['backend_config_observation']['receipt_sha256'] = writer_cut.digest(obs)
        bound = self.cut.journal.state['steps']['writer-cut-config-plan']
        bound['details']['intent'] = copy.deepcopy(self.intent)
        bound['details_sha256'] = writer_cut.digest(bound['details'])
        bound['receipt']['config_intent_sha256'] = writer_cut.digest(self.intent)
        self.base.ledger.row['writer_cut_effects']['writer-cut-config-plan'] = {
            'status':'done','nonce':bound['nonce'],'details_sha256':bound['details_sha256'],
            'receipt_sha256':writer_cut.digest(bound['receipt'])}
        with self.assertRaises(ValueError): deployment._plan(self.cut,self.journal,'user')

    def test_referenced_secret_drift_blocks_successor(self):
        with patch.object(deployment.media_operation_guard,'verify'), \
             patch.object(deployment.metadata,'linux_private_bytes',return_value=b'secret-uid\n10'):
            with self.assertRaises(deployment.DeploymentError):
                deployment.verify_deployment_successor(self.cut,self.journal,'user')

    def test_optional_absent_object_must_remain_absent(self):
        observation = {'env_sources':[{'kind':'secret','name':'gone','uid':None,'resource_version':None,
            'prefix':'','fields':[],'optional':True,'object_present':False}], 'explicit_sources':[], 'fields':{}}
        with patch.object(deployment.media_operation_guard,'verify'), \
             patch.object(deployment.metadata,'linux_private_bytes',return_value=b''):
            deployment._refs(self.cut,observation)
        with patch.object(deployment.media_operation_guard,'verify'), \
             patch.object(deployment.metadata,'linux_private_bytes',return_value=b'created-uid\n1'):
            with self.assertRaises(ValueError): deployment._refs(self.cut,observation)
        with patch.object(deployment.media_operation_guard,'verify'), \
             patch.object(deployment.metadata,'linux_private_bytes',side_effect=OSError('private')):
            with self.assertRaises(OSError): deployment._refs(self.cut,observation)


if __name__ == '__main__': unittest.main()
