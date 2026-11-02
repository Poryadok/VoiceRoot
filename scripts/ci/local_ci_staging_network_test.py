import ast
from pathlib import Path
import shutil
import subprocess
import unittest

import local_ci_staging_network as policy


class NetworkPolicyTests(unittest.TestCase):
    def test_rejects_unknown_wrapper_and_duplicate_policy(self):
        with self.assertRaises(ValueError):
            policy.apply('unrecognized wrapper')
        sample = '''python_image=0
    --network)
      [ "$n" -ge 2 ] && [ "$1" = none ] || { echo 'unsupported network option' >&2; exit 2; }
      shift; n=$((n-2)); continue ;;
    --net|--net=*|--network=*|--network-*|--pull|--pull=*)
set -- run --pull=never --network none "$@"
'''
        result = policy.apply(sample)
        self.assertIn('network_seen=0', result)
        self.assertIn('container:voice-nats-proof-*', result)
        with self.assertRaises(ValueError):
            policy.apply(result)

    def test_actual_worker_wrapper_refuses_host_and_duplicate_networks(self):
        worker = Path(__file__).with_name('local_ci_staging_worker.py').read_text(encoding='utf-8')
        tree = ast.parse(worker)
        candidates = [node.value for node in ast.walk(tree) if isinstance(node, ast.Assign)
                      and len(node.targets) == 1 and isinstance(node.targets[0], ast.Name)
                      and node.targets[0].id == 'wrapper' and isinstance(node.value, ast.Constant)]
        self.assertEqual(len(candidates), 1)
        wrapper = policy.apply(ast.literal_eval(candidates[0]))
        wrapper = wrapper.replace('exec /run/user/1000/tools/docker-cli -H unix:///run/user/1000/docker.sock "$@"',
                                  'printf "%s\\n" "$@"')
        bash = r'C:\Program Files\Git\bin\bash.exe' if Path(r'C:\Program Files\Git\bin\bash.exe').exists() else shutil.which('bash')
        def call(*args):
            return subprocess.run([bash, '-c', wrapper, 'wrapper', *args], capture_output=True, timeout=10)
        self.assertEqual(call('run', '--network', 'voice-nats-proof-123', 'nats:2.12-alpine').returncode, 0)
        for args in (('run', '--network', 'host', 'nats:2.12-alpine'),
                     ('run', '--network', 'none', '--network', 'none', 'nats:2.12-alpine'),
                     ('run', '--pull=always', 'nats:2.12-alpine')):
            with self.subTest(args=args): self.assertNotEqual(call(*args).returncode, 0)


if __name__ == '__main__':
    unittest.main()
