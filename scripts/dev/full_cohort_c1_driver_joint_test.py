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
import media_live_baseline as live_baseline


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
    def __init__(self,images=None,platform_digests=None):
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
            key=deployment['metadata']['name']+'/'+schema.CONTAINERS[workload]
            image=(images or {}).get(key,'ghcr.io/poryadok/voiceroot/'+workload+'@sha256:'+'b'*64)
            deployment['spec']['template']['spec']['containers'][0]['image']=image
            pod['spec']['containers'][0]['image']=image
            if platform_digests and key in platform_digests:
                pod['status']['containerStatuses'][0]['imageID']='containerd://'+platform_digests[key]
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
        if args[9]=='--raw':
            path=args[10].split('?',1)[0]
            name='namespace' if path=='/api/v1/namespaces/voice-dev' else path.rsplit('/',1)[-1]
            if name=='persistentvolumeclaims':name='pvcs'
            return json.dumps(self.docs[name]).encode()
        if args[9]=='configmap' and args[-1]=='json':
            name=args[10]; return json.dumps({'kind':'ConfigMap','metadata':{'name':name,'namespace':'voice-dev',
                'uid':'config-'+name,'resourceVersion':'1'},'data':copy.deepcopy(self.configs[name])}).encode()
        if args[9]=='secret' and 'range $k' in args[-1]:
            name=args[10]; keys=self.secrets[name]
            return ('secret-'+name+'\t1\n'+'\n'.join(sorted(keys))+'\n').encode()
        return json.dumps(self.objects[args[9],args[10]]).encode()

    def enable_census(self):
        """Reuse canonical infra shapes, joined to the same 23 application Pods."""
        import full_cohort_namespace_census as census
        import full_cohort_namespace_census_test as census_fixture
        root,docs=census_fixture.SnapshotTest().fixture()
        docs['namespace']['metadata']['uid']='namespace'
        docs['namespace']['metadata']['labels']={'voice.environment':'dev','app.kubernetes.io/managed-by':'voice-dev'}
        model={row['metadata']['name']:row for row in self.apps.deployments}
        old_pods={row['metadata']['name']:row for row in self.apps.pods}
        for deployment in docs['deployments']['items']:
            name=deployment['metadata']['name']
            if name not in model:continue
            source=model[name]; spec=deployment['spec']; app=source['spec']['template']['spec']['containers'][0]
            sidecars=spec['template']['spec']['containers'][1:]
            spec['selector']=copy.deepcopy(source['spec']['selector']); spec['replicas']=1
            spec['template']['spec']['containers']=[copy.deepcopy(app),*sidecars]
            deployment['status']={'observedGeneration':1,'replicas':1,'updatedReplicas':1,'readyReplicas':1}
            pod=next(row for row in docs['pods']['items'] if row['metadata']['name']==name+'-pod')
            pod['metadata']['labels']=copy.deepcopy(spec['selector']['matchLabels'])
            pod['spec']['containers']=copy.deepcopy(spec['template']['spec']['containers'])
            old=old_pods[name+'-pod']['status']['containerStatuses'][0]
            statuses=pod['status']['containerStatuses']; statuses[0]=copy.deepcopy(old)
        apps=set('voice-'+name for name in census.APP_NAMES)
        self.apps.deployments=[row for row in docs['deployments']['items'] if row['metadata']['name'] in apps]
        self.apps.replicasets=[row for row in docs['replicasets']['items'] if row['metadata']['name'].removesuffix('-rs') in apps]
        self.apps.pods=[row for row in docs['pods']['items'] if row['metadata']['name'].removesuffix('-pod') in apps]
        for kind,objects in (('deployment',self.apps.deployments),('replicaset',self.apps.replicasets),('pod',self.apps.pods)):
            for obj in objects:self.objects[kind,obj['metadata']['name']]=obj
        postgres=next(row for row in docs['statefulsets']['items'] if row['metadata']['name']=='voice-postgres')
        pgpod=next(row for row in docs['pods']['items'] if row['metadata']['name']=='voice-postgres-pod')
        pgpod['metadata']['name']='voice-postgres-0'; pgpod['metadata']['labels']={'app':'voice-postgres'}
        pgpod['status']['podIP']='10.0.0.1'; self.objects['statefulset','voice-postgres']=postgres; self.objects['pod','voice-postgres-0']=pgpod
        pvc=next(row for row in docs['pvcs']['items'] if row['metadata']['name']=='pgdata-voice-postgres-0')
        pvc['spec']={'volumeName':'pv-one'}; self.objects['persistentvolumeclaim','pgdata-voice-postgres-0']=pvc
        self.objects['persistentvolume','pv-one']['spec']['claimRef']['uid']=pvc['metadata']['uid']
        self.objects['endpoints','voice-postgres']['subsets'][0]['addresses'][0]['targetRef']['uid']=pgpod['metadata']['uid']
        self.objects['endpointslices','-l']['items'][0]['endpoints'][0]['targetRef']['uid']=pgpod['metadata']['uid']
        self.configs={'voice-dev-mail':{'server.mjs':(root/'docker/verification-stub/server.mjs').read_text(encoding='utf8')},
                      'voice-nats-hub-config':{'nats.conf':'synthetic'},'voice-livekit-config':{'livekit.yaml':'synthetic'}}
        self.secrets={'voice-app-secrets':{'DATABASE_URL'}}
        def refs(value):
            if isinstance(value,dict):
                for key,child in value.items():
                    if isinstance(child,dict) and key in ('configMap','configMapRef','configMapKeyRef'):
                        name=child.get('name'); entries=self.configs.setdefault(name,{}) if name else None
                        if entries is not None:
                            if child.get('key'):entries.setdefault(child['key'],'synthetic')
                            for item in child.get('items',[]):entries.setdefault(item['key'],'synthetic')
                    if isinstance(child,dict) and key in ('secret','secretRef','secretKeyRef'):
                        name=child.get('name',child.get('secretName')); entries=self.secrets.setdefault(name,set()) if name else None
                        if entries is not None:
                            if child.get('key'):entries.add(child['key'])
                            entries.update(item['key'] for item in child.get('items',[]))
                    refs(child)
            elif isinstance(value,list):
                for item in value:refs(item)
        for kind in ('deployments','statefulsets'):
            for obj in docs[kind]['items']:refs(obj['spec']['template']['spec'])
        self.docs=docs
        return root


