import copy
import base64
import hashlib
import unittest
from unittest.mock import Mock,patch
import migrations
from controller import Blocked

def plan():
    sql='CREATE TABLE IF NOT EXISTS example (id bigint PRIMARY KEY);\n'
    return [{'database':'social','image':migrations.IMAGE,'secret_key':'SOCIAL_DATABASE_URL',
             'files':{'000001_example.up.sql':sql},
             'hashes':{'000001_example.up.sql':hashlib.sha256(sql.encode()).hexdigest()}}]

class MigrationPlanTest(unittest.TestCase):
    def test_emits_only_nonce_database_job_and_sql_configmap(self):
        rows=migrations.documents(migrations.validate_plan(plan(),'app-only'),'123456abcdef')
        self.assertEqual([r['kind'] for r in rows],['ConfigMap','Job'])
        pod=rows[1]['spec']['template']['spec']
        self.assertFalse(pod['automountServiceAccountToken'])
        self.assertNotIn('persistentVolumeClaim',str(pod))
        self.assertEqual(pod['containers'][0]['image'],migrations.IMAGE)
        self.assertEqual(pod['containers'][0]['env'][0]['valueFrom']['secretKeyRef'],
                         {'name':'voice-app-secrets','key':'SOCIAL_DATABASE_URL'})
    def test_images_only_requires_explicit_empty_migration_plan(self):
        with self.assertRaises(Blocked):migrations.validate_plan(plan(),'images-only')
        self.assertEqual(migrations.validate_plan([],'images-only'),[])
    def test_auth_uses_canonical_repair_migrate_and_existing_authority(self):
        sql='CREATE TABLE example (id bigint);\n'
        row={'database':'auth','image':migrations.FLYWAY_IMAGE,'secret_key':'POSTGRES_PASSWORD',
             'files':{'V1__example.sql':sql},'hashes':{'V1__example.sql':hashlib.sha256(sql.encode()).hexdigest()}}
        cm,job=migrations.documents(migrations.validate_plan([row],'app-only'),'123456abcdef')
        c=job['spec']['template']['spec']['containers'][0]
        self.assertEqual(c['args'],['repair','migrate']);self.assertEqual(c['image'],migrations.FLYWAY_IMAGE)
        self.assertEqual(c['env'][2]['valueFrom']['secretKeyRef']['name'],'voice-app-secrets')
    def test_sql_digest_drift_rejected(self):
        row=plan();row[0]['files']['000001_example.up.sql']+='DROP TABLE example;'
        with self.assertRaises(Blocked):migrations.validate_plan(row,'full')
    def test_untrusted_names_duplicate_db_and_image_veto(self):
        for mutation in ('name','duplicate','image','secret'):
            with self.subTest(mutation=mutation):
                rows=plan()
                if mutation=='name':rows[0]['files']={'../escape.sql':'x'}
                if mutation=='duplicate':rows+=copy.deepcopy(rows)
                if mutation=='image':rows[0]['image']='migrate/migrate:latest'
                if mutation=='secret':rows[0]['secret_key']='NATS_OPERATOR_CREDS'
                with self.assertRaises(Blocked):migrations.validate_plan(rows,'app-only')

    def test_metadata_secret_veto_precedes_any_job_create(self):
        kube=Mock();kube.secret_meta.return_value={'uid':'old','resourceVersion':'100'}
        kube.run.return_value=True
        stage=Mock();stage.operation='123456abcdef'
        with self.assertRaises(Blocked):
            migrations.execute(kube,stage,plan(),'full',{'uid':'old','resourceVersion':'99'},lambda e:None)
        self.assertFalse(any(c.args[0][0]=='create' for c in kube.run.call_args_list))

    def test_bot_forward_collision_recheck_precedes_any_migration_create(self):
        import bot_migration
        sql='CREATE UNIQUE INDEX example ON bot_event_log(bot_id,interaction_token);'
        row={'database':'bot','image':migrations.IMAGE,'secret_key':'BOT_DATABASE_URL',
             'files':{'000004_slash_interaction_outbox.up.sql':sql},
             'hashes':{'000004_slash_interaction_outbox.up.sql':hashlib.sha256(sql.encode()).hexdigest()}}
        kube=Mock();kube.secret_meta.return_value={'uid':'old','resourceVersion':'100'}
        kube.run.return_value=True
        kube.get.return_value={'metadata':{'uid':'config','resourceVersion':'200'},'data':{'POSTGRES_USER':'voice'}}
        stage=Mock();stage.operation='123456abcdef'
        with patch('bot_migration.capture',side_effect=bot_migration.PrerequisiteError('collision')):
            with self.assertRaisesRegex(Blocked,'bot_forward_constraint_prerequisite_failed'):
                migrations.execute(kube,stage,[row],'full',{},lambda e:None)
        self.assertFalse(any(c.args[0][0]=='create' for c in kube.run.call_args_list))
        stage.verify_final_storage.assert_not_called()

    def test_owned_fence_loss_precedes_any_job_create(self):
        kube=Mock();meta={'uid':'old','resourceVersion':'100'};kube.secret_meta.return_value=meta
        kube.run.return_value=True
        stage=Mock();stage.operation='123456abcdef';stage.verify_final_storage.side_effect=Blocked('ownership_changed')
        with self.assertRaises(Blocked):migrations.execute(kube,stage,plan(),'full',meta,lambda e:None)
        self.assertFalse(any(c.args[0][0]=='create' for c in kube.run.call_args_list))

    def test_completed_job_uid_must_match_created_job(self):
        kube=Mock();meta={'uid':'old','resourceVersion':'100'};kube.secret_meta.return_value=meta
        cm,job=migrations.documents(plan(),'123456abcdef')
        for row in (cm,job):row['metadata']=dict(row['metadata'],uid='aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa')
        kube.run.side_effect=[True,[cm['metadata']['name'],cm['metadata']['uid']],
                              [job['metadata']['name'],job['metadata']['uid']],
                              {'metadata':{'uid':'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb'},'status':{'succeeded':1}}]
        stage=Mock();stage.operation='123456abcdef'
        with self.assertRaises(Blocked):migrations.execute(kube,stage,plan(),'full',meta,lambda e:None)
        self.assertEqual(sum(c.args[0][0]=='create' for c in kube.run.call_args_list),2)

    def test_legacy_copy_url_encodes_existing_authority_without_mutating_source(self):
        metadata={'uid':'secret-uid','resourceVersion':'100','app_config':{'uid':'config-uid','resourceVersion':'200'}}
        secret={'metadata':{'uid':'secret-uid','resourceVersion':'100'},
                'data':{'POSTGRES_PASSWORD':base64.b64encode(b'disposable:password@/').decode()}}
        original=copy.deepcopy(secret)
        kube=Mock();kube.get.side_effect=[secret,{'metadata':metadata['app_config'],'data':{'POSTGRES_USER':'voice'}}]
        row=plan();row[0]['database']='bot';row[0]['secret_key']='BOT_DATABASE_URL'
        result=migrations.legacy_credentials(kube,row,'123456abcdef',metadata)
        self.assertTrue(result['immutable']);self.assertEqual(result['metadata']['name'],'voice-rollout-123456abcdef-database-urls')
        self.assertIn('disposable%3Apassword%40%2F',base64.b64decode(result['data']['BOT_DATABASE_URL']).decode())
        self.assertEqual(secret,original);kube.run.assert_not_called()

    def test_legacy_copy_vetoes_source_secret_uid_reuse(self):
        kube=Mock();kube.get.return_value={'metadata':{'uid':'new','resourceVersion':'100'}}
        row=plan();row[0]['database']='bot'
        with self.assertRaises(Blocked):migrations.legacy_credentials(kube,row,'123456abcdef',{'uid':'old','resourceVersion':'100'})
        kube.run.assert_not_called()

if __name__=='__main__':unittest.main()
