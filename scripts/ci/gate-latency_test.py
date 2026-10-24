import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location('latency', Path(__file__).with_name('gate-latency.py'))
latency = importlib.util.module_from_spec(spec)
spec.loader.exec_module(latency)


class LatencyTests(unittest.TestCase):
    def fixture(self, status='completed'):
        run = {'id': 1, 'html_url': 'run', 'head_sha': 'a'*40, 'created_at': '2026-10-24T00:00:00Z',
               'status': status, 'conclusion': 'failure', 'run_attempt': 2}
        job = {'name': 'ci-gate', 'html_url': 'job', 'created_at': '2026-10-24T00:04:00Z',
               'started_at': '2026-10-24T00:04:10Z', 'completed_at': '2026-10-24T00:05:00Z',
               'conclusion': 'failure', 'steps': []}
        return run, [{'jobs': [{**job, 'started_at': '2026-10-24T00:00:10Z',
                               'created_at': '2026-10-24T00:00:00Z',
                               'completed_at': '2026-10-24T00:02:00Z'}]}, {'jobs': [job]}]

    def test_wall_includes_retry_gap_and_failure_is_visible(self):
        row = latency.summarize(*self.fixture(), 'CI/global')
        self.assertEqual(row['gate_wall_seconds'], 300)
        self.assertEqual(row['initial_queue_seconds'], 10)
        self.assertEqual(row['execution_envelope_seconds'], 290)
        self.assertEqual(row['attempts'], 2)
        self.assertEqual(row['gate_conclusion'], 'failure')

    def test_incomplete_and_missing_gate_are_censored(self):
        row = latency.summarize(*self.fixture('in_progress'), 'Backend')
        self.assertIsNone(row['gate_wall_seconds'])
        run, attempts = self.fixture()
        attempts[-1]['jobs'] = []
        self.assertIsNone(latency.summarize(run, attempts, 'Backend')['gate_wall_seconds'])

    def test_nearest_rank_and_empty_sample(self):
        self.assertEqual(latency.aggregate([])['n'], 0)
        result = latency.aggregate([{'gate_wall_seconds': n} for n in range(1, 21)])
        self.assertEqual(result['p95_seconds'], 19)
        self.assertEqual(result['mean_seconds'], 10.5)


if __name__ == '__main__':
    unittest.main()
