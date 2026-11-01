"""Domain and lineage checks for the shared fenced cohort journal."""
import copy
import json
from pathlib import Path
import sys
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import Mock, patch

import resource_claim as claim
from controller import Blocked
from resource_claim_test import MemoryJournal, card, context


class CohortInstallationJournalTests(unittest.TestCase):
    def test_story_composition_requires_scoped_source_and_parent(self):
        import bridge_root,staging_prerequisites as prerequisites
        c=context();session=claim.Session(c,'a'*12,'b'*64,lambda:card(c),MemoryJournal());session.admit(start=True)
        actions=SimpleNamespace(tt_session=session,installation=session.installation)
        approved={'source_sha':c['commit'],'source_files':{}};execution={'fixture':'execution'}
        source={'schema':'voice-staging-source-capture-v2','installation_sha256':session.installation.sha256,
                'operation':session.operation,'context':c,'helper_sha256':session.helper,
                'approved_sha256':claim.digest(approved),'execution_sha256':claim.digest(execution)}
        plan={'schema':'voice-staging-prerequisites-plan-v3','installation_sha256':session.installation.sha256,
              'operation':session.operation,'context':c,'helper_sha256':session.helper,'source_capture_sha256':claim.digest(source),
              'approved_sha256':claim.digest(approved),'execution_sha256':claim.digest(execution),'namespace_uid':'namespace',
              'marker_sha256':'1'*64,'baseline_sha256':'2'*64,'actual_before':card(c)['observed']['images'],
              'policy_before_sha256':'3'*64,'children':{slot:{} for slot in prerequisites.SLOTS},
              'auxiliary_images':{},'renderer_authority_sha256':None,'previous_active_sha256':None}
        prepared={'schema':'voice-staging-prerequisites-prepared-v1','plan':plan,
                  'request':{'mode':'full','changed_services':[],'gis_first_install':False},'gis_authority_sha256':'4'*64}
        def read(path):
            if path.name=='prepared.json':return prepared
            if path.name=='execution.json':return execution
            return source
        with patch.object(bridge_root,'private_json',side_effect=read),patch.object(prerequisites,'_read_private',return_value=json.dumps(source).encode()), \
             patch.object(bridge_root.compiler,'compile_migrations',return_value=[]) as compile_plan:
            self.assertEqual(bridge_root._cohort_story_composition(actions,{'mode':'full','changed_services':[]},approved,Path('/unused'))[0],[])
            for mutation in ('legacy','foreign-source','foreign-parent','unknown','missing'):
                original_source=copy.deepcopy(source);original_plan=copy.deepcopy(plan)
                if mutation=='legacy':source['schema']='voice-staging-source-capture-v1';del source['installation_sha256']
                elif mutation=='foreign-source':source['installation_sha256']='f'*64
                elif mutation=='foreign-parent':plan['installation_sha256']='f'*64
                elif mutation=='unknown':plan['unexpected']=True
                else:del plan['baseline_sha256']
                with self.subTest(mutation=mutation),self.assertRaises(Blocked):
                    bridge_root._cohort_story_composition(actions,{'mode':'full','changed_services':[]},approved,Path('/unused'))
                source.clear();source.update(original_source);plan.clear();plan.update(original_plan)
            self.assertEqual(compile_plan.call_count,1)

    def test_qualified_reader_rejects_mutated_initial_fence(self):
        import staging_cohort_preservation as cohort
        c=context();session=claim.Session(c,'a'*12,'b'*64,lambda:card(c),MemoryJournal());session.admit(start=True)
        plan={'fixture':'p2'};session.journal.row.update(target_sha256='c'*64)
        initial={**session.binding(),'target_sha256':claim.digest(plan)}
        for mutation in ('missing','foreign','legacy','context','helper','operation','target'):
            row=copy.deepcopy(initial)
            if mutation=='missing':del row['installation_sha256']
            elif mutation=='foreign':row['installation_sha256']='f'*64
            elif mutation=='legacy':row['schema']='voice-staging-resource-fence-v1';del row['installation_sha256']
            elif mutation=='context':row['context']['revision']+=1
            elif mutation=='helper':row['helper_sha256']='f'*64
            elif mutation=='operation':row['operation']='f'*12
            else:row['target_sha256']='f'*64
            with self.subTest(mutation=mutation),patch.object(cohort,'load_plan',return_value=plan), \
                 patch.object(cohort,'_phase_continuity'),patch.object(cohort,'_no_pending'), \
                 patch.object(cohort,'_state',return_value={'initial_resource_fence':row}),patch.object(cohort,'_read') as read:
                with self.assertRaisesRegex(Blocked,'cohort_initial_fence_changed'):cohort.load_qualified(session)
                read.assert_not_called()
        with patch.object(cohort,'load_plan',return_value=plan),patch.object(cohort,'_phase_continuity'), \
             patch.object(cohort,'_no_pending'),patch.object(cohort,'_state',return_value={'initial_resource_fence':initial}), \
             patch.object(cohort,'_read',side_effect=RuntimeError('receipt read reached')):
            with self.assertRaisesRegex(RuntimeError,'receipt read reached'):cohort.load_qualified(session)

    def test_prepare_full_story_checks_nats_before_renderer_and_actor(self):
        from contextlib import ExitStack
        import bridge_root,actor_root,renderer_root,nats_root_plan,nats_contract_plan,migrations,source_plan,gis_existing_update
        installation=claim.production_installation();actions=object.__new__(bridge_root.Actions)
        actions._installation=installation;actions.code=Path('/fixture');actions.binding={}
        image='registry/story@sha256:'+'c'*64;renderer='registry/renderer@sha256:'+'d'*64
        stage=SimpleNamespace(snapshots={'voice-story':{'spec':{'template':{'spec':{'containers':[{'name':'story'}]}}}}},
            old_images={'voice-story/story':image,bridge_root.HUB+'/nats-config-renderer':renderer},expected={})
        previous={'operation':'b'*12,'input_target':{'mode':'full'},'target':{'images':{'voice-story/story':image}},
                  'context':{'old_images':{'voice-story/story':image}},
                  'renderer_authority':{'descriptor':{'images':{'target':renderer,'old':renderer}}}}
        request={'mode':'full','changed_services':['story'],'nonce':'a'*64}
        approved={'source_sha':'c'*40,'images':{'story':image,'nats-hub-config-renderer':renderer}}
        prior={'schema':'fixture'}
        enrollment={'schema':'voice-nats-bootstrap-enrollment-scoped-v4','previous_enrollment':prior,
                    'previous_enrollment_sha256':nats_contract_plan.digest(prior),'installation_sha256':'f'*64,
                    'binding':{},'secret':{},'credential_sha256':'a'*64,'verified':True,'auth_proof':{}}
        def read(path):
            if path.name=='policy.json':return {'s3_signing_endpoint':'fixture'}
            if path.name=='active-contract.json':raise FileNotFoundError()
            return enrollment if path.name=='bootstrap-enrollment-v4.json' else prior
        kube=Mock();kube.get.return_value={'data':{'generation':'r20261031','dataPVC':'fixture'}}
        with ExitStack() as stack:
            stack.enter_context(patch.object(migrations,'require_preserving_admission'))
            stack.enter_context(patch.object(actions,'_idle'))
            stack.enter_context(patch.object(actions,'_capture_source',return_value=(1,{},False,Path('/source'),approved)))
            stack.enter_context(patch.object(bridge_root,'Kube',return_value=kube))
            stack.enter_context(patch.object(bridge_root.RolloutStage,'capture',return_value=stage))
            stack.enter_context(patch.object(source_plan,'principal_publisher_source',return_value=False))
            stack.enter_context(patch.object(gis_existing_update,'load_authority',return_value=None))
            stack.enter_context(patch.object(bridge_root,'private_json',side_effect=read))
            stack.enter_context(patch.object(bridge_root,'public_read',return_value=b'{"consumer_pairs":[]}'))
            actor=stack.enter_context(patch.object(actor_root,'preflight',return_value={'fixture':'actor'}))
            renderer_prove=stack.enter_context(patch.object(renderer_root,'prove',return_value={'fixture':'renderer'}))
            validate=stack.enter_context(patch.object(bridge_root.transaction,'validate_full_story_rollback_schema',side_effect=RuntimeError('checked actor')))
            with self.assertRaises(Blocked):actions._prepare(request,previous=previous)
            actor.assert_not_called();renderer_prove.assert_not_called();validate.assert_not_called()
            with patch.object(nats_root_plan,'preflight',return_value={'scripts':{}}):
                with self.assertRaisesRegex(RuntimeError,'checked actor'):actions._prepare(request,previous=previous)
            actor.assert_called_once();renderer_prove.assert_called_once()
            validate.assert_called_once_with(previous,{'fixture':'actor'})

    def test_cohort_checks_enrollment_before_actor_effects(self):
        import bridge_root,actor_root,nats_root_plan,nats_contract_plan
        installation=claim.production_installation();c=context()
        session=claim.Session(c,'a'*12,'b'*64,lambda:card(c),MemoryJournal());session.admit(start=True)
        actions=object.__new__(bridge_root.Actions);actions._installation=installation;actions.tt_session=session
        actions.code=Path('/synthetic');actions.binding={}
        workspace=bridge_root.INSTALLED/'sources'/session.operation
        stage=SimpleNamespace(operation=session.operation,kube=Mock(),snapshots={'voice-story':{}},expected={})
        previous={'schema':'fixture'}
        enrollment={'schema':'voice-nats-bootstrap-enrollment-scoped-v4','previous_enrollment':previous,
                    'previous_enrollment_sha256':nats_contract_plan.digest(previous),'installation_sha256':'f'*64,
                    'binding':{},'secret':{},'credential_sha256':'a'*64,'verified':True,'auth_proof':{}}
        def read(path):
            if path.name=='active-contract.json':raise FileNotFoundError()
            return enrollment if path.name=='bootstrap-enrollment-v4.json' else previous
        with claim.command_scope(session.check),patch.object(bridge_root,'private_json',side_effect=read), \
             patch.object(nats_root_plan,'monitor') as monitor, \
             patch.object(actor_root,'preflight') as actor:
            with self.assertRaises(Blocked):
                bridge_root.capture_cohort_authorities(actions,{'mode':'full'},{'source_sha':c['commit']},workspace,stage,{'consumer_pairs':[]})
            actor.assert_not_called();monitor.assert_not_called();stage.kube.get.assert_not_called()

    def test_historical_bridge_reader_uses_actions_installation(self):
        import bridge_root, paused_recovery
        installation=claim.production_installation();actions=object.__new__(bridge_root.Actions)
        actions._installation=installation;actions.binding={};actions.tt_session=SimpleNamespace(operation='a'*12)
        state={'schema':'nats-rollout-root-v2','operation':'b'*12}
        with patch.object(bridge_root.root_cli,'operation_path',return_value=Path('/unused')), \
             patch.object(bridge_root,'private_json',return_value=state), \
             patch.object(claim,'historical_binding') as historical,patch.object(paused_recovery,'verify_adopted_binding'):
            actions.state('b'*12)
        historical.assert_called_once_with(state,installation=installation)

    def test_reconstructed_renderer_callback_keeps_explicit_installation(self):
        import transaction, renderer_root
        installation=claim.production_installation()
        ctx={key:{} for key in ('expected','snapshots','original_snapshots','old_images','marker','service','final_claim','final_pv')}
        ctx['final_path']='/synthetic';authority={'descriptor':{'fixture':True}}
        state={'context':ctx,'operation':'a'*12,'events':[],'renderer_authority':authority}
        stage=SimpleNamespace()
        with patch.object(transaction,'RolloutStage',return_value=stage),patch.object(renderer_root,'verify_live') as verify:
            result=transaction.reconstruct(None,state,lambda _:None,installation=installation)
            result.renderer_post_start('current')
            verify.assert_called_once_with('current',authority,installation=installation)

    def test_active_release_writer_preserves_scoped_authority(self):
        import bridge_root
        installation=claim.production_installation();actions=object.__new__(bridge_root.Actions);actions._installation=installation
        c=context();helper={'nats-rollout-preservation/resource_claim.py':'b'*64}
        session=claim.Session(c,'a'*12,claim.digest(helper),lambda:card(c),MemoryJournal());session.admit(start=True)
        target={'registry':'registry','tag':c['commit'],'mode':'full','changed_services':[],
                'source_hashes':{'source':'1'*64},'template_hashes':{'voice-user':'2'*64},
                'input_template_hashes':{'voice-user':'3'*64},'images':c['images'],
                'manifest_sha256':'4'*64,'migration_sha256':'5'*64,'nonnats_sha256':'6'*64,
                'installation_sha256':installation.sha256}
        session.journal.row.update(target_sha256=claim.digest(target));actions.tt_session=session
        state={'schema':'nats-rollout-root-v2','installation_sha256':installation.sha256,'operation':'a'*12,
               'resource_fence':session.binding(),'code_capture':helper,
               'context':{'expected':{'namespace_uid':'ns','source_claim_uid':'pvc'}},
               'target':target}
        def read(path):
            if path.name=='checkpoint.json':return copy.deepcopy(state)
            raise FileNotFoundError()
        with claim.command_scope(session.check),patch.object(bridge_root,'private_json',side_effect=read),patch.object(bridge_root,'save') as save:
            actions._active_release(state)
            row=save.call_args.args[1];claim.validate_active_release(row,installation=installation)
            self.assertEqual(row['schema'],'voice-active-release-v1')
            state['target']['installation_sha256']='f'*64
            with self.assertRaises(Blocked):actions._active_release(state)
            state['target']['installation_sha256']=installation.sha256
            fence=state.pop('resource_fence')
            with self.assertRaises(RuntimeError):actions._active_release(state)
            state['resource_fence']=dict(fence,installation_sha256='f'*64)
            with self.assertRaises(RuntimeError):actions._active_release(state)
            state['resource_fence']=fence
            state['target']['manifest_sha256']='e'*64
            with self.assertRaises(RuntimeError):actions._active_release(state)
            state['target']['manifest_sha256']='4'*64
            foreign=dict(row,installation_sha256='f'*64)
            with patch.object(bridge_root,'private_json',side_effect=lambda path:copy.deepcopy(state) if path.name=='checkpoint.json' else foreign):
                with self.assertRaises(RuntimeError):actions._active_release(state)
            self.assertEqual(save.call_count,1)
            session.journal.row.update(status='complete',phase='finished',actual=copy.deepcopy(c['images']))
            actions._active_release(state)
            self.assertEqual(save.call_count,2)

    def test_scoped_root_status_is_redacted_and_rejects_invalid_installation(self):
        import bridge_root
        state={'schema':'nats-rollout-root-v2','installation_sha256':'a'*64,'operation':'b'*12,'status':'WAITING','private':'secret'}
        self.assertEqual(bridge_root.status_summary(state),{'operation':'b'*12,'status':'WAITING'})
        state['installation_sha256']='invalid'
        with self.assertRaises(Blocked):bridge_root.status_summary(state)

    def test_transaction_scoped_capture_dispatches_c1_v2_before_effects(self):
        import transaction, realtime_bootstrap
        installation=claim.production_installation();session=SimpleNamespace(installation=installation)
        receipt={'schema':'voice-staging-realtime-bootstrap-receipt-v2'}
        target={'installation_sha256':installation.sha256,'realtime_bootstrap_sha256':transaction.canonical(receipt)}
        args=(None,Path('/unused'),Path('/unused'),target,{},'a'*12,{},[],[])
        with patch.object(transaction,'save') as save,patch.object(realtime_bootstrap,'load_completed',side_effect=RuntimeError('sentinel')) as load:
            with self.assertRaisesRegex(RuntimeError,'sentinel'):
                transaction._prepare(*args,resource_session=session,realtime_bootstrap_receipt=receipt)
            state=save.call_args.args[1]
            self.assertEqual((state['schema'],state['installation_sha256']),('nats-rollout-root-v2',installation.sha256))
            load.assert_called_once_with(session)
        foreign=dict(target,installation_sha256='f'*64)
        with patch.object(transaction,'save') as save, self.assertRaises(Blocked):
            transaction._prepare(*args[:3],foreign,*args[4:],resource_session=session,realtime_bootstrap_receipt=receipt)
        save.assert_not_called()

    def test_capture_inputs_passes_expected_authority_before_secret_access(self):
        import root_main, bootstrap_selection
        installation=claim.production_installation();kube=Mock()
        with patch.object(bootstrap_selection,'select',side_effect=RuntimeError('sentinel')) as select:
            with self.assertRaisesRegex(RuntimeError,'sentinel'):
                root_main.capture_inputs(kube,Path('/unused'),{},'r20261031',bootstrap_enrollment={'fixture':True},installation=installation)
        select.assert_called_once_with(kube,{'fixture':True},'r20261031',installation=installation)
        kube.get.assert_not_called()

    def test_historical_checkpoint_requires_parent_child_version_match(self):
        import realtime_bootstrap as realtime
        receipt={'schema':'receipt'};plan={'schema':'unparented'}
        state={'status':'PASS','target':{'realtime_bootstrap_sha256':claim.digest(receipt)}}
        binding={'context':{},'operation':'a'*12,'helper_sha256':'b'*64}
        with patch.object(claim,'historical_binding',return_value=binding),patch.object(realtime,'_records',return_value=(plan,receipt,None,None)):
            for parent,child,allowed in ((1,1,True),(2,2,True),(2,1,False),(1,2,False)):
                binding['schema']=f'voice-staging-resource-fence-v{parent}'
                checkpoint={'schema':f'voice-staging-realtime-bootstrap-checkpoint-v{child}'}
                with self.subTest(parent=parent,child=child):
                    if allowed:self.assertEqual(realtime.validate_historical_checkpoint(checkpoint,state),(plan,receipt))
                    else:
                        with self.assertRaises(Blocked):realtime.validate_historical_checkpoint(checkpoint,state)

    def test_realtime_receipt_dispatch_is_closed(self):
        import realtime_bootstrap as realtime, staging_cohort_preservation as cohort
        with patch.object(realtime,'load_completed',return_value='c1') as c1, patch.object(realtime,'load_existing',return_value='c2') as c2:
            for version in (1,2):
                self.assertEqual(cohort._load_realtime_receipt('session',{'schema':f'voice-staging-realtime-bootstrap-receipt-v{version}'},rollout_target='target'),'c1')
            for version in (1,2,3,4):
                self.assertEqual(cohort._load_realtime_receipt('session',{'schema':f'voice-staging-realtime-bootstrap-existing-v{version}'}),'c2')
            with self.assertRaises(Blocked):cohort._load_realtime_receipt('session',{'schema':'unknown'})
            self.assertEqual(c1.call_count,2);self.assertEqual(c2.call_count,4)

    def setUp(self):
        self.installation=claim.production_installation()
        self.row={'schema':'voice-staging-resource-fence-v2','context':context(),
                  'operation':'a'*12,'helper_sha256':'b'*64,'target_sha256':'c'*64,
                  'installation_sha256':self.installation.sha256,'status':'active',
                  'phase':'cohort-preservation','cohort_preservation_sha256':'c'*64}

    def test_bound_cohort_lineage_is_not_discarded_or_reinterpreted(self):
        binding=claim._journal_binding(self.row,self.installation)
        self.assertEqual(binding['installation_sha256'],self.installation.sha256)
        self.assertEqual(binding['target_sha256'],self.row['target_sha256'])
        self.assertNotIn('cohort_preservation_sha256',binding)
        for mutation in ('missing','invalid','unknown','foreign'):
            row=copy.deepcopy(self.row)
            if mutation=='missing':del row['cohort_preservation_sha256']
            elif mutation=='invalid':row['cohort_preservation_sha256']='invalid'
            elif mutation=='unknown':row['unexpected']=True
            else:row['installation_sha256']='f'*64
            with self.subTest(mutation=mutation),self.assertRaises(RuntimeError):
                claim._journal_binding(row,self.installation)

    def test_finished_cohort_parent_keeps_closed_lineage_and_actual_images(self):
        row=copy.deepcopy(self.row)
        row.update(status='complete',phase='finished',actual=copy.deepcopy(row['context']['images']))
        claim._journal_binding(row,self.installation)
        row['cohort_preservation_sha256']='invalid'
        with self.assertRaises(RuntimeError):claim._journal_binding(row,self.installation)

    def test_restart_rejects_removed_or_changed_durable_cohort_lineage(self):
        import guard,staging_cohort_preservation as cohort
        c=context();journal=MemoryJournal()
        first=claim.Session(c,'a'*12,'b'*64,lambda:card(c),journal);first.admit(start=True)
        plan={key:'d'*64 for key in cohort.PLAN_FIELDS}
        plan.update(schema=cohort.SCHEMA,operation=first.operation,context=c,helper_sha256=first.helper)
        lineage=claim.digest(plan)
        journal.row.update(phase='prerequisites-captured',target_sha256='c'*64,cohort_preservation_sha256=lineage)
        with tempfile.TemporaryDirectory(dir='/root') as folder:
            root=Path(folder);requests=root/'installed/cohort-requests';requests.mkdir(parents=True,mode=0o700)
            base=root/'installed/cohort-preservation'/first.operation;base.mkdir(parents=True,mode=0o700)
            request={'action':'prepare','nonce':'a'*64,'resource_context':c,'source_sha':c['commit']}
            (requests/(first.operation+'.json')).write_text(json.dumps(request))
            (base/'checkpoint.json').write_text(json.dumps({'plan':plan}))
            for path in (requests/(first.operation+'.json'),base/'checkpoint.json'):path.chmod(0o600)
            with patch.object(guard,'ROOT',root):
                resumed=claim.Session(c,first.operation,first.helper,lambda:card(c),journal)
                resumed.admit(start=False)
                self.assertEqual(resumed.check()['cohort_preservation_sha256'],lineage)
                for mutation in ('removed','changed'):
                    saved=copy.deepcopy(journal.row)
                    if mutation=='removed':del journal.row['cohort_preservation_sha256']
                    else:journal.row['cohort_preservation_sha256']='e'*64
                    fresh=claim.Session(c,first.operation,first.helper,lambda:card(c),journal)
                    with self.subTest(mutation=mutation),self.assertRaisesRegex(RuntimeError,'cohort_lineage'):
                        fresh.admit(start=False)
                    self.assertFalse(fresh.admitted)
                    journal.row=saved
                request_path=requests/(first.operation+'.json');checkpoint=base/'checkpoint.json'
                request_raw=request_path.read_bytes();checkpoint_raw=checkpoint.read_bytes()
                for mutation in ('request-removed','checkpoint-removed','request-context','plan-changed'):
                    if mutation=='request-removed':request_path.unlink()
                    elif mutation=='checkpoint-removed':checkpoint.unlink()
                    elif mutation=='request-context':request_path.write_text(json.dumps(dict(request,source_sha='f'*40)))
                    else:checkpoint.write_text(json.dumps({'plan':dict(plan,helper_sha256='f'*64)}))
                    fresh=claim.Session(c,first.operation,first.helper,lambda:card(c),journal)
                    with self.subTest(durable_mutation=mutation),self.assertRaisesRegex(RuntimeError,'cohort_lineage'):
                        fresh.admit(start=False)
                    self.assertFalse(fresh.admitted)
                    request_path.write_bytes(request_raw);request_path.chmod(0o600)
                    checkpoint.write_bytes(checkpoint_raw);checkpoint.chmod(0o600)
            # Legacy production v1 without a cohort marker remains admissible.
            legacy=MemoryJournal();legacy.row=copy.deepcopy(journal.row)
            legacy.row.update(schema='voice-staging-resource-fence-v1',phase='prepare')
            del legacy.row['installation_sha256'];del legacy.row['cohort_preservation_sha256']
            with tempfile.TemporaryDirectory(dir='/root') as empty,patch.object(guard,'ROOT',Path(empty)):
                normal=claim.Session(c,first.operation,first.helper,lambda:card(c),legacy)
                normal.admit(start=False);self.assertTrue(normal.admitted)

    def test_journal_actual_is_only_allowed_for_exact_terminal_images(self):
        for version in (1,2):
            base=copy.deepcopy(self.row)
            if version==1:
                base['schema']='voice-staging-resource-fence-v1';del base['installation_sha256']
            for status,phase,actual,valid in (
                    ('active','prepare',None,True),('active','prepare',{},False),
                    ('active','prerequisites-captured',{},False),('active','awaiting-ci',None,False),
                    ('active','unknown',None,False),('uncertain','unknown',None,False),
                    ('active','awaiting-ci',base['context']['images'],True),
                    ('active','awaiting-ci',{},False),('complete','finished',base['context']['images'],True),
                    ('complete','finished',None,False),('uncertain','awaiting-ci',base['context']['images'],True),
                    ('uncertain','awaiting-ci',{},False),('uncertain','prepare',{},False)):
                row=copy.deepcopy(base);row.update(status=status,phase=phase)
                if actual is not None:row['actual']=actual
                if status=='uncertain':row['reason']='interrupted'
                with self.subTest(version=version,status=status,phase=phase,valid=valid):
                    if valid:claim._journal_binding(row,self.installation)
                    else:
                        with self.assertRaises(RuntimeError):claim._journal_binding(row,self.installation)

    def test_unknown_journal_phase_cannot_reach_ready_after_restart(self):
        c=context();journal=MemoryJournal()
        first=claim.Session(c,'a'*12,'b'*64,lambda:card(c),journal);first.admit(start=True)
        journal.row.update(phase='unknown',target_sha256='c'*64)
        fresh=claim.Session(c,first.operation,first.helper,lambda:card(c),journal)
        with self.assertRaises(RuntimeError):fresh.admit(start=False)
        self.assertFalse(fresh.admitted)
        self.assertEqual(journal.row['phase'],'unknown')

    def test_legacy_fence_rejects_unknown_or_mixed_root_schema(self):
        c=context();helper={'nats-rollout-preservation/resource_claim.py':'b'*64}
        target={'tag':c['commit'],'images':c['images']}
        binding={'schema':'voice-staging-resource-fence-v1','context':c,'operation':'a'*12,
                 'helper_sha256':claim.digest(helper),'target_sha256':claim.digest(target)}
        state={'schema':'nats-rollout-root-v1','operation':'a'*12,'target':target,
               'code_capture':helper,'resource_fence':binding}
        self.assertEqual(claim.historical_binding(state),binding)
        for schema in ('unknown','nats-rollout-root-v2'):
            with self.subTest(schema=schema),self.assertRaises(RuntimeError):
                claim.historical_binding(dict(state,schema=schema))

    def test_scoped_p2_target_keeps_cohort_prerequisite_and_realtime_digests(self):
        authority=claim.InstallationAuthority('owned-isolated-v1','11111111-1111-4111-8111-111111111111',
                                              'r20260930a4','a'*64,'b'*64,'c'*64)
        target={'registry':'registry','tag':'c'*40,'mode':'full','changed_services':[],
                'source_hashes':{'source':'1'*64},'template_hashes':{'voice-user':'2'*64},
                'input_template_hashes':{'voice-user':'3'*64},'images':context()['images'],
                'manifest_sha256':'4'*64,'migration_sha256':'5'*64,'nonnats_sha256':'6'*64,
                'installation_sha256':authority.sha256,'cohort_preservation_sha256':'7'*64,
                'prerequisites_sha256':'8'*64,'realtime_bootstrap_sha256':'9'*64}
        self.assertEqual(claim._final_target_inventory(target,authority),target)
        for mutation in ('missing-parent','bad-digest','extra'):
            bad=copy.deepcopy(target)
            if mutation=='missing-parent':del bad['prerequisites_sha256']
            elif mutation=='bad-digest':bad['cohort_preservation_sha256']='invalid'
            else:bad['unexpected']=True
            with self.subTest(mutation=mutation),self.assertRaises(RuntimeError):claim._final_target_inventory(bad,authority)

    def test_compiler_capture_accepts_only_bound_p2_reference_set(self):
        import hashlib,source_authority
        authority=self.installation;c=context()
        template={'spec':{'containers':[{'name':'user','image':c['images']['voice-user/user']}]}}
        documents=[{'kind':'Deployment','metadata':{'name':'voice-user','namespace':'voice-staging'},'spec':{'template':template}}]
        with tempfile.TemporaryDirectory(dir='/root') as folder:
            base=Path(folder);source=base/'source';path=source/'scripts/fixture.py';path.parent.mkdir(parents=True,mode=0o700)
            path.write_bytes(b'fixture');path.chmod(0o400)
            sha=hashlib.sha256(b'fixture').hexdigest()
            target={'registry':source_authority.REGISTRY,'tag':c['commit'],'mode':'full','changed_services':[],
                    'source_hashes':{'scripts/fixture.py':sha},'template_hashes':{'voice-user':claim.digest(template)},
                    'images':c['images'],'manifest_sha256':claim.digest(documents),'migration_sha256':claim.digest([]),
                    'nonnats_sha256':claim.digest({}),'installation_sha256':authority.sha256,
                    'cohort_preservation_sha256':'7'*64,'prerequisites_sha256':'8'*64,'realtime_bootstrap_sha256':'9'*64}
            compiled={'target':target,'manifests':documents,'migrations':[],'nonnats':{},'contract':{}}
            file=base/'compiler-input.json';file.write_text(json.dumps(compiled));file.chmod(0o600)
            state={'input_target':target,'target':target,'contract':{},'installation_sha256':authority.sha256,
                   'resource_fence':{'context':c}}
            approved={'source_files':{'scripts/fixture.py':sha}}
            self.assertEqual(claim._capture_compiler(state,base,approved,source,authority),compiled)
            for mutation in ('missing-parent','bad-digest','extra'):
                bad=copy.deepcopy(compiled)
                if mutation=='missing-parent':del bad['target']['prerequisites_sha256']
                elif mutation=='bad-digest':bad['target']['cohort_preservation_sha256']='invalid'
                else:bad['target']['unexpected']=True
                file.write_text(json.dumps(bad));file.chmod(0o600)
                bad_state={**state,'input_target':bad['target'],'target':bad['target']}
                with self.subTest(mutation=mutation),self.assertRaises(RuntimeError):
                    claim._capture_compiler(bad_state,base,approved,source,authority)

    def test_prerequisite_p2_legacy_and_scoped_versions_are_distinct(self):
        authority=claim.InstallationAuthority('owned-isolated-v1','11111111-1111-4111-8111-111111111111',
                                              'r20260930a4','a'*64,'b'*64,'c'*64)
        fields={'schema','preservation_sha256'}
        legacy={'schema':'voice-staging-prerequisites-plan-v2','preservation_sha256':'d'*64}
        scoped={'schema':'voice-staging-prerequisites-plan-v4','preservation_sha256':'d'*64,
                'installation_sha256':authority.sha256}
        claim.record_fields(legacy,'prerequisites_plan',fields)
        with self.assertRaises(RuntimeError):claim.record_fields(legacy,'prerequisites_plan',fields,installation=authority)
        claim.record_fields(scoped,'prerequisites_plan',fields,installation=authority)
        with self.assertRaises(RuntimeError):claim.record_fields(scoped,'prerequisites_plan',fields)
        import staging_cohort_preservation as cohort
        self.assertTrue(cohort._p2_prerequisite_version(legacy,None))
        self.assertFalse(cohort._p2_prerequisite_version(legacy,authority))
        self.assertTrue(cohort._p2_prerequisite_version(scoped,authority))
        self.assertFalse(cohort._p2_prerequisite_version(scoped,None))

    def test_scoped_p2_plan_validates_preservation_and_domain(self):
        import staging_prerequisites as prerequisites
        authority=claim.InstallationAuthority('owned-isolated-v1','11111111-1111-4111-8111-111111111111',
                                              'r20260930a4','a'*64,'b'*64,'c'*64)
        c=context();session=claim.Session(c,'a'*12,'b'*64,lambda:card(c),MemoryJournal(),installation=authority)
        session.admit(start=True)
        source={'schema':'voice-staging-source-capture-v2','installation_sha256':authority.sha256,
                'operation':session.operation,'context':c,'helper_sha256':session.helper,
                'approved_sha256':'c'*64,'execution_sha256':'d'*64}
        plan={'schema':'voice-staging-prerequisites-plan-v4','installation_sha256':authority.sha256,
              'preservation_sha256':'a'*64,'operation':session.operation,'context':c,'helper_sha256':session.helper,
              'source_capture_sha256':claim.digest(source),'approved_sha256':'c'*64,'execution_sha256':'d'*64,
              'namespace_uid':'namespace','marker_sha256':'e'*64,'baseline_sha256':'f'*64,
              'actual_before':card(c)['observed']['images'],'policy_before_sha256':'1'*64,
              'children':{slot:{} for slot in prerequisites.SLOTS},'auxiliary_images':{},
              'renderer_authority_sha256':None,'previous_active_sha256':None}
        with patch.object(prerequisites,'_read_private',return_value=json.dumps(source).encode()):
            self.assertEqual(prerequisites.validate_plan(plan,session),plan)
            for mutation in ('domain','preservation','extra','legacy'):
                bad=copy.deepcopy(plan)
                if mutation=='domain':bad['installation_sha256']='f'*64
                elif mutation=='preservation':del bad['preservation_sha256']
                elif mutation=='extra':bad['extra']=True
                else:bad['schema']='voice-staging-prerequisites-plan-v2';del bad['installation_sha256']
                with self.subTest(mutation=mutation),self.assertRaises(Blocked):prerequisites.validate_plan(bad,session)

    def test_readonly_launch_cannot_be_consumed_as_cohort_auth(self):
        c=context();journal=MemoryJournal()
        session=claim.Session(c,'a'*12,'b'*64,lambda:card(c),journal,installation=self.installation)
        session.admit(start=True)
        journal.row.update(phase='prerequisites-captured',target_sha256='c'*64,
                           cohort_preservation_sha256='d'*64)
        session.expected_target='c'*64;session._cohort_lineage='d'*64
        argv=['docker','inspect','fixture']
        session._prerequisite_readonly_permit=(tuple(argv),'private-fd')
        session._cohort_auth_permit={'effect_id':'read','argv':argv,'body_sha256':'e'*64}
        with self.assertRaisesRegex(RuntimeError,'staging_prerequisite_readonly_launch_changed'):
            session.check(argv,launch=(tuple(argv),'private-fd'))
        self.assertIsNone(session._prerequisite_readonly_permit)

    def test_readonly_command_metadata_is_bound_before_launch(self):
        import commands
        argv=['/usr/local/bin/k3s','kubectl','--namespace','voice-staging','exec','voice-postgres-0']
        expected={'body':b'fixed','timeout':25,'limit':256,'env':{'PATH':'/bin'},'pass_fds':()}
        for field,value in [('baseline',None),('argv',argv+['extra']),('body',b'changed'),
                            ('timeout',26),('limit',257),('env',None),('pass_fds',(9,))]:
            with self.subTest(field=field):
                c=context();journal=MemoryJournal()
                session=claim.Session(c,'a'*12,'b'*64,lambda:card(c),journal);session.admit(start=True)
                captured={'protected':'fixture'}
                journal.row.update(phase='prerequisites-captured',target_sha256=claim.digest(captured))
                session.expected_target=claim.digest(captured);session._prerequisite_readonly_busy=True
                session._prerequisite_readonly_capture=captured
                session._prerequisite_readonly_permit=commands.launch_snapshot(argv,**expected)
                actual=dict(expected);actual_argv=argv
                if field=='argv':actual_argv=value
                elif field!='baseline':actual[field]=value
                with patch.dict(sys.modules,{'staging_prerequisites':SimpleNamespace(load_capture=lambda _:captured)}), \
                     patch.object(commands,'authority_guard',session.check), \
                     patch.object(commands.subprocess,'Popen',side_effect=AssertionError('launched')) as popen:
                    if field=='baseline':
                        with self.assertRaisesRegex(AssertionError,'launched'):commands.capture(actual_argv,**actual)
                        popen.assert_called_once()
                    else:
                        with self.assertRaisesRegex(RuntimeError,'readonly_launch_changed'):commands.capture(actual_argv,**actual)
                        popen.assert_not_called()
                self.assertIsNone(session._prerequisite_readonly_permit)

    def test_readonly_guard_loss_after_observer_blocks_process_launch(self):
        import commands
        c=context();current=card(c);journal=MemoryJournal()
        session=claim.Session(c,'a'*12,'b'*64,lambda:copy.deepcopy(current),journal);session.admit(start=True)
        captured={'protected':'fixture'}
        journal.row.update(phase='prerequisites-captured',target_sha256=claim.digest(captured))
        session.expected_target=claim.digest(captured);session._prerequisite_readonly_busy=True
        session._prerequisite_readonly_capture=captured
        argv=['/usr/local/bin/k3s','kubectl','--namespace','voice-staging','exec','voice-postgres-0']
        session._prerequisite_readonly_permit=commands.launch_snapshot(argv,b'',30,2<<20,None,())
        def observer(action,event):current['owner']='foreign'
        with patch.dict(sys.modules,{'staging_prerequisites':SimpleNamespace(load_capture=lambda _:captured)}), \
             patch.object(commands,'authority_guard',session.check),patch.object(commands,'diagnostic_observer',observer), \
             patch.object(commands.subprocess,'Popen',side_effect=AssertionError('launched')) as popen:
            with self.assertRaises(RuntimeError):commands.capture(argv)
            popen.assert_not_called()
        self.assertIsNone(session._prerequisite_readonly_permit)

    def test_realtime_readonly_producer_passes_exact_launch_to_session_guard(self):
        import commands,realtime_bootstrap
        c=context();journal=MemoryJournal()
        session=claim.Session(c,'a'*12,'b'*64,lambda:card(c),journal);session.admit(start=True)
        captured={'protected':'fixture'}
        journal.row.update(phase='prerequisites-captured',target_sha256=claim.digest(captured))
        session.expected_target=claim.digest(captured)
        with patch.dict(sys.modules,{'staging_prerequisites':SimpleNamespace(load_capture=lambda _:captured)}), \
             patch.object(commands,'authority_guard',session.check), \
             patch.object(commands.subprocess,'Popen',side_effect=AssertionError('launched')) as popen:
            with self.assertRaisesRegex(AssertionError,'launched'):
                realtime_bootstrap._capture_readonly('catalog')
            popen.assert_called_once()
        self.assertIsNone(session._prerequisite_readonly_permit)

    def test_realtime_readonly_cannot_reenter_after_permit_consumed(self):
        import commands,realtime_bootstrap
        c=context();journal=MemoryJournal()
        session=claim.Session(c,'a'*12,'b'*64,lambda:card(c),journal);session.admit(start=True)
        captured={'protected':'fixture'}
        journal.row.update(phase='prerequisites-captured',target_sha256=claim.digest(captured))
        session.expected_target=claim.digest(captured);observed=[]
        def observer(action,event):
            if event=='STARTED':
                with self.assertRaisesRegex(Blocked,'readonly_permit_busy'):
                    realtime_bootstrap._capture_readonly('catalog')
                observed.append(event)
        with patch.dict(sys.modules,{'staging_prerequisites':SimpleNamespace(load_capture=lambda _:captured)}), \
             patch.object(commands,'authority_guard',session.check),patch.object(commands,'diagnostic_observer',observer), \
             patch.object(commands.subprocess,'Popen',side_effect=AssertionError('launched')):
            with self.assertRaisesRegex(AssertionError,'launched'):realtime_bootstrap._capture_readonly('catalog')
        self.assertEqual(observed,['STARTED'])
        self.assertFalse(session._prerequisite_readonly_busy)

    def test_actions_accepts_only_explicit_immutable_authority(self):
        import bridge_root
        authority=claim.InstallationAuthority('owned-isolated-v1','11111111-1111-4111-8111-111111111111',
                                              'r20260930a4','a'*64,'b'*64,'c'*64)
        with patch.object(bridge_root.root_cli,'code_binding',return_value={}):
            actions=bridge_root.Actions('/fixture',installation=authority)
            self.assertIs(actions.installation,authority)
            self.assertEqual(bridge_root.Actions('/fixture').installation,self.installation)
            with self.assertRaises(ValueError):bridge_root.Actions('/fixture',installation=authority.descriptor())
            with self.assertRaises(AttributeError):actions.installation=self.installation

    def test_p2_prepare_rejects_session_in_another_domain_before_effects(self):
        import bridge_root, commands
        authority=claim.InstallationAuthority('owned-isolated-v1','11111111-1111-4111-8111-111111111111',
                                              'r20260930a4','a'*64,'b'*64,'c'*64)
        c=context();binding={};journal=MemoryJournal()
        session=claim.Session(c,'a'*12,claim.digest(binding),lambda:card(c),journal,installation=authority)
        session.admit(start=True)
        with patch.object(bridge_root.root_cli,'code_binding',return_value=binding):
            actions=bridge_root.Actions('/fixture');actions.tt_session=session
        request={'action':'prepare','mode':'full','nonce':'a'*64,'resource_context':c,'source_sha':c['commit']}
        with patch.object(commands,'authority_guard',session.check),patch.object(actions,'_idle') as idle, \
             patch('staging_cohort_preservation.record_prepare_request') as record:
            with self.assertRaisesRegex(RuntimeError,'staging_installation_binding_changed'):
                actions._prepare_cohort(request)
            idle.assert_not_called()
            record.assert_not_called()

    def test_prerequisite_capture_refuses_foreign_domain_before_protected_read(self):
        import bridge_root,commands,staging_prerequisites as prerequisites
        authority=claim.InstallationAuthority('owned-isolated-v1','11111111-1111-4111-8111-111111111111',
                                              'r20260930a4','a'*64,'b'*64,'c'*64)
        c=context();binding={};session=claim.Session(c,'a'*12,claim.digest(binding),lambda:card(c),MemoryJournal(),installation=authority)
        session.admit(start=True)
        with patch.object(bridge_root.root_cli,'code_binding',return_value=binding):
            actions=bridge_root.Actions('/fixture');actions.tt_session=session
        with patch.object(commands,'authority_guard',session.check),patch.object(prerequisites,'_read_private',return_value=b'{}') as read:
            with self.assertRaisesRegex(Blocked,'staging_prerequisites_session_invalid'):
                prerequisites._capture_authority(actions,session,{}, {})
            read.assert_not_called()

    def test_owned_session_binds_cohort_and_keeps_domain_through_journal(self):
        authority=claim.InstallationAuthority('owned-isolated-v1','11111111-1111-4111-8111-111111111111',
                                              'r20260930a4','a'*64,'b'*64,'c'*64)
        c=context();observed=card(c);journal=MemoryJournal()
        session=claim.Session(c,'a'*12,'b'*64,lambda:copy.deepcopy(observed),journal,installation=authority)
        session.admit(start=True)
        plan={'actual_before':copy.deepcopy(observed['observed']['images']),
              'candidate':{'commit':c['commit'],'images':copy.deepcopy(c['images'])}}
        with patch.dict(sys.modules,{'staging_cohort_preservation':Mock()}):
            binding=session.bind_cohort_preservation(plan,plan['actual_before'])
        self.assertEqual(binding['schema'],'voice-staging-resource-fence-v2')
        self.assertEqual(binding['installation_sha256'],authority.sha256)
        self.assertEqual(binding['target_sha256'],claim.digest(plan))
        self.assertEqual(session.check()['cohort_preservation_sha256'],claim.digest(plan))
        with self.assertRaises(RuntimeError):claim.validate_fence_binding(binding)

    def test_source_capture_is_bound_to_real_session_domain(self):
        import bridge_root,commands
        authority=claim.InstallationAuthority('owned-isolated-v1','11111111-1111-4111-8111-111111111111',
                                              'r20260930a4','a'*64,'b'*64,'c'*64)
        c=context();binding={'script':'d'*64};journal=MemoryJournal()
        session=claim.Session(c,'a'*12,claim.digest(binding),lambda:card(c),journal,installation=authority)
        session.admit(start=True)
        with tempfile.TemporaryDirectory(dir='/root') as folder:
            root=Path(folder);installed=root/'installed';installed.mkdir(mode=0o700)
            code=root/'code';code.mkdir(mode=0o700)
            approved={'source_files':{}};execution={'run_id':1}
            with patch.object(bridge_root,'INSTALLED',installed),patch.object(commands,'authority_guard',session.check), \
                 patch.object(bridge_root.root_cli,'code_binding',return_value=binding):
                row=bridge_root.record_source_authority(session,approved,execution,code=code,binding=binding)
            self.assertEqual(row['schema'],'voice-staging-source-capture-v2')
            self.assertEqual(row['installation_sha256'],authority.sha256)
            self.assertEqual(json.loads((installed/'source-authority'/(session.operation+'.json')).read_text()),row)
            self.assertEqual(claim.validate_source_capture(row,installation=authority),row)
            with self.assertRaises(RuntimeError):claim.validate_source_capture(row)
            import guard,realtime_bootstrap
            with patch.object(guard,'ROOT',root):
                realtime_bootstrap._verify_source_capture(session,approved,execution)
            import staging_cohort_preservation as cohort
            import staging_prerequisites as prerequisites
            cohort._validate_source_receipt(session,row,approved,execution)
            prerequisites._validate_source_identity(session,row,claim.digest(approved),claim.digest(execution))
            session.bind_target(c['commit'],c['images'],card(c)['observed']['images'],'d'*64)
            binding=session.binding()
            cohort._validate_historical_source_receipt(binding,row,approved,execution,installation=authority)
            with self.assertRaisesRegex(Blocked,'cohort_source_changed'):
                cohort._validate_historical_source_receipt(binding,row,approved,execution)
            legacy_binding=copy.deepcopy(binding);legacy_binding['schema']='voice-staging-resource-fence-v1'
            del legacy_binding['installation_sha256']
            legacy=copy.deepcopy(row);legacy['schema']='voice-staging-source-capture-v1';del legacy['installation_sha256']
            cohort._validate_historical_source_receipt(legacy_binding,legacy,approved,execution)
            with self.assertRaisesRegex(Blocked,'cohort_source_changed'):
                cohort._validate_historical_source_receipt(legacy_binding,legacy,approved,execution,installation=authority)
            for change in ('domain','schema','extra','digest'):
                changed=copy.deepcopy(row)
                if change=='domain':changed['installation_sha256']='f'*64
                elif change=='schema':changed['schema']='voice-staging-source-capture-v1';del changed['installation_sha256']
                elif change=='extra':changed['extra']=True
                else:changed['approved_sha256']='f'*64
                with self.subTest(change=change),self.assertRaisesRegex(Blocked,'cohort_source_changed'):
                    cohort._validate_source_receipt(session,changed,approved,execution)
                with self.subTest(prerequisite_change=change),self.assertRaises(ValueError):
                    prerequisites._validate_source_identity(session,changed,claim.digest(approved),claim.digest(execution))
                with patch.object(guard,'ROOT',root),patch.object(claim,'trusted_json',return_value=changed):
                    with self.subTest(realtime_change=change),self.assertRaisesRegex(Blocked,'realtime_bootstrap_source_authority_changed'):
                        realtime_bootstrap._verify_source_capture(session,approved,execution)

    def test_previous_pass_uses_expected_domain_before_historical_read(self):
        import staging_prerequisites as prerequisites
        authority=claim.InstallationAuthority('owned-isolated-v1','11111111-1111-4111-8111-111111111111',
                                              'r20260930a4','a'*64,'b'*64,'c'*64)
        active={'schema':'voice-active-release-v1','installation_sha256':authority.sha256,
                'operation':'a'*12,'namespace_uid':'namespace','pvc_uid':'pvc','target_sha256':'d'*64}
        state={'plan':{'namespace_uid':'namespace'}}
        rollout={'context':{'expected':{'namespace_uid':'namespace','source_claim_uid':'pvc'}},
                 'target':{'manifest_sha256':'d'*64}}
        with patch.object(prerequisites,'_read_private',return_value=json.dumps(active).encode()), \
             patch.object(prerequisites,'_historical',return_value=(state,{},rollout)) as historical:
            prerequisites._previous_pass('namespace',installation=authority)
            historical.assert_called_once_with(active['operation'],set(),installation=authority)
        active['installation_sha256']='f'*64
        with patch.object(prerequisites,'_read_private',return_value=json.dumps(active).encode()), \
             patch.object(prerequisites,'_historical') as historical:
            with self.assertRaises(RuntimeError):prerequisites._previous_pass('namespace',installation=authority)
            historical.assert_not_called()

    def test_realtime_successor_source_uses_bound_v2_receipt(self):
        import realtime_bootstrap_existing as existing,gis_existing_update
        authority=claim.InstallationAuthority('owned-isolated-v1','11111111-1111-4111-8111-111111111111',
                                              'r20260930a4','a'*64,'b'*64,'c'*64)
        c=context();session=claim.Session(c,'a'*12,'b'*64,lambda:card(c),MemoryJournal(),installation=authority)
        session.admit(start=True)
        approved={'schema':'voice-source-authority-v1','verified':True,'source_sha':c['commit']}
        execution={'head_sha':c['commit']}
        receipt={'schema':'voice-staging-source-capture-v2','installation_sha256':authority.sha256,
                 'operation':session.operation,'context':c,'helper_sha256':session.helper,
                 'approved_sha256':claim.digest(approved),'execution_sha256':claim.digest(execution)}
        with patch.object(existing,'_session'),patch.object(existing,'_private',return_value=(b'',receipt)), \
             patch.object(gis_existing_update,'validate_source_authority',side_effect=AssertionError('validated')):
            with self.assertRaisesRegex(AssertionError,'validated'):existing._source(session,approved,execution)
        receipt['installation_sha256']='f'*64
        with patch.object(existing,'_session'),patch.object(existing,'_private',return_value=(b'',receipt)), \
             patch.object(gis_existing_update,'validate_source_authority') as validate:
            with self.assertRaisesRegex(Blocked,'realtime_existing_source_invalid'):existing._source(session,approved,execution)
            validate.assert_not_called()

    def test_realtime_original_provenance_dispatches_by_expected_domain(self):
        import realtime_bootstrap_existing as existing
        authority=claim.InstallationAuthority('owned-isolated-v1','11111111-1111-4111-8111-111111111111',
                                              'r20260930a4','a'*64,'b'*64,'c'*64)
        plan={'schema':'voice-staging-realtime-bootstrap-v3','installation_sha256':authority.sha256,
              'operation':'a'*12,'context':context(),'helper_sha256':'b'*64,
              'source_authority_sha256':'c'*64,'execution_sha256':'d'*64}
        receipt={'schema':'voice-staging-source-capture-v2','installation_sha256':authority.sha256,
                 'operation':plan['operation'],'context':plan['context'],'helper_sha256':plan['helper_sha256'],
                 'approved_sha256':plan['source_authority_sha256'],'execution_sha256':plan['execution_sha256']}
        existing._validate_original_provenance(plan,receipt,installation=authority)
        with self.assertRaises(Blocked):existing._validate_original_provenance(plan,receipt)
        legacy=copy.deepcopy(plan);legacy['schema']='voice-staging-realtime-bootstrap-v1';del legacy['installation_sha256']
        old=copy.deepcopy(receipt);old['schema']='voice-staging-source-capture-v1';del old['installation_sha256']
        existing._validate_original_provenance(legacy,old)
        for p,r in ((legacy,receipt),(plan,old),(dict(plan,installation_sha256='f'*64),receipt)):
            with self.subTest(plan=p['schema'],receipt=r['schema']),self.assertRaises(Blocked):
                existing._validate_original_provenance(p,r,installation=authority)


if __name__=='__main__':unittest.main()
