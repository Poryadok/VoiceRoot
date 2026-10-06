"""Composed root phase bindings with explicit external substrate fixtures.

Actual pinned broker/native/payload coverage lives in nats_contract_integration.
This test joins producer, durable files, custody, CM advancement and finisher;
its Kubernetes stage and transport proofs are synthetic external fixtures.
"""
import copy,datetime as dt,hashlib,json,os,tempfile,unittest
from pathlib import Path
from unittest.mock import Mock,patch
import bridge_root,guard,root_cli,transaction,encrypted_cut,github_custody
import actor_root
import github_custody_test as http
from native_store import archive_closed_store
from nats_contract_plan import digest

@unittest.skipUnless(os.name=='posix' and os.geteuid()==0,'disposable root Linux fixture')
class Tests(unittest.TestCase):
    def test_root_producer_custody_cm_advance_and_finish_share_bindings(self):
        root=Path('/var/lib/voice-nats-preservation');root.mkdir(exist_ok=True,mode=0o750)
        with tempfile.TemporaryDirectory(dir=root) as td:
            workspace=Path(td);installed=workspace/'installed';installed.mkdir()
            code=workspace/'code';(code/'nats-known-baseline').mkdir(parents=True)
            kernel=code/'nats-known-baseline/kernel';kernel.write_bytes(b'fixture-external-kernel')
            binding={'nats-known-baseline/kernel':hashlib.sha256(kernel.read_bytes()).hexdigest()}
            store=workspace/'store';store.mkdir();(store/'record.blk').write_bytes(b'five known fixture records')
            uid='a'*8+'-'+ 'a'*4+'-'+ 'a'*4+'-'+ 'a'*4+'-'+ 'a'*12
            old_image=next(iter(actor_root.STORY_IMAGES));child,config,component=actor_root.STORY_IMAGES[old_image]
            old={'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':'voice-story','namespace':guard.NS,'uid':uid,'resourceVersion':'1'},
                'spec':{'replicas':1,'template':{'spec':{'containers':[{'name':'story','image':old_image}]}}}}
            wanted=copy.deepcopy(old);wanted['spec']['template']['spec']['containers'][0]['image']='ghcr.io/poryadok/voiceroot/story@sha256:09c6481710665e056d5b29c092189bff899a78e6c457be5a521edec12bbb0fa9'
            empty={'checks':[],'actions':[]}
            target={'mode':'images-only','changed_services':['story'],'tag':'c'*40,'images':{'voice-story/story':wanted['spec']['template']['spec']['containers'][0]['image']},
                'template_hashes':{'voice-story':digest(wanted['spec']['template'])},'manifest_sha256':digest([wanted]),
                'migration_sha256':digest([]),'nonnats_sha256':digest(empty)}
            scripts={part:{'part':part,'configMapUID':part,'configMapResourceVersion':'1','sha256':hashlib.sha256(b'old').hexdigest(),'bytes':3} for part in ('realtime','analytics-chat')}
            contract={'scripts':list(scripts.values()),'consumer_pairs':[]}
            cms={'voice-nats-'+part+'-bootstrap':{'metadata':{'uid':part,'name':'voice-nats-'+part+'-bootstrap','resourceVersion':'1'},'data':{'bootstrap.sh':'old'}} for part in scripts}
            marker={'metadata':{'uid':'marker','resourceVersion':'1'},'data':{'generation':'r20260930a4','previousGeneration':'r20260930a3','dataPVC':'selected','phase':'active'}}
            claim={'metadata':{'name':'selected','uid':'claim'},'spec':{'volumeName':'pv'},'status':{'capacity':{'storage':'1Gi'}}}
            class Kube:
                def get(self,kind,name):
                    if kind=='configmap':return copy.deepcopy(cms[name])
                    if kind=='pvc':return copy.deepcopy(claim)
                    return copy.deepcopy(old)
                def run(self,args,body=None):return copy.deepcopy(body)
                def cas(self,kind,row,changes):
                    current=cms[row['metadata']['name']]
                    assert current['metadata']==row['metadata']
                    current['data']['bootstrap.sh']=changes[0]['value'];current['metadata']['annotations']=changes[1]['value'];current['metadata']['resourceVersion']='2'
                    return copy.deepcopy(current)
            kube=Kube();stage=Mock();stage.operation='a'*12;stage.expected={'namespace_uid':'namespace','generation':'r20260930a4','source_claim':'selected','source_claim_uid':'claim'}
            stage.snapshots={'voice-story':copy.deepcopy(old)};stage.original_snapshots=copy.deepcopy(stage.snapshots)
            stage.old_images={'voice-story/story':old_image}
            stage.marker=marker;stage.service={};stage.final_claim=claim;stage.final_pv={'metadata':{'uid':'pv'},'spec':{'local':{'path':str(store)}}};stage.final_path=store
            order=[];messages=[5]
            def fence():
                stage.marker['data'].update(phase='rollout-capturing',knownRolloutOperation=stage.operation)
                stage.snapshots['voice-story']['spec']['replicas']=0;order.append('closed-fence')
            stage.fence.side_effect=fence
            def prepared():stage.marker['data']['phase']='rollout-prepared';order.append('prepared-after-config-proof')
            stage.prepared.side_effect=prepared;stage.verified.side_effect=lambda:order.append('verified-before-restart')
            def restart():
                order.append('sole-restart');stage.marker['data']['phase']='active';stage.marker['data'].pop('knownRolloutOperation',None)
                return {'verified':True}
            stage.restart.side_effect=restart
            enrollment={'fixture':'existing-actor-auth-proof'};authority={'plan':{'migration_id':'fixed-approved','sources':{}},'binding':{},
                'scripts':{part:{'script':'target','sha256':hashlib.sha256(b'target').hexdigest()} for part in scripts}}
            # Explicit external actor fixture; actual role/auth/schema checks have
            # separate real-capture and pinned-binary tests, never caller flags.
            actor={'roles':['story'],'proofs':{'story':{'fixture-external-auth':True}},'mounts':{'story':{'fixture-exact-mount':True}},'binding':{'fixture-same-account':True},'story_schema':{'clean_version':4}}
            actor['compatible_story_candidate']={'schema':'voice-reviewed-story-compatible-pair-v1','image':old_image,'child_sha256':child,'config_sha256':config,'component_source_sha':component,'schema_sha256':transaction.canonical(actor['story_schema']),'actor_sha256':transaction.canonical({'binding':actor['binding'],'mount':actor['mounts']['story'],'proof':actor['proofs']['story']})}
            def cold(runtime,source,fence):
                fence();manifest=archive_closed_store(source,runtime.base/'rollout-before.tar')
                order.append('original-cold-cut')
                return {'manifest':manifest,'census':{'streams':[{'state':{'messages':messages[0]}}]},'census_sha256':'3'*64}
            def runtime(base,op):return type('Transport',(),{'base':Path(base)})()
            def migration(base,state,observed,journal):
                assert state['custody']['verified'] and state['phase']=='NATS_CONTRACT_MIGRATION'
                order.append('migration-after-custody')
                return {'verified':True,'old_record_files_verified':True,'proof':{'fixture-external-proof':True},'cut':{'post-config-baseline':messages[0]}}
            def proof(runtime,source,cut,fence):
                self.assertEqual(cut,{'post-config-baseline':messages[0]});order.append('post-config-store-proof');return {'messages':messages[0],'native_files_verified':True}
            actions=object.__new__(bridge_root.Actions);actions.code=code;actions.binding=binding
            protected_receipt=guard.protected_receipt
            with patch.object(root_cli,'ROOT',workspace),patch.object(guard,'ROOT',workspace),patch.object(bridge_root,'INSTALLED',installed),\
                patch.object(root_cli,'Kube',return_value=kube),patch.object(bridge_root,'Kube',return_value=kube),\
                patch('grp.getgrnam',return_value=type('Group',(),{'gr_gid':1000})()),\
                patch.object(transaction.RolloutStage,'capture',return_value=stage),patch.object(transaction,'reconstruct',return_value=stage),\
                patch.multiple(transaction,native_preflight=Mock(return_value={}),private_json=Mock(return_value=enrollment)),\
                patch.object(transaction,'capture_inputs',return_value={}) as capture,\
                patch.object(transaction.nonnats_runtime,'preflight',return_value={}),patch.object(transaction.nonnats_runtime,'verify',return_value={'verified':True}),\
                patch.object(transaction.nonnats_plan,'revalidate'),patch('source_plan.verify_db_init'),\
                patch.object(actor_root,'revalidate'),\
                patch.object(transaction,'DockerRuntime',side_effect=runtime),patch.object(transaction,'capture_cut',side_effect=cold),\
                patch.object(transaction,'apply_contract',side_effect=migration),patch.object(transaction,'verify_post_apply',side_effect=proof):
                state=root_cli.prepare_value({'target':target,'manifests':[wanted],'contract':contract,'migrations':[],'nonnats':empty},code,binding,'a'*12,nats_authority=authority,actor_authority=actor,actor_services=['story'])
                capture.assert_called_once();self.assertEqual(capture.call_args.kwargs['bootstrap_enrollment'],enrollment)
                self.assertEqual(order,['closed-fence','original-cold-cut']);base=workspace/('rollout-'+'a'*12)
                recovery=installed/'recovery';recovery.mkdir(mode=0o700);encrypted_cut.initialize_recovery_key(recovery)
                def authorize_bound(base,state):
                    actions._encrypt(base,state,{'dispatcher_run_id':123,'head_sha':'c'*40})
                    zipped=http.archive((base/'rollout-backup.cms').read_bytes());binding=state['cipher_binding']
                    metadata={'id':99,'name':'voice-nats-rollout-'+state['operation']+'-'+binding['challenge'],
                        'url':'https://api.github.com/repos/Poryadok/VoiceRoot/actions/artifacts/99',
                        'workflow_run':{'id':123,'head_sha':'c'*40},'expired':False,'created_at':binding['created_at'],
                        'expires_at':(dt.datetime.now(dt.timezone.utc)+dt.timedelta(days=1)).isoformat(),'size_in_bytes':len(zipped)}
                    def request(url,headers,limit,deadline):
                        if url.endswith('/99'):return http.Reply(200,json.dumps(metadata).encode())
                        if url.endswith('/zip'):return http.Reply(302,headers={'Location':'https://productionresultssa1.blob.core.windows.net/artifact'})
                        order.append('complete-offnode-readback');return http.Reply(200,zipped)
                    with patch.object(github_custody,'_request',side_effect=request):
                        actions.execute({'action':'authorize','operation':state['operation'],'token':'fixture','artifact_id':99})
                authorize_bound(base,state)
                state=actions.state('a'*12)[1]
                self.assertEqual(state['contract']['scripts'][0]['sha256'],hashlib.sha256(b'target').hexdigest())
                self.assertTrue((installed/'active-contract.json').is_file())
                self.assertTrue(all(row['data']['bootstrap.sh']=='target' for row in cms.values()))
                # Guard reads the same actual root-written authorization bytes.
                from apply import paused_documents
                paused=paused_documents(json.loads((base/'apply-manifests.json').read_bytes()),state['authorization'])
                self.assertEqual(paused[0]['spec']['replicas'],0)
                self.assertEqual(paused[0]['spec']['template']['spec']['containers'][0]['image'],target['images']['voice-story/story'])
                with patch.object(guard,'protected_receipt',side_effect=lambda path:protected_receipt(path,root=workspace)):
                    result=actions.execute({'action':'finish','operation':'a'*12,'claim_rv':'2'})
                self.assertEqual(result['status'],'PASS');self.assertEqual(order,['closed-fence','original-cold-cut','complete-offnode-readback','migration-after-custody','prepared-after-config-proof','post-config-store-proof','verified-before-restart','sole-restart'])
                self.assertEqual(json.loads((installed/'active-release.json').read_bytes())['operation'],'a'*12)
                self.assertEqual(json.loads((base/'checkpoint.json').read_bytes())['preservation']['messages'],5)
                previous=actions.state('a'*12)[1];package=transaction.rollback_package(previous)
                self.assertEqual(package['target']['images'],previous['context']['old_images'])
                old_archive=previous['cut']['manifest']['archive_sha256']
                (store/'record.blk').write_bytes((store/'record.blk').read_bytes()+b'known-post-release-record');messages[0]=6
                stage.operation='b'*12;stage.snapshots={'voice-story':copy.deepcopy(wanted)}
                stage.original_snapshots=copy.deepcopy(stage.snapshots);stage.old_images=copy.deepcopy(target['images'])
                order.clear()
                def rollback_producer(request,prior):
                    self.assertEqual(prior['operation'],'a'*12)
                    fresh_actor=copy.deepcopy(actor);image=stage.old_images['voice-story/story']
                    fresh_child,fresh_config,fresh_component=actor_root.STORY_IMAGES[image]
                    fresh_actor['compatible_story_candidate'].update(image=image,child_sha256=fresh_child,config_sha256=fresh_config,component_source_sha=fresh_component)
                    return root_cli.prepare_value(package,code,actions.binding,request['nonce'][:12],previous=prior,nats_authority=authority,actor_authority=fresh_actor,actor_services=['story'])
                with patch.object(actions,'_prepare',side_effect=rollback_producer):
                    fresh=actions.execute({'action':'prepare-rollback','operation':'a'*12,'nonce':'b'*64})
                fresh_base=workspace/('rollout-'+'b'*12)
                self.assertEqual(fresh['cut']['census']['streams'][0]['state']['messages'],6)
                self.assertNotEqual(fresh['cut']['manifest']['archive_sha256'],old_archive)
                self.assertEqual(fresh['rollback_from']['operation'],'a'*12)
                authorize_bound(fresh_base,fresh)
                with patch.object(guard,'protected_receipt',side_effect=lambda path:protected_receipt(path,root=workspace)):
                    self.assertEqual(actions.execute({'action':'finish','operation':'b'*12,'claim_rv':'3'})['status'],'PASS')
                self.assertEqual(json.loads((fresh_base/'checkpoint.json').read_bytes())['preservation']['messages'],6)
                self.assertEqual(json.loads((installed/'active-release.json').read_bytes())['operation'],'b'*12)
                self.assertTrue(all(row['data']['bootstrap.sh']=='target' for row in cms.values()))

if __name__=='__main__':unittest.main()
