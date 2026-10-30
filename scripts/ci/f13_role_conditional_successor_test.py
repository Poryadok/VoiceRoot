"""Real Git fixtures for one historical Role condition; never producer admission."""
import json
import hashlib
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

import f13_role_conditional as legacy
import f13_role_conditional_successor as successor

ROOT=Path(__file__).resolve().parents[2]

# These are private fixture inputs, never producer or runtime authority.
INVENTORY_SOURCE = '0a65f54010e834fcdea5ee0d0d64844c1889f158'
MULTI_OWNER_SOURCE = '1cc1d46023bd31c473136fa285446ba30e9ef99a'
USER_INVENTORY_PATH = 'src/backend/user/pb/voice/common/v1/reference_inventory.pb.go'
INVENTORY_ENTRIES = {
    USER_INVENTORY_PATH: '9311529c5aaf5b56e31102c4a0fb7b7c3e47ade0',
    'src/backend/voice/pb/voice/story/v1/reference_inventory.pb.go': 'b505d59b968af59177acd22ad5161a748d48efcc',
    'src/backend/voice/pb/voice/story/v1/reference_inventory_grpc.pb.go': '06656fe9cd9478cbacde07b63eacb6b32bef5035',
}
# The User common blob is identical to the independently regenerated Voice
# common blob at 1cc; its only proto addition is optional owner_chat_id field11.
MULTI_OWNER_ENTRIES = dict(INVENTORY_ENTRIES, **{
    USER_INVENTORY_PATH: '3d8918d448ab7c7c50e798f689fdb4862944ebaa'})


def git(*args,data=None,env=None):
    return subprocess.check_output(['git','--no-replace-objects','-C',str(ROOT),*args],input=data,env=env).decode().strip()


def reviewed_inventory_fixture_source(source):
    def ancestor(reviewed):
        return subprocess.run(['git','--no-replace-objects','-C',str(ROOT),
                               'merge-base','--is-ancestor',reviewed,source],
                              capture_output=True).returncode == 0
    # Select one whole tuple by history, not a per-file union. A descendant of
    # 1cc cannot roll back just common to the earlier reviewed inventory blob.
    reviewed, entries = ((MULTI_OWNER_SOURCE, MULTI_OWNER_ENTRIES)
                         if ancestor(MULTI_OWNER_SOURCE) else
                         (INVENTORY_SOURCE, INVENTORY_ENTRIES))
    legacy.require(ancestor(reviewed), 'role_fixture_unreviewed_inventory')
    for path, oid in entries.items():
        expected = {'mode':'100644','type':'blob','oid':oid}
        legacy.require(legacy.entry(ROOT,reviewed,path) == expected and
                       legacy.entry(ROOT,source,path) == expected,
                       'role_fixture_unreviewed_inventory')
    return reviewed


class ReviewedRoleInventoryFixtureTest(unittest.TestCase):
    def revision(self, source, path=None, variant=None, parent=None):
        with tempfile.TemporaryDirectory() as folder:
            env=dict(os.environ,GIT_INDEX_FILE=str(Path(folder)/'index'),
                     GIT_AUTHOR_NAME='F13 fixture',GIT_COMMITTER_NAME='F13 fixture',
                     GIT_AUTHOR_EMAIL='fixture@example.invalid',GIT_COMMITTER_EMAIL='fixture@example.invalid')
            git('read-tree',source,env=env)
            if variant == 'missing':
                git('update-index','--force-remove',path,env=env)
            elif path:
                oid = (git('hash-object','-w','--stdin',data=b'unreviewed inventory\n')
                       if variant == 'content' else
                       legacy.entry(ROOT,source,path)['oid'] if variant == 'mode' else
                       INVENTORY_ENTRIES[path] if variant == 'old' else
                       MULTI_OWNER_ENTRIES[path])
                git('update-index','--cacheinfo',
                    ('100755' if variant == 'mode' else '100644')+','+oid+','+path,env=env)
            return git('commit-tree',git('write-tree',env=env),'-p',parent or source,
                       data=b'Synthetic Role inventory guard fixture; no admission\n',env=env)

    def test_exact_old_new_sources_and_descendant(self):
        for source, expected in ((INVENTORY_SOURCE, INVENTORY_SOURCE),
                                 (MULTI_OWNER_SOURCE, MULTI_OWNER_SOURCE),
                                 (git('rev-parse','HEAD'), MULTI_OWNER_SOURCE)):
            with self.subTest(source=source):
                self.assertEqual(reviewed_inventory_fixture_source(source),expected)

    def test_each_tuple_rejects_content_mode_missing_and_disconnected_history(self):
        for source in (INVENTORY_SOURCE, MULTI_OWNER_SOURCE):
            for path in INVENTORY_ENTRIES:
                for variant in ('content','mode','missing'):
                    with self.subTest(source=source,path=path,variant=variant), self.assertRaises(legacy.Rejected):
                        reviewed_inventory_fixture_source(self.revision(source,path,variant))
            disconnected=self.revision(source,parent=legacy.PINNED_BASE)
            with self.subTest(source=source,variant='ancestry'), self.assertRaisesRegex(
                    legacy.Rejected,'role_fixture_unreviewed_inventory'):
                reviewed_inventory_fixture_source(disconnected)

    def test_mixed_old_common_and_new_common_without_new_ancestry_reject(self):
        for source, variant in ((MULTI_OWNER_SOURCE,'old'),(INVENTORY_SOURCE,'new')):
            with self.subTest(source=source), self.assertRaisesRegex(
                    legacy.Rejected,'role_fixture_unreviewed_inventory'):
                reviewed_inventory_fixture_source(self.revision(source,USER_INVENTORY_PATH,variant))


class RoleSuccessorTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.actual_head=git('rev-parse','HEAD')
        cls.record=successor.load_record(ROOT)
        current={line.split('\t',1)[1]:line.split()[2] for line in
                 git('ls-tree','-r',cls.actual_head).splitlines()}
        cls.restored_context=[row for row in cls.record['context']
                              if current.get(row['path'])!=row['entry']['oid']]
        closure=json.loads(legacy.git(ROOT,'show',successor.PINNED_RECORD_COMMIT+':'+
                                          cls.record['closure_manifest']['path']))
        packages={item['directory']:set(item['paths']) for item in closure['package_inventories']}
        cls.removed_package_paths=sorted(path for path in current if
            str(Path(path).parent).replace('\\','/') in packages and
            path not in packages[str(Path(path).parent).replace('\\','/')])
        with tempfile.TemporaryDirectory() as folder:
            env=dict(os.environ,GIT_INDEX_FILE=str(Path(folder)/'index'),
                     GIT_AUTHOR_NAME='F13 fixture',GIT_COMMITTER_NAME='F13 fixture',
                     GIT_AUTHOR_EMAIL='fixture@example.invalid',GIT_COMMITTER_EMAIL='fixture@example.invalid')
            git('read-tree',cls.actual_head,env=env)
            for row in cls.restored_context:
                entry=row['entry']
                git('update-index','--add','--cacheinfo',
                    entry['mode']+','+entry['oid']+','+row['path'],env=env)
            for path in cls.removed_package_paths:
                git('update-index','--force-remove',path,env=env)
            cls.target=git('commit-tree',git('write-tree',env=env),'-p',cls.actual_head,
                           data=b'Synthetic Role context fixture; no admission\n',env=env)
        old=json.loads(legacy.git(ROOT,'show',legacy.PINNED_RECORD_COMMIT+':'+legacy.RECORD_PATH))
        cls.prior='\t'.join(old['actual_prior_row'])
        m=old['incoming_metadata_at_failure']
        cls.incoming='\t'.join([legacy.ROLE_PATH,'true',m['owner'],m['parent'],m['source_base'],m['source_head'],m['content_base'],legacy.PINNED_BASE])

    def qualify(self,target=None,prior=None,incoming=None):
        # The pinned record was fully verified in setUpClass. Each private target
        # still exercises the real ancestry, carrier, entry and inventory guards.
        with patch.object(successor,'load_record',return_value=self.record):
            return successor.qualify(ROOT,target or self.target,prior or self.prior,incoming or self.incoming)

    def changed(self,name,content=b'unreviewed fixture\n',mode='100644'):
        with tempfile.TemporaryDirectory() as folder:
            env=dict(os.environ,GIT_INDEX_FILE=str(Path(folder)/'index'),
                     GIT_AUTHOR_NAME='F13 fixture',GIT_COMMITTER_NAME='F13 fixture',
                     GIT_AUTHOR_EMAIL='fixture@example.invalid',GIT_COMMITTER_EMAIL='fixture@example.invalid')
            git('read-tree',self.target,env=env)
            if content is None:git('update-index','--force-remove',name,env=env)
            else:
                blob=git('hash-object','-w','--stdin',data=content)
                git('update-index','--add','--cacheinfo',mode+','+blob+','+name,env=env)
            return git('commit-tree',git('write-tree',env=env),'-p',self.target,
                       data=b'Synthetic Role source fixture; no admission\n',env=env)

    def test_exact_historical_and_current_correction(self):
        result=successor.qualify(ROOT,self.target,self.prior,self.incoming)
        self.assertEqual(result['actual_target'],self.target)
        self.assertEqual(result['historical_validation_target'],successor.HISTORICAL_CARRIER)
        self.assertNotEqual(result['actual_target'],result['historical_validation_target'])
        self.assertEqual(result['prior_row'],self.prior.split('\t'))
        self.assertEqual(result['incoming_row'],self.incoming.split('\t'))
        self.assertEqual(result['historical_runtime_gap'],self.record['historical_runtime_gap'])
        self.assertTrue(result['retain_runtime_and_content_taint'])
        self.assertTrue(all(result[name] is False for name in successor.FALSE_FLAGS))

    def test_private_fixture_restores_only_reviewed_context(self):
        self.assertEqual({row['path'] for row in self.restored_context},{
            'src/backend/chat/pb/voice/chat/v1/chat.pb.go',
            'src/backend/chat/pb/voice/chat/v1/chat_grpc.pb.go',
            'src/backend/file/pb/voice/file/v1/file.pb.go',
            'src/backend/file/pb/voice/file/v1/file_grpc.pb.go',
            'src/backend/pkg/integrationtest/migrations.go'})
        story_file_grpc = 'src/backend/file/pb/voice/file/v1/file_grpc.pb.go'
        reviewed_story_source = 'f38e45008308e5309e0a371a411afb4c7bace6ae'
        reviewed_entry = {'mode':'100644','type':'blob',
                          'oid':'6d88bec7977afd2364341c7a6d3073b2f5f35abe'}
        self.assertEqual(legacy.entry(ROOT,reviewed_story_source,story_file_grpc),reviewed_entry)
        self.assertEqual(legacy.entry(ROOT,self.actual_head,story_file_grpc),reviewed_entry)
        self.assertEqual(subprocess.run(['git','-C',str(ROOT),'merge-base','--is-ancestor',
                                        reviewed_story_source,self.actual_head]).returncode,0)
        self.assertEqual(set(self.removed_package_paths),{
            'src/backend/chat/pb/voice/chat/v1/catalog_assets.pb.go',
            'src/backend/chat/pb/voice/chat/v1/catalog_assets_grpc.pb.go',
            'src/backend/file/pb/voice/file/v1/catalog_assets.pb.go',
            'src/backend/file/pb/voice/file/v1/catalog_assets_grpc.pb.go',
            'src/backend/pkg/authoritysource/gis_state.go',
            'src/backend/pkg/authoritysource/gis_state_test.go',
            'src/backend/user/pb/voice/common/v1/reference_inventory.pb.go',
            'src/backend/voice/pb/voice/story/v1/reference_inventory.pb.go',
            'src/backend/voice/pb/voice/story/v1/reference_inventory_grpc.pb.go'})
        self.assertEqual(reviewed_inventory_fixture_source(self.actual_head),MULTI_OWNER_SOURCE)
        self.assertEqual(git('rev-list','--parents','-n','1',self.target).split(),
                         [self.target,self.actual_head])
        self.assertEqual(set(git('diff','--name-only',self.actual_head,self.target).splitlines()),
                         {row['path'] for row in self.restored_context}|set(self.removed_package_paths))
        for row in self.restored_context:
            with self.subTest(path=row['path']):
                self.assertEqual(legacy.entry(ROOT,self.target,row['path']),row['entry'])
        for path in self.removed_package_paths:
            with self.subTest(path=path),self.assertRaisesRegex(legacy.Rejected,'single_entry'):
                legacy.entry(ROOT,self.target,path)

    def test_actual_head_context_drift_is_rejected(self):
        with self.assertRaisesRegex(legacy.Rejected,'actual_entry'):
            self.qualify(self.actual_head)

    def test_old_actual_target_is_still_denied(self):
        with self.assertRaisesRegex(legacy.Rejected,'actual_entry'):
            legacy.qualify(ROOT,self.target,self.prior,self.incoming)

    def test_each_of_eleven_current_bindings_is_required(self):
        for row in self.record['required_current_source']:
            with self.subTest(path=row['path']),self.assertRaisesRegex(legacy.Rejected,'actual_entry'):
                self.qualify(self.changed(row['path']))

    def test_dependency_drift(self):
        with self.assertRaisesRegex(legacy.Rejected,'actual_entry'):
            self.qualify(self.changed(self.record['context'][0]['path']))

    def test_dependency_removed(self):
        with self.assertRaisesRegex(legacy.Rejected,'single_entry'):
            self.qualify(self.changed(self.record['context'][0]['path'],None))

    def test_dependency_mode(self):
        name=self.record['context'][0]['path']
        with self.assertRaisesRegex(legacy.Rejected,'actual_entry'):
            self.qualify(self.changed(name,(ROOT/name).read_bytes(),'100755'))

    def test_new_package_source(self):
        with self.assertRaisesRegex(legacy.Rejected,'role_successor_target_package_drift'):
            self.qualify(self.changed('src/backend/role/internal/principalruntime/unreviewed.go'))

    def test_record_and_evidence_drift(self):
        for name in (successor.RECORD_PATH,self.record['closure_manifest']['path'],self.record['evidence'][0]['path']):
            with self.subTest(path=name),self.assertRaisesRegex(legacy.Rejected,'role_successor_artifact_drift'):
                self.qualify(self.changed(name))

    def test_both_carriers_immutable(self):
        for name in (successor.VERIFIER_PATH,legacy.VERIFIER_PATH):
            with self.subTest(path=name),self.assertRaisesRegex(legacy.Rejected,'role_successor_executed_carrier'):
                self.qualify(self.changed(name))

    def test_predecessor_record_immutable(self):
        with self.assertRaisesRegex(legacy.Rejected,'role_successor_predecessor_record_drift'):
            self.qualify(self.changed(legacy.RECORD_PATH))

    def test_predecessor_record_cannot_be_deleted(self):
        with self.assertRaisesRegex(legacy.Rejected,'single_entry'):
            self.qualify(self.changed(legacy.RECORD_PATH,None))

    def test_carrier_mode(self):
        with self.assertRaisesRegex(legacy.Rejected,'role_successor_carrier_mode'):
            self.qualify(self.changed(successor.VERIFIER_PATH,(ROOT/successor.VERIFIER_PATH).read_bytes(),'100755'))

    def test_record_not_carrier(self):
        with self.assertRaisesRegex(legacy.Rejected,'role_successor_later_carrier'):
            self.qualify(successor.PINNED_RECORD_COMMIT)

    def test_changed_historical_owner(self):
        changed=self.incoming.split('\t');changed[2]=self.target
        with self.assertRaisesRegex(legacy.Rejected,'exact_actual_incoming_row'):
            self.qualify(incoming='\t'.join(changed))

    def test_changed_prior_row(self):
        with self.assertRaisesRegex(legacy.Rejected,'exact_observed_prior_row'):
            self.qualify(prior=self.prior.replace('\tfalse\t','\ttrue\t',1))

    def test_runtime_taint_is_not_normalized(self):
        for flag in ('true','false'):
            incoming=self.incoming.split('\t');incoming[1]=flag
            self.assertEqual(self.qualify(incoming='\t'.join(incoming))['incoming_row'][1],flag)

    def test_dynamic_import_outside_scripts(self):
        code="""import importlib.util,sys
spec=importlib.util.spec_from_file_location('extracted_role',sys.argv[1])
m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
r=m.qualify(*sys.argv[2:]);assert r['actual_target']==sys.argv[3]
"""
        with tempfile.TemporaryDirectory() as folder:
            subprocess.run([sys.executable,'-c',code,str(ROOT/successor.VERIFIER_PATH),str(ROOT),self.target,
                            self.prior,self.incoming],cwd=folder,capture_output=True,check=True)

    def altered_record(self, mutate, reason):
        record=json.loads(json.dumps(self.record));mutate(record)
        raw=(json.dumps(record,sort_keys=True)+'\n').encode()
        revision=self.changed(successor.RECORD_PATH,raw)
        with patch.object(successor,'PINNED_RECORD_COMMIT',revision),patch.object(
                successor,'PINNED_RECORD_SHA256',hashlib.sha256(raw).hexdigest()):
            with self.assertRaisesRegex(legacy.Rejected,reason):successor.load_record(ROOT)

    def test_missing_current_binding_in_record(self):
        self.altered_record(lambda r:r['required_current_source'].pop(),'role_successor_all_eleven')

    def test_omitted_dependency_in_record(self):
        self.altered_record(lambda r:r['context'].pop(),'role_successor_closure_complete')

    def test_unknown_historical_lineage(self):
        self.altered_record(lambda r:r['predecessor'].update(carrier=self.target),'role_successor_lineage')

    def test_cannot_claim_historical_runtime(self):
        self.altered_record(lambda r:r.update(runtime_pass=True),'role_successor_no_broader_authority')

    def test_historical_hold_cannot_be_removed(self):
        self.altered_record(lambda r:r.update(historical_runtime_gap=''),'role_successor_historical_gap')

    def test_record_needs_independent_evidence_reference(self):
        self.altered_record(lambda r:r.update(review=[True]),'role_successor_review')

    def test_unpinned_draft_rejected(self):
        with patch.object(successor,'PINNED_RECORD_COMMIT',None),self.assertRaisesRegex(
                legacy.Rejected,'role_successor_carrier_not_pinned'):successor.load_record(ROOT)


if __name__=='__main__':unittest.main()
