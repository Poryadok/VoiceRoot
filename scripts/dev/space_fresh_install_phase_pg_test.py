"""Fixed SQL planning tests; real PostgreSQL behavior is checked separately."""
import unittest
import sys
from unittest import mock

import space_fresh_install_phase_pg as subject


class DropScriptTest(unittest.TestCase):
    def test_drop_is_fixed_to_exact_positive_oid_and_one_database(self):
        sql = subject.drop_script(123)
        self.assertIn("datname='space_db'", sql)
        self.assertIn('oid=123::oid', sql)
        self.assertIn('DROP DATABASE space_db', sql)
        self.assertIn('SELECT 1/0', sql)
        self.assertIn('\\gexec', sql)
        self.assertIn("pg_try_advisory_lock(pg_catalog.hashtextextended('voice-space-native-installer',0))", sql)
        self.assertNotIn('FORCE', sql)

    def test_non_integer_and_out_of_range_are_rejected(self):
        for bad in (True, 0, -1, 2**32, '123', None):
            with self.subTest(value=bad), self.assertRaises(ValueError):
                subject.drop_script(bad)

    def test_fixed_transport_uses_stdin_and_guards_actual_child(self):
        original = subject.subprocess.Popen
        calls = []
        guards = []
        def launch(argv, **kwargs):
            calls.append(argv)
            program = "import sys; s=sys.stdin.read(); sys.stdout.write('DROP DATABASE\\n' if 'DROP DATABASE space_db' in s else '42\\n')"
            return original([sys.executable, '-c', program], **kwargs)
        with mock.patch.object(subject.metadata, 'kube_engine_snapshot', return_value=lambda: {'fixture': True}), \
             mock.patch.object(subject.subprocess, 'Popen', side_effect=launch):
            pg = subject.KubeSpacePostgres(lambda: guards.append(True))
            self.assertEqual(pg.space_oid(), 42)
            pg.drop_exact(42)
        self.assertEqual(calls, [subject.COMMAND, subject.COMMAND])
        self.assertEqual(len(guards), 4)
        self.assertNotIn('42', ' '.join(calls[1]))
        self.assertNotIn('DROP DATABASE', ' '.join(calls[1]))

    def test_guard_loss_after_child_is_not_success(self):
        original = subject.subprocess.Popen
        guards = []
        def guard():
            guards.append(True)
            if len(guards) == 2: raise RuntimeError('authority lost')
        def launch(argv, **kwargs):
            return original([sys.executable, '-c', "import sys; sys.stdin.read(); print('DROP DATABASE')"], **kwargs)
        with mock.patch.object(subject.metadata, 'kube_engine_snapshot', return_value=lambda: {'fixture': True}), \
             mock.patch.object(subject.subprocess, 'Popen', side_effect=launch):
            pg = subject.KubeSpacePostgres(guard)
            with self.assertRaises(subject.metadata.GuardLost): pg.drop_exact(42)
        self.assertEqual(len(guards), 2)

    def test_oversized_child_output_is_rejected(self):
        original = subject.subprocess.Popen
        def launch(argv, **kwargs):
            return original([sys.executable, '-c', "import sys; sys.stdin.read(); print('x'*9000)"], **kwargs)
        with mock.patch.object(subject.metadata, 'kube_engine_snapshot', return_value=lambda: {'fixture': True}), \
             mock.patch.object(subject.subprocess, 'Popen', side_effect=launch):
            pg = subject.KubeSpacePostgres(lambda: None)
            with self.assertRaises(subject.SpacePostgresError): pg.drop_exact(42)


if __name__ == '__main__': unittest.main()
