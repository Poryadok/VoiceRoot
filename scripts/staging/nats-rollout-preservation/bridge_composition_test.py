"""Actual bridge-generated identities cross CMS and remote artifact custody."""
import copy
import hashlib
import json
import os
from pathlib import Path
import shutil
import tempfile
import unittest
from unittest.mock import patch
import bridge_root
import encrypted_cut
import github_custody
import github_custody_test as http
import native_store


class CompositionTests(unittest.TestCase):
    def test_actor_drift_after_offnode_readback_prevents_transaction_authorization(self):
        import actor_root
        actions=object.__new__(bridge_root.Actions);actions.code=Path('/captured-code');actions.binding={'fixed':'hash'}
        state={'operation':'123456abcdef','cipher_binding':{},'service_actor_services':['social'],
            'service_actor_authority':{'binding':'root-produced'}}
        order=[]
        with patch.object(actions,'state',return_value=(Path('/private-op'),state)),\
             patch.object(bridge_root.github_custody,'verify_artifact',side_effect=lambda *a:order.append('full-offnode-readback') or {'verified':True}),\
             patch.object(actions,'_verify_decrypted_restore',side_effect=lambda *a:order.append('decrypted-restore')),\
             patch.object(bridge_root,'save'),patch.object(bridge_root,'Kube'),\
             patch.object(bridge_root.transaction,'reconstruct'),\
             patch.object(actor_root,'revalidate',side_effect=bridge_root.Blocked('existing_service_actor_authority_changed')) as actor,\
             patch.object(bridge_root.transaction,'authorize') as authorize:
            with self.assertRaises(bridge_root.Blocked):actions.execute({'action':'authorize','operation':'123456abcdef','token':'private-token','artifact_id':1})
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
        previous={'target':{'images':{'voice-user/user':expected}},
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
                    patch.object(bridge_root.RolloutStage,'capture',return_value=stage),\
                    patch.object(bridge_root.source_authority,'same_image_content',return_value=matches) as identity,\
                    patch.object(bridge_root,'public_read',side_effect=ReachedContract) as contract:
                    with self.assertRaises(ReachedContract if matches else bridge_root.Blocked):
                        actions._prepare({'nonce':'a'*64,'token':'private','mode':'images-only','changed_services':['user']},previous)
                    identity.assert_called_once()
                    self.assertEqual(identity.call_args.args[:2],(actual,expected))
                    self.assertEqual(contract.call_count,1 if matches else 0)

    def test_root_prepare_full_and_app_only_actual_hub_catalog_reaches_canonical_compiler(self):
        import compiler, source_plan_test as fixtures, runtime_stage
        from types import SimpleNamespace
        root=Path('/var/lib/voice-nats-preservation');root.mkdir(exist_ok=True,mode=0o750)
        for mode in ('full','app-only'):
            with self.subTest(mode=mode),tempfile.TemporaryDirectory(dir=root) as directory:
                base=Path(directory);installed=base/'installed';(installed/'sources').mkdir(parents=True)
                kube,p=fixtures.SourceTests().canonical_fixture(mode)
                p.update(generation='r20260930a4',dataPVC='voice-nats-jsdata-d202610040049430b')
                targets={'voice-gateway',*('voice-'+n for n in runtime_stage.LEAVES),'voice-web','voice-admin','voice-developer-portal'}
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
                stage=SimpleNamespace(snapshots=snapshots,original_snapshots=copy.deepcopy(snapshots),old_images=old,
                    expected={'generation':p['generation'],'source_claim':p['dataPVC'],'source_claim_uid':'pvc','source_pv_uid':'pv'})
                code=base/'code';(code/'nats-known-baseline').mkdir(parents=True)
                contract={'scripts':[]}
                for part in ('realtime','notification','search','analytics-chat'):
                    raw=(fixtures.SOURCE/('deploy/templates/nats-'+part+'-bootstrap.yaml')).read_bytes().split(b'---')[0]
                    contract['scripts'].append({'part':part,'sha256':hashlib.sha256(fixtures.decoder(raw)['data']['bootstrap.sh'].encode()).hexdigest()})
                (code/'nats-known-baseline/deployed-contract.json').write_text(json.dumps(contract))
                optional={'s3_signing_endpoint','gateway_host','storage_host','livekit_host','web_host','admin_host','developer_portal_host','gateway_tls_secret','storage_tls_secret','image_pull_secret','apply_observability','minio_image','minio_mc_image','minio_storage_class','minio_storage_size','web_origin'}
                policy={k:v for k,v in p.items() if k in optional};(installed/'policy.json').write_text(json.dumps(policy));(installed/'policy.json').chmod(0o600)
                approved={'source_sha':p['tag'],'images':{name.removeprefix('voice-'):bridge_root.source_authority.REGISTRY+'/'+name.removeprefix('voice-')+'@sha256:'+'a'*64 for name in targets}}
                approved['images']['nats-hub-config-renderer']=renderer_repo+'@sha256:'+'d'*64
                import renderer_root,renderer_transition,actor_root
                def renderer_proof(kube,current,source_sha,image):
                    descriptor=renderer_transition.plan(current.snapshots[bridge_root.HUB],old[bridge_root.HUB+'/nats-config-renderer'],image,
                        source_sha,renderer_root.storage(current))
                    return {'descriptor':descriptor,'broker_image':old[bridge_root.HUB+'/nats'],
                        'output':{'sha256':'e'*64,'bytes':10,'verified':True},'input_binding':{}}
                def capture_source(token,run,sha,workspace):
                    for relative in ('scripts','deploy','src/backend/migrations','src/backend/auth/src/main/resources/db/migration','docker/clickhouse/init'):
                        shutil.copytree(fixtures.SOURCE/relative,workspace/relative,dirs_exist_ok=True)
                    return approved
                def capture(args,body=b'',**kwargs):
                    if args[1]=='create':return json.dumps(fixtures.decoder(body)).encode()
                    return json.dumps(kube.run(args[3:],None if not body else json.loads(body))).encode() if args[3] in ('apply','exec') else json.dumps(kube.get(args[4],args[5])).encode()
                reached=[]
                def prepare_value(value,*args,**kwargs):
                    import guard_test,datetime as dt
                    authorization=guard_test.receipt()
                    authorization.update(schema='nats-rollout-preservation-v1',target=copy.deepcopy(value['target']),
                        backup={'archive_sha256':'1'*64,'manifest_sha256':'2'*64,'census_sha256':'3'*64,'off_node_verified':True},
                        expires_at=(dt.datetime.now(dt.timezone.utc)+dt.timedelta(hours=1)).isoformat())
                    authorization['target']['input_template_hashes']=dict(value['target']['template_hashes'])
                    bridge_root.guard.validate(authorization,value['target']['registry'],value['target']['tag'],mode,[],installed/'sources'/('a'*12))
                    reached.append(value);op=args[2];operation=base/('rollout-'+op);operation.mkdir()
                    return {'operation':op,'target':value['target']}
                actions=object.__new__(bridge_root.Actions);actions.code=code;actions.binding={}
                execution={'dispatcher_run_id':123,'head_sha':p['tag']}
                with patch.object(bridge_root,'INSTALLED',installed),patch.object(bridge_root.guard,'ROOT',base),patch.object(actions,'_idle'),patch.object(actions,'_dispatcher',return_value=(123,execution)),patch.object(bridge_root.source_authority,'capture_source',capture_source),patch.object(bridge_root,'Kube',return_value=SimpleNamespace(get=lambda *a:{'data':{'generation':p['generation'],'dataPVC':p['dataPVC']}})),patch.object(bridge_root.RolloutStage,'capture',return_value=stage),patch('commands.capture',side_effect=capture),patch.object(compiler,'capture',side_effect=capture),patch.object(renderer_root,'prove',side_effect=renderer_proof),patch.object(actor_root,'preflight',return_value=None),patch.object(bridge_root.root_cli,'prepare_value',side_effect=prepare_value),patch.object(actions,'_encrypt',side_effect=lambda base,state,execution:state):
                    result=actions._prepare({'nonce':'a'*64,'token':'private','mode':mode,'changed_services':[]})
                self.assertEqual(result['target']['mode'],mode);self.assertEqual(len(reached[0]['nonnats']['checks']),16)
                self.assertEqual(any(key.startswith(bridge_root.HUB+'/') for key in result['target']['images']),mode=='full')
                self.assertFalse(any(row['metadata']['name']==bridge_root.HUB for row in reached[0]['manifests']))
    def test_actual_root_cipher_binding_twelve_hex_operation_reaches_remote_readback(self):
        root=Path('/var/lib/voice-nats-preservation');root.mkdir(exist_ok=True,mode=0o750)
        base=Path(tempfile.mkdtemp(prefix='composition-',dir=root));self.addCleanup(lambda:shutil.rmtree(base))
        installed=base/'installed';installed.mkdir(mode=0o750);keys=installed/'recovery';keys.mkdir(mode=0o700)
        encrypted_cut.initialize_recovery_key(keys)
        source=base/'native';source.mkdir();(source/'record.blk').write_bytes(b'known synthetic record with pending ACK')
        manifest=native_store.archive_closed_store(source,base/'rollout-before.tar')
        (base/'rollout-before-manifest.json').write_text(json.dumps(manifest));(base/'rollout-before-manifest.json').chmod(0o600)
        nonce='a'*64;state={'operation':nonce[:12],'phase':'AWAITING_OFF_NODE','status':'WAITING','fence_status':'VERIFIED',
            'target':{'changed_services':['web'],'mode':'images-only','tag':'c'*40},
            'cut':{'manifest':manifest,'manifest_sha256':hashlib.sha256((base/'rollout-before-manifest.json').read_bytes()).hexdigest(),'census_sha256':'d'*64}}
        actions=object.__new__(bridge_root.Actions)
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
