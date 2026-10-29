from pathlib import Path
import os
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import realtime_p3_create_only_source as source


class SourceTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(); self.addCleanup(self.tmp.cleanup); self.root = Path(self.tmp.name)
        for index,name in enumerate(source.PATHS):
            path = self.root/name; path.parent.mkdir(parents=True,exist_ok=True); path.write_bytes(('fixture-'+str(index)).encode())
        self.issuer = 'a'*40; self.candidate = 'b'*40
        self.values = {name:(self.root/name).read_bytes() for name in source.PATHS}

    def test_exact_reviewed_candidate_and_disk_bytes(self):
        with patch.object(source,'REVIEWED_ISSUER_SHA',self.issuer), \
             patch.object(source,'_blob',side_effect=lambda root,sha,name:self.values[name]):
            source.verify(self.root,self.issuer,self.candidate)
            (self.root/source.PATHS[0]).write_bytes(b'changed')
            with self.assertRaises(source.SourceError): source.verify(self.root,self.issuer,self.candidate)

    def test_unknown_issuer_candidate_drift_and_symlink_fail(self):
        with patch.object(source,'REVIEWED_ISSUER_SHA',self.issuer), \
             patch.object(source,'_blob',side_effect=lambda root,sha,name:b'drift' if sha == self.candidate else self.values[name]):
            with self.assertRaises(source.SourceError): source.verify(self.root,self.issuer,self.candidate)
            with self.assertRaises(source.SourceError): source.verify(self.root,'f'*40,self.candidate)
        path = self.root/source.PATHS[0]; path.unlink(); path.symlink_to(self.root/source.PATHS[1])
        with patch.object(source,'REVIEWED_ISSUER_SHA',self.issuer), \
             patch.object(source,'_blob',side_effect=lambda root,sha,name:self.values[name]):
            with self.assertRaises(source.SourceError): source.verify(self.root,self.issuer,self.candidate)

    def test_real_git_blob_comparison_rejects_candidate_drift(self):
        env = dict(os.environ, GIT_AUTHOR_NAME='Fixture', GIT_AUTHOR_EMAIL='fixture@example.invalid',
                   GIT_COMMITTER_NAME='Fixture', GIT_COMMITTER_EMAIL='fixture@example.invalid')
        def git(*args):
            return subprocess.run(['git', '-C', str(self.root), *args], env=env, check=True,
                                  stdout=subprocess.PIPE, stderr=subprocess.PIPE).stdout.decode().strip()
        git('init', '-q'); git('add', '.'); git('commit', '-qm', 'issuer')
        issuer = git('rev-parse', 'HEAD')
        with patch.object(source, 'REVIEWED_ISSUER_SHA', issuer):
            source.verify(self.root, issuer, issuer)
            (self.root/source.PATHS[0]).write_bytes(b'changed')
            git('add', '.'); git('commit', '-qm', 'candidate drift')
            with self.assertRaises(source.SourceError): source.verify(self.root, issuer, git('rev-parse', 'HEAD'))


if __name__ == '__main__': unittest.main()
