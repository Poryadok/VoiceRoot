"""Real Git fixtures for one historical Role condition; never producer admission."""
import json
import hashlib
import io
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from types import SimpleNamespace
from unittest.mock import patch
from contextlib import redirect_stderr

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


def reviewed_current_compiled_entry(source,path):
    """Return only a fully source-qualified v5 context entry for this fixture."""
    try:
        import f13_release_source as release
    except ModuleNotFoundError as error:
        if error.name!='f13_release_source':raise
        return None
    if not release.applicable(ROOT,source):return None
    production=release.production_record(ROOT);native=release.load_native(ROOT)
    reviewed=release.FOUNDATION_BASE
    legacy.require(native['version']==release.NATIVE_VERSION and production['source']==native['source']==reviewed,
        'fixture_v5_source_binding')
    legacy.require(subprocess.run(['git','-C',str(ROOT),'merge-base','--is-ancestor',reviewed,source],
        capture_output=True).returncode==0,'fixture_v5_source_ancestry')
    rows=[row for row in native['context'] if row['path']==path]
    legacy.require(path in native['compiled_closure'] and len(rows)==1,'fixture_v5_compiled_context')
    expected=rows[0]['entry']
    legacy.require(isinstance(expected,dict) and set(expected)=={'mode','type','oid'}
        and expected['mode']=='100644' and expected['type']=='blob'
        and legacy.entry(ROOT,reviewed,path)==expected and legacy.entry(ROOT,source,path)==expected,
        'fixture_v5_compiled_entry')
    return expected


class ReviewedCurrentCompiledFixtureTest(unittest.TestCase):
    def test_v5_context_requires_exact_compiled_entry_and_ancestry(self):
        with tempfile.TemporaryDirectory() as directory:
            repo=Path(directory); path='src/backend/user/pb/voice/common/v1/reference_inventory.pb.go'
            def g(*args,data=None):
                return subprocess.check_output(['git','-C',str(repo),*args],input=data).decode().strip()
            g('init','-q');g('config','user.name','fixture');g('config','user.email','fixture@example.invalid')
            (repo/path).parent.mkdir(parents=True);(repo/path).write_text('reviewed\n')
            g('add','.');g('commit','-qm','reviewed');h=g('rev-parse','HEAD');entry=legacy.entry(repo,h,path)
            native=dict(version='NATIVE-P3-SOURCE-v5',source=h,compiled_closure=[path],context=[dict(path=path,entry=entry)])
            release=SimpleNamespace(FOUNDATION_BASE=h,NATIVE_VERSION='NATIVE-P3-SOURCE-v5',
                applicable=lambda r,s:subprocess.run(['git','-C',str(r),'merge-base','--is-ancestor',h,s],capture_output=True).returncode==0,
                production_record=lambda r:dict(source=h),load_native=lambda r:native)
            with patch(__name__+'.ROOT',repo),patch.dict(sys.modules,{'f13_release_source':release}):
                self.assertEqual(reviewed_current_compiled_entry(h,path),entry)
                for kind in ('content','mode','type','missing','disconnected'):
                    index=repo/('index-'+kind);env=dict(os.environ,GIT_INDEX_FILE=str(index))
                    subprocess.check_call(['git','-C',str(repo),'read-tree',h],env=env)
                    if kind=='missing':
                        subprocess.check_call(['git','-C',str(repo),'update-index','--force-remove','--',path],env=env)
                    elif kind!='disconnected':
                        oid=entry['oid'] if kind in ('mode','type') else g('hash-object','-w','--stdin',data=b'unreviewed\n')
                        mode='100755' if kind=='mode' else '120000' if kind=='type' else '100644'
                        subprocess.check_call(['git','-C',str(repo),'update-index','--cacheinfo',mode+','+oid+','+path],env=env)
                    tree=subprocess.check_output(['git','-C',str(repo),'write-tree'],env=env).decode().strip()
                    child=g('commit-tree',tree,*([] if kind=='disconnected' else ['-p',h]),'-m','fixture')
                    with self.subTest(kind=kind):
                        if kind=='disconnected':self.assertIsNone(reviewed_current_compiled_entry(child,path))
                        else:
                            with self.assertRaises(legacy.Rejected):reviewed_current_compiled_entry(child,path)
                for invalid in (dict(native,version='NATIVE-P3-SOURCE-v4'),dict(native,source='0'*40),
                                dict(native,compiled_closure=[]),dict(native,context=[]),
                                dict(native,context=native['context']*2)):
                    with patch.object(release,'load_native',return_value=invalid),self.assertRaises(legacy.Rejected):
                        reviewed_current_compiled_entry(h,path)
                with patch.object(release,'production_record',side_effect=legacy.Rejected('unreviewed')):
                    with self.assertRaisesRegex(legacy.Rejected,'unreviewed'):reviewed_current_compiled_entry(h,path)


