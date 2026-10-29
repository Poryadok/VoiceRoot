"""Actual C1 driver authority checks with synthetic external TT/Kubernetes."""
import copy
import json
import unittest
from unittest.mock import patch

import full_cohort_backup_phase_joint_test as fixture
import full_cohort_c1_driver as driver
import full_cohort_supervisor as supervisor
import full_cohort_inventory_phase as inventory
import full_cohort_inventory_phase_test as inventory_fixture
import full_cohort_affected_plan as affected
import full_cohort_schema_metadata as schema
import full_cohort_schema_metadata_test as schema_fixture


def checked_nested(case,created):
    ok=case.doCleanups()
    for base in created:
        try:base.doCleanups()
        except BaseException:ok=False
    if not ok:raise RuntimeError('nested fixture cleanup failed')


def checked_base(base):
    cleanup=base.doCleanups
    def checked():
        if not cleanup():raise RuntimeError('base fixture cleanup failed')
    base.doCleanups=checked
    return base


class ExternalKube:
    """Raw synthetic Kubernetes results shared by inventory and engine probes."""
    def __init__(self):
        self.apps=inventory_fixture.Client()
        self.runtime={}
        for deployment,rs,pod in zip(self.apps.deployments,self.apps.replicasets,self.apps.pods):
            workload=deployment['metadata']['name'].removeprefix('voice-')
            rs_name='voice-'+workload+'-rs'; pod_name='voice-'+workload+'-pod'
            rs['metadata']['name']=rs_name; pod['metadata']['name']=pod_name; pod['metadata']['resourceVersion']='2'
            pod['metadata']['ownerReferences'][0]['name']=rs_name
            env={}
            if workload in schema.OWNERS and workload!='realtime':
                key='SPRING_DATASOURCE_URL' if workload=='auth' else 'VOICE_DATABASE_URL' if workload=='voice' else 'DATABASE_URL'
                uri='postgresql://voice:synthetic@voice-postgres:5432/'+schema.OWNERS[workload]
                env[key]='jdbc:'+uri if workload=='auth' else uri
            if workload=='auth':env.update(SPRING_DATASOURCE_USERNAME='voice',AUTH_FLYWAY_ENABLED='false')
            if workload=='gateway':env['GATEWAY_DATABASE_URL']='postgresql://voice:synthetic@voice-postgres:5432/gateway_db'
            for key,database in schema.SECONDARY.get(workload,{}).items():
                env[key]='postgresql://voice:synthetic@voice-postgres:5432/'+database
            entries=[{'name':name,'value':value} for name,value in env.items()]
            pod['spec']['containers'][0]['env']=copy.deepcopy(entries)
            deployment['spec']['template']['spec']['containers'][0]['env']=copy.deepcopy(entries)
            self.runtime[pod_name]=env
        self.objects={
            ('namespace','voice-dev'):{'metadata':{'name':'voice-dev','uid':'namespace'}},
            ('statefulset','voice-postgres'):{'metadata':{'name':'voice-postgres','namespace':'voice-dev','uid':'pg-sts'}},
            ('pod','voice-postgres-0'):{'metadata':{'name':'voice-postgres-0','namespace':'voice-dev','uid':'pg-pod','resourceVersion':'2',
                'labels':{'app':'voice-postgres'},'ownerReferences':[{'controller':True,'kind':'StatefulSet','uid':'pg-sts'}]},
                'spec':{'containers':[{'name':'postgres','volumeMounts':[{'name':'pgdata','mountPath':'/var/lib/postgresql/data'}]}],
                        'volumes':[{'name':'pgdata','persistentVolumeClaim':{'claimName':'pgdata-voice-postgres-0'}}]},
                'status':{'podIP':'10.0.0.1','containerStatuses':[{'name':'postgres','containerID':'pg-container','restartCount':0,'state':{'running':{}}}]}},
            ('persistentvolumeclaim','pgdata-voice-postgres-0'):{'metadata':{'name':'pgdata-voice-postgres-0','namespace':'voice-dev','uid':'pvc','resourceVersion':'3'},
                'spec':{'volumeName':'pv-one'},'status':{'phase':'Bound'}},
            ('persistentvolume','pv-one'):{'metadata':{'name':'pv-one','uid':'pv','resourceVersion':'3'},
                'spec':{'claimRef':{'name':'pgdata-voice-postgres-0','namespace':'voice-dev','uid':'pvc'}},'status':{'phase':'Bound'}},
            ('service','voice-postgres'):{'metadata':{'uid':'pg-service','resourceVersion':'4'},'spec':{'type':'ClusterIP',
                'clusterIP':'10.1.0.1','selector':{'app':'voice-postgres'},'ports':[{'port':5432,'targetPort':5432}]}},
            ('endpoints','voice-postgres'):{'metadata':{'uid':'pg-endpoints','resourceVersion':'5'},'subsets':[{'ports':[{'port':5432}],
                'addresses':[{'ip':'10.0.0.1','targetRef':{'kind':'Pod','uid':'pg-pod'}}]}]},
            ('endpointslices','-l'):{'items':[{'metadata':{'uid':'pg-slice','resourceVersion':'6'},'ports':[{'port':5432}],
                'endpoints':[{'addresses':['10.0.0.1'],'conditions':{'ready':True},'targetRef':{'kind':'Pod','uid':'pg-pod'}}]}]},
        }
        for kind,objects in (('deployment',self.apps.deployments),('replicaset',self.apps.replicasets),('pod',self.apps.pods)):
            for obj in objects:self.objects[kind,obj['metadata']['name']]=obj

    def call(self,*args):
        return self.apps.call(*args)

    def process(self,args):
        if args[8]=='exec':
            if 'getent' in args:return b'10.1.0.1 voice-postgres\n'
            if args[-1]==schema.AUTH_ENV_POLICY:return b'ok'
            value=self.runtime[args[9]].get(args[-1])
            return b'M' if value is None else b'P'+value.encode()
        return json.dumps(self.objects[args[9],args[10]]).encode()


