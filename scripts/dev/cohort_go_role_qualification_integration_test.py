"""Isolated PostgreSQL direct-login migration privilege regression."""
import hashlib
import io
import subprocess
import tarfile
import time
import unittest
import json
from pathlib import Path
from unittest.mock import patch

import story_media_backup_cohort_restore as restore
import story_media_backup_restore as media
import story_media_backup_integration_test as fixtures
from story_media_backup_capture_integration_test import cleanup_fixture
from story_media_backup_restore_integration_test import Journal, MIGRATE


class GoRoleIntegrationTest(unittest.TestCase):
    def test_story_reference_is_independent_and_matches_real_migration(self):
        import story_fixture
        fixture = fixtures.PopulatedRestoreTest()
        self.addCleanup(cleanup_fixture, fixture)
        fixture.setUp()
        pg=fixture.target; admin='target_admin'; role='story_owner'
        deadline=time.monotonic()+60
        while True:
            ready=subprocess.run(['docker','exec',pg,'sh','-ec',
                'test "$(cat /proc/1/comm)" = postgres && exec psql -X -qAt -v ON_ERROR_STOP=1 -U target_admin -d postgres -c "SELECT 1"'],
                stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,timeout=10)
            if ready.returncode==0 and ready.stdout.strip()==b'1':break
            if time.monotonic()>=deadline:self.fail('target readiness failed')
        fixture.sql(pg,admin,'postgres','CREATE ROLE story_owner LOGIN')
        fixture.sql(pg,admin,'postgres','CREATE DATABASE story_db OWNER story_owner TEMPLATE template0')
        fixture.sql(pg,admin,'story_db','ALTER SCHEMA public OWNER TO story_owner')
        journal=Journal(); resources=[]
        try:
            tool_container=media._create(journal,resources,'migrate',MIGRATE,['--entrypoint','/bin/true'])
            packed=media._docker('cp',tool_container+':/usr/local/bin/migrate','-')
            with tarfile.open(fileobj=io.BytesIO(packed),mode='r:') as archive:
                raw=archive.extractfile(archive.getmembers()[0]).read()
            tool=media._tool_bytes(tool_container,hashlib.sha256(raw).hexdigest())
            media._docker('exec','-i',pg,'sh','-ec',
                'cat > /tmp/voice-migrate && chmod 0555 /tmp/voice-migrate',input=tool)
            files={p.name:p.read_bytes() for p in Path('/repo/src/backend/migrations/story_db').glob('*.sql')}
            restore._install_go(pg,{'story_db':files},{'story_db':{'from':1,'to':4}},['story_db'])
            restore._run_go_as_role(pg,admin,'story_db',role)
            sql=lambda query:media._psql(pg,admin,'story_db',query)
            before=story_fixture.preserved_snapshot(sql)
            restore._install_go(pg,{'story_db':files},{'story_db':{'from':4,'to':8}},['story_db'],story_reference=True)
            props=json.loads(fixture.sql(pg,admin,'story_db',"SELECT json_build_object('encoding',pg_encoding_to_char(encoding),'collate',datcollate,'ctype',datctype,'locale_provider',datlocprovider,'icu_locale',daticulocale) FROM pg_database WHERE datname=current_database()"))
            reference=restore._story_reference(pg,admin,role,props,before)
            restore._run_go_as_role(pg,admin,'story_db',role)
            migrated=story_fixture.preserved_snapshot(sql)
            story_fixture.validate_preserved_migration(before['rows'],migrated['rows'])
            story_fixture.retained_authority(before['authority'],migrated['authority'],reference['authority'])
            self.assertEqual(reference['structure'],migrated['structure'])
            self.assertEqual(set(reference['rows']),set(migrated['rows']))
            self.assertNotEqual(reference['rows']['story_reference_inventory_gate'],migrated['rows']['story_reference_inventory_gate'])
            fixture.sql(pg,admin,'story_db','REVOKE USAGE ON SCHEMA public FROM PUBLIC')
            changed=story_fixture.preserved_snapshot(sql)
            self.assertNotEqual(changed['authority']['schema'],before['authority']['schema'])
            with self.assertRaisesRegex(restore.CohortRestoreError,'story reference authority differs'):
                restore._story_reference(pg,admin,role,props,changed)
        finally:
            media._cleanup(journal,resources)

    def test_direct_login_migration_succeeds_with_grant_and_denies_without(self):
        fixture = fixtures.PopulatedRestoreTest()
        self.addCleanup(cleanup_fixture, fixture)
        fixture.setUp()
        deadline = time.monotonic() + 60
        while True:
            ready = subprocess.run(['docker','exec',fixture.target,'sh','-ec',
                'test "$(cat /proc/1/comm)" = postgres && exec psql -X -qAt -v ON_ERROR_STOP=1 -U target_admin -d postgres -c "SELECT 1"'],
                stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,timeout=10)
            if ready.returncode == 0 and ready.stdout.strip() == b'1': break
            if time.monotonic() >= deadline: self.fail('target readiness failed')
        fixture.sql(fixture.target,'target_admin','postgres',
                    'CREATE ROLE clone_writer LOGIN; CREATE ROLE clone_denied LOGIN')
        for db in ('allowed_db','denied_db'):
            fixture.sql(fixture.target,'target_admin','postgres','CREATE DATABASE '+db+' TEMPLATE template0')
            fixture.sql(fixture.target,'target_admin',db,'REVOKE CREATE ON SCHEMA public FROM PUBLIC')
        fixture.sql(fixture.target,'target_admin','allowed_db','GRANT CREATE ON SCHEMA public TO clone_writer')
        tool_journal=Journal(); resources=[]
        try:
            tool_container=media._create(tool_journal,resources,'migrate',MIGRATE,['--entrypoint','/bin/true'])
            packed=media._docker('cp',tool_container+':/usr/local/bin/migrate','-')
            with tarfile.open(fileobj=io.BytesIO(packed),mode='r:') as archive:
                member=archive.getmembers()[0]; raw=archive.extractfile(member).read()
            tool=media._tool_bytes(tool_container,hashlib.sha256(raw).hexdigest())
            media._docker('exec','-i',fixture.target,'sh','-ec','cat > /tmp/voice-migrate && chmod 0555 /tmp/voice-migrate',input=tool)
            for db in ('allowed_db','denied_db'):
                media._docker('exec','-i',fixture.target,'sh','-ec',
                    'mkdir -p /tmp/voice-cohort-migrations/'+db+' && cat > /tmp/voice-cohort-migrations/'+db+'/000001_create.up.sql',
                    input=b'CREATE TABLE public.qualified_role_marker(id integer);\n')
            original=media._docker
            def observed(*args,**kwargs):
                self.assertFalse(any('PASSWORD=' in str(arg) or 'postgres://' in str(arg) and '@' in str(arg) and ':' in str(arg).split('@')[0][11:] for arg in args))
                return original(*args,**kwargs)
            with patch.object(media,'_docker',side_effect=observed):
                restore._run_go_as_role(fixture.target,'target_admin','allowed_db','clone_writer')
                with self.assertRaises(media.RestoreError):
                    restore._run_go_as_role(fixture.target,'target_admin','denied_db','clone_denied')
            self.assertEqual(fixture.sql(fixture.target,'target_admin','allowed_db',
                "SELECT pg_get_userbyid(relowner) FROM pg_class WHERE oid='public.qualified_role_marker'::regclass").strip(),'clone_writer')
            self.assertEqual(fixture.sql(fixture.target,'target_admin','denied_db',
                "SELECT to_regclass('public.qualified_role_marker') IS NULL").strip(),'t')
        finally:
            media._cleanup(tool_journal,resources)


if __name__ == '__main__': unittest.main()
