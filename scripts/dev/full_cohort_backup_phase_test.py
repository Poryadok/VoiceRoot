"""Fixed full-cohort upload route; external ledger/source are synthetic."""
import copy
import json
import os
from pathlib import Path
import tempfile
import unittest
from types import SimpleNamespace
from unittest.mock import patch

import cohort_upload_runner as runner
import cohort_upload_run as upload
import media_operation_journal as journals
import story_media_backup as backup


class FixedUploadRouteTest(unittest.TestCase):
    def test_open_fixed_journal_and_refuse_missing_closed_or_changed_route(self):
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp);operation='media-c2-'+'a'*32;source='b'*40;challenge='c'*32
            op=root/operation;op.mkdir(mode=0o700)
            context={'operation_id':operation,'commit':source,'project':'voice','resource':'DEV','actor':'scheduled_devops',
                     'claim':'claim','window':'window','generation':1,'revision':1}
            binding={'operation_id':operation,'snapshot_id':'snapshot','source_sha':source,'project':'voice','resource':'DEV',
                     'actor':'scheduled_devops','claim':'claim','window':'window','generation':1,'revision':1,
                     'databases':['voice_db'],'sources':{'roles':{'engine':{'namespace_uid':'namespace'}}},'full_catalog':{}}
            intent={'resource_context':context,'namespace_uid':'namespace','kube_lock':{'fixture':True},
                    'cohort_route_mode':'full-cohort-v1','cohort_binding':binding}
            root_journal=journals.Journal(op/'journal',intent);self.addCleanup(root_journal.close)
            backup_journal=journals.Journal(op/'backup-journal',intent);self.addCleanup(backup_journal.close)
            planned={'status':'dispatch-pending','operation_id':operation,'snapshot_id':'snapshot','workflow_sha':source,'source_sha':source,
                     'challenge':challenge,'members':['roles','voice_db'],'invocation_nonce':'d'*32,
                     'bundle_bytes':1,'bundle_sha256':'e'*64,'manifest_sha256':'f'*64,'receipt_sha256':{}}
            marker={'status':'done','operation_id':operation,'snapshot_id':'snapshot','invocation_nonce':'d'*32,
                    'bundle_bytes':1,'bundle_sha256':'e'*64,'manifest_sha256':'f'*64,'receipt_sha256':{}}
            for journal in (root_journal,backup_journal):
                journal.state['steps'].update({'cohort-upload':planned,'cohort-dispatch':upload.dispatch_marker(planned),
                                               'cohort-bundle':{'status':'done','marker':marker}})
                journal.save()
            row={'status':'active','full_cohort_backup_route':runner.route_marker(binding,'namespace')}
            fence=SimpleNamespace(ledger=SimpleNamespace(read=lambda:(copy.deepcopy(row),('uid','rv'))),owned=lambda value:True)
            env={'GITHUB_SHA':source}
            with patch.object(runner,'ROOT',root),patch.object(runner,'OPERATOR_UID',os.geteuid()), \
                 patch.object(runner.publish_candidate,'source_snapshot'),patch.object(runner.media,'Client'), \
                 patch.object(runner.resource_fencing,'ResourceCheck'),patch.object(runner.resource_fencing,'Fence',return_value=fence), \
                 patch.object(runner.media_operation_guard,'verify'),patch.object(runner.finalize,'validate_binding',return_value={'roles','voice_db'}):
                loaded=runner.load(env,operation,challenge,root)
                self.assertEqual(loaded[10],op/'backup-journal')
                row['full_cohort_backup_route']['binding_sha256']='0'*64
                with self.assertRaises(runner.RunnerError):runner.load(env,operation,challenge,root)
                row['full_cohort_backup_route']=runner.route_marker(binding,'namespace')
                backup_journal.close()
                with self.assertRaises(runner.RunnerError):runner.load(env,operation,challenge,root)
                backup_journal=journals.Journal(op/'backup-journal',intent);self.addCleanup(backup_journal.close)
                (op/'backup-journal'/'state.json').unlink()
                with self.assertRaises((runner.RunnerError,OSError,backup.BackupError)):runner.load(env,operation,challenge,root)
                row['full_cohort_backup_route']=None
                with self.assertRaises(runner.RunnerError):runner.load(env,operation,challenge,root)
                del row['full_cohort_backup_route']
                with self.assertRaises(runner.RunnerError):runner.load(env,operation,challenge,root)

    def test_real_claim_derives_full_members_from_held_journals(self):
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp);operation='media-c2-'+'a'*32;source='b'*40;challenge='c'*32
            op=root/operation;op.mkdir(mode=0o700);uid='00000000-0000-0000-0000-000000000001'
            context={'operation_id':operation,'commit':source,'project':'voice','resource':'DEV','actor':'scheduled_devops',
                     'claim':uid,'window':uid,'generation':1,'revision':1}
            engine={k:uid for k in ('namespace_uid','pod_uid','pvc_uid','pv_uid')}
            engine.update(container_id='containerd://'+'d'*64,restart_count=0)
            databases=['custom_template','postgres','template1','voice_db'];members=sorted(databases+['roles'])
            sources={'roles':{'engine':engine,'server_start':'2026-01-01T00:00:00Z'}}
            sources.update({db:{'engine':engine,'server_start':'2026-01-01T00:00:00Z','database_oid':100+i} for i,db in enumerate(databases)})
            inputs={k:'e'*64 for k in ('catalog_sha256','roles_catalog_sha256','migration_catalog_sha256','migrate_binary_sha256')}
            inputs.update(postgres_image='sha256:'+'f'*64,migrate_image='sha256:'+'f'*64,postgres_major=16)
            binding={'operation_id':operation,'snapshot_id':'capture-'+'1'*32,'source_sha':source,
                     **{k:context[k] for k in ('project','resource','actor','claim','window','generation','revision')},
                     'databases':databases,'sources':sources,'restore_inputs':inputs,
                     'full_catalog':{'receipt_sha256':'9'*64,'properties':{db:{'allowconn':True} for db in databases}}}
            intent={'resource_context':context,'namespace_uid':uid,'kube_lock':{'fixture':True},
                    'cohort_route_mode':'full-cohort-v1','cohort_binding':binding}
            opened={}
            for name in ('journal','backup-journal'):
                obj=journals.Journal(op/name,intent);self.addCleanup(obj.close);opened[name]=obj
            receipt_hashes={m:'8'*64 for m in members}
            planned={'status':'dispatch-pending','operation_id':operation,'snapshot_id':binding['snapshot_id'],
                     'source_sha':source,'workflow_sha':source,'invocation_nonce':'2'*32,'challenge':challenge,
                     'bundle_bytes':1,'bundle_sha256':'3'*64,'members':members,'receipt_sha256':receipt_hashes,
                     'manifest_sha256':'4'*64}
            marker={'status':'done','operation_id':operation,'snapshot_id':binding['snapshot_id'],'invocation_nonce':'2'*32,
                    'bundle_bytes':1,'bundle_sha256':'3'*64,'manifest_sha256':'4'*64,'receipt_sha256':receipt_hashes}
            opened['backup-journal'].state['steps'].update({'cohort-upload':planned,
                'cohort-dispatch':upload.dispatch_marker(planned),'cohort-bundle':{'status':'done','marker':marker}})
            opened['backup-journal'].save()
            ledger_members={m:{'status':'done','nonce':format(i+1,'032x'),'snapshot_id':binding['snapshot_id'],
                               'operation_id':operation,'receipt_sha256':'8'*64} for i,m in enumerate(members)}
            row={'status':'active','full_cohort_backup_route':runner.route_marker(binding,uid),
                 'backup_invocation':{'nonce':'2'*32,'operation_id':operation},'cohort_upload':planned,
                 'cohort_dispatch':upload.dispatch_marker(planned),'cohort_bundle':marker,'cohort_members':ledger_members}
            class Ledger:
                def read(self):return copy.deepcopy(row),('uid','rv')
                def write(self,revision,value):row.clear();row.update(copy.deepcopy(value))
            fence=SimpleNamespace(context=context,ledger=Ledger(),owned=lambda value:True,authority=lambda:None,uncertain=lambda reason:None)
            env={'GITHUB_REPOSITORY':upload.REPOSITORY,'GITHUB_EVENT_NAME':'workflow_dispatch','GITHUB_REF':'refs/heads/master',
                 'GITHUB_WORKFLOW_REF':upload.WORKFLOW_REF,'GITHUB_SHA':source,'GITHUB_WORKFLOW_SHA':source,
                 'GITHUB_RUN_ID':'123','GITHUB_RUN_ATTEMPT':'1'}
            with patch.object(runner,'ROOT',root),patch.object(upload.media_operation_guard,'verify'):
                with self.assertRaises(upload.CohortRunError):
                    upload.claim(fence,None,intent['kube_lock'],uid,env,challenge,members+['unexpected'])
                selected=op/'backup-journal'/'state.json';original=selected.read_bytes()
                for field in ('source_sha','actor','namespace_uid','snapshot_id'):
                    state=json.loads(original);foreign=state['intent']['cohort_binding']
                    if field=='namespace_uid':foreign['sources']['roles']['engine']['namespace_uid']='00000000-0000-0000-0000-000000000002'
                    else:foreign[field]='f'*40 if field=='source_sha' else 'capture-'+'f'*32 if field=='snapshot_id' else 'foreign'
                    selected.write_text(json.dumps(state),encoding='utf8')
                    row['full_cohort_backup_route']=runner.route_marker(foreign,uid)
                    with self.subTest(field=field),self.assertRaises(upload.CohortRunError):
                        upload.claim(fence,None,intent['kube_lock'],uid,env,challenge,members)
                selected.write_bytes(original);row['full_cohort_backup_route']=runner.route_marker(binding,uid)
                old=row.pop('full_cohort_backup_route')
                with self.assertRaises(upload.CohortRunError):upload.claim(fence,None,intent['kube_lock'],uid,env,challenge,members)
                row['full_cohort_backup_route']=old
                self.assertEqual(upload.claim(fence,None,intent['kube_lock'],uid,env,challenge,members)['run_id'],'123')


if __name__ == '__main__': unittest.main()
