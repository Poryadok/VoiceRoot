"""Ensure the real chain fixture stays reachable through required CI."""
import fnmatch
from pathlib import Path
import unittest
import yaml

ROOT = Path(__file__).resolve().parents[2]


class ChainRoutingTests(unittest.TestCase):
    def test_affected_inputs_select_required_shards_and_publication_validation(self):
        filters = yaml.safe_load((ROOT / '.github/ci/path-filters.yml').read_text())
        workflow = yaml.safe_load((ROOT / '.github/workflows/dev-candidate-publish.yml').read_text())
        events = workflow.get('on', workflow.get(True))
        paths = events['pull_request']['paths']
        for path in ('scripts/dev/story_media_backup_cohort_chain_test.py',
                     'scripts/dev/story_media_backup_capture_test.py',
                     'scripts/dev/story_media_backup_restore.py',
                     'scripts/staging/github_custody.py',
                     'src/backend/migrations/voice_db/000015_fixture.up.sql',
                     'scripts/ci/story-media-cohort-chain.sh',
                     'scripts/ci/story-media-cohort-chain.Dockerfile'):
            with self.subTest(path=path):
                self.assertTrue(any(fnmatch.fnmatchcase(path, p) for p in filters['ci_scripts']))
                self.assertTrue(any(fnmatch.fnmatchcase(path, p) for p in paths))

    def test_runner_is_invoked_in_required_runtime_shard_and_publication(self):
        make = (ROOT / 'Makefile').read_text()
        runtime = make.split('ci-script-tests-runtime:', 1)[1].split('ci-script-tests-staging:', 1)[0]
        self.assertIn('scripts/ci/story-media-cohort-chain.sh', runtime)
        workflow = yaml.safe_load((ROOT / '.github/workflows/dev-candidate-publish.yml').read_text())
        self.assertTrue(any('bash scripts/ci/story-media-cohort-chain.sh' in step.get('run', '')
                            for step in workflow['jobs']['validate']['steps']))


if __name__ == '__main__': unittest.main()
