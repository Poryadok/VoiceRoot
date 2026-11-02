"""Source-bound tests for the fetched PR proofs-shard component."""
import subprocess
import tempfile
from types import SimpleNamespace
import unittest
from pathlib import Path
from unittest.mock import patch

import local_ci_proofs_component as proofs


class ProofComponentTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='voice-ci-proofs-component-')
        self.addCleanup(self.temp.cleanup)
        self.repo = Path(self.temp.name)
        self.git('init', '-q')
        self.git('config', 'user.email', 'fixture@example.invalid')
        self.git('config', 'user.name', 'CI fixture')
        self.write('README.md', 'base\n')
        self.event_base = self.commit('base')
        self.base = self.event_base
        self.write('README.md', 'head\n')
        self.head = self.commit('head')
        self.candidate = self.merge(self.base, self.head)

    def git(self, *args):
        return subprocess.check_output(['git', '-C', str(self.repo), *args],
                                       stderr=subprocess.PIPE, text=True).strip()

    def write(self, name, content):
        path = self.repo / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content, encoding='utf-8')

    def commit(self, message):
        self.git('add', '.')
        self.git('commit', '-qm', message)
        return self.git('rev-parse', 'HEAD')

    def merge(self, base, head, tree=None):
        tree = tree or self.git('merge-tree', '--write-tree', base, head).splitlines()[0]
        return self.git('commit-tree', tree, '-p', base, '-p', head, '-m', 'PR merge')

    def test_candidate_requires_exact_parents_and_clean_merge_tree(self):
        result = proofs.validate_candidate(self.repo, self.event_base, self.base,
                                           self.head, self.candidate)
        self.assertEqual(result['parents'], [self.base, self.head])
        self.assertEqual(result['tree'], self.git('rev-parse', self.candidate + '^{tree}'))

    def test_candidate_rejects_wrong_head_parent_and_forged_tree(self):
        wrong_parent = self.merge(self.base, self.event_base)
        wrong_tree = self.merge(self.base, self.head,
                                self.git('rev-parse', self.base + '^{tree}'))
        with self.assertRaisesRegex(ValueError, 'parent'):
            proofs.validate_candidate(self.repo, self.event_base, self.base,
                                      self.head, wrong_parent)
        with self.assertRaisesRegex(ValueError, 'tree'):
            proofs.validate_candidate(self.repo, self.event_base, self.base,
                                      self.head, wrong_tree)

    def test_stale_event_base_must_be_ancestor_of_actual_merge_base(self):
        self.write('docs/advance.md', 'advanced base\n')
        self.base = self.commit('advance target base')
        self.git('checkout', '-q', self.event_base)
        self.write('docs/source.md', 'source\n')
        self.head = self.commit('source')
        self.git('checkout', '-q', self.base)
        self.candidate = self.merge(self.base, self.head)
        result = proofs.validate_candidate(self.repo, self.event_base, self.base,
                                           self.head, self.candidate)
        self.assertEqual(result['base'], self.base)
        with self.assertRaisesRegex(ValueError, 'ancestor'):
            proofs.validate_candidate(self.repo, self.head, self.base,
                                      self.head, self.candidate)

    def test_execution_requires_every_recipe_and_rejects_unittest_skips(self):
        commands = [f'command-{number}' for number in range(15)]
        dry_run = ('\n'.join(commands) + '\n').encode()
        output = ('\n'.join(commands) + '\nRan 4 tests in 0.1s\n\nOK\n').encode()
        result = proofs.validate_execution(dry_run, output, b'', 0, expected_recipes=15)
        self.assertEqual(result['recipes'], 15)
        for text in (
                '\n'.join(commands[:-1]) + '\n',
                '\n'.join(commands[:-1] + ['other-command']) + '\n'):
            with self.subTest(text=text), self.assertRaisesRegex(ValueError, 'recipe'):
                proofs.validate_execution(dry_run, text.encode(), b'Ran 4 tests in 0.1s\n\nOK\n',
                                          0, expected_recipes=15)
        skipped = ('\n'.join(commands) + '\nRan 4 tests in 0.1s\n\n'
                   'OK (skipped=1)\n').encode()
        with self.assertRaisesRegex(ValueError, 'skip'):
            proofs.validate_execution(dry_run, skipped, b'', 0, expected_recipes=15)

    def test_execution_rejects_empty_test_run_failure_and_bounded_output(self):
        commands = ('\n'.join(f'command-{number}' for number in range(15)) + '\n').encode()
        for output, error, code in ((commands, b'', 0),
                                    (commands + b'Ran 0 tests in 0.1s\n\nOK\n', b'', 0),
                                    (commands + b'Ran 1 test in 0.1s\n\nFAILED\n', b'', 1),
                                    (commands + b'Ran 1 test in 0.1s\n\nOK\n', b'SKIP: unavailable\n', 0)):
            with self.subTest(output=output[-32:], error=error, code=code), self.assertRaises(ValueError):
                proofs.validate_execution(commands, output, error, code, expected_recipes=15)

    def test_child_environment_removes_make_and_shell_failure_overrides(self):
        source = {'PATH': 'private-shims', 'MAKEFLAGS': '-i', 'GNUMAKEFLAGS': '-k',
                  'MFLAGS': '-s', 'MAKEFILES': 'untrusted.mk', 'MAKEOVERRIDES': 'x=y',
                  'BASH_ENV': 'startup.sh', 'ENV': 'startup.sh', 'SHELLOPTS': 'xtrace',
                  'BASHOPTS': 'expand_aliases', 'GIT_DIR': 'foreign.git',
                  'PYTHONPATH': 'foreign-python', 'OTHER_VALUE': 'keep'}
        result = proofs.child_environment(source, self.repo / 'bash.exe')
        self.assertEqual(result['PATH'], 'private-shims')
        self.assertEqual(result['OTHER_VALUE'], 'keep')
        self.assertEqual(result['VOICE_TEST_BASH'], str(self.repo / 'bash.exe'))
        self.assertFalse({'MAKEFLAGS', 'GNUMAKEFLAGS', 'MFLAGS', 'MAKEFILES',
                          'MAKEOVERRIDES', 'BASH_ENV', 'ENV', 'SHELLOPTS', 'BASHOPTS',
                          'GIT_DIR', 'PYTHONPATH'} & set(result))

    def test_tool_version_accepts_standard_multiline_gnu_make_output(self):
        output = (b'GNU Make 4.4.1\nBuilt for Windows32\nCopyright (C) Free Software Foundation\n')
        with patch.object(proofs.execution, '_execute', return_value=SimpleNamespace(
                returncode=0, stdout=output, stderr=b'')):
            self.assertEqual(proofs._tool_version(self.repo / 'make.exe', 'GNU Make', {}),
                             output.decode().strip())
        for malformed in (b'Fake Make 4.4.1\n', b'GNU Make unknown\n'):
            with self.subTest(output=malformed), patch.object(
                    proofs.execution, '_execute', return_value=SimpleNamespace(
                        returncode=0, stdout=malformed, stderr=b'')):
                with self.assertRaisesRegex(ValueError, 'not recognized'):
                    proofs._tool_version(self.repo / 'make.exe', 'GNU Make', {})

    def test_private_git_config_pins_newline_behavior_and_rejects_deep_windows_paths(self):
        proofs.configure_private_git('git', self.repo)
        self.assertEqual(self.git('config', '--get', 'core.autocrlf'), 'false')
        self.assertEqual(self.git('config', '--get', 'core.longpaths'), 'false')
        with patch.object(proofs, '_windows', return_value=True):
            proofs.validate_windows_checkout_paths(self.repo, ['short/file.txt'])
            long_name = 'nested/' + ('x' * proofs.WINDOWS_PATH_LIMIT)
            with self.assertRaisesRegex(ValueError, 'shorter output path'):
                proofs.validate_windows_checkout_paths(self.repo, [long_name])
            astral_name = 'nested/' + ('😀' * 130)
            with self.assertRaisesRegex(ValueError, 'shorter output path'):
                proofs.validate_windows_checkout_paths(self.repo, [astral_name])
            long_root = self.repo / ('x' * proofs.WINDOWS_PATH_LIMIT)
            with self.assertRaisesRegex(ValueError, 'shorter output path'):
                proofs.validate_windows_checkout_paths(long_root, [])

    def test_recipe_expansion_is_exact_and_unknown_make_syntax_refuses(self):
        bash = self.repo / 'bash.exe'
        rows = [{'recipe': '$(BASH) "$(ROOT)/scripts/ci/a.sh"'},
                {'recipe': 'python3 "$(ROOT)/scripts/ci/b.py"'}]
        expanded = proofs.expected_recipe_commands(rows, self.repo, bash)
        if proofs.os.name == 'nt':
            self.assertEqual(expanded[0], '"' + bash.resolve().as_posix() + '" "' +
                             self.repo.resolve().as_posix() + '/scripts/ci/a.sh"')
        else:
            self.assertEqual(expanded[0], str(bash.resolve()) + ' "' +
                             self.repo.resolve().as_posix() + '/scripts/ci/a.sh"')
        with self.assertRaisesRegex(ValueError, 'unsupported expansion'):
            proofs.expected_recipe_commands([{'recipe': 'make $(shell true)'}], self.repo, bash)

    def test_runner_refuses_platform_without_descendant_supervision(self):
        from unittest.mock import patch
        with patch.object(proofs, '_windows', return_value=False):
            with self.assertRaisesRegex(ValueError, 'Windows descendant supervision'):
                proofs.run(self.repo, self.event_base, self.head, 'codex/test',
                           'refs/pull/1089/merge',
                           Path(self.temp.name).parent / (Path(self.temp.name).name + '-outside'))


if __name__ == '__main__':
    unittest.main()
