"""Approved-source role selection cannot promote caller flags or drifted Secrets."""
import base64,copy,json,tempfile,types,unittest
from pathlib import Path
from unittest.mock import Mock,patch
import actor_root as root

class Tests(unittest.TestCase):
    def test_story_promoted_alias_binds_reviewed_content_not_master_revision(self):
        alias='ghcr.io/poryadok/voiceroot/story@sha256:'+'d'*64
        child,config,component=next(iter(root.STORY_IMAGES.values()))
        chain={'requested_image':alias,'manifest_image':'ghcr.io/poryadok/voiceroot/story@sha256:'+child,'config_sha256':config}
        with patch('source_authority.immutable_image_identity',return_value=chain) as resolve:
            self.assertEqual(root.story_content(alias),(child,config,component))
            self.assertEqual(resolve.call_args.args[0],alias)
            for field,value in (('requested_image','other'),('manifest_image','ghcr.io/poryadok/voiceroot/story@sha256:'+'f'*64),('config_sha256','f'*64)):
                altered=dict(chain);altered[field]=value;resolve.return_value=altered
                with self.subTest(field=field),self.assertRaises(root.Blocked):root.story_content(alias)
            resolve.reset_mock()
            with self.assertRaises(root.Blocked):root.story_content(alias.replace('/story@','/bot@'))
            resolve.assert_not_called()

    def test_actual_capture_then_actor_preflight_binds_exact_leaf_mount(self):
        import runtime_stage
        with tempfile.TemporaryDirectory() as td:
            source=Path(td);kube,fixture,decoder,service,operator,account=self.fixture(source)
            captured_rows={name:{'metadata':{'name':name,'namespace':'voice-staging','uid':name+'-uid','resourceVersion':'1'},'spec':{'template':{'spec':{'containers':[],'volumes':[]}}}} for name in (runtime_stage.HUB,'voice-gateway',*('voice-'+s for s in runtime_stage.LEAVES))}
            captured_rows['voice-social']=fixture.snapshots['voice-social']
            captured_rows[runtime_stage.HUB]['spec']['template']['spec']['volumes']=[{'name':'jsdata','persistentVolumeClaim':{'claimName':'selected'}}]
            marker={'metadata':{'name':runtime_stage.MARKER,'uid':'marker','resourceVersion':'1'},'data':{'phase':'active','generation':'generation','dataPVC':'selected'}}
            claim={'metadata':{'name':'selected','uid':'pvc'},'spec':{'volumeName':'pv'}};pv={'metadata':{'uid':'pv'}}
            original_get=kube.get.side_effect
            def get(kind,name):
                if kind=='deployment':return copy.deepcopy(captured_rows[name])
                if kind=='namespace':return {'metadata':{'uid':'namespace'}}
                if kind=='configmap' and name==runtime_stage.MARKER:return copy.deepcopy(marker)
                if kind=='pvc':return copy.deepcopy(claim)
                if kind=='pv':return copy.deepcopy(pv)
                if kind=='service':return {}
                return original_get(kind,name)
            def cas(kind,row,changes):
                current=marker if kind=='configmap' else captured_rows[row['metadata']['name']]
                self.assertEqual(row,current)
                result=copy.deepcopy(row)
                for change in changes:
                    parts=change['path'].strip('/').split('/');parent=result
                    for part in parts[:-1]:parent=parent.setdefault(part,{})
                    if change['op']=='test':self.assertEqual(parent.get(parts[-1]),change['value'])
                    else:parent[parts[-1]]=change['value']
                result['metadata']['resourceVersion']=str(int(row['metadata']['resourceVersion'])+1)
                if kind=='configmap':marker.clear();marker.update(result)
                else:
                    captured_rows[row['metadata']['name']]=copy.deepcopy(result)
                    # Controller reconciliation follows the scale CAS, before no-pods polling.
                    captured_rows[row['metadata']['name']]['metadata']['resourceVersion']=str(int(result['metadata']['resourceVersion'])+1)
                    captured_rows[row['metadata']['name']]['status']={'replicas':0,'availableReplicas':0}
                return copy.deepcopy(result)
            kube.cas.side_effect=cas
            kube.get.side_effect=get;kube.run.return_value={'items':[]}
            def captured_preflight(stage):stage.snapshots=copy.deepcopy(captured_rows);stage.marker=copy.deepcopy(marker);stage.service={}
            with patch.object(runtime_stage.Staging,'preflight',captured_preflight),patch.object(runtime_stage,'pv_storage_path',return_value='/captured/selected'),patch.object(runtime_stage,'running_image_pins',return_value={}):
                stage=runtime_stage.RolloutStage.capture(kube,'a'*12,lambda event:None)
            self.assertNotIn('secret_refs',stage.expected)
            with patch.object(root,'ACCOUNT_SHA',root.hash_bytes(account.encode())),patch.object(root.actor_verification,'verify',return_value={'verified':'root-produced'}) as verify:
                result=root.preflight(kube,source,source,{'nats-rollout-preservation/bootstrap-renewer':'c'*64},stage,['social'],decoder)
                self.assertEqual(result['mounts']['social']['container'],'nats-leaf')
                self.assertEqual(result['mounts']['social']['key'],'social.creds')
                initial_rv=stage.snapshots['voice-social']['metadata']['resourceVersion']
                self.assertTrue(stage.fence()['verified']) # real maintenance/scale/no-pods/closed sequence
                self.assertNotEqual(stage.snapshots['voice-social']['metadata']['resourceVersion'],initial_rv)
                self.assertNotEqual(captured_rows['voice-social']['metadata']['resourceVersion'],stage.snapshots['voice-social']['metadata']['resourceVersion'])
                root.revalidate(kube,source,source,{'nats-rollout-preservation/bootstrap-renewer':'c'*64},stage,['social'],decoder,result)
                captured_rows['voice-social']['spec']['template']['spec']['unowned']='drift'
                with self.assertRaises(root.Blocked):root.revalidate(kube,source,source,{'nats-rollout-preservation/bootstrap-renewer':'c'*64},stage,['social'],decoder,result)
                captured_rows['voice-social']=copy.deepcopy(stage.snapshots['voice-social'])
                for field in ('replicas','uid','namespace','credential'):
                    live=copy.deepcopy(captured_rows['voice-social'])
                    if field=='replicas':captured_rows['voice-social']['spec']['replicas']=1
                    elif field=='credential':captured_rows['voice-social']['spec']['template']['spec']['volumes'][0]['secret']['secretName']='unowned'
                    else:captured_rows['voice-social']['metadata'][field]='unowned'
                    with self.subTest(live_drift=field),self.assertRaises(root.Blocked):root.revalidate(kube,source,source,{'nats-rollout-preservation/bootstrap-renewer':'c'*64},stage,['social'],decoder,result)
                    captured_rows['voice-social']=live
                baseline=copy.deepcopy(stage.snapshots['voice-social'])
                for field in ('container','env','key','path','readonly','namespace','uid'):
                    stage.snapshots['voice-social']=copy.deepcopy(baseline);row=stage.snapshots['voice-social'];spec=row['spec']['template']['spec'];leaf=spec['containers'][0]
                    if field=='container':leaf['name']='other'
                    elif field=='env':leaf['env'][0]['value']='/other/social.creds'
                    elif field=='key':spec['volumes'][0]['secret']['items'][0]['key']='space.creds'
                    elif field=='path':leaf['volumeMounts'][0]['subPath']='space.creds'
                    elif field=='readonly':leaf['volumeMounts'][0]['readOnly']=False
                    else:row['metadata'][field]='other'
                    verify.reset_mock()
                    with self.subTest(field=field),self.assertRaises(root.Blocked):root.preflight(kube,source,source,{'nats-rollout-preservation/bootstrap-renewer':'c'*64},stage,['social'],decoder)
                    verify.assert_not_called()

    def fixture(self,source,role='social'):
        path=source/root.ACL;path.parent.mkdir(parents=True);path.write_text('version: 1\nservices:\n  social:\n    no_response: true\n    publish:\n      - social.friend_removed\n    subscribe:\n      - _INBOX.voice.social.>\n')
        public='U'+'A'*55;account='A'+'B'*55
        token='header.'+base64.urlsafe_b64encode(json.dumps({'sub':public,'name':'voice-'+role}).encode()).decode().rstrip('=')+'.signature'
        creds=('-----BEGIN NATS USER JWT-----\n'+token+'\n------END NATS USER JWT------\n').encode()
        def secret(name,data):return {'metadata':{'name':name,'uid':name+'-uid','resourceVersion':'1'},'data':{k:base64.b64encode(v).decode() for k,v in data.items()}}
        service=secret('voice-nats-service-credentials-generation',{role+'.creds':creds})
        operator=secret('voice-nats-operator-generation',{'account.public':account.encode(),'account.jwt':b'account','operator.jwt':b'operator'})
        database=secret('voice-app-secrets',{'STORY_DATABASE_URL':b'postgres://voice:synthetic@voice-postgres:5432/story_db?sslmode=disable','POSTGRES_PASSWORD':b'synthetic'})
        config={'metadata':{'name':'voice-app-config','uid':'config-uid','resourceVersion':'1'},'data':{'POSTGRES_USER':'voice'}}
        kube=Mock();kube.get.side_effect=lambda kind,name:copy.deepcopy(database if name=='voice-app-secrets' else config if name=='voice-app-config' else service if name==service['metadata']['name'] else operator)
        stage=types.SimpleNamespace(expected={'generation':'generation','deployment_uids':{'voice-story':'story-uid','voice-'+role:role+'-uid'}})
        stage.snapshots={'voice-story':{'metadata':{'name':'voice-story','namespace':'voice-staging','uid':'story-uid'},'spec':{'template':{'spec':{'containers':[{'name':'story','env':[{'name':'DATABASE_URL','valueFrom':{'secretKeyRef':{'name':'voice-app-secrets','key':'STORY_DATABASE_URL'}}}]}]}}}}}
        if role!='story':stage.snapshots['voice-'+role]={'metadata':{'name':'voice-'+role,'namespace':'voice-staging','uid':role+'-uid','resourceVersion':'1'},'spec':{'template':{'spec':{'containers':[]}}}}
        else:stage.expected['deployment_uids']['voice-story']='story-uid'
        snapshot=stage.snapshots['voice-'+role];snapshot['metadata']['resourceVersion']='1';spec=snapshot['spec']['template']['spec'];key=role+'.creds'
        spec['containers'].append({'name':'nats-leaf','env':[{'name':'NATS_CREDS','value':'/var/run/nats/creds/'+key}],'volumeMounts':[{'name':'nats-service-creds','mountPath':'/var/run/nats/creds/'+key,'subPath':key,'readOnly':True}]})
        spec['volumes']=[{'name':'nats-service-creds','secret':{'secretName':service['metadata']['name'],'defaultMode':256,'items':[{'key':key,'path':key}]}}]
        decoder=lambda text:[{'version':1,'services':{'social':{'publish':['social.friend_removed'],'subscribe':['_INBOX.voice.social.>'],'no_response':True}}}]
        stage.old_images={'voice-story/story':next(iter(root.STORY_IMAGES))}
        previous_get=kube.get.side_effect
        kube.get.side_effect=lambda kind,name:copy.deepcopy(stage.snapshots[name]) if kind=='deployment' else previous_get(kind,name)
        def image_chain(image,deadline):
            child,config,_=root.STORY_IMAGES.get(image,('f'*64,'f'*64,'f'*40))
            return {'requested_image':image,'manifest_image':'ghcr.io/poryadok/voiceroot/story@sha256:'+child,'config_sha256':config}
        chain_patch=patch('source_authority.immutable_image_identity',side_effect=image_chain);chain_patch.start();self.addCleanup(chain_patch.stop)
        return kube,stage,decoder,service,operator,account

    def test_story_candidate_requires_actual_mounted_actor_and_canonical_language(self):
        canonical=(Path(__file__).resolve().parents[3]/root.ACL).read_bytes()
        with tempfile.TemporaryDirectory() as td:
            source=Path(td);kube,stage,decoder,service,operator,account=self.fixture(source,'story')
            (source/root.ACL).write_bytes(canonical)
            migrations=Path(__file__).resolve().parents[3]/'src/backend/migrations/story_db'
            for path in migrations.glob('*.up.sql'):
                destination=source/path.relative_to(Path(__file__).resolve().parents[3])
                destination.parent.mkdir(parents=True,exist_ok=True);destination.write_bytes(path.read_bytes())
            kube.run.return_value={'database':'story_db','version':4,'dirty':False,'username':'voice'}
            with patch.object(root,'ACCOUNT_SHA',root.hash_bytes(account.encode())),patch('space_authority.capture',return_value={'root-pg':'bound'}),patch.object(root.actor_verification,'verify',return_value={'server_authentication_verified':True}) as verify:
                result=root.preflight(kube,source,source,{'nats-rollout-preservation/bootstrap-renewer':'c'*64},stage,['story'],decoder)
                self.assertIsNotNone(result,'candidate Story actor cannot be skipped')
                self.assertEqual(result['roles'],['story'])
                self.assertEqual(result['story_schema']['observation'],{k:v for k,v in kube.run.return_value.items() if k!='username'})
                self.assertIn('database_url_sha256',result['story_schema'])
                self.assertEqual(len(result['story_schema']['migration_hashes']),4)
                required=verify.call_args.args[7]
                self.assertEqual(set(required['pub']),{'story.created','story.viewed','story.reacted','story.expired','story.highlight_created','story.lfp_created','story.lfp_response','$JS.API.STREAM.INFO.story_events'})
                self.assertEqual(required['sub'],['_INBOX.voice.story.>'])
                stage.snapshots['voice-story']['spec']['template']['spec']['volumes'][0]['secret']['secretName']='other'
                with self.assertRaises(root.Blocked):root.preflight(kube,source,source,{'nats-rollout-preservation/bootstrap-renewer':'c'*64},stage,['story'],decoder)
                stage.snapshots['voice-story']['spec']['template']['spec']['volumes'][0]['secret']['secretName']=service['metadata']['name']
                verify.reset_mock();kube.run.return_value={'database':'story_db','version':4,'dirty':True}
                with self.assertRaises(root.Blocked):root.preflight(kube,source,source,{'nats-rollout-preservation/bootstrap-renewer':'c'*64},stage,['story'],decoder)
                verify.assert_not_called()

    def test_source_requirements_and_actual_mount_bound_before_fence(self):
        with tempfile.TemporaryDirectory() as td:
            source=Path(td);kube,stage,decoder,service,operator,account=self.fixture(source)
            with patch.object(root,'ACCOUNT_SHA',root.hash_bytes(account.encode())),patch.object(root.actor_verification,'verify',return_value={'verified':'root-produced'}) as verify:
                result=root.preflight(kube,source,source,{'nats-rollout-preservation/bootstrap-renewer':'c'*64},stage,['social','user'],decoder)
                self.assertEqual(result['roles'],['social'])
                self.assertEqual(verify.call_args.args[7],{'pub':['social.friend_removed'],'sub':['_INBOX.voice.social.>']})
                stage.snapshots['voice-social']['spec']['template']['spec']['volumes'][0]['secret']['secretName']='other'
                with self.assertRaises(root.Blocked):root.preflight(kube,source,source,{'nats-rollout-preservation/bootstrap-renewer':'c'*64},stage,['social'],decoder)

    def test_secret_drift_or_different_role_identity_refuses_revalidation(self):
        with tempfile.TemporaryDirectory() as td:
            source=Path(td);kube,stage,decoder,service,operator,account=self.fixture(source)
            with patch.object(root,'ACCOUNT_SHA',root.hash_bytes(account.encode())),patch.object(root.actor_verification,'verify',return_value={'verified':'root-produced'}):
                result=root.preflight(kube,source,source,{'nats-rollout-preservation/bootstrap-renewer':'c'*64},stage,['social'],decoder)
                service['metadata']['resourceVersion']='2'
                with self.assertRaises(root.Blocked):root.revalidate(kube,source,source,{'nats-rollout-preservation/bootstrap-renewer':'c'*64},stage,['social'],decoder,result)
                raw=root.secret_bytes(service,'social.creds')
                with self.assertRaises(root.Blocked):root.public_user(raw,'space')

    def test_story_schema_source_version_and_root_identity_drift_veto(self):
        original=Path(__file__).resolve().parents[3]
        with tempfile.TemporaryDirectory() as td:
            source=Path(td);kube,stage,*_=self.fixture(source,'story')
            for path in (original/'src/backend/migrations/story_db').glob('*.up.sql'):
                destination=source/path.relative_to(original);destination.parent.mkdir(parents=True,exist_ok=True)
                destination.write_bytes(path.read_bytes())
            clean={'database':'story_db','version':4,'dirty':False,'username':'voice'}
            with patch('space_authority.capture',return_value={'pod':'root-bound'}):
                for observed in (dict(clean,version=5),dict(clean,version=True),dict(clean,dirty=True),
                                 dict(clean,database='bot_db'),dict(clean,extra=0)):
                    kube.run.return_value=observed
                    with self.subTest(observed=observed),self.assertRaises(root.Blocked):root.story_schema(kube,source,stage)
            kube.run.return_value=clean
            with patch('space_authority.capture',side_effect=[{'pod':'old'},{'pod':'new'}]):
                with self.assertRaises(root.Blocked):root.story_schema(kube,source,stage)
            path=source/'src/backend/migrations/story_db/000001_init.up.sql';path.write_bytes(path.read_bytes()+b'\n')
            kube.run.reset_mock()
            with self.assertRaises(root.Blocked):root.story_schema(kube,source,stage)
            kube.run.assert_not_called()

    def test_unaffected_user_has_no_hidden_credential_or_actor_mutation(self):
        kube=Mock()
        self.assertIsNone(root.preflight(kube,Path('/unused'),Path('/unused'),{},None,['user'],None))
        kube.get.assert_not_called()

    def test_story_database_reference_dsn_and_secret_reread_veto(self):
        original=Path(__file__).resolve().parents[3]
        for scenario in ('reference','namespace','container','missing','uid','dsn-host','dsn-database','dsn-password','username','secret-drift'):
            with self.subTest(scenario=scenario),tempfile.TemporaryDirectory() as td:
                source=Path(td);kube,stage,*_=self.fixture(source,'story')
                for path in (original/'src/backend/migrations/story_db').glob('*.up.sql'):
                    destination=source/path.relative_to(original);destination.parent.mkdir(parents=True,exist_ok=True)
                    destination.write_bytes(path.read_bytes())
                kube.run.return_value={'database':'story_db','version':4,'dirty':False,'username':'voice'}
                if scenario=='reference':stage.snapshots['voice-story']['spec']['template']['spec']['containers'][0]['env'][0]['valueFrom']['secretKeyRef']['key']='OTHER_DATABASE_URL'
                if scenario=='namespace':stage.snapshots['voice-story']['metadata']['namespace']='another-namespace'
                if scenario=='uid':stage.snapshots['voice-story']['metadata']['uid']='another-uid'
                if scenario=='container':stage.snapshots['voice-story']['spec']['template']['spec']['containers'][0]['name']='another-container'
                if scenario=='missing':del stage.snapshots['voice-story']
                if scenario=='username':kube.run.return_value['username']='another_role'
                existing=kube.get.side_effect;reads=[]
                def changed(kind,name):
                    row=existing(kind,name)
                    if name=='voice-app-secrets':
                        reads.append(name)
                        if scenario.startswith('dsn-'):
                            raw=base64.b64decode(row['data']['STORY_DATABASE_URL'])
                            before,after={'dsn-host':(b'@voice-postgres',b'@foreign-host'),'dsn-database':(b'/story_db',b'/bot_db'),'dsn-password':(b':synthetic@',b':different@')}[scenario]
                            row['data']['STORY_DATABASE_URL']=base64.b64encode(raw.replace(before,after)).decode()
                        if scenario=='secret-drift' and len(reads)>1:row['metadata']['resourceVersion']='2'
                    return row
                kube.get.side_effect=changed
                with patch('space_authority.capture',return_value={'pod':'root-bound'}):
                    with self.assertRaises(root.Blocked):root.story_schema(kube,source,stage)
                if scenario not in ('username','secret-drift'):kube.run.assert_not_called()

    def test_real_canonical_acl_subset_and_unsupported_yaml_refusal(self):
        source=globals().get('CANONICAL_ACL')
        if source is None:source=(Path(__file__).resolve().parents[3]/root.ACL).read_bytes()
        grants=root.source_grants(source)
        self.assertIn('space.restored',grants['space']['publish'])
        self.assertIn('$JS.API.CONSUMER.INFO.social_events.rt_realtime1_friend_removed',grants['realtime']['publish'])
        for raw in (b'version: 1\nservices: &alias\n',b'version: 1\nservices:\n  social:\n    publish: [wildcard]\n',source+b'\nservices:\n',source+b'\nbootstrap:\n'):
            with self.assertRaises(root.Blocked):root.source_grants(raw)

if __name__=='__main__':unittest.main()
