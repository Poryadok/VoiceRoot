import copy
import hashlib
import json
import tempfile
import unittest
from unittest.mock import patch
from pathlib import Path
import operator_retirement as retirement
import installer
from controller import Blocked


class MediaAbortTest(unittest.TestCase):
    def test_complete_receipt_and_retained_bytes_are_required_for_idle_admission(self):
        def encoded(value):return json.dumps(value,sort_keys=True,separators=(',',':')).encode()
        with tempfile.TemporaryDirectory() as td:
            root=Path(td);base=root/('rollout-'+retirement.MEDIA_ABORT_OPERATION);base.mkdir()
            prefix=retirement.MEDIA_ABORT_PREFIX
            names=retirement.CORE|{'voice-web','voice-admin'}
            originals={n:{'metadata':{'uid':n},'spec':{'template':{'spec':{'containers':[{'name':'app','image':'old'}]}}}} for n in names}
            context={'original_snapshots':originals,'snapshots':copy.deepcopy(originals),
                'old_images':{n+'/app':'example.invalid/app@sha256:'+'a'*64 for n in names},
                'marker':{'metadata':{'uid':'marker','resourceVersion':'1'},'data':{'phase':'rollout-verified','knownRolloutOperation':retirement.MEDIA_ABORT_OPERATION}},
                'expected':{'namespace_uid':'ns','source_claim':'pvc','source_claim_uid':'claim','source_pv_uid':'pv'},
                'final_claim':{'spec':{}},'final_pv':{'spec':{}},'final_path':'/private/pvc',
                'service':{'metadata':{'uid':'service'}}}
            snapshot={'operation':retirement.MEDIA_ABORT_OPERATION,'phase':'RESTART','status':'BLOCKED','fence_status':'VERIFIED',
                'error':'restart_readiness_timeout','events':[],'migration_jobs':[{'name':str(i)} for i in range(16)],
                'active_contract':{'contract':'retained'},'preservation':{'native_files_verified':True,'archive':'post/archive.tar'},
                'nats_migration':{'verified':True},'code_capture':{},'context':context}
            original=encoded(snapshot);active=retirement.canonical(snapshot['active_contract']);note=b'compatibility'
            def write(name,value):
                path=base/name;path.parent.mkdir(parents=True,exist_ok=True);path.write_bytes(value);path.chmod(0o600)
            write(prefix+'-snapshot.json',original)
            roles=set(retirement.LEAVES)|{'gateway'}
            setup={n:{'uid':n,'template_sha256':retirement.canonical(t),'replicas':1,'ready':True}
                for n,t in retirement.media_abort_templates(snapshot).items()}
            proof={'setup':setup,'storage':retirement.precut_retained_setup(snapshot),'active_contract_sha256':active,
                'migration_jobs_sha256':retirement.canonical(snapshot['migration_jobs']),'no_migration_replay':True,
                'actors':{'roles':sorted(roles),'proofs':{s:{'proof':{k:True for k in ('signed_chain_verified',
                    'server_authentication_verified','signed_effective_permissions_verified','broker_grant_compatibility_verified')}} for s in roles}},
                'leaf_connections':{s:{'connected':True,'remote_server_sha256':'a'*64} for s in retirement.LEAVES}}
            values={name:b'retained' for name in ('rollout-before.tar','rollout-before-manifest.json','copy-checkpoint.json',
                'rollout-backup.cms','decrypted-restore.json','apply-manifests.json','apply-authorization.json','post/archive.tar')}
            values.update({prefix+'-compatibility.md':note,prefix+'-active-contract.json':encoded(snapshot['active_contract']),prefix+'-verified.json':encoded(proof)})
            for name,value in values.items():write(name,value)
            receipt={'schema':'voice-media-original-restore-closure-v1','operation':retirement.MEDIA_ABORT_OPERATION,
                'waiver':retirement.WAIVER,'snapshot_sha256':hashlib.sha256(original).hexdigest(),'compatibility_sha256':hashlib.sha256(note).hexdigest(),
                'deployment_pass':False,'rollback_eligible':False,'configuration_basis':'ORIGINAL_APPLICATION_IMAGES_RETAIN_MIGRATED_NATS_AND_DATABASES',
                'migration_jobs_sha256':retirement.canonical(snapshot['migration_jobs']),'active_contract_sha256':active,'setup':setup,
                'released_marker':{'uid':'marker','resourceVersion':'2','data':{'phase':'active'}},
                'retained_files':{n:{'sha256':hashlib.sha256(v).hexdigest(),'bytes':len(v)} for n,v in values.items()},'journal':{'absent':True}}
            with patch.object(retirement.guard,'ROOT',root),patch.object(retirement,'MEDIA_ABORT_CHECKPOINT',receipt['snapshot_sha256']),\
                 patch.object(retirement,'MEDIA_ABORT_COMPATIBILITY',receipt['compatibility_sha256']),patch.object(retirement,'MEDIA_ABORT_ACTIVE',active),\
                 patch.object(retirement.root_cli,'code_binding',return_value={}),patch.object(retirement,'media_abort_journal',return_value={'absent':True}):
                write(prefix+'-receipt.json',encoded(receipt));terminal=retirement.media_abort_terminal(snapshot,receipt)
                self.assertEqual(retirement.verify_media_abort_terminal(base,terminal),receipt)
                for field,value in [('deployment_pass',True),('rollback_eligible',True),('configuration_basis','TARGET')]:
                    changed=copy.deepcopy(receipt);changed[field]=value;write(prefix+'-receipt.json',encoded(changed))
                    with self.subTest(field=field),self.assertRaises(Blocked):retirement.verify_media_abort_terminal(base,retirement.media_abort_terminal(snapshot,changed))
                write(prefix+'-receipt.json',encoded(receipt));write('apply-manifests.json',b'drift')
                with self.assertRaises(Blocked):retirement.verify_media_abort_terminal(base,terminal)

    def test_exact_snapshot_hash_is_required_before_any_structural_authority(self):
        for raw in (b'{}', b'{"operation":"0e2be62a3326","status":"BLOCKED"}'):
            with self.assertRaises(Blocked): retirement.media_abort_snapshot(raw)

    def test_terminal_preserves_failed_evidence_and_never_grants_rollback(self):
        original={'events':[{'kind':'failed'}],'error':'restart_readiness_timeout',
                  'nats_migration':{'verified':True},'migration_jobs':[{'name':'kept'}]}
        terminal=retirement.media_abort_terminal(original,{'deployment_pass':False})
        self.assertEqual(terminal['status'],'ABORTED')
        self.assertEqual(terminal['error'],original['error'])
        self.assertEqual(terminal['nats_migration'],original['nats_migration'])
        self.assertEqual(terminal['migration_jobs'],original['migration_jobs'])
        self.assertEqual(original['events'],[{'kind':'failed'}])
        self.assertIs(terminal['events'][-1]['deployment_pass'],False)
        self.assertIs(terminal['events'][-1]['rollback_eligible'],False)

    def test_original_apps_are_digest_pinned_while_migrated_hub_is_retained(self):
        names=retirement.CORE|{'voice-web','voice-admin'}
        originals={n:{'spec':{'template':{'spec':{'containers':[{'name':'app','image':'old:tag'}]}}}} for n in names}
        current=copy.deepcopy(originals);current[retirement.HUB]['spec']['template']['spec']['containers'][0]['image']='migrated-hub'
        images={n+'/app':'example.invalid/app@sha256:'+'a'*64 for n in names}
        snapshot={'context':{'original_snapshots':originals,'snapshots':current,'old_images':images}}
        result=retirement.media_abort_templates(snapshot)
        self.assertEqual(result[retirement.HUB],current[retirement.HUB]['spec']['template'])
        self.assertEqual(result['voice-user']['spec']['containers'][0]['image'],images['voice-user/app'])
        images['voice-user/app']='floating:tag'
        with self.assertRaises(Blocked):retirement.media_abort_templates(snapshot)

    def test_absent_finish_response_is_preserved_and_any_new_response_vetoed(self):
        request={'action':'finish','operation':retirement.MEDIA_ABORT_OPERATION,'nonce':retirement.MEDIA_ABORT_FINISH[:-5]}
        row={'phase':'STARTED','request':request,'request_sha256':retirement.canonical(request)}
        raw=json.dumps(row).encode()
        with tempfile.TemporaryDirectory() as td:
            root=Path(td);(root/'responses').mkdir()
            with patch.object(retirement,'private_bytes',return_value=raw),patch.object(retirement,'MEDIA_ABORT_JOURNAL',hashlib.sha256(raw).hexdigest()):
                self.assertTrue(retirement.media_abort_journal(root)['response_absent'])
                (root/'responses'/retirement.MEDIA_ABORT_FINISH).write_text('{}')
                with self.assertRaises(Blocked):retirement.media_abort_journal(root)

    def test_pending_admission_requires_exact_verified_receipt_membership(self):
        root=Path('/private/installed');path=root/'journal'/retirement.MEDIA_ABORT_FINISH
        row={'phase':'STARTED','request':{'operation':retirement.MEDIA_ABORT_OPERATION}}
        with patch.object(retirement,'document',return_value=row),patch.object(retirement,'verify_media_abort_terminal') as verify:
            retirement.pending_request_accepted(root,path,{'journals':{}});verify.assert_called_once()
            with self.assertRaises(Blocked):retirement.pending_request_accepted(root,path.with_name('a'*64+'.json'),{'journals':{}})
        with patch.object(retirement,'document',return_value=row),patch.object(retirement,'verify_media_abort_terminal',side_effect=Blocked('bad')):
            with self.assertRaises(Blocked):retirement.pending_request_accepted(root,path,{'journals':{}})

    def test_installer_has_exact_predecessor_and_unique_history_path(self):
        with patch.object(installer, '_upgrade_idle_transition') as call:
            installer.upgrade_idle_media_v1('candidate')
        self.assertEqual(call.call_args.args[:4], ('candidate',
            '1295ba61d4603a18790c33cfb9b12ae5e3d337c799d9853bd0fca3a80cc00c20',
            'idle-media-v1', 'media-1295'))

    def test_exact_completed_original_restore_is_admitted_without_relabeling_pass(self):
        state = {'operation': '0e2be62a3326', 'status': 'ABORTED', 'operator_media_abort': {}}
        base = Path('/private/rollout-0e2be62a3326')
        # The terminal dispatcher must use the dedicated validator, never PASS.
        with patch.object(retirement, 'verify_media_abort_terminal', return_value={'deployment_pass': False}, create=True) as verify:
            self.assertTrue(retirement.idle_state(base, state))
            verify.assert_called_once_with(base, state)


if __name__ == '__main__': unittest.main()