class ExternalPostgres:
    """Bounded synthetic process output; production catalog parser remains active."""
    def __init__(self,kube,catalog=None):
        self.kube=kube; self.oid=42; self.reads=0; self.drift=False; self.dirty=False
        databases=sorted((set(schema.OWNERS.values())|set(schema.OPTIONAL_DATABASES.values()))-{'realtime_db','user_db'})
        self.oids={name:100+index for index,name in enumerate(databases)}|{'user_db':self.oid}
        self.versions={name:min(entry['versions']) for name,entry in (catalog or {}).get('databases',{}).items()}

    def process(self,args,limit):
        if args[8]!='exec' or args[9]!='voice-postgres-0':return self.kube.process(args)
        if args[-2]!='postgres':
            name=args[-2]
            value={'database':name,'database_oid':self.oids[name],'role':'voice','pid':123,'server_start':'fixture-start',
                   'read_only':'on','data_directory':'/var/lib/postgresql/data','version':self.versions.get(name,1),'dirty':self.dirty,'ledger_rows':1,
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

    def test_same_raw_kube_fixture_supports_inventory_census_and_engine(self):
        import full_cohort_namespace_census as census
        kube=ExternalKube(); source_root=kube.enable_census()
        result=live_baseline.collect('default','voice-dev',reader=lambda _context,_namespace,name:copy.deepcopy(kube.docs[name]))
        pods,controllers,_=inventory.inventory(kube)
        self.assertEqual((len(result['workload_uids']),len(result['images'])),(31,49))
        self.assertEqual((len(pods),len(controllers)),(23,23))
        self.assertEqual(schema.kube_engine_snapshot(kube.process,lambda:None)()['pod_uid'],kube.objects['pod','voice-postgres-0']['metadata']['uid'])
        database=ExternalPostgres(kube); catalog=schema.catalog(schema_fixture.CatalogTest().snapshot())
        with patch.object(schema,'_bounded_process',side_effect=database.process):
            observed=schema.inspect(catalog,'user',pods['user'][0],'voice',lambda:None)
        self.assertEqual(observed['metadata']['session']['database_oid'],42)
        mail=(source_root/'docker/verification-stub/server.mjs').read_text(encoding='utf8')
        for kind,name in census.INFRA_NAMES:
            listing='deployments' if kind=='Deployment' else 'statefulsets'
            obj=next(row for row in kube.docs[listing]['items'] if row['metadata']['name']==name)
            census.validate_infra(name,obj['spec']['template']['spec'],{'server.mjs':mail} if name=='voice-dev-mail' else None,
                                  mail if name=='voice-dev-mail' else None)
        redis=next(row for row in kube.docs['deployments']['items'] if row['metadata']['name']=='voice-redis')
        foreign=copy.deepcopy(redis['spec']['template']['spec']); foreign['containers'][0].setdefault('env',[]).append({'name':'DATABASE_URL','value':'foreign'})
        with self.assertRaises(census.CensusError):census.validate_infra('voice-redis',foreign)

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
            rows=result['workloads'][name][0]['result']['secondary']
            self.assertEqual({row['metadata']['database'] for row in rows},set(schema.SECONDARY[name].values()))
            for secondary in rows:
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

    def test_census_uses_same_raw_kube_objects_under_real_guard(self):
        case=fixture.RealPreflightTest('test_pre_supervisor_hook_uses_the_real_root_facade')
        created=[]; self.addCleanup(checked_nested,case,created); original=fixture.writer_cut_test.WriterCutTest
        def make(*args):
            base=checked_base(original(*args)); created.append(base); return base
        with patch.object(fixture.writer_cut_test,'WriterCutTest',side_effect=make):
            with case.phases() as work:
                kube=ExternalKube(); root=kube.enable_census(); kube.docs['namespace']['metadata']['uid']=work.owner.namespace_uid
                process=ExternalPostgres(kube)
                with patch.object(schema,'_bounded_process',side_effect=process.process):
                    self.assertEqual(len(work.owner.census._read('deployments')['items']),28)
                    self.assertEqual(work.owner.census._read('namespace')['metadata']['uid'],work.owner.namespace_uid)
                    self.assertEqual(work.owner.census._config('voice-dev-mail')['data']['server.mjs'],
                                     (root/'docker/verification-stub/server.mjs').read_text(encoding='utf8'))
                    self.assertIn('DATABASE_URL',work.owner.census._secret_metadata('voice-app-secrets')['keys'])

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
