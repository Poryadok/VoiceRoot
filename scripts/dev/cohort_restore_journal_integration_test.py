"""Real populated PG restore through actual phase journals, with fake GitHub."""
import hashlib
import json
from pathlib import Path
import subprocess
import unittest
from unittest.mock import patch

import cohort_readback_test as readback_fixture
import cohort_restore_journal as restore
import cohort_upload_phase as upload
import story_media_backup_cohort as cohort
import story_media_backup_cohort_restore as cohort_restore
import story_media_backup_restore as media_restore
import story_media_backup_integration_test as fixtures
from story_media_backup_capture_integration_test import cleanup_fixture
from story_media_backup_restore_integration_test import POSTGRES, MIGRATE


class FencedRestoreIntegrationTest(unittest.TestCase):
    def test_real_dump_readback_and_isolated_restore_with_actual_journals(self):
        fixture = fixtures.PopulatedRestoreTest(); self.addCleanup(cleanup_fixture,fixture); fixture.setUp()
        for db in ('auth_db','voice_db'):
            fixture.sql(fixture.source,'source_admin','postgres','CREATE DATABASE '+db)
        fixture.sql(fixture.source,'source_admin','auth_db',
            "CREATE TABLE flyway_schema_history(installed_rank int PRIMARY KEY, version varchar(50), script varchar(1000) NOT NULL, checksum int, success bool NOT NULL);"
            "INSERT INTO flyway_schema_history VALUES(1,'32','V32__linked_oauth_transport.sql',123,true)")
        voice = Path('/repo/src/backend/migrations/voice_db')
        for path in sorted(voice.glob('*.up.sql')):
            if int(path.name[:6]) <= 15: fixture.sql(fixture.source,'source_admin','voice_db',path.read_text())
        fixture.sql(fixture.source,'source_admin','voice_db',
            'CREATE TABLE schema_migrations(version bigint NOT NULL, dirty boolean NOT NULL); INSERT INTO schema_migrations VALUES(15,false)')
        for db in ('auth_db','voice_db'):
            fixture.sql(fixture.source,'source_admin',db,'CREATE TABLE fixture_marker(id int PRIMARY KEY); INSERT INTO fixture_marker VALUES(1)')
        auth = Path('/repo/src/backend/auth/src/main/resources/db/migration')
        plans = {db:{'engine':engine,'from':version,'to':version,
                     'files':{p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in folder.glob('*.sql')}}
                 for db,engine,version,folder in [('auth_db','flyway',32,auth),('voice_db','golang-migrate',15,voice)]}
        catalogs = {db:json.loads(fixture.sql(fixture.source,'source_admin',db,
                    cohort_restore.AUTH_CATALOG if db == 'auth_db' else media_restore.CATALOG)) for db in ('auth_db','voice_db')}
        roles = json.loads(fixture.sql(fixture.source,'source_admin','postgres',media_restore.role_catalog_sql()))
        metadata = {'catalog':catalogs,'roles':roles,'migrations':plans}
        def digest(value):
            return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',',':'),ensure_ascii=True).encode()).hexdigest()
        setup = readback_fixture.fixtures.FinalizeCohortTest.setUp
        def setup_with_metadata(template):
            setup(template)
            template.binding['restore_inputs'] = {'catalog_sha256':digest(catalogs), 'roles_catalog_sha256':digest(roles),
                'migration_catalog_sha256':digest(plans), 'postgres_image':POSTGRES, 'migrate_image':MIGRATE,
                'migrate_binary_sha256':'e'*64, 'postgres_major':16}
        export = cohort.export
        def populated_export(root,key,binding,databases,unused):
            def dump(member,out):
                command = (['pg_dumpall','-U','source_admin','--roles-only','--no-role-passwords'] if member == 'roles'
                           else ['pg_dump','-U','source_admin','-Fc','-d',member])
                result = subprocess.run(['docker','exec',fixture.source,*command],stdout=out,stderr=subprocess.DEVNULL,timeout=90)
                if result.returncode: raise RuntimeError('fixture dump failed')
            return export(root,key,binding,databases,dump)
        state = readback_fixture.ReadbackTest('test_real_encrypted_readback_and_no_replay')
        self.addCleanup(readback_fixture.cleanup,state)
        with patch.object(readback_fixture.fixtures.FinalizeCohortTest,'setUp',setup_with_metadata), patch.object(cohort,'export',side_effect=populated_export):
            state.setUp()
        _, manifest = state.extract(); original = media_restore._psql; observed = []
        def observe(cid,admin,db,sql,timeout=120):
            result = original(cid,admin,db,sql,timeout=timeout)
            if sql in (cohort_restore.AUTH_CATALOG,media_restore.CATALOG):
                self.assertEqual(original(cid,admin,db,'SELECT count(*) FROM fixture_marker').strip(),b'1'); observed.append(db)
            return result
        with patch.object(upload,'ROOT',state.root), patch.object(media_restore,'_psql',side_effect=observe), \
             patch.object(media_restore,'_tool_bytes',side_effect=AssertionError('no migration')):
            receipt = restore.run(state.phase,state.key,state.destination,manifest,metadata,Path('/repo'),POSTGRES,MIGRATE,lambda:None)
        self.assertEqual(receipt['cleanup'],'verified'); self.assertEqual(sorted(observed),['auth_db','auth_db','voice_db','voice_db'])
        self.assertEqual(state.base.ledger.row['cohort_restore']['status'],'done')
        self.assertEqual(state.base.ledger.row['cohort_restore_resources']['postgres']['status'],'removed')


if __name__ == '__main__': unittest.main()
