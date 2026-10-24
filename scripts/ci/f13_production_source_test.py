"""Real Git production qualification fixtures; no actual producer success claim."""
import hashlib
import json
import os
import re
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import f13_production_source as source
import f13_f54_history as history
import f13_native_p3_authority as native
import f13_native_p3_documentation as documentation

ROOT=Path(__file__).resolve().parents[2]


def git(*args,data=None,env=None):
    return subprocess.check_output(['git','--no-replace-objects','-C',str(ROOT),*args],input=data,env=env).decode().strip()


class FixtureSourceUnitTest(unittest.TestCase):
    def test_current_head_never_falls_back_to_legacy_success(self):
        with patch.object(source,'fixture_applicable',return_value=True),\
                patch.object(native.legacy,'qualify',return_value={'unqualified_historical_success':True}) as old,\
                patch.object(documentation,'qualify_context',side_effect=ValueError('current-bridge-required')) as current:
            with self.assertRaisesRegex(ValueError,'^current-bridge-required$'):
                native.qualify(ROOT,'historical-base','historical-source','historical-target',
                    'pull_request','refs/pull/875/merge','historical-target','','master')
            old.assert_not_called();current.assert_called_once()

    def test_exact_fixture_successor_retains_native_v4(self):
        records=source.load_fixture_fix(ROOT);record=records[0]
        self.assertEqual(record['source'],source.FIXTURE_SOURCE)
        self.assertEqual([r['path'] for r in record['changes']],['scripts/ci/phase0-fixture_test.py'])
        self.assertEqual(records[1]['version'],source.INTEGRATION_VERSION)
        self.assertEqual(record['native_record']['record'],documentation.CONTEXT_RECORD_COMMIT)
        self.assertEqual(record['native_record']['source'],documentation.CONTEXT_SOURCE)
        self.assertEqual(record['native_record']['version'],documentation.CONTEXT_VERSION)

    def test_unpinned_fixture_record_denied(self):
        with patch.object(source,'FIXTURE_RECORD_COMMIT',None),self.assertRaisesRegex(ValueError,'production_fixture_not_pinned'):
            source.load_fixture_fix(ROOT)


class IntegratedSourceUnitTest(unittest.TestCase):
    def test_exact_integration_source_keeps_prior_units(self):
        records=source.load_integration(ROOT);record=records[0]
        self.assertEqual(record['source'],source.INTEGRATION_SOURCE)
        self.assertEqual(len(record['changes']),46)
        self.assertEqual(record['event_base'],source.INTEGRATION_EVENT_BASE)
        self.assertEqual(record['historical_event_base'],source.EVENT_BASE)
        self.assertEqual(records[1]['version'],source.SOCIAL_VERSION)
        self.assertEqual(record['native_record']['version'],documentation.CONTEXT_VERSION)

    def test_unpinned_integration_denied(self):
        with patch.object(source,'INTEGRATION_RECORD_COMMIT',None),self.assertRaisesRegex(ValueError,'production_integration_not_pinned'):
            source.load_integration(ROOT)


