"""Actual bridge-generated identities cross CMS and remote artifact custody."""
import copy
import hashlib
import json
import os
from pathlib import Path
import shutil
import tempfile
import unittest
from contextlib import ExitStack, contextmanager, nullcontext
from unittest.mock import Mock, patch
import bridge_root
import encrypted_cut
import github_custody
import github_custody_test as http
import native_store


@contextmanager
def captured_cohort_modules(code,dev_root):
    """Temporarily load captured modules; callers must run sequentially."""
    import importlib.util,sys
    with ExitStack() as stack:
        stack.enter_context(patch.dict(sys.modules,{}))
        for name,loaded in list(sys.modules.items()):
            filename=getattr(loaded,'__file__',None)
            if filename and Path(filename).resolve().is_relative_to(dev_root):
                sys.modules.pop(name,None)
        base=Path(code)/'nats-rollout-preservation'
        for name in ('staging_cohort_preservation','staging_cohort_preservation_runtime',
                     'staging_cohort_preservation_pg','staging_cohort_preservation_auxiliary',
                     'staging_cohort_preservation_auth'):
            spec=importlib.util.spec_from_file_location(name,base/(name+'.py'))
            module=importlib.util.module_from_spec(spec)
            stack.enter_context(patch.dict(sys.modules,{name:module}))
            spec.loader.exec_module(module)
        yield


def canonical_stores():
    import source_plan_test as fixtures
    kube,params=fixtures.SourceTests().canonical_fixture('app-only')
    for (kind,name),claim in list(kube.objects.items()):
        if kind!='PersistentVolumeClaim':continue
        pvname=claim['spec']['volumeName']
        kube.objects[('PersistentVolume',pvname)]={'apiVersion':'v1','kind':'PersistentVolume',
            'metadata':{'name':pvname,'uid':'pv-'+claim['metadata']['uid'],'resourceVersion':'1'},
            'spec':{'claimRef':{'uid':claim['metadata']['uid'],'name':name,'namespace':'voice-staging'}}}
    get=kube.get
    aliases={'pvc':'PersistentVolumeClaim','pv':'PersistentVolume','secret':'Secret',
             'configmap':'ConfigMap','deployment':'Deployment'}
    kube.get=lambda kind,name:get(aliases.get(kind,kind),name)
    policy={key:value for key,value in params.items() if key not in
        ('registry','tag','mode','changed_services','images','enrolled','generation','dataPVC')}
    return kube,params,policy


def align_public_fixture_app_secret(kube,live):
    """Use one synthetic pre-C1 Secret for store and prerequisite reads."""
    key=('secret','voice-app-secrets')
    original=live[key]
    combined=copy.deepcopy(kube.objects[('Secret','voice-app-secrets')])
    combined['metadata']=copy.deepcopy(original['metadata'])
    combined['type']=original['type']
    for name in ('REALTIME_DATABASE_URL','GAME_INTEGRATION_DATABASE_URL','GAME_INTEGRATION_DB_PASSWORD'):
        combined['data'].pop(name,None)
    combined['data'].update(original['data'])
    live[key]=combined
    kube.objects[('Secret','voice-app-secrets')]=copy.deepcopy(combined)


def monitor_pod_inventory(argv,through_hub,hub_released,monitor_pod):
    """Supply the synthetic NATS pod only while its drain is in progress."""
    if through_hub and not hub_released and argv[-3:]==['pods','-o','json']:
        return json.dumps({'items':[monitor_pod]}).encode()
    return None


def postgres_capture_required(argv):
    """Keep unrelated endpoint inventories on the synthetic Kube transport."""
    return ('voice-postgres-0' in argv or argv[-5:]==[
        'endpointslices','-l','kubernetes.io/service-name=voice-postgres','-o','json'])


def p3_failure_locations(error):
    """Return bounded code locations, never exception text or fixture data."""
    allowed={'staging_p3_prerequisites.py','staging_p3_prerequisites_crypto.py',
        'staging_prerequisites.py','resource_claim.py','stage_runtime.py',
        'bridge_composition_test.py','staging_prerequisites_test.py','cohort_hosted_pg.py',
        'gis_existing_update.py','commands.py'}
    kinds={ValueError:'ValueError',KeyError:'KeyError',TypeError:'TypeError',
        FileNotFoundError:'FileNotFoundError',RuntimeError:'RuntimeError',AssertionError:'AssertionError'}
    result=[];seen=set()
    for _ in range(8):
        if error is None or id(error) in seen:break
        seen.add(id(error));frames=[];tb=error.__traceback__
        while tb is not None and len(frames)<32:
            name=Path(tb.tb_frame.f_code.co_filename).name
            if name in allowed:frames.append([name,tb.tb_lineno])
            tb=tb.tb_next
        result.append({'type':kinds.get(type(error),'Other'),'frames':frames})
        error=error.__cause__ or error.__context__
    return result


def report_p3_failure(original):
    def checked(*args,**kwargs):
        try:return original(*args,**kwargs)
        except Exception as error:
            import sys
            print('P3_FIXTURE_FAILURE '+json.dumps(p3_failure_locations(error),separators=(',',':')),
                file=sys.stderr,flush=True)
            raise
    return checked


