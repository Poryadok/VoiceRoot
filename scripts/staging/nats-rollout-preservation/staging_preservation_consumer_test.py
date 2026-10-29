"""Consumer routing tests; external cohort qualification and Kubernetes are fixtures."""
import copy
import hashlib
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import Mock,patch

import commands
import bridge_root
import migrations
import operator_retirement
import resource_claim
import root_main
from resource_claim_test import context,card
import staging_cohort_preservation
import staging_space_native
import staging_core_release
import transaction
import guard


class ConsumerTests(unittest.TestCase):
    def session(self,root,plan):
        (root/'installed').mkdir(mode=0o700)
        c=context();journal=resource_claim.RootJournal(root)
        session=resource_claim.Session(c,'a'*12,'b'*64,lambda:copy.deepcopy(card(c)),journal)
        session.admit(start=True)
        target={'mode':'full','migration_sha256':transaction.canonical(plan)}
        row=journal.read();row['target_sha256']=transaction.canonical(target);journal.save(row)
        session.expected_target=row['target_sha256']
        state={'operation':session.operation,'target':target,'migrations':copy.deepcopy(plan),
               'resource_fence':session.binding()}
        return session,state

    def test_bound_session_requires_exact_target_and_qualified_cohort(self):
        with tempfile.TemporaryDirectory(dir='/root') as path:
            plan=[{'database':'space'}];session,state=self.session(Path(path),plan)
            with resource_claim.command_scope(session.check),patch.object(staging_cohort_preservation,'load_qualified',return_value={'qualified':'fixture'}) as loader:
                self.assertEqual(migrations.require_preserving_admission('full',plan,state=state),{'qualified':'fixture'})
                altered=copy.deepcopy(state);altered['target']['migration_sha256']='f'*64
                with self.assertRaisesRegex(Exception,'staging_migration_preservation_authority_missing'):
                    migrations.require_preserving_admission('full',plan,state=altered)
                self.assertEqual(loader.call_count,1)

    def test_full_prepare_captures_source_before_calling_cohort_producer(self):
        with tempfile.TemporaryDirectory(dir='/root') as path:
            root=Path(path);(root/'installed').mkdir(mode=0o700);(root/'installed'/'sources').mkdir(mode=0o700)
            c=context();session=resource_claim.Session(c,'a'*12,'b'*64,lambda:copy.deepcopy(card(c)),resource_claim.RootJournal(root))
            session.admit(start=True);actions=object.__new__(bridge_root.Actions);actions.tt_session=session
            actions._idle=Mock();actions._dispatcher=Mock(return_value=(42,{'head_sha':c['commit']}))
            request={'mode':'full','nonce':'a'*64,'token':'fixture','changed_services':[]}
            approved={'source_sha':c['commit']};result={'status':'WAITING','phase':'AWAITING_OFF_NODE'}
            def producer(actual_actions,actual_request,actual_approved,execution,workspace):
                self.assertTrue((root/'installed'/'source-authority'/(session.operation+'.json')).exists())
                self.assertEqual((actual_actions,actual_request,actual_approved),(actions,request,approved))
                self.assertEqual(workspace,root/'installed'/'sources'/session.operation)
                return result
            with resource_claim.command_scope(session.check),patch.object(bridge_root,'INSTALLED',root/'installed'), \
                 patch.object(bridge_root.source_authority,'capture_source',return_value=approved), \
                 patch.object(staging_cohort_preservation,'prepare',side_effect=producer):
                self.assertEqual(actions._prepare(request),result)

    def test_authorize_continues_only_after_fixed_offnode_qualification(self):
        with tempfile.TemporaryDirectory(dir='/root') as path:
            root=Path(path);(root/'installed').mkdir(mode=0o700)
            c=context();session=resource_claim.Session(c,'a'*12,'b'*64,lambda:copy.deepcopy(card(c)),resource_claim.RootJournal(root))
            session.admit(start=True);row=session.journal.read();row.update(phase='cohort-preservation',target_sha256='d'*64,cohort_preservation_sha256='d'*64)
            session.journal.save(row);session.expected_target='d'*64;session._cohort_lineage='d'*64
            actions=object.__new__(bridge_root.Actions);actions.tt_session=session
            receipt={'qualified':'fixture'};loaded={'request':{'mode':'full','changed_services':[],'gis_first_install':True}}
            state={'operation':session.operation,'phase':'PAUSED_APPLY','status':'WAITING','fence_status':'VERIFIED',
                   'cut':{'manifest':{'archive_sha256':'f'*64},'manifest_sha256':'e'*64},'contract':{}}
            actions._prepare=Mock(return_value=state);order=[]
            def qualify(actual,request):order.append('qualified');return receipt
            def authorize(*args):order.append('authorized')
            with resource_claim.command_scope(session.check),patch.object(staging_cohort_preservation,'qualify_offnode',side_effect=qualify), \
                 patch.object(staging_cohort_preservation,'load_prepared',return_value=loaded), \
                 patch.object(bridge_root.root_cli,'operation_path',return_value=root),patch.object(bridge_root,'Kube'), \
                 patch.object(transaction,'authorize',side_effect=authorize),patch.object(bridge_root.grp,'getgrnam',return_value=SimpleNamespace(gr_gid=1)), \
                 patch.object(bridge_root.os,'chown'),patch.object(Path,'chmod'):
                result=actions.execute({'action':'authorize','operation':session.operation,'artifact_id':42,'token':'fixture'})
                self.assertEqual(order,['qualified','authorized']);self.assertEqual(result['phase'],'PAUSED_APPLY')
                self.assertEqual(actions._prepare.call_args.kwargs,{'cohort_ref':receipt})
            actions._prepare.reset_mock()
            with resource_claim.command_scope(session.check),patch.object(staging_cohort_preservation,'qualify_offnode',side_effect=RuntimeError('offnode unqualified')):
                with self.assertRaisesRegex(RuntimeError,'offnode unqualified'):
                    actions.execute({'action':'authorize','operation':session.operation,'artifact_id':42,'token':'fixture'})
            actions._prepare.assert_not_called()

    def test_space_uses_native_receipt_without_generic_job(self):
        with tempfile.TemporaryDirectory(dir='/root') as path:
            plan=[{'database':'space','image':migrations.IMAGE,'secret_key':'SPACE_DATABASE_URL',
                   'files':{'000001_fixture.up.sql':'select 1'},'hashes':{'000001_fixture.up.sql':'fixture'}}]
            session,state=self.session(Path(path),plan);kube=Mock();kube.secret_meta.return_value={'uid':'u','resourceVersion':'1'}
            stage=Mock();stage.operation=session.operation;journal=Mock();receipt={'status':'COMPLETE','fixture':'native'}
            with resource_claim.command_scope(session.check),patch.object(staging_cohort_preservation,'load_qualified',return_value={'qualified':'fixture'}), \
                 patch.object(migrations,'preflight',return_value=kube.secret_meta.return_value), \
                 patch.object(staging_space_native,'execute_live',return_value=receipt) as execute, \
                 patch.object(staging_space_native,'load_live_completed',return_value=receipt):
                self.assertEqual(migrations.execute(kube,stage,plan,'full',kube.secret_meta.return_value,journal,state=state),[])
                execute.assert_called_once_with(session);kube.run.assert_not_called()
                self.assertEqual(state['native_space_migration'],receipt)
                journal.assert_called_once_with({'kind':'native_space_migration_complete','receipt_sha256':transaction.canonical(receipt)})

    def test_partial_preserved_authorization_never_enters_legacy_custody(self):
        actions=object.__new__(bridge_root.Actions)
        state={'operation':'a'*12,'target':{'mode':'full','cohort_preservation_sha256':'c'*64},'migrations':[]}
        actions.state=Mock(return_value=(Path('/unused'),state))
        with patch.object(bridge_root.github_custody,'verify_artifact') as custody,patch.object(bridge_root,'save') as save:
            with self.assertRaisesRegex(Exception,'cohort_preserved_authorization_already_started'):
                actions.execute({'action':'authorize','operation':'a'*12})
            custody.assert_not_called();save.assert_not_called()

    def test_legacy_retirement_does_not_accept_native_space_receipt(self):
        with patch.object(operator_retirement,'private_bytes') as read:
            with self.assertRaisesRegex(Exception,'operator_retirement_binding_invalid'):
                operator_retirement.verify_terminal(Path('/unused'),{'native_space_migration':{'status':'COMPLETE'}})
            read.assert_not_called()
        with tempfile.TemporaryDirectory(dir='/root') as path,patch.object(guard,'ROOT',Path(path)):
            native=Path(path)/'installed'/'cohort-preservation'/('a'*12)/'space-live'
            native.mkdir(parents=True)
            with patch.object(operator_retirement,'private_bytes') as read:
                with self.assertRaisesRegex(Exception,'operator_retirement_binding_invalid'):
                    operator_retirement.verify_terminal(Path('/unused'),{'operation':'a'*12})
                read.assert_not_called()

    def test_finish_core_proofs_hold_actual_lock_and_never_replay_existing_smoke(self):
        with tempfile.TemporaryDirectory(dir='/root') as path,patch.object(root_main,'ROOT',Path(path)),patch.object(guard,'ROOT',Path(path)):
            root=Path(path);session,state=self.session(root,[])
            state.update(status='PASS',phase='POST_RESTART');state['target']['cohort_preservation_sha256']='c'*64
            row=session.journal.read();row['target_sha256']=transaction.canonical(state['target']);session.journal.save(row)
            session.expected_target=row['target_sha256'];state['resource_fence']=session.binding()
            base=root/('rollout-'+session.operation);base.mkdir(mode=0o700)
            release=Mock();smoke_phase=Mock();smoke_phase.journal=Mock();smoke=Mock()
            smoke.StagingCoreSmoke.return_value=smoke_phase;smoke_phase.run.return_value={'status':'done'}
            smoke.verify_completed.return_value={'status':'done'};smoke.recover_completed.return_value=(smoke_phase,{'status':'done'})
            actions=object.__new__(bridge_root.Actions);actions.tt_session=session
            with root_main.operation_lock() as lock,resource_claim.command_scope(session.check), \
                 patch.object(staging_core_release,'CoreReleasePhase',return_value=release), \
                 patch.object(staging_core_release,'verify_completed'), \
                 patch.object(staging_cohort_preservation,'shared_module',return_value=smoke):
                session.operation_lock=lock;actions._finish_core_proofs(state)
                smoke_phase.run.assert_called_once();smoke_phase.journal.close.assert_called_once()
                (base/'core-smoke').mkdir();smoke_phase.run.reset_mock();actions._finish_core_proofs(state)
                smoke.recover_completed.assert_called_once_with(session,release);smoke_phase.run.assert_not_called()
                with self.assertRaisesRegex(Exception,'staging_core_finish_reconciliation_required'):
                    actions._finish_core_proofs(state,recover_only=True)
                smoke_phase.run.assert_not_called()
                (base/'core-release.json').write_text('{}');actions._finish_core_proofs(state,recover_only=True)
                self.assertEqual(smoke.recover_completed.call_count,2);smoke_phase.run.assert_not_called()
            with resource_claim.command_scope(session.check):
                with self.assertRaisesRegex(Exception,'operation_lock_proof_invalid'):actions._finish_core_proofs(state)

    def test_historical_native_receipt_rejects_tampered_effect_and_rollout_event(self):
        operation='a'*12;ctx={'operation_id':'stg-run-17','commit':'c'*40};binding={'context':ctx,'operation':operation}
        session=SimpleNamespace(operation=operation,context=ctx)
        tool={'immutable_image':'ghcr.io/poryadok/voiceroot/space-native-installer@sha256:'+'c'*64,
              'source_inventory':{'build_source_sha256':'d'*64},'source_sha':ctx['commit'],
              'platform_digest':'sha256:'+'c'*64,'config_digest':'sha256:'+'e'*64}
        catalog={'engine':{'namespace_uid':'ns'},'server_start':'start','databases':{'space_db':42},
                 'properties':{'space_db':{'owner':'admin'}}}
        metadata={'catalog_sha256':transaction.canonical(catalog),'source':{'catalog':{'space_db':{'database_owner':'admin','schema_owner':'admin'}}}}
        archive={'binding':{'snapshot_id':'snapshot-1'},'catalog_ref':{'record_sha256':transaction.canonical(catalog)},
                 'metadata_ref':{'record_sha256':transaction.canonical(metadata)}}
        qualified={'archive_sha256':transaction.canonical(archive),'namespace_uid':'ns'}
        rehearsal={'proof':'fixture'};offnode={'proof':{'native_space':rehearsal,'space_native_rehearsal_sha256':transaction.canonical(rehearsal)}}
        original={'engine':catalog['engine'],'server_start':'start','database_name':'space_db','database_oid':42,
                  'database_owner':'admin','schema_owner':'admin','installer_role':'admin'}
        authority={'schema':'voice-staging-space-native-authority-v1','operation':operation,'preservation_sha256':transaction.canonical(qualified),
                   'tool':tool,'rehearsal':rehearsal,'original':original}
        docs=staging_space_native._live_documents(session,tool,'node-a','local-path','snapshot-1')
        plan={'schema':'voice-staging-space-live-plan-v1','operation':operation,'binding':binding,'authority_sha256':transaction.canonical(authority),
              'archive_sha256':qualified['archive_sha256'],'namespace_uid':'ns','documents':docs,'node':'node-a',
              'tool_platform_digest':tool['platform_digest'],'tool_config_digest':tool['config_digest'],
              'postgres_pvc_uid':'other-pvc','postgres_pv_uid':'other-pv'}
        uid=lambda n:'%08d-1111-4111-8111-111111111111'%n
        volume={'pvc_uid':uid(1),'pvc_rv':'1','pv_name':'pv-owned','pv_uid':uid(2),'pv_rv':'2'}
        hashes={'/evidence/origin.json':'1'*64,'/evidence/completed.jsonl':'2'*64}
        with tempfile.TemporaryDirectory(dir='/root') as path,patch.object(guard,'ROOT',Path(path)):
            root=Path(path);base=root/'installed'/'cohort-preservation'/operation;live=base/'space-live';effects=live/'effects'
            effects.mkdir(parents=True,mode=0o700);rollout=root/('rollout-'+operation);rollout.mkdir(mode=0o700)
            write=staging_cohort_preservation._write
            for name,value in [('plan.json',plan)]:write(live/name,value)
            for name,value in [('archive-record.json',archive),('catalog.json',catalog),('metadata.json',metadata),
                               ('space-tool.json',{'authenticated_publication':{'publication':tool}}),('offnode-restore.json',offnode)]:write(base/name,value)
            closed={};results={}
            for index,kind in enumerate(('pvc','bind','upgrade','verify'),1):
                job_uid=volume['pvc_uid'] if kind=='pvc' else uid(index+2)
                event={'schema':'voice-staging-space-live-effect-v1','operation':operation,'effect':kind+'-create',
                    'plan_sha256':transaction.canonical(plan),'binding':binding,'argv':staging_cohort_preservation.PREFIX+['create','-f','/dev/stdin','-o','json'],
                    'body_sha256':hashlib.sha256(staging_cohort_preservation.private._bytes(docs[kind])).hexdigest(),'status':'DONE',
                    'response':{'uid':job_uid,'resourceVersion':'7','creationTimestamp':'t','spec_sha256':'a'*64}}
                write(effects/(kind+'-create.json'),event)
                if kind=='pvc':continue
                options={'apiVersion':'v1','kind':'DeleteOptions','propagationPolicy':'Foreground','preconditions':{'uid':job_uid,'resourceVersion':'8'}}
                event.update(effect=kind+'-delete',argv=staging_cohort_preservation.PREFIX+['delete','--raw=/apis/batch/v1/namespaces/voice-staging/jobs/'+docs[kind]['metadata']['name'],'-f','/dev/stdin'],
                    body_sha256=hashlib.sha256(staging_cohort_preservation.private._bytes(options)).hexdigest(),response={'uid':job_uid,'resourceVersion':'8'})
                write(effects/(kind+'-delete.json'),event)
                closed[kind]={'schema':'voice-staging-space-live-job-closed-v1','status':'DONE','operation':operation,'kind':kind,
                    'plan_sha256':transaction.canonical(plan),'job_uid':job_uid,'pod_uid':uid(index+6),'volume':volume}
                write(live/(kind+'-closed.json'),closed[kind])
                result={'operation':operation,'plan_sha256':transaction.canonical(plan),'job_uid':job_uid,'pod_uid':uid(index+6),'volume':volume}
                if kind=='bind':result.update(schema='voice-staging-space-live-bind-result-v1')
                else:result.update(schema='voice-staging-space-live-job-result-v1',kind=kind,pod_before_rv='9',pod_after_rv='10',hashes=hashes)
                results[kind]=result;write(live/(kind+'-result.json'),result)
            receipt={'schema':'voice-staging-space-live-complete-v1','status':'COMPLETE','operation':operation,
                'plan_sha256':transaction.canonical(plan),'authority_sha256':plan['authority_sha256'],'volume':volume,
                'upgrade_result_sha256':transaction.canonical(results['upgrade']),'verify_result_sha256':transaction.canonical(results['verify']),
                'upgrade_closed_sha256':transaction.canonical(closed['upgrade']),'verify_closed_sha256':transaction.canonical(closed['verify']),
                'evidence_hashes':hashes}
            write(live/'complete.json',receipt)
            checkpoint={'native_space_migration':receipt,'events':[{'kind':'native_space_migration_complete','receipt_sha256':transaction.canonical(receipt)}]}
            write(rollout/'checkpoint.json',checkpoint)
            with patch.object(staging_cohort_preservation,'validate_historical_capture',return_value=qualified), \
                 patch.object(resource_claim,'historical_binding',return_value=binding):
                self.assertEqual(staging_space_native.validate_historical_live_completed(operation),receipt)
                original_effect=staging_cohort_preservation._read(effects/'upgrade-create.json')
                changed=copy.deepcopy(original_effect);changed['body_sha256']='f'*64
                staging_cohort_preservation.save(effects/'upgrade-create.json',changed)
                with self.assertRaisesRegex(Exception,'space_live_history_changed'):staging_space_native.validate_historical_live_completed(operation)
                staging_cohort_preservation.save(effects/'upgrade-create.json',original_effect)
                checkpoint['events'][0]['receipt_sha256']='f'*64
                staging_cohort_preservation.save(rollout/'checkpoint.json',checkpoint)
                with self.assertRaisesRegex(Exception,'space_live_history_changed'):staging_space_native.validate_historical_live_completed(operation)

if __name__=='__main__':unittest.main()