class ProductionSourceTest(unittest.TestCase):
    @classmethod
    def commit(cls,tree,parents):
        env=dict(os.environ,GIT_AUTHOR_NAME='F13 fixture',GIT_COMMITTER_NAME='F13 fixture',
                 GIT_AUTHOR_EMAIL='fixture@example.invalid',GIT_COMMITTER_EMAIL='fixture@example.invalid')
        args=['commit-tree',tree]
        for parent in parents:args+=['-p',parent]
        return git(*args,data=b'Synthetic production qualification fixture; no admission\n',env=env)

    @classmethod
    def setUpClass(cls):
        global ROOT
        cls.original_root=ROOT
        cls.directory=tempfile.TemporaryDirectory(prefix='f13-production-fixture-')
        candidate=Path(cls.directory.name)/'repo'
        subprocess.run(['git','clone','--shared','--no-checkout',str(ROOT),str(candidate)],check=True,capture_output=True)
        revision=git('rev-parse','HEAD')
        ROOT=candidate;source.ROOT=candidate;history.ROOT=candidate
        git('checkout','--detach',revision)
        cls.original=git('rev-parse','HEAD')
        cls.event_base=source.INTEGRATION_EVENT_BASE
        cls.target=cls.commit(git('rev-parse',cls.original+'^{tree}'),[cls.event_base,cls.original])
        cls.env=dict(GITHUB_EVENT_NAME='pull_request',GITHUB_REF='refs/pull/875/merge',GITHUB_BASE_REF='master',
                     GITHUB_SHA=cls.target,VOICE_CI_PUSH_BEFORE_SHA='')
        cls.record=source.load_record(ROOT)

    @classmethod
    def tearDownClass(cls):
        global ROOT
        ROOT=cls.original_root;source.ROOT=ROOT;history.ROOT=ROOT
        cls.directory.cleanup()

    def setUp(self):
        self.environment=patch.dict(os.environ,self.env);self.environment.start()
        git('update-ref','HEAD',self.target)

    def tearDown(self):
        git('update-ref','HEAD',self.original);self.environment.stop()

    def changed(self,name,content=b'unreviewed input\n',mode='100644'):
        with tempfile.TemporaryDirectory() as folder:
            env=dict(os.environ,GIT_INDEX_FILE=str(Path(folder)/'index'))
            git('read-tree',self.original,env=env)
            if content is None:git('update-index','--force-remove',name,env=env)
            else:
                blob=git('hash-object','-w','--stdin',data=content)
                git('update-index','--add','--cacheinfo',mode+','+blob+','+name,env=env)
            candidate=self.commit(git('write-tree',env=env),[self.original])
        target=self.commit(git('rev-parse',candidate+'^{tree}'),[self.event_base,candidate])
        git('update-ref','HEAD',target);os.environ['GITHUB_SHA']=target
        return target

    def test_complete_current_source(self):
        value=source.qualify(ROOT)
        self.assertEqual(value['target'],self.target)
        self.assertEqual(value['source'],self.original)
        self.assertEqual(value['historical_collector'],source.HISTORICAL_SCOPE)
        self.assertNotEqual(value['base'],value['historical_collector']['base'])
        self.assertFalse(value['admission']);self.assertFalse(value['current_or_future_path_authority'])

    def test_foreign_repo_same_carrier_tree_rejected(self):
        with tempfile.TemporaryDirectory() as folder:
            foreign=Path(folder)/'foreign'
            subprocess.run(['git','clone','--shared','--no-checkout',str(ROOT),str(foreign)],check=True,capture_output=True)
            subprocess.run(['git','-C',str(foreign),'checkout','--detach',self.target],check=True,capture_output=True)
            self.assertEqual((foreign/source.VERIFIER_PATH).read_bytes(),(ROOT/source.VERIFIER_PATH).read_bytes())
            with self.assertRaisesRegex(ValueError,'^production_actual_root$'):source.qualify(foreign)

    def test_integration_current_native_and_old_event_base_denial(self):
        value=source.qualify(ROOT)
        self.assertEqual(value['base'],source.INTEGRATION_EVENT_BASE)
        binding=native.qualify(ROOT,self.event_base,self.original,self.target,'pull_request',
            'refs/pull/875/merge',self.target,'','master')
        self.assertEqual(binding['version'],documentation.CONTEXT_VERSION)
        self.assertEqual(binding['record'],source.INTEGRATION_RECORD_COMMIT)
        old_target=self.commit(git('rev-parse',self.original+'^{tree}'),[source.EVENT_BASE,self.original])
        git('update-ref','HEAD',old_target);os.environ['GITHUB_SHA']=old_target
        with self.assertRaisesRegex(ValueError,'production_actual_ordered_parents'):source.qualify(ROOT)

    def fixture_record_fixture(self,record,changes=(),parent=None):
        raw=(json.dumps(record)+'\n').encode()
        with tempfile.TemporaryDirectory() as folder:
            env=dict(os.environ,GIT_INDEX_FILE=str(Path(folder)/'index'))
            git('read-tree',source.FIXTURE_RECORD_COMMIT,env=env)
            for name,data in ((source.FIXTURE_RECORD_PATH,raw),*changes):
                if data is None:git('update-index','--force-remove',name,env=env)
                else:
                    blob=git('hash-object','-w','--stdin',data=data)
                    git('update-index','--add','--cacheinfo','100644,'+blob+','+name,env=env)
            revision=self.commit(git('write-tree',env=env),[parent or source.FIXTURE_SOURCE])
        return revision,hashlib.sha256(raw).hexdigest()

    def test_fixture_bridge_keeps_exact_native_record_and_current_event(self):
        binding=native.qualify(ROOT,self.event_base,self.original,self.target,'pull_request',
            'refs/pull/875/merge',self.target,'','master')
        self.assertEqual(source.qualify(ROOT)['type'],source.FIXTURE_VERSION)
        self.assertEqual(binding['record'],source.INTEGRATION_RECORD_COMMIT)
        self.assertEqual(binding['snapshot'],source.INTEGRATION_SOURCE)
        self.assertEqual(binding['source_applicability'],documentation.fixture_bridge_binding(self.original,self.target))
        native.verified_record(ROOT,binding)
        for key,value in (('source_applicability',None),('retain_guarded_and_runtime_history',False)):
            forged=dict(binding)
            if value is None:forged.pop(key)
            else:forged[key]=value
            with self.subTest(field=key),self.assertRaisesRegex(ValueError,'context_fixture_current_binding'):
                native.verified_record(ROOT,forged)
        for field in ('source_snapshot','record','record_sha256','base','source','target'):
            forged=json.loads(json.dumps(binding));forged['source_applicability'][field]='forged'
            with self.subTest(field=field),self.assertRaisesRegex(ValueError,'context_fixture_current_binding'):
                documentation.audit_map(ROOT,forged,'absent-map','absent-history')
        historical=dict(binding,target=source.INTEGRATION_RECORD_COMMIT)
        historical.pop('source_applicability')
        with self.assertRaisesRegex(ValueError,'context_fixture_actual_head'):
            native.verified_record(ROOT,historical)
        with self.assertRaisesRegex(ValueError,'context_fixture_actual_head'):
            native.qualify(ROOT,source.EVENT_BASE,source.INTEGRATION_PREDECESSOR,
                source.INTEGRATION_RECORD_COMMIT,'pull_request','refs/pull/875/merge',
                source.INTEGRATION_RECORD_COMMIT,'','master')

    def test_fixture_unpinned_denies_production_and_native_without_fallback(self):
        with patch.object(source,'FIXTURE_RECORD_COMMIT',None):
            with self.assertRaisesRegex(ValueError,'production_fixture_not_pinned'):source.qualify(ROOT)
            with self.assertRaisesRegex(ValueError,'production_fixture_not_pinned'):
                native.qualify(ROOT,self.event_base,self.original,self.target,'pull_request',
                    'refs/pull/875/merge',self.target,'','master')

    def test_fixture_next_edit_and_other_paths_are_not_authorized(self):
        for name in (source.FIXTURE_PATH,'scripts/ci/other-fixture_test.py','Makefile'):
            target=self.changed(name)
            candidate=git('rev-list','--parents','-n','1',target).split()[2]
            with self.subTest(path=name),self.assertRaisesRegex(ValueError,'production_only_reviewed_carrier_delta'):
                source.qualify(ROOT)
            with self.subTest(native_path=name),self.assertRaisesRegex(ValueError,'context_fixture_post_record_source_drift'):
                documentation.qualify_context(ROOT,self.event_base,candidate,target,'pull_request',
                    'refs/pull/875/merge',target,'','master')

    def test_fixture_reconstructs_inventory_closure_and_review_semantics(self):
        original=json.loads(git('show',source.FIXTURE_RECORD_COMMIT+':'+source.FIXTURE_RECORD_PATH))
        for case,code in (('inventory','production_fixture_complete_inventory'),
                ('changes','production_fixture_complete_delta'),('native','production_fixture_retained_native_tuple'),
                ('closure','production_fixture_native_closure'),('review','production_fixture_review')):
            record=json.loads(json.dumps(original));changes=[]
            if case=='inventory':
                name=record['current_inputs']['path'];value=json.loads(git('show',source.FIXTURE_RECORD_COMMIT+':'+name))
                value['entries'].pop();data=(json.dumps(value)+'\n').encode()
                record['current_inputs']['sha256']=hashlib.sha256(data).hexdigest();changes=[(name,data)]
            elif case=='changes':record['changes'].pop()
            elif case=='native':record['native_record']['record']=source.SOCIAL_RECORD_COMMIT
            elif case=='closure':record['native_closure']['directory_inventories'].pop()
            else:record['review']=[True]
            revision,digest=self.fixture_record_fixture(record,changes)
            with patch.object(source,'FIXTURE_RECORD_COMMIT',revision),patch.object(source,'FIXTURE_RECORD_SHA256',digest),self.subTest(case=case),self.assertRaisesRegex(ValueError,code):
                source.load_fixture_fix(ROOT)

    def test_fixture_preserves_old_artifacts_and_exact_parents(self):
        original=json.loads(git('show',source.FIXTURE_RECORD_COMMIT+':'+source.FIXTURE_RECORD_PATH))
        for name in (source.INTEGRATION_RECORD_PATH,documentation.CONTEXT_RECORD_PATH,'docs/CONTRIBUTING.md'):
            revision,digest=self.fixture_record_fixture(original,[(name,b'changed')])
            with patch.object(source,'FIXTURE_RECORD_COMMIT',revision),patch.object(source,'FIXTURE_RECORD_SHA256',digest),self.subTest(path=name),self.assertRaisesRegex(ValueError,'production_fixture_immutable_source_and_named_metadata'):
                source.load_fixture_fix(ROOT)
        revision,digest=self.fixture_record_fixture(original,parent=source.FIXTURE_PREDECESSOR)
        with patch.object(source,'FIXTURE_RECORD_COMMIT',revision),patch.object(source,'FIXTURE_RECORD_SHA256',digest),self.assertRaisesRegex(ValueError,'production_fixture_record_parent'):
            source.load_fixture_fix(ROOT)
        fake_h=self.commit(git('rev-parse',source.FIXTURE_SOURCE+'^{tree}'),[source.INTEGRATION_RECORD_COMMIT])
        original['source']=fake_h;revision,digest=self.fixture_record_fixture(original,parent=fake_h)
        with patch.object(source,'FIXTURE_SOURCE',fake_h),patch.object(source,'FIXTURE_RECORD_COMMIT',revision),patch.object(source,'FIXTURE_RECORD_SHA256',digest),self.assertRaisesRegex(ValueError,'production_fixture_source_parent'):
            source.load_fixture_fix(ROOT)

    def test_integration_unpinned_has_no_fallback(self):
        with patch.object(source,'INTEGRATION_RECORD_COMMIT',None),self.assertRaisesRegex(ValueError,'production_integration_not_pinned'):
            source.qualify(ROOT)

    def integration_record_fixture(self,record,changes=()):
        raw=(json.dumps(record)+'\n').encode()
        with tempfile.TemporaryDirectory() as folder:
            env=dict(os.environ,GIT_INDEX_FILE=str(Path(folder)/'index'))
            git('read-tree',source.INTEGRATION_RECORD_COMMIT,env=env)
            for name,data in ((source.INTEGRATION_RECORD_PATH,raw),*changes):
                if data is None:git('update-index','--force-remove',name,env=env)
                else:
                    blob=git('hash-object','-w','--stdin',data=data)
                    git('update-index','--add','--cacheinfo','100644,'+blob+','+name,env=env)
            revision=self.commit(git('write-tree',env=env),[source.INTEGRATION_SOURCE])
        return revision,hashlib.sha256(raw).hexdigest()

    def test_integration_reconstructs_complete_semantics(self):
        original=json.loads(git('show',source.INTEGRATION_RECORD_COMMIT+':'+source.INTEGRATION_RECORD_PATH))
        for case,code in (('inventory','production_integration_complete_inventory'),
                ('chain','production_integration_source_chain_binding'),('changes','production_integration_complete_delta'),
                ('event','production_integration_distinct_event_base'),('native','production_integration_native_tuple'),
                ('review','production_integration_review')):
            record=json.loads(json.dumps(original));changes=[]
            if case=='inventory':
                name=record['current_inputs']['path'];inventory=json.loads(git('show',source.INTEGRATION_RECORD_COMMIT+':'+name))
                inventory['entries'].pop();data=(json.dumps(inventory)+'\n').encode()
                record['current_inputs']['sha256']=hashlib.sha256(data).hexdigest();changes=[(name,data)]
            elif case=='chain':record['integration']['source_chain'].pop()
            elif case=='changes':record['changes'].pop()
            elif case=='event':record['event_base']=source.EVENT_BASE
            elif case=='native':record['native_record']['source']=documentation.PINNED_SOURCE
            else:record['review']=[True]
            revision,digest=self.integration_record_fixture(record,changes)
            with patch.object(source,'INTEGRATION_RECORD_COMMIT',revision),patch.object(source,'INTEGRATION_RECORD_SHA256',digest),patch.object(documentation,'CONTEXT_RECORD_COMMIT',revision),self.subTest(case=case),self.assertRaisesRegex(ValueError,code):
                source.load_integration(ROOT)

    def test_integration_preserves_source_and_predecessor_evidence(self):
        original=json.loads(git('show',source.INTEGRATION_RECORD_COMMIT+':'+source.INTEGRATION_RECORD_PATH))
        for name in (source.SOCIAL_RECORD_PATH,'docs/CONTRIBUTING.md'):
            revision,digest=self.integration_record_fixture(original,[(name,b'changed')])
            with patch.object(source,'INTEGRATION_RECORD_COMMIT',revision),patch.object(source,'INTEGRATION_RECORD_SHA256',digest),patch.object(documentation,'CONTEXT_RECORD_COMMIT',revision),self.subTest(name=name),self.assertRaisesRegex(ValueError,'production_integration_immutable_source_and_named_metadata|context_predecessor_artifact_drift'):
                source.load_integration(ROOT)

    def test_unreviewed_current_input(self):
        self.changed('src/backend/voice/main.go')
        with self.assertRaisesRegex(ValueError,'production_only_reviewed_carrier_delta'):source.qualify(ROOT)

    def test_record_missing_or_changed(self):
        for content in (None,b'{}'):
            self.changed(source.RECORD_PATH,content)
            with self.assertRaisesRegex(ValueError,'production_only_reviewed_carrier_delta'):source.qualify(ROOT)

    def test_carrier_bytes(self):
        self.changed(source.VERIFIER_PATH)
        with self.assertRaisesRegex(ValueError,'production_executed_carrier'):source.qualify(ROOT)

    def test_carrier_mode(self):
        self.changed(source.VERIFIER_PATH,(ROOT/source.VERIFIER_PATH).read_bytes(),'100755')
        with self.assertRaisesRegex(ValueError,'production_carrier_mode'):source.qualify(ROOT)

    def test_wrong_actual_event(self):
        os.environ['GITHUB_SHA']=self.original
        with self.assertRaisesRegex(ValueError,'production_actual_master_event'):source.qualify(ROOT)

    def test_exact_master_push(self):
        os.environ.update(GITHUB_EVENT_NAME='push',GITHUB_REF='refs/heads/master',GITHUB_BASE_REF='',
                          VOICE_CI_PUSH_BEFORE_SHA=self.event_base)
        self.assertEqual(source.qualify(ROOT)['target'],self.target)

    def test_other_push_and_wrong_before_rejected(self):
        for ref,before in (('refs/heads/develop',self.event_base),('refs/heads/master',self.original),
                           ('refs/heads/master','')):
            os.environ.update(GITHUB_EVENT_NAME='push',GITHUB_REF=ref,VOICE_CI_PUSH_BEFORE_SHA=before)
            with self.subTest(ref=ref,before=before),self.assertRaisesRegex(ValueError,'production_actual_master_event'):
                source.qualify(ROOT)

    def test_wrong_parent_order(self):
        target=self.commit(git('rev-parse',self.original+'^{tree}'),[self.original,self.event_base])
        git('update-ref','HEAD',target);os.environ['GITHUB_SHA']=target
        with self.assertRaisesRegex(ValueError,'production_actual_ordered_parents'):source.qualify(ROOT)

    def test_result_tree_drift(self):
        changed=self.changed('outside-unit-fixture.txt')
        target=self.commit(git('rev-parse',changed+'^{tree}'),[self.event_base,self.original])
        git('update-ref','HEAD',target);os.environ['GITHUB_SHA']=target
        with self.assertRaisesRegex(ValueError,'production_source_result_tree'):source.qualify(ROOT)

    def test_existing_destination_refused(self):
        with tempfile.TemporaryDirectory() as folder,self.assertRaisesRegex(ValueError,'production_fresh_input_directory'):
            source.materialize(ROOT,folder)

    def test_whole_M_and_F_production_constructors(self):
        with tempfile.TemporaryDirectory() as folder:
            inputs=Path(folder)/'inputs';source.materialize(ROOT,inputs)
            value=history.prepare_production_capabilities(ROOT,inputs/'unit')
            self.assertEqual(value['type'],'F54_FIXED_PRODUCTION_CAPABILITIES_v1')
            self.assertEqual(len(value['complete_41_entries']),41)
            self.assertEqual(len(value['complete_dag']),33)
            self.assertEqual(history.checked_capability_value(ROOT,value),value)
            binding=native.qualify(ROOT,self.event_base,self.original,self.target,'pull_request',
                'refs/pull/875/merge',self.target,'','master')
            native_path=Path(folder)/'native.json';native_path.write_text(json.dumps(binding))
            structural=history.prepare_3071_production_capabilities(ROOT,inputs/'fixed3071-unit',native_path)
            self.assertEqual(structural['type'],'F54_FIXED3071_PRODUCTION_CAPABILITIES_v1')
            self.assertEqual(history.checked_3071_capability_value(ROOT,structural),structural)
            entry=history.master_context(ROOT,value,native_path.read_bytes(),self.env)
            self.assertEqual(entry['type'],'F54_EXPLICIT_MASTER_PRODUCTION_ENTRY_v1')
            cap=history.prepare_corrected_F_production_capability(ROOT,inputs/'fixed3071-unit/corrected-F/source-unit.json')
            self.assertEqual(len(cap['whole']['complete_manual_paths']),33)
            self.assertEqual(len(cap['whole']['current_bindings']),34)
            self.assertEqual(cap['whole']['historical_validation_target'],history.F_CORRECTED_TUPLE['current'])
            self.assertEqual(cap['whole']['current_target'],self.target)
            self.assertNotEqual(cap['whole']['current_target'],cap['whole']['historical_validation_target'])
            self.assertEqual(history.checked_corrected_F_capability(ROOT,cap),cap)
            value['type']='F54_FIXED_CURRENT_AUTHOR_CAPABILITIES'
            with self.assertRaisesRegex(ValueError,'current_author_only_own_four_file_delta'):
                history.checked_capability_value(ROOT,value)

    def test_legacy_author_guard_unchanged(self):
        with self.assertRaisesRegex(ValueError,'current_author_only_own_four_file_delta'):
            history.current_author_applicability(ROOT)

    def test_unpinned_source_refused(self):
        with patch.object(source,'PINNED_RECORD_COMMIT',None),self.assertRaisesRegex(ValueError,'production_record_not_pinned'):
            source.qualify(ROOT)

    def test_successor_is_distinct_from_historical_source(self):
        value=source.qualify(ROOT)
        social=source.SOCIAL_SOURCE is not None
        rendering=source.RENDER_SOURCE is not None
        current=source.TRANSPORT_SOURCE is not None
        fixes=source.TEST_SOURCE is not None
        self.assertEqual(value['type'],source.FIXTURE_VERSION)
        self.assertEqual(value['reviewed_source'],source.FIXTURE_SOURCE)
        self.assertEqual(value['historical_validation_source'],self.record['source'])
        self.assertNotEqual(value['historical_validation_source'],value['reviewed_source'])
        self.assertEqual(value['predecessor_record'],source.INTEGRATION_RECORD_COMMIT)

    def test_transport_retains_exact_native_authority(self):
        value=source.qualify(ROOT)
        self.assertEqual(value['type'],source.FIXTURE_VERSION)
        record,prior,old=source.load_transport(ROOT)
        binding=native.qualify(ROOT,self.event_base,self.original,self.target,'pull_request',
            'refs/pull/875/merge',self.target,'','master')
        self.assertEqual(binding['record'],documentation.CONTEXT_RECORD_COMMIT)
        self.assertEqual(binding['snapshot'],documentation.CONTEXT_SOURCE)
        self.assertEqual(binding['version'],documentation.CONTEXT_VERSION)
        self.assertEqual(binding['record_sha256'],documentation.CONTEXT_RECORD_SHA256)
        self.assertEqual(value['record'],source.FIXTURE_RECORD_COMMIT)
        self.assertEqual(value['record_sha256'],source.FIXTURE_RECORD_SHA256)
        self.assertNotEqual(binding['record'],value['record'])
        self.assertEqual(binding['source_applicability'],dict(
            type='NATIVE-P3-v4-FIXTURE-SOURCE-APPLICABILITY-v1',
            source_snapshot=source.FIXTURE_SOURCE,record=source.FIXTURE_RECORD_COMMIT,
            record_sha256=source.FIXTURE_RECORD_SHA256,base=source.INTEGRATION_EVENT_BASE,
            source=value['source'],target=value['target']))
        self.assertEqual(binding['source_applicability']['record'],value['record'])
        self.assertEqual(binding['source_applicability']['record_sha256'],value['record_sha256'])
        self.assertNotIn(source.TRANSPORT_ADDITION['path'],binding['paths'])
        self.assertEqual(record['addition'],source.TRANSPORT_ADDITION)

    def test_transport_unpinned_never_falls_back(self):
        with patch.object(source,'TRANSPORT_RECORD_COMMIT',None),self.assertRaisesRegex(ValueError,'production_transport_not_pinned'):
            source.qualify(ROOT)

    def test_test_fixes_are_exact_and_keep_native_tuple(self):
        value=source.qualify(ROOT);record,previous,_,_=source.load_test_fixes(ROOT)
        self.assertEqual(value['type'],source.FIXTURE_VERSION)
        self.assertEqual(record['source'],source.TEST_SOURCE)
        self.assertEqual(record['native_record'],previous['native_record'])
        self.assertEqual({x['path']:x['after']['sha256'] for x in record['changes']},source.TEST_FIXES)
        binding=native.qualify(ROOT,self.event_base,self.original,self.target,'pull_request',
            'refs/pull/875/merge',self.target,'','master')
        self.assertEqual(binding['record'],documentation.CONTEXT_RECORD_COMMIT)
        self.assertEqual(binding['snapshot'],documentation.CONTEXT_SOURCE)
        self.assertTrue(set(source.TEST_FIXES).isdisjoint(binding['paths']))

    def test_test_fixes_unpinned_and_future_edits_fail_closed(self):
        with patch.object(source,'TEST_RECORD_COMMIT',None),self.assertRaisesRegex(ValueError,'production_test_fixes_not_pinned'):
            source.qualify(ROOT)
        for name in (*source.TEST_FIXES,'src/backend/messaging/internal/messageevents/third_test.go',source.TEST_RECORD_PATH):
            self.changed(name)
            with self.subTest(name=name),self.assertRaisesRegex(ValueError,'production_only_reviewed_carrier_delta'):
                source.qualify(ROOT)

    def fixes_record_fixture(self,record,changes=(),source_revision=None):
        # Helper name is deliberately not a unittest test case.
        raw=(json.dumps(record)+'\n').encode()
        with tempfile.TemporaryDirectory() as folder:
            env=dict(os.environ,GIT_INDEX_FILE=str(Path(folder)/'index'))
            git('read-tree',source.TEST_RECORD_COMMIT,env=env)
            for name,data in ((source.TEST_RECORD_PATH,raw),*changes):
                if data is None:git('update-index','--force-remove',name,env=env)
                else:
                    blob=git('hash-object','-w','--stdin',data=data)
                    git('update-index','--add','--cacheinfo','100644,'+blob+','+name,env=env)
            revision=self.commit(git('write-tree',env=env),[source_revision or source.TEST_SOURCE])
        return revision,hashlib.sha256(raw).hexdigest()

    def test_test_fixes_reconstruct_record_semantics(self):
        original=json.loads(git('show',source.TEST_RECORD_COMMIT+':'+source.TEST_RECORD_PATH))
        for alteration,code in (('inventory','production_test_fixes_complete_inventory'),
                ('native','production_test_fixes_retained_native_tuple'),('changes','production_test_fixes_exact_delta'),
                ('review','production_test_fixes_review')):
            record=json.loads(json.dumps(original));changes=[]
            if alteration=='inventory':
                name=record['current_inputs']['path'];inventory=json.loads(git('show',source.TEST_RECORD_COMMIT+':'+name))
                inventory['entries'].pop();raw=(json.dumps(inventory)+'\n').encode()
                record['current_inputs']['sha256']=hashlib.sha256(raw).hexdigest();changes=[(name,raw)]
            elif alteration=='native':record['native_record']['commit']=source.TEST_RECORD_COMMIT
            elif alteration=='changes':record['changes'][0]['after']['bytes']-=1
            else:record['review']=[True]
            revision,digest=self.fixes_record_fixture(record,changes)
            with patch.object(source,'TEST_RECORD_COMMIT',revision),patch.object(source,'TEST_RECORD_SHA256',digest),self.subTest(alteration=alteration),self.assertRaisesRegex(ValueError,code):
                source.load_test_fixes(ROOT)

    def test_test_fixes_exact_tree_not_test_allowlist(self):
        original=json.loads(git('show',source.TEST_RECORD_COMMIT+':'+source.TEST_RECORD_PATH))
        name=next(iter(source.TEST_FIXES))
        actual=subprocess.check_output(['git','-C',str(ROOT),'show',source.TEST_SOURCE+':'+name])
        for path,data,mode,code in (
            ('src/backend/messaging/internal/messageevents/third_test.go',b'extra','100644','production_test_fixes_exact_two_paths'),
            ('src/backend/voice/main.go',b'changed','100644','production_test_fixes_exact_two_paths'),
            (name,None,'100644','production_test_fixes_exact_two_paths'),
            (name,b'next edit','100644','production_test_fixes_exact_bytes'),
            (name,actual,'100755','production_test_fixes_file_mode')):
            with tempfile.TemporaryDirectory() as folder:
                env=dict(os.environ,GIT_INDEX_FILE=str(Path(folder)/'index'));git('read-tree',source.TEST_SOURCE,env=env)
                if data is None:git('update-index','--force-remove',path,env=env)
                else:
                    blob=git('hash-object','-w','--stdin',data=data)
                    git('update-index','--add','--cacheinfo',mode+','+blob+','+path,env=env)
                tree=git('write-tree',env=env);changed=self.commit(tree,[source.TEST_PREDECESSOR])
            record=json.loads(json.dumps(original));record.update(source=changed,source_tree=tree)
            revision,digest=self.fixes_record_fixture(record,source_revision=changed)
            with patch.object(source,'TEST_SOURCE',changed),patch.object(source,'TEST_RECORD_COMMIT',revision),patch.object(source,'TEST_RECORD_SHA256',digest),self.subTest(path=path,mode=mode),self.assertRaisesRegex(ValueError,code):
                source.load_test_fixes(ROOT)

    def test_test_fixes_preserve_old_artifacts_and_parent(self):
        record=json.loads(git('show',source.TEST_RECORD_COMMIT+':'+source.TEST_RECORD_PATH))
        for path in (source.TRANSPORT_RECORD_PATH,record['evidence'][0]['path']):
            for data in (None,b'changed'):
                revision,digest=self.fixes_record_fixture(record,[(path,data)])
                code=('production_test_fixes_immutable_predecessor' if path==source.TRANSPORT_RECORD_PATH
                      else '^single_entry$' if data is None else '^production_artifact_hash$')
                with patch.object(source,'TEST_RECORD_COMMIT',revision),patch.object(source,'TEST_RECORD_SHA256',digest),self.subTest(path=path),self.assertRaisesRegex(ValueError,code):
                    source.load_test_fixes(ROOT)
        revision=self.commit(git('rev-parse',source.TEST_RECORD_COMMIT+'^{tree}'),[source.TEST_PREDECESSOR])
        with patch.object(source,'TEST_RECORD_COMMIT',revision),self.assertRaisesRegex(ValueError,'production_test_fixes_record_parent'):
            source.load_test_fixes(ROOT)
        changed=self.commit(git('rev-parse',source.TEST_SOURCE+'^{tree}'),[source.TRANSPORT_RECORD_COMMIT])
        record['source']=changed
        revision,digest=self.fixes_record_fixture(record,source_revision=changed)
        with patch.object(source,'TEST_SOURCE',changed),patch.object(source,'TEST_RECORD_COMMIT',revision),patch.object(source,'TEST_RECORD_SHA256',digest),self.assertRaisesRegex(ValueError,'production_test_fixes_source_parent'):
            source.load_test_fixes(ROOT)

    def test_rendering_fix_are_exact_and_keep_native_tuple(self):
        value=source.qualify(ROOT);record,previous,_,_,_=source.load_rendering_fix(ROOT)
        self.assertEqual(value['type'],source.FIXTURE_VERSION)
        self.assertEqual(record['source'],source.RENDER_SOURCE)
        self.assertEqual(record['native_record'],previous['native_record'])
        self.assertEqual({x['path']:x['after']['sha256'] for x in record['changes']},source.RENDER_FIXES)
        binding=native.qualify(ROOT,self.event_base,self.original,self.target,'pull_request',
            'refs/pull/875/merge',self.target,'','master')
        self.assertEqual(binding['record'],documentation.CONTEXT_RECORD_COMMIT)
        self.assertEqual(binding['snapshot'],documentation.CONTEXT_SOURCE)
        self.assertTrue(set(source.RENDER_FIXES).isdisjoint(binding['paths']))

    def test_rendering_fix_unpinned_and_future_edits_fail_closed(self):
        with patch.object(source,'RENDER_RECORD_COMMIT',None),self.assertRaisesRegex(ValueError,'production_rendering_fix_not_pinned'):
            source.qualify(ROOT)
        for name in (*source.RENDER_FIXES,'scripts/ci/another_historical_test.py',source.RENDER_RECORD_PATH):
            self.changed(name)
            with self.subTest(name=name),self.assertRaisesRegex(ValueError,'production_only_reviewed_carrier_delta'):
                source.qualify(ROOT)

    def rendering_record_fixture(self,record,changes=(),source_revision=None):
        # Helper name is deliberately not a unittest test case.
        raw=(json.dumps(record)+'\n').encode()
        with tempfile.TemporaryDirectory() as folder:
            env=dict(os.environ,GIT_INDEX_FILE=str(Path(folder)/'index'))
            git('read-tree',source.RENDER_RECORD_COMMIT,env=env)
            for name,data in ((source.RENDER_RECORD_PATH,raw),*changes):
                if data is None:git('update-index','--force-remove',name,env=env)
                else:
                    blob=git('hash-object','-w','--stdin',data=data)
                    git('update-index','--add','--cacheinfo','100644,'+blob+','+name,env=env)
            revision=self.commit(git('write-tree',env=env),[source_revision or source.RENDER_SOURCE])
        return revision,hashlib.sha256(raw).hexdigest()

    def test_rendering_fix_reconstruct_record_semantics(self):
        original=json.loads(git('show',source.RENDER_RECORD_COMMIT+':'+source.RENDER_RECORD_PATH))
        for alteration,code in (('inventory','production_rendering_fix_complete_inventory'),
                ('native','production_rendering_fix_retained_native_tuple'),('changes','production_rendering_fix_exact_delta'),
                ('review','production_rendering_fix_review')):
            record=json.loads(json.dumps(original));changes=[]
            if alteration=='inventory':
                name=record['current_inputs']['path'];inventory=json.loads(git('show',source.RENDER_RECORD_COMMIT+':'+name))
                inventory['entries'].pop();raw=(json.dumps(inventory)+'\n').encode()
                record['current_inputs']['sha256']=hashlib.sha256(raw).hexdigest();changes=[(name,raw)]
            elif alteration=='native':record['native_record']['commit']=source.RENDER_RECORD_COMMIT
            elif alteration=='changes':record['changes'][0]['after']['bytes']-=1
            else:record['review']=[True]
            revision,digest=self.rendering_record_fixture(record,changes)
            with patch.object(source,'RENDER_RECORD_COMMIT',revision),patch.object(source,'RENDER_RECORD_SHA256',digest),self.subTest(alteration=alteration),self.assertRaisesRegex(ValueError,code):
                source.load_rendering_fix(ROOT)

    def test_rendering_fix_exact_tree_not_helper_allowlist(self):
        original=json.loads(git('show',source.RENDER_RECORD_COMMIT+':'+source.RENDER_RECORD_PATH))
        name=next(iter(source.RENDER_FIXES))
        actual=subprocess.check_output(['git','-C',str(ROOT),'show',source.RENDER_SOURCE+':'+name])
        for path,data,mode,code in (
            ('scripts/ci/another_historical_test.py',b'extra','100644','production_rendering_fix_exact_two_paths'),
            ('src/backend/voice/main.go',b'changed','100644','production_rendering_fix_exact_two_paths'),
            (name,None,'100644','production_rendering_fix_exact_two_paths'),
            (name,b'next edit','100644','production_rendering_fix_exact_bytes'),
            (name,actual,'100755','production_rendering_fix_file_mode')):
            with tempfile.TemporaryDirectory() as folder:
                env=dict(os.environ,GIT_INDEX_FILE=str(Path(folder)/'index'));git('read-tree',source.RENDER_SOURCE,env=env)
                if data is None:git('update-index','--force-remove',path,env=env)
                else:
                    blob=git('hash-object','-w','--stdin',data=data)
                    git('update-index','--add','--cacheinfo',mode+','+blob+','+path,env=env)
                tree=git('write-tree',env=env);changed=self.commit(tree,[source.RENDER_PREDECESSOR])
            record=json.loads(json.dumps(original));record.update(source=changed,source_tree=tree)
            revision,digest=self.rendering_record_fixture(record,source_revision=changed)
            with patch.object(source,'RENDER_SOURCE',changed),patch.object(source,'RENDER_RECORD_COMMIT',revision),patch.object(source,'RENDER_RECORD_SHA256',digest),self.subTest(path=path,mode=mode),self.assertRaisesRegex(ValueError,code):
                source.load_rendering_fix(ROOT)

    def test_rendering_fix_preserve_old_artifacts_and_parent(self):
        record=json.loads(git('show',source.RENDER_RECORD_COMMIT+':'+source.RENDER_RECORD_PATH))
        for path in (source.TEST_RECORD_PATH,record['evidence'][0]['path']):
            for data in (None,b'changed'):
                revision,digest=self.rendering_record_fixture(record,[(path,data)])
                code=('production_rendering_fix_immutable_predecessor' if path==source.TEST_RECORD_PATH
                      else '^single_entry$' if data is None else '^production_artifact_hash$')
                with patch.object(source,'RENDER_RECORD_COMMIT',revision),patch.object(source,'RENDER_RECORD_SHA256',digest),self.subTest(path=path),self.assertRaisesRegex(ValueError,code):
                    source.load_rendering_fix(ROOT)
        revision=self.commit(git('rev-parse',source.RENDER_RECORD_COMMIT+'^{tree}'),[source.RENDER_PREDECESSOR])
        with patch.object(source,'RENDER_RECORD_COMMIT',revision),self.assertRaisesRegex(ValueError,'production_rendering_fix_record_parent'):
            source.load_rendering_fix(ROOT)
        changed=self.commit(git('rev-parse',source.RENDER_SOURCE+'^{tree}'),[source.TEST_RECORD_COMMIT])
        record['source']=changed
        revision,digest=self.rendering_record_fixture(record,source_revision=changed)
        with patch.object(source,'RENDER_SOURCE',changed),patch.object(source,'RENDER_RECORD_COMMIT',revision),patch.object(source,'RENDER_RECORD_SHA256',digest),self.assertRaisesRegex(ValueError,'production_rendering_fix_source_parent'):
            source.load_rendering_fix(ROOT)

    def test_social_fix_are_exact_and_keep_native_tuple(self):
        value=source.qualify(ROOT);record,previous,_,_,_,_=source.load_social_fix(ROOT)
        self.assertEqual(value['type'],source.FIXTURE_VERSION)
        self.assertEqual(value['reviewed_source'],source.FIXTURE_SOURCE)
        self.assertEqual(record['native_record'],previous['native_record'])
        self.assertEqual({x['path']:x['after']['sha256'] for x in record['changes']},source.SOCIAL_FIXES)
        binding=native.qualify(ROOT,self.event_base,self.original,self.target,'pull_request',
            'refs/pull/875/merge',self.target,'','master')
        self.assertEqual(binding['record'],documentation.CONTEXT_RECORD_COMMIT)
        self.assertEqual(binding['snapshot'],documentation.CONTEXT_SOURCE)
        self.assertTrue(set(source.SOCIAL_FIXES).isdisjoint(binding['paths']))

    def test_social_fix_unpinned_and_future_edits_fail_closed(self):
        with patch.object(source,'SOCIAL_RECORD_COMMIT',None),self.assertRaisesRegex(ValueError,'production_social_fix_not_pinned'):
            source.qualify(ROOT)
        for name in (*source.SOCIAL_FIXES,'src/backend/social/internal/socialevents/another_test.go',source.SOCIAL_RECORD_PATH):
            self.changed(name)
            with self.subTest(name=name),self.assertRaisesRegex(ValueError,'production_only_reviewed_carrier_delta'):
                source.qualify(ROOT)

    def social_record_fixture(self,record,changes=(),source_revision=None):
        # Helper name is deliberately not a unittest test case.
        raw=(json.dumps(record)+'\n').encode()
        with tempfile.TemporaryDirectory() as folder:
            env=dict(os.environ,GIT_INDEX_FILE=str(Path(folder)/'index'))
            git('read-tree',source.SOCIAL_RECORD_COMMIT,env=env)
            for name,data in ((source.SOCIAL_RECORD_PATH,raw),*changes):
                if data is None:git('update-index','--force-remove',name,env=env)
                else:
                    blob=git('hash-object','-w','--stdin',data=data)
                    git('update-index','--add','--cacheinfo','100644,'+blob+','+name,env=env)
            revision=self.commit(git('write-tree',env=env),[source_revision or source.SOCIAL_SOURCE])
        return revision,hashlib.sha256(raw).hexdigest()

    def test_social_fix_reconstruct_record_semantics(self):
        original=json.loads(git('show',source.SOCIAL_RECORD_COMMIT+':'+source.SOCIAL_RECORD_PATH))
        for alteration,code in (('inventory','production_social_fix_complete_inventory'),
                ('native','production_social_fix_retained_native_tuple'),('changes','production_social_fix_exact_delta'),
                ('review','production_social_fix_review')):
            record=json.loads(json.dumps(original));changes=[]
            if alteration=='inventory':
                name=record['current_inputs']['path'];inventory=json.loads(git('show',source.SOCIAL_RECORD_COMMIT+':'+name))
                inventory['entries'].pop();raw=(json.dumps(inventory)+'\n').encode()
                record['current_inputs']['sha256']=hashlib.sha256(raw).hexdigest();changes=[(name,raw)]
            elif alteration=='native':record['native_record']['commit']=source.SOCIAL_RECORD_COMMIT
            elif alteration=='changes':record['changes'][0]['after']['bytes']-=1
            else:record['review']=[True]
            revision,digest=self.social_record_fixture(record,changes)
            with patch.object(source,'SOCIAL_RECORD_COMMIT',revision),patch.object(source,'SOCIAL_RECORD_SHA256',digest),self.subTest(alteration=alteration),self.assertRaisesRegex(ValueError,code):
                source.load_social_fix(ROOT)

    def test_social_fix_exact_tree_not_test_allowlist(self):
        original=json.loads(git('show',source.SOCIAL_RECORD_COMMIT+':'+source.SOCIAL_RECORD_PATH))
        name=next(iter(source.SOCIAL_FIXES))
        actual=subprocess.check_output(['git','-C',str(ROOT),'show',source.SOCIAL_SOURCE+':'+name])
        for path,data,mode,code in (
            ('src/backend/social/internal/socialevents/another_test.go',b'extra','100644','production_social_fix_exact_one_path'),
            ('src/backend/voice/main.go',b'changed','100644','production_social_fix_exact_one_path'),
            (name,None,'100644','production_social_fix_exact_one_path'),
            (name,b'next edit','100644','production_social_fix_exact_bytes'),
            (name,actual,'100755','production_social_fix_file_mode')):
            with tempfile.TemporaryDirectory() as folder:
                env=dict(os.environ,GIT_INDEX_FILE=str(Path(folder)/'index'));git('read-tree',source.SOCIAL_SOURCE,env=env)
                if data is None:git('update-index','--force-remove',path,env=env)
                else:
                    blob=git('hash-object','-w','--stdin',data=data)
                    git('update-index','--add','--cacheinfo',mode+','+blob+','+path,env=env)
                tree=git('write-tree',env=env);changed=self.commit(tree,[source.SOCIAL_PREDECESSOR])
            record=json.loads(json.dumps(original));record.update(source=changed,source_tree=tree)
            revision,digest=self.social_record_fixture(record,source_revision=changed)
            with patch.object(source,'SOCIAL_SOURCE',changed),patch.object(source,'SOCIAL_RECORD_COMMIT',revision),patch.object(source,'SOCIAL_RECORD_SHA256',digest),self.subTest(path=path,mode=mode),self.assertRaisesRegex(ValueError,code):
                source.load_social_fix(ROOT)

    def test_social_fix_preserve_old_artifacts_and_parent(self):
        record=json.loads(git('show',source.SOCIAL_RECORD_COMMIT+':'+source.SOCIAL_RECORD_PATH))
        for path in (source.RENDER_RECORD_PATH,record['evidence'][0]['path']):
            for data in (None,b'changed'):
                revision,digest=self.social_record_fixture(record,[(path,data)])
                code=('production_social_fix_immutable_predecessor' if path==source.RENDER_RECORD_PATH
                      else '^single_entry$' if data is None else '^production_artifact_hash$')
                with patch.object(source,'SOCIAL_RECORD_COMMIT',revision),patch.object(source,'SOCIAL_RECORD_SHA256',digest),self.subTest(path=path),self.assertRaisesRegex(ValueError,code):
                    source.load_social_fix(ROOT)
        revision=self.commit(git('rev-parse',source.SOCIAL_RECORD_COMMIT+'^{tree}'),[source.SOCIAL_PREDECESSOR])
        with patch.object(source,'SOCIAL_RECORD_COMMIT',revision),self.assertRaisesRegex(ValueError,'production_social_fix_record_parent'):
            source.load_social_fix(ROOT)
        changed=self.commit(git('rev-parse',source.SOCIAL_SOURCE+'^{tree}'),[source.RENDER_RECORD_COMMIT])
        record['source']=changed
        revision,digest=self.social_record_fixture(record,source_revision=changed)
        with patch.object(source,'SOCIAL_SOURCE',changed),patch.object(source,'SOCIAL_RECORD_COMMIT',revision),patch.object(source,'SOCIAL_RECORD_SHA256',digest),self.assertRaisesRegex(ValueError,'production_social_fix_source_parent'):
            source.load_social_fix(ROOT)

    def test_transport_added_evidence_cannot_change_or_expand(self):
        for name in (source.TRANSPORT_ADDITION['path'],source.TRANSPORT_RECORD_PATH,
                     'docs/testing/native-p3-source-v2-evidence/another-companion.json'):
            for data,mode in ((None,'100644'),(b'changed','100644'),(b'changed','100755')):
                if data is None and name.endswith('/another-companion.json'):
                    continue  # Removing an absent path makes no source change.
                self.changed(name,data,mode)
                with self.subTest(name=name,data=data,mode=mode),self.assertRaisesRegex(ValueError,'production_only_reviewed_carrier_delta'):
                    source.qualify(ROOT)

    def transport_record_fixture(self,record,changes=(),source_revision=None):
        raw=(json.dumps(record)+'\n').encode()
        with tempfile.TemporaryDirectory() as folder:
            env=dict(os.environ,GIT_INDEX_FILE=str(Path(folder)/'index'))
            git('read-tree',source.TRANSPORT_RECORD_COMMIT,env=env)
            for name,data in ((source.TRANSPORT_RECORD_PATH,raw),*changes):
                if data is None:
                    git('update-index','--force-remove',name,env=env)
                    continue
                blob=git('hash-object','-w','--stdin',data=data)
                git('update-index','--add','--cacheinfo','100644,'+blob+','+name,env=env)
            revision=self.commit(git('write-tree',env=env),[source_revision or source.TRANSPORT_SOURCE])
        return revision,hashlib.sha256(raw).hexdigest()

    def test_transport_record_preserves_predecessor_and_exact_parent(self):
        record=json.loads(git('show',source.TRANSPORT_RECORD_COMMIT+':'+source.TRANSPORT_RECORD_PATH))
        for data in (None,b'changed pinned review'):
            revision,digest=self.transport_record_fixture(record,
                [('docs/testing/native-p3-source-v2-evidence/role-space-review.md',data)])
            with patch.object(source,'TRANSPORT_RECORD_COMMIT',revision),patch.object(source,'TRANSPORT_RECORD_SHA256',digest),self.assertRaisesRegex(ValueError,'production_transport_immutable_predecessor'):
                source.load_transport(ROOT)
        revision=self.commit(git('rev-parse',source.TRANSPORT_RECORD_COMMIT+'^{tree}'),[source.TRANSPORT_PREDECESSOR])
        with patch.object(source,'TRANSPORT_RECORD_COMMIT',revision),self.assertRaisesRegex(ValueError,'production_transport_record_parent'):
            source.load_transport(ROOT)
        changed=self.commit(git('rev-parse',source.TRANSPORT_SOURCE+'^{tree}'),[source.SUCCESSOR_RECORD_COMMIT])
        record['source']=changed
        revision,digest=self.transport_record_fixture(record,source_revision=changed)
        with patch.object(source,'TRANSPORT_SOURCE',changed),patch.object(source,'TRANSPORT_RECORD_COMMIT',revision),patch.object(source,'TRANSPORT_RECORD_SHA256',digest),self.assertRaisesRegex(ValueError,'production_transport_source_parent'):
            source.load_transport(ROOT)

    def test_transport_record_semantics_not_only_hash(self):
        original=json.loads(git('show',source.TRANSPORT_RECORD_COMMIT+':'+source.TRANSPORT_RECORD_PATH))
        for alteration,code in (('inventory','production_transport_complete_inventory'),
            ('native','production_transport_retained_native_tuple'),('addition','production_transport_exact_addition'),
            ('review','production_transport_review')):
            record=json.loads(json.dumps(original));changes=[]
            if alteration=='inventory':
                name=record['current_inputs']['path']
                inventory=json.loads(git('show',source.TRANSPORT_RECORD_COMMIT+':'+name));inventory['entries'].pop()
                raw=(json.dumps(inventory)+'\n').encode();record['current_inputs']['sha256']=hashlib.sha256(raw).hexdigest()
                changes=[(name,raw)]
            elif alteration=='native':record['native_record']['commit']=source.TRANSPORT_RECORD_COMMIT
            elif alteration=='addition':record['addition']['bytes']-=1
            else:record['review']=[True]
            revision,digest=self.transport_record_fixture(record,changes)
            with patch.object(source,'TRANSPORT_RECORD_COMMIT',revision),patch.object(source,'TRANSPORT_RECORD_SHA256',digest),self.subTest(alteration=alteration),self.assertRaisesRegex(ValueError,code):
                source.load_transport(ROOT)

    def test_transport_source_is_exact_addition_not_doc_allowlist(self):
        original=json.loads(git('show',source.TRANSPORT_RECORD_COMMIT+':'+source.TRANSPORT_RECORD_PATH))
        for name,data,mode,code in (
            ('docs/testing/another-review.json',b'{}','100644','production_transport_only_missing_companion'),
            ('docs/testing/native-p3-source-v2-evidence/role-space-review.md',b'changed','100644','production_transport_only_missing_companion'),
            (source.TRANSPORT_ADDITION['path'],b'changed','100644','production_transport_companion_bytes'),
            (source.TRANSPORT_ADDITION['path'],(ROOT/source.TRANSPORT_ADDITION['path']).read_bytes(),'100755','production_transport_companion_bytes')):
            with tempfile.TemporaryDirectory() as folder:
                env=dict(os.environ,GIT_INDEX_FILE=str(Path(folder)/'index'))
                git('read-tree',source.TRANSPORT_SOURCE,env=env)
                blob=git('hash-object','-w','--stdin',data=data)
                git('update-index','--add','--cacheinfo',mode+','+blob+','+name,env=env)
                tree=git('write-tree',env=env)
                changed=self.commit(tree,[source.TRANSPORT_PREDECESSOR])
            record=json.loads(json.dumps(original));record.update(source=changed,source_tree=tree)
            revision,digest=self.transport_record_fixture(record,source_revision=changed)
            with patch.object(source,'TRANSPORT_SOURCE',changed),patch.object(source,'TRANSPORT_RECORD_COMMIT',revision),patch.object(source,'TRANSPORT_RECORD_SHA256',digest),self.subTest(name=name,mode=mode),self.assertRaisesRegex(ValueError,code):
                source.load_transport(ROOT)

    def test_successor_record_missing_changed_or_mode_changed(self):
        for name in (source.SUCCESSOR_RECORD_PATH,'docs/testing/f13-production-successor-closure-v3.json'):
            for content,mode in ((None,'100644'),(b'{}','100644'),((ROOT/name).read_bytes(),'100755')):
                self.changed(name,content,mode)
                with self.subTest(name=name,mode=mode),self.assertRaisesRegex(ValueError,'production_only_reviewed_carrier_delta'):
                    source.qualify(ROOT)

    def test_successor_does_not_grant_future_frontend_changes(self):
        for name in (*source.SUCCESSOR_PATHS,'src/frontend/lib/backend/new_unreviewed.dart'):
            self.changed(name)
            with self.subTest(name=name),self.assertRaisesRegex(ValueError,'production_only_reviewed_carrier_delta'):
                source.qualify(ROOT)

    def test_successor_unpinned_never_falls_back(self):
        with patch.object(source,'SUCCESSOR_RECORD_COMMIT',None),self.assertRaisesRegex(ValueError,'production_successor_not_pinned'):
            source.qualify(ROOT)

    def test_successor_preserves_archive_and_old_evidence(self):
        for name in (source.ARCHIVE_PATH,self.record['evidence'][0]['path'],
                     'src/backend/voice/internal/new_unreviewed.go'):
            self.changed(name)
            with self.subTest(name=name),self.assertRaisesRegex(ValueError,'production_only_reviewed_carrier_delta'):
                source.qualify(ROOT)

    def test_successor_reconstructs_inventory_not_just_its_pin(self):
        original=json.loads(git('show',source.SUCCESSOR_RECORD_COMMIT+':'+source.SUCCESSOR_RECORD_PATH))
        for field,code in (('current_inputs','production_successor_complete_inventory'),
                           ('successor_closure','production_successor_complete_closure')):
            record=json.loads(json.dumps(original));name=record[field]['path']
            artifact=json.loads(git('show',source.SUCCESSOR_RECORD_COMMIT+':'+name))
            if field=='current_inputs':artifact['entries'].pop()
            else:artifact['participating']['native_compiled'].pop()
            raw=(json.dumps(artifact)+'\n').encode();record[field]['sha256']=hashlib.sha256(raw).hexdigest()
            record_raw=(json.dumps(record)+'\n').encode()
            with tempfile.TemporaryDirectory() as folder:
                env=dict(os.environ,GIT_INDEX_FILE=str(Path(folder)/'index'))
                git('read-tree',source.SUCCESSOR_RECORD_COMMIT,env=env)
                for path,data in ((name,raw),(source.SUCCESSOR_RECORD_PATH,record_raw)):
                    blob=git('hash-object','-w','--stdin',data=data)
                    git('update-index','--add','--cacheinfo','100644,'+blob+','+path,env=env)
                revision=self.commit(git('write-tree',env=env),[source.SUCCESSOR_SOURCE])
            with patch.object(source,'SUCCESSOR_RECORD_COMMIT',revision),patch.object(documentation,'PINNED_RECORD_COMMIT',revision),patch.object(source,'SUCCESSOR_RECORD_SHA256',hashlib.sha256(record_raw).hexdigest()),self.assertRaisesRegex(ValueError,code):
                source.load_successor(ROOT)

    def test_actual_3071_composition_requires_all_eleven_manual_inputs(self):
        with tempfile.TemporaryDirectory() as folder:
            inputs=Path(folder)/'inputs';source.materialize(ROOT,inputs)
            binding=native.qualify(ROOT,self.event_base,self.original,self.target,'pull_request',
                'refs/pull/875/merge',self.target,'','master')
            native_path=Path(folder)/'native.json';native_path.write_text(json.dumps(binding))
            cap=history.prepare_3071_production_capabilities(ROOT,inputs/'fixed3071-unit',native_path)
            cap_path=Path(folder)/'cap.json';cap_path.write_text(json.dumps(cap))
            manual=inputs/'fixed3071-unit/manual';facts=json.loads((manual/'facts.json').read_text())
            data={name:(manual/name).read_bytes() for name in facts['bindings']}
            self.assertEqual(len(data),11)
            name='protos/voice/auth/v1/auth.proto'
            args=(ROOT,cap_path,history.J3071,history.J3071_O,history.J3071_P1,history.J3071_P2,
                  name,history.tree_entry(ROOT,history.J3071,name).rstrip('\0'))
            for filename in data:(manual/filename).unlink()
            with self.assertRaisesRegex(ValueError,'fixed3071_all_manual_evidence_bindings'):
                history.composition_3071_capability(*args)
            for filename,raw in data.items():(manual/filename).write_bytes(raw)
            result=history.composition_3071_capability(*args)
            self.assertEqual(result['composition'],'REVIEWED_MANUAL_WHOLE')
            self.assertEqual(result['merge_tree_exit'],1)
            for filename,raw in data.items():
                (manual/filename).write_bytes(raw+b'altered')
                with self.subTest(filename=filename),self.assertRaisesRegex(ValueError,'fixed3071_all_manual_evidence_bindings'):
                    history.composition_3071_capability(*args)
                (manual/filename).write_bytes(raw)

    def test_applicable_initialization_failure_never_falls_back(self):
        shell=(ROOT/'scripts/ci/voice-db-runtime-provisioning-contract_test.sh').read_text()
        functions='\n'.join(re.search(r'(?ms)^'+name+r'\(\) \{\n.*?^\}',shell).group(0)
            for name in ('f13_f54_production_applicable','f13_f54_collect_production_master_unit'))
        start=shell.index('if f13_f54_production_applicable ||')
        route=shell[start:shell.index('  if [[ "${f13_collect_status}" != 0 ]]',start)]+'fi\n'
        with tempfile.TemporaryDirectory() as directory:
            for phase in ('native-failed','existing-partial-trace'):
                folder=Path(directory)/phase;folder.mkdir()
                if phase=='existing-partial-trace':(folder/'f13-production-trace.jsonl').write_text('partial trace\n')
                program='set -eu\n'+functions+'\n'+'''
f13_develop_aggregate_enabled() { return 1; }
f13_collect_master_push_contributions() { touch "$TMP_DIR/fallback"; }
f13_collect_master_release_pr_contributions() { touch "$TMP_DIR/fallback"; }
f13_collect_develop_contributions() { touch "$TMP_DIR/fallback"; }
f13_collect_status=0
f13_master_release_pr=false
f13_master_merge_push=false
native_p3_master_pr=false
base_sha="$VOICE_R22_BASE_SHA"
'''+route+'[[ "$f13_collect_status" != 0 && ! -e "$TMP_DIR/fallback" ]]\n'
                env=dict(os.environ,ROOT=str(ROOT),TMP_DIR=str(folder),VOICE_R22_BASE_SHA=self.event_base,
                         native_p3_active='true' if phase=='existing-partial-trace' else 'false')
                result=subprocess.run(['bash','-c',program],env=env,capture_output=True,text=True)
                self.assertEqual(result.returncode,0,result.stderr)


if __name__=='__main__':unittest.main()
