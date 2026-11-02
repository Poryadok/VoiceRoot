"""Explicit P2 source8/live4 descriptors; never current4 witness admission."""
import copy
import hashlib
import json
from pathlib import Path
import shutil
import tempfile
import unittest
from unittest.mock import patch
import actor_root as root
import actor_root_test as legacy
import bridge_root
import commands
import guard
import resource_claim
import resource_claim_test
import staging_prerequisites


def digest(row):
    return hashlib.sha256(json.dumps(row,sort_keys=True,separators=(',',':')).encode()).hexdigest()


class Tests(unittest.TestCase):
    fixture=legacy.Tests.fixture

    def inputs(self,folder):
        source=Path(folder)
        kube,stage,*_=self.fixture(source,'story')
        stage.operation='a'*12
        original=Path(__file__).resolve().parents[3]/'src/backend/migrations/story_db'
        destination=source/'src/backend/migrations/story_db'
        shutil.copytree(original,destination)
        files={p.name:p.read_text() for p in destination.glob('*.sql')}
        plan=[{'database':'story','files':files,'hashes':{n:root.hash_bytes(v.encode()) for n,v in files.items()}}]
        binding={'operation':'a'*12,'source_sha':'b'*40,'source_capture_sha256':'c'*64,
            'helper_sha256':'d'*64,'context_sha256':'e'*64,'migration_plan_sha256':digest(plan)}
        kube.run.return_value={'database':'story_db','username':'voice','version':4,'dirty':False}
        return source,kube,stage,plan,binding

    def test_explicit_source8_live4_target8_descriptor_keeps_legacy_closed(self):
        with tempfile.TemporaryDirectory() as td:
            source,kube,stage,plan,binding=self.inputs(td)
            with self.assertRaises(root.Blocked):root.story_schema(kube,source,stage)
            kube.run.assert_not_called()
            with patch('space_authority.capture',return_value={'postgres':'actual-bound'}):
                row=root.story_schema_p2(kube,source,stage,plan,binding)
            self.assertEqual(row['schema'],'voice-story-baseline4-target8-composition-v1')
            self.assertEqual(row['baseline']['observation'],{'database':'story_db','version':4,'dirty':False})
            self.assertEqual(row['baseline']['migration_hashes'],root.STORY_UP)
            self.assertEqual(row['candidate']['version'],8)
            self.assertEqual(len(row['candidate']['up_sha256']),8)
            self.assertEqual(row['binding'],binding)
            self.assertEqual(row['retained_image'],stage.old_images['voice-story/story'])
            self.assertNotIn('qualified',row)

    def test_inventory_and_plan_drift_reject_before_database_transport(self):
        for fault in ('extra9','missing','modified1','modified8','rename','plan4','plan-bytes','plan-hash','binding-plan','binding-extra','image'):
            with self.subTest(fault=fault),tempfile.TemporaryDirectory() as td:
                source,kube,stage,plan,binding=self.inputs(td)
                folder=source/'src/backend/migrations/story_db'
                if fault=='extra9':(folder/'000009_foreign.up.sql').write_text('SELECT 1;')
                elif fault=='missing':(folder/'000008_reference_inventory.up.sql').unlink()
                elif fault in ('modified1','modified8'):
                    path=folder/('000001_init.up.sql' if fault=='modified1' else '000008_reference_inventory.up.sql')
                    path.write_bytes(path.read_bytes()+b'\n')
                elif fault=='rename':(folder/'000008_reference_inventory.up.sql').rename(folder/'000008_renamed.up.sql')
                elif fault=='plan4':
                    plan[0]['files']={n:v for n,v in plan[0]['files'].items() if int(n[:6])<=4}
                    plan[0]['hashes']={n:root.hash_bytes(v.encode()) for n,v in plan[0]['files'].items()}
                    binding['migration_plan_sha256']=digest(plan)
                elif fault=='plan-bytes':plan[0]['files']['000008_reference_inventory.up.sql']+='\n'
                elif fault=='plan-hash':plan[0]['hashes']['000008_reference_inventory.up.sql']='f'*64
                elif fault=='binding-plan':binding['migration_plan_sha256']='f'*64
                elif fault=='binding-extra':binding['qualified']=True
                elif fault=='image':stage.old_images['voice-story/story']='story:mutable'
                with self.assertRaises(root.Blocked):root.story_schema_p2(kube,source,stage,plan,binding)
                kube.run.assert_not_called()

    def test_candidate8_never_substitutes_live4_database_or_authority(self):
        for change in ({'version':8},{'version':True},{'dirty':True},{'database':'file_db'},{'extra':0}):
            with self.subTest(change=change),tempfile.TemporaryDirectory() as td:
                source,kube,stage,plan,binding=self.inputs(td)
                kube.run.return_value.update(change)
                with patch('space_authority.capture',return_value={'postgres':'actual-bound'}),self.assertRaises(root.Blocked):
                    root.story_schema_p2(kube,source,stage,plan,binding)

    def test_operation_and_drift_during_sql_cannot_replace_bound_profile(self):
        for fault in ('operation','source','binding','plan-and-binding'):
            with self.subTest(fault=fault),tempfile.TemporaryDirectory() as td:
                source,kube,stage,plan,binding=self.inputs(td)
                clean=copy.deepcopy(kube.run.return_value)
                if fault=='operation':binding['operation']='f'*12
                else:
                    def observe(*args,**kwargs):
                        if fault=='source':
                            path=source/'src/backend/migrations/story_db/000008_reference_inventory.up.sql'
                            path.write_bytes(path.read_bytes()+b'\n')
                        elif fault=='binding':binding['context_sha256']='f'*64
                        elif fault=='plan-and-binding':
                            plan[0]['database']='other';binding['migration_plan_sha256']=digest(plan)
                        return clean
                    kube.run.side_effect=observe
                with patch('space_authority.capture',return_value={'postgres':'actual-bound'}),self.assertRaises(root.Blocked):
                    root.story_schema_p2(kube,source,stage,plan,binding)

    def test_p2_actor_labels_only_baseline4_and_retains_fresh_mount_checks(self):
        with tempfile.TemporaryDirectory() as td:
            source,kube,stage,plan,binding=self.inputs(td)
            original=Path(__file__).resolve().parents[3]
            (source/root.ACL).write_bytes((original/root.ACL).read_bytes())
            account='A'+'B'*55
            with patch.object(root,'ACCOUNT_SHA',root.hash_bytes(account.encode())),\
                 patch('space_authority.capture',return_value={'postgres':'actual-bound'}),\
                 patch.object(root.actor_verification,'verify',return_value={'signed_chain_verified':True}) as actor:
                result=root.preflight(kube,source,source,{'nats-rollout-preservation/bootstrap-renewer':'c'*64},
                    stage,['story'],None,story_composition=(plan,binding),installation=self.installation)
                self.assertEqual(result['compatible_story_candidate']['schema'],'voice-reviewed-story-baseline4-compatible-pair-v1')
                self.assertEqual(result['story_schema']['binding'],binding)
                self.assertEqual(result['mounts']['story']['key'],'story.creds')
                self.assertEqual(actor.call_count,1)
                stage.snapshots['voice-story']['spec']['template']['spec']['volumes'][0]['secret']['secretName']='foreign'
                with self.assertRaises(root.Blocked):
                    root.preflight(kube,source,source,{'nats-rollout-preservation/bootstrap-renewer':'c'*64},
                        stage,['story'],None,story_composition=(plan,binding),installation=self.installation)
                self.assertEqual(actor.call_count,1)

    def test_precut_composition_consumes_v2_receipt_and_all_db_plan(self):
        original=Path(__file__).resolve().parents[3]
        with tempfile.TemporaryDirectory() as td:
            installed=Path(td)/'installed';installed.mkdir(mode=0o700)
            context=resource_claim_test.context();card=resource_claim_test.card(context)
            session=resource_claim.Session(context,'a'*12,'b'*64,lambda:copy.deepcopy(card),resource_claim_test.MemoryJournal())
            session.admit(start=True)
            actions=object.__new__(bridge_root.Actions);actions._installation=session.installation;actions.tt_session=session
            workspace=installed/'sources'/session.operation;workspace.mkdir(parents=True,mode=0o700)
            for relative in ('src/backend/migrations','src/backend/auth/src/main/resources/db/migration'):
                shutil.copytree(original/relative,workspace/relative)
            # Match capture_source custody instead of copying a Windows bind's
            # permissive directory modes into the private Linux workspace.
            for path in workspace.rglob('*'):path.chmod(0o700 if path.is_dir() else 0o600)
            approved={'source_sha':context['commit'],'source_files':{
                str(path.relative_to(workspace)):root.hash_bytes(path.read_bytes())
                for path in workspace.rglob('*') if path.is_file()}}
            execution={'head_sha':context['commit']}
            receipt={'schema':'voice-staging-source-capture-v2','installation_sha256':session.installation.sha256,'operation':session.operation,'context':session.context,
                'helper_sha256':session.helper,'approved_sha256':digest(approved),'execution_sha256':digest(execution)}
            plan={'schema':'voice-staging-prerequisites-plan-v3','installation_sha256':session.installation.sha256,
                'operation':session.operation,'context':session.context,'helper_sha256':session.helper,
                'source_capture_sha256':digest(receipt),'approved_sha256':digest(approved),'execution_sha256':digest(execution),
                'namespace_uid':'fixture-namespace','marker_sha256':'1'*64,'baseline_sha256':'2'*64,
                'actual_before':context['images'],'policy_before_sha256':'3'*64,
                'children':{slot: None for slot in staging_prerequisites.SLOTS},'auxiliary_images':{},
                'renderer_authority_sha256':None,'previous_active_sha256':None}
            prepared={'schema':'voice-staging-prerequisites-prepared-v1','plan':plan,
                'gis_authority_sha256':'4'*64,'request':{'mode':'full','changed_services':[], 'gis_first_install':False}}
            def save(path,row):
                path.parent.mkdir(parents=True,exist_ok=True,mode=0o700)
                path.write_text(json.dumps(row));path.chmod(0o600)
            parent=installed/'prerequisites'/session.operation
            save(parent/'prepared.json',prepared);save(parent/'execution.json',execution)
            receipt_path=installed/'source-authority'/(session.operation+'.json');save(receipt_path,receipt)
            with patch.object(bridge_root,'INSTALLED',installed),patch.object(guard,'ROOT',Path(td)),patch.object(commands,'authority_guard',session.check):
                request={'mode':'full','changed_services':[]}
                migrations,binding=bridge_root._cohort_story_composition(actions,request,approved,workspace)
                self.assertEqual(len(migrations),17)
                self.assertEqual(binding['migration_plan_sha256'],digest(migrations))
                self.assertEqual(binding['source_capture_sha256'],digest(receipt))
                story=next(row for row in migrations if row['database']=='story')
                self.assertEqual({n:v for n,v in story['hashes'].items() if n.endswith('.up.sql')},root.STORY_P2_UP)
                prepared['request']['mode']='app-only';save(parent/'prepared.json',prepared)
                other,other_binding=bridge_root._cohort_story_composition(actions,{'mode':'app-only','changed_services':[]},approved,workspace)
                self.assertEqual(other,migrations);self.assertEqual(other_binding,binding)
                prepared['request']['mode']='full';save(parent/'prepared.json',prepared)
                for fault in ('receipt','source','parent'):
                    with self.subTest(fault=fault):
                        if fault=='receipt':save(receipt_path,dict(receipt,extra=True))
                        elif fault=='source':
                            save(receipt_path,receipt)
                            path=workspace/'src/backend/migrations/story_db/000008_reference_inventory.up.sql'
                            raw=path.read_bytes();path.write_bytes(raw+b'\n')
                        else:
                            path.write_bytes(raw);save(parent/'prepared.json',dict(prepared,plan=dict(plan,helper_sha256='f'*64)))
                        with self.assertRaises(bridge_root.Blocked):
                            bridge_root._cohort_story_composition(actions,request,approved,workspace)


if __name__=='__main__':unittest.main()
