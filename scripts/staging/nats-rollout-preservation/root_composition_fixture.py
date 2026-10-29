"""Private synthetic cluster fixture with source-checked installed helpers."""
import hashlib
import json
import os
import base64
import contextlib
from pathlib import Path
import shutil
import tarfile

import staging_prerequisites as prerequisites
from root_main import save


ARCHIVE_SHA256='a3b48489d83965fbd96512d501989551db063dcab6d46943dea413a2428d6a1a'
ARCHIVE_SIZE=38502400
SYNTHETIC_BINARIES={name:('synthetic installed binary: '+name+'\n').encode() for name in
    ('nats-known-baseline/kernel','nats-rollout-preservation/bootstrap-renewer',
     'nats-rollout-preservation/story-current4-probe')}
SYNTHETIC_PINS={name:hashlib.sha256(data).hexdigest() for name,data in SYNTHETIC_BINARIES.items()}


def helper_binding(source, code):
    """Use current source for Python helpers and independently pinned binary bytes."""
    archive_name=os.environ.get('VOICE_TEST_PINNED_ARCHIVE')
    archive=Path(archive_name) if archive_name else None
    if archive is not None:
        raw=archive.read_bytes()
        if len(raw)!=ARCHIVE_SIZE or hashlib.sha256(raw).hexdigest()!=ARCHIVE_SHA256:
            raise ValueError('pinned_fixture_archive_changed')
    code.mkdir(parents=True,mode=0o700)
    import bundle
    members=({'nats-known-baseline/'+name for name in bundle.KNOWN}
        | {'nats-rollout-preservation/'+name for name in bundle.ROLLOUT}
        | {'configure-kubectl-ci.sh','mail-only-patch.py'})
    binding={}
    for name in sorted(members):
        path=code/name;path.parent.mkdir(parents=True,exist_ok=True,mode=0o700)
        shutil.copyfile(source/'scripts'/'staging'/name,path);path.chmod(0o600)
        binding[name]=hashlib.sha256(path.read_bytes()).hexdigest()
    binaries=SYNTHETIC_BINARIES
    if archive is not None:
        binaries={}
        with tarfile.open(archive,'r:') as bundle:
            for name in prerequisites.BINARY_PINS:
                matched=[entry for entry in bundle if entry.name==name]
                if len(matched)!=1 or not matched[0].isfile():raise ValueError('pinned_fixture_member_invalid')
                binaries[name]=bundle.extractfile(matched[0]).read()
    for name,wanted in prerequisites.BINARY_PINS.items():
        data=binaries[name]
        if hashlib.sha256(data).hexdigest()!=wanted:raise ValueError('pinned_fixture_member_changed')
        path=code/name;path.parent.mkdir(parents=True,exist_ok=True,mode=0o700);path.write_bytes(data);path.chmod(0o600)
        binding[name]=wanted
    save(code/'capture-manifest.json',binding)
    return binding


