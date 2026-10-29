import copy
from pathlib import Path
import unittest
from unittest import mock

import full_cohort_supervisor as supervisor
import media_backup_phase_test as fixtures
import media_operation_journal
import resource_fencing


class SupervisorTest(unittest.TestCase):
    def setUp(self):
        self.base=fixtures.PhaseTest(); self.base.setUp(); self.addCleanup(self.base.doCleanups)
        self.root=Path(self.base.temp.name)/'operations'; self.root.mkdir(mode=0o700)
        self.operation=self.root/self.base.context['operation_id']; self.operation.mkdir(mode=0o700)
        self.root_journal=media_operation_journal.Journal(self.operation/'journal',
            {'resource_context':self.base.context,'namespace_uid':'ns','cohort_route_mode':'full-cohort-v1'})
        self.addCleanup(self.root_journal.close)
        descriptor={'snapshot_sha256':'d'*64}
        self.root_journal.state['steps']['full-cohort-image-only-baseline']={'descriptor':descriptor}
        self.root_journal.state['steps']['image-only-admission']={'status':'done','descriptor':descriptor,
            'old_revision':['ledger','0'],'new_revision':['ledger','1']}
        self.base.ledger.row['image_only_baseline']=copy.deepcopy(descriptor)
        base=self.base
        class Client:
            def __init__(inner,journal): inner.journal=journal
            def call(inner,*args):
                if args[:3]==('get','namespace','voice-dev'):
                    return {'metadata':{'name':'voice-dev','uid':'ns','labels':supervisor.media_operation_guard.LABELS}}
                if args[:3]==('get','configmap',supervisor.media_operation_guard.LOCK):return copy.deepcopy(base.lock)
                return {'items':[]}
        class Ledger:
            def __init__(inner,client): inner.client=client
            def read(inner): return base.ledger.read()
            def write(inner,revision,row): return base.ledger.write(revision,row)
        class Check:
            endpoint='http://fixture'
            def __init__(inner,context): inner.context=context
            def __call__(inner): return copy.deepcopy(base.card)
            def mark_uncertain(inner,reason): pass
        self.client=Client(self.root_journal); check=Check(base.context)
        self.fence=resource_fencing.Fence(base.context,check,Ledger(self.client),check.mark_uncertain)
        self.guard_patch=mock.patch.object(supervisor.media_operation_guard,'verify')
        for patcher in (mock.patch.object(supervisor.frontend_runtime_executor,'KubectlTransport',Client),
                        mock.patch.object(resource_fencing,'KubeLedger',Ledger),
                        mock.patch.object(resource_fencing,'ResourceCheck',Check),
                        mock.patch.object(supervisor.media_backup_upload_runner,'ROOT',self.root),
                        mock.patch.object(supervisor.media_backup_upload_runner,'TT','http://fixture'),
                        self.guard_patch):
            patcher.start(); self.addCleanup(patcher.stop)

    def test_prepare_calls_owned_phases_and_never_replays(self):
        calls=[]
        nodes=[{'name':'node','uid':'uid','hostname':'node','architecture':'amd64'}]
        with mock.patch.object(supervisor.full_cohort_package_phase.PackagePhase,'run',side_effect=lambda *a:calls.append('package')), \
             mock.patch.object(supervisor.full_cohort_image_only_baseline,'ImageOnlyPhase') as baseline, \
             mock.patch.object(supervisor.full_cohort_inventory_phase.InventoryPhase,'run',side_effect=lambda *a:calls.append('inventory')), \
             mock.patch.object(supervisor.full_cohort_inventory_phase.InventoryPhase,'verify_receipt',return_value={
                 'database_observations':{'workloads':{'realtime':[{'status':'observed'}]}}}), \
             mock.patch.object(supervisor.full_cohort_namespace_census.NamespaceCensus,'run',side_effect=lambda *a:calls.append('census')), \
             mock.patch.object(supervisor.full_cohort_affected_plan.FullCatalogPhase,'run',side_effect=lambda *a:calls.append('catalog')), \
             mock.patch.object(supervisor.full_cohort_affected_plan,'build',return_value={'plan':'fixture'}):
            subject=supervisor.Supervisor(self.root_journal,self.fence,self.client,self.base.lock,'ns')
            self.assertEqual(subject.prepare('token','actor',1,1,nodes),{'plan':'fixture'})
            baseline.return_value.run.assert_called_once_with('token')
            self.assertEqual(calls,['package','inventory','census','catalog'])
            self.assertEqual(len(subject.journals),4)
            for journal in subject.journals: self.addCleanup(journal.close)
            self.assertIs(subject.package.journal,subject.journals[0])
            self.assertIs(subject.inventory.package,subject.package)
            self.assertIs(subject.census.inventory,subject.inventory)
            self.assertIs(subject.catalog.inventory,subject.inventory)
            with self.assertRaises(supervisor.SupervisorError): subject.prepare('token','actor',1,1,nodes)

    def test_replaced_ledger_client_refuses_before_phase(self):
        self.fence.ledger.client=object()
        with self.assertRaises(supervisor.SupervisorError):
            supervisor.Supervisor(self.root_journal,self.fence,self.client,self.base.lock,'ns')

    def _pre_drain_constructors(self,close_late=False,drift=False,creator=False):
        import writer_cut_test as cut_fixture
        self.guard_patch.stop()
        source=cut_fixture.workload('Deployment','voice-space','space-uid',1,'source-image')
        receipt={'engine':{'namespace_uid':'ns'},'server_start':'start','all_databases':{'postgres':1,'space_db':2}}
        plan={'writers':{'voice-space':{'uid':'other' if drift else 'space-uid','generation':1,
            'spec_sha256':supervisor.writer_cut.digest(source['spec'])}},'global_database_oids':copy.deepcopy(receipt['all_databases'])}
        calls=[]
        def current(*args):
            calls.append(None)
            if close_late and len(calls)==3:
                (subject.package.journal if close_late=='package' else subject.cut.journal).close()
            return copy.deepcopy(receipt)
        nodes=[{'name':'node','uid':'uid','hostname':'node','architecture':'amd64'}]
        with mock.patch.object(supervisor.full_cohort_package_phase.PackagePhase,'run'), \
             mock.patch.object(supervisor.full_cohort_image_only_baseline,'ImageOnlyPhase') as baseline, \
             mock.patch.object(supervisor.full_cohort_inventory_phase.InventoryPhase,'run'), \
             mock.patch.object(supervisor.full_cohort_inventory_phase.InventoryPhase,'verify_current'), \
             mock.patch.object(supervisor.full_cohort_inventory_phase.InventoryPhase,'verify_receipt',return_value={
                 'database_observations':{'workloads':{'realtime':[{'status':'observed'}]}}}), \
             mock.patch.object(supervisor.full_cohort_namespace_census.NamespaceCensus,'run'), \
             mock.patch.object(supervisor.full_cohort_namespace_census.NamespaceCensus,'verify_current'), \
             mock.patch.object(supervisor.full_cohort_affected_plan.FullCatalogPhase,'run'), \
             mock.patch.object(supervisor.full_cohort_affected_plan.FullCatalogPhase,'verify_current',side_effect=current), \
             mock.patch.object(supervisor.full_cohort_affected_plan,'build',return_value=plan), \
             mock.patch.object(supervisor.full_cohort_inventory_phase,'inventory',return_value=({}, {'voice-space':source}, {})), \
             mock.patch.object(supervisor.space_native_tool_identity.SpaceNativeToolPhase,'run') as tool_run, \
             mock.patch.object(supervisor.space_fresh_install_phase.SpaceFreshInstallPhase,'prepare') as space_prepare:
            subject=supervisor.Supervisor(self.root_journal,self.fence,self.client,self.base.lock,'ns')
            def close_phase_journals():
                for phase_journal in reversed(subject.journals):phase_journal.close()
            self.addCleanup(close_phase_journals)
            subject.prepare('token','actor',1,1,nodes)
            subject.baseline.journal=subject.journal;subject.baseline.fence=subject.fence
            subject.baseline.client=subject.client;subject.baseline.package_phase=subject.package
            if drift:
                with self.assertRaises(supervisor.SupervisorError):subject.prepare_execution('token','actor')
                self.assertIsNone(subject.cut);tool_run.assert_not_called();space_prepare.assert_not_called()
                return
            if close_late:
                with self.assertRaises(supervisor.SupervisorError):subject.prepare_execution('token','actor')
                self.assertEqual(len(calls),3);self.assertFalse(subject.cut.started)
                with self.assertRaises(supervisor.SupervisorError):subject.prepare_execution('token','actor')
                return
            self.assertEqual(subject.prepare_execution('token','actor')['scope'],'pre-drain-phases-only')
            self.assertEqual(len({id(x.journal) for x in (subject.cut,subject.physical,subject.tool,subject.space)}),4)
            self.assertIs(subject.space.cut,subject.cut);self.assertIs(subject.physical.cut,subject.cut)
            self.assertFalse(subject.cut.started);self.assertFalse(subject.cut.drained)
            tool_run.assert_called_once_with('token');space_prepare.assert_called_once_with()
            with self.assertRaises(supervisor.SupervisorError):subject.prepare_execution('token','actor')
            if creator in ('rootless','rootless-peer-close','rootless-config-drift','rootless-package',
                           'rootless-inventory','rootless-jdk-lock','rootless-peer-lock'):
                def peer_verify(*args):
                    if creator=='rootless-peer-close':subject.peer_target.journal.close()
                    if creator=='rootless-config-drift':subject.creator_config.journal.state['steps']['drift']={}
                    if creator=='rootless-package':subject.package.journal.state['steps'].pop('protected')
                    if creator=='rootless-inventory':subject.inventory.journal.state['steps'].pop('protected')
                    if creator=='rootless-jdk-lock':subject.target_jdk.lock['metadata']['uid']='foreign'
                    if creator=='rootless-peer-lock':subject.peer_target.lock['metadata']['uid']='foreign'
                    return {'scope':'fixture'}
                with mock.patch.object(supervisor.space_creator_runtime_profile_config.CreatorConfigPhase,'run') as config_run, \
                     mock.patch.object(supervisor.space_creator_runtime_profile_jdk.TargetJdkPhase,'run') as jdk_run, \
                     mock.patch.object(supervisor.space_creator_runtime_profile_config,'verify_completed'), \
                     mock.patch.object(supervisor.space_creator_runtime_profile_jdk,'verify_completed'), \
                     mock.patch.object(supervisor.space_creator_peer_phase.PeerTargetPhase,'run',return_value={'scope':'fixture'}) as peer_run, \
                     mock.patch.object(supervisor.space_creator_peer_phase,'verify_current',side_effect=peer_verify), \
                     mock.patch.object(supervisor.space_p3_issuer_preflight.IssuerPreflightPhase,'prepare') as issuer_prepare:
                    self.assertEqual(subject.prepare_rootless_creator_base('token')['scope'],'rootless-creator-base-only')
                    if creator=='rootless-package':subject.package.journal.state['steps']['protected']={'status':'done'}
                    if creator=='rootless-inventory':subject.inventory.journal.state['steps']['protected']={'status':'done'}
                    if creator!='rootless':
                        with self.assertRaises(supervisor.SupervisorError):subject.prepare_peer_targets('token')
                        with self.assertRaises(supervisor.SupervisorError):subject.prepare_peer_targets('token')
                        self.assertFalse(subject.cut.started);issuer_prepare.assert_not_called()
                        return
                    self.assertEqual(subject.prepare_peer_targets('token')['scope'],'static-peer-targets-only')
                    config_run.assert_called_once_with('token');jdk_run.assert_called_once_with('token','actor')
                    peer_run.assert_called_once_with('token');issuer_prepare.assert_not_called()
                    self.assertIs(subject.peer_target.config,subject.creator_config)
                    self.assertIsNone(subject.issuer_preflight);self.assertFalse(subject.cut.started)
                    with self.assertRaises(supervisor.SupervisorError):subject.prepare_rootless_creator_base('token')
                    with self.assertRaises(supervisor.SupervisorError):subject.prepare_peer_targets('token')
                return
            if creator:
                def late_jdk(*args):
                    if creator=='late':subject.target_jdk.journal.close()
                    if creator=='census':subject.census.journal.close()
                    if creator=='local':subject.creator_config.journal.state['steps']['drift']={}
                    if creator=='lock':subject.lock['metadata']['uid']='changed'
                    if creator=='authority':subject._execution_fixed=tuple(list(subject._execution_fixed))
                    if creator=='refs':subject._execution_refs=tuple(list(subject._execution_refs))
                with mock.patch.object(supervisor.space_creator_runtime_profile_config.CreatorConfigPhase,'run') as config_run, \
                     mock.patch.object(supervisor.space_creator_runtime_profile_jdk.TargetJdkPhase,'run') as jdk_run, \
                     mock.patch.object(supervisor.space_creator_runtime_profile_config,'verify_completed'), \
                     mock.patch.object(supervisor.space_creator_runtime_profile_jdk,'verify_completed',
                        side_effect=late_jdk), \
                     mock.patch.object(supervisor.space_p3_issuer_preflight.IssuerPreflightPhase,'prepare',return_value={'status':'ready'}) as issuer_prepare, \
                     mock.patch.object(supervisor.space_p3_issuer_preflight.IssuerPreflightPhase,'finish',return_value={'custody':'fixture'}), \
                     mock.patch.object(supervisor.space_p3_issuer_preflight,'verify_current',return_value={'custody':'fixture'}):
                    if creator in ('late','census','local','lock','authority','refs'):
                        with self.assertRaises(supervisor.SupervisorError):subject.prepare_creator_inputs('token')
                        with self.assertRaises(supervisor.SupervisorError):subject.prepare_creator_inputs('token')
                        self.assertFalse(subject.cut.started)
                        return
                    self.assertEqual(subject.prepare_creator_inputs('token'),{'status':'ready'})
                    self.assertEqual(len({id(p.journal) for p in (subject.creator_config,subject.target_jdk,subject.issuer_preflight)}),3)
                    config_run.assert_called_once_with('token');jdk_run.assert_called_once_with('token','actor')
                    issuer_prepare.assert_called_once_with()
                    self.assertEqual(subject.finish_creator_inputs('token')['scope'],'creator-inputs-only')
                    self.assertFalse(subject.cut.started);self.assertFalse(subject.cut.drained)
                    with self.assertRaises(supervisor.SupervisorError):subject.prepare_creator_inputs('token')

    def test_pre_drain_constructors_use_separate_journals_and_never_replay(self):
        self._pre_drain_constructors()

    def test_creator_inputs_are_typed_and_pre_drain(self):
        self._pre_drain_constructors(creator=True)

    def test_rootless_creator_base_and_peer_plan_do_not_start_issuer_or_drain(self):
        self._pre_drain_constructors(creator='rootless')

    def test_rootless_peer_closed_during_final_callback_is_refused(self):
        self._pre_drain_constructors(creator='rootless-peer-close')

    def test_rootless_config_drift_during_peer_callback_is_refused(self):
        self._pre_drain_constructors(creator='rootless-config-drift')

    def test_rootless_package_evidence_removal_is_refused(self):
        self._pre_drain_constructors(creator='rootless-package')

    def test_rootless_inventory_evidence_removal_is_refused(self):
        self._pre_drain_constructors(creator='rootless-inventory')

    def test_rootless_jdk_lock_drift_is_refused(self):
        self._pre_drain_constructors(creator='rootless-jdk-lock')

    def test_rootless_peer_lock_drift_is_refused(self):
        self._pre_drain_constructors(creator='rootless-peer-lock')

    def test_creator_input_late_closed_jdk_fails_and_cannot_replay(self):
        self._pre_drain_constructors(creator='late')

    def test_creator_input_late_closed_census_fails(self):
        self._pre_drain_constructors(creator='census')

    def test_creator_input_late_local_record_drift_fails(self):
        self._pre_drain_constructors(creator='local')

    def test_creator_input_late_lock_drift_fails(self):
        self._pre_drain_constructors(creator='lock')

    def test_creator_input_late_authority_replacement_fails(self):
        self._pre_drain_constructors(creator='authority')

    def test_creator_input_late_execution_refs_replacement_fails(self):
        self._pre_drain_constructors(creator='refs')

    def test_pre_drain_late_closed_cut_journal_refuses_success(self):
        self._pre_drain_constructors(close_late='cut')

    def test_pre_drain_late_closed_package_journal_refuses_success(self):
        self._pre_drain_constructors(close_late='package')

    def test_pre_drain_writer_identity_drift_refuses_before_phase_creation(self):
        self._pre_drain_constructors(drift=True)

    def test_go_journal_is_fixed_to_source_database_set(self):
        subject=supervisor.Supervisor(self.root_journal,self.fence,self.client,self.base.lock,'ns')
        journal,_,_=subject._open_go('chat_db');self.addCleanup(journal.close)
        self.assertEqual(journal.path,self.operation/'go-migration-chat_db-journal')
        with self.assertRaises(supervisor.SupervisorError):subject._open_go('../foreign')

    def test_bootstrap_creates_fixed_immutable_intent_and_never_replays(self):
        other=copy.deepcopy(self.base.context);other['operation_id']='media-c1-'+'f'*32
        check=type(self.fence.check)(other)
        bootstrap=supervisor.OperationBootstrap(check,self.base.lock,'ns')
        self.addCleanup(bootstrap.journal.close)
        self.assertEqual(bootstrap.journal.path,self.root/other['operation_id']/'journal')
        self.assertEqual(bootstrap.journal.intent['cohort_route_mode'],'full-cohort-v1')
        self.assertEqual(bootstrap.journal.intent['kube_lock'],self.base.lock)
        with mock.patch.object(supervisor,'begin_operation',side_effect=RuntimeError('unknown')) as begin:
            with self.assertRaises(RuntimeError):bootstrap.begin()
            with self.assertRaises(supervisor.SupervisorError):bootstrap.begin()
            begin.assert_called_once()

    def test_bootstrap_refuses_changed_lock_before_begin(self):
        other=copy.deepcopy(self.base.context);other['operation_id']='media-c1-'+'e'*32
        bootstrap=supervisor.OperationBootstrap(type(self.fence.check)(other),self.base.lock,'ns')
        self.addCleanup(bootstrap.journal.close)
        bootstrap.lock['metadata']['resourceVersion']='changed'
        with mock.patch.object(supervisor,'begin_operation') as begin:
            with self.assertRaises(supervisor.SupervisorError):bootstrap.begin()
            begin.assert_not_called()

    def test_begin_routes_only_fixed_operation_and_requires_immutable_mode(self):
        with mock.patch.object(self.fence,'begin_image_only') as c1, \
             mock.patch.object(self.fence,'begin_with_predecessor') as c2:
            supervisor.begin_operation(self.root_journal,self.fence,self.client,self.base.lock,'ns')
            c1.assert_called_once();c2.assert_not_called()
            self.root_journal.intent['cohort_route_mode']='legacy'
            with self.assertRaises(supervisor.SupervisorError):
                supervisor.begin_operation(self.root_journal,self.fence,self.client,self.base.lock,'ns')
            self.assertEqual(c1.call_count,1)

    def test_pending_or_missing_admission_cannot_reconstruct_supervisor(self):
        step=self.root_journal.state['steps']['image-only-admission']
        step['status']='pending'
        with self.assertRaises(supervisor.SupervisorError):
            supervisor.Supervisor(self.root_journal,self.fence,self.client,self.base.lock,'ns')
        step['status']='done';self.base.ledger.row.pop('image_only_baseline')
        with self.assertRaises(supervisor.SupervisorError):
            supervisor.Supervisor(self.root_journal,self.fence,self.client,self.base.lock,'ns')

    def test_c2_pending_admission_cannot_reconstruct_supervisor(self):
        import full_cohort_predecessor_phase as predecessor
        context=copy.deepcopy(self.base.context);context['operation_id']='media-c2-'+'e'*32
        operation=self.root/context['operation_id'];operation.mkdir(mode=0o700)
        root_journal=media_operation_journal.Journal(operation/'journal',{
            'resource_context':context,'namespace_uid':'ns','cohort_route_mode':'full-cohort-v1'})
        self.addCleanup(root_journal.close)
        client=type(self.client)(root_journal);check=type(self.fence.check)(context)
        ledger=type(self.fence.ledger)(client);fence=resource_fencing.Fence(context,check,ledger,check.mark_uncertain)
        snapshot={'fixture':'predecessor'};descriptor={'snapshot_sha256':predecessor.digest(snapshot)}
        self.base.ledger.row=copy.deepcopy(context)|{'status':'active','predecessor':descriptor}
        root_journal.state['steps']['full-cohort-predecessor']={'status':'captured','descriptor':descriptor,'snapshot':snapshot}
        root_journal.state['steps']['predecessor-admission']={'status':'pending','descriptor':descriptor,'old_revision':['ledger','0']}
        with self.assertRaises(supervisor.SupervisorError):
            supervisor.Supervisor(root_journal,fence,client,self.base.lock,'ns')
        root_journal.state['steps']['predecessor-admission']={'status':'done','descriptor':descriptor,
            'old_revision':['ledger','0'],'new_revision':['ledger','1']}
        self.assertIsInstance(supervisor.Supervisor(root_journal,fence,client,self.base.lock,'ns'),supervisor.Supervisor)
        root_journal.state['steps']['full-cohort-predecessor']['snapshot']['fixture']='changed'
        with self.assertRaises(supervisor.SupervisorError):
            supervisor.Supervisor(root_journal,fence,client,self.base.lock,'ns')

    def test_wrong_journal_path_and_replaced_check_are_refused(self):
        other=media_operation_journal.Journal(self.operation/'other',self.root_journal.intent)
        self.addCleanup(other.close)
        self.client.journal=other
        with self.assertRaises(supervisor.SupervisorError):
            supervisor.Supervisor(other,self.fence,self.client,self.base.lock,'ns')
        self.client.journal=self.root_journal
        subject=supervisor.Supervisor(self.root_journal,self.fence,self.client,self.base.lock,'ns')
        self.fence.check=type(self.fence.check)(self.base.context)
        with self.assertRaises(supervisor.SupervisorError): subject._guard()

    def test_mutated_root_journal_state_is_refused(self):
        subject=supervisor.Supervisor(self.root_journal,self.fence,self.client,self.base.lock,'ns')
        self.root_journal.state['intent']['namespace_uid']='foreign'
        with self.assertRaises(supervisor.SupervisorError): subject._guard()


if __name__=='__main__': unittest.main()
