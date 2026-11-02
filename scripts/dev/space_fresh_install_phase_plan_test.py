import hashlib
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import space_fresh_install_phase_plan as plan


class FixedPlanTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(); self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        template = self.root / plan.TEMPLATE
        template.parent.mkdir(parents=True)
        original = Path(__file__).resolve().parents[2] / plan.TEMPLATE
        template.write_bytes(original.read_bytes())
        directory = self.root / plan.SQL_DIR; directory.mkdir(parents=True)
        (directory / '000001_init.up.sql').write_bytes(b'SELECT 1;\n')
        (directory / '000001_init.down.sql').write_bytes(b'SELECT 2;\n')
        self.files = {p.relative_to(self.root).as_posix(): hashlib.sha256(p.read_bytes()).hexdigest()
                      for p in self.root.rglob('*') if p.is_file()}
        self.source = 'a'*40; self.operation = 'media-c1-'+'b'*32
        self.image = 'example.invalid/space-native@sha256:'+'c'*64
        self.build = 'd'*64

    def render(self):
        with patch.object(plan.publish_candidate, 'source_snapshot', return_value={'source_sha':self.source,'files':self.files}):
            return plan.render(self.root,self.source,self.operation,self.image,self.build,'node-a')

    def test_fixed_job_and_immutable_sql_match_source(self):
        cm, job = self.render()
        self.assertTrue(cm['immutable'])
        self.assertEqual(cm['data']['000001_init.up.sql'],'SELECT 1;\n')
        self.assertEqual(job['spec']['backoffLimit'],0)
        pod = job['spec']['template']['spec']; container = pod['containers'][0]
        self.assertEqual(pod['restartPolicy'],'Never')
        self.assertEqual(pod['nodeName'],'node-a')
        self.assertEqual(container['image'],self.image)
        self.assertEqual(container['env'][-1], {'name':'SPACE_NATIVE_REQUIRED_SOURCE_SHA256','value':self.build})
        self.assertEqual(pod['volumes'][0]['configMap']['name'],cm['metadata']['name'])

    def test_source_drift_and_nonimmutable_image_hold(self):
        (self.root / plan.SQL_DIR / '000001_init.up.sql').write_bytes(b'changed')
        with self.assertRaises(plan.PlanError): self.render()
        with patch.object(plan.publish_candidate, 'source_snapshot', return_value={'source_sha':self.source,'files':self.files}):
            with self.assertRaises(plan.PlanError): plan.render(self.root,self.source,self.operation,'example.invalid/latest',self.build,'node-a')

    def test_tracked_gitattributes_is_verified_but_not_published_as_sql(self):
        path = self.root / plan.SQL_DIR / '.gitattributes'
        path.write_bytes(b'*.sql text eol=lf\n')
        self.files[plan.SQL_DIR + '/.gitattributes'] = hashlib.sha256(path.read_bytes()).hexdigest()
        cm, _ = self.render()
        self.assertNotIn('.gitattributes', cm['data'])
        path.write_bytes(b'changed\n')
        with self.assertRaises(plan.PlanError): self.render()

    def test_untracked_metadata_or_unknown_entry_is_refused(self):
        for name in ('.gitattributes', 'README', 'invalid.sql'):
            path = self.root / plan.SQL_DIR / name
            path.write_bytes(b'unexpected')
            with self.subTest(name=name), self.assertRaises(plan.PlanError): self.render()
            path.unlink()

    def test_qualified_image_sql_directory_is_not_overlaid_by_projected_volume(self):
        _, job = self.render()
        pod = job['spec']['template']['spec']
        container = pod['containers'][0]
        self.assertFalse(any(m['mountPath'] == '/migrations' for m in container['volumeMounts']))
        self.assertIn('--migrations-dir=/migrations', container['args'][0])

    def test_complete_checkout_inventory_includes_tracked_metadata(self):
        repo = Path(__file__).resolve().parents[2]
        self.root = repo
        paths = [repo / plan.TEMPLATE, *(repo / plan.SQL_DIR).iterdir()]
        self.files = {p.relative_to(repo).as_posix(): hashlib.sha256(p.read_bytes()).hexdigest()
                      for p in paths}
        self.assertIn(plan.SQL_DIR + '/.gitattributes', self.files)
        cm, _ = self.render()
        expected = {p.name for p in paths if p.suffix == '.sql'}
        self.assertEqual(set(cm['data']), expected)
        self.assertGreater(len(expected), 80)


if __name__ == '__main__': unittest.main()
