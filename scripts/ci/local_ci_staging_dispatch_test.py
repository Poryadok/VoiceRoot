import importlib.util
import ast
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest
import local_ci_matrix_inventory as matrix

spec = importlib.util.spec_from_file_location('dispatch', Path(__file__).with_name('local_ci_staging_dispatch.py'))
m = importlib.util.module_from_spec(spec); spec.loader.exec_module(m)
def rows():
    return [{'recipe':'$(BASH) "$(ROOT)/'+(m.PRESERVATION if i == 0 else 'scripts/ci/test%d.sh'%i)+'"'} for i in range(34)]

class DispatchTests(unittest.TestCase):
    def test_runtime_current_provisioning_marker_matches_exact_recipe(self):
        root = Path(__file__).resolve().parents[2]
        recipes = m.inventory(matrix.build(root)['shards']['runtime'], shard='runtime')
        self.assertEqual(recipes[2], ('bash', 'scripts/ci/voice-db-runtime-provisioning-contract_test.sh'))
        source = (root / recipes[2][1]).read_text(encoding='utf-8')
        marker = 'Voice DB F05-F12 current provisioning/docs contract: PASS'
        self.assertIn("echo '" + marker + "'", source)
        m.validate_recipe(2, (marker + '\n').encode(), shard='runtime')
        for bad in (b'Voice DB RED-F runtime/provisioning/docs contract: PASS\n',
                    (marker + '\n' + marker + '\n').encode(), b''):
            with self.subTest(output=bad), self.assertRaises(ValueError):
                m.validate_recipe(2, bad, shard='runtime')
    def test_phase0_complementary_proof_binds_exact_source_and_tests(self):
        root = Path(__file__).resolve().parents[2]
        proof_id = '__main__.RestorerPathTests.test_cleanup_restores_service_owned_fixture_for_real_caller_uid'
        normal_id = '__main__.FixtureTests.test_general_fixture_inputs_remain_readable_by_the_test_user'
        skipped = (f'test_cleanup_restores_service_owned_fixture_for_real_caller_uid ({proof_id}) ... skipped '
                   "'requires Linux root to exercise real service and caller UIDs'\n")
        normal = f'test_general_fixture_inputs_remain_readable_by_the_test_user ({normal_id}) ... ok\n'
        tree = ast.parse((root / 'scripts/ci/phase0-fixture_test.py').read_text(encoding='utf-8'))
        names = [f'__main__.{cls.name}.{method.name}' for cls in tree.body if isinstance(cls, ast.ClassDef)
                 for method in cls.body if isinstance(method, ast.FunctionDef) and method.name.startswith('test_')]
        others = ''.join(f'{name.rsplit(".", 1)[1]} ({name}) ... ok\n'
                         for name in names if name not in (normal_id, proof_id))
        suite = (normal + others + skipped + '\nRan 78 tests in 1s\n\nOK (skipped=1)\n').encode()
        proof = (f'test_cleanup_restores_service_owned_fixture_for_real_caller_uid ({proof_id}) ... ok\n'
                 '\nRan 1 test in 1s\n\nOK\n').encode()
        m.validate_phase0_proof(root, suite, proof)
        with self.assertRaises(ValueError): m.validate_make_log(suite, shard='runtime')
        m.validate_make_log(suite, shard='runtime', phase0_proof_verified=True)
        with self.assertRaises(ValueError): m.validate_make_log(suite + b'\nother skipped\n', shard='runtime', phase0_proof_verified=True)
        for bad_suite, bad_proof in ((suite, b'Ran 1 test in 1s\nOK\n'),
                                     (suite.replace(b'... ok', b'... skipped', 1), proof),
                                     (suite, proof.replace(b'... ok', b'... skipped')),
                                     (suite.replace(b'Ran 78 tests', b'Ran 77 tests'), proof)):
            with self.assertRaises(ValueError):
                m.validate_phase0_proof(root, bad_suite, bad_proof)
    def test_policy_skip_logger_records_real_test_identity(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'sitecustomize.py').write_bytes(m.policy_skip_logger())
            log = root / 'skips.jsonl'; log.write_bytes(b'')
            program = ('import unittest\nclass T(unittest.TestCase):\n'
                       ' @unittest.skip("reviewed reason")\n def test_case(self): pass\n'
                       'unittest.main()\n')
            env = dict(os.environ, PYTHONPATH=str(root), VOICE_CI_SKIP_LOG=str(log), VOICE_CI_RECIPE_INDEX='21')
            result = subprocess.run([sys.executable, '-B', '-c', program], env=env, capture_output=True, timeout=20)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(m.parse_policy_skip_events(log.read_bytes()), {21: {('__main__.T.test_case', 'reviewed reason')}})
            with self.assertRaises(ValueError): m.parse_policy_skip_events(log.read_bytes() * 2)
            with self.assertRaises(ValueError): m.parse_policy_skip_events(b'not-json\n')
    def test_policy_skips_are_named_source_pinned_and_counted(self):
        root = Path(__file__).resolve().parents[2]
        rows = matrix.build(root)['shards']['policy']
        expected = m.policy_expected_skips(root, rows)
        recipes = m.inventory(rows, shard='policy')
        self.assertEqual(len(recipes), 69)
        self.assertIn(b'export VOICE_CI_RECIPE_INDEX="$idx"', m.wrappers(recipes, shard='policy')['python'])
        self.assertEqual({index: len(items) for index, items in expected.items()}, {21: 1, 24: 33, 33: 1, 41: 2})
        event = next(iter(expected[21]))
        summary = b'Ran 3 tests in 0.1s\nOK (skipped=1)\n'
        m.validate_policy_recipe(21, summary, {event}, expected)
        for bad in (set(), {('foreign.test', event[1])}, {event, ('extra.test', event[1])}):
            with self.subTest(bad=bad), self.assertRaises(ValueError):
                m.validate_policy_recipe(21, summary, bad, expected)
        with self.assertRaises(ValueError): m.validate_policy_recipe(21, b'Ran 1 test in 0s\nOK (skipped=1)\n', {event}, expected)
        for bad in (b'Ran 0 tests in 0s\nOK\n', b'Ran 2 tests in 0s\nFAILED (failures=1)\n',
                    b'Ran 2 tests in 0s\nOK (skipped=1)\n', b'OK\n',
                    b'Ran 2 tests in 0s\nOK\nRan 2 tests in 0s\nOK\n'):
            with self.subTest(output=bad), self.assertRaises(ValueError):
                m.validate_policy_recipe(0, bad, set(), {})
        with self.assertRaises(ValueError): m.validate_policy_recipe(1, b'skip (shallow clone has no HEAD^)\nAll resolve-staging-matrix tests passed.\n', set(), expected)
        m.validate_policy_recipe(3, b'== docs-only PR skips gate ==\nAll verify-required-jobs tests passed.\n', set(), expected)
        with tempfile.TemporaryDirectory() as directory:
            copy = Path(directory)
            for name, _, _ in m.POLICY_SKIP_FILES.values():
                target = copy / name; target.parent.mkdir(parents=True, exist_ok=True)
                target.write_bytes((root / name).read_bytes())
            target.write_bytes(target.read_bytes() + b'\n')
            with self.assertRaises(ValueError): m.policy_expected_skips(copy, rows)
    def test_story_adapter_binds_source_and_private_resources(self):
        root = Path(__file__).resolve().parents[2]
        source = (root / 'scripts/ci/story-media-cohort-chain.sh').read_bytes()
        dockerfile = (root / 'scripts/ci/story-media-cohort-chain.Dockerfile').read_bytes()
        cli_id, python_id = 'sha256:' + 'a' * 64, 'sha256:' + 'b' * 64
        adapter = m.story_media_adapter(source, dockerfile, cli_id, python_id)
        self.assertIn(b'docker build --network none --pull=false', adapter)
        self.assertIn(b'/run/user/1000/docker.sock:/var/run/docker.sock', adapter)
        self.assertIn(b'/run/user/1000/source-mirror:/repo:ro', adapter)
        self.assertNotIn(b'docker pull ', adapter)
        self.assertNotIn(b'-v /var/run/docker.sock:', adapter)
        self.assertIn(cli_id.encode(), adapter)
        self.assertIn(python_id.encode(), adapter)
        restored = adapter.decode()
        replacements = (
            ('ROOT="/workspace/repo"', 'ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"'),
            ("postgres='sha256:75f5a96988cdf694a215073c3e9c001b706b371e2f94df3967f2efdec2787f6b'",
             "postgres='postgres@sha256:cf78e76683b9ca8c5733cbbdce6c9262b45b6767934dd0a95e671f9a0fc20685'\ndocker pull --platform linux/amd64 \"$postgres\""),
            ('[[ "$(docker image inspect docker:27-cli --format \'{{.Id}}\')" == ' + cli_id + ' ]]\n'
             '[[ "$(docker image inspect python:3.12-bookworm --format \'{{.Id}}\')" == ' + python_id + ' ]]\n'
             'docker build --network none --pull=false --platform linux/amd64', 'docker build --platform linux/amd64'),
            ('-v /run/user/1000/docker.sock:/var/run/docker.sock -v /run/user/1000/source-mirror:/repo:ro',
             '-v /var/run/docker.sock:/var/run/docker.sock -v "$ROOT:/repo:ro"'),
        )
        for before, after in replacements:
            self.assertEqual(restored.count(before), 1)
            restored = restored.replace(before, after, 1)
        self.assertEqual(restored.encode(), source)
        with self.assertRaises(ValueError): m.story_media_adapter(source + b'\n', dockerfile, cli_id, python_id)
        with self.assertRaises(ValueError): m.story_media_adapter(source, dockerfile + b'\n', cli_id, python_id)
        with self.assertRaises(ValueError): m.story_media_adapter(source, dockerfile, 'unsafe', python_id)
    def test_runtime_discovery_recipes_require_exact_arguments(self):
        root = Path(__file__).resolve().parents[2]
        rows = matrix.build(root)['shards']['runtime']
        recipes = m.inventory(rows, shard='runtime')
        self.assertEqual(len(recipes), 23)
        discovery = [item for item in recipes if item[0] == 'discover']
        self.assertEqual(len(discovery), 3)
        self.assertEqual(discovery[0][1], ('scripts/release', 'capability_manifest_test.py'))
        self.assertEqual(m.command(discovery[0]), ['python3', '-m', 'unittest', 'discover', '-s',
                         '/workspace/repo/scripts/release', '-p', 'capability_manifest_test.py'])
        script = m.wrappers(recipes, shard='runtime')['python']
        self.assertIn(b'capability_manifest_test.py', script)
        self.assertNotIn(b'--owned-temp-root', script)
        self.assertIn(b'/run/user/1000/adapter-root/story-media.sh', m.wrappers(recipes, shard='runtime')['bash'])
        malformed = list(rows); malformed[12] = dict(malformed[12], recipe='python3 -m unittest discover -s "$(ROOT)/scripts/release" -p ../unsafe')
        with self.assertRaises(ValueError): m.inventory(malformed, shard='runtime')

    def test_runtime_discovery_wrapper_routes_only_exact_invocation(self):
        root = Path(__file__).resolve().parents[2]
        recipes = m.inventory(matrix.build(root)['shards']['runtime'], shard='runtime')
        bash = r'C:\Program Files\Git\bin\bash.exe' if Path(r'C:\Program Files\Git\bin\bash.exe').exists() else shutil.which('bash')
        with tempfile.TemporaryDirectory() as directory:
            temp = Path(directory); events = temp / 'events'
            wrapper = m.wrappers(recipes, shard='runtime')['python'].decode()
            wrapper = wrapper.replace('/usr/local/bin/python3', '/bin/true')
            wrapper = wrapper.replace('/run/user/1000/recipe-events', events.as_posix())
            wrapper = wrapper.replace('/run/user/1000/recipe-${idx}.log', (temp/'recipe-${idx}.log').as_posix())
            exact = m.command(recipes[12])[1:]
            result = subprocess.run([bash, '-c', wrapper, 'wrapper', *exact], capture_output=True, timeout=10)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(events.read_text().splitlines(), ['12\tstart', '12\texit\t0'])
            events.unlink()
            result = subprocess.run([bash, '-c', wrapper, 'wrapper', *exact[:-1], 'different_test.py'], capture_output=True, timeout=10)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertFalse(events.exists())

    def test_runtime_story_dispatches_only_the_pinned_adapter(self):
        root = Path(__file__).resolve().parents[2]
        recipes = m.inventory(matrix.build(root)['shards']['runtime'], shard='runtime')
        bash = r'C:\Program Files\Git\bin\bash.exe' if Path(r'C:\Program Files\Git\bin\bash.exe').exists() else shutil.which('bash')
        with tempfile.TemporaryDirectory() as directory:
            temp = Path(directory); events = temp / 'events'
            wrapper = m.wrappers(recipes, shard='runtime')['bash'].decode().replace('/bin/bash', '/bin/echo')
            wrapper = wrapper.replace('/run/user/1000/recipe-events', events.as_posix())
            wrapper = wrapper.replace('/run/user/1000/recipe-${idx}.log', (temp/'recipe-${idx}.log').as_posix())
            result = subprocess.run([bash, '-c', wrapper, 'wrapper', m.command(recipes[11])[1]], capture_output=True, timeout=10)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(events.read_text().splitlines(), ['11\tstart', '11\texit\t0'])
            self.assertIn(b'/run/user/1000/adapter-root/story-media.sh', result.stdout)
    def test_proofs_inventory_and_receipts_are_exact(self):
        entries = [{'recipe': '$(BASH) "$(ROOT)/scripts/ci/proof%d.sh"' % i} for i in range(15)]
        recipes = m.inventory(entries, shard='proofs')
        self.assertEqual(len(recipes), 15)
        self.assertNotIn(b'--owned-temp-root', m.wrappers(recipes, shard='proofs')['bash'])
        m.validate_recipe(0, b'Missing Docker Go module download helper test passed.\n', shard='proofs')
        m.validate_recipe(2, b'Ran 3 tests in 1s\nOK\n', shard='proofs')
        for bad in (b'', b'Ran 0 tests in 1s\nOK\n', b'Ran 3 tests in 1s\nOK (skipped=1)\n'):
            with self.subTest(bad=bad), self.assertRaises(ValueError): m.validate_recipe(2, bad, shard='proofs')
        with self.assertRaises(ValueError): m.inventory(entries[:-1], shard='proofs')
        with self.assertRaises(ValueError): m.inventory(entries, shard='runtime')
    def test_exact_events_and_missing_duplicate_failure(self):
        recipes = m.inventory(rows())
        events = ''.join(f'{i}\tstart\n{i}\texit\t0\n' for i in range(34)).encode()
        self.assertEqual(m.validate_events(events, recipes), 34)
        for bad in (events[:-1], events + b'0\tstart\n', events.replace(b'1\texit\t0',b'1\texit\t1'), events.replace(b'2\tstart\n',b'')):
            with self.subTest(bad=bad[:30]), self.assertRaises(ValueError): m.validate_events(bad, recipes)
    def test_inventory_rejects_unsupported_and_duplicate(self):
        entries = rows(); entries[4] = entries[3]
        with self.assertRaises(ValueError): m.inventory(entries)
        entries = rows(); entries[2] = {'recipe':'echo bypass'}
        with self.assertRaises(ValueError): m.inventory(entries)
    def test_skip_and_go_test_events_fail_closed(self):
        m.validate_make_log(b'normal output\n')
        with self.assertRaises(ValueError): m.validate_make_log(b'SKIP: missing prerequisite\n')
        events = [{'Action':'start','Package':'p'},{'Action':'run','Package':'p','Test':'TestA'},
                  {'Action':'pass','Package':'p','Test':'TestA'},{'Action':'pass','Package':'p'}]
        encode = lambda r: b'\n'.join(json.dumps(x).encode() for x in r)
        self.assertEqual(m.validate_go_json(encode(events),['p']),1)
        for value in ([{'Action':'skip','Package':'p','Test':'TestA'}],[{'Action':'pass','Package':'p'}],
                      events+[{'Action':'fail','Package':'p','Test':'TestB'}],events+[events[2]],events[:-1],
                      [events[1],events[0],events[2],events[3]],events+[events[3]]):
            with self.subTest(value=value), self.assertRaises(ValueError): m.validate_go_json(encode(value),['p'])
        with self.assertRaises(ValueError): m.validate_go_json(encode(events),['p','missing'])
    def test_wrappers_include_only_preservation_override(self):
        result = m.wrappers(m.inventory(rows()))
        self.assertEqual(result['bash'].count(b'--owned-temp-root'),1)
        self.assertNotIn(b'--owned-temp-root',result['python'])
    def test_actual_bash_wrapper_records_exit_and_preservation_root(self):
        bash = r'C:\Program Files\Git\bin\bash.exe' if Path(r'C:\Program Files\Git\bin\bash.exe').exists() else shutil.which('bash')
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory); script = root / 'preserve.sh'; events = root / 'events'
            script.write_text('printf "%s\\n" "$@"\nexit 7\n')
            wrapper = m.wrappers(m.inventory(rows()))['bash'].decode()
            wrapper = wrapper.replace(m.ROOT + m.PRESERVATION, script.as_posix())
            wrapper = wrapper.replace('/run/user/1000/recipe-events', events.as_posix())
            wrapper = wrapper.replace('/run/user/1000/recipe-${idx}.log', (root/'recipe-${idx}.log').as_posix())
            result = subprocess.run([bash,'-c',wrapper,'wrapper',script.as_posix()],capture_output=True,text=True,timeout=10)
            self.assertEqual(result.returncode,7)
            self.assertIn('--owned-temp-root',result.stdout)
            self.assertEqual(events.read_text().splitlines(),['0\tstart','0\texit\t7'])
    def test_recipe_markers_and_positive_counts(self):
        m.validate_recipe(0,b'Full rollout restores Auth before User.\n')
        for bad in (b'',b'Ran 0 tests in 0s\nOK\n',b'Ran 1 test in 1s\nOK (skipped=1)\n'):
            with self.subTest(bad=bad), self.assertRaises(ValueError): m.validate_recipe(4,bad)
        m.validate_recipe(4,b'Ran 1 test in 1s\nOK\n')
        preservation = b'Ran 3 tests in 1s\nOK\nRan 10 tests in 2s\nOK\nRan 4 tests in 1s\nOK\nNATS_ROLLOUT_PRESERVATION=PASS valid-receipt-paused-apply-and-second-use-veto-all-modes\n'
        m.validate_recipe(20,preservation)
        with self.assertRaises(ValueError): m.validate_recipe(20,preservation.replace(b'Ran 10 tests',b'Ran 0 tests'))

if __name__ == '__main__': unittest.main()