def exercise_candidate_drift(test, *, accept_candidate=False, trace_after_capture=False):
    """Join real bridge/source/candidate admission; cluster and OCI are fixtures."""
    import copy
    from unittest.mock import patch
    import bridge_root
    import commands
    import resource_claim
    import resource_claim_test
    import root_cli
    import runtime_stage
    import stage_runtime
    import gis_first_install
    import uuid
    import realtime_bootstrap as realtime
    import nats_root_plan
    import actor_root
    import space_safeguards
    import bootstrap_selection
    import bootstrap_selection_test
    import compiler
    import source_plan_test
    import re
    import source_authority
    import staging_prerequisites_test
    import staging_release_policy_prepare as policy

    real_prepare=prerequisites.run_prepare
    real_candidate=prerequisites._candidate_images
    real_policy_source=policy.validate_source
    required=prerequisites.REQUIRED_HELPERS
    production_pins=prerequisites.BINARY_PINS
    source=Path(__file__).resolve().parents[3]

    class Rejected(Exception):pass

    def join(actions,request,approved,execution,workspace):
        root=workspace.parents[2];installed=workspace.parents[1];old_session=actions.tt_session
        saved=workspace.with_name(workspace.name+'-fixture');workspace.rename(saved)
        (installed/'source-authority'/(old_session.operation+'.json')).unlink()
        if trace_after_capture:
            compiler_kube,params=source_plan_test.SourceTests().canonical_fixture('app-only')
            optional={'s3_signing_endpoint','gateway_host','storage_host','livekit_host','web_host','admin_host',
                'developer_portal_host','gateway_tls_secret','storage_tls_secret','image_pull_secret','apply_observability',
                'minio_image','minio_mc_image','minio_storage_class','minio_storage_size','web_origin','release_policy'}
            save(installed/'policy.json',{key:value for key,value in params.items() if key in optional})
            _,prior,nats_rows,nats_account_sha=bootstrap_selection_test.fixture()
            prior['binding']['marker_uid']=str(uuid.UUID(int=8))
            save(root/'bootstrap-enrollment.json',prior)
            save(root/'bootstrap-enrollment-v4.json',{'schema':'voice-nats-bootstrap-enrollment-v4',
                'previous_enrollment':prior,'previous_enrollment_sha256':resource_claim.digest(prior)})
        code=installed/'code';shutil.rmtree(code)
        pins=production_pins if os.environ.get('VOICE_TEST_PINNED_ARCHIVE') else SYNTHETIC_PINS
        with patch.object(prerequisites,'BINARY_PINS',pins),patch.object(prerequisites,'REQUIRED_HELPERS',required):
            binding=helper_binding(source,code)
            images={name:source_authority.REGISTRY+'/'+name+'@sha256:'+'a'*64 for name in source_authority.IMAGE_NAMES}
            stage=runtime_stage.RolloutStage.capture.return_value
            if trace_after_capture:
                wanted=gis_first_install.baseline_workload_names(include_hub=True)-{'voice-p3-principal-jwks',stage_runtime.HUB}
                snapshots={name:copy.deepcopy(compiler_kube.objects[('Deployment',name)]) for name in wanted}
                snapshots[stage_runtime.HUB]={'apiVersion':'apps/v1','kind':'Deployment','metadata':{
                    'name':stage_runtime.HUB,'namespace':'voice-staging'},'spec':{'replicas':1,'template':{'spec':{
                    'containers':[{'name':'nats','image':'example.invalid/nats@sha256:'+'c'*64}]}}}}
                for index,name in enumerate(sorted(snapshots)):
                    snapshots[name]['metadata'].update(uid=str(uuid.UUID(int=index+100)),generation=1,namespace='voice-staging')
                stage.snapshots=snapshots;stage.original_snapshots=copy.deepcopy(snapshots)
                stage.expected['deployment_uids']={name:row['metadata']['uid'] for name,row in snapshots.items()}
                stage.old_images={name+'/'+container['name']:'example.invalid/'+container['name']+'@sha256:'+'c'*64
                    for name,row in snapshots.items() for container in row['spec']['template']['spec'].get('initContainers',[])
                    +row['spec']['template']['spec'].get('containers',[])}
            context=copy.deepcopy(old_session.context)
            candidate=images['gameintegration'] if accept_candidate else images['gameintegration'][:-1]+'b'
            context['images']={'voice-gameintegration/gameintegration':candidate}
            card=resource_claim_test.card(context);card['observed']['images']=copy.deepcopy(stage.old_images)
            session=resource_claim.Session(context,old_session.operation,resource_claim.digest(binding),
                lambda:copy.deepcopy(card),resource_claim_test.MemoryJournal());session.admit(start=True)
            actions.code=code;actions.binding=binding;actions.tt_session=session
            execution={'dispatcher_run_id':17,'head_sha':context['commit']}
            approved={'schema':'voice-source-authority-v1','source_sha':context['commit'],'repository':source_authority.REPO,
                'workflow_id':source_authority.WORKFLOW,'run_id':17,'run_attempt':1,'tree_sha':'b'*40,'lock_artifact_id':19,
                'lock_sha256':'d'*64,'image_provenance':'synthetic test','verified':True,'source_files':{},'images':images}
            def capture(_token,_run,_sha,destination):
                for part in ('scripts','deploy','src/backend/migrations','src/backend/auth/src/main/resources/db/migration','docker'):
                    shutil.copytree(source/part,destination/part,dirs_exist_ok=True)
                for path in destination.rglob('*'):path.chmod(0o700 if path.is_dir() else 0o600)
                approved['source_files']={path.relative_to(destination).as_posix():hashlib.sha256(path.read_bytes()).hexdigest()
                    for path in destination.rglob('*') if path.is_file()}
                return copy.deepcopy(approved)
            request=dict(request,action='prepare',nonce=session.operation+'a'*52,token='fixture')
            source_get=stage_runtime.Kube.get.side_effect
            trace_secret=[None]
            if trace_after_capture:
                claim='voice-nats-jsdata-d202610040049430b'
                stage.marker['data'].update(generation='r20260930a4',dataPVC=claim)
                stage.marker['metadata']['uid']=str(uuid.UUID(int=8))
                stage.expected['source_claim']=claim
                stage.expected['generation']='r20260930a4'
                stage.final_claim['metadata']['name']=claim
                stage.final_claim['status']={'capacity':{'storage':'1Gi'}}
                stage.final_pv['spec']['local']['path']=('/var/lib/rancher/k3s/storage/pvc-'+
                    stage.final_claim['metadata']['uid']+'_voice-staging_'+claim)
                stage.final_path=Path(stage.final_pv['spec']['local']['path'])
                stage.final_path.mkdir(parents=True,mode=0o700)
                (stage.final_path/'fixture.dat').write_bytes(b'fixture native stream')
                stage.service={'metadata':{'uid':str(uuid.UUID(int=9)),'resourceVersion':'1'}}
            def initial_get(kind,name):
                if trace_after_capture:
                    if kind=='deployment' and name in stage.snapshots and name!='voice-p3-principal-jwks':
                        return copy.deepcopy(stage.snapshots[name])
                    if (kind,name)==('secret','voice-app-secrets') and trace_secret[0] is not None:
                        return copy.deepcopy(trace_secret[0])
                    if (kind,name)==('configmap','voice-nats-generation'):return copy.deepcopy(stage.marker)
                    if (kind,name)==('pvc',stage.expected['source_claim']):return copy.deepcopy(stage.final_claim)
                    if (kind,name)==('pv',stage.final_claim['spec']['volumeName']):return copy.deepcopy(stage.final_pv)
                return source_get(kind,name)
            def capture_stage(*args,**kwargs):
                if session.check()['phase']=='prerequisites':
                    publisher=stage.kube.get('deployment','voice-p3-principal-jwks')
                    stage.snapshots['voice-p3-principal-jwks']=publisher
                    stage.original_snapshots=copy.deepcopy(stage.snapshots)
                    stage.expected['deployment_uids']['voice-p3-principal-jwks']=publisher['metadata']['uid']
                    stage.old_images=prerequisites._checkpoint(session)['receipt']['actual_after']
                return stage
            class ReachedPolicy(Exception):pass
            trace_stack=contextlib.ExitStack()
            def report(name,status):
                if not trace_after_capture and (name,status)==('policy-validation','STARTED'):raise ReachedPolicy()
                if trace_after_capture and (name,status)==('policy-validation','STARTED'):
                    old_get=stage_runtime.Kube.get.side_effect
                    secret=old_get('secret','voice-app-secrets')
                    trace_secret[0]=secret
                    role={'oid':13,'login':True,'superuser':False,'createdb':False,'createrole':False,
                        'inherit':False,'replication':False,'bypassrls':False,'memberships':0,
                        'connection_limit':-1,'valid_until':None,'config':None}
                    catalog={'role':None,'database':None}
                    pg={'statefulset':{'uid':'sts','resourceVersion':'1'},'pod':{'uid':'pod','resourceVersion':'2',
                        'container_id':'containerd://id','restart_count':0},'pvc':{'name':'pgdata-voice-postgres-0','uid':'pvc','resourceVersion':'3'},
                        'pv':{'name':'pv','uid':'pvuid','resourceVersion':'4'},'service':{'uid':'svc','resourceVersion':'5','cluster_ip':'10.0.0.1'},
                        'endpoints':{'uid':'ep','resourceVersion':'6','pod_ip':'10.0.0.2'},'slice':{'uid':'sl','resourceVersion':'7'}}
                    def create_role(_password,_system):catalog['role']=copy.deepcopy(role)
                    def create_database(_system,_oid):catalog['database']={'oid':29,'owner_oid':13}
                    def patch_secret(_before,password):
                        secret['data']['REALTIME_DATABASE_URL']=base64.b64encode(b'postgres://realtime:'+password+b'@voice-postgres:5432/realtime_db?sslmode=disable').decode()
                        secret['metadata']['resourceVersion']=str(int(secret['metadata']['resourceVersion'])+1)
                    class SyntheticPostgresSnapshot:
                        def begin(self):
                            return '00000003-00000008-1',{'database':'space_db','version':51,'dirty':False,
                                'allow_guests_true':0,'column_default':'false','column_type':'boolean'}
                        def alive(self):return True
                        def close(self):pass
                    scripts={}
                    for part in ('realtime','notification','search','analytics-chat'):
                        text=(workspace/('deploy/templates/nats-'+part+'-bootstrap.yaml')).read_text()
                        first=re.split(r'^---\s*$',text,flags=re.M)[0].replace('__NAMESPACE__','voice-staging').replace('__K_NAMESPACE__','voice-staging')
                        script=compiler.decode_yaml(first.encode())['data']['bootstrap.sh']
                        scripts[part]={'sha256':hashlib.sha256(script.encode()).hexdigest()}
                    for kind in ('Service','Deployment'):compiler_kube.objects.pop((kind,'voice-gameintegration'),None)
                    base_run=compiler_kube.run
                    def compiler_run(args,body=None,timeout=30):
                        if args[:2]==['get','endpointslices'] and 'kubernetes.io/service-name=voice-postgres' in args:
                            return {'items':[{'metadata':{'uid':str(uuid.UUID(int=705))},'endpoints':[{
                                'conditions':{'ready':True},'targetRef':{'kind':'Pod','name':'voice-postgres-0',
                                    'uid':str(uuid.UUID(int=702))}}]}]}
                        if args[:3]==['get','secret','voice-app-secrets'] and args[-1].startswith('go-template='):
                            key=re.search(r'index \.data "([A-Z0-9_]+)"',args[-1])
                            return bool(key and key.group(1) in compiler_kube.get('Secret','voice-app-secrets')['data'])
                        if args[:2]==['get','services,deployments']:
                            return {'apiVersion':'v1','kind':'List','metadata':{},'items':[
                                copy.deepcopy(obj) for (kind,_),obj in compiler_kube.objects.items()
                                if kind in ('Service','Deployment')]}
                        return base_run(args,body=body,timeout=timeout)
                    compiler_kube.run=compiler_run
                    base_get=compiler_kube.get
                    postgres_uid=str(uuid.UUID(int=701));pod_uid=str(uuid.UUID(int=702));claim_uid=str(uuid.UUID(int=703));pv_uid=str(uuid.UUID(int=704))
                    postgres={('namespace','voice-staging'):{'metadata':{'uid':stage.expected['namespace_uid'],'resourceVersion':'1'}},
                        ('statefulset','voice-postgres'):{'metadata':{'uid':postgres_uid,'resourceVersion':'1'},'spec':{'replicas':1}},
                        ('pod','voice-postgres-0'):{'metadata':{'uid':pod_uid,'resourceVersion':'1','namespace':'voice-staging',
                            'labels':{'app':'voice-postgres'},'ownerReferences':[
                            {'kind':'StatefulSet','name':'voice-postgres','uid':postgres_uid,'controller':True}]},
                            'spec':{'volumes':[{'persistentVolumeClaim':{'claimName':'pgdata-voice-postgres-0'}}]},
                            'status':{'phase':'Running','conditions':[{'type':'Ready','status':'True'}],'containerStatuses':[{'name':'postgres','ready':True,
                                'imageID':'docker.io/library/postgres@sha256:'+'a'*64}]}},
                        ('service','voice-postgres'):{'metadata':{'uid':str(uuid.UUID(int=706)),'namespace':'voice-staging','resourceVersion':'1'},
                            'spec':{'type':'ClusterIP','selector':{'app':'voice-postgres'},'ports':[{'port':5432}]}},
                        ('persistentvolumeclaim','pgdata-voice-postgres-0'):{'metadata':{'uid':claim_uid,'resourceVersion':'1'},
                            'status':{'phase':'Bound'},'spec':{'volumeName':'fixture-pg-pv'}},
                        ('persistentvolume','fixture-pg-pv'):{'metadata':{'uid':pv_uid,'resourceVersion':'1'},
                            'spec':{'claimRef':{'uid':claim_uid,'name':'pgdata-voice-postgres-0','namespace':'voice-staging'}}}}
                    deployed=json.loads((code/'nats-known-baseline'/'deployed-contract.json').read_bytes())
                    bootstrap_rows={}
                    for item in deployed['scripts']:
                        name='voice-nats-'+item['part']+'-bootstrap'
                        raw=(source/'scripts/staging/nats-known-baseline/testdata/deployed-bootstrap-20261004'/(item['part']+'-bootstrap.sh')).read_bytes()
                        if len(raw)!=item['bytes'] or hashlib.sha256(raw).hexdigest()!=item['sha256']:
                            raise ValueError('synthetic_nats_fixture_source_changed')
                        bootstrap_rows[name]={'metadata':{'name':name,'uid':item['configMapUID'],
                            'resourceVersion':item['configMapResourceVersion']},'data':{'bootstrap.sh':raw.decode()}}
                    def compiler_get(kind,name):
                        if (kind,name)==('pvc',stage.expected['source_claim']):return copy.deepcopy(stage.final_claim)
                        if (kind,name)==('configmap','voice-nats-generation'):return copy.deepcopy(stage.marker)
                        if kind=='configmap' and name in bootstrap_rows:return copy.deepcopy(bootstrap_rows[name])
                        if kind in ('secret','Secret') and name in nats_rows:return copy.deepcopy(nats_rows[name])
                        if kind in ('secret','Secret') and name=='voice-nats-hub-tls-r20260930a4':
                            return {'metadata':{'uid':str(uuid.UUID(int=707)),'resourceVersion':'1'}}
                        if (kind,name) in postgres:return copy.deepcopy(postgres[(kind,name)])
                        kind={'secret':'Secret','configmap':'ConfigMap','service':'Service','deployment':'Deployment'}.get(kind,kind)
                        if (kind,name)==('Secret','voice-app-secrets'):
                            row=base_get(kind,name);row['data'].update(copy.deepcopy(secret['data']))
                            row['metadata'].update(uid=secret['metadata']['uid'],resourceVersion=secret['metadata']['resourceVersion']);return row
                        if (kind,name)==('ConfigMap','voice-p3-principal-public'):
                            return stage.kube.get('configmap',name)
                        return base_get(kind,name)
                    compiler_kube.get=compiler_get
                    previous_run=compiler_kube.run
                    def database_run(args,body=None,timeout=30):
                        if args[:2]==['exec','voice-postgres-0']:
                            if 'sha256sum' in ' '.join(args):
                                return {'password_sha256':hashlib.sha256(base64.b64decode(secret['data']['POSTGRES_PASSWORD'])).hexdigest()}
                            return {'database':'bot_db','version':9,'dirty':False,'duplicate_token_groups':0}
                        return previous_run(args,body=body,timeout=timeout)
                    compiler_kube.run=database_run
                    compiler_kube.secret_meta=lambda name:{key:compiler_kube.get('Secret',name)['metadata'][key]
                                                            for key in ('uid','resourceVersion')}
                    import root_composition_contract_test
                    root_composition_contract_test.with_gis_server_defaults(compiler_kube)
                    for seam in (patch.object(compiler,'ProducerKube',return_value=compiler_kube),
                        patch.object(root_cli,'Kube',return_value=compiler_kube),
                        patch.object(bootstrap_selection,'ACCOUNT_SHA',nats_account_sha),
                        patch.object(realtime,'engine_snapshot',return_value=(stage.expected['namespace_uid'],pg)),
                        patch.object(realtime,'_system_identifier',return_value='12345'),patch.object(realtime,'_verify_admin'),
                        patch.object(realtime,'_verify_runtime_auth'),patch.object(realtime,'_writer_check'),
                        patch.object(realtime,'_catalog',side_effect=lambda:copy.deepcopy(catalog)),
                        patch.object(realtime,'_create_role',side_effect=create_role),
                        patch.object(realtime,'_create_database',side_effect=create_database),
                        patch.object(realtime,'_patch_secret',side_effect=patch_secret),
                        patch.object(nats_root_plan,'preflight',return_value={'scripts':scripts,'plan':{'actions':[]},'binding':{}}),
                        patch.object(space_safeguards,'Postgres',SyntheticPostgresSnapshot),
                        patch.object(source_authority,'_head'),patch.object(source_authority,'_headers',return_value={}),
                        patch.object(actor_root,'preflight',return_value=None),patch.object(actor_root,'revalidate'),
                        patch.object(bridge_root,'actor_after_realtime_bootstrap',return_value=None)):
                        trace_stack.enter_context(seam)
                    test.assertEqual(bootstrap_selection.select(compiler_kube,prior,'r20260930a4'),prior['secret']['name'])
                    operator=nats_rows[prior['binding']['operator_secret']['name']]
                    saved_account=operator['data']['account.public']
                    operator['data']['account.public']=base64.b64encode(('A'+'C'*55).encode()).decode()
                    with test.assertRaises(bootstrap_selection.Blocked):bootstrap_selection.select(compiler_kube,prior,'r20260930a4')
                    operator['data']['account.public']=saved_account
                    current=nats_rows[prior['secret']['name']];saved_creds=current['data']['bootstrap.creds']
                    current['data']['bootstrap.creds']=base64.b64encode(b'foreign-test-credential').decode()
                    with test.assertRaises(bootstrap_selection.Blocked):bootstrap_selection.select(compiler_kube,prior,'r20260930a4')
                    current['data']['bootstrap.creds']=saved_creds
            with patch.object(prerequisites,'run_prepare',real_prepare),patch.object(prerequisites,'_candidate_images',real_candidate), \
                 patch.object(policy,'validate_source',real_policy_source),patch.object(bridge_root,'INSTALLED',installed), \
                 patch.object(root_cli,'ROOT',root),patch('grp.getgrnam',return_value=__import__('types').SimpleNamespace(gr_gid=0)), \
                 patch.object(actions,'_idle'),patch.object(actions,'_dispatcher',return_value=(17,execution)), \
                 patch.object(source_authority,'capture_source',side_effect=capture), \
                 patch.object(runtime_stage.RolloutStage,'capture',side_effect=capture_stage), \
                 patch.object(stage_runtime.Kube.get,'side_effect',initial_get), \
                 patch.object(actions,'_report',side_effect=report), \
                 patch.object(prerequisites,'submit_effect',wraps=prerequisites.submit_effect) as submit, \
                 patch.object(session,'bind_prerequisites',wraps=session.bind_prerequisites) as bind, \
                 resource_claim.command_scope(session.check):
                if accept_candidate:
                    if trace_after_capture:
                        try:actions._prepare(request)
                        finally:trace_stack.close()
                    else:
                        with test.assertRaises(ReachedPolicy):actions._prepare(request)
                    test.assertEqual(session.check()['phase'],'prerequisites-captured')
                    test.assertEqual(prerequisites._checkpoint(session)['status'],'COMPLETE')
                    test.assertEqual(len(submit.call_args_list),16)
                    bind.assert_called_once()
                else:
                    with test.assertRaisesRegex(bridge_root.Blocked,'realtime_bootstrap_candidate_changed'):
                        actions._prepare(request)
                    submit.assert_not_called();bind.assert_not_called()
                    for call in commands.capture.call_args_list:
                        argv=call.args[0]
                        test.assertFalse(any(verb in argv for verb in ('patch','apply','create','delete','replace'))
                                         and not any(arg.startswith('--dry-run=') for arg in argv))
                    test.assertEqual(session.check()['phase'],'prepare')
                    test.assertFalse((installed/'prerequisites'/session.operation/'checkpoint.json').exists())
        raise Rejected()

    fixture=staging_prerequisites_test.PrepareCompositionTests()
    with patch.object(prerequisites,'run_prepare',side_effect=join):
        with test.assertRaises(Rejected):fixture._cold_prepare()
