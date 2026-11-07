import subprocess
import tempfile
import unittest
from copy import deepcopy
from pathlib import Path

import f13_source_dag as dag


class RealGitDagTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.repo = Path(self.tmp.name)
        self.git('init', '-q', '-b', 'master')
        self.git('config', 'user.name', 'Test')
        self.git('config', 'user.email', 'test@example.invalid')

    def git(self, *args):
        return subprocess.check_output(['git', '-C', str(self.repo), *args],
                                       stderr=subprocess.PIPE).decode().strip()

    def commit(self, value):
        (self.repo / 'unit').write_text(value, encoding='utf-8')
        self.git('add', 'unit')
        self.git('commit', '-qm', value)
        return self.git('rev-parse', 'HEAD')

    def test_change_restore_and_merge_parent_edges_are_recorded(self):
        base = self.commit('base')
        self.git('checkout', '-qb', 'side')
        change = self.commit('changed')
        restore = self.commit('base')
        self.git('checkout', '-q', 'master')
        self.git('commit', '--allow-empty', '-qm', 'other')
        first = self.git('rev-parse', 'HEAD')
        self.git('merge', '--no-ff', '-qm', 'merge', 'side')
        source = self.git('rev-parse', 'HEAD')
        result = dag.inventory(self.repo, base, source)
        raw = dag.raw_history(self.repo, base, source)
        self.assertIn(change.encode(), raw)
        self.assertIn(restore.encode(), raw)
        self.assertIn((source + ' ').encode(), raw)
        rows = {row['commit']: row for row in result['commits']}
        self.assertEqual(set(rows), {change, restore, first, source})
        self.assertEqual(rows[source]['parents'], [first, restore])
        self.assertEqual(len(rows[source]['edges']), 2)
        self.assertNotEqual(rows[change]['edges'][0]['raw_sha256'],
                            rows[restore]['edges'][0]['raw_sha256'])
        self.assertEqual(result['status'], 'OPEN')
        self.assertFalse(result['authority'])
        dag.verify(self.repo, result, base, source)
        for mutation in ('missing_commit', 'parent_order', 'raw', 'status', 'authority_type'):
            altered = deepcopy(result)
            if mutation == 'missing_commit':
                altered['commits'].pop(0)
            elif mutation == 'parent_order':
                altered['commits'][-1]['parents'].reverse()
            elif mutation == 'raw':
                next(row for row in altered['commits'] if row['commit'] == change)['edges'][0]['raw_base64'] = ''
            elif mutation == 'status':
                altered['status'] = 'ACCEPTED'
            else:
                altered['authority'] = 0
            with self.subTest(mutation=mutation), self.assertRaises(ValueError):
                dag.verify(self.repo, altered, base, source)


if __name__ == '__main__':
    unittest.main()