class ExternalPostgres:
    """Bounded synthetic process output; production catalog parser remains active."""
    def __init__(self,kube):
        self.kube=kube; self.oid=42; self.reads=0; self.drift=False; self.dirty=False
        databases=sorted((set(schema.OWNERS.values())|set(schema.OPTIONAL_DATABASES.values()))-{'realtime_db','user_db'})
        self.oids={name:100+index for index,name in enumerate(databases)}|{'user_db':self.oid}

    def process(self,args,limit):
        if args[8]!='exec' or args[9]!='voice-postgres-0':return self.kube.process(args)
        if args[-2]!='postgres':
            name=args[-2]
            value={'database':name,'database_oid':self.oids[name],'role':'voice','pid':123,'server_start':'fixture-start',
                   'read_only':'on','data_directory':'/var/lib/postgresql/data','version':1,'dirty':self.dirty,'ledger_rows':1,
                   'bad_versions':0,'ledger_kind':'flyway' if name=='auth_db' else 'go',
                   'voice_lifecycle_tables_present':True}
            return json.dumps(value).encode()
        def database(name,oid,allow,template):
            return {'name':name,'oid':oid,'connect':allow,'allowconn':allow,'template':template,'sessions':0,'prepared':0,
                    'encoding':'UTF8','collate':'C','ctype':'C','locale_provider':'c','icu_locale':None,
                    'connection_limit':-1,'owner':'voice','acl':None}
        self.reads+=1; oid=self.oid+int(self.drift and self.reads>1)
        databases=[database(name,oid if name=='user_db' else number,True,False) for name,number in sorted(self.oids.items())]
        value={'server_start':'fixture-start','can_read':True,'databases':[
            database('postgres',1,True,False),*databases,database('template0',2,False,True)]}
        raw=json.dumps(value).encode()
        if len(raw)>limit:raise ValueError('fixture response too large')
        return raw


