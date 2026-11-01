import importlib.util
import json
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('dispatch', Path(__file__).with_name('local_ci_staging_dispatch.py'))
m = importlib.util.module_from_spec(spec); spec.loader.exec_module(m)
def rows():
    return [{'recipe':'$(BASH) "$(ROOT)/'+(m.PRESERVATION if i == 0 else 'scripts/ci/test%d.sh'%i)+'"'} for i in range(34)]

class DispatchTests(unittest.TestCase):
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
