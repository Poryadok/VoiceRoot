import hashlib
import json
import shutil
import subprocess
import tempfile
from pathlib import Path
import unittest
from unittest.mock import patch

import local_ci_staging_request as request_contract
import local_ci_staging_worker as worker
import local_ci_staging_dispatch as dispatch
import local_ci_matrix_inventory as matrix


class WorkerBindingTests(unittest.TestCase):
    def test_prewarm_tuning_is_scoped_to_build_command(self):
        root = Path(__file__).resolve().parents[2]
        source = (root / 'scripts/ci/nats-rollout-preservation_test.sh').read_text(encoding='utf-8')
        hashes = {name: hashlib.sha256((root / 'scripts/staging/nats-known-baseline' / name).read_bytes()).hexdigest()
                  for name in ('go.mod', 'go.sum')}
        _, data = worker.build_adapter(source, hashes)
        tuned = b'timeout 600 env GOGC=30 GOMEMLIMIT=1200MiB go build github.com/nats-io/nats-server/v2/server;'
        self.assertEqual(data.count(tuned), 1)
        self.assertEqual(data.count(b'GOGC=30'), 1)
        self.assertEqual(data.count(b'GOMEMLIMIT=1200MiB'), 1)
        self.assertIn(b'python -m unittest "$@"', data)
        self.assertIn(b'python -m unittest controller_test root_main_test docker_runtime_test stage_runtime_test', data)

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        parent = Path(self.tmp.name)
        root = parent / 'source'
        root.mkdir()
        self.request = {'root': str(root), 'output': str(parent / 'output'),
                        'commit': 'a' * 40, 'tree': 'b' * 40, 'base': 'c' * 40, 'shard': 'staging',
                        'assets': {name: str(parent / name) for name in request_contract.ASSET_NAMES}}
        worker.configure(self.request)

    def test_request_sets_source_base_and_exact_asset_paths(self):
        self.assertEqual(worker.SOURCE_COMMIT, 'a' * 40)
        self.assertEqual(worker.SOURCE_TREE, 'b' * 40)
        self.assertEqual(worker.SOURCE_BASE, 'c' * 40)
        self.assertEqual(worker.SHARD, 'staging')
        self.assertEqual([row[0] for row in worker.IMAGES],
                         [Path(self.request['assets'][key]) for key in
                          ('postgres_image', 'go_image', 'python_image', 'nats_image', 'nats_box_image', 'alpine_image')])

    def test_kubectl_requires_absolute_verified_unchanged_file(self):
        path = Path(self.tmp.name) / 'kubectl'
        path.write_bytes(b'client bytes')
        digest = hashlib.sha256(path.read_bytes()).hexdigest()
        with patch.object(worker, 'KUBECTL_HASH', digest):
            worker.verify_policy_kubectl(path)
            with self.assertRaises(ValueError): worker.verify_policy_kubectl(Path('relative'))
            path.write_bytes(b'changed')
            with self.assertRaises(ValueError): worker.verify_policy_kubectl(path)

    def test_host_identity_and_cleanliness_fail_closed(self):
        good = [b'a' * 40 + b'\n', b'b' * 40 + b'\n', b'']
        with patch.object(worker, 'host_git', side_effect=good): worker.verify_host()
        for bad in ([b'd' * 40 + b'\n'], [good[0], b'd' * 40 + b'\n'], [good[0], good[1], b'?? foreign\n']):
            with self.subTest(bad=bad), patch.object(worker, 'host_git', side_effect=bad), self.assertRaises(RuntimeError):
                worker.verify_host()

    def test_current_pinned_matrix_generates_both_complete_dispatchers(self):
        root = Path(__file__).resolve().parents[2]
        inventory = matrix.build(root)
        for shard, count in (('staging', 34), ('proofs', 15)):
            with self.subTest(shard=shard):
                recipes = dispatch.inventory(inventory['shards'][shard], shard=shard)
                self.assertEqual(len(recipes), count)
                wrappers = dispatch.wrappers(recipes, shard=shard)
                self.assertEqual(set(wrappers), {'bash', 'python'})
                if shard == 'staging':
                    self.assertEqual({kind: hashlib.sha256(data).hexdigest() for kind, data in wrappers.items()}, {
                        'bash': 'ebf2214ac2a6aaccc3d7f99903d103ed8623b2c65a7342ce3130f49a985401d8',
                        'python': '3d0cdaba229063d5a482328c75d3d5e53fb8daefab3806bcd92ff811c799d09d'})

    def test_hosted_go_json_and_download_diagnostics_stay_separate(self):
        bash = r'C:\Program Files\Git\bin\bash.exe' if Path(r'C:\Program Files\Git\bin\bash.exe').exists() else shutil.which('bash')
        rows = [{'Action':'start','Package':'p'}, {'Action':'run','Package':'p','Test':'TestA'},
                {'Action':'pass','Package':'p','Test':'TestA'}, {'Action':'pass','Package':'p'}]
        payload = '\n'.join(json.dumps(row, separators=(',', ':')) for row in rows)
        def run(fake):
            return subprocess.run([bash, '-c', fake + '\n' + worker.hosted_go_script('.')],
                                  capture_output=True, timeout=10)
        result = run("go() { printf '%s\\n' '" + payload.replace('\n', "' '") + "'; echo 'go: downloading example' >&2; }")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stderr, b'go: downloading example\n')
        self.assertEqual(dispatch.validate_go_json(result.stdout, ['p']), 1)
        failed = run("go() { echo 'go: failure' >&2; return 7; }")
        self.assertEqual(failed.returncode, 7)
        with self.assertRaises(ValueError): dispatch.validate_go_json(b'{malformed\n', ['p'])
        with self.assertRaises(ValueError): dispatch.validate_go_json(b'{"Action":"skip","Package":"p"}\n', ['p'])
        stderr_overflow = run('go() { head -c 16777218 /dev/zero >&2; }')
        self.assertEqual(len(stderr_overflow.stderr), 16777217)
        stdout_overflow = run('go() { head -c 16777218 /dev/zero; }')
        self.assertEqual(len(stdout_overflow.stdout), 16777217)
        with tempfile.TemporaryDirectory() as directory:
            logs = Path(directory)
            (logs / 'hosted-go.stdout').write_bytes(result.stdout)
            (logs / 'hosted-go.stderr').write_bytes(result.stderr)
            self.assertEqual(worker.validate_hosted_go_logs(logs, ['p'], dispatch), 1)
            with (logs / 'hosted-go.stderr').open('wb') as stream: stream.truncate(16777217)
            with self.assertRaisesRegex(RuntimeError, 'diagnostics exceeded'):
                worker.validate_hosted_go_logs(logs, ['p'], dispatch)


if __name__ == '__main__':
    unittest.main()
