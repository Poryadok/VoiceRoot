"""Actual Realtime factory/phases; Kubernetes, PostgreSQL and source inputs synthetic."""
import copy
from pathlib import Path
import unittest
from unittest import mock

import canonical_application_package as canonical
import frontend_runtime_executor
import full_cohort_affected_plan as affected
import full_cohort_c1_activation as activation
import full_cohort_c1_driver as driver_module
import media_backup_phase_test as base_fixture
import media_operation_journal as journals
import realtime_db_provision as provision
import realtime_migration_job as migration
import realtime_migration_job_test as migration_fixture
import resource_fencing
import writer_cut
import writer_cut_migration
import writer_cut_test


class RealtimeFactoryJointTest(unittest.TestCase):
    def exercise(self,fault):
        base=base_fixture.PhaseTest(); self.addCleanup(lambda:self._cleanup(base)); base.setUp()
        renderer=migration_fixture.RendererTest(); self.addCleanup(lambda:self._cleanup(renderer)); renderer.setUp()
        objects={}; events=[]
        class Client(writer_cut_test.Client):
            def call(inner,*args,body=None):
                if args[:2]==('get','nodes'):return {'items':[migration_fixture.PhaseTest.node_object(renderer.node)]}
                if args[0]=='create':
                    obj=copy.deepcopy(body); kind=obj['kind']; meta=obj['metadata']; dry='--dry-run=server' in args
                    if kind!='Pod':meta.update(uid='uid-'+kind,resourceVersion='5')
                    if kind=='Job':
                        uid=meta['uid']; name=meta['name']; generated={'batch.kubernetes.io/controller-uid':uid,'controller-uid':uid,
                            'batch.kubernetes.io/job-name':name,'job-name':name}
                        meta['labels']={'app':name,**generated}; obj['spec']['selector']={'matchLabels':{'batch.kubernetes.io/controller-uid':uid}}
                        obj['spec']['template']['metadata']['labels']['batch.kubernetes.io/controller-uid']=uid
                        obj['status']={'conditions':[{'type':'Complete','status':'True'}]}
                    if not dry:
                        objects[kind]=copy.deepcopy(obj)
                        if kind=='Secret':events.append('secret-post')
                        if kind=='Job':
                            events.append('job-post')
                            if fault=='lost':raise TimeoutError('synthetic lost response')
                    return obj
                if args[0]=='get' and args[1] in ('secret','configmap','job') and len(args)>2:
                    kind={'secret':'Secret','configmap':'ConfigMap','job':'Job'}[args[1]]; value=objects.get(kind)
                    if value and value['metadata']['name']==args[2]:return copy.deepcopy(value)
                    if kind!='ConfigMap' or args[2]!='voice-dev-image-update-lock':return {}
                if args[:2]==('get','jobs'):return {'items':[copy.deepcopy(objects['Job'])] if 'Job' in objects else []}
                if args[:2]==('get','pods') and 'Job' in objects:
                    job=objects['Job']; template=job['spec']['template']; spec=copy.deepcopy(template['spec']); spec['nodeName']=renderer.node['name']
                    return {'items':[{'apiVersion':'v1','kind':'Pod','metadata':{'name':'job-pod','namespace':'voice-dev','uid':'pod-uid',
                        'resourceVersion':'2','labels':copy.deepcopy(template['metadata']['labels']),
                        'ownerReferences':[{'kind':'Job','uid':job['metadata']['uid'],'controller':True}]},'spec':spec,
                        'status':{'phase':'Succeeded','podIP':'10.1.2.3','containerStatuses':[{'name':'migrate',
                            'containerID':'containerd://fixture','restartCount':0,'imageID':'containerd://'+migration.PLATFORMS['amd64'],
                            'state':{'terminated':{'exitCode':0}}}]}}]}
                return super().call(*args)
        backing=base.ledger; tail_drift=[False]; cut_ref=[None]
        class LedgerView:
            def __init__(inner,client):inner.client=client
            def read(inner):
                result=backing.read()
                if tail_drift[0]:
                    tail_drift[0]=False; cut_ref[0].namespace_uid='drifted'
                row=result[0]
                if (cut_ref[0] is not None and cut_ref[0].journal.dirfd is not None and
                    ((fault=='secret-completion-close' and row.get('realtime_secret',{}).get('status')=='done') or
                     (fault=='database-completion-close' and row.get('realtime_db_provision',{}).get('steps',{}).get('database',{}).get('status')=='done'))):
                    cut_ref[0].journal.close()
                return result
            def write(inner,revision,row):return backing.write(revision,row)
        check=lambda:copy.deepcopy(base.card)
        def facade(journal):
            client=Client(base.lock,journal); return client,resource_fencing.Fence(base.context,check,LedgerView(client))
        def make_journal(name,extra):
            value=journals.Journal(Path(base.temp.name)/name,{'resource_context':base.context,'namespace_uid':'ns',**extra})
            self.addCleanup(value.close); client,fence=facade(value); return value,fence,client
        self.enterContext(mock.patch.object(frontend_runtime_executor,'KubectlTransport',Client))
        self.enterContext(mock.patch.object(resource_fencing,'KubeLedger',LedgerView))
        engine={'namespace_uid':'ns','pod_uid':'p','pvc_uid':'pvc','pv_uid':'pv','container_id':'containerd://'+'c'*64,'restart_count':1}
        scope={'engine':engine,'server_start':'start','databases':['auth_db'],'database_oids':{'auth_db':10}}
        baseline={'engine':engine,'server_start':'start','all_databases':{'auth_db':10},'databases':{'auth_db':10},'excluded':{},
                  'properties':{'auth_db':{'owner':1,'allowconn':True}},'operation_id':base.context['operation_id'],
                  'source_sha':base.context['commit'],'namespace_uid':'ns','scope':'all-databases-connectable-backup'}
        class Catalog:
            def verify_receipt(inner,token):return copy.deepcopy(baseline)
            def verify_current(inner,token):
                if pg.state['database'] is not None:raise ValueError('original catalog changed')
                return copy.deepcopy(baseline)
            def _guard(inner):pass
        self.enterContext(mock.patch.object(affected,'FullCatalogPhase',Catalog)); catalog=Catalog()
        catalog.journal,catalog.fence,catalog.client=make_journal('catalog',{}); catalog.lock,catalog.namespace_uid=base.lock,'ns'
        probe=affected.CatalogScopedProbe(catalog,'token'); source=writer_cut_test.workload('Deployment','voice-a','uid-a',1,'old')
        admission={'controllers':[source],'guards':{'jobs':[],'cronjobs':[],'hpas':[]},'linked':{'jobs':[],'cronjobs':[]},'session_scope':scope}
        cut_j,cut_f,cut_c=make_journal('cut',{'writer_cut':admission}); cut_c.workloads={('Deployment','voice-a'):copy.deepcopy(source)}
        cut=writer_cut.WriterCut(cut_j,cut_f,cut_c,base.lock,'ns',admission,probe)
        cut_ref[0]=cut
        self.enterContext(mock.patch.object(cut,'_patch',side_effect=cut_c.patch))
        real_psql=provision.KubePostgres._psql
        class PG:
            instance=None
            def __new__(cls,*_args):
                if cls.instance is None:cls.instance=super().__new__(cls); cls.instance.state={'role':None,'database':None}
                return cls.instance
            def __init__(inner,*_args):
                if _args:inner.operation_guard=_args[0]
            def snapshot(inner):return copy.deepcopy(engine)
            def catalog(inner):return copy.deepcopy(inner.state)
            def execute(inner,sql,worker=None):
                if fault=='transport-close':
                    original=inner.operation_guard
                    def close_after_guard():original(); cut_j.close()
                    inner.operation_guard=close_after_guard
                    with mock.patch.object(provision.subprocess,'run') as submit:
                        try:real_psql(inner,sql,worker)
                        finally:self.assertFalse(submit.called)
                    return
                events.append('database-sql' if sql==provision.database_sql() else 'role-sql')
                if sql==provision.database_sql():inner.state['database']={'oid':13,'owner_oid':12}
                else:inner.state['role']={'oid':12,'login':True,'superuser':False,'createdb':False,'createrole':False,'inherit':False,
                    'replication':False,'bypassrls':False,'memberships':0,'connection_limit':-1,'valid_until':None,'config':None}
        pg=PG(); self.enterContext(mock.patch.object(provision,'KubePostgres',PG))
        def full_catalog(*_):
            db={'auth_db':10}
            if pg.state['database']:db['realtime_db']=13
            return {'all_databases':db,'databases':copy.deepcopy(db),'excluded':{},'properties':{'auth_db':{'owner':1,'allowconn':True}}}
        self.enterContext(mock.patch.object(affected,'_full_catalog',side_effect=full_catalog))
        self.enterContext(mock.patch.object(affected.pg,'inspect',side_effect=lambda scope,guard:{'scope':copy.deepcopy(scope),'active_sessions':0,'active_transactions':0}))
        cut.drain()
        original_verify=cut.verify_drained
        def checked_cut():
            row,_=backing.read()
            secret=row.get('realtime_secret',{})
            steps=row.get('realtime_db_provision',{}).get('steps',{})
            if ((fault=='secret-guard' and secret.get('status')=='pending') or
                    (fault=='role-guard' and steps.get('role',{}).get('status')=='pending') or
                    (fault=='database-guard' and steps.get('database',{}).get('status')=='pending')):
                raise ValueError('synthetic lost writer cut')
            result=original_verify()
            if fault=='secret-check-drift' and secret.get('status')=='pending':
                cut.fence.check=lambda:copy.deepcopy(base.card)
            if ((fault=='secret-tail-drift' and secret.get('status')=='pending') or
                    (fault=='role-tail-drift' and steps.get('role',{}).get('status')=='pending')):
                tail_drift[0]=True
            return result
        self.enterContext(mock.patch.object(cut,'verify_drained',side_effect=checked_cut))
        supervisor=activation.supervisor_module.Supervisor.__new__(activation.supervisor_module.Supervisor)
        supervisor.context=base.context; supervisor.lock=base.lock; supervisor.namespace_uid='ns'; supervisor.cut=cut; supervisor.physical=object()
        supervisor.fence=cut_f; supervisor.client=cut_c; supervisor._guard=mock.Mock(); supervisor._open=make_journal
        driver=driver_module.C1Driver.__new__(driver_module.C1Driver); driver.supervisor=supervisor
        prepared=activation.RealtimePreparation.__new__(activation.RealtimePreparation); prepared.supervisor=supervisor
        plan={'node':renderer.node,'image':migration.MIGRATE_IMAGE,'platform_digest':migration.PLATFORMS['amd64']}
        files=renderer.files | {'src/backend/realtime/internal/lifecycle/store.go':'1e34ba05a4831031409bb90e37b541b8adfbe233cb307618cbb6ba471fb5fdc9'}
        self.enterContext(mock.patch.object(activation,'verify_realtime_receipt',return_value=plan))
        self.enterContext(mock.patch.object(activation.physical,'verify_receipt',return_value={'physical':{'engine':engine,'node':{k:renderer.node[k] for k in ('name','uid')}}}))
        self.enterContext(mock.patch.object(activation.canonical,'ROOT',renderer.root))
        self.enterContext(mock.patch.object(migration.publish_candidate,'source_snapshot',return_value={'source_sha':base.context['commit'],'files':files}))
        self.enterContext(mock.patch.object(migration.realtime_migration_verify,'check',return_value={'database_oid':13,'version':1,'dirty':False,'scope':'realtime-login-schema-readiness'}))
        self.enterContext(mock.patch.object(writer_cut_migration.writer_cut_pg_probe,'inspect_migration',side_effect=lambda scope,identity,guard:{'scope':copy.deepcopy(scope),'active_sessions':0,'active_transactions':0,'attributed_sessions':[]}))
        if fault:
            with self.assertRaises(activation.ActivationError):activation._realtime_database(driver,prepared,'token')
            expected={'lost':['secret-post','role-sql','database-sql','job-post'],
                      'secret-guard':[],'secret-check-drift':[],'secret-tail-drift':[],
                      'role-guard':['secret-post'],'role-tail-drift':['secret-post'],'transport-close':['secret-post'],
                      'database-guard':['secret-post','role-sql'],'secret-completion-close':['secret-post'],
                      'database-completion-close':['secret-post','role-sql','database-sql']}[fault]
            self.assertEqual(events,expected); self.assertEqual(backing.row['status'],'uncertain')
            with self.assertRaises(activation.ActivationError):activation._realtime_database(driver,prepared,'token')
            self.assertEqual(events,expected); return
        credential,secret,worker,phase=activation._realtime_database(driver,prepared,'token'); self.addCleanup(credential.close)
        self.assertEqual(events,['secret-post','role-sql','database-sql','job-post']); self.assertEqual(pg.state['database']['oid'],13)
        self.assertEqual(phase.journal.state['steps']['job']['status'],'done'); self.assertTrue(cut.migration['finished'])
        self.assertEqual(cut.current_effective_session_scope()['database_oids']['realtime_db'],13)

    def test_actual_factory_with_guarded_secret_provision_and_migration(self):self.exercise(None)
    def test_lost_migration_reply_is_uncertain_and_never_replayed(self):self.exercise('lost')
    def test_cut_loss_after_secret_pending_never_posts(self):self.exercise('secret-guard')
    def test_cut_check_identity_drift_after_pending_never_posts(self):self.exercise('secret-check-drift')
    def test_cut_namespace_drift_in_secret_tail_never_posts(self):self.exercise('secret-tail-drift')
    def test_cut_namespace_drift_in_role_tail_never_executes(self):self.exercise('role-tail-drift')
    def test_cut_close_after_last_transport_guard_never_submits_sql(self):self.exercise('transport-close')
    def test_cut_close_during_secret_completion_holds(self):self.exercise('secret-completion-close')
    def test_cut_close_during_database_completion_holds(self):self.exercise('database-completion-close')
    def test_cut_loss_after_role_pending_never_executes_role(self):self.exercise('role-guard')
    def test_cut_loss_after_database_pending_never_executes_database(self):self.exercise('database-guard')

    @staticmethod
    def _cleanup(case):
        if not case.doCleanups():raise RuntimeError('nested fixture cleanup failed')


if __name__=='__main__':unittest.main()
