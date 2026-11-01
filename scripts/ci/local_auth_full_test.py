import io
import tarfile
import tempfile
import unittest
from pathlib import Path
from unittest import mock

import local_auth_full as subject


def reports(*entries):
    stream = io.BytesIO()
    with tarfile.open(fileobj=stream, mode='w') as tar:
        for name, payload in entries:
            info = tarfile.TarInfo(name); info.size = len(payload)
            tar.addfile(info, io.BytesIO(payload))
    return stream.getvalue()


def suite(name, child='', skipped='0'):
    return (f'<testsuite name="{name}" tests="1" failures="0" errors="0" skipped="{skipped}">'
            f'<testcase classname="{name}" name="case">{child}</testcase></testsuite>').encode()


class FullAuthTests(unittest.TestCase):
    def test_full_report_inventory_rejects_missing_failed_and_skipped_required(self):
        name = 'voice.backend.auth.RequiredTest'
        path = './TEST-' + name + '.xml'
        result = subject.validate_reports(reports((path, suite(name))), {name})
        self.assertEqual(result['tests'], 1)
        self.assertEqual(result['skipped'], [])
        for raw in (reports(), reports((path, suite(name, '<skipped/>', '1'))),
                    reports((path, suite(name, '<failure/>'))), reports((path, suite('other'))),
                    reports((path, suite(name)), (path, suite(name))),
                    reports(('../escape', suite(name)))):
            with self.subTest(raw=raw[:40]), self.assertRaises(ValueError):
                subject.validate_reports(raw, {name})

    def test_nonrequired_skip_is_reported_and_never_a_pass(self):
        a, b = 'voice.RequiredTest', 'voice.OptionalTest'
        raw = reports(('./TEST-' + a + '.xml', suite(a)),
                      ('./TEST-' + b + '.xml', suite(b, '<skipped/>', '1')))
        result = subject.validate_reports(raw, {a})
        self.assertEqual(result['skipped'], [{'suite': b, 'case': 'case'}])

    def test_canonical_full_command_has_no_case_selector_and_is_offline(self):
        command = subject.full_command('172.18.0.2')
        self.assertIn('make --no-print-directory auth-test-ci', command[2])
        self.assertIn('MAVEN_ARGS="-o -ntp"', command[2])
        self.assertIn('ulimit -f 8192', command[2])
        self.assertNotIn('-Dtest=', command[2])

    def test_full_runner_executes_make_and_preserves_incomplete_status(self):
        name = 'voice.backend.auth.RequiredTest'
        raw = reports(('./TEST-' + name + '.xml', suite(name)))
        owner = mock.Mock()
        def execute(client, command, out, err, timeout, **kwargs):
            if command[0] == 'sha256sum':
                digest = 'd' * 64 if command[1].endswith('build-source.tar') else 'b' * 64
                out.write((digest + '  ' + command[1] + '\n').encode())
            elif command[0] == 'sh' and command[1:3] == ['-c', subject.COPY_SCRIPT]:
                out.write(raw if command[-2].endswith('reports.tar') else b'log')
            return 0
        owner.execute.side_effect = execute
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'root'; root.mkdir()
            cache = Path(directory) / 'cache'; cache.write_bytes(b'cache')
            source = {'head': 'a' * 40}
            with mock.patch.object(subject.local_chat_private, 'inspect_pair', return_value='172.18.0.2'), \
                 mock.patch.object(subject.snapshot, 'verify_materialized'), \
                 mock.patch.object(subject.snapshot, 'capture', return_value=source), \
                 mock.patch.object(subject, 'required_suites', return_value={name}), \
                 mock.patch.object(subject.selected, 'cache_digest', return_value='b' * 64), \
                 mock.patch.object(subject.selected, 'normalize_archive', side_effect=lambda src, dst, digest: (dst.write_bytes(b'build'), 'd' * 64)[1]), \
                 mock.patch.object(subject.local_go_execution, 'transfer_source', return_value='c' * 64), \
                 mock.patch.object(subject.local_go_execution, '_verify_workspace'), \
                 mock.patch.object(subject.selected, 'verify_build') as verify:
                result = subject.run_full(owner, 'daemon', 'client', root, source, root / 'copy', cache, Path(directory) / 'out')
        self.assertEqual(result['status'], 'incomplete')
        self.assertEqual(result['reports']['tests'], 1)
        self.assertEqual(verify.call_count, 2)
        self.assertTrue(any(call.args[1] == result['command'] for call in owner.execute.call_args_list))

    def test_bounded_copy_rejects_overflow_before_reading(self):
        owner = mock.Mock()
        def execute(name, command, out, err, timeout):
            out.write(b'12345')
            return 0
        owner.execute.side_effect = execute
        with tempfile.TemporaryDirectory() as directory, self.assertRaisesRegex(ValueError, 'exceeds'):
            subject.bounded_copy(owner, 'client', '/workspace/report', Path(directory) / 'report', 4)

    def test_make_log_and_report_archive_overflow_fail_before_validation(self):
        for overflow in ('auth-make.stdout', 'auth-reports.tar'):
            with self.subTest(overflow=overflow), tempfile.TemporaryDirectory() as directory:
                root = Path(directory) / 'root'; root.mkdir()
                cache = Path(directory) / 'cache'; cache.write_bytes(b'cache')
                source = {'head': 'a' * 40}; owner = mock.Mock()
                def execute(client, command, out, err, timeout, **kwargs):
                    if command[0] == 'sha256sum':
                        digest = 'd' * 64 if command[1].endswith('build-source.tar') else 'b' * 64
                        out.write((digest + '  ' + command[1] + '\n').encode())
                    elif command[0] == 'sh' and command[1:3] == ['-c', subject.COPY_SCRIPT]:
                        out.write(b'12345' if command[-2].endswith(overflow) else b'ok')
                    return 0
                owner.execute.side_effect = execute
                with mock.patch.object(subject.local_chat_private, 'inspect_pair', return_value='172.18.0.2'), \
                     mock.patch.object(subject.snapshot, 'verify_materialized'), \
                     mock.patch.object(subject.snapshot, 'capture', return_value=source), \
                     mock.patch.object(subject, 'required_suites', return_value={'voice.RequiredTest'}), \
                     mock.patch.object(subject.selected, 'cache_digest', return_value='b' * 64), \
                     mock.patch.object(subject.selected, 'normalize_archive', side_effect=lambda src, dst, digest: (dst.write_bytes(b'build'), 'd' * 64)[1]), \
                     mock.patch.object(subject.local_go_execution, 'transfer_source', return_value='c' * 64), \
                     mock.patch.object(subject.local_go_execution, '_verify_workspace'), \
                     mock.patch.object(subject.selected, 'verify_build'), \
                     mock.patch.object(subject, 'LOG_LIMIT', 4), mock.patch.object(subject, 'REPORT_LIMIT', 4), \
                     mock.patch.object(subject, 'validate_reports') as validate, \
                     self.assertRaisesRegex(ValueError, 'exceeds'):
                    subject.run_full(owner, 'daemon', 'client', root, source, root / 'copy', cache, Path(directory) / 'out')
                validate.assert_not_called()


if __name__ == '__main__':
    unittest.main()
