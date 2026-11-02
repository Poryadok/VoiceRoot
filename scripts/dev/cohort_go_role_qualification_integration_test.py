"""Isolated PostgreSQL direct-login migration privilege regression."""
import hashlib
import io
import subprocess
import tarfile
import time
import unittest
from unittest.mock import patch

import story_media_backup_cohort_restore as restore
import story_media_backup_restore as media
import story_media_backup_integration_test as fixtures
from story_media_backup_capture_integration_test import cleanup_fixture
from story_media_backup_restore_integration_test import Journal, MIGRATE


class GoRoleIntegrationTest(unittest.TestCase):
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
