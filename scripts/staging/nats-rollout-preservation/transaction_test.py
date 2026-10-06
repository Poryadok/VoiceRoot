import copy
import datetime as dt
from pathlib import Path
import unittest
from unittest.mock import Mock, patch
import transaction


class TransactionFailureTest(unittest.TestCase):
    def test_only_reviewed_story_pair_is_admitted_for_changed_backend_rollback(self):
        import actor_root
        image=next(iter(actor_root.STORY_IMAGES));child,config,source=actor_root.STORY_IMAGES[image]
        authority={'roles':['story'],'proofs':{'story':{'root_verified':True}},'mounts':{'story':{'exact':'captured'}},'binding':{'same':'account'},'story_schema':{'clean_version':4}}
        authority['compatible_story_candidate']={'schema':'voice-reviewed-story-compatible-pair-v1','image':image,'child_sha256':child,'config_sha256':config,'component_source_sha':source,'schema_sha256':transaction.canonical(authority['story_schema']),'actor_sha256':transaction.canonical({'binding':authority['binding'],'mount':authority['mounts']['story'],'proof':authority['proofs']['story']})}
        previous={'status':'PASS','input_target':{'mode':'images-only','changed_services':['story'],'template_hashes':{'voice-story':'x'}},'context':{'original_snapshots':{'voice-story':{'spec':{'template':{'spec':{'containers':[{'name':'story','image':'old'}]}}}}},'old_images':{'voice-story/story':image}},'service_actor_authority':authority,'nonnats':{'checks':[]},'target':{'images':{}},'contract':{}}
        self.assertEqual(transaction.rollback_package(previous)['target']['images']['voice-story/story'],image)
        alias='ghcr.io/poryadok/voiceroot/story@sha256:'+'d'*64
        promoted=copy.deepcopy(previous)
        promoted['context']['old_images']['voice-story/story']=alias
        promoted['service_actor_authority']['compatible_story_candidate']['image']=alias
        self.assertEqual(transaction.rollback_package(promoted)['target']['images']['voice-story/story'],alias)
        for fault in ('unknown-image','wrong-source','wrong-config','schema','actor','bot','search'):
            row=copy.deepcopy(previous)
            if fault=='unknown-image':row['context']['old_images']['voice-story/story']='ghcr.io/poryadok/voiceroot/story@sha256:'+'f'*64
            elif fault=='wrong-source':row['service_actor_authority']['compatible_story_candidate']['component_source_sha']='f'*40
            elif fault=='wrong-config':row['service_actor_authority']['compatible_story_candidate']['config_sha256']='f'*64
            elif fault=='schema':row['service_actor_authority']['story_schema']['clean_version']=5
            elif fault=='actor':row['service_actor_authority']['binding']['same']='changed'
            else:row['input_target']['changed_services']=[fault]
            with self.subTest(fault=fault),self.assertRaisesRegex(transaction.Blocked,'backend_candidate_unproved'):transaction.rollback_package(row)

    def test_bot_completed_version_renderer_failure_retry_does_not_run_jobs_twice(self):
        plan=[{'database':'bot','files':{'000004_slash_interaction_outbox.up.sql':'trusted'}}]
        before={'uid':'secret','resourceVersion':'1','bot_prerequisite':{'authority':{'pg':'same'},'observation':{'database':'bot_db','version':3,'dirty':False,'duplicate_token_groups':0}}}
        after=copy.deepcopy(before);after['bot_prerequisite']['observation']['version']=4
        state={'phase':'AWAITING_OFF_NODE','status':'WAITING','operation':'123456abcdef','events':[],'context':{},'provenance':{},'migrations':plan,'migration_secret_metadata':before,'custody':{'verified':True},'target':{'mode':'full','migration_sha256':transaction.canonical(plan)},'cut':{'manifest':{'archive_sha256':'a'*64},'manifest_sha256':'b'*64},'renderer_authority':{'root':'bound'}}
        stage=Mock();stage.final_path=Path('/same');stage.expected={'namespace_uid':'namespace'};stage.marker={'metadata':{'uid':'marker','resourceVersion':'7'},'data':{'generation':'g','previousGeneration':'p','dataPVC':'same'}};stage.final_claim={'metadata':{'name':'same','uid':'pvc'},'spec':{'volumeName':'pv'}};stage.final_pv={'metadata':{'uid':'pv'},'spec':{'local':{}}};stage.refence.return_value={'verified':True}
        state['cut']['census_sha256']='c'*64
        with patch.object(transaction,'reconstruct',return_value=stage),patch.object(transaction,'context',return_value={'marker':{'data':{'phase':'rollout-capturing'}}}),patch.object(transaction,'save'),patch.object(transaction,'file_sha',return_value='b'*64),patch.object(transaction,'verify_archive',return_value=True),patch.object(transaction,'revalidate_inputs'),patch.object(transaction.space_safeguards,'before_migrate'),patch.object(transaction.space_safeguards,'after_migrate'),patch.object(transaction.migrations,'preflight',side_effect=[before,after,after,dict(after,uid='drift')]),patch.object(transaction.migrations,'execute',return_value=['owned-job']) as jobs,patch.object(transaction,'DockerRuntime'),patch.object(transaction,'verify_post_apply'),patch.object(transaction.renderer_root,'apply_paused',side_effect=[transaction.Blocked('renderer_cas_interrupted'),None]):
            with self.assertRaisesRegex(transaction.Blocked,'renderer_cas_interrupted'):transaction.authorize(None,Path('/private'),state,'a'*64,'b'*64,{})
            self.assertEqual(state['migration_completed_metadata'],after);stage.prepared.assert_not_called()
            transaction.authorize(None,Path('/private'),state,'a'*64,'b'*64,{})
            jobs.assert_called_once();stage.prepared.assert_called_once()
            state.update(phase='RENDERER_TEMPLATE',status='BLOCKED');state.pop('authorization',None)
            with self.assertRaisesRegex(transaction.Blocked,'database_secret_changed'):transaction.authorize(None,Path('/private'),state,'a'*64,'b'*64,{})
            jobs.assert_called_once()

    def test_full_prior_selects_complete_root_recorded_catalog_without_hub_or_empty_services(self):
        import source_authority
        names=sorted(set(source_authority.IMAGE_NAMES)-{'nats-hub-config-renderer'})
        previous={'status':'PASS','input_target':{'mode':'full','changed_services':[],
            'template_hashes':{'voice-'+name:'a'*64 for name in names}},'renderer_authority':{'fixed':'bound'}}
        previous['input_target']['template_hashes'][transaction.renderer_root.HUB]='b'*64
        self.assertEqual(transaction.rollback_services(previous),names)
        self.assertEqual(len(names),22)
        # Selection never authorizes the unproven full-schema rollback itself.
        with self.assertRaises(transaction.Blocked):transaction.rollback_package(previous)
        for change in ('missing','unknown','hub-unbound'):
            row=copy.deepcopy(previous)
            if change=='missing':row['input_target']['template_hashes'].pop('voice-user')
            elif change=='unknown':row['input_target']['template_hashes']['voice-unknown']='c'*64
            else:row.pop('renderer_authority')
            with self.subTest(change=change),self.assertRaises(transaction.Blocked):transaction.rollback_services(row)

    def test_renderer_retry_reproves_postmigration_cut_without_reexecuting_jobs(self):
        for failed in (False,True):
            with self.subTest(renderer_failed=failed):
                cut={'manifest':{'archive_sha256':'a'*64},'manifest_sha256':'b'*64,'census_sha256':'c'*64}
                post={'verified_postmigration_baseline':True}
                state={'phase':'RENDERER_TEMPLATE','status':'BLOCKED','operation':'123456abcdef','events':[],
                    'custody':{'verified':True},'renderer_authority':{'root_bound':True},
                    'migration_jobs':[],'space_postcondition':None,'provenance':{},'migrations':[],
                    'target':{'mode':'full','migration_sha256':transaction.canonical([])},'cut':cut,
                    'nats_migration':{'cut':post},
                    'context':{'marker':{'data':{'phase':'rollout-capturing'}}}}
                stage=Mock();stage.final_path=Path('/selected-store');stage.expected={'namespace_uid':'namespace'}
                stage.marker={'metadata':{'uid':'marker','resourceVersion':'7'},
                    'data':{'generation':'generation','previousGeneration':'previous','dataPVC':'same-pvc'}}
                stage.final_claim={'metadata':{'name':'same-pvc','uid':'pvc'},'spec':{'volumeName':'same-pv'}}
                stage.final_pv={'metadata':{'uid':'pv'},'spec':{'local':{}}}
                order=[]
                stage.prepared.side_effect=lambda:order.append('prepared')
                def renderer(*a):
                    order.append('renderer')
                    if failed:raise transaction.Blocked('renderer_paused_template_changed')
                with patch.object(transaction,'reconstruct',return_value=stage),patch.object(transaction,'context',return_value={}),\
                     patch.object(transaction,'save'),patch.object(transaction,'file_sha',return_value='b'*64),\
                     patch.object(transaction,'verify_archive',return_value=True),patch.object(transaction,'revalidate_inputs'),\
                     patch.object(transaction,'DockerRuntime'),\
                     patch.object(transaction,'verify_post_apply',side_effect=lambda *a:order.append('native-proof')) as proof,\
                     patch.object(transaction.space_safeguards,'before_migrate') as before,\
                     patch.object(transaction.space_safeguards,'after_migrate',side_effect=lambda *a:order.append('postcondition')),\
                     patch.object(transaction.migrations,'execute') as jobs,patch.object(transaction,'apply_contract') as nats,\
                     patch.object(transaction.renderer_root,'apply_paused',side_effect=renderer):
                    if failed:
                        with self.assertRaises(transaction.Blocked):transaction.authorize(None,Path('/private'),state,'a'*64,'b'*64,{})
                        self.assertNotIn('authorization',state);stage.prepared.assert_not_called()
                    else:
                        transaction.authorize(None,Path('/private'),state,'a'*64,'b'*64,{})
                        self.assertEqual(order,['native-proof','postcondition','renderer','prepared'])
                        self.assertEqual(state['phase'],'PAUSED_APPLY')
                    self.assertIs(proof.call_args.args[2],post)
                    before.assert_not_called();jobs.assert_not_called();nats.assert_not_called()
                stage.restart.assert_not_called()

    def test_capture_interruption_retains_actor_binding_before_any_fence(self):
        import json,tempfile
        proof={'schema':'voice-nats-service-actor-authority-v1','roles':['social'],'binding':{'services_uid':'fixed'}}
        saved=[]
        with tempfile.TemporaryDirectory() as td:
            base=Path(td);(base/'apply-manifests.json').write_text('[]')
            with patch.object(transaction,'save',side_effect=lambda p,row:saved.append(copy.deepcopy(row))),\
                 patch.object(transaction.guard,'frontend_image_only',return_value=False),\
                 patch.object(transaction.RolloutStage,'capture',side_effect=RuntimeError('owned-capture-interrupted')):
                with self.assertRaisesRegex(RuntimeError,'owned-capture-interrupted'):
                    transaction.prepare(None,base,base,{'mode':'full'},{},'123456abcdef',{},[],{},
                        actor_authority=proof,actor_services=['social'])
        self.assertEqual(saved[0]['service_actor_authority'],proof)
        self.assertEqual(saved[0]['service_actor_services'],['social'])
        self.assertEqual(saved[0]['phase'],'READ_ONLY_CAPTURE')

    def test_normal_space_gate_orders_offnode_migration_postcondition_before_prepared(self):
        import space_migration
        before={'database':'space_db','version':15,'dirty':False,'allow_guests_true':2,
            'column_default':'true','column_type':'boolean'}
        after=dict(before,version=24,allow_guests_true=0,column_default='false')
        gate={'authority':{},'requirement':space_migration.requirement(before,24)}
        backup={'schema':'voice-space-backup-v1','before':before,'snapshot':'00000003-00000008-1',
            'dump_sha256':'c'*64,'dump_bytes':10,'restored':True,'offnode_verified':True}
        for scenario in ('offnode-failed','postcondition-failed','renderer-failed','success'):
            with self.subTest(scenario=scenario):
                state={'phase':'AWAITING_OFF_NODE','status':'WAITING','operation':'123456abcdef','events':[],
                    'context':{},'provenance':{},'migrations':[],'migration_secret_metadata':{},
                    'target':{'mode':'full','migration_sha256':transaction.canonical([])},
                    'cut':{'manifest':{'archive_sha256':'a'*64},'manifest_sha256':'b'*64,'census_sha256':'c'*64},
                    'space_gate':gate,'space_backup':{'private_root_produced':True},
                    'renderer_authority':{'root_produced_descriptor':True}}
                stage=Mock();stage.final_path=Path('/same-selected-store');stage.expected={'namespace_uid':'namespace'}
                stage.marker={'metadata':{'uid':'marker','resourceVersion':'7'},
                    'data':{'generation':'generation','previousGeneration':'previous','dataPVC':'same-pvc'}}
                stage.final_claim={'metadata':{'name':'same-pvc','uid':'pvc'},'spec':{'volumeName':'same-pv'}}
                stage.final_pv={'metadata':{'uid':'pv'},'spec':{'local':{}}}
                order=[]
                stage.prepared.side_effect=lambda:order.append('prepared')
                class Snapshot:
                    def __init__(self,*args):
                        migrated='migration' in order
                        self.before=copy.deepcopy(after if migrated else before)
                        if migrated and scenario=='postcondition-failed':self.before['allow_guests_true']=1
                    def __enter__(self):order.append('snapshot-open');return self
                    def __exit__(self,*a):order.append('snapshot-closed')
                def offnode(*args):
                    order.append('offnode')
                    if scenario=='offnode-failed':raise transaction.encrypted_cut.CryptoError('backup_space_offnode_unverified')
                    return backup
                def migrate(*args):
                    self.assertTrue(order,'migration was attempted before offnode/snapshot checks')
                    self.assertEqual(order[-1],'snapshot-closed')
                    order.append('migration');return []
                def renderer(*args):
                    self.assertEqual(order[-1],'snapshot-closed')
                    self.assertEqual(state['space_postcondition'],after)
                    self.assertEqual(state['phase'],'RENDERER_TEMPLATE')
                    order.append('renderer')
                    if scenario=='renderer-failed':raise transaction.Blocked('renderer_paused_template_changed')
                with patch.object(transaction,'reconstruct',return_value=stage),patch.object(transaction,'context',return_value={}),\
                     patch.object(transaction,'save'),patch.object(transaction,'file_sha',return_value='b'*64),\
                     patch.object(transaction,'verify_archive',return_value=True),patch.object(transaction,'revalidate_inputs'),\
                     patch.object(transaction.encrypted_cut,'authorize_space',side_effect=offnode),\
                     patch.object(transaction.space_safeguards.space_authority,'revalidate'),\
                     patch.object(transaction.space_safeguards,'Postgres'),\
                     patch.object(transaction.space_safeguards,'Snapshot',Snapshot),\
                     patch.object(transaction.migrations,'execute',side_effect=migrate) as jobs,\
                     patch.object(transaction.renderer_root,'apply_paused',side_effect=renderer) as renderer_apply:
                    if scenario=='success':
                        receipt=transaction.authorize(None,Path('/private'),state,'a'*64,'b'*64,{})
                        self.assertEqual(state['space_postcondition'],after)
                        self.assertEqual(order,['offnode','snapshot-open','snapshot-closed','migration',
                            'snapshot-open','snapshot-closed','renderer','prepared'])
                        self.assertEqual(state['phase'],'PAUSED_APPLY')
                    else:
                        with self.assertRaises((transaction.encrypted_cut.CryptoError,space_migration.SpaceError,transaction.Blocked)):
                            transaction.authorize(None,Path('/private'),state,'a'*64,'b'*64,{})
                        stage.prepared.assert_not_called()
                        self.assertNotIn('authorization',state)
                        if scenario!='renderer-failed':renderer_apply.assert_not_called()
                        if scenario=='offnode-failed':jobs.assert_not_called()
                        else:self.assertEqual(state['status'],'BLOCKED')
                stage.restart.assert_not_called()

    def test_normal_authorize_migrates_only_after_verified_closed_backup(self):
        state={'phase':'AWAITING_OFF_NODE','status':'WAITING','operation':'123456abcdef','events':[],
            'context':{},'provenance':{},'nats_contract':{'schema':'trusted-plan'},'custody':{'verified':True},
            'nats_contract_expires_at':(dt.datetime.now(dt.timezone.utc)+dt.timedelta(hours=1)).isoformat(),
            'migrations':[],'migration_secret_metadata':{},'target':{'mode':'images-only','migration_sha256':transaction.canonical([])},
            'cut':{'manifest':{'archive_sha256':'a'*64},'manifest_sha256':'b'*64}}
        stage=Mock();order=[]
        stage.prepared.side_effect=RuntimeError('stop_before_authorization_receipt')
        with patch.object(transaction,'reconstruct',return_value=stage),patch.object(transaction,'context',return_value={}),\
             patch.object(transaction,'save'),patch.object(transaction,'file_sha',return_value='b'*64),\
             patch.object(transaction,'verify_archive',return_value=True),patch.object(transaction,'revalidate_inputs'),\
             patch.object(transaction,'apply_contract',create=True,side_effect=lambda *a:order.append('nats-after-offnode') or {'verified':True}) as migration,\
             patch('nats_contract_custody.advance',side_effect=lambda *a:order.append('cm-after-proof') or {}),\
             patch.object(transaction.migrations,'execute',side_effect=lambda *a:order.append('db-migrations') or []):
            with self.assertRaisesRegex(RuntimeError,'stop_before_authorization_receipt'):
                transaction.authorize(None,Path('/private'),state,'a'*64,'b'*64,{})
        migration.assert_called_once();self.assertEqual(order,['nats-after-offnode','cm-after-proof','db-migrations'])
        stage.restart.assert_not_called()

    def test_bot_closed_preflight_failure_blocks_nats_and_database_mutations(self):
        sql='trusted-sql';plan=[{'database':'bot','files':{'000004_slash_interaction_outbox.up.sql':sql}}]
        state={'phase':'AWAITING_OFF_NODE','status':'WAITING','operation':'123456abcdef','events':[],
            'context':{},'provenance':{},'nats_contract':{'schema':'trusted-plan'},'custody':{'verified':True},
            'nats_contract_expires_at':(dt.datetime.now(dt.timezone.utc)+dt.timedelta(hours=1)).isoformat(),
            'migrations':plan,'migration_secret_metadata':{'root-produced':True},
            'target':{'mode':'full','migration_sha256':transaction.canonical(plan)},
            'cut':{'manifest':{'archive_sha256':'a'*64},'manifest_sha256':'b'*64}}
        stage=Mock();stage.prepared.side_effect=RuntimeError('mutation_boundary_was_reached')
        with patch.object(transaction,'reconstruct',return_value=stage),patch.object(transaction,'context',return_value={}),\
             patch.object(transaction,'save'),patch.object(transaction,'file_sha',return_value='b'*64),\
             patch.object(transaction,'verify_archive',return_value=True),patch.object(transaction,'revalidate_inputs'),\
             patch.object(transaction.migrations,'preflight',side_effect=transaction.Blocked('bot_forward_constraint_prerequisite_failed')) as prerequisite,\
             patch.object(transaction,'apply_contract',return_value={}) as nats,\
             patch('nats_contract_custody.advance',return_value={}),\
             patch.object(transaction.migrations,'execute',return_value=[]) as jobs:
            with self.assertRaisesRegex(transaction.Blocked,'bot_forward_constraint_prerequisite_failed'):
                transaction.authorize(None,Path('/private'),state,'a'*64,'b'*64,{})
            prerequisite.assert_called_once();nats.assert_not_called();jobs.assert_not_called()
        stage.prepared.assert_not_called();stage.restart.assert_not_called()
        self.assertNotIn('authorization',state)
    def test_successful_release_rollback_package_uses_actual_old_images_and_fresh_cut(self):
        old={'metadata':{'name':'voice-web','namespace':'voice-staging','uid':'deployment'},
             'kind':'Deployment','apiVersion':'apps/v1','spec':{'replicas':1,'template':{'spec':{'containers':[{'name':'web','image':'mutable-old-tag'}]}}}}
        pin='example.invalid/web@sha256:'+'a'*64
        previous={'status':'PASS','operation':'123456abcdef','contract':{},
            'nonnats':{'checks':[],'actions':[]},'target':{'images':{}},
            'input_target':{'registry':'example.invalid','tag':'b'*40,'mode':'images-only','changed_services':['web'],
                'source_hashes':{'deploy/nats/acl-intent.yaml':'c'*64},'images':{'voice-web/web':'new'},'template_hashes':{'voice-web':'d'*64}},
            'context':{'original_snapshots':{'voice-web':old},'old_images':{'voice-web/web':pin}}}
        package=transaction.rollback_package(previous)
        self.assertEqual(package['target']['images'],{'voice-web/web':pin})
        self.assertEqual(package['target']['tag'],'b'*40) # trusted source identity; images remain explicit
        self.assertEqual(package['migrations'],[])
        self.assertNotIn('cut',package)
        self.assertEqual(previous['context']['original_snapshots']['voice-web']['spec']['template']['spec']['containers'][0]['image'],'mutable-old-tag')
    def test_renderer_rollback_package_keeps_reserved_hub_out_of_runner_manifests(self):
        from stage_runtime import HUB
        app={'metadata':{'name':'voice-web'},'spec':{'template':{'spec':{'containers':[{'name':'web','image':'old-tag'}]}}}}
        hub={'metadata':{'name':HUB},'spec':{'template':{'spec':{'containers':[{'name':'nats','image':'nats@sha256:'+'c'*64}],
            'initContainers':[{'name':'nats-config-renderer','image':'renderer:old'}]}}}}
        previous={'status':'PASS','operation':'123456abcdef','contract':{},'nonnats':{'checks':[],'actions':[]},
            'renderer_authority':{'root_owned':True},'target':{'images':{}},
            'input_target':{'registry':'example.invalid','tag':'b'*40,'mode':'images-only','changed_services':['web'],
                'images':{},'template_hashes':{'voice-web':'d'*64,HUB:'e'*64}},
            'context':{'original_snapshots':{'voice-web':app,HUB:hub},'old_images':{
                'voice-web/web':'example.invalid/web@sha256:'+'a'*64,
                HUB+'/nats':'nats@sha256:'+'c'*64,HUB+'/nats-config-renderer':'example.invalid/renderer@sha256:'+'f'*64}}}
        package=transaction.rollback_package(previous)
        self.assertEqual([r['metadata']['name'] for r in package['manifests']],['voice-web'])
        self.assertNotIn(HUB,package['target']['template_hashes'])
        self.assertFalse(any(key.startswith(HUB+'/') for key in package['target']['images']))
    def test_guarded_image_rollback_proves_store_before_only_verified_restart(self):
        receipt={'operation':'123456abcdef','target':{'mode':'images-only'}}
        state={'phase':'PAUSED_APPLY','status':'BLOCKED','operation':'123456abcdef','target':receipt['target'],
               'authorization':receipt,'events':[],'provenance':{},'cut':{},'context':{}}
        stage=Mock();stage.final_path=Path('/selected-store');order=[]
        for method in ('adopt_rollback_target','restore_original_templates','verified','restart'):
            getattr(stage,method).side_effect=lambda *a,m=method:order.append(m)
        with patch.object(transaction,'reconstruct',return_value=stage),patch.object(transaction,'context',return_value={}),\
             patch.object(transaction,'revalidate_inputs'),patch.object(transaction,'DockerRuntime'),patch.object(transaction,'save'),\
             patch.object(transaction,'verify_post_apply',side_effect=lambda *a:order.append('proof') or {'verified':True}):
            result=transaction.rollback(None,Path('/private'),state,receipt,'101',{})
        self.assertEqual(order,['adopt_rollback_target','restore_original_templates','proof','verified','restart'])
        self.assertEqual(result['status'],'ROLLED_BACK')
    def test_rollback_after_restart_or_with_database_changes_has_zero_mutations(self):
        for phase,mode in (('RESTART','images-only'),('PAUSED_APPLY','full')):
            receipt={'operation':'123456abcdef','target':{'mode':mode}}
            state={'phase':phase,'status':'BLOCKED','operation':'123456abcdef','target':receipt['target'],'authorization':receipt}
            with patch.object(transaction,'reconstruct') as reconstruct:
                with self.assertRaisesRegex(transaction.Blocked,'rollback_fresh_transaction_required'):
                    transaction.rollback(None,Path('/private'),state,receipt,'101',{})
            reconstruct.assert_not_called()

    def test_partial_app_rollback_retains_verified_additive_contract_baseline(self):
        receipt={'operation':'123456abcdef','target':{'mode':'images-only'}}
        post={'manifest':{'approved_additive_config':True},'census':{'post_config':True}}
        state={'phase':'PAUSED_APPLY','status':'BLOCKED','operation':'123456abcdef','target':receipt['target'],
            'authorization':receipt,'events':[],'provenance':{},'cut':{'pre_config':True},'context':{},
            'nats_migration':{'verified':True,'cut':post}}
        stage=Mock();stage.final_path=Path('/same-selected-store')
        with patch.object(transaction,'reconstruct',return_value=stage),patch.object(transaction,'context',return_value={}),\
             patch.object(transaction,'revalidate_inputs'),patch.object(transaction,'DockerRuntime'),patch.object(transaction,'save'),\
             patch.object(transaction,'verify_post_apply',return_value={'verified':True}) as proof:
            transaction.rollback(None,Path('/private'),state,receipt,'101',{})
        self.assertEqual(proof.call_args.args[2],post)
    def test_process_interruption_retains_current_restart_mutation_context(self):
        receipt={'operation':'123456abcdef','target':{},'expires_at':(dt.datetime.now(dt.timezone.utc)+dt.timedelta(hours=1)).isoformat()}
        state={'phase':'PAUSED_APPLY','status':'WAITING','operation':'123456abcdef','target':{},'authorization':receipt,
               'events':[],'provenance':{},'cut':{},'context':{}}
        stage=Mock();stage.final_path=Path('/selected-store');stage.current={'replicas':0,'override':False}
        written=[]
        def reconstruct(kube,state,journal):
            stage.save=journal
            return stage
        def interrupted_restart():
            stage.current={'replicas':1,'override':True}
            stage.save({'kind':'user_cycle_override','owner':state['operation']})
            raise KeyboardInterrupt('process killed after owned mutation')
        stage.restart.side_effect=interrupted_restart
        with patch.object(transaction,'reconstruct',side_effect=reconstruct),\
             patch.object(transaction,'context',side_effect=lambda s:copy.deepcopy(s.current)),\
             patch.object(transaction,'revalidate_inputs'),patch.object(transaction,'DockerRuntime'),\
             patch.object(transaction,'verify_post_apply',return_value={'verified':True}),\
             patch.object(transaction,'save',side_effect=lambda p,s:written.append(copy.deepcopy(s))):
            with self.assertRaises(KeyboardInterrupt):transaction.finish(None,Path('/private'),state,receipt,'101',{})
        self.assertEqual(written[-1]['context'],{'replicas':1,'override':True})
        self.assertEqual(written[-1]['events'][-1]['kind'],'user_cycle_override')
        self.assertEqual(written[-1]['phase'],'RESTART')
        stage.refence.assert_not_called() # abrupt death has no exception recovery
    def test_renderer_partial_recovery_journals_inverse_before_cas_and_retries_only_that_intent(self):
        receipt={'operation':'123456abcdef','target':{'mode':'images-only'}}
        forward={'descriptor':{'direction':'forward'}}
        reverse={'descriptor':{'direction':'reverse'},'output':{'verified':True}}
        state={'phase':'PAUSED_APPLY','status':'BLOCKED','operation':'123456abcdef','target':receipt['target'],
            'authorization':receipt,'events':[],'provenance':{},'cut':{},'context':{},'renderer_authority':forward}
        stage=Mock();stage.final_path=Path('/same-selected-store');stage.refence.return_value={'verified':True}
        written=[];calls=[]
        def cas(current,authority):
            self.assertEqual(written[-1]['renderer_recovery_authority'],reverse)
            self.assertEqual(written[-1]['renderer_authority'],reverse)
            calls.append('cas')
            if len(calls)==1:raise transaction.Blocked('renderer_cas_interrupted')
        with patch.object(transaction,'reconstruct',return_value=stage),patch.object(transaction,'context',return_value={}),\
             patch.object(transaction,'revalidate_inputs'),patch.object(transaction,'DockerRuntime'),\
             patch.object(transaction,'save',side_effect=lambda p,s:written.append(copy.deepcopy(s))),\
             patch.object(transaction.renderer_root,'recovery_authority',return_value=reverse) as invert,\
             patch.object(transaction.renderer_root,'apply_paused',side_effect=cas),\
             patch.object(transaction.renderer_root,'verify_live',return_value={'verified':True}) as live,\
             patch.object(transaction,'verify_post_apply',return_value={'verified':True}) as proof:
            with self.assertRaisesRegex(transaction.Blocked,'renderer_cas_interrupted'):
                transaction.rollback(None,Path('/private'),state,receipt,'101',{})
            stage.restore_original_templates.assert_not_called();stage.restart.assert_not_called();proof.assert_not_called()
            self.assertEqual(state['phase'],'ROLLBACK_TEMPLATES')
            transaction.rollback(None,Path('/private'),state,receipt,'101',{})
            stage.renderer_post_start(stage)
        invert.assert_called_once();stage.adopt_rollback_target.assert_called_once()
        stage.restore_original_templates.assert_called_once();proof.assert_called_once();stage.restart.assert_called_once()
        live.assert_called_once_with(stage,reverse)
        self.assertEqual(state['status'],'ROLLED_BACK')

    def test_post_apply_proof_failure_records_blocked_and_never_restarts(self):
        stage=Mock();stage.final_path=Path('/selected-store')
        stage.refence.return_value={'verified':True}
        receipt={'operation':'123456abcdef','target':{},'expires_at':(dt.datetime.now(dt.timezone.utc)+dt.timedelta(hours=1)).isoformat()}
        state={'phase':'PAUSED_APPLY','status':'WAITING','operation':'123456abcdef','target':{},'authorization':receipt,
               'events':[],'provenance':{},'cut':{},'context':{}}
        written=[]
        with patch.object(transaction,'reconstruct',return_value=stage),\
             patch.object(transaction,'context',return_value={}),\
             patch.object(transaction,'revalidate_inputs'),\
             patch.object(transaction,'DockerRuntime'),\
             patch.object(transaction,'save',side_effect=lambda p,s:written.append(copy.deepcopy(s))),\
             patch.object(transaction,'verify_post_apply',side_effect=transaction.Blocked('rollout_native_store_changed')):
            with self.assertRaises(transaction.Blocked):
                transaction.finish(None,Path('/private'),state,receipt,'101',{})
        stage.restart.assert_not_called();stage.verified.assert_not_called()
        self.assertEqual(written[-1]['status'],'BLOCKED')
        self.assertEqual(written[-1]['fence_status'],'VERIFIED')

if __name__=='__main__':unittest.main()
