import hashlib
from pathlib import Path
import tempfile
import unittest
from unittest import mock
import sys
import types

import story_media_backup_cohort_restore as restore


class CohortPlanTest(unittest.TestCase):
    def test_staging_changed_space_requires_native_proof_before_docker(self):
        session=object()
        metadata={'migrations':{'space_db':{'engine':'golang-migrate','from':24,'to':51,'files':{'x':'a'*64}}}}
        adapter=types.SimpleNamespace(validate_backup_binding=mock.Mock(),load_metadata=mock.Mock(return_value={'source':metadata}))
        journal=types.SimpleNamespace(**{name:mock.Mock() for name in ('begin','created','begin_remove','removed','uncertain')})
        with mock.patch.dict(sys.modules,{'staging_cohort_preservation':adapter}), \
             mock.patch.object(restore,'_source_shape'), mock.patch.object(restore.cohort,'verify_staging'), \
             mock.patch.object(restore,'migration_inventory',return_value={}), \
             mock.patch.object(restore,'classify_plan',wraps=restore.classify_plan) as classify, \
             mock.patch.object(restore.media_restore,'_docker') as docker:
            with self.assertRaisesRegex(restore.CohortRestoreError,'cohort restore incomplete'):
                restore._qualify(None,None,{'full_catalog':{}},['space_db'],{},metadata,None,None,None,journal,
                                 _staging=(session,{},{}))
        docker.assert_not_called()
        classify.assert_called_once_with('space_db',metadata['migrations']['space_db'],True)

    def test_staging_restore_uses_fixed_transport_and_clears_context_on_failure(self):
        session=object()
        context={key: object() for key in ('root','key','source_root','postgres_image','migrate_image','journal','source_metadata')}
        adapter=types.SimpleNamespace(validate_backup_binding=mock.Mock(),restore_context=mock.Mock(return_value=context),
                                      restore_command=mock.Mock(return_value=b'ok'))
        def failing_qualify(*args,**kwargs):
            self.assertEqual(restore.media_restore._docker('inspect','owned',input=b'data'),b'ok')
            raise restore.CohortRestoreError('fixture failure')
        with mock.patch.dict(sys.modules,{'staging_cohort_preservation':adapter}), mock.patch.object(restore,'_qualify',side_effect=failing_qualify):
            with self.assertRaisesRegex(restore.CohortRestoreError,'fixture failure'):
                restore.qualify_staging(session,{},[],{}, {}, {})
        adapter.restore_command.assert_called_once_with(session,['docker','inspect','owned'],body=b'data',timeout=120)
        self.assertIsNone(restore.media_restore._STAGING_SESSION.get())

    def test_restore_only_never_selects_a_migration_runner(self):
        for db, engine in [('voice_db', 'golang-migrate'), ('auth_db', 'flyway')]:
            plan = {'engine': engine, 'from': 3, 'to': 3, 'files': {'fixture': 'a' * 64}}
            self.assertEqual(restore.classify_plan(db, plan), 'restore-only')

    def test_unchanged_and_migrating_plans_are_distinct(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            voice = root / 'src/backend/migrations/voice_db'; voice.mkdir(parents=True)
            auth = root / 'src/backend/auth/src/main/resources/db/migration'; auth.mkdir(parents=True)
            files = {'000001_base.up.sql': b'SELECT 1;', '000001_base.down.sql': b'SELECT 1;',
                     '000002_next.up.sql': b'SELECT 2;'}
            for name, raw in files.items(): (voice / name).write_bytes(raw)
            (auth / 'V1__base.sql').write_bytes(b'SELECT 1;')
            def hashes(values): return {name: hashlib.sha256(raw).hexdigest() for name, raw in values.items()}
            plans = {'auth_db': {'engine': 'flyway', 'from': 1, 'to': 1,
                                  'files': hashes({'V1__base.sql': b'SELECT 1;'})},
                     'voice_db': {'engine': 'golang-migrate', 'from': 1, 'to': 2, 'files': hashes(files)}}
            actual = restore.migration_inventory(root, ['auth_db', 'voice_db'], plans)
            self.assertEqual(actual['auth_db']['V1__base.sql'], b'SELECT 1;')
            self.assertEqual(actual['voice_db']['000002_next.up.sql'], b'SELECT 2;')
            (voice / '000003_extra.up.sql').write_bytes(b'new')
            with self.assertRaises(restore.CohortRestoreError):
                restore.migration_inventory(root, ['auth_db', 'voice_db'], plans)

    def test_changed_flyway_is_explicitly_rejected(self):
        with self.assertRaisesRegex(restore.CohortRestoreError, 'changed Flyway'):
            restore.classify_plan('auth_db', {'engine': 'flyway', 'from': 1, 'to': 2, 'files': {'V1__base.sql': 'a' * 64}})


if __name__ == '__main__': unittest.main()
