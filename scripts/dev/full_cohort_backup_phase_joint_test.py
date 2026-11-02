"""Real typed preflight and distinct Journals; external Kube/TT are synthetic.

This is not a successful full backup or a live environment qualification.
"""
import copy
from contextlib import contextmanager
import hashlib
import io
import json
import os
from pathlib import Path
import subprocess
import time
import traceback
from types import SimpleNamespace
import unittest
from unittest import mock
import zipfile
from datetime import datetime, timedelta, timezone

import canonical_application_package as canonical
import cohort_backup_operation
import cohort_dispatch
import cohort_readback
import cohort_upload_phase
import cohort_upload_runner
import cohort_upload_run
import frontend_runtime_executor
import full_cohort_backup_metadata as metadata
import full_cohort_backup_phase as backup
import full_cohort_image_only_baseline as baseline
import full_cohort_image_only_baseline_test as baseline_fixture
import full_cohort_inventory_phase as inventory
import full_cohort_namespace_census as census
import full_cohort_package_phase as package
import full_cohort_pg_physical_identity as physical
import full_cohort_affected_plan as affected
import full_cohort_supervisor as supervisor
import media_operation_journal
import resource_fencing
import story_media_backup_capture as capture
import story_media_backup_cohort_custody as custody
import cohort_unmanaged_actual as unmanaged
import story_media_backup_restore as media_restore
import story_media_backup_integration_test as pg_fixture
from story_media_backup_capture_integration_test import cleanup_fixture
import update_images
import writer_cut
import writer_cut_test


def fixture_begin_image_only(work):
    """Run the real admission using synthetic predecessor ledger and TT history."""
    base, fence = work.base, work.fence
    previous = copy.deepcopy(base.context)
    previous.update(generation=2, revision=1, operation_id='media-c1-' + 'b' * 32,
                    claim='33333333-3333-4333-8333-333333333333',
                    window='44444444-4444-4444-8444-444444444444',
                    commit='b' * 40, status='complete')
    previous['actual'] = copy.deepcopy(previous['images'])
    base.ledger.row = copy.deepcopy(previous)
    history = baseline_fixture.HistoryTests().history()
    event = history['events'][0]
    for card in (event['before'], event['after']):
        card.update(owner=previous['actor'], claim=previous['claim'], window=previous['window'],
                    generation=previous['generation'])
        card['candidate'].update(commit=previous['commit'], images=copy.deepcopy(previous['images']))
    event['after']['observed'].update(actor=previous['actor'], claim=previous['claim'],
                                      window=previous['window'], commit=previous['commit'],
                                      images=copy.deepcopy(previous['images']))
    observed = copy.deepcopy(event['after']['observed'])
    base.card.update(operation=3, observed=observed)
    prior = copy.deepcopy(event['after'])
    for revision in (2, 3):
        next_card = copy.deepcopy(prior)
        next_card['revision'] = revision
        history['events'].append({'id': 'r' + str(revision), 'kind': 'checkpoint',
                                  'before': prior, 'after': next_card})
        prior = next_card
    tip = copy.deepcopy(prior)
    tip.update(copy.deepcopy(base.card))
    history['events'].append({'id': 'r4', 'kind': 'claim', 'actor': base.context['actor'],
                              'before': prior, 'after': tip})
    history['revision'] = 4
    with mock.patch.object(baseline.predecessor, 'TT_ENDPOINT', fence.check.endpoint), \
         mock.patch.object(baseline.predecessor, '_no_tt_proxy'), \
         mock.patch.object(baseline, '_read_history', return_value=history):
        fence.begin_image_only(work.client, base.lock, work.namespace_uid, work.journal)


