import copy
from pathlib import Path
import tempfile
import unittest
from unittest import mock

import frontend_runtime_executor as transport
import full_cohort_inventory_phase as inventory
import full_cohort_package_phase as package
import media_backup_phase_test as fixture
import media_operation_guard as guard
import media_operation_journal as journals
import resource_fencing
import space_creator_peer_phase as peer
import space_creator_runtime_profile_config as config
import space_creator_runtime_profile_test as profile_fixture
import space_creator_peer_role_target_test as role_fixture


class PeerPhaseTest(unittest.TestCase):
    def test_fixed_listener_resolution_preserves_kubernetes_precedence(self):
        container={'envFrom':[{'configMapRef':{'name':'voice-app-config'}}],
                   'env':[{'name':'USER_GRPC_ADDR','value':':9090'}]}
        self.assertEqual(peer._literal(container,'USER_GRPC_ADDR',{'USER_GRPC_ADDR':'voice-user:9090'}),':9090')
        container['env'][0]['value']='$(OTHER)'
        with self.assertRaises(ValueError):peer._literal(container,'USER_GRPC_ADDR',{})
        with self.assertRaises(ValueError):peer._literal({'envFrom':[{'secretRef':{'name':'other'}}]},'USER_GRPC_ADDR',{})

    def test_image_target_changes_only_selected_image_and_replicas(self):
        source={'metadata':{'uid':'uid','generation':3},'spec':{'replicas':2,'template':{'spec':
                {'containers':[{'name':'user','image':'old','env':[{'name':'X','value':'kept'}]}]}}}}
        original=copy.deepcopy(source); image='ghcr.io/poryadok/voiceroot/user@sha256:'+'a'*64
        result=peer._target(source,'user',image)
        self.assertEqual(source,original);self.assertEqual(result['config_target']['replicas'],0)
        self.assertEqual(result['config_target']['template']['spec']['containers'][0]['image'],'old')
        self.assertEqual(result['image_target']['template']['spec']['containers'][0]['image'],image)
        with self.assertRaises(ValueError):peer._target(source,'user','foreign:latest')

    def _phase(self,root):
        context={'project':'voice','resource':'DEV','actor':'scheduled_devops',
          'claim':'11111111-1111-4111-8111-111111111111','window':'22222222-2222-4222-8222-222222222222',
          'revision':4,'generation':3,'operation_id':'media-c1-'+'a'*32,'commit':'b'*40,
          'images':{'voice-user/user':'ghcr.io/poryadok/voiceroot/user@sha256:'+'c'*64}}
        backing=fixture.Ledger(context); lock={'metadata':{'uid':'lock'}}
        def facade(name):
            journal=journals.Journal(root/name,{'resource_context':context,'namespace_uid':'ns'});self.addCleanup(journal.close)
            client=transport.KubectlTransport(journal);ledger=resource_fencing.KubeLedger(client)
            ledger.read=backing.read;ledger.write=backing.write
            return journal,resource_fencing.Fence(context,lambda:{},ledger),client
        pkg=package.PackagePhase.__new__(package.PackagePhase);inv=inventory.InventoryPhase.__new__(inventory.InventoryPhase)
        for name,obj in (('pkg',pkg),('inv',inv)):
            obj.journal,obj.fence,obj.client=facade(name);obj.lock=lock;obj.namespace_uid='ns'
        inv.package=pkg
        journal,fence,client=facade('config');cfg=config.CreatorConfigPhase(journal,fence,client,lock,'ns',pkg,inv)
        journal,fence,client=facade('peer');phase=peer.PeerTargetPhase(journal,fence,client,lock,'ns',pkg,inv,cfg)
        return phase,backing,cfg

    def test_protected_receipt_and_late_local_tamper_hold(self):
        with tempfile.TemporaryDirectory() as temp:
            phase,backing,cfg=self._phase(Path(temp));result={'scope':'synthetic','source_pins_sha256':peer.writer_cut.digest({'fixture':'x'})}
            with mock.patch.object(guard,'verify'),mock.patch.object(phase,'_observe',return_value=result), \
                 mock.patch.object(phase,'_sources',return_value={'fixture':'x'}):
                self.assertEqual(phase.run('token'),result)
                self.assertEqual(peer.verify_current(phase,'token'),result)
                with self.assertRaises((peer.PeerPlanError,peer.metadata.GuardLost)):phase.run('token')
                original=phase._ledger;calls=[]
                def late():
                    value=original();calls.append(1)
                    if len(calls)==2:phase.journal.state['steps'].pop(peer.STEP,None)
                    return value
                phase._ledger=late
                with self.assertRaises(peer.PeerPlanError):peer.verify_current(phase,'token')

    def test_lost_committed_reservation_is_uncertain_and_never_replayed(self):
        with tempfile.TemporaryDirectory() as temp:
            phase,backing,cfg=self._phase(Path(temp));write=backing.write
            def lost(revision,row):write(revision,row);raise OSError('lost response')
            phase.fence.ledger.write=lost;phase.fence.uncertain=mock.Mock()
            with mock.patch.object(guard,'verify'),mock.patch.object(phase,'_observe') as observe:
                with self.assertRaises(peer.PeerPlanError):phase.run('token')
                self.assertTrue(phase.poisoned);phase.fence.uncertain.assert_called_once();observe.assert_not_called()
                with self.assertRaises(peer.metadata.GuardLost):phase.run('token')

    def test_last_ledger_callback_cannot_change_config_receipt(self):
        with tempfile.TemporaryDirectory() as temp:
            phase,backing,cfg=self._phase(Path(temp));result={'scope':'synthetic','source_pins_sha256':peer.writer_cut.digest({'fixture':'x'})}
            with mock.patch.object(guard,'verify'),mock.patch.object(phase,'_observe',return_value=result), \
                 mock.patch.object(phase,'_sources',return_value={'fixture':'x'}):
                phase.run('token');original=phase._ledger;calls=[]
                def late():
                    value=original();calls.append(1)
                    if len(calls)==2:cfg.journal.state['steps']['drift']={}
                    return value
                phase._ledger=late
                with self.assertRaises(peer.PeerPlanError):peer.verify_current(phase,'token')

    def test_last_ledger_callback_cannot_change_phase_record(self):
        with tempfile.TemporaryDirectory() as temp:
            phase,backing,cfg=self._phase(Path(temp));result={'scope':'synthetic','source_pins_sha256':peer.writer_cut.digest({'fixture':'x'})}
            with mock.patch.object(guard,'verify'),mock.patch.object(phase,'_observe',return_value=result), \
                 mock.patch.object(phase,'_sources',return_value={'fixture':'x'}):
                phase.run('token');original=phase._ledger;calls=[]
                def late():
                    value=original();calls.append(1)
                    if len(calls)==2:phase.record=None
                    return value
                phase._ledger=late
                with self.assertRaises(peer.PeerPlanError):peer.verify_current(phase,'token')

    def test_last_ledger_callback_cannot_remove_predecessor_evidence(self):
        for name in ('package','inventory'):
            with self.subTest(name=name),tempfile.TemporaryDirectory() as temp:
                phase,backing,cfg=self._phase(Path(temp));target=getattr(phase,name)
                target.journal.state['steps']['protected']={'status':'done'}
                with mock.patch.object(guard,'verify'),mock.patch.object(phase,'_observe',return_value={
                        'source_pins_sha256':peer.writer_cut.digest({'fixture':'x'})}), \
                     mock.patch.object(phase,'_sources',return_value={'fixture':'x'}):
                    phase.run('token');original=phase._ledger;calls=[]
                    def late():
                        value=original();calls.append(1)
                        if len(calls)==2:target.journal.state['steps'].pop('protected')
                        return value
                    phase._ledger=late
                    with self.assertRaises(peer.PeerPlanError):peer.verify_current(phase,'token')

    def test_run_cannot_return_after_final_record_drift(self):
        with tempfile.TemporaryDirectory() as temp:
            phase,backing,cfg=self._phase(Path(temp));original=phase._ledger
            def late():
                value=original()
                if value[0].get(peer.LEDGER,{}).get('status')=='done':phase.record=None
                return value
            phase._ledger=late;phase.fence.uncertain=mock.Mock()
            with mock.patch.object(guard,'verify'),mock.patch.object(phase,'_observe',return_value={
                    'source_pins_sha256':peer.writer_cut.digest({'fixture':'x'})}), \
                 mock.patch.object(phase,'_sources',return_value={'fixture':'x'}):
                with self.assertRaises(peer.PeerPlanError):phase.run('token')
                self.assertTrue(phase.poisoned);phase.fence.uncertain.assert_called_once()

    def test_actual_static_observation_builds_bound_targets_without_claiming_readiness(self):
        with tempfile.TemporaryDirectory() as temp:
            phase,backing,cfg=self._phase(Path(temp));names={};deployments={};projection={}
            for name in ('space','role','user','social','auth'):
                item=role_fixture.role() if name=='role' else profile_fixture.deployment(name)
                item['metadata']['generation']=1;item['spec']['replicas']=1
                item['spec']['template']['metadata']={'labels':{'app':'voice-'+name}}
                container=item['spec']['template']['spec']['containers'][0]
                if name=='space':
                    container['env']=[{'name':'S2S_JWKS_URLS_JSON',
                        'value':'{"role":"https://voice-role:8443/.well-known/jwks.json"}'}]
                    container['volumeMounts']=[];item['spec']['template']['spec']['volumes']=[]
                if name=='user':container['env'].append({'name':'USER_GRPC_ADDR','value':':9090'})
                deployments['voice-'+name]=item;names[name]=['pod-'+name];projection[name]={'uid':'uid-'+name}
            current={'pods':names,'projection':projection,'database_observations':{'workloads':{
                name:[{'status':'observed','result':{'binding':{'state':'resolved','database':name+'_db','role':'fixture'},
                    'metadata':{'database':name+'_db','session':{'database_oid':1}}}}] for name in ('space','role','user','social')}}}
            phase.inventory.verify_current=mock.Mock(return_value=current)
            images={'voice-'+name+'/'+name:'ghcr.io/poryadok/voiceroot/'+name+'@sha256:'+'a'*64
                    for name in ('space','role','user','social')}
            phase.package.verify_current=mock.Mock(return_value={'images':images})
            observed={'workloads':{'space':[{'fields':{'USER_GRPC_ADDR':{'state':'expected'},
                                                       'SOCIAL_GRPC_ADDR':{'state':'expected'}}}]}}
            configmap={'apiVersion':'v1','kind':'ConfigMap','metadata':{'name':'voice-app-config','namespace':'voice-dev',
                       'uid':'config','resourceVersion':'9'},'data':{'REDIS_ADDR':'redis:6379'}}
            services={}
            for name,port in (('space',8080),('role',9091),('user',9090),('social',9090),('auth',9092)):
                services[name]={'apiVersion':'v1','kind':'Service','metadata':{'name':'voice-'+name,'namespace':'voice-dev',
                    'uid':'service-'+name,'resourceVersion':'4'},'spec':{'clusterIP':'10.0.0.1',
                    'selector':{'app':'voice-'+name},'ports':[{'name':'peer','port':port}]}}
            def call(verb,kind,name,*args):
                return copy.deepcopy(configmap if kind=='configmap' else services[name.removeprefix('voice-')])
            phase.client.call=call
            with mock.patch.object(guard,'verify'),mock.patch.object(peer.inventory,'inventory',return_value=(names,deployments,projection)), \
                 mock.patch.object(peer.config,'verify_completed',return_value=observed), \
                 mock.patch.object(peer.source,'_blob',return_value=b'source'),mock.patch.object(peer.source,'_disk',return_value=b'source') as disk:
                receipt=phase.run('token');self.assertEqual(peer.verify_current(phase,'token'),receipt)
                self.assertFalse(receipt['runtime_qualified']);self.assertEqual(receipt['scope'],'static-peer-targets-no-material-or-runtime-proof')
                self.assertEqual({item['port'] for item in receipt['space_service']['target_spec']['ports']},{8080,8443,9096,9099})
                self.assertEqual({item['port'] for item in receipt['services']['role']['target_spec']['ports']},{8443,9091})
                self.assertEqual(set(receipt['targets']),{'space','role','user','social'})
                current['database_observations']['workloads']['role'][0]['result']['metadata']['session']['database_oid']=2
                with self.assertRaises(peer.PeerPlanError):peer.verify_current(phase,'token')
                current['database_observations']['workloads']['role'][0]['result']['metadata']['session']['database_oid']=1
                services['role']['spec']['selector']['app']='foreign'
                with self.assertRaises(peer.PeerPlanError):peer.verify_current(phase,'token')
                services['role']['spec']['selector']['app']='voice-role'
                original=phase._observe
                def drift_after_observe(token):
                    result=original(token);disk.return_value=b'changed';return result
                phase._observe=drift_after_observe
                with self.assertRaises(peer.PeerPlanError):peer.verify_current(phase,'token')


if __name__=='__main__':unittest.main()
