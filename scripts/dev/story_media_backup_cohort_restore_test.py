import hashlib
from pathlib import Path
import tempfile
import unittest
from unittest import mock
import sys
import types
import io
import tarfile
import json

import story_media_backup_cohort_restore as restore


class CohortPlanTest(unittest.TestCase):
    def test_staging_story_proof_runs_between_migration_and_owned_cleanup(self):
        events=[]; session=object()
        plan={'engine':'golang-migrate','from':4,'to':8,'files':{'fixture':'a'*64}}
        metadata={'roles':{},'catalog':{'story_db':{}},'migrations':{'story_db':plan}}
        binding={'full_catalog':{'properties':{'story_db':{}}},'restore_inputs':{'migrate_binary_sha256':'a'*64},
                 'operation_id':'operation','snapshot_id':'snapshot','source_sha':'source'}
        adapter=types.SimpleNamespace(validate_backup_binding=mock.Mock(),load_metadata=mock.Mock(return_value={'source':metadata}),
                                      story_restore_inputs=mock.Mock(return_value=('profile','caller')))
        fixture=types.SimpleNamespace(preserved_snapshot=mock.Mock(side_effect=lambda sql:events.append('baseline') or 'baseline'),
            prove_preserved=mock.Mock(side_effect=lambda *args:events.append('proof') or {'qualified':True}))
        journal=types.SimpleNamespace(**{name:mock.Mock() for name in ('begin','created','begin_remove','removed','uncertain')})
        def psql(cid,admin,db,query):
            return json.dumps({'history':[{'version':8,'dirty':False}]} if db=='story_db' and query==restore.media_restore.CATALOG and 'migrate' in events else {})
        def docker(*args,**kwargs):return b'linux' if args[:1]==('info',) else b''
        with mock.patch.dict(sys.modules,{'staging_cohort_preservation':adapter,'story_fixture':fixture}), \
             mock.patch.object(restore,'_source_shape'),mock.patch.object(restore.cohort,'verify_staging'), \
             mock.patch.object(restore,'migration_inventory',return_value={}), \
             mock.patch.object(restore,'_qualified_staging_roles',return_value={'story_db':{'role':'owner'}}), \
             mock.patch.object(restore.media_restore,'_docker',side_effect=docker), \
             mock.patch.object(restore.media_restore,'_check_image',return_value='image'), \
             mock.patch.object(restore.media_restore,'_create',return_value='clone'), \
             mock.patch.object(restore.media_restore,'_tool_bytes',return_value=b'tool'), \
             mock.patch.object(restore.media_restore,'_ready_major',return_value=16), \
             mock.patch.object(restore.media_restore,'_psql',side_effect=psql), \
             mock.patch.object(restore,'_decrypt_command'),mock.patch.object(restore.unmanaged,'verify_restored_properties'), \
             mock.patch.object(restore,'_install_go'), \
             mock.patch.object(restore,'_story_reference',side_effect=lambda *args:events.append('reference') or 'reference'), \
             mock.patch.object(restore,'_run_go_as_role',side_effect=lambda *args,**kw:events.append('migrate')), \
             mock.patch.object(restore.media_restore,'_cleanup',side_effect=lambda *args:events.append('cleanup')) as cleanup:
            receipt=restore._qualify(None,None,binding,['story_db'],{},metadata,None,'pg','migrate',journal,
                                     _staging=(session,{},{}))
            self.assertEqual(events,['baseline','reference','migrate','proof','cleanup'])
            self.assertEqual(receipt['story_target8'],{'qualified':True})
            self.assertEqual(receipt['cleanup'],'verified')
            self.assertEqual(fixture.prove_preserved.call_args.args[3:5],('baseline','reference'))
            events.clear();fixture.prove_preserved.side_effect=ValueError('probe failed')
            with self.assertRaisesRegex(restore.CohortRestoreError,'cohort restore incomplete'):
                restore._qualify(None,None,binding,['story_db'],{},metadata,None,'pg','migrate',journal,
                                 _staging=(session,{},{}))
            self.assertEqual(events,['baseline','reference','migrate','cleanup'])
            events.clear();fixture.prove_preserved.side_effect=lambda *args:events.append('proof') or {'qualified':True}
            cleanup.side_effect=RuntimeError('cleanup uncertain')
            with self.assertRaisesRegex(RuntimeError,'cleanup uncertain'):
                restore._qualify(None,None,binding,['story_db'],{},metadata,None,'pg','migrate',journal,
                                 _staging=(session,{},{}))

    def test_story_reference_rejects_authority_drift_before_migration(self):
        authority={'database':{'owner':'story_owner'},'schema':{'owner':'story_owner'},'defaults':[]}
        props={'locale_provider':'c','encoding':'UTF8','collate':'C','ctype':'C','icu_locale':None}
        fixture=types.SimpleNamespace(PRESERVED_AUTHORITY_SQL='authority',preserved_snapshot=mock.Mock(return_value='snapshot'))
        for field in ('database','schema','defaults'):
            actual=dict(authority);actual[field]={'changed':True}
            with self.subTest(field=field), mock.patch.dict(sys.modules,{'story_fixture':fixture}), \
                 mock.patch.object(restore.media_restore,'_psql',return_value=json.dumps(actual)), \
                 mock.patch.object(restore,'_run_go_as_role') as migrate:
                with self.assertRaisesRegex(restore.CohortRestoreError,'story reference authority differs'):
                    restore._story_reference('clone','admin','story_owner',props,{'authority':authority})
                migrate.assert_not_called()
        with mock.patch.dict(sys.modules,{'story_fixture':fixture}), \
             mock.patch.object(restore.media_restore,'_psql',return_value=json.dumps(authority)), \
             mock.patch.object(restore,'_run_go_as_role') as migrate:
            self.assertEqual(restore._story_reference('clone','admin','story_owner',props,{'authority':authority}),'snapshot')
            self.assertTrue(migrate.call_args.kwargs['story_reference'])
            self.assertTrue(migrate.call_args.args[2].startswith('voice_story_reference_'))

    def test_story_reference_installs_all_pinned_migrations_separately(self):
        files={f'00000{i}_step.up.sql':f'SELECT {i};'.encode() for i in range(1,9)}
        plans={'story_db':{'from':4,'to':8}}
        with mock.patch.object(restore.media_restore,'_docker') as docker:
            restore._install_go('clone',{'story_db':files},plans,['story_db'],story_reference=True)
        payload=docker.call_args.kwargs['input']
        with tarfile.open(fileobj=io.BytesIO(payload)) as archive:
            names=set(archive.getnames())
            self.assertEqual({f'story_db/{name}' for name in files if int(name[:6])>=4},
                             {name for name in names if name.startswith('story_db/')})
            self.assertEqual({f'story_reference/{name}' for name in files},
                             {name for name in names if name.startswith('story_reference/')})
            for name,raw in files.items():
                self.assertEqual(archive.extractfile('story_reference/'+name).read(),raw)

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