class DriverJointTest(unittest.TestCase):
    def test_external_kube_fixture_authenticates_inventory_and_engine(self):
        kube=ExternalKube(); pods,controllers,_=inventory.inventory(kube)
        self.assertEqual(len(pods),23); self.assertEqual(len(controllers),23)
        observe=schema.kube_engine_snapshot(kube.process,lambda:None)
        self.assertEqual(observe()['pvc_uid'],'pvc')
        kube.objects['endpoints','voice-postgres']['subsets'][0]['addresses'][0]['targetRef']['uid']='foreign'
        with self.assertRaises(schema.Hold):observe()

    def test_external_kube_fixture_resolves_actual_pod_database_binding(self):
        kube=ExternalKube(); snap=schema.kube_workload_snapshot('user','voice-user-pod',kube.process,lambda:None)
        key=schema.kube_key_reader(kube.process); runtime=schema.kube_runtime_reader('user','voice-user-pod',kube.process)
        dns=schema.kube_dns_reader('user','voice-user-pod',kube.process)
        binding=schema.resolve('user',snap,key,runtime,dns,lambda:None)
        self.assertEqual(binding['database'],'user_db'); self.assertEqual(binding['addresses'],['10.1.0.1'])
        kube.runtime['voice-user-pod']['DATABASE_URL']='postgresql://voice:synthetic@voice-postgres:5432/foreign_db'
        with self.assertRaises(schema.Hold):schema.resolve('user',snap,key,runtime,dns,lambda:None)

    def test_external_pod_and_postgres_model_reach_real_schema_observer(self):
        kube=ExternalKube(); database=ExternalPostgres(kube)
        catalog=schema.catalog(schema_fixture.CatalogTest().snapshot())
        with patch.object(schema,'_bounded_process',side_effect=database.process):
            result=schema.inspect(catalog,'user','voice-user-pod','voice',lambda:None)
            database.dirty=True
            with self.assertRaises(schema.Hold):schema.inspect(catalog,'user','voice-user-pod','voice',lambda:None)
        self.assertEqual(result['metadata']['session']['database_oid'],42)
        self.assertEqual(result['metadata']['ledger']['version'],1)
        self.assertFalse(result['startup_qualified'])

    def test_external_model_reaches_full_inventory_observer_without_faking_holds(self):
        kube=ExternalKube(); database=ExternalPostgres(kube); pods,_,_=inventory.inventory(kube)
        snapshot=schema_fixture.CatalogTest().snapshot()
        for direction in ('up','down'):
            snapshot['files']['src/backend/migrations/gateway_db/000001_first.'+direction+'.sql']='a'*64
        catalog=schema.catalog(snapshot)
        with patch.object(schema,'_bounded_process',side_effect=database.process):
            result=schema.inspect_all(catalog,pods,'voice',lambda:None)
        self.assertEqual(set(result['workloads']),set(schema.CONTAINERS))
        self.assertEqual(result['workloads']['user'][0]['status'],'observed')
        self.assertEqual(result['workloads']['gateway'][0]['status'],'observed')
        self.assertEqual(result['workloads']['realtime'][0],{'status':'hold','reason':'database_configuration_missing'})
        self.assertEqual(result['workloads']['auth'][0],{'status':'hold','reason':'auth_source_policy_unknown'})
        for name in set(schema.CONTAINERS)-{'auth','realtime'}:
            self.assertEqual(result['workloads'][name][0]['status'],'observed',name)
        for name in ('chat','messaging'):
            for secondary in result['workloads'][name][0]['result']['secondary']:
                self.assertEqual(secondary['owner_consistency'],'matched')
        self.assertFalse(result['startup_qualified'])

    def test_external_postgres_catalog_runs_real_fenced_parser(self):
        case=fixture.RealPreflightTest('test_pre_supervisor_hook_uses_the_real_root_facade')
        created=[]; self.addCleanup(checked_nested,case,created); original=fixture.writer_cut_test.WriterCutTest
        def make(*args):
            base=checked_base(original(*args)); created.append(base); return base
        with patch.object(fixture.writer_cut_test,'WriterCutTest',side_effect=make):
            with case.phases() as work:
                kube=ExternalKube(); kube.objects['namespace','voice-dev']['metadata']['uid']=work.owner.namespace_uid
                database=ExternalPostgres(kube); engine=schema.kube_engine_snapshot(kube.process,work.owner.catalog._guard)()
                with patch.object(schema,'_bounded_process',side_effect=database.process):
                    result=affected._full_catalog(work.owner.catalog,engine,'fixture-start')
                    database.reads=0; database.drift=True
                    with self.assertRaises(ValueError):affected._full_catalog(work.owner.catalog,engine,'fixture-start')
                self.assertEqual(result['all_databases']['user_db'],42)
                self.assertEqual(result['excluded']['template0']['reason'],'connections_disabled_zero_activity')

    def test_changed_tt_generation_refuses_before_package_effect(self):
        """Exercise the real driver, supervisor, journals and fence without a live claim."""
        class StopFixture(Exception):pass
        case=fixture.RealPreflightTest('test_pre_supervisor_hook_uses_the_real_root_facade')
        created=[]; self.addCleanup(checked_nested,case,created)
        original=fixture.writer_cut_test.WriterCutTest
        def make(*args):
            base=checked_base(original(*args)); created.append(base); return base
        observed=[]
        def before(work):
            fixture.fixture_begin_image_only(work)
            owner=supervisor.Supervisor(work.journal,work.fence,work.client,work.base.lock,work.namespace_uid)
            owner._guard(); initial=copy.deepcopy(work.base.ledger.row); work.base.card['generation']+=1
            nodes=[{'name':'node','uid':'uid','hostname':'node','architecture':'amd64'}]
            with self.assertRaises(driver.DriverError):driver.C1Driver(owner).prepare('token','actor',1,1,nodes)
            self.assertIsNone(owner.package)
            self.assertFalse((work.operation/'package-journal').exists())
            self.assertNotIn('full_cohort_package',work.base.ledger.row)
            self.assertEqual(work.base.ledger.row,initial); observed.append(True)
            raise StopFixture()
        with patch.object(fixture.writer_cut_test,'WriterCutTest',side_effect=make):
            with self.assertRaises(StopFixture):
                with case.phases(before_supervisor=before):pass
        self.assertEqual(observed,[True])

    def test_nested_cleanup_failure_is_reported_and_remaining_cleanup_runs(self):
        outer=unittest.TestCase(); base=checked_base(unittest.TestCase()); events=[]
        base.addCleanup(lambda:events.append('later'))
        def fail():raise RuntimeError('synthetic cleanup failure')
        base.addCleanup(fail); outer.addCleanup(base.doCleanups)
        with self.assertRaises(RuntimeError):checked_nested(outer,[base])
        self.assertEqual(events,['later'])


if __name__=='__main__':unittest.main()