class CompositionTests(unittest.TestCase):
    def test_p3_failure_diagnostic_preserves_exception_without_sensitive_values(self):
        import contextlib,io
        marker='sensitive-fixture-value'
        def fail():
            try:raise KeyError(marker)
            except KeyError:raise RuntimeError(marker) from None
        output=io.StringIO()
        with contextlib.redirect_stderr(output):
            with self.assertRaisesRegex(RuntimeError,marker):report_p3_failure(fail)()
        text=output.getvalue();self.assertNotIn(marker,text)
        records=json.loads(text.removeprefix('P3_FIXTURE_FAILURE '))
        self.assertEqual([row['type'] for row in records],['RuntimeError','KeyError'])
        self.assertTrue(all(all(row[0]=='bridge_composition_test.py' and isinstance(row[1],int)
            for row in record['frames']) for record in records))

    def test_public_fixture_aligns_app_secret_before_prerequisite_capture(self):
        import source_plan, source_plan_test, staging_gis_material
        kube,params,_=canonical_stores()
        original={'apiVersion':'v1','kind':'Secret','metadata':{'name':'voice-app-secrets',
            'namespace':'voice-staging','uid':'00000000-0000-4000-8000-000000000123','resourceVersion':'1'},
            'type':'Opaque','data':{'POSTGRES_PASSWORD':'cHc='}}
        live={('secret','voice-app-secrets'):original}
        align_public_fixture_app_secret(kube,live)
        saved=live[('secret','voice-app-secrets')]
        self.assertEqual(saved,kube.objects[('Secret','voice-app-secrets')])
        self.assertEqual(saved['metadata'],original['metadata'])
        self.assertEqual(saved['data']['POSTGRES_PASSWORD'],'cHc=')
        self.assertIn('USER_R2_ENDPOINT',saved['data'])
        self.assertNotIn('REALTIME_DATABASE_URL',saved['data'])
        self.assertEqual(staging_gis_material.inspect_app_secret(saved)['credentials'],'absent')
        self.assertIsInstance(source_plan.preservation_projection(source_plan_test.SOURCE,params,
            source_plan_test.decoder,kube),dict)

    def test_aligned_app_secret_reaches_real_gis_preflight_and_patch(self):
        self._exercise_aligned_gis_preflight()

    def test_changed_app_secret_still_refuses_before_prerequisite_patch(self):
        self._exercise_aligned_gis_preflight(drift=True)

    def test_p3_pod_inventory_uses_prerequisite_fixture_after_hub_release(self):
        self._exercise_aligned_gis_preflight(post_hub_monitor=True)

    def test_postgres_endpoint_selector_reaches_hosted_adapter(self):
        import commands, cohort_hosted_pg, staging_prerequisites as prereq
        adapter=object.__new__(cohort_hosted_pg.HostedPostgres)
        adapter.slices={'items':[{'metadata':{'name':'synthetic-postgres-slice'}}]}
        argv=prereq.KUBE_PREFIX+['get','endpointslices','-l',
            'kubernetes.io/service-name=voice-postgres','-o','json']
        self.assertTrue(postgres_capture_required(argv))
        fallback=Mock(side_effect=AssertionError('postgres selection escaped fixture'))
        with patch.object(commands,'authority_guard') as guard:
            result=adapter.capture(argv,fallback=fallback)
        self.assertEqual(json.loads(result),adapter.slices)
        guard.assert_called_once_with(argv);fallback.assert_not_called()

    def _exercise_aligned_gis_preflight(self,*,drift=False,post_hub_monitor=False):
        import staging_prerequisites as prereq, staging_cohort_preservation as cohort
        import staging_prerequisites_test as fixture, staging_gis_material, stage_runtime, commands
        def ready(actions,request,approved,execution,workspace,stage,live,transport,get,card):
            kube,_,_=canonical_stores()
            align_public_fixture_app_secret(kube,live)
            prepared=prereq.prepare_inputs(actions,request,approved,execution,workspace)
            qualified={'prerequisite_inputs_sha256':prepared['inputs_sha256']}
            plan={'actual_before':stage.old_images,'candidate':{'commit':actions.tt_session.context['commit'],
                'images':actions.tt_session.context['images']}}
            with patch.object(cohort,'validate_plan'),patch.object(cohort,'load_qualified',return_value=qualified), \
                 patch.object(cohort,'capture_stage',return_value=stage),patch.object(cohort,'capture_final_stage',return_value=stage), \
                 patch.object(cohort,'load_auxiliary_successor',return_value=None),patch.object(cohort,'revalidate_paused'):
                actions.tt_session.bind_cohort_preservation(plan,stage.old_images)
                if drift:
                    changed=copy.deepcopy(live[('secret','voice-app-secrets')])
                    changed['metadata']['uid']='00000000-0000-4000-8000-000000000999'
                    def changed_get(kind,name):
                        return copy.deepcopy(changed) if (kind,name)==('secret','voice-app-secrets') else get(kind,name)
                    with patch.object(stage_runtime.Kube,'get',side_effect=changed_get):
                        with self.assertRaisesRegex(staging_gis_material.MaterialError,'staging_gis_preflight_invalid'):
                            prereq.run_prepare(actions,request,approved,execution,workspace,preservation_ref=qualified)
                elif post_hub_monitor:
                    monitor={'metadata':{'name':'synthetic-nats-monitor'}}; seen=[]; slices=[]
                    argv=prereq.KUBE_PREFIX+['get','pods','-o','json']
                    self.assertEqual(json.loads(monitor_pod_inventory(argv,True,False,monitor)),{'items':[monitor]})
                    self.assertTrue(postgres_capture_required(prereq.KUBE_PREFIX+['get','endpointslices','-l',
                        'kubernetes.io/service-name=voice-postgres','-o','json']))
                    self.assertTrue(postgres_capture_required(prereq.KUBE_PREFIX+['exec','voice-postgres-0']))
                    postgres_capture=Mock(side_effect=FileNotFoundError('synthetic host has no k3s'))
                    def captured(args,*,body=b'',**kwargs):
                        result=monitor_pod_inventory(args,True,True,monitor)
                        if args[-3:]==['pods','-o','json']:seen.append(args)
                        if args[-3:]==['endpointslices','-o','json']:slices.append(args)
                        if postgres_capture_required(args):return postgres_capture(args)
                        return result if result is not None else transport(args,body=body,**kwargs)
                    with patch.object(commands,'capture',side_effect=captured),patch.object(stage_runtime,'capture',side_effect=captured):
                        prereq.run_prepare(actions,request,approved,execution,workspace,preservation_ref=qualified)
                    self.assertGreaterEqual(len(seen),2);self.assertGreaterEqual(len(slices),2)
                    postgres_capture.assert_not_called()
                else:prereq.run_prepare(actions,request,approved,execution,workspace,preservation_ref=qualified)
            if drift:self.assertNotIn('GAME_INTEGRATION_DATABASE_URL',live[('secret','voice-app-secrets')]['data'])
            else:self.assertIn('GAME_INTEGRATION_DATABASE_URL',live[('secret','voice-app-secrets')]['data'])
        fixture.PrepareCompositionTests()._cold_prepare(on_ready=ready)

    def test_captured_module_mapping_is_restored_after_import_failure(self):
        import sys
        from types import SimpleNamespace
        import staging_cohort_preservation as original
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);code=root/'code'/'nats-rollout-preservation';code.mkdir(parents=True)
            dev=root/'dev';dev.mkdir()
            (code/'staging_cohort_preservation.py').write_text('fixture = True\n')
            (code/'staging_cohort_preservation_runtime.py').write_text("raise RuntimeError('synthetic import failure')\n")
            sentinel=SimpleNamespace(__file__=str(dev/'sentinel.py'))
            with patch.dict(sys.modules,{'fixture_dev_sentinel':sentinel}):
                with self.assertRaisesRegex(RuntimeError,'synthetic import failure'):
                    with captured_cohort_modules(root/'code',dev):pass
                self.assertIs(sys.modules['fixture_dev_sentinel'],sentinel)
                self.assertIs(sys.modules['staging_cohort_preservation'],original)

    def test_public_prepare_binds_stores_before_enrollment_admission(self):
        """Synthetic source/Kube, real pre-cut and store projection; no P2 proof."""
        self._exercise_public_precut(enrolled=False)

    def test_public_prepare_selects_enrollment_before_broker_observation(self):
        """The broker response remains an external fixture boundary."""
        self._exercise_public_precut(enrolled=True)

    def test_public_prepare_validates_nats_plan_before_captured_inputs(self):
        """NATS transport is synthetic; bootstrap and planner checks execute."""
        self._exercise_public_precut(enrolled=True,through_nats=True)

    def test_public_prepare_journals_fenced_marker_before_workload_cut(self):
        """Stop the synthetic API at the first workload mutation."""
        self._exercise_public_precut(enrolled=True,through_nats=True,through_fence=True)

    def test_public_prepare_journals_client_cut_before_pod_drain(self):
        """Stop the synthetic API at the first pod inventory."""
        self._exercise_public_precut(enrolled=True,through_nats=True,through_fence=True,through_workload=True)

    def test_public_prepare_journals_hub_shutdown_before_storage_recheck(self):
        """Simulate Kubernetes pod termination; storage recheck is the boundary."""
        self._exercise_public_precut(enrolled=True,through_nats=True,through_fence=True,through_workload=True,through_hub=True)

    def test_public_prepare_rechecks_storage_before_postgres_capture(self):
        """A private container store is real; Kubernetes and NATS are fixtures."""
        self._exercise_public_precut(enrolled=True,through_nats=True,through_fence=True,
            through_workload=True,through_hub=True,through_storage=True)

    def _exercise_public_precut(self,*,enrolled,through_nats=False,through_fence=False,through_workload=False,through_hub=False,through_storage=False,postgres=None,qualify=False,final_prepare=False,final_admission=False):
        import commands, compiler, guard, staging_cohort_preservation as cohort
        import staging_prerequisites_test as prerequisites
        import source_plan_test as fixtures
        actual_capture=commands.capture
        class ReachedBroker(Exception):
            pass
        class ReachedInput(Exception):
            pass
        class ReachedFence(Exception):
            pass
        def run(actions,request,approved,execution,workspace,stage,live,transport,get,card):
            from docker_runtime import NATS_IMAGE
            kube,params,policy=canonical_stores()
            if qualify:align_public_fixture_app_secret(kube,live)
            installed=workspace.parents[1]
            (installed/'policy.json').write_text(json.dumps(policy))
            stage.kube=kube;stage.service={}
            params['dataPVC']=stage.expected['source_claim']
            stage.marker['data'].update(generation=params['generation'],dataPVC=params['dataPVC'])
            if enrolled:
                import base64, bootstrap_selection, bootstrap_selection_test, nats_contract_plan as plan
                _,previous,rows,installation=bootstrap_selection_test.owned_fixture()
                self.assertEqual(actions.tt_session.installation,installation)
                old_rows=[{'stream':stream,'durable':config['durable_name'],'config':copy.deepcopy(config)}
                    for stream,config in plan.V3_DECLARATIONS]
                previous['auth_proof'].update(normalized_consumer=old_rows[0]['config'],
                    normalized_consumer_sha256=plan.digest(old_rows[0]['config']),normalized_consumers=old_rows,
                    normalized_consumers_sha256=plan.digest(old_rows))
                current=copy.deepcopy(previous)
                secret=copy.deepcopy(rows[current['secret']['name']]);secret['metadata'].update(name='voice-nats-bootstrap-contract-'+'b'*24,uid='v4-uid')
                secret['data']['bootstrap.creds']=base64.b64encode(b'new-music-private-creds').decode()
                rows[secret['metadata']['name']]=secret
                current.update(schema='voice-nats-bootstrap-enrollment-scoped-v4',previous_enrollment=previous,
                    previous_enrollment_sha256=plan.digest(previous),
                    secret={k:secret['metadata'][k] for k in ('name','uid','resourceVersion')},
                    credential_sha256=bootstrap_selection.credential_hash(secret))
                current['binding']['previous_enrollment_sha256']=plan.digest(previous)
                stream,config=plan.DECLARATIONS[-1]
                normalized=old_rows+[{'stream':stream,'durable':config['durable_name'],'config':copy.deepcopy(config)}]
                current['auth_proof'].update(schema='voice-bootstrap-auth-v4',normalized_consumers=normalized,
                    normalized_consumers_sha256=plan.digest(normalized))
                cohort._write(guard.ROOT/'bootstrap-enrollment.json',previous)
                cohort._write(guard.ROOT/'bootstrap-enrollment-v4.json',current)
                for name,row in rows.items():kube.objects[('Secret',name)]=row
                def secret_meta(name):
                    if name.startswith('voice-nats-hub-tls-'):
                        return {'uid':'tls-uid','resourceVersion':'1'}
                    meta=kube.objects[('Secret',name)]['metadata']
                    return {key:meta[key] for key in ('uid','resourceVersion')}
                kube.secret_meta=secret_meta
                stage.marker['metadata'].update(uid='marker-uid',resourceVersion='4')
                stage.marker['data']['generation']=installation.generation
            kube.objects[('ConfigMap','voice-nats-generation')]=copy.deepcopy(stage.marker)
            kube.objects[('Deployment','voice-user')]['metadata']['generation']=1
            stage.snapshots={'voice-user':copy.deepcopy(kube.objects[('Deployment','voice-user')])}
            stage.old_images['voice-user/nats-leaf']=NATS_IMAGE
            card['observed']['images']=copy.deepcopy(stage.old_images)
            if through_hub:
                if through_storage:
                    import os,uuid
                    if not Path('/.dockerenv').exists() and os.environ.get('RUNNER_ENVIRONMENT')!='github-hosted':
                        raise RuntimeError('disposable_container_or_hosted_runner_required')
                    claim_uid=str(uuid.uuid4());pv_uid=str(uuid.uuid4())
                    stage.expected.update(source_claim_uid=claim_uid,source_pv_uid=pv_uid)
                    stage.final_claim['metadata']['uid']=claim_uid
                    stage.final_pv['metadata']['uid']=pv_uid
                    stage.final_pv['spec']['claimRef']['uid']=claim_uid
                    storage=Path('/var/lib/rancher/k3s/storage')/('pvc-'+claim_uid+'_voice-staging_'+stage.expected['source_claim'])
                    storage.parent.mkdir(parents=True,exist_ok=True)
                    storage.mkdir(mode=0o700);os.chown(storage,65532,10000);storage.chmod(0o2770)
                    self.addCleanup(storage.rmdir)
                    stage.final_path=storage;stage.final_pv['spec']['local']['path']=str(storage)
                    live[('pvc',stage.expected['source_claim'])]=copy.deepcopy(stage.final_claim)
                    live[('pv',stage.final_claim['spec']['volumeName'])]=copy.deepcopy(stage.final_pv)
                hub={'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':bridge_root.HUB,
                    'namespace':'voice-staging','uid':'00000000-0000-4000-8000-000000000010','resourceVersion':'1','generation':1},
                    'spec':{'replicas':1,'template':{'spec':{'containers':[{'name':'nats','image':NATS_IMAGE}]}}}}
                if through_storage:
                    spec=hub['spec']['template']['spec']
                    spec['securityContext']={'fsGroup':10000,'fsGroupChangePolicy':'OnRootMismatch'}
                    spec['volumes']=[{'name':'jsdata','persistentVolumeClaim':{'claimName':stage.expected['source_claim']}}]
                    spec['containers'][0].update(securityContext={'runAsUser':10000,'runAsGroup':10000,'runAsNonRoot':True},
                        volumeMounts=[{'name':'jsdata','mountPath':'/data'}])
                stage.snapshots[bridge_root.HUB]=copy.deepcopy(hub)
                stage.expected['deployment_uids'][bridge_root.HUB]=hub['metadata']['uid']
                stage.expected['marker_uid']=stage.marker['metadata']['uid']
                stage.old_images[bridge_root.HUB+'/nats']=NATS_IMAGE
                card['observed']['images']=copy.deepcopy(stage.old_images)
                live[('deployment',bridge_root.HUB)]=copy.deepcopy(hub)
            stage.original_snapshots=copy.deepcopy(stage.snapshots)
            stage.expected['deployment_uids']['voice-user']=stage.snapshots['voice-user']['metadata']['uid']
            live[('configmap','voice-nats-generation')]=copy.deepcopy(stage.marker)
            live[('deployment','voice-user')]=copy.deepcopy(stage.snapshots['voice-user'])
            if through_hub:live[('service','voice-nats')]=copy.deepcopy(stage.service)
            if enrolled and not through_nats:
                old_get=kube.get
                def broker_boundary(kind,name):
                    if (kind,name)==('deployment',bridge_root.HUB):raise ReachedBroker()
                    return old_get(kind,name)
                kube.get=broker_boundary
            original_run=kube.run
            pod={'spec':{'volumes':[{'persistentVolumeClaim':{'claimName':stage.expected['source_claim']}}]}}
            kube.run=lambda args,body=None,timeout=30: ({'items':[pod]} if args==['get','pods','-o','json']
                else original_run(args,body=body,timeout=timeout))
            if through_nats:
                import bootstrap_auth, nats_a3_roster_test, rollout_census_test
                import cohort_fake_nats
                stream_rows=nats_a3_roster_test.Planner().inputs(voice=10)[0]
                detail=[]
                for name,value in stream_rows.items():
                    state={**value['state'],'bytes':100,'consumer_count':0}
                    detail.append({'name':name,'created':rollout_census_test.CREATED,
                        'config':value['config'],'state':state,'consumer_detail':[]})
                tree={'streams':len(detail),'consumers':0,'messages':sum(x['state']['messages'] for x in detail),
                    'bytes':sum(x['state']['bytes'] for x in detail),'total':1,
                    'account_details':[{'id':'A'+'A'*55,'stream_detail':detail}]}
                hub=copy.deepcopy(stage.snapshots[bridge_root.HUB]) if through_hub else {'metadata':{'name':bridge_root.HUB,'uid':'hub'}}
                hub_uid=hub['metadata']['uid'];rs_uid='00000000-0000-4000-8000-000000000011' if through_hub else 'rs'
                replica={'metadata':{'uid':rs_uid,'ownerReferences':[{'kind':'Deployment','uid':hub_uid,'controller':True}]}}
                monitor_pod={'metadata':{'name':bridge_root.HUB+'-fixture','namespace':'voice-staging',
                    'uid':'00000000-0000-4000-8000-000000000012','resourceVersion':'1',
                    'ownerReferences':[{'kind':'ReplicaSet','name':'rs','uid':rs_uid,'controller':True}]},
                    'spec':{'nodeName':'pmdebook','containers':[{'name':'nats','image':NATS_IMAGE}] if through_hub else [],
                        'volumes':[{'persistentVolumeClaim':{'claimName':stage.expected['source_claim']}}]},
                    'status':{'phase':'Running','conditions':[{'type':'Ready','status':'True'}],
                        'containerStatuses':[{'name':'nats','ready':True,'imageID':NATS_IMAGE,
                            'containerID':'containerd://'+'c'*64,'restartCount':0,
                            'state':{'running':{'startedAt':'2026-10-29T00:00:00Z'}}}] if through_hub else []}}
                if through_hub:
                    live[('replicaset','rs')]=copy.deepcopy(replica)
                    live[('pod',monitor_pod['metadata']['name'])]=copy.deepcopy(monitor_pod)
                    pod=copy.deepcopy(monitor_pod)
                old_get=kube.get
                def get_nats(kind,name):
                    if (kind,name)==('deployment',bridge_root.HUB):return copy.deepcopy(hub)
                    if (kind,name)==('replicaset','rs'):return copy.deepcopy(replica)
                    if not through_fence and kind=='configmap' and name.startswith('voice-nats-') and name!='voice-nats-generation':raise ReachedInput()
                    return old_get(kind,name)
                kube.get=get_nats
                old_run=kube.run
                def run_nats(args,body=None,timeout=30):
                    if args==['get','pods','-l','app='+bridge_root.HUB,'-o','json']:return {'items':[monitor_pod]}
                    if args[:2]==['get','--raw']:return copy.deepcopy(tree)
                    return old_run(args,body=body,timeout=timeout)
                kube.run=run_nats
                if through_fence:
                    import stage_runtime, commands
                    from cohort_fake_kube import FakeKube
                    contract=json.loads((actions.code/'nats-known-baseline'/'deployed-contract.json').read_bytes())
                    scripts=Path(__file__).parent/'fixtures'/'cohort-captured-scripts'
                    for entry in contract['scripts']:
                        raw=(scripts/(entry['part']+'.sh')).read_bytes()
                        self.assertEqual((len(raw),hashlib.sha256(raw).hexdigest()),(entry['bytes'],entry['sha256']))
                        name='voice-nats-'+entry['part']+'-bootstrap'
                        row={'metadata':{'name':name,'uid':entry['configMapUID'],
                            'resourceVersion':entry['configMapResourceVersion']},'data':{'bootstrap.sh':raw.decode()}}
                        kube.objects[('ConfigMap',name)]=row;live[('configmap',name)]=copy.deepcopy(row)
                    for name,row in rows.items():live[('secret',name)]=copy.deepcopy(row)
                    live[('secret','voice-nats-hub-tls-'+installation.generation)]={'metadata':{'uid':'tls-uid','resourceVersion':'1'}}
                    objects={('configmap','voice-nats-generation'):stage.marker,
                        ('deployment','voice-user'):stage.snapshots['voice-user']}
                    if through_hub:
                        objects[('deployment',bridge_root.HUB)]=stage.snapshots[bridge_root.HUB]
                        objects[('pod',monitor_pod['metadata']['name'])]=monitor_pod
                    fake=FakeKube(objects);flags={'hub_released':False}
                    def capture_fence(argv,*,body=b'',**kwargs):
                        if through_storage and argv[:2]==['/usr/bin/docker','--host=unix:///var/run/docker.sock']:
                            if postgres is None or ((cohort._base(actions.tt_session)/'local-restore.json').exists() and not qualify):
                                commands.authority_guard(argv)
                                raise ReachedFence()
                            return postgres.capture(argv,body=body,fallback=actual_capture,**kwargs)
                        if 'patch' in argv:
                            index=argv.index('patch');kind=argv[index+1]
                            if qualify and kind=='secret' and argv[index+2]=='voice-app-secrets':
                                result=transport(argv,body=body,**kwargs)
                                kube.objects[('Secret','voice-app-secrets')]=copy.deepcopy(live[('secret','voice-app-secrets')])
                                return result
                            if kind=='deployment' and not through_workload:raise ReachedFence()
                            result=fake.capture_patch(argv,body=body,**kwargs)
                            name=argv[index+2];live[(kind,name)]=fake.get(kind,name)
                            if through_hub and kind=='deployment' and name==bridge_root.HUB:
                                retained=fake.objects[('pod',monitor_pod['metadata']['name'])]
                                retained['metadata']['deletionTimestamp']='2026-10-29T00:00:01Z'
                                retained['status']['containerStatuses'][0]['state']={'terminated':{'exitCode':0,
                                    'startedAt':'2026-10-29T00:00:00Z','finishedAt':'2026-10-29T00:00:01Z'}}
                                live[('pod',monitor_pod['metadata']['name'])]=copy.deepcopy(retained)
                            if through_hub and kind=='pod' and not live[(kind,name)]['metadata'].get('finalizers'):
                                flags['hub_released']=True
                            return result
                        if through_workload and 'pods,replicasets' in argv:
                            if through_hub:return json.dumps({'items':[]}).encode()
                            raise ReachedFence()
                        if through_hub and 'hpa,jobs,deployments' in argv:
                            return json.dumps({'items':[]}).encode()
                        if through_hub and 'version' in argv:
                            commands.authority_guard(argv)
                            return json.dumps({'serverVersion':{'major':'1','minor':'28'}}).encode()
                        if through_hub and 'exec' in argv and 'wget' in argv:
                            commands.authority_guard(argv)
                            return json.dumps(tree).encode()
                        inventory=monitor_pod_inventory(argv,through_hub,flags['hub_released'],
                            live.get(('pod',monitor_pod['metadata']['name'])))
                        if inventory is not None:return inventory
                        if postgres is not None and postgres_capture_required(argv):
                            return postgres.capture(argv,body=body,fallback=actual_capture,**kwargs)
                        if 'jsonpath-as-json=' in ' '.join(argv):
                            name=argv[argv.index('secret')+1];meta=live[('secret',name)]['metadata']
                            return json.dumps([meta['uid'],meta['resourceVersion']]).encode()
                        return transport(argv,body=body,**kwargs)
            saved=workspace.with_name(workspace.name+'-source-fixture');workspace.rename(saved)
            (installed/'source-authority'/(actions.tt_session.operation+'.json')).unlink()
            execution={**execution,'head_sha':actions.tt_session.context['commit']}
            request={**request,'action':'prepare','nonce':actions.tt_session.operation+'a'*52,
                'source_sha':actions.tt_session.context['commit'],'resource_context':actions.tt_session.context,'token':'fixture'}
            def capture_source(_token,_run,_sha,destination):
                shutil.copytree(saved,destination,dirs_exist_ok=True)
                return copy.deepcopy(approved)
            with patch.object(bridge_root,'INSTALLED',installed),patch.object(compiler,'decode_yaml',side_effect=fixtures.decoder),\
                 patch.object(actions,'_idle'),patch.object(actions,'_dispatcher',return_value=(17,execution)),\
                 patch.object(bridge_root.source_authority,'capture_source',side_effect=capture_source),ExitStack() as nats_patches:
                if through_nats:
                    # Source-owned dynamic modules must be loaded from the
                    # captured tree even if unittest discovery imported their
                    # working-tree equivalents before this test started.
                    dev_root=Path(__file__).resolve().parents[2]/'dev'
                    nats_patches.enter_context(captured_cohort_modules(actions.code,dev_root))
                    nats_patches.enter_context(patch.object(bootstrap_auth,'DockerRuntime',
                        side_effect=lambda base,operation:cohort_fake_nats.ScratchBroker(base,operation,'A'+'A'*55)))
                    nats_patches.enter_context(patch.object(bootstrap_auth,'Actor',cohort_fake_nats.ScratchActor))
                    if through_fence:
                        nats_patches.enter_context(patch.object(commands,'capture',side_effect=capture_fence))
                        nats_patches.enter_context(patch.object(stage_runtime,'capture',side_effect=capture_fence))
                        if through_hub:
                            def get_after_hub(kind,name):
                                if flags['hub_released'] and not through_storage and kind=='pvc':raise ReachedFence()
                                if postgres is not None and postgres.has_get(kind,name):return postgres.get(kind,name)
                                if through_storage and postgres is None and flags['hub_released'] and kind=='statefulset' and name=='voice-postgres':raise ReachedFence()
                                if through_storage and flags['hub_released'] and kind.lower()=='secret' and name=='voice-app-secrets':
                                    commands.authority_guard(prerequisites.prereq.KUBE_PREFIX+['get',kind,name,'-o','json'])
                                    return copy.deepcopy(live[('secret',name)] if qualify else kube.objects[('Secret',name)])
                                if through_storage and flags['hub_released'] and kind=='Deployment' and name in stage.snapshots:
                                    commands.authority_guard(prerequisites.prereq.KUBE_PREFIX+['get',kind,name,'-o','json'])
                                    return copy.deepcopy(live[('deployment',name)])
                                try:return get(kind,name)
                                except KeyError:
                                    return kube.get(kind,name)
                            nats_patches.enter_context(patch.object(stage_runtime.Kube,'get',side_effect=get_after_hub))
                        if postgres is not None:
                            import subprocess
                            nats_patches.enter_context(patch.object(subprocess,'Popen',side_effect=postgres.popen))
                        if qualify:
                            import importlib,subprocess
                            import staging_p3_prerequisites as p3
                            nats_patches.enter_context(patch.object(p3,'verify_complete',
                                side_effect=report_p3_failure(p3.verify_complete)))
                            captured=importlib.import_module('staging_cohort_preservation')
                            auxiliary=importlib.import_module('staging_cohort_preservation_auxiliary')
                            import github_custody
                            recovery=installed/'recovery';recovery.mkdir(mode=0o700)
                            subprocess.run(['/usr/bin/openssl','req','-x509','-newkey','rsa:2048','-nodes',
                                '-keyout',str(recovery/'recovery-key.pem'),'-out',str(recovery/'recovery-cert.pem'),
                                '-subj','/CN=isolated-cohort-test','-days','1'],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
                            for name in ('recovery-key.pem','recovery-cert.pem'):(recovery/name).chmod(0o600)
                            def synthetic_native(session):
                                base=captured._runtime_base(session);payload=b'synthetic-native-nats-boundary'
                                captured.private._private_write(base/'rollout-before.tar',payload)
                                manifest={'archive_sha256':hashlib.sha256(payload).hexdigest()}
                                captured._write(base/'rollout-before-manifest.json',manifest)
                                cut={'manifest':manifest,'manifest_sha256':captured._file_identity(base/'rollout-before-manifest.json')['sha256'],
                                    'census_sha256':'f'*64}
                                captured._write(captured._base(session)/'native-cut.json',cut)
                                return cut
                            def remote(_token,binding,_path,artifact_id):
                                return {'schema':'voice-nats-custody-v1','destination':'github:Poryadok/VoiceRoot',
                                    **{key:binding[key] for key in ('operation','challenge','run_id','head_sha','cipher_sha256','cipher_bytes')},
                                    'artifact_id':artifact_id,'verified':True}
                            nats_patches.enter_context(patch.object(captured,'_capture_native',side_effect=synthetic_native))
                            nats_patches.enter_context(patch.object(captured,'_native_readback',return_value={'synthetic_nats_boundary':True}))
                            # The tar bytes above are a named synthetic NATS boundary.
                            nats_patches.enter_context(patch('native_store.verify_archive',return_value=True))
                            nats_patches.enter_context(patch.object(auxiliary,'resume',return_value=None))
                            nats_patches.enter_context(patch.object(github_custody,'verify_artifact',side_effect=remote))
                            prepared=actions.execute(request)
                            self.assertIsInstance(prepared,dict)
                            authorize={'action':'authorize','operation':actions.tt_session.operation,'token':'fixture',
                                'artifact_id':42,'resource_context':actions.tt_session.context}
                            if final_admission:
                                # Call the actual issuer and validate its typed
                                # capability, then stop before any migration.
                                issue=captured.issue_final_admission;issued=[]
                                def issue_then_stop(session,base,state,request):
                                    altered=copy.deepcopy(state)
                                    old=altered['target']['manifest_sha256']
                                    altered['target']['manifest_sha256']=('0' if old[0]!='0' else '1')+old[1:]
                                    with self.assertRaisesRegex(captured.Blocked,'cohort_final_target_changed'):
                                        issue(session,base,altered,request)
                                    self.assertFalse((captured._base(session)/'authorizing.json').exists())
                                    handle=issue(session,base,state,request)
                                    self.assertIs(type(handle),captured.QualifiedFinalAdmission)
                                    captured._issued(handle,session,base,state)
                                    with self.assertRaises(FileExistsError):
                                        issue(session,base,state,request)
                                    captured._issued(handle,session,base,state)
                                    issued.append(handle)
                                    raise ReachedFence()
                                with patch.object(captured,'issue_final_admission',side_effect=issue_then_stop) as issuer:
                                    with self.assertRaises(ReachedFence):actions.execute(authorize)
                                issuer.assert_called_once();self.assertEqual(len(issued),1)
                                self.assertTrue((captured._base(actions.tt_session)/'authorizing.json').exists())
                            if final_prepare or final_admission:
                                # The only stop is after the actual continuation,
                                # prerequisites, C1 and preserved final prepare.
                                # NATS/GitHub and Kubernetes remain fixtures.
                                if not final_admission:
                                    with patch.object(bridge_root.transaction,'authorize',side_effect=ReachedFence) as stopped:
                                        with self.assertRaises(ReachedFence):actions.execute(authorize)
                                    stopped.assert_called_once()
                                    self.assertIs(stopped.call_args.kwargs['resource_session'],actions.tt_session)
                                    self.assertEqual(stopped.call_args.kwargs['cohort_request'],authorize)
                                final=captured._read(captured._runtime_base(actions.tt_session)/'checkpoint.json',limit=128<<20)
                                self.assertEqual(final['operation'],actions.tt_session.operation)
                                self.assertEqual(final['resource_fence'],actions.tt_session.binding())
                                self.assertEqual(final['realtime_bootstrap']['schema'],'voice-staging-realtime-bootstrap-receipt-v2')
                                self.assertEqual(final['target']['realtime_bootstrap_sha256'],captured.digest(final['realtime_bootstrap']))
                                self.assertEqual(final['target']['prerequisites_sha256'],captured.digest(final['prerequisites']))
                                self.assertEqual(final['target']['cohort_preservation_sha256'],captured.digest(final['cohort_preservation']))
                            else:
                                with patch.object(actions,'_prepare',side_effect=ReachedFence):
                                    with self.assertRaises(ReachedFence):actions.execute(authorize)
                                self.assertEqual(captured.load_qualified(actions.tt_session)['operation'],actions.tt_session.operation)
                                self.assertEqual(captured.require_continuation(actions.tt_session,authorize)['qualified_sha256'],
                                    captured.digest(captured.load_qualified(actions.tt_session)))
                            base=captured._base(actions.tt_session)
                            self.assertTrue((base/'precut.json').exists());self.assertTrue((base/'paused.json').exists())
                            self.assertTrue(captured._read(base/'stores.json')['claims'])
                            self.assertEqual(captured._read(base/'local-restore.json')['status'],'COMPLETE')
                            self.assertEqual(captured._read(base/'local-restore.json')['proof']['cleanup'],'verified')
                            self.assertEqual(fake.get('deployment','voice-user')['spec']['replicas'],0)
                            self.assertEqual(fake.get('deployment',bridge_root.HUB)['spec']['replicas'],0)
                            self.assertTrue(flags['hub_released'])
                            for suffix in ('retain','release'):
                                self.assertEqual(captured._read(base/'effects'/('cut-pod-'+monitor_pod['metadata']['name']+'-'+suffix+'.json'))['status'],'DONE')
                            self.assertEqual(actions.tt_session.check()['status'],'active')
                            return
                        with self.assertRaises(ReachedFence):actions.execute(request)
                    else:
                        with self.assertRaises(ReachedInput):actions.execute(request)
                elif enrolled:
                    with self.assertRaises(ReachedBroker):actions.execute(request)
                else:
                    with self.assertRaisesRegex(cohort.Blocked,'enrolled_bootstrap_selection_refused'):
                        actions.execute(request)
            base=cohort._base(actions.tt_session)
            self.assertTrue((base/'precut.json').exists())
            stores=cohort._read(base/'stores.json')
            self.assertEqual(stores['projection']['schema'],'voice-staging-store-projection-v1')
            self.assertTrue(stores['claims'])
            if through_nats:
                self.assertIn('plan',cohort._read(base/'preflight.json')['nats'])
            if through_fence:
                self.assertEqual(fake.get('configmap','voice-nats-generation')['data']['phase'],'rollout-capturing')
                record=cohort._read(base/'effects'/'cut-configmap-voice-nats-generation.json')
                self.assertEqual(record['status'],'DONE')
                if through_workload:
                    self.assertEqual(fake.get('deployment','voice-user')['spec']['replicas'],0)
                    self.assertEqual(cohort._read(base/'effects'/'cut-deployment-voice-user.json')['status'],'DONE')
                if through_hub:
                    self.assertTrue(flags['hub_released'])
                    self.assertEqual(fake.get('deployment',bridge_root.HUB)['spec']['replicas'],0)
                    for suffix in ('retain','release'):
                        self.assertEqual(cohort._read(base/'effects'/('cut-pod-'+monitor_pod['metadata']['name']+'-'+suffix+'.json'))['status'],'DONE')
                if through_storage:
                    self.assertTrue((base/'paused.json').exists())
                    checkpoint=cohort._read(cohort._runtime_base(actions.tt_session)/'checkpoint.json')
                    self.assertEqual((checkpoint['phase'],checkpoint['status']),('COHORT_BACKUP','BLOCKED'))
                    if postgres is not None:
                        restore=cohort._read(base/'local-restore.json')
                        self.assertEqual(restore['status'],'COMPLETE')
                        self.assertEqual(restore['proof']['cleanup'],'verified')
                self.assertEqual(actions.tt_session.journal.read()['status'],'uncertain')
            else:self.assertEqual(actions.tt_session.check()['phase'],'prepare')
        overrides=None
        if through_nats:
            source=Path(__file__).parent/'testdata'/'a3-roster-source'
            overrides={f'deploy/templates/nats-{part}-bootstrap.yaml':source/f'deploy/templates/nats-{part}-bootstrap.yaml'
                for part in ('realtime','analytics-chat','search','notification')}
        workspace_context=None
        if qualify:
            from contextlib import nullcontext
            retained=Path(os.environ['VOICE_COHORT_RETAINED_ROOT'])
            cohort.private._private_directory(retained)
            workspace_context=nullcontext(tempfile.mkdtemp(prefix='public-cohort-',dir=retained))
        prerequisites.PrepareCompositionTests()._cold_prepare(on_ready=run,context_commit=fixtures.TAG,
            full_source=True,strict_helpers=True,source_overrides=overrides,workspace_context=workspace_context,
            fixture_complete_candidate=qualify)

    def test_cut_store_capture_uses_canonical_source_and_live_pvc_identity(self):
        """Source rendering and store binding are real; Kubernetes is synthetic."""
        import commands, compiler, guard, resource_claim, resource_claim_test
        import source_authority, source_plan, source_plan_test as fixtures
        import staging_cohort_preservation as cohort
        from types import SimpleNamespace
        kube,params,policy=canonical_stores()
        stage=SimpleNamespace(kube=kube,snapshots={},marker={'data':{'generation':params['generation'],'dataPVC':params['dataPVC']}})
        context=resource_claim_test.context();context['commit']=fixtures.TAG
        card=resource_claim_test.card(context)
        with tempfile.TemporaryDirectory(dir='/root') as directory:
            root=Path(directory);(root/'installed').mkdir(mode=0o700)
            with patch('grp.getgrnam',return_value=SimpleNamespace(gr_gid=0)),patch.object(guard,'ROOT',root),\
                 patch.object(compiler,'decode_yaml',side_effect=fixtures.decoder):
                session=resource_claim.Session(context,'a'*12,'b'*64,lambda:copy.deepcopy(card),resource_claim.RootJournal(root))
                session.admit(start=True)
                cohort._write(root/'installed'/'policy.json',policy)
                request={'mode':'app-only','changed_services':[]}
                parameters={**policy,'registry':source_authority.REGISTRY,
                    'tag':context['commit'],'mode':'app-only','changed_services':[],'images':{},'enrolled':[],
                    'generation':params['generation'],'dataPVC':params['dataPVC']}
                projection=source_plan.preservation_projection(fixtures.SOURCE,parameters,fixtures.decoder,kube)
                approved={'source_sha':context['commit'],'source_files':{row['path']:row['sha256'] for row in projection['sources']}}
                with resource_claim.command_scope(session.check):
                    stores=cohort._capture_stores(session,stage,request,approved,fixtures.SOURCE)
                    self.assertEqual(stores['projection'],projection)
                    self.assertTrue(stores['claims'])
                    claim=next(iter(stores['claims'].values()))
                    kube.objects[('PersistentVolume',claim['pv']['metadata']['name'])]['spec']['claimRef']['uid']='foreign'
                    with self.assertRaisesRegex(cohort.Blocked,'cohort_store_storage_changed'):
                        cohort._capture_stores(session,stage,request,approved,fixtures.SOURCE)

    def test_cut_captures_real_prerequisite_inputs_before_kubernetes_observation(self):
        """The Kubernetes read is a fixture boundary, not P2 qualification."""
        import staging_cohort_preservation as cohort
        import staging_prerequisites_test as fixtures
        class ReachedKube(Exception):
            pass
        def run(actions,request,approved,execution,workspace,stage,live,transport,get,card):
            with patch.object(stage.kube,'run',side_effect=ReachedKube):
                with self.assertRaises(ReachedKube):
                    cohort._start_cut(actions,request,approved,execution,workspace)
            session=actions.tt_session
            self.assertEqual(session.check()['phase'],'prepare')
            prepared=cohort.private._read_private(cohort.private._base(session)/'prepared.json')
            self.assertTrue(prepared)
        fixtures.PrepareCompositionTests()._cold_prepare(on_ready=run)

    def test_cohort_cut_uses_fenced_resource_version_cas(self):
        """Exercise the real cut journal against an in-memory Kubernetes API.

        This is a Kubernetes boundary fixture, not a P2 qualification proof.
        """
        import commands, guard, resource_claim, staging_cohort_preservation as cohort
        import staging_cohort_preservation_test as fixtures
        from cohort_fake_kube import FakeKube
        from types import SimpleNamespace
        before={'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':'voice-user',
            'namespace':'voice-staging','uid':'user','resourceVersion':'3','generation':1},
            'spec':{'replicas':1,'template':{'spec':{'containers':[{'name':'user','image':'old'}]}}}}
        with tempfile.TemporaryDirectory(dir='/root') as directory:
            root=Path(directory);(root/'installed').mkdir(mode=0o700)
            with patch('grp.getgrnam',return_value=SimpleNamespace(gr_gid=0)),\
                 patch.object(guard,'ROOT',root),patch.object(cohort,'_source'):
                session,_=fixtures.CutEffectTests().fixture(root,before)
                kube=FakeKube({('deployment','voice-user'):before})
                with resource_claim.command_scope(session.check),patch.object(commands,'capture',side_effect=kube.capture_patch):
                    result=cohort.cut_cas(session,'deployment',before,[{'op':'replace','path':'/spec/replicas','value':0}])
                    self.assertEqual(result,kube.get('deployment','voice-user'))
                    with self.assertRaisesRegex(Exception,'cohort_reconciliation_required'):
                        cohort.cut_cas(session,'deployment',before,[{'op':'replace','path':'/spec/replicas','value':0}])
                self.assertEqual(kube.calls,[('deployment','voice-user',[
                    {'op':'test','path':'/metadata/uid','value':'user'},
                    {'op':'test','path':'/metadata/resourceVersion','value':'3'},
                    {'op':'replace','path':'/spec/replicas','value':0}])])

    def test_actor_drift_after_offnode_readback_prevents_transaction_authorization(self):
        import actor_root
        from preserved_upload_actions_test import bind_copy
        base=Path(tempfile.mkdtemp());self.addCleanup(lambda:shutil.rmtree(base))
        actions=object.__new__(bridge_root.Actions);actions.code=Path('/captured-code');actions.binding={'fixed':'hash'}
        state={'operation':'123456abcdef','cipher_binding':{},'service_actor_services':['social'],
            'service_actor_authority':{'binding':'root-produced'},'target':{'mode':'images-only'},
            'cut':{'manifest':{'archive_sha256':'a'*64},'manifest_sha256':'b'*64}}
        actions._installation=bind_copy(base,state)
        order=[]
        with patch.object(actions,'state',return_value=(base,state)),\
             patch.object(bridge_root.github_custody,'verify_artifact',side_effect=lambda *a:order.append('full-offnode-readback') or {'verified':True}),\
             patch.object(actions,'_verify_decrypted_restore',side_effect=lambda *a:order.append('decrypted-restore')),\
             patch.object(bridge_root,'save'),patch.object(bridge_root,'Kube'),\
             patch.object(bridge_root.transaction,'reconstruct'),\
             patch.object(actor_root,'revalidate',side_effect=bridge_root.Blocked('existing_service_actor_authority_changed')) as actor,\
             patch.object(bridge_root.transaction,'authorize') as authorize:
            with self.assertRaisesRegex(bridge_root.Blocked,'existing_service_actor_authority_changed'):
                actions.execute({'action':'authorize','operation':'123456abcdef','token':'private-token','artifact_id':1})
        self.assertEqual(order,['full-offnode-readback','decrypted-restore'])
        actor.assert_called_once();authorize.assert_not_called()

    def test_real_prepare_source_failure_saves_boundary_before_build(self):
        import bridge
        with tempfile.TemporaryDirectory() as directory:
            base=Path(directory);installed=base/'installed';(installed/'sources').mkdir(parents=True)
            journal=base/'journal';journal.mkdir()
            actions=object.__new__(bridge_root.Actions)
            request={'action':'prepare','source_sha':'a'*40,'run_id':17,'mode':'images-only','changed_services':['user'],'token':'private-token','nonce':'b'*64}
            execution={'head_sha':request['source_sha']}
            error=bridge_root.source_authority.SourceError('private response text')
            with patch.object(bridge_root,'INSTALLED',installed),patch.object(actions,'_idle'),patch.object(actions,'_dispatcher',return_value=(17,execution)),patch.object(bridge_root.source_authority,'capture_source',side_effect=error),patch.object(bridge_root.RolloutStage,'capture') as stage:
                with self.assertRaises(bridge_root.source_authority.SourceError):
                    bridge.dispatch(journal,request,actions.execute,actions.recover,execute_observed=actions.execute_observed)
                stage.assert_not_called()
            saved=json.loads((journal/(request['nonce']+'.json')).read_text())
            self.assertEqual(saved['prepare_stage'],{'name':'source-capture','status':'STARTED'})
            self.assertEqual(saved['prepare_error'],'source_authority_rejected')
            self.assertTrue((installed/'sources'/('b'*12)).is_dir())
            self.assertFalse((installed/'sources'/('b'*12+'-build')).exists())
            self.assertIsNone(actions._observer)
            self.assertNotIn('private',json.dumps(saved))

    def test_real_rollback_prepare_accepts_verified_index_child_and_rejects_changed_content(self):
        from types import SimpleNamespace
        class ReachedContract(Exception):pass
        repo=bridge_root.source_authority.REGISTRY+'/user@sha256:'
        actual=repo+'a'*64;expected=repo+'b'*64
        previous={'input_target':{'mode':'images-only','changed_services':['user']},
                  'target':{'images':{'voice-user/user':expected}},
                  'context':{'old_images':{'voice-user/user':repo+'c'*64}}}
        stage=SimpleNamespace(snapshots={'voice-user':{'spec':{'template':{'spec':{
            'containers':[{'name':'user','image':repo+'old-tag'}]}}}}},
            old_images={'voice-user/user':actual})
        approved={'source_sha':'d'*40,'images':{'user':expected}}
        for matches in (True,False):
            with self.subTest(matches=matches),tempfile.TemporaryDirectory() as directory:
                installed=Path(directory);(installed/'sources').mkdir()
                policy=installed/'policy.json';policy.write_text(json.dumps({'s3_signing_endpoint':'https://storage.example'}));policy.chmod(0o600)
                actions=object.__new__(bridge_root.Actions);actions.code=installed;actions.binding={}
                with patch.object(bridge_root,'INSTALLED',installed),patch.object(actions,'_idle'),\
                    patch.object(actions,'_dispatcher',return_value=(123,{'head_sha':'d'*40})),\
                    patch.object(bridge_root.source_authority,'capture_source',return_value=approved),\
                    patch.object(bridge_root,'Kube',return_value=SimpleNamespace(get=lambda *a:{'data':{'generation':'g','dataPVC':'pvc'}})),\
                    patch('gis_existing_update.load_authority',return_value=None),\
                    patch.object(bridge_root.RolloutStage,'capture',return_value=stage),\
                    patch.object(bridge_root.source_authority,'same_image_content',return_value=matches) as identity,\
                    patch.object(bridge_root,'public_read',side_effect=ReachedContract) as contract:
                    with self.assertRaises(ReachedContract if matches else bridge_root.Blocked):
                        actions._prepare({'nonce':'a'*64,'token':'private','mode':'images-only','changed_services':['user']},previous)
                    identity.assert_called_once()
                    self.assertEqual(identity.call_args.args[:2],(actual,expected))
                    self.assertEqual(contract.call_count,1 if matches else 0)

    def test_cohort_preflight_rejects_malformed_v4_enrollment_before_effects(self):
        """Use the real pre-cut guard; Kubernetes and TT authority are fixtures."""
        import actor_root, commands, compiler, nats_root_plan, resource_claim
        import resource_claim_test, staging_prerequisites
        from types import SimpleNamespace

        context=resource_claim_test.context()
        session=resource_claim.Session(context,'a'*12,'b'*64,
            lambda:resource_claim_test.card(context),resource_claim_test.MemoryJournal())
        session.admit(start=True)
        actions=object.__new__(bridge_root.Actions);actions.tt_session=session
        actions._installation=session.installation;actions.code=Path('/synthetic');actions.binding={}
        workspace=bridge_root.INSTALLED/'sources'/session.operation
        stage=SimpleNamespace(operation=session.operation,kube=Mock(),snapshots={'voice-user':{}},expected={})
        def read(path):
            if path.name=='active-contract.json':raise FileNotFoundError()
            if path.name=='bootstrap-enrollment-v4.json':return invalid
            return {'fixture':'prior-enrollment'}
        with resource_claim.command_scope(session.check),patch.object(bridge_root,'private_json',side_effect=read),\
             patch.object(nats_root_plan,'preflight') as nats,patch.object(actor_root,'preflight') as actor,\
             patch.object(compiler,'main') as build,patch.object(bridge_root.root_cli,'prepare_preserved_value') as prepare,\
             patch.object(staging_prerequisites,'run_prepare') as coordinator:
            for mode,invalid in ((mode,invalid) for mode in ('full','app-only') for invalid in (None,[],'invalid')):
                with self.subTest(mode=mode,enrollment=invalid):
                    with self.assertRaisesRegex(bridge_root.Blocked,'enrolled_bootstrap_selection_refused'):
                        bridge_root.capture_cohort_authorities(actions,{'mode':mode},
                            {'source_sha':context['commit']},workspace,stage,{'consumer_pairs':[]})
                    nats.assert_not_called();actor.assert_not_called();build.assert_not_called()
                    prepare.assert_not_called();coordinator.assert_not_called();stage.kube.get.assert_not_called()
                    self.assertEqual(session.check()['phase'],'prepare')

    def _compiler_catalog_stage(self,mode):
        """Complete compiler inventory; Kubernetes observations are synthetic."""
        import source_plan_test as fixtures, runtime_stage
        from types import SimpleNamespace
        kube,p=fixtures.SourceTests().canonical_fixture(mode)
        p.update(generation='r20260930a4',dataPVC='voice-nats-jsdata-d202610040049430b')
        targets={'voice-gateway',*('voice-'+n for n in runtime_stage.LEAVES),'voice-web','voice-admin','voice-developer-portal','voice-gameintegration','voice-p3-principal-jwks'}
        snapshots={name:copy.deepcopy(obj) for (kind,name),obj in kube.objects.items() if kind=='Deployment' and name in targets}
        renderer_repo=bridge_root.source_authority.REGISTRY+'/nats-hub-config-renderer'
        snapshots[bridge_root.HUB]={'apiVersion':'apps/v1','kind':'Deployment',
            'metadata':{'name':bridge_root.HUB,'namespace':'voice-staging','uid':'hub','resourceVersion':'9'},
            'spec':{'replicas':1,'template':{'spec':{'containers':[{'name':'nats','image':'nats:2.12.12-alpine@sha256:2ca98656a279b2d88cfdf2b8c3f0d5d7f3941ae9dc2ab12ebaa92d83e0f4ccdb'}],
                'initContainers':[{'name':'nats-config-renderer','image':renderer_repo+':captured-old'}],
                'volumes':[{'name':'jsdata','persistentVolumeClaim':{'claimName':p['dataPVC']}}]}}}}
        old={name+'/'+c['name']:'example.invalid/'+c['name']+'@sha256:'+'a'*64 for name,obj in snapshots.items() if name!=bridge_root.HUB for c in obj['spec']['template']['spec'].get('initContainers',[])+obj['spec']['template']['spec']['containers']}
        old[bridge_root.HUB+'/nats-config-renderer']=renderer_repo+'@sha256:'+'b'*64
        old[bridge_root.HUB+'/nats']='docker.io/library/nats@sha256:'+'c'*64
        old['voice-p3-principal-jwks/jwks']='docker.io/library/nginx@sha256:'+'e'*64
        stage=SimpleNamespace(snapshots=snapshots,original_snapshots=copy.deepcopy(snapshots),old_images=old,
            expected={'generation':p['generation'],'source_claim':p['dataPVC'],'source_claim_uid':'pvc','source_pv_uid':'pv'})
        return kube,p,targets,renderer_repo,old,stage

    def _exercise_catalog_compiler(self,mode,policy_case='valid',deep=None):
        """Real compiler and session; P2, NATS, actor, renderer, Realtime and Kube are fixtures."""
        import staging_prerequisites_test as fixtures, staging_prerequisites as prereq
        import staging_cohort_preservation as cohort
        import source_plan_test as source, actor_root, realtime_bootstrap, resource_claim_test, renderer_root, renderer_transition
        class ReachedCompiler(Exception):pass
        def run(actions,request,approved,execution,workspace,stage,live,transport,get,card):
            request=dict(request,mode=mode,changed_services=[])
            if deep is not None and deep.is_c2:request.pop('gis_first_install')
            kube,parameters,targets,renderer_repo,old,catalog=self._compiler_catalog_stage(mode)
            if deep is not None and deep.is_c2:
                import uuid
                for index,(identity,row) in enumerate(kube.objects.items(),1):
                    row['metadata']['uid']=str(uuid.UUID(int=index))
                    if identity[0]=='Deployment' and identity[1] in catalog.snapshots:
                        catalog.snapshots[identity[1]]['metadata']['uid']=row['metadata']['uid']
            stage.snapshots=copy.deepcopy(catalog.snapshots)
            stage.snapshots.pop('voice-p3-principal-jwks')
            if deep is None or not deep.is_c2:stage.snapshots.pop('voice-gameintegration')
            old.pop('voice-p3-principal-jwks/jwks')
            for key in list(old):
                if key.startswith('voice-gameintegration/') and (deep is None or not deep.is_c2):del old[key]
            for name,row in stage.snapshots.items():
                row['metadata'].setdefault('generation',1)
                live[('deployment',name)]=copy.deepcopy(row)
            if deep is not None and deep.is_c2:
                for identity,row in kube.objects.items():live[identity]=copy.deepcopy(row)
            stage.original_snapshots=copy.deepcopy(stage.snapshots)
            stage.old_images=copy.deepcopy(old)
            stage.marker['data'].update(generation=parameters['generation'],dataPVC=parameters['dataPVC'])
            if deep is not None:stage.marker['data']['previousGeneration']='old'
            live[('configmap','voice-nats-generation')]=copy.deepcopy(stage.marker)
            stage.expected.update(generation=parameters['generation'],source_claim=parameters['dataPVC'])
            stage.final_claim['metadata']['name']=parameters['dataPVC']
            live[('pvc',parameters['dataPVC'])]=copy.deepcopy(stage.final_claim)
            stage.final_path=Path('/var/lib/rancher/k3s/storage/pvc-'+stage.final_claim['metadata']['uid']+'_voice-staging_'+parameters['dataPVC'])
            stage.final_pv['spec']['local']['path']=str(stage.final_path)
            live[('pv',stage.final_claim['spec']['volumeName'])]=copy.deepcopy(stage.final_pv)
            stage.expected['deployment_uids']={name:row['metadata']['uid'] for name,row in stage.snapshots.items()}
            card['observed']['images']=copy.deepcopy(old)
            self.assertEqual(actions.tt_session.check()['phase'],'prepare')
            approved['images']={name.removeprefix('voice-'):bridge_root.source_authority.REGISTRY+'/'+name.removeprefix('voice-')+'@sha256:'+'a'*64
                for name in targets if name!='voice-p3-principal-jwks'}
            approved['images']['nats-hub-config-renderer']=renderer_repo+'@sha256:'+'d'*64
            if deep is not None:deep.approve(approved)
            if deep is not None and deep.is_c2:
                optional={'s3_signing_endpoint','gateway_host','storage_host','livekit_host','web_host','admin_host',
                    'developer_portal_host','gateway_tls_secret','storage_tls_secret','image_pull_secret',
                    'apply_observability','minio_image','minio_mc_image','minio_storage_class','minio_storage_size',
                    'web_origin','release_policy'}
                existing_policy={key:copy.deepcopy(value) for key,value in parameters.items() if key in optional}
                existing_policy['release_policy']['source_sha']=approved['source_sha']
                bridge_root.save(workspace.parents[1]/'policy.json',existing_policy)
            source_record=workspace.parents[1]/'source-authority'/(actions.tt_session.operation+'.json')
            source_value=json.loads(source_record.read_bytes());source_value['approved_sha256']=prereq.digest(approved)
            bridge_root.save(source_record,source_value)
            def renderer_proof(kube,current,source_sha,image,**kwargs):
                descriptor=renderer_transition.plan(current.snapshots[bridge_root.HUB],old[bridge_root.HUB+'/nats-config-renderer'],image,
                    source_sha,renderer_root.storage(current))
                return {'descriptor':descriptor,'broker_image':old[bridge_root.HUB+'/nats'],
                    'output':{'sha256':'e'*64,'bytes':10,'verified':True},'input_binding':{}}
            import gis_existing_update
            boundary=patch.object(gis_existing_update,'load_authority',return_value=None) if deep is not None and deep.is_c2 else nullcontext()
            with patch.object(renderer_root,'prove',side_effect=renderer_proof) as renderer,patch.object(renderer_root,'revalidate'),boundary:
                prepared=prereq.prepare_inputs(actions,request,approved,execution,workspace)
            if mode=='full':renderer.assert_called_once()
            else:renderer.assert_not_called()
            session=actions.tt_session
            qualified={'prerequisite_inputs_sha256':prepared['inputs_sha256']}
            contract={'scripts':[]}
            for part in ('realtime','notification','search','analytics-chat'):
                raw=(source.SOURCE/('deploy/templates/nats-'+part+'-bootstrap.yaml')).read_bytes().split(b'---')[0]
                contract['scripts'].append({'part':part,'sha256':hashlib.sha256(source.decoder(raw)['data']['bootstrap.sh'].encode()).hexdigest()})
            if deep is not None:contract=deep.contract(contract,actions.code)
            p2_plan={'actual_before':stage.old_images,'candidate':{'commit':session.context['commit'],'images':session.context['images']}}
            if deep is not None:
                p2_plan=deep.plan(p2_plan,stage,source,request,prepared,contract)
            # P2 qualification is a synthetic boundary; Session checks and
            # the prerequisite producer still run against the real journal.
            with ExitStack() as stack:
                stack.enter_context(patch.object(cohort,'load_qualified',return_value=qualified))
                stack.enter_context(patch.object(cohort,'capture_stage',return_value=stage))
                stack.enter_context(patch.object(cohort,'load_auxiliary_successor',return_value=None))
                stack.enter_context(patch.object(cohort,'revalidate_paused'))
                with patch.object(cohort,'validate_plan'):
                    session.bind_cohort_preservation(p2_plan,stage.old_images)
                def final_stage(_session):
                    receipt=prereq._checkpoint(session)['receipt']
                    stage.snapshots['voice-p3-principal-jwks']=copy.deepcopy(live[('deployment','voice-p3-principal-jwks')])
                    stage.original_snapshots=copy.deepcopy(stage.snapshots)
                    stage.expected['deployment_uids']['voice-p3-principal-jwks']=stage.snapshots['voice-p3-principal-jwks']['metadata']['uid']
                    stage.old_images=receipt['actual_after']
                    return stage
                loaded={'request':{'mode':request['mode'],'changed_services':request['changed_services'],
                    'gis_first_install':request.get('gis_first_install') is True},
                    'approved':approved,'execution':execution,'workspace':str(workspace)}
                public={'action':'authorize','operation':session.operation,'token':'fixture','artifact_id':42,'resource_context':session.context}
                source_request=dict(request,nonce=session.operation+'a'*52,resource_context=session.context)
                real_read=cohort._read
                def read(path,*args,**kwargs):
                    if path==cohort._request_record(session):return source_request
                    if path.name=='stores.json':return {'app_before':{'metadata':{}}}
                    if path.name=='archive-record.json':return {'catalog_ref':{}}
                    if path.name=='qualified.json':return qualified
                    return real_read(path,*args,**kwargs)
                stack.enter_context(patch.object(bridge_root,'INSTALLED',workspace.parents[1]))
                stack.enter_context(patch.object(cohort,'qualify_offnode'))
                stack.enter_context(patch.object(cohort,'load_prepared',return_value=loaded))
                stack.enter_context(patch.object(cohort,'load_plan',return_value={'namespace_uid':'namespace','prerequisite_inputs_sha256':prepared['inputs_sha256']}))
                stack.enter_context(patch.object(cohort,'_offnode_records',return_value={'artifact_id':42}))
                stack.enter_context(patch.object(cohort,'_read',side_effect=read))
                stack.enter_context(patch.object(cohort,'capture_final_stage',side_effect=final_stage))
                contract_path=actions.code/'nats-known-baseline'/'deployed-contract.json'
                if deep is None:
                    contract_path.parent.mkdir(parents=True,exist_ok=True);contract_path.write_text(json.dumps(contract));contract_path.chmod(0o600)
                policy={key:value for key,value in parameters.items() if key in {'s3_signing_endpoint','gateway_host','storage_host','livekit_host','web_host','admin_host','developer_portal_host','gateway_tls_secret','storage_tls_secret','image_pull_secret','apply_observability','minio_image','minio_mc_image','minio_storage_class','minio_storage_size','web_origin','release_policy'}}
                policy['release_policy']['source_sha']=approved['source_sha']
                if policy_case=='null':policy['release_policy']=None
                elif policy_case=='empty':policy['release_policy']={}
                elif policy_case=='wrong-source':policy['release_policy']['source_sha']='f'*40
                elif policy_case=='missing':del policy['release_policy']
                private_read=bridge_root.private_json
                stack.enter_context(patch.object(bridge_root,'private_json',side_effect=lambda path:policy if path==workspace.parents[1]/'policy.json' else private_read(path)))
                stack.enter_context(patch.object(cohort,'load_preflight',return_value={'enrollment':{},'nats':{'scripts':{},'plan':{},'binding':{}},'contract':contract,'services':[],'actor':None}))
                stack.enter_context(patch('nats_root_plan.preflight',return_value={'scripts':{},'plan':{},'binding':{}}))
                stack.enter_context(patch.object(bridge_root,'_cohort_story_composition',return_value={}))
                stack.enter_context(patch.object(actor_root,'preflight',return_value=None))
                stack.enter_context(patch.object(cohort,'_base',return_value=workspace))
                stack.callback(cohort._continuations.pop,session,None)
                stack.enter_context(patch.object(cohort,'_app_successor'))
                stack.enter_context(patch.object(cohort,'record_actor_successor'))
                stack.enter_context(patch.object(renderer_root,'revalidate'))
                bootstrap={'schema':'voice-staging-realtime-bootstrap-existing-v1','fixture':'synthetic'}
                if deep is not None:
                    bootstrap=deep.bootstrap(session,stage,stack,prereq,cohort,actions,workspace,kube,approved,execution,contract,p2_plan)
                stack.enter_context(patch.object(realtime_bootstrap,'run_prepare',
                    **({'side_effect':deep.run_bootstrap} if deep is not None else {'return_value':bootstrap})))
                if deep is not None and deep.is_c2:
                    import realtime_bootstrap_existing
                    stack.enter_context(patch.object(realtime_bootstrap,'load_existing',side_effect=realtime_bootstrap_existing.load_existing))
                else:stack.enter_context(patch.object(realtime_bootstrap,'load_existing',return_value=bootstrap))
                compiler_phase=[False]
                def report(name,status):
                    if (name,status)==('policy-validation','STARTED'):
                        compiler_phase[0]=True
                    if (name,status)==('target-build','STARTED'):
                        if mode=='full':
                            import stage_runtime, staging_cohort_preservation_pg as pg
                            engine={'pod':{'uid':'pod','container_id':'container','restart_count':0},
                                'pvc':{'uid':'pvc'},'pv':{'uid':'pv'}}
                            old_engine={'namespace_uid':'namespace','pod_uid':'pod','pvc_uid':'pvc','pv_uid':'pv',
                                'container_id':'container','restart_count':0,'system_identifier':'12345'}
                            # Synthetic P2 records and external PostgreSQL observations;
                            # guard_postgres itself and Session.check remain real.
                            stack.enter_context(patch.object(cohort,'_record',return_value={'engine':old_engine,'server_start':'fixture'}))
                            stack.enter_context(patch.object(realtime_bootstrap,'engine_snapshot',return_value=('namespace',engine)))
                            def postgres_identity(args,body=None,timeout=30):
                                self.assertEqual(args,pg.IDENTITY_ARGS)
                                session.check(pg.owner.PREFIX+args)
                                return {'system_identifier':'12345','server_start':'fixture'}
                            stack.enter_context(patch.object(stage_runtime.Kube,'_run',side_effect=postgres_identity))
                stack.enter_context(patch.object(actions,'_report',side_effect=report))
                def capture(args,body=b'',**kwargs):
                    import commands
                    if not compiler_phase[0]:return transport(args,body=body,**kwargs)
                    commands.authority_guard(args)
                    if args[1]=='create':return json.dumps(source.decoder(body)).encode()
                    if args[3:5]==['get','services,deployments']:
                        return json.dumps({'apiVersion':'v1','kind':'List','metadata':{},'items':list(stage.snapshots.values())}).encode()
                    if args[3:5]==['get','--raw']:return json.dumps(kube.run(args[3:])).encode()
                    return json.dumps(kube.run(args[3:],None if not body else json.loads(body))).encode() if args[3] in ('apply','exec') else json.dumps(kube.get(args[4],args[5])).encode()
                stack.enter_context(patch.object(bridge_root.compiler,'capture',side_effect=capture))
                stack.enter_context(patch('commands.capture',side_effect=capture))
                stack.enter_context(patch.object(bridge_root.source_authority,'_head'))
                def compiled(value,*args,**kwargs):
                    import guard_test, datetime as dt, resource_claim, resource_claim_test
                    self.assertEqual(len(value['nonnats']['checks']),16)
                    self.assertEqual(value['target']['mode'],mode)
                    self.assertEqual(any(key.startswith(bridge_root.HUB+'/') for key in value['target']['images']),mode=='full')
                    self.assertFalse(any(row['metadata']['name']==bridge_root.HUB for row in value['manifests']))
                    receipt=bootstrap if deep is None else deep.receipt
                    self.assertEqual(kwargs['realtime_bootstrap_receipt'],receipt)
                    self.assertEqual(value['target']['realtime_bootstrap_sha256'],bridge_root.transaction.canonical(receipt))
                    if deep is not None:
                        return deep.prepare(value,args,kwargs,session,stage,kube,workspace,qualified,contract,public,stack)
                    authorization=guard_test.receipt()
                    authorization.update(schema='nats-rollout-preservation-v2',installation_sha256=session.installation.sha256,target=copy.deepcopy(value['target']),
                        backup={'archive_sha256':'1'*64,'manifest_sha256':'2'*64,'census_sha256':'3'*64,'off_node_verified':True},
                        expires_at=(dt.datetime.now(dt.timezone.utc)+dt.timedelta(hours=1)).isoformat())
                    # The downstream transaction normally computes this after
                    # server-side GIS normalization; this sink does not run it.
                    authorization['target']['gis_first_install_normalized_sha256']='4'*64
                    authorization['target']['input_template_hashes']=dict(value['target']['template_hashes'])
                    fence_context=copy.deepcopy(session.context);fence_context['images']=copy.deepcopy(authorization['target']['images'])
                    authorization['resource_fence']={'schema':'voice-staging-resource-fence-v2',
                        'context':fence_context,'operation':authorization['operation'],'installation_sha256':session.installation.sha256,
                        'helper_sha256':'b'*64,'target_sha256':resource_claim.digest(authorization['target'])}
                    bridge_root.guard.validate(authorization,value['target']['registry'],value['target']['tag'],mode,[],workspace,installation=session.installation)
                    raise ReachedCompiler()
                prepare=stack.enter_context(patch.object(bridge_root.root_cli,'prepare_preserved_value',side_effect=compiled))
                build=stack.enter_context(patch.object(bridge_root.compiler,'main',wraps=bridge_root.compiler.main))
                if policy_case in ('null','empty','wrong-source'):
                    with self.assertRaisesRegex(bridge_root.Blocked,'bridge_release_policy_invalid'):actions.execute(public)
                    build.assert_not_called();prepare.assert_not_called()
                elif policy_case=='missing':
                    with self.assertRaisesRegex(ValueError,'release_policy_input_shape'):actions.execute(public)
                    prepare.assert_not_called()
                else:
                    if deep is None:
                        with self.assertRaises(ReachedCompiler):actions.execute(public)
                    else:
                        deep.result=actions.execute(public)
                    build.assert_called_once();prepare.assert_called_once()
                if deep is None:
                    self.assertEqual(prereq._checkpoint(session)['status'],'COMPLETE')
                    self.assertEqual(session.check()['phase'],'prerequisites-captured')
                else:
                    self.assertEqual(session.check()['phase'],'prepare')
        context=resource_claim_test.context()
        context['images']={'voice-gameintegration/gameintegration':bridge_root.source_authority.REGISTRY+'/gameintegration@sha256:'+'a'*64}
        if deep is not None and deep.is_c2:context['images']=deep.candidate(self,source,mode)
        if mode=='full':
            context['images'].update({bridge_root.HUB+'/nats':'nats:2.12.12-alpine@sha256:2ca98656a279b2d88cfdf2b8c3f0d5d7f3941ae9dc2ab12ebaa92d83e0f4ccdb',
                bridge_root.HUB+'/nats-config-renderer':bridge_root.source_authority.REGISTRY+'/nats-hub-config-renderer@sha256:'+'d'*64})
        with patch.object(resource_claim_test,'context',return_value=context):
            with ExitStack() as setup:
                overrides=None
                if deep is not None:
                    import yaml
                    directory=Path(setup.enter_context(tempfile.TemporaryDirectory(dir='/root')))
                    source_root=Path(__file__).parent/'testdata'/'a3-roster-source'
                    scripts=Path(__file__).parent/'fixtures'/'cohort-captured-scripts'
                    overrides={}
                    for part in ('realtime','notification','search','analytics-chat'):
                        relative=f'deploy/templates/nats-{part}-bootstrap.yaml'
                        documents=list(yaml.safe_load_all((source_root/relative).read_text()))
                        documents[0]['data']['bootstrap.sh']=(scripts/(part+'.sh')).read_text()
                        output=directory/(part+'.yaml');output.write_text(yaml.safe_dump_all(documents,sort_keys=False))
                        overrides[relative]=output
                fixtures.PrepareCompositionTests()._cold_prepare(on_ready=run,full_source=policy_case=='valid',
                    strict_helpers=deep is not None,context_commit=source.TAG,source_overrides=overrides,
                    **({'fixture_operation':'b'*12,'fixture_candidate_images':context['images']}
                       if deep is not None and deep.is_c2 else {}))

    def test_root_prepare_full_and_app_only_actual_hub_catalog_reaches_canonical_compiler(self):
        for mode in ('full','app-only'):
            for policy_case in ('null','empty','wrong-source','missing','valid'):
                with self.subTest(mode=mode,policy=policy_case):
                    self._exercise_catalog_compiler(mode,policy_case)

    def test_actual_root_cipher_binding_twelve_hex_operation_reaches_remote_readback(self):
        root=Path('/var/lib/voice-nats-preservation');root.mkdir(exist_ok=True,mode=0o750)
        base=Path(tempfile.mkdtemp(prefix='composition-',dir=root));self.addCleanup(lambda:shutil.rmtree(base))
        installed=base/'installed';installed.mkdir(mode=0o750);keys=installed/'recovery';keys.mkdir(mode=0o700)
        encrypted_cut.initialize_recovery_key(keys)
        source=base/'native';source.mkdir();(source/'record.blk').write_bytes(b'known synthetic record with pending ACK')
        manifest=native_store.archive_closed_store(source,base/'rollout-before.tar')
        (base/'rollout-before-manifest.json').write_text(json.dumps(manifest));(base/'rollout-before-manifest.json').chmod(0o600)
        import resource_claim
        from resource_claim_test import MemoryJournal,card,context,scoped_target
        nonce='a'*64;c=context();helper={'nats-rollout-preservation/resource_claim.py':'b'*64}
        session=resource_claim.Session(c,nonce[:12],resource_claim.digest(helper),lambda:card(c),MemoryJournal())
        session.admit(start=True)
        target=scoped_target(c);target.update(changed_services=['web'],mode='images-only')
        session.journal.row['target_sha256']=resource_claim.digest(target)
        state={'schema':'nats-rollout-root-v2','installation_sha256':session.installation.sha256,
            'resource_fence':session.binding(),'code_capture':helper,
            'operation':nonce[:12],'phase':'AWAITING_OFF_NODE','status':'WAITING','fence_status':'VERIFIED',
            'target':target,
            'cut':{'manifest':manifest,'manifest_sha256':hashlib.sha256((base/'rollout-before-manifest.json').read_bytes()).hexdigest(),'census_sha256':'d'*64}}
        actions=object.__new__(bridge_root.Actions);actions._installation=session.installation
        with patch.object(bridge_root,'INSTALLED',installed),patch('grp.getgrnam',return_value=type('G',(),{'gr_gid':65532})()):
            exported=actions._encrypt(base,state,{'dispatcher_run_id':123,'head_sha':'c'*40})
        self.assertEqual(state['cipher_binding']['operation'],nonce[:12]);self.assertEqual(len(state['cipher_binding']['challenge']),32)
        zipped=http.archive((base/'rollout-backup.cms').read_bytes())
        from datetime import datetime,timezone,timedelta
        metadata={'id':99,'name':exported['artifact_name'],'url':'https://api.github.com/repos/Poryadok/VoiceRoot/actions/artifacts/99',
            'workflow_run':{'id':123,'head_sha':'c'*40},'expired':False,'created_at':state['cipher_binding']['created_at'],
            'expires_at':(datetime.now(timezone.utc)+timedelta(days=1)).isoformat(),'size_in_bytes':len(zipped)}
        calls=[]
        def request(url,headers,limit,deadline):
            calls.append(url)
            if url.endswith('/99'):return http.Reply(200,json.dumps(metadata).encode())
            if url.endswith('/zip'):return http.Reply(302,headers={'Location':'https://productionresultssa1.blob.core.windows.net/artifact'})
            return http.Reply(200,zipped)
        with patch.object(github_custody,'_request',request):receipt=github_custody.verify_artifact('private-token',state['cipher_binding'],base/'rollout-backup.cms',99)
        self.assertTrue(receipt['verified']);self.assertEqual(receipt['operation'],state['operation']);self.assertEqual(len(calls),3)
        for field,value in [('operation','a'*32),('challenge','b'*12),('challenge','z'*32)]:
            with self.subTest(field=field,value=value):
                forged={**state['cipher_binding'],field:value}
                with patch.object(github_custody,'_request') as remote,self.assertRaises(github_custody.CustodyError):
                    github_custody.verify_artifact('private-token',forged,base/'rollout-backup.cms',99)
                remote.assert_not_called()
