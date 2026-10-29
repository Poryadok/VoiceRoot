import copy
import base64
import hashlib
import json
import tempfile
import uuid
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest.mock import Mock,patch

import staging_space_native as native


class InspectSourceTest(unittest.TestCase):
    def test_running_guard_never_calls_full_space_identity_loader(self):
        plan={'fixture':'protected'}
        with patch.object(native,'_live_base',return_value=Path('/unused')), \
             patch.object(native.owner,'_read',return_value=plan), \
             patch.object(native,'_live_running_context',return_value=plan) as running, \
             patch.object(native,'_live_context',side_effect=AssertionError('space connection')) as full:
            self.assertEqual(native._live_checked_running_plan(SimpleNamespace()),plan)
            running.assert_called_once();full.assert_not_called()

    def test_live_documents_keep_private_evidence_on_owned_volume(self):
        session=SimpleNamespace(operation='a'*12,context={'operation_id':'stg-run-17'})
        tool={'immutable_image':'ghcr.io/poryadok/voiceroot/space-native-installer@sha256:'+'c'*64,
              'source_inventory':{'build_source_sha256':'d'*64},'platform_digest':'sha256:'+'c'*64,'config_digest':'sha256:'+'e'*64}
        docs=native._live_documents(session,tool,'node-a','local-path','snapshot-1')
        pvc=docs['pvc'];self.assertEqual(pvc['spec']['storageClassName'],'local-path')
        self.assertEqual(pvc['spec']['accessModes'],['ReadWriteOnce'])
        self.assertNotIn('volumeName',pvc['spec']);self.assertNotIn('dataSource',pvc['spec'])
        for mode in ('bind','upgrade','verify'):
            spec=docs[mode]['spec']['template']['spec'];container=spec['containers'][0]
            self.assertNotIn('nodeName',spec)
            self.assertEqual(spec['affinity']['nodeAffinity']['requiredDuringSchedulingIgnoredDuringExecution']['nodeSelectorTerms'][0]['matchFields'][0]['values'],['node-a'])
            self.assertFalse(spec['automountServiceAccountToken'])
            self.assertEqual(spec['securityContext']['runAsUser'],65532)
            self.assertNotIn('fsGroup',spec['securityContext'])
            self.assertEqual(spec['volumes'][0]['persistentVolumeClaim']['claimName'],pvc['metadata']['name'])
            self.assertEqual(spec['volumes'][0]['persistentVolumeClaim']['readOnly'],mode=='verify')
            self.assertEqual(container['image'],tool['immutable_image']);self.assertTrue(container['securityContext']['readOnlyRootFilesystem'])
            self.assertFalse(any('hostPath' in row for row in spec['volumes']))
        self.assertEqual(docs['bind']['spec']['template']['spec']['containers'][0]['env'],[])
        self.assertIn('set -C',native.LIVE_UPGRADE_SCRIPT)
        self.assertIn('> /evidence/completed.jsonl',native.LIVE_UPGRADE_SCRIPT)
        self.assertIn('sync -f /evidence/completed.jsonl',native.LIVE_UPGRADE_SCRIPT)
        self.assertNotIn('cat ',native.LIVE_VERIFY_SCRIPT)
        desired=docs['upgrade'];spec=copy.deepcopy(desired['spec']);native._check_live_job_spec(spec,desired)
        for key,value in (('parallelism',2),('completions',2),('manualSelector',True),('successPolicy',{})):
            altered=copy.deepcopy(spec);altered[key]=value
            with self.subTest(key=key),self.assertRaisesRegex(native.Blocked,'space_live_admission_changed'):
                native._check_live_job_spec(altered,desired)
        with self.assertRaisesRegex(native.Blocked,'space_live_binding_invalid'):
            native._live_documents(session,tool,'node-a','local-path','$(foreign)')

    def test_live_context_derives_pvc_from_current_protected_database(self):
        session=Mock();session.operation='a'*12;session.context={'operation_id':'stg-run-17'}
        session.check.return_value={'phase':'prepare','target_sha256':'b'*64};session.binding.return_value={'target':'b'*64}
        tool={'immutable_image':'ghcr.io/poryadok/voiceroot/space-native-installer@sha256:'+'c'*64,
              'source_inventory':{'build_source_sha256':'d'*64},'platform_digest':'sha256:'+'c'*64,'config_digest':'sha256:'+'e'*64}
        original={'engine':{'namespace_uid':'ns','pod_uid':'pgpod','pvc_uid':'pgpvc','pv_uid':'pgpv'},'installer_role':'voice'}
        authority={'original':original,'tool':tool};archive={'binding':{'snapshot_id':'snapshot-1'}}
        qualified={'archive_sha256':native.owner.digest(archive)}
        engine={'pod':{'uid':'pgpod'},'pvc':{'uid':'pgpvc'},'pv':{'uid':'pgpv'}}
        env=[{'name':'POSTGRES_USER','valueFrom':{'configMapKeyRef':{'name':'voice-app-config','key':'POSTGRES_USER'}}},
             {'name':'POSTGRES_PASSWORD','valueFrom':{'secretKeyRef':{'name':'voice-app-secrets','key':'POSTGRES_PASSWORD'}}}]
        objects={('pod','voice-postgres-0'):{'spec':{'nodeName':'node-a','containers':[{'name':'postgres','env':env}]}},
            ('persistentvolumeclaim','pgdata-voice-postgres-0'):{'metadata':{'uid':'pgpvc'},'spec':{'storageClassName':'local-path'}},
            ('storageclass','local-path'):{'metadata':{'name':'local-path','uid':'class','resourceVersion':'1'},'provisioner':'local-path-provisioner'},
            ('configmap','voice-app-config'):{'data':{'POSTGRES_USER':'voice'}},
            ('secret','voice-app-secrets'):{'data':{'POSTGRES_PASSWORD':'private'}}}
        kube=Mock();kube.get.side_effect=lambda kind,name:copy.deepcopy(objects[(kind,name)])
        with patch.object(native.space,'load_native_authority',return_value=authority), \
             patch.object(native.owner,'load_qualified',return_value=qualified),patch.object(native.owner,'_base',return_value=Path('/unused')), \
             patch.object(native.owner,'_read',return_value=archive),patch.object(native.owner,'revalidate_paused'), \
             patch.object(native,'Kube',return_value=kube),patch.object(native.realtime_bootstrap,'engine_snapshot',return_value=('ns',engine)):
            result=native._live_context(session)
            self.assertEqual(result['documents']['pvc']['spec']['storageClassName'],'local-path')
            objects[('persistentvolumeclaim','pgdata-voice-postgres-0')]['metadata']['uid']='foreign'
            with self.assertRaisesRegex(native.Blocked,'space_live_source_changed'):native._live_context(session)

    def test_live_volume_rejects_foreign_claim_and_original_database_volume(self):
        claim_uid='11111111-1111-4111-8111-111111111111';pv_uid='22222222-2222-4222-8222-222222222222'
        plan={'documents':{'pvc':{'metadata':{'name':'evidence'},'spec':{'storageClassName':'local-path'}}},'storage_provisioner':'local-path-provisioner',
              'postgres_pvc_uid':'33333333-3333-4333-8333-333333333333','postgres_pv_uid':'44444444-4444-4444-8444-444444444444'}
        pvc={'metadata':{'name':'evidence','namespace':'voice-staging','uid':claim_uid,'resourceVersion':'1'},
             'spec':{'volumeName':'pv-evidence','storageClassName':'local-path','accessModes':['ReadWriteOnce']},'status':{'phase':'Bound'}}
        pv={'metadata':{'name':'pv-evidence','uid':pv_uid,'resourceVersion':'2','creationTimestamp':'2026-01-01T00:00:01Z',
            'annotations':{'pv.kubernetes.io/provisioned-by':'local-path-provisioner'}},'status':{'phase':'Bound'},
            'spec':{'storageClassName':'local-path','claimRef':{'name':'evidence','namespace':'voice-staging','uid':claim_uid}}}
        kube=Mock();kube.get.side_effect=lambda kind,name:copy.deepcopy(pvc if kind=='persistentvolumeclaim' else pv)
        with patch.object(native,'Kube',return_value=kube),patch.object(native,'_live_base',return_value=Path('/unused')), \
             patch.object(native.owner,'_read',return_value={'status':'DONE','response':{'uid':claim_uid,'creationTimestamp':'2026-01-01T00:00:00Z'}}):
            self.assertEqual(native._live_volume(SimpleNamespace(),plan)['pv_uid'],pv_uid)
            pv['spec']['claimRef']['uid']='foreign'
            with self.assertRaisesRegex(native.Blocked,'space_live_volume_changed'):native._live_volume(SimpleNamespace(),plan)
            pv['spec']['claimRef']['uid']=claim_uid;pv['metadata']['uid']=plan['postgres_pv_uid']
            with self.assertRaisesRegex(native.Blocked,'space_live_volume_changed'):native._live_volume(SimpleNamespace(),plan)
            pv['metadata']['uid']=pv_uid;pv['metadata']['annotations']['pv.kubernetes.io/provisioned-by']='foreign'
            with self.assertRaisesRegex(native.Blocked,'space_live_volume_changed'):native._live_volume(SimpleNamespace(),plan)
            pv['metadata']['annotations']['pv.kubernetes.io/provisioned-by']='local-path-provisioner'
            pv['metadata']['creationTimestamp']='2025-12-31T23:59:59Z'
            with self.assertRaisesRegex(native.Blocked,'space_live_volume_changed'):native._live_volume(SimpleNamespace(),plan)

    def test_live_pvc_lost_create_response_remains_pending(self):
        import commands
        session=Mock();session.operation='a'*12;session.context={'operation_id':'stg-run-17'};session.binding.return_value={'target':'b'*64}
        tool={'immutable_image':'ghcr.io/poryadok/voiceroot/space-native-installer@sha256:'+'c'*64,
              'source_inventory':{'build_source_sha256':'d'*64}}
        docs=native._live_documents(session,tool,'node-a','local-path','snapshot-1');plan={'documents':docs}
        kube=Mock();kube.run.return_value=copy.deepcopy(docs['pvc'])
        with tempfile.TemporaryDirectory(dir='/root') as directory:
            base=Path(directory)
            with patch.object(native,'_live_base',return_value=base),patch.object(native,'_live_checked_plan',return_value=plan), \
                 patch.object(native,'Kube',return_value=kube),patch.object(commands,'capture',side_effect=RuntimeError('lost-create')) as capture:
                with self.assertRaisesRegex(RuntimeError,'lost-create'):native._live_create(session,'pvc')
                with self.assertRaisesRegex(native.Blocked,'space_live_reconciliation_required'):native._live_create(session,'pvc')
                self.assertEqual(capture.call_count,1)
                self.assertEqual(native.owner._read(base/'effects'/'pvc-create.json')['status'],'PENDING')

    def test_live_jobs_retain_private_volume_and_verify_without_export(self):
        import commands
        session=Mock();session.operation='a'*12;session.context={'operation_id':'stg-run-17'};session.binding.return_value={'target':'b'*64}
        tool={'immutable_image':'ghcr.io/poryadok/voiceroot/space-native-installer@sha256:'+'c'*64,
              'source_inventory':{'build_source_sha256':'d'*64},'platform_digest':'sha256:'+'c'*64,'config_digest':'sha256:'+'e'*64}
        docs=native._live_documents(session,tool,'node-a','local-path','snapshot-1')
        plan={'documents':docs,'authority_sha256':'f'*64,'node':'node-a','storage_provisioner':'local-path-provisioner','postgres_pvc_uid':'old-pvc','postgres_pv_uid':'old-pv',
              'tool_platform_digest':tool['platform_digest'],'tool_config_digest':tool['config_digest']}
        live={};pvc_uid=str(uuid.uuid4());pv_uid=str(uuid.uuid4());pod_uid={};lost=False
        def pod_for(job):
            name=job['metadata']['name'];running=name.endswith('-bind')
            uid=pod_uid.setdefault(name,str(uuid.uuid4()));spec=copy.deepcopy(job['spec']['template']['spec']);spec['nodeName']='node-a'
            state={'running':{}} if running else {'terminated':{'exitCode':0}}
            return {'metadata':{'name':name+'-pod','namespace':'voice-staging','uid':uid,'resourceVersion':'9',
                'ownerReferences':[{'controller':True,'kind':'Job','uid':job['metadata']['uid']}]},'spec':spec,
                'status':{'phase':'Running' if running else 'Succeeded','containerStatuses':[{'name':'native',
                    'imageID':'fixture@'+tool['platform_digest'],'state':state}]}}
        class Kube:
            def get(self,kind,name):
                if kind=='persistentvolumeclaim':return copy.deepcopy(live['pvc'])
                if kind=='persistentvolume':return {'metadata':{'name':'pv-evidence','uid':pv_uid,'resourceVersion':'3',
                    'creationTimestamp':'2026-01-01T00:00:01Z','annotations':{'pv.kubernetes.io/provisioned-by':'local-path-provisioner'}},
                    'status':{'phase':'Bound'},'spec':{'storageClassName':'local-path','claimRef':{
                        'name':docs['pvc']['metadata']['name'],'namespace':'voice-staging','uid':pvc_uid}}}
                return copy.deepcopy(live[name])
            def run(self,args,body=None):
                if args[0]=='create':return copy.deepcopy(body)
                if args[1]=='--raw':return {'apiVersion':'batch/v1','kind':'JobList','metadata':{'resourceVersion':'4'},'items':[]}
                name=args[3].removeprefix('job-name=') if '-l' in args else None
                items=[pod_for(live[name])] if name in live else []
                return {'apiVersion':'v1','kind':'PodList','metadata':{'resourceVersion':'5'},'items':items}
        def capture(argv,body=b'',**kwargs):
            tail=argv[len(native.owner.PREFIX):]
            if tail[0]=='create':
                value=json.loads(body);name=value['metadata']['name']
                value['metadata'].update(uid=pvc_uid if value['kind']=='PersistentVolumeClaim' else str(uuid.uuid4()),resourceVersion='7',creationTimestamp='2026-01-01T00:00:00Z')
                if value['kind']=='PersistentVolumeClaim':
                    value['spec']['volumeName']='pv-evidence';value['status']={'phase':'Bound'};live['pvc']=copy.deepcopy(value)
                else:
                    value['status']={'active':1} if name.endswith('-bind') else {'succeeded':1};live[name]=copy.deepcopy(value)
                    if lost and name.endswith('-upgrade'):raise RuntimeError('upgrade-create-response-lost')
                return json.dumps(value).encode()
            if tail[0]=='delete':
                name=tail[1].rsplit('/',1)[1];del live[name]
                return json.dumps({'apiVersion':'v1','kind':'Status','status':'Success'}).encode()
            if tail[0]=='logs':
                result='1'*64+'  /evidence/origin.json\n'+'2'*64+'  /evidence/completed.jsonl\n'
                if '-verify-pod' in tail[1]:result+='verification=ok\n'
                return result.encode()
            raise AssertionError(tail)
        for lost in (False,True):
            live.clear();pod_uid.clear()
            with self.subTest(lost=lost),tempfile.TemporaryDirectory(dir='/root') as directory:
                base=Path(directory)/'live'
                with patch.object(native,'_live_base',return_value=base),patch.object(native,'_live_context',return_value=plan), \
                     patch.object(native,'_live_running_context',return_value=plan) as running, \
                     patch.object(native,'Kube',return_value=Kube()),patch.object(commands,'capture',side_effect=capture):
                    if lost:
                        with self.assertRaisesRegex(RuntimeError,'upgrade-create-response-lost'):native.execute_live(session)
                        self.assertEqual(native.owner._read(base/'effects'/'upgrade-create.json')['status'],'PENDING')
                        self.assertFalse((base/'effects'/'verify-create.json').exists())
                    else:
                        receipt=native.execute_live(session)
                        self.assertEqual(receipt['status'],'COMPLETE');self.assertEqual(receipt['volume']['pvc_uid'],pvc_uid)
                        self.assertEqual(receipt['evidence_hashes']['/evidence/completed.jsonl'],'2'*64)
                        self.assertEqual(set(live),{'pvc'})
                        self.assertEqual(running.call_count,2)
                        result=native.owner._read(base/'verify-result.json');result['hashes']['/evidence/origin.json']='4'*64
                        native.owner.save(base/'verify-result.json',result)
                        with self.assertRaisesRegex(native.Blocked,'space_live_receipt_changed'):native.load_live_completed(session)
                    with self.assertRaises(FileExistsError):native.execute_live(session)

    def test_changed_namespace_blocks_before_job_effect(self):
        kube=Mock();kube.get.return_value={}
        with patch.object(native.owner,'_guard',return_value={'namespace_uid':'original'}), \
             patch.object(native.owner,'_no_pending'),patch.object(native.space,'load_tool',return_value={}), \
             patch.object(native,'Kube',return_value=kube),patch.object(native,'_identity',return_value={'namespace_uid':'foreign'}), \
             patch.object(native,'_mutate') as mutate:
            with self.assertRaisesRegex(native.Blocked,'space_inspect_namespace_changed'):
                native.inspect_source(SimpleNamespace())
            mutate.assert_not_called();kube.run.assert_not_called()

    def test_pod_reread_requires_same_uid_spec_and_image_but_allows_new_rv(self):
        status={'name':'inspect','imageID':'immutable','state':{'terminated':{'exitCode':0}}}
        original={'metadata':{'uid':'owned'},'spec':{'nodeName':'node'},'status':{'containerStatuses':[status]}}
        fresh=copy.deepcopy(original);fresh['metadata'].update(namespace='voice-staging',resourceVersion='new')
        native._check_fresh_pod(original,fresh,status)
        for mutation in ('uid','spec','image','missing_rv'):
            changed=copy.deepcopy(fresh)
            if mutation=='uid':changed['metadata']['uid']='foreign'
            if mutation=='spec':changed['spec']['nodeName']='foreign'
            if mutation=='image':changed['status']['containerStatuses'][0]['imageID']='foreign'
            if mutation=='missing_rv':del changed['metadata']['resourceVersion']
            with self.subTest(mutation=mutation),self.assertRaisesRegex(native.Blocked,'space_inspect_pod_changed'):
                native._check_fresh_pod(original,changed,status)

    def test_historical_loader_rejects_noncanonical_operation_without_read(self):
        with patch.object(native.owner,'_read') as read:
            with self.assertRaisesRegex(native.Blocked,'space_inspect_history_invalid'):
                native.validate_historical_inspection('../foreign')
            read.assert_not_called()

    def test_historical_inspection_requires_released_rollout_and_receipt_link(self):
        operation='a'*12;source={'source':'fixture'};tool={'authenticated_publication':{'publication':{'tool':'fixture'}},
            'operation':operation,'selected':True};context={'commit':'b'*40}
        plan={'operation':operation,'context':context,'helper_sha256':'c'*64,
            'source_capture_sha256':native.owner.digest(source),'space_tool_sha256':native.owner.digest(tool)}
        receipt={'inspection':'fixture'}
        qualified={'operation':operation,'plan_sha256':native.owner.digest(plan),'space_inspection_sha256':native.owner.digest(receipt)}
        rollout={'status':'PASS','phase':'POST_RESTART','fence_status':'RELEASED','operation':operation,
            'target':{'cohort_preservation_sha256':native.owner.digest(qualified)}}
        values={'checkpoint.json':{'status':'QUALIFIED','plan':plan},'qualified.json':qualified,'space-tool.json':tool,
            operation+'.json':source}
        def read(path,**kwargs):
            return copy.deepcopy(rollout if path.parent.name=='rollout-'+operation else values[path.name])
        with patch.object(native.owner,'_read',side_effect=read), \
             patch.object(native.owner.resource_claim,'historical_binding',return_value={'context':context,'helper_sha256':'c'*64}), \
             patch.object(native,'_retained_record',return_value=receipt):
            self.assertEqual(native.validate_historical_inspection(operation),receipt)
            rollout['fence_status']='ACTIVE'
            with self.assertRaisesRegex(native.Blocked,'space_inspect_history_invalid'):
                native.validate_historical_inspection(operation)

    def test_owned_job_and_cleanup_with_real_session_and_journal(self):
        import commands, guard, resource_claim
        from staging_cohort_preservation_test import CutEffectTests
        with tempfile.TemporaryDirectory(dir='/root') as directory:
            root=Path(directory);(root/'installed').mkdir(mode=0o700)
            with patch.object(guard,'ROOT',root),patch.object(native.owner,'_source'):
                session,_=CutEffectTests().fixture(root,{'spec':{'replicas':1}})
                tool={'immutable_image':'ghcr.io/poryadok/voiceroot/space-native-installer@sha256:'+'c'*64,
                    'source_inventory':{'build_source_sha256':'d'*64},'platform_digest':'sha256:'+'c'*64,
                    'config_digest':'sha256:'+'e'*64,'content':{'executable_sha256':'f'*64,
                    'migrator_sha256':'1'*64,'sql_inventory_sha256':'2'*64}}
                identity={'namespace_uid':'namespace','engine':{},'database':{'system_identifier':'42','server_start':'t',
                    'database_oid':123,'database_owner':'admin','session_user':'admin','schema_owner':'admin'}}
                config={'metadata':{'uid':'config','resourceVersion':'1'},'data':{'POSTGRES_USER':'admin'}}
                secret={'metadata':{'uid':'secret','resourceVersion':'2'},'data':{'POSTGRES_PASSWORD':base64.b64encode(b'private').decode()}}
                env=[{'name':'POSTGRES_USER','valueFrom':{'configMapKeyRef':{'name':'voice-app-config','key':'POSTGRES_USER'}}},
                     {'name':'POSTGRES_PASSWORD','valueFrom':{'secretKeyRef':{'name':'voice-app-secrets','key':'POSTGRES_PASSWORD'}}}]
                pgpod={'spec':{'nodeName':'node-a','containers':[{'name':'postgres','env':env}]}}
                job=native._job(session,tool,'node-a');uid='11111111-1111-4111-8111-111111111111'
                job['metadata'].update(uid=uid,resourceVersion='7');complete=copy.deepcopy(job);complete['status']={'succeeded':1}
                pod={'metadata':{'name':'inspection-pod','namespace':'voice-staging','resourceVersion':'8','uid':'22222222-2222-4222-8222-222222222222',
                    'ownerReferences':[{'kind':'Job','uid':uid,'controller':True}]},'spec':copy.deepcopy(job['spec']['template']['spec']),
                    'status':{'containerStatuses':[{'name':'inspect','imageID':'fixture@'+tool['platform_digest'],
                    'state':{'terminated':{'exitCode':0}}}]}}
                result={'event':'inspected-legacy24','schema':'voice.space.native-legacy24-inspection.v1',
                    'source_revision':session.context['commit'],'build_sha256':'d'*64,'executable_sha256':'f'*64,
                    'migrator_sha256':'1'*64,'inventory_sha256':'2'*64,'target':{'database_name':'space_db',
                    'database_oid':123,'database_system_id':'42','database_owner':'admin','installer_role':'admin','version':24}}
                kube=Mock();values={('configmap','voice-app-config'):config,('secret','voice-app-secrets'):secret,
                    ('pod','voice-postgres-0'):pgpod,('pod','inspection-pod'):pod,('job',job['metadata']['name']):complete}
                kube.get.side_effect=lambda kind,name:copy.deepcopy(values[(kind,name)])
                kube.run.return_value=copy.deepcopy(job);deleted=[]
                def capture(argv,**kwargs):
                    commands.authority_guard(argv);tail=argv[len(native.owner.PREFIX):]
                    if tail[:1]==['create']:return json.dumps(job).encode()
                    if tail[:1]==['delete']:
                        deleted.append(True);return json.dumps({'apiVersion':'v1','kind':'Status','status':'Success'}).encode()
                    if tail[:1]==['exec']:return ('admin\n'+hashlib.sha256(b'private').hexdigest()+'  -\n').encode()
                    if tail[:2]==['get','pods']:
                        return json.dumps({'apiVersion':'v1','kind':'PodList','metadata':{'resourceVersion':'8'},'items':[] if deleted else [pod]}).encode()
                    if tail[:2]==['get','--raw']:
                        return json.dumps({'apiVersion':'batch/v1','kind':'JobList','metadata':{'resourceVersion':'9'},'items':[]}).encode()
                    if tail[:1]==['logs']:return json.dumps(result).encode()
                    raise AssertionError(tail)
                with resource_claim.command_scope(session.check),patch.object(commands,'capture',side_effect=capture), \
                     patch.object(native,'Kube',return_value=kube),patch.object(native.space,'load_tool',return_value=tool), \
                     patch.object(native,'_identity',return_value=identity):
                    receipt=native.inspect_source(session)
                    self.assertEqual(native.load_inspection(session),receipt)
                    values[('secret','voice-app-secrets')]=copy.deepcopy(secret)
                    values[('secret','voice-app-secrets')]['metadata']['resourceVersion']='3'
                    self.assertEqual(native.load_retained_inspection(session),receipt)
                    with self.assertRaisesRegex(native.Blocked,'space_inspect_credentials_changed'):
                        native.load_inspection(session)
                self.assertEqual(len(deleted),1)
                for name in ('space-inspect-create','space-inspect-delete'):
                    self.assertEqual(native.owner._read(native.owner._base(session)/'effects'/(name+'.json'))['status'],'DONE')

    def test_delete_response_must_confirm_success_or_same_job(self):
        native._check_delete_response({'apiVersion':'v1','kind':'Status','status':'Success'},'owned')
        native._check_delete_response({'apiVersion':'batch/v1','kind':'Job','metadata':{'uid':'owned'}},'owned')
        for response in ({},{'apiVersion':'v1','kind':'Status','status':'Failure'},
                         {'apiVersion':'batch/v1','kind':'Job','metadata':{'uid':'foreign'}}):
            with self.assertRaisesRegex(native.Blocked,'space_inspect_delete_invalid'):
                native._check_delete_response(response,'owned')

    def test_pod_list_requires_complete_versioned_observation(self):
        for value in ({'apiVersion':'v1','kind':'PodList','metadata':{'resourceVersion':'8','continue':'next'},'items':[]},
                      {'apiVersion':'v1','kind':'PodList','metadata':{},'items':[]}):
            with patch.object(native.owner,'_observation',return_value=json.dumps(value).encode()):
                with self.assertRaisesRegex(native.Blocked,'space_inspect_absence_invalid'):
                    native._pod_list(SimpleNamespace(),'voice-space-inspect-aaaaaaaaaaaa')

    def test_cleanup_rejects_foreign_job_before_delete(self):
        kube=Mock();kube.get.return_value={'metadata':{'uid':'foreign','resourceVersion':'9'}}
        with patch.object(native,'_mutate') as mutate:
            with self.assertRaisesRegex(native.Blocked,'space_inspect_job_changed'):
                native._cleanup(SimpleNamespace(),kube,'voice-space-inspect-aaaaaaaaaaaa','owned')
            mutate.assert_not_called()

    def test_effect_boundary_rejects_foreign_job_delete(self):
        session=SimpleNamespace(operation='a'*12)
        uid='11111111-1111-4111-8111-111111111111'
        kube=Mock();kube.get.return_value={'metadata':{'uid':uid,'resourceVersion':'9'}}
        options={'apiVersion':'v1','kind':'DeleteOptions','propagationPolicy':'Foreground',
                 'preconditions':{'uid':uid,'resourceVersion':'9'}}
        argv=native.owner.PREFIX+['delete','--raw=/apis/batch/v1/namespaces/voice-staging/jobs/voice-space-inspect-'+session.operation,'-f','/dev/stdin']
        with patch.object(native.owner,'_guard'),patch.object(native.owner,'_no_pending'),patch.object(native,'Kube',return_value=kube), \
             patch.object(native.owner,'_base',return_value=Path('/unused')),patch.object(native.owner,'_read',return_value={'status':'DONE','response':{'uid':'foreign'}}), \
             patch.object(native.owner,'_send') as submit:
            with self.assertRaisesRegex(native.Blocked,'space_inspect_effect_invalid'):
                native._mutate(session,'space-inspect-delete',argv,native.owner.private._bytes(options))
            submit.assert_not_called()

    def test_lost_create_reply_stays_pending_and_cannot_be_replayed(self):
        import commands, guard, resource_claim
        from staging_cohort_preservation_test import CutEffectTests
        with tempfile.TemporaryDirectory(dir='/root') as directory:
            root=Path(directory);(root/'installed').mkdir(mode=0o700)
            with patch.object(guard,'ROOT',root),patch.object(native.owner,'_source'):
                session,_=CutEffectTests().fixture(root,{'spec':{'replicas':1}})
                calls=[]
                def lost(argv,**kwargs):
                    commands.authority_guard(argv);calls.append(argv)
                    raise RuntimeError('create_response_lost')
                argv=native.owner.PREFIX+['create','-f','/dev/stdin','-o','json']
                tool={'immutable_image':'ghcr.io/poryadok/voiceroot/space-native-installer@sha256:'+'c'*64,
                      'source_inventory':{'build_source_sha256':'d'*64}}
                kube=Mock();kube.get.return_value={'spec':{'nodeName':'node-a'}}
                body=native.owner.private._bytes(native._job(session,tool,'node-a'))
                with resource_claim.command_scope(session.check),patch.object(commands,'capture',side_effect=lost), \
                     patch.object(native,'Kube',return_value=kube),patch.object(native.space,'load_tool',return_value=tool):
                    with self.assertRaisesRegex(native.Blocked,'space_inspect_effect_invalid'):
                        native._mutate(session,'space-inspect-create',argv,b'{}')
                    self.assertEqual(calls,[])
                    with self.assertRaisesRegex(RuntimeError,'create_response_lost'):
                        native._mutate(session,'space-inspect-create',argv,body)
                    with self.assertRaisesRegex(Exception,'cohort_reconciliation_required'):
                        native._mutate(session,'space-inspect-create',argv,body)
                self.assertEqual(len(calls),1)
                record=native.owner._read(native.owner._base(session)/'effects'/'space-inspect-create.json')
                self.assertEqual(record['status'],'PENDING')

    def test_rejects_admission_injected_container_or_host_access(self):
        session=SimpleNamespace(operation='a'*12)
        tool={'immutable_image':'ghcr.io/poryadok/voiceroot/space-native-installer@sha256:'+'c'*64,
              'source_inventory':{'build_source_sha256':'d'*64}}
        wanted=native._job(session,tool,'node-a')['spec']['template']['spec']
        native._check_template(copy.deepcopy(wanted),wanted)
        defaulted=copy.deepcopy(wanted)
        defaulted['containers'][0].update(resources={},terminationMessagePath='/dev/termination-log',terminationMessagePolicy='File')
        native._check_template(defaulted,wanted)
        for field,value in [('initContainers',[{'name':'foreign'}]),('hostNetwork',True),('volumes',[{'name':'foreign'}]),
                            ('dnsConfig',{'nameservers':['192.0.2.1']})]:
            changed=copy.deepcopy(wanted);changed[field]=value
            with self.subTest(field=field),self.assertRaisesRegex(native.Blocked,'space_inspect_job_invalid'):
                native._check_template(changed,wanted)

    def test_fixed_inspection_requires_bound_tool_identity_and_owned_cleanup(self):
        session=SimpleNamespace(operation='a'*12,context={'commit':'b'*40})
        tool={'immutable_image':'ghcr.io/poryadok/voiceroot/space-native-installer@sha256:'+'c'*64,
              'source_inventory':{'build_source_sha256':'d'*64},'platform_digest':'sha256:'+'c'*64,
              'config_digest':'sha256:'+'e'*64,'content':{'executable_sha256':'f'*64,'migrator_sha256':'1'*64,'sql_inventory_sha256':'2'*64}}
        identity={'namespace_uid':'ns','engine':{},'database':{'system_identifier':'42','server_start':'t','database_oid':123,
                  'database_owner':'admin','session_user':'admin','schema_owner':'admin'}}
        config={'metadata':{'uid':'config','resourceVersion':'1'},'data':{'POSTGRES_USER':'admin'}}
        secret={'metadata':{'uid':'secret','resourceVersion':'2'},'data':{'POSTGRES_PASSWORD':base64.b64encode(b'private').decode()}}
        env=[{'name':'POSTGRES_USER','valueFrom':{'configMapKeyRef':{'name':'voice-app-config','key':'POSTGRES_USER'}}},
             {'name':'POSTGRES_PASSWORD','valueFrom':{'secretKeyRef':{'name':'voice-app-secrets','key':'POSTGRES_PASSWORD'}}}]
        pgpod={'spec':{'nodeName':'node-a','containers':[{'name':'postgres','env':env}]}}
        job=native._job(session,tool,'node-a');uid='11111111-1111-4111-8111-111111111111'
        job['metadata'].update(uid=uid,resourceVersion='7');complete=copy.deepcopy(job);complete['status']={'succeeded':1}
        pod={'metadata':{'name':'inspection-pod','namespace':'voice-staging','resourceVersion':'8','uid':'22222222-2222-4222-8222-222222222222',
             'ownerReferences':[{'kind':'Job','uid':uid,'controller':True}]},
             'spec':copy.deepcopy(job['spec']['template']['spec']),
             'status':{'containerStatuses':[{'name':'inspect','imageID':'fixture@'+tool['platform_digest'],'state':{'terminated':{'exitCode':0}}}]}}
        result={'event':'inspected-legacy24','schema':'voice.space.native-legacy24-inspection.v1',
            'source_revision':session.context['commit'],'build_sha256':'d'*64,'executable_sha256':'f'*64,
            'migrator_sha256':'1'*64,'inventory_sha256':'2'*64,'target':{'database_name':'space_db','database_oid':123,
                'database_system_id':'42','database_owner':'admin','installer_role':'admin','version':24}}
        values={('configmap','voice-app-config'):config,('secret','voice-app-secrets'):secret,('pod','voice-postgres-0'):pgpod,('pod','inspection-pod'):pod,
                ('job',job['metadata']['name']):complete}
        kube=Mock();kube.get.side_effect=lambda kind,name:copy.deepcopy(values[(kind,name)])
        kube.run.return_value=copy.deepcopy(job)
        observe=[('admin\n'+hashlib.sha256(b'private').hexdigest()+'  -\n').encode(),
                 json.dumps({'apiVersion':'v1','kind':'PodList','metadata':{'resourceVersion':'8'},'items':[pod]}).encode(),json.dumps(result).encode()]
        with patch.object(native,'Kube',return_value=kube),patch.object(native.owner,'_guard',return_value={'namespace_uid':'ns'}), \
             patch.object(native.owner,'_no_pending'),patch.object(native.space,'load_tool',return_value=tool), \
             patch.object(native,'_identity',return_value=identity),patch.object(native,'_mutate',return_value=('path',{},json.dumps(job).encode())), \
             patch.object(native,'_done'),patch.object(native.owner,'_observation',side_effect=observe), \
             patch.object(native,'_cleanup') as cleanup,patch.object(native.owner,'_write'):
            receipt=native.inspect_source(session)
        self.assertEqual(receipt['result'],result);self.assertEqual(receipt['cleanup'],'verified')
        cleanup.assert_called_once_with(session,kube,job['metadata']['name'],uid)
        self.assertNotIn('private',json.dumps(job))


if __name__=='__main__':unittest.main()