class ReleaseRoleFixtureTest(unittest.TestCase):
    """Production boundary is mocked; target tree and raw rows use real Git."""
    def test_current_v3_rejection_does_not_fall_back_to_successful_v1(self):
        import f13_release_source as release
        target='a'*40
        args=['role','--repo',str(ROOT),'--target',target,'--prior-row','prior','--incoming-row','incoming']
        with patch.object(sys,'argv',args),patch.multiple(release,ROOT=ROOT,FOUNDATION_BASE='b'*40), \
             patch.object(legacy,'git',return_value=(target+'\n').encode()), \
             patch.object(successor.subprocess,'run',return_value=SimpleNamespace(returncode=0)), \
             patch.object(legacy,'qualify',return_value={'legacy_success':True}) as old, \
             patch.object(successor,'qualify_release',side_effect=release.Rejected('wrong_event')) as current, \
             redirect_stderr(io.StringIO()) as stderr:
            self.assertEqual(successor.main(),1)
            self.assertIn('wrong_event',stderr.getvalue())
            current.assert_called_once()
            old.assert_not_called()

    def test_current_target_entries_and_both_runtime_flags_are_retained(self):
        import f13_release_source as release
        with tempfile.TemporaryDirectory() as folder:
            repo=Path(folder)
            def g(*args):
                return subprocess.check_output(['git','-C',str(repo),*args]).decode().strip()
            g('init','-q');g('config','user.name','fixture');g('config','user.email','fixture@example.invalid')
            unit='src/backend/role/dep.go'
            paths=(unit,successor.VERIFIER_PATH,legacy.VERIFIER_PATH,'scripts/ci/f13_release_source.py',
                   release.ROLE_RECORD_PATH,release.ROLE_CLOSURE_PATH)
            for path in paths:
                f=repo/path;f.parent.mkdir(parents=True,exist_ok=True);f.write_text(path+'\n')
            g('add','.');g('commit','-qm','foundation');base=g('rev-parse','HEAD')
            (repo/'later').write_text('later\n');g('add','later');g('commit','-qm','target');target=g('rev-parse','HEAD')
            tree=release.documentation.tree_entries(repo,base)
            inventories=release.documentation.immediate_directory_entries(tree)
            entry=legacy.entry(repo,base,unit)
            record=dict(source=base,evidence_source=base,required_current_source=[dict(path=unit,entry=entry)],
                context=[],closure={'directory_inventories':[dict(directory='src/backend/role',
                    entries=inventories['src/backend/role'])]},evidence=[],historical_runtime_gap='HOLD')
            raw=lambda flag:'\t'.join([legacy.ROLE_PATH,flag]+['1'*40]*6)
            with patch.multiple(release,ROOT=repo,FOUNDATION_BASE=base,RECORD_COMMIT=base,ROLE_SHA256='a'*64), \
                 patch.multiple(successor,ORIGINAL_CORRECTION=base,HISTORICAL_CARRIER=base), \
                 patch.object(release,'production_qualify'),patch.object(release,'load_role',return_value=record), \
                 patch.object(legacy,'qualify',return_value={'historical':True}):
                for flag in ('false','true'):
                    receipt=successor.qualify_release(repo,target,raw('false'),raw(flag))
                    self.assertEqual(receipt['incoming_row'][1],flag)
                    self.assertTrue(receipt['retain_runtime_and_content_taint'])
                    self.assertEqual(receipt['historical_runtime_gap'],'HOLD')
                    self.assertFalse(receipt['runtime_pass'])
                with self.assertRaisesRegex(legacy.Rejected,'role_release_actual_head'):
                    successor.qualify_release(repo,base,raw('false'),raw('true'))
                with patch.object(release,'production_qualify',side_effect=release.Rejected('wrong_event')):
                    with self.assertRaisesRegex(release.Rejected,'wrong_event'):
                        successor.qualify_release(repo,target,raw('false'),raw('true'))
                (repo/unit).write_text('drift\n');g('add',unit);g('commit','-qm','drift')
                with self.assertRaises(legacy.Rejected):
                    successor.qualify_release(repo,g('rev-parse','HEAD'),raw('false'),raw('true'))


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
            'src/backend/pkg/go.mod',
            'src/backend/pkg/integrationtest/migrations.go'})
        pkg_mod = 'src/backend/pkg/go.mod'
        pkg_source = '2c279b1e733e62b8af653294a421b3a20f2266ba'
        pkg_entry = {'mode':'100644','type':'blob',
                     'oid':'60bc3002aa7ebd5e952d2c988166e93c618cb697'}
        self.assertEqual(legacy.entry(ROOT,pkg_source,pkg_mod),pkg_entry)
        self.assertEqual(legacy.entry(ROOT,self.actual_head,pkg_mod),pkg_entry)
        self.assertEqual(subprocess.run(['git','-C',str(ROOT),'merge-base','--is-ancestor',
                                        pkg_source,self.actual_head]).returncode,0)
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
        inventory_source = '0a65f54010e834fcdea5ee0d0d64844c1889f158'
        inventory_entries = {
            'src/backend/user/pb/voice/common/v1/reference_inventory.pb.go': '9311529c5aaf5b56e31102c4a0fb7b7c3e47ade0',
            'src/backend/voice/pb/voice/story/v1/reference_inventory.pb.go': 'b505d59b968af59177acd22ad5161a748d48efcc',
            'src/backend/voice/pb/voice/story/v1/reference_inventory_grpc.pb.go': '06656fe9cd9478cbacde07b63eacb6b32bef5035',
        }
        self.assertEqual(subprocess.run(['git','-C',str(ROOT),'merge-base','--is-ancestor',
                                        inventory_source,self.actual_head]).returncode,0)
        current_compiled=reviewed_current_compiled_entry(self.actual_head,USER_INVENTORY_PATH)
        if current_compiled is None:
            self.assertEqual(reviewed_inventory_fixture_source(self.actual_head),MULTI_OWNER_SOURCE)
        for path, oid in inventory_entries.items():
            with self.subTest(path=path):
                expected = {'mode':'100644','type':'blob','oid':oid}
                self.assertEqual(legacy.entry(ROOT,inventory_source,path),expected)
                if path=='src/backend/user/pb/voice/common/v1/reference_inventory.pb.go':
                    expected=current_compiled or dict(mode='100644',type='blob',oid=MULTI_OWNER_ENTRIES[path])
                self.assertEqual(legacy.entry(ROOT,self.actual_head,path),expected)
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
