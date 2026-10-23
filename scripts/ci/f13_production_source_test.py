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

ROOT=Path(__file__).resolve().parents[2]


def git(*args,data=None,env=None):
    return subprocess.check_output(['git','--no-replace-objects','-C',str(ROOT),*args],input=data,env=env).decode().strip()


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
        cls.target=cls.commit(git('rev-parse',cls.original+'^{tree}'),[source.EVENT_BASE,cls.original])
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
        target=self.commit(git('rev-parse',candidate+'^{tree}'),[source.EVENT_BASE,candidate])
        git('update-ref','HEAD',target);os.environ['GITHUB_SHA']=target
        return target

    def test_complete_current_source(self):
        value=source.qualify(ROOT)
        self.assertEqual(value['target'],self.target)
        self.assertEqual(value['source'],self.original)
        self.assertEqual(value['historical_collector'],source.HISTORICAL_SCOPE)
        self.assertNotEqual(value['base'],value['historical_collector']['base'])
        self.assertFalse(value['admission']);self.assertFalse(value['current_or_future_path_authority'])

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
                          VOICE_CI_PUSH_BEFORE_SHA=source.EVENT_BASE)
        self.assertEqual(source.qualify(ROOT)['target'],self.target)

    def test_other_push_and_wrong_before_rejected(self):
        for ref,before in (('refs/heads/develop',source.EVENT_BASE),('refs/heads/master',self.original),
                           ('refs/heads/master','')):
            os.environ.update(GITHUB_EVENT_NAME='push',GITHUB_REF=ref,VOICE_CI_PUSH_BEFORE_SHA=before)
            with self.subTest(ref=ref,before=before),self.assertRaisesRegex(ValueError,'production_actual_master_event'):
                source.qualify(ROOT)

    def test_wrong_parent_order(self):
        target=self.commit(git('rev-parse',self.original+'^{tree}'),[self.original,source.EVENT_BASE])
        git('update-ref','HEAD',target);os.environ['GITHUB_SHA']=target
        with self.assertRaisesRegex(ValueError,'production_actual_ordered_parents'):source.qualify(ROOT)

    def test_result_tree_drift(self):
        changed=self.changed('outside-unit-fixture.txt')
        target=self.commit(git('rev-parse',changed+'^{tree}'),[source.EVENT_BASE,self.original])
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
            binding=native.qualify(ROOT,source.EVENT_BASE,self.original,self.target,'pull_request',
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
                env=dict(os.environ,ROOT=str(ROOT),TMP_DIR=str(folder),VOICE_R22_BASE_SHA=source.EVENT_BASE,
                         native_p3_active='true' if phase=='existing-partial-trace' else 'false')
                result=subprocess.run(['bash','-c',program],env=env,capture_output=True,text=True)
                self.assertEqual(result.returncode,0,result.stderr)


if __name__=='__main__':unittest.main()
