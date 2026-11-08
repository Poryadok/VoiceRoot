import unittest

import full_cohort_go_migration_plan as plan


class PlanTest(unittest.TestCase):
    def setUp(self):
        self.source = {'files': {'scripts/dev/migrations.py': plan.MIGRATIONS_SOURCE_SHA256}}
        self.files = {'000001_initial.up.sql': 'a' * 64, '000001_initial.down.sql': 'b' * 64,
                      '000003_second.up.sql': 'c' * 64, '000003_second.down.sql': 'd' * 64}
        self.source['files'].update({'src/backend/migrations/chat_db/' + k:v for k,v in self.files.items()})
        self.baseline = {'engine': 'golang-migrate', 'from': 1, 'to': 1, 'files': self.files}

    def test_source_qualified_up_plan_retains_full_inventory(self):
        result = plan.for_restore('chat_db', self.baseline, self.source)
        self.assertEqual((result['from'], result['to']), (1, 3))
        self.assertEqual(result['files'], self.files)

    def test_different_source_or_absent_start_and_down_are_rejected(self):
        missing_start = {'000003_second.up.sql': 'a' * 64}
        missing_down = {k:v for k,v in self.files.items() if not k.endswith('initial.down.sql')}
        def source_for(files):
            return {'files': {'scripts/dev/migrations.py': plan.MIGRATIONS_SOURCE_SHA256} |
                    {'src/backend/migrations/chat_db/' + k:v for k,v in files.items()}}
        for bad_source, files in [(None, self.files), (source_for(missing_start), missing_start),
                                  (source_for(missing_down), missing_down)]:
            bad = self.baseline | {'files': files}
            with self.subTest(source=bad_source, files=files), self.assertRaises(plan.PlanError):
                plan.for_restore('chat_db', bad, bad_source)

    def test_special_databases_never_infer_generic_target(self):
        for db in ('space_db', 'realtime_db', 'auth_db', 'gateway_db', 'game_integration_db', 'postgres'):
            self.assertEqual(plan.for_restore(db, self.baseline, self.source), self.baseline)

    def test_noop_retains_same_version_and_no_duplicate_versions(self):
        no_op = self.baseline | {'from': 3, 'to': 3}
        self.assertEqual(plan.for_restore('chat_db', no_op, self.source), no_op)
        bad = self.baseline | {'files': self.files | {'000003_other.up.sql': 'e' * 64}}
        with self.assertRaises(plan.PlanError): plan.for_restore('chat_db', bad, self.source)