class RealPreflightTest(unittest.TestCase):
    @contextmanager
    def phases(self, before_cut=None, after_cut=None, configure=None, before_supervisor=None):
        base=writer_cut_test.WriterCutTest('test_drain_and_resume_use_current_template_not_old')
        base.setUp();self.addCleanup(base.doCleanups)
        if configure is not None:configure(base)
        root=Path(base.temp.name)/'operations';root.mkdir(mode=0o700)
        op=root/base.context['operation_id'];op.mkdir(mode=0o700)
        admission=copy.deepcopy(base.admission);namespace_uid='00000000-0000-0000-0000-000000000001'
        external={'namespace_uid':namespace_uid}
        shared_workloads=base.client.workloads
        class Client(writer_cut_test.Client):
            def __init__(self,journal):
                super().__init__(base.lock,journal);self.workloads=shared_workloads
            def call(self,*args):
                if args[1]=='node':
                    return {'metadata':{'name':'node','uid':'uid','labels':{'kubernetes.io/os':'linux',
                        'kubernetes.io/arch':'amd64','kubernetes.io/hostname':'node'}},'spec':{},
                        'status':{'nodeInfo':{'architecture':'amd64','operatingSystem':'linux'},
                                  'conditions':[{'type':'Ready','status':'True'}]}}
                value=super().call(*args)
                if args[1]=='namespace':value['metadata']['uid']=external['namespace_uid']
                return value
        class Ledger:
            def __init__(self,client):self.client=client
            def read(self):return base.ledger.read()
            def write(self,revision,row):return base.ledger.write(revision,row)
        class Check:
            endpoint='http://fixture'
            def __init__(self,*args):self.context=args[-1]
            def __call__(self):return copy.deepcopy(base.card)
            def mark_uncertain(self,reason):pass
        check=Check(base.context)
        def facade(name,extra):
            intent={'resource_context':copy.deepcopy(base.context),'namespace_uid':namespace_uid,**extra}
            journal=media_operation_journal.Journal(op/name,intent);self.addCleanup(journal.close)
            client=Client(journal);fence=resource_fencing.Fence(copy.deepcopy(base.context),check,Ledger(client),check.mark_uncertain)
            return journal,client,fence
        with mock.patch.object(frontend_runtime_executor,'KubectlTransport',Client), \
             mock.patch.object(resource_fencing,'KubeLedger',Ledger),mock.patch.object(resource_fencing,'ResourceCheck',Check), \
             mock.patch.object(supervisor.media_backup_upload_runner,'ROOT',root), \
             mock.patch.object(supervisor.media_backup_upload_runner,'TT','http://fixture'), \
             mock.patch.object(backup.cohort_backup_operation,'ROOT',root):
            j,c,f=facade('journal',{'cohort_route_mode':'full-cohort-v1','kube_lock':base.lock})
            prepare = before_supervisor or fixture_begin_image_only
            prepare(SimpleNamespace(journal=j,client=c,fence=f,base=base,root=root,
                                    operation=op,namespace_uid=namespace_uid))
            owner=supervisor.Supervisor(j,f,c,base.lock,namespace_uid)
            pj,pc,pf=facade('package-journal',{'full_cohort_package':{'run_id':123,'run_attempt':1,
                'nodes':[{'name':'node','uid':'uid','hostname':'node','architecture':'amd64'}]}})
            owner.package=package.PackagePhase(pj,pf,pc,base.lock,namespace_uid)
            ij,ic,inf=facade('inventory-journal',{})
            owner.inventory=inventory.InventoryPhase(ij,inf,ic,base.lock,namespace_uid,owner.package)
            cj,cc,cf=facade('catalog-journal',{})
            owner.catalog=affected.FullCatalogPhase(cj,cf,cc,base.lock,namespace_uid,owner.inventory)
            nj,nc,nf=facade('census-journal',{})
            owner.census=census.NamespaceCensus(nj,nf,nc,base.lock,namespace_uid,owner.package,owner.inventory)
            probe=lambda scope:{'scope':scope,'active_sessions':0,'active_transactions':0}
            if before_cut is not None:admission,probe=before_cut(owner,admission,namespace_uid,base)
            wj,wc,wf=facade('cut-journal',{'writer_cut':admission})
            cut=writer_cut.WriterCut(wj,wf,wc,base.lock,namespace_uid,admission,probe)
            with mock.patch.object(cut,'_patch',side_effect=wc.patch):cut.drain()
            phj,phc,phf=facade('physical-journal',{})
            storage=physical.PhysicalPhase(phj,phf,phc,base.lock,namespace_uid,owner.catalog,cut)
            mj,mc,mf=facade('metadata-journal',{})
            source=metadata.BackupMetadataPhase(mj,mf,mc,base.lock,namespace_uid,owner.package,owner.inventory,owner.catalog,cut,metadata.canonical.ROOT)
            if after_cut is not None:after_cut(owner,cut,storage,source,namespace_uid)
            phase=backup.BackupPhase(owner,cut,storage,source)
            yield SimpleNamespace(base=base,root=root,operation=op,owner=owner,cut=cut,storage=storage,source=source,phase=phase,external=external)

    def test_missing_census_refuses_before_backup_journal_or_key(self):
        with self.phases() as work:
            phase=work.phase;op=work.operation;base=work.base
            with self.assertRaises(backup.BackupError):phase.run('fixture-token')
            self.assertFalse((op/'backup-journal').exists())
            self.assertFalse((op/'backup.key').exists())
            self.assertNotIn('full_cohort_backup_route',base.ledger.row)
            self.assertEqual(base.ledger.row['status'],'active')

    def test_pre_supervisor_hook_uses_the_real_root_facade(self):
        seen=[]
        def prepare(root):
            fixture_begin_image_only(root)
            self.assertIs(root.client.journal,root.journal)
            self.assertIs(root.fence.ledger.client,root.client)
            self.assertEqual(root.journal.intent['cohort_route_mode'],'full-cohort-v1')
            seen.append((root.journal,root.client,root.fence))
        with self.phases(before_supervisor=prepare) as work:
            self.assertEqual(len(seen),1)
            self.assertIs(work.owner.journal,seen[0][0])
            self.assertIs(work.owner.client,seen[0][1])
            self.assertIs(work.owner.fence,seen[0][2])

    def test_actual_guard_refuses_namespace_lock_and_tt_drift_before_effects(self):
        for fault in ('namespace','lock','tt'):
            with self.subTest(fault=fault),self.phases() as work:
                work.phase._guard()
                if fault=='namespace':work.external['namespace_uid']='foreign'
                elif fault=='lock':work.base.lock['metadata']['resourceVersion']='999'
                else:work.base.card['generation']+=1
                with self.assertRaises(Exception):work.phase._guard()
                with self.assertRaises(backup.BackupError):work.phase.run('fixture-token')
                self.assertFalse((work.operation/'backup-journal').exists())
                self.assertFalse((work.operation/'backup.key').exists())
                self.assertNotIn('full_cohort_backup_route',work.base.ledger.row)

    def test_synthetic_observations_reach_one_shot_dispatch_and_reconcile(self):
        """Real journals/capture/encryption; synthetic source, Kube, PG and failed GH."""
        self.scenario(False)

    def test_synthetic_transport_readback_and_real_pg16_restore(self):
        """Real capture/readback/restore; Kube, source and GitHub are synthetic."""
        self.scenario(True)

    def test_seed_hook_uses_actual_catalog_and_keeps_extra_database(self):
        def seed(pg):
            pg.sql(pg.source,'source_admin','postgres','CREATE DATABASE extra_db')
            pg.sql(pg.source,'source_admin','extra_db',
                   'CREATE SCHEMA tenant AUTHORIZATION app_writer; CREATE TABLE tenant.items(id integer PRIMARY KEY); '
                   'ALTER TABLE tenant.items OWNER TO app_writer; INSERT INTO tenant.items VALUES (7)')
        def checked(work,state):
            self.assertEqual(set(state['catalog']['databases']),{'postgres','template1','source_admin','extra_db'})
            self.assertEqual(set(state['catalog']['excluded']),{'template0'})
            self.assertEqual(backup.verify_completed(work.phase,'fixture-token')['scope'],
                             'full-cohort-private-backup-and-isolated-restore')
        self.scenario(True,seed=seed,after_restore=checked)

    def scenario(self, succeed, seed=None, source_files=None, kube_setup=None, after_restore=None, before_supervisor=None):
        images={name:canonical.REGISTRY+'/'+name+'@sha256:'+'b'*64 for name in canonical.IMAGE_NAMES}
        mapped={'voice-'+name+'/'+container:images[name] for name,container in update_images.SERVICES.items()}
        engine={'namespace_uid':'00000000-0000-0000-0000-000000000001',
                'pod_uid':'00000000-0000-0000-0000-000000000002',
                'pvc_uid':'00000000-0000-0000-0000-000000000003',
                'pv_uid':'00000000-0000-0000-0000-000000000004',
                'container_id':'containerd://'+'c'*64,'restart_count':0,'service_ip':'10.0.0.1'}
        start='2026-01-01T00:00:00Z'; catalog={'databases':{'postgres':1},'all_databases':{'postgres':1},
            'excluded':{},'properties':{'postgres':{'allowconn':True}}}
        state={};observations=[]
        if succeed:
            pg=pg_fixture.PopulatedRestoreTest();self.addCleanup(cleanup_fixture,pg);pg.setUp()
            deadline=time.monotonic()+60
            while True:
                ready=subprocess.run(['docker','exec',pg.source,'sh','-ec',
                    'test "$(cat /proc/1/comm)" = postgres && exec psql -X -qAt -v ON_ERROR_STOP=1 -U source_admin -d postgres -c "SELECT 1"'],
                    stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,timeout=10)
                if ready.returncode==0 and ready.stdout.strip()==b'1':break
                if time.monotonic()>=deadline:self.fail('source postgres readiness failed')
            pg.sql(pg.source,'source_admin','postgres',
                   "CREATE ROLE app_writer LOGIN; CREATE SCHEMA tenant AUTHORIZATION app_writer; "
                   "CREATE TABLE tenant.items(id integer PRIMARY KEY); ALTER TABLE tenant.items OWNER TO app_writer; "
                   "INSERT INTO tenant.items VALUES (7)")
            if seed is not None:seed(pg)
            source_catalog=json.loads(pg.sql(pg.source,'source_admin','postgres',unmanaged.CATALOG_SQL))
            roles=json.loads(pg.sql(pg.source,'source_admin','postgres',media_restore.role_catalog_sql()))
            oid=int(pg.sql(pg.source,'source_admin','postgres',"SELECT oid FROM pg_database WHERE datname='postgres'"))
            props=source_catalog['database_properties']
            selected={k:props[k] for k in ('template','allowconn','encoding','collate','ctype','locale_provider','icu_locale','connection_limit')}
            selected.update(owner=source_catalog['database_owner'],acl=props['acl_raw'])
            catalog={'databases':{'postgres':oid},'all_databases':{'postgres':oid},'excluded':{},'properties':{'postgres':selected}}
            state.update(pg=pg,roles=roles,source_catalog=source_catalog)
            if seed is not None:
                start=pg.sql(pg.source,'source_admin','postgres','SELECT pg_postmaster_start_time()').strip()
                query=("SELECT json_agg(json_build_object('name',datname,'oid',oid::bigint,'template',datistemplate,"
                       "'allowconn',datallowconn,'encoding',pg_encoding_to_char(encoding),'collate',datcollate,'ctype',datctype,"
                       "'locale_provider',datlocprovider::text,'icu_locale',daticulocale,'connection_limit',datconnlimit,"
                       "'owner',pg_get_userbyid(datdba),'acl',datacl::text) ORDER BY datname) FROM pg_database")
                rows=json.loads(pg.sql(pg.source,'source_admin','postgres',query));all_dbs={x['name']:x['oid'] for x in rows}
                active={x['name']:x['oid'] for x in rows if x['allowconn']}
                excluded={x['name']:{'oid':x['oid'],'template':x['template'],'reason':'connections_disabled_zero_activity'}
                          for x in rows if not x['allowconn']}
                properties={x['name']:{k:x[k] for k in ('template','allowconn','encoding','collate','ctype',
                            'locale_provider','icu_locale','connection_limit','owner','acl')} for x in rows}
                catalog={'databases':active,'all_databases':all_dbs,'excluded':excluded,'properties':properties}
                state['source_catalogs']={db:json.loads(pg.sql(pg.source,'source_admin',db,metadata._query_for(db))) for db in active}
            state.update(engine=engine,start=start,catalog=catalog)
        admin='source_admin' if succeed else 'postgres'
        def configure(base):
            base.context['images']=copy.deepcopy(mapped);base.fence.context['images']=copy.deepcopy(mapped)
            base.ledger.row['images']=copy.deepcopy(mapped);base.card['candidate']['images']=copy.deepcopy(mapped)
            base.lock['data']['resource_context']=json.dumps(base.context,sort_keys=True,separators=(',',':'))
            second=base.sources[1];second['kind']='Deployment'
            base.client.workloads.pop(('StatefulSet',second['metadata']['name']))
            base.client.workloads[('Deployment',second['metadata']['name'])]=copy.deepcopy(second)
            base.admission['controllers']=copy.deepcopy(base.sources)
        def package_capture(token,sha,run,attempt,output):
            data={'schema':'voice-canonical-application-package-v1','source_sha':sha,'verifier_sha':sha,'run_id':run,
                  'run_attempt':attempt,'request':{'cycle':1},'images':images,'runtime_qualified':False,'deployment_performed':False}
            output.write_text(json.dumps(data),encoding='utf8');output.chmod(0o600);return data
        def before(owner,admission,namespace_uid,base):
            state.update(owner=owner,base=base)
            root=owner.journal.path.parent.parent
            self.enterContext(mock.patch.object(package,'ROOT',root))
            self.enterContext(mock.patch.object(package.package,'capture',side_effect=package_capture))
            self.enterContext(mock.patch.object(package.package,'validate_images'))
            self.enterContext(mock.patch.object(package.oci,'resolve',return_value={k:'sha256:'+'f'*64 for k in mapped}))
            self.enterContext(mock.patch.object(package.source,'Registry'))
            self.enterContext(mock.patch.object(package.package.source,'_head'))
            self.enterContext(mock.patch.object(package.package.source,'_headers',return_value={}))
            candidate=owner.package.run('fixture-token','fixture-actor')
            pods={};controllers={};projection={};objects={}
            for item in base.sources:
                name=item['metadata']['name'].removeprefix('voice-');count=item['spec']['replicas']
                controllers['voice-'+name]=copy.deepcopy(item);pods[name]=['pod-'+name+str(i) for i in range(count)]
                children=[{'name':p,'uid':p,'replicaset_uid':'rs-'+name,'containers':[]} for p in pods[name]]
                projection[name]={'uid':item['metadata']['uid'],'generation':item['metadata']['generation'],
                    'spec_sha256':census.digest(item['spec']),'replicas':count,'pods':children}
                objects[name]=[{'name':p,'uid':p,'owner_uid':'rs-'+name,'containers':[]} for p in pods[name]]
            observed={'database_observations':{'workloads':{'fixture':[{'status':'observed','result':{
                      'metadata':{'engine':engine,'session':{'server_start':start}}}}]}}}
            self.enterContext(mock.patch.object(inventory,'inventory',return_value=(pods,controllers,projection)))
            self.enterContext(mock.patch.object(inventory.metadata,'catalog_from_source',return_value={}))
            self.enterContext(mock.patch.object(inventory.metadata,'inspect_all',return_value=observed['database_observations']))
            self.enterContext(mock.patch.object(owner.inventory,'_admin_role',return_value=admin))
            self.enterContext(mock.patch.object(census,'APP_NAMES',frozenset(('a','b'))))
            owner.inventory.run('fixture-token')
            def snapshot(token):
                uids={};specs={};generations={};current={}
                for item in base.sources:
                    name=item['metadata']['name'].removeprefix('voice-');live=base.client.workloads[(item['kind'],item['metadata']['name'])]
                    key=item['kind']+'/'+item['metadata']['name'];uids[key]=item['metadata']['uid']
                    specs[key]=census.digest(live['spec']);generations[key]=live['metadata']['generation']
                    current[key]=copy.deepcopy(objects[name]) if live['spec']['replicas'] else []
                return {'package_sha256':candidate['package_sha256'],'namespace_uid':namespace_uid,
                        'baseline':{'workload_uids':uids,'workload_spec_sha256':specs},
                        'generations':generations,'application_pods':current}
            self.enterContext(mock.patch.object(owner.census,'_snapshot',side_effect=snapshot))
            owner.census.run('fixture-token')
            self.enterContext(mock.patch.object(affected,'_full_catalog',return_value=catalog))
            self.enterContext(mock.patch.object(affected.pg,'inspect',side_effect=lambda scope,guard:
                {'scope':scope,'active_sessions':0,'active_transactions':0}))
            receipt=owner.catalog.run('fixture-token')
            admission['session_scope']={'engine':receipt['engine'],'server_start':start,'databases':sorted(catalog['all_databases']),
                                        'database_oids':catalog['all_databases']}
            return admission,affected.CatalogScopedProbe(owner.catalog,'fixture-token')
        def after(owner,cut,storage,source,namespace_uid):
            expected={k:engine[k] for k in ('namespace_uid','pod_uid','pvc_uid','pv_uid','container_id','restart_count')}
            expected.update({'statefulset_uid':'statefulset','pv_name':'pv','container':'postgres',
                 'mount_path':'/var/lib/postgresql/data','data_directory':'/var/lib/postgresql/data/pgdata','pg_major':16}
                            )
            observed={'expected':expected,'engine':engine,'server_start':start,'database_oid':catalog['databases']['postgres'],
                      'system_identifier':'7','node':{'name':'node','uid':'uid'}}
            self.enterContext(mock.patch.object(physical.PhysicalPhase,'_observe',return_value=observed))
            self.enterContext(mock.patch.object(metadata.media_backup_upload_phase,'ROOT',owner.journal.path.parent.parent))
            self.enterContext(mock.patch.object(metadata.schema,'kube_engine_snapshot',return_value=lambda:engine))
            if not succeed:self.enterContext(mock.patch.object(metadata.unmanaged,'validate_source_catalog'))
            files={} if source_files is None else source_files()
            self.enterContext(mock.patch.object(source,'_source_files',return_value={'files':files}))
            self.enterContext(mock.patch.object(source,'_read',side_effect=lambda db,*args:
                (copy.deepcopy(state['roles']) if db=='roles' else copy.deepcopy(state['source_catalogs'][db] if seed is not None else state['source_catalog'])) if succeed else
                ({'roles':[],'memberships':[]} if db=='roles' else {'database_owner':'postgres'})))
            def tool():source._tool_state({'status':'removed'});return 'd'*64
            if seed is None:self.enterContext(mock.patch.object(source,'_tool_sha',side_effect=tool))
            state.update(owner=owner,cut=cut,source=source,storage=storage)
            if kube_setup is not None:kube_setup(state,self.enterContext)
            storage.run('fixture-token');source.run('fixture-token')
        with self.phases(before_cut=before,after_cut=after,configure=configure,before_supervisor=before_supervisor) as work:
            self.enterContext(mock.patch.object(cohort_upload_phase,'ROOT',work.root))
            self.enterContext(mock.patch.object(backup.publish_candidate,'source_snapshot',return_value={}))
            original_run=subprocess.run
            if succeed:
                output=work.operation/'github-output';output.touch(mode=0o600)
                env={'GITHUB_REPOSITORY':cohort_upload_run.REPOSITORY,'GITHUB_EVENT_NAME':'workflow_dispatch',
                     'GITHUB_REF':'refs/heads/master','GITHUB_WORKFLOW_REF':cohort_upload_run.WORKFLOW_REF,
                     'GITHUB_SHA':work.base.context['commit'],'GITHUB_WORKFLOW_SHA':work.base.context['commit'],
                     'GITHUB_RUN_ID':'123','GITHUB_RUN_ATTEMPT':'1','GITHUB_OUTPUT':str(output)}
                self.enterContext(mock.patch.object(cohort_upload_runner,'ROOT',work.root))
                self.enterContext(mock.patch.object(cohort_upload_runner,'OPERATOR_UID',os.geteuid()))
                self.enterContext(mock.patch.object(cohort_upload_runner.media,'Client',return_value=work.owner.client))
                now=datetime.now(timezone.utc);created=(now-timedelta(seconds=1)).isoformat()
                run_json={'id':123,'head_sha':env['GITHUB_SHA'],'head_branch':'master','event':'workflow_dispatch',
                          'path':cohort_readback.WORKFLOW,'run_attempt':1,'status':'completed','conclusion':'success'}
                def get_json(token,url):
                    if url.endswith('/artifacts?per_page=100'):
                        return {'total_count':1,'artifacts':[{'id':456,'name':state['artifact_name'],'created_at':created}]}
                    return run_json
                self.enterContext(mock.patch.object(cohort_readback,'get_json',side_effect=get_json))
                def upload_transport(command,payload):
                    request=json.loads(payload);challenge=request['inputs']['challenge']
                    state['artifact_name']='voice-cohort-backup-'+work.base.context['operation_id']+'-'+challenge
                    try:path=cohort_upload_runner.stage(env,work.base.context['operation_id'],challenge,Path('/fixture'))
                    except Exception as exc:
                        state['upload_failure']=(type(exc).__name__,[(Path(x.filename).name,x.lineno,x.name) for x in traceback.extract_tb(exc.__traceback__)])
                        raise
                    wire=io.BytesIO()
                    with zipfile.ZipFile(wire,'w',compression=zipfile.ZIP_STORED) as zipped:zipped.writestr('cohort-backup.tar',path.read_bytes())
                    state['artifact']=wire.getvalue()
                    return json.dumps({'workflow_run_id':123,'run_url':cohort_readback.API+'123',
                        'html_url':'https://github.com/Poryadok/VoiceRoot/actions/runs/123'}).encode()
                self.enterContext(mock.patch.object(cohort_dispatch,'_transport',side_effect=upload_transport))
                def download(url,headers,target,limit,deadline,redirects=False):
                    data=state['artifact'] if redirects else json.dumps({'id':456,'url':custody.API+'456','expired':False,
                        'name':state['artifact_name'],'created_at':now.isoformat(),'expires_at':(now+timedelta(days=1)).isoformat(),
                        'workflow_run':{'id':123,'head_sha':env['GITHUB_SHA']},'size_in_bytes':len(state['artifact'])}).encode()
                    target.write(data);return len(data)
                self.enterContext(mock.patch.object(custody.transport,'_download',side_effect=download))
            else:
                self.enterContext(mock.patch.object(cohort_dispatch,'_transport',side_effect=OSError('synthetic lost dispatch')))
            mount='42 1 0:42 / /var/lib/postgresql/data rw - ext4 /dev/x rw\n'
            def transport(command,**kwargs):
                if command[0]!='kubectl':return original_run(command,**kwargs)
                text=' '.join(command)
                if 'get' in command:
                    kind=command[command.index('get')+1];objects={
                        'namespace':{'metadata':{'uid':engine['namespace_uid']}},
                        'pod':{'metadata':{'uid':engine['pod_uid'],'namespace':'voice-dev','ownerReferences':[{'uid':'statefulset','kind':'StatefulSet','controller':True}]},
                               'spec':{'containers':[{'name':'postgres','volumeMounts':[{'name':'data','mountPath':'/var/lib/postgresql/data'}]}],
                                       'volumes':[{'name':'data','persistentVolumeClaim':{'claimName':'pgdata-voice-postgres-0'}}]},
                               'status':{'containerStatuses':[{'name':'postgres','containerID':engine['container_id'],'restartCount':0,'ready':True,'state':{'running':{}}}]}},
                        'pvc':{'metadata':{'uid':engine['pvc_uid'],'namespace':'voice-dev'},'spec':{'volumeName':'pv'},'status':{'phase':'Bound'}},
                        'pv':{'metadata':{'uid':engine['pv_uid']},'spec':{'claimRef':{'uid':engine['pvc_uid'],'namespace':'voice-dev','name':'pgdata-voice-postgres-0'}}}}
                    return SimpleNamespace(returncode=0,stdout=json.dumps(objects[kind]).encode())
                if 'pg_postmaster_start_time' in text:
                    db=command[-2]
                    return SimpleNamespace(returncode=0,stdout=json.dumps({'database':db,'database_oid':catalog['databases'][db],
                                                                           'role':admin,'server_start':start}).encode())
                if 'pg_read_file' in text:
                    return SimpleNamespace(returncode=0,stdout=json.dumps({'data_directory':'/var/lib/postgresql/data/pgdata','version':'160014','mountinfo':mount}).encode())
                if 'readlink' in text:
                    return SimpleNamespace(returncode=0,stdout=('/var/lib/postgresql/data/pgdata\n/var/lib/postgresql/data\n'+mount).encode())
                observations.append(command[-1])
                if succeed:
                    program=['pg_dumpall','-U',admin,'--roles-only','--no-role-passwords'] if command[-1]=='roles' else ['pg_dump','-U',admin,'-Fc','-d',command[-1]]
                    return original_run(['docker','exec',state['pg'].source,*program],stdout=kwargs['stdout'],stderr=subprocess.DEVNULL,timeout=90)
                kwargs['stdout'].write(b'synthetic-pg-dump');return SimpleNamespace(returncode=0,stdout=b'')
            with mock.patch.object(capture.subprocess,'run',side_effect=transport):
                if succeed:
                    original_psql=media_restore._psql;checked=[]
                    def checked_psql(cid,role,db,sql,timeout=120):
                        result=original_psql(cid,role,db,sql,timeout=timeout)
                        if (db=='postgres' or (seed is not None and db=='extra_db')) and sql==unmanaged.CATALOG_SQL:
                            value=original_psql(cid,role,db,"SELECT id,pg_get_userbyid(relowner) FROM tenant.items CROSS JOIN pg_class WHERE oid='tenant.items'::regclass")
                            self.assertEqual(value.strip(),b'7|app_writer');checked.append(True)
                        return result
                    with mock.patch.object(media_restore,'_psql',side_effect=checked_psql):
                        try:result=work.phase.run('fixture-token')
                        except backup.BackupError:
                            steps=sorted(work.phase.journal.state['steps']) if work.phase.journal else []
                            self.fail('synthetic joint incomplete; stages='+repr(steps)+'; dump_count='+str(len(observations))+'; upload='+repr(state.get('upload_failure')))
                    self.assertEqual(len(checked),4 if seed is not None and 'extra_db' in catalog['databases'] else 2)
                    self.assertEqual(result['scope'],'full-cohort-private-backup-and-isolated-restore')
                    self.assertEqual(backup.verify_completed(work.phase,'fixture-token'),result)
                    if after_restore is not None:after_restore(work,state)
                else:
                    with self.assertRaises(backup.BackupError):work.phase.run('fixture-token')
                self.assertEqual(sorted(observations),sorted(list(catalog['databases'])+['roles']))
                with self.assertRaises(backup.BackupError):work.phase.run('fixture-token')
            self.assertEqual(work.base.ledger.row['status'],'active' if succeed else 'uncertain')


if __name__=='__main__':unittest.main()
