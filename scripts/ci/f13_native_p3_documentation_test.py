"""Real Git checks for the fixed documentation-only Native successor."""
import os
import tempfile
from pathlib import Path
import unittest
from unittest.mock import patch

import f13_native_p3_documentation as documentation
import f13_production_source_test as fixtures


class ContextSourceUnitTest(unittest.TestCase):
    def test_exact_makefile_context_reconstructs_old_unit(self):
        current=documentation.load_context_record(fixtures.ROOT)
        old=documentation.load_record(fixtures.ROOT)
        self.assertEqual(current['projection'],old['projection'])
        self.assertEqual(len(current['projection']),99)
        self.assertEqual(current['compiled_closure'],old['compiled_closure'])
        changed=[a['path'] for a,b in zip(current['context'],old['context']) if a!=b]
        self.assertEqual(changed,['Makefile'])
        self.assertEqual(current['source'],documentation.CONTEXT_SOURCE)

    def test_unpinned_context_rejected(self):
        with patch.object(documentation,'CONTEXT_RECORD_COMMIT',None),self.assertRaisesRegex(ValueError,'context_successor_not_pinned'):
            documentation.load_context_record(fixtures.ROOT)


class DocumentationSourceTest(unittest.TestCase):
    commit=fixtures.ProductionSourceTest.__dict__['commit']
    @classmethod
    def setUpClass(cls):
        fixtures.ProductionSourceTest.setUpClass.__func__(cls)
        current=documentation.load_context_record(fixtures.ROOT)
        old=documentation.load_record(fixtures.ROOT)
        actual=next(r['entry'] for r in current['context'] if r['path']=='Makefile')
        expected=next(r['entry'] for r in old['context'] if r['path']=='Makefile')
        documentation.legacy.require(documentation.legacy.entry(fixtures.ROOT,cls.original,'Makefile')==actual,
            'v3_fixture_exact_reviewed_context')
        with tempfile.TemporaryDirectory() as folder:
            env=dict(os.environ,GIT_INDEX_FILE=str(Path(folder)/'index'))
            fixtures.git('read-tree',cls.original,env=env)
            fixtures.git('update-index','--add','--cacheinfo',expected['mode']+','+expected['oid']+',Makefile',env=env)
            cls.original=cls.commit(fixtures.git('write-tree',env=env),[cls.original])
        cls.event_base=fixtures.source.EVENT_BASE
        cls.target=cls.commit(fixtures.git('rev-parse',cls.original+'^{tree}'),[cls.event_base,cls.original])
        cls.env['GITHUB_SHA']=cls.target
    tearDownClass=fixtures.ProductionSourceTest.__dict__['tearDownClass']
    setUp=fixtures.ProductionSourceTest.setUp
    tearDown=fixtures.ProductionSourceTest.tearDown
    changed=fixtures.ProductionSourceTest.changed

    def qualify(self):
        target=fixtures.git('rev-parse','HEAD')
        source=fixtures.git('rev-list','--parents','-n','1',target).split()[2]
        return documentation.qualify(fixtures.ROOT,fixtures.source.EVENT_BASE,source,target,
            'pull_request','refs/pull/875/merge',os.environ['GITHUB_SHA'],'','master')

    def test_exact_current_documentation_tuple(self):
        binding=self.qualify()
        self.assertEqual(binding['version'],documentation.VERSION)
        self.assertEqual(binding['snapshot'],documentation.PINNED_SOURCE)
        self.assertEqual(len(binding['paths']),99)
        self.assertIn(documentation.DOCUMENT,binding['paths'])
        self.assertTrue(binding['retain_guarded_and_runtime_history'])

    def test_unpinned_documentation_is_not_permission(self):
        with patch.object(documentation,'PINNED_RECORD_COMMIT',None),self.assertRaisesRegex(ValueError,'documentation_successor_not_pinned'):
            self.qualify()

    def test_same_document_future_bytes_rejected(self):
        self.changed(documentation.DOCUMENT)
        with self.assertRaisesRegex(ValueError,'projection_drift'):self.qualify()

    def test_other_projection_and_context_rejected(self):
        for name,code in (('docs/DATA_STORES.md','projection_drift'),
                          ('src/backend/pkg/authoritysource/postgres.go','context_drift')):
            self.changed(name)
            with self.subTest(name=name),self.assertRaisesRegex(ValueError,code):self.qualify()

    def test_new_dependency_path_rejected(self):
        self.changed('src/backend/pkg/authoritysource/undocumented_dependency.go')
        with self.assertRaisesRegex(ValueError,'documentation_dependency_directory_drift'):self.qualify()

    def test_predecessor_and_current_records_remain_immutable(self):
        for name in (documentation.RECORD_PATH,documentation.previous.RECORD_PATH,documentation.legacy.RECORD_PATH):
            self.changed(name,b'{}')
            with self.subTest(name=name),self.assertRaisesRegex(ValueError,'documentation_record_or_evidence_drift'):
                self.qualify()

    def test_wrong_event_parent_order_rejected(self):
        target=self.commit(fixtures.git('rev-parse',self.original+'^{tree}'),[self.original,fixtures.source.EVENT_BASE])
        fixtures.git('update-ref','HEAD',target);os.environ['GITHUB_SHA']=target
        with self.assertRaisesRegex(ValueError,'master_shape_or_before'):self.qualify()


if __name__=='__main__':unittest.main()
