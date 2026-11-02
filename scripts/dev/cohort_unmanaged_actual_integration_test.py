"""Synthetic PG16 populated extra DB, encrypted readback and isolated restore.

This checks source data and catalog only, not a protected cohort authorization.
"""
import io
import copy
import hashlib
import json
import os
from pathlib import Path
import secrets
import subprocess
import tempfile
import time
import tarfile
import unittest
from types import SimpleNamespace
from unittest.mock import patch

import cohort_unmanaged_actual as unmanaged
import full_cohort_backup_metadata as source_metadata
import full_cohort_affected_plan as affected
import story_media_backup_cohort as cohort
import story_media_backup_cohort_restore as restore
import story_media_backup_restore as media
import story_media_backup as backup
import story_media_backup_integration_test as fixtures
from story_media_backup_capture_integration_test import cleanup_fixture
from story_media_backup_restore_integration_test import Journal, POSTGRES, MIGRATE


class UnmanagedIntegrationTest(unittest.TestCase):
    def test_restore_queries_ignore_shadowed_migration_ledger(self):
        fixture=fixtures.PopulatedRestoreTest();self.addCleanup(cleanup_fixture,fixture);fixture.setUp()
        deadline=time.monotonic()+60
        while True:
            ready=subprocess.run(['docker','exec',fixture.target,'sh','-ec',
                'test "$(cat /proc/1/comm)" = postgres && exec psql -X -qAt -v ON_ERROR_STOP=1 -U target_admin -d postgres -c "SELECT 1"'],
                stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=10)
            if ready.returncode==0 and ready.stdout.strip()==b'1':break
            if time.monotonic()>=deadline:
                raise RuntimeError('shadow fixture final-postgres readiness failed; exit='+str(ready.returncode)+'; stderr=redacted')
        sql=('CREATE SCHEMA shadow; CREATE TABLE public.schema_migrations(version bigint,dirty boolean); '
            'CREATE TABLE shadow.schema_migrations(version bigint,dirty boolean); '
            'INSERT INTO public.schema_migrations VALUES (25,false); INSERT INTO shadow.schema_migrations VALUES (99,true); '
            'ALTER ROLE target_admin SET search_path=shadow,public,pg_catalog')
        result=subprocess.run(['docker','exec','-i',fixture.target,'psql','-X','-qAt','-v','ON_ERROR_STOP=1',
                               '-U','target_admin','-d','postgres'],input=sql.encode(),stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=30)
        if result.returncode:
            raise RuntimeError('shadow fixture setup failed; exit='+str(result.returncode)+'; stderr=redacted')
        result=json.loads(media._psql(fixture.target,'target_admin','postgres',media.CATALOG))
        self.assertEqual(result['history'],[{'version':25,'dirty':False}])

    def test_synthetic_receipt_full_encrypted_cohort_uses_real_restore(self):
        """Actual dump/crypto/restore; receipt verifier is explicitly synthetic."""
        fixture=fixtures.PopulatedRestoreTest();self.addCleanup(cleanup_fixture,fixture);fixture.setUp()
        fixture.sql(fixture.source,'source_admin','postgres','CREATE ROLE app_writer LOGIN')
        fixture.sql(fixture.source,'source_admin','postgres','CREATE DATABASE extra_db')
        fixture.sql(fixture.source,'source_admin','postgres','CREATE DATABASE gateway_db')
        for db in ('auth_db','voice_db'):
            fixture.sql(fixture.source,'source_admin','postgres','CREATE DATABASE '+db)
        fixture.sql(fixture.source,'source_admin','auth_db',
            "CREATE TABLE public.flyway_schema_history(installed_rank int PRIMARY KEY, version varchar(50), script varchar(1000) NOT NULL, checksum int, success bool NOT NULL);"
            "INSERT INTO public.flyway_schema_history VALUES (1,'32','V32__linked_oauth_transport.sql',123,true)")
        voice_folder=Path('/work/src/backend/migrations/voice_db')
        auth_folder=Path('/work/src/backend/auth/src/main/resources/db/migration')
        for migration in sorted(voice_folder.glob('*.up.sql')):
            if int(migration.name[:6])<=14:fixture.sql(fixture.source,'source_admin','voice_db',migration.read_text())
        fixture.sql(fixture.source,'source_admin','voice_db',
                    'CREATE TABLE public.schema_migrations(version bigint NOT NULL,dirty boolean NOT NULL); INSERT INTO public.schema_migrations VALUES (14,false)')
        for db in ('auth_db','voice_db'):
            fixture.sql(fixture.source,'source_admin',db,'CREATE TABLE public.fixture_marker(id int PRIMARY KEY); INSERT INTO public.fixture_marker VALUES (1)')
        fixture.sql(fixture.source,'source_admin','postgres','ALTER DATABASE gateway_db CONNECTION LIMIT 7')
        fixture.sql(fixture.source,'source_admin','postgres','REVOKE CONNECT ON DATABASE gateway_db FROM PUBLIC')
        gateway_folder=Path('/work/src/backend/migrations/gateway_db')
        for migration in sorted(gateway_folder.glob('*.up.sql')):
            fixture.sql(fixture.source,'source_admin','gateway_db',migration.read_text())
        fixture.sql(fixture.source,'source_admin','postgres','CREATE DATABASE custom_template TEMPLATE template0')
        fixture.sql(fixture.source,'source_admin','postgres','ALTER DATABASE custom_template IS_TEMPLATE true')
        for db in ('extra_db','postgres','template1','custom_template'):
            fixture.sql(fixture.source,'source_admin',db,
                "CREATE SCHEMA tenant AUTHORIZATION app_writer; CREATE TABLE tenant.events(id bigint PRIMARY KEY, value text); "
                "ALTER TABLE tenant.events OWNER TO app_writer; INSERT INTO tenant.events VALUES (7,'kept'); "
                "CREATE TABLE public.schema_migrations(version bigint,dirty boolean); INSERT INTO public.schema_migrations VALUES (991,true)")
        databases=sorted(('extra_db','postgres','template1','custom_template','gateway_db','auth_db','voice_db'))
        catalogs={db:json.loads(fixture.sql(fixture.source,'source_admin',db,
            restore.AUTH_CATALOG if db=='auth_db' else media.CATALOG if db=='voice_db' else
            restore.GATEWAY_CATALOG if db=='gateway_db' else unmanaged.CATALOG_SQL)) for db in databases}
        properties={}
        for db in databases:
            source_catalog=(json.loads(fixture.sql(fixture.source,'source_admin',db,unmanaged.CATALOG_SQL))
                            if db in ('gateway_db','auth_db','voice_db') else catalogs[db])
            value=source_catalog['database_properties']
            properties[db]={key:value[key] for key in ('template','allowconn','encoding','collate','ctype','locale_provider','icu_locale','connection_limit')}
            properties[db].update(owner=source_catalog['database_owner'],acl=value['acl_raw'])
        properties['template0']={'allowconn':False}
        oids={db:int(fixture.sql(fixture.source,'source_admin','postgres',
                              "SELECT oid FROM pg_database WHERE datname='"+db+"'")) for db in databases}
        all_oids=dict(oids,template0=int(fixture.sql(fixture.source,'source_admin','postgres',"SELECT oid FROM pg_database WHERE datname='template0'")))
        engine={'namespace_uid':'fixture'};start='fixture'
        event={'operation_id':'media-c2-'+'a'*32,'source_sha':'b'*40,'namespace_uid':'fixture',
               'engine':engine,'server_start':start,'databases':oids,'all_databases':all_oids,'properties':properties,
               'excluded':{'template0':{'oid':all_oids['template0']}},'scope':'all-databases-connectable-backup'}
        def digest(value):return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',',':'),ensure_ascii=True).encode()).hexdigest()
        roles=json.loads(fixture.sql(fixture.source,'source_admin','postgres',media.role_catalog_sql()))
        plans={db:{'engine':'unmanaged','from':None,'to':None,'files':{}} for db in databases}
        plans['gateway_db']['files']={p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in gateway_folder.glob('*.sql')}
        plans['auth_db']={'engine':'flyway','from':32,'to':32,
                          'files':{p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in auth_folder.glob('*.sql')}}
        plans['voice_db']={'engine':'golang-migrate','from':14,'to':15,
                           'files':{p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in voice_folder.glob('*.sql')}}
        metadata={'catalog':catalogs,'roles':roles,'migrations':plans}
        tool_journal=Journal();resources=[]
        try:
            cid=media._create(tool_journal,resources,'migrate',MIGRATE,['--entrypoint','/bin/true'])
            raw=media._docker('cp',cid+':/usr/local/bin/migrate','-')
            with tarfile.open(fileobj=io.BytesIO(raw),mode='r:') as archive:
                files=archive.getmembers();self.assertEqual(len(files),1);self.assertEqual(files[0].name,'migrate')
                tool_sha=hashlib.sha256(archive.extractfile(files[0]).read()).hexdigest()
        finally:media._cleanup(tool_journal,resources)
        binding={'operation_id':event['operation_id'],'snapshot_id':'capture-'+'c'*32,'source_sha':event['source_sha'],
                 'databases':databases,'sources':{db:{'engine':engine,'server_start':start,'database_oid':oids[db]} for db in databases},
                 'full_catalog':{'receipt_sha256':digest(event),'properties':properties},
                 'dump_receipts':{name:'d'*64 for name in databases+['roles']},
                 'restore_inputs':{'catalog_sha256':digest(catalogs),'roles_catalog_sha256':digest(roles),
                    'migration_catalog_sha256':digest(plans),'postgres_image':POSTGRES,'migrate_image':MIGRATE,
                    'migrate_binary_sha256':tool_sha,'postgres_major':16}}
        binding['sources']['roles']={'engine':engine,'server_start':start}
        phase=object.__new__(affected.FullCatalogPhase)
        fence=SimpleNamespace(context={'operation_id':event['operation_id']},ledger=SimpleNamespace(read=lambda:({'fixture':True},('uid','rv'))))
        phase.fence=fence;phase.lock={'fixture':True};phase.namespace_uid='fixture'
        metadata_phase=object.__new__(source_metadata.BackupMetadataPhase)
        metadata_phase.fence=fence;metadata_phase.catalog=phase;metadata_phase.lock=phase.lock;metadata_phase.namespace_uid='fixture'
        metadata_receipt={'operation_id':binding['operation_id'],'source_sha':binding['source_sha'],
            'databases':databases,'sources':binding['sources'],'full_catalog':binding['full_catalog'],
            'restore_inputs':binding['restore_inputs'],'metadata':metadata,
            'migration_roles':{'voice_db':{'role':'source_admin'}}}
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);key=secrets.token_bytes(32)
            def producer(member,output):
                command=(['pg_dumpall','-U','source_admin','--roles-only','--no-role-passwords'] if member=='roles'
                         else ['pg_dump','-U','source_admin','-Fc','-d',member])
                result=subprocess.run(['docker','exec',fixture.source,*command],stdout=output,stderr=subprocess.DEVNULL,timeout=90)
                if result.returncode:raise RuntimeError('fixture dump failed')
            manifest=cohort.export(root,key,binding,databases,producer)
            with self.assertRaises(restore.CohortRestoreError):
                restore.qualify(root,key,binding,databases,manifest,metadata,Path('/work'),POSTGRES,MIGRATE,Journal())
            changed=copy.deepcopy(metadata);changed['catalog']['extra_db']['relations'].clear()
            with patch.object(affected.FullCatalogPhase,'verify_current',return_value=event),self.assertRaises(restore.CohortRestoreError):
                restore.qualify(root,key,binding,databases,manifest,changed,Path('/work'),POSTGRES,MIGRATE,Journal(),phase,'synthetic')
            stale=copy.deepcopy(event);stale['databases']['extra_db']+=1
            with patch.object(affected.FullCatalogPhase,'verify_current',return_value=stale),self.assertRaises(restore.CohortRestoreError):
                restore.qualify(root,key,binding,databases,manifest,metadata,Path('/work'),POSTGRES,MIGRATE,Journal(),phase,'synthetic')
            observed=[];events=[];histories={'auth_db':[],'voice_db':[]};original=media._psql;docker=media._docker
            def checked_docker(*args,**kwargs):
                self.assertFalse(any('POSTGRES_PASSWORD=' in str(arg) or 'VOICE_RESTORE_URL=' in str(arg)
                                     or ('postgres://' in str(arg) and ':' in str(arg).split('@')[0][11:]) for arg in args))
                return docker(*args,**kwargs)
            def psql(cid,admin,db,sql,timeout=120):
                result=original(cid,admin,db,sql,timeout=timeout)
                if sql==restore.GATEWAY_CATALOG:
                    value=json.loads(original(cid,admin,db,unmanaged.CATALOG_SQL))['database_properties']
                    self.assertEqual(value['connection_limit'],7)
                    self.assertEqual(value['acl'],json.loads(fixture.sql(fixture.source,'source_admin',db,unmanaged.CATALOG_SQL))['database_properties']['acl'])
                    events.append('gateway')
                if sql=='ALTER DATABASE template0 WITH ALLOW_CONNECTIONS true':events.append('template')
                if sql in (media.CATALOG,restore.AUTH_CATALOG):
                    self.assertEqual(original(cid,admin,db,'SELECT count(*) FROM public.fixture_marker').strip(),b'1')
                    query=("SELECT coalesce(json_agg(json_build_object('version',version,'dirty',dirty) ORDER BY version),'[]'::json) FROM public.schema_migrations"
                           if db=='voice_db' else "SELECT coalesce(json_agg(json_build_object('version',version,'script',script,'checksum',checksum,'success',success) ORDER BY installed_rank),'[]'::json) FROM public.flyway_schema_history")
                    history=json.loads(original(cid,admin,db,query))
                    self.assertEqual(json.loads(result)['history'],history)
                    histories[db].append(history)
                if sql==unmanaged.CATALOG_SQL and db not in ('gateway_db','auth_db','voice_db'):
                    self.assertEqual(original(cid,admin,db,'SELECT id,value FROM tenant.events').strip(),b'7|kept')
                    self.assertEqual(original(cid,admin,db,'SELECT version,dirty FROM public.schema_migrations').strip(),b'991|t')
                    observed.append(db)
                return result
            with patch.object(affected.FullCatalogPhase,'verify_current',return_value=event),patch.object(media,'_psql',side_effect=psql), \
                 patch.object(media,'_docker',side_effect=checked_docker), \
                 patch.object(source_metadata,'verify_completed',return_value=metadata_receipt):
                receipt=restore.qualify(root,key,binding,databases,manifest,metadata,Path('/work'),POSTGRES,MIGRATE,Journal(),phase,'synthetic',metadata_phase,'synthetic')
            self.assertEqual(receipt['cleanup'],'verified')
            expected={db:{'mode':'restore-only','from':None,'to':None} for db in databases}
            expected['auth_db']={'mode':'restore-only','from':32,'to':32}
            expected['voice_db']={'mode':'migrate','from':14,'to':15}
            self.assertEqual(receipt['databases'],expected)
            self.assertEqual(receipt['migration_logins'],{'voice_db':{'role':'source_admin','from':14,'to':15}})
            self.assertEqual(catalogs['voice_db']['history'],[{'version':14,'dirty':False}])
            self.assertEqual(histories['voice_db'],[[{'version':14,'dirty':False}],[{'version':15,'dirty':False}]])
            self.assertEqual(len(histories['auth_db']),2)
            self.assertEqual(histories['auth_db'],[catalogs['auth_db']['history'],catalogs['auth_db']['history']])
            self.assertEqual(catalogs['auth_db']['history'],[{'version':'32','script':'V32__linked_oauth_transport.sql','checksum':123,'success':True}])
            extras=[db for db in databases if db not in ('gateway_db','auth_db','voice_db')]
            self.assertEqual(sorted(observed),sorted(extras*2))
            self.assertIn('gateway',events);self.assertIn('template',events)
            self.assertLess(events.index('gateway'),events.index('template'))
            damaged=copy.deepcopy(manifest);damaged['mac']='0'*64
            with patch.object(affected.FullCatalogPhase,'verify_current',return_value=event),self.assertRaises(restore.CohortRestoreError):
                restore.qualify(root,key,binding,databases,damaged,metadata,Path('/work'),POSTGRES,MIGRATE,Journal(),phase,'synthetic')

    def test_populated_extra_catalog_and_ledger_are_preserved(self):
        fixture=fixtures.PopulatedRestoreTest()
        self.addCleanup(cleanup_fixture,fixture);fixture.setUp()
        fixture.sql(fixture.source,'source_admin','postgres','CREATE ROLE app_writer LOGIN; CREATE ROLE a_reader LOGIN')
        fixture.sql(fixture.source,'source_admin','postgres','CREATE DATABASE extra_db')
        fixture.sql(fixture.source,'source_admin','postgres','CREATE DATABASE custom_template TEMPLATE template0')
        fixture.sql(fixture.source,'source_admin','postgres','ALTER DATABASE custom_template IS_TEMPLATE true')
        fixture.sql(fixture.source,'source_admin','extra_db',
            "CREATE SCHEMA tenant AUTHORIZATION app_writer; "
            "CREATE TABLE tenant.events(id bigint PRIMARY KEY, value text NOT NULL); "
            "ALTER TABLE tenant.events OWNER TO app_writer; INSERT INTO tenant.events VALUES (7,'kept'); "
            "ALTER TABLE tenant.events ENABLE ROW LEVEL SECURITY; "
            "CREATE POLICY allow_writer ON tenant.events TO app_writer,a_reader USING (id > 0); "
            "CREATE SEQUENCE tenant.counter START 42; SELECT nextval('tenant.counter'); "
            "CREATE TABLE public.schema_migrations(version bigint,dirty boolean); "
            "INSERT INTO public.schema_migrations VALUES (991,true)")
        for db in ('postgres','template1','custom_template'):
            fixture.sql(fixture.source,'source_admin',db,
                        "CREATE SCHEMA preserved; CREATE TABLE preserved.marker(value text); INSERT INTO preserved.marker VALUES ('kept')")
        source=json.loads(fixture.sql(fixture.source,'source_admin','extra_db',unmanaged.CATALOG_SQL))
        self.assertTrue(any(row['schema']=='tenant' and row['name']=='events' for row in source['relations']))
        self.assertTrue(any(row['name']=='schema_migrations' for row in source['relations']))
        self.assertTrue(next(row for row in source['relations'] if row['schema']=='tenant' and row['name']=='events')['row_security'])
        self.assertEqual(next(row for row in source['policies'] if row['name']=='allow_writer')['roles'],['a_reader','app_writer'])
        self.assertEqual(next(row for row in source['sequences'] if row['name']=='counter')['last_value'],42)
        key=secrets.token_bytes(32)
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory)
            def encrypted(member,command):
                dump=subprocess.run(['docker','exec',fixture.source,*command],stdout=subprocess.PIPE,
                                    stderr=subprocess.PIPE,timeout=90,check=True).stdout
                readfd,writefd=os.pipe();os.write(writefd,key.hex().encode()+b'\n');os.close(writefd)
                try:
                    result=subprocess.run(['openssl','enc','-aes-256-cbc','-pbkdf2','-iter','200000','-salt',
                         '-pass','file:/proc/self/fd/'+str(readfd)],input=dump,stdout=subprocess.PIPE,
                         stderr=subprocess.PIPE,pass_fds=(readfd,),check=True,timeout=90)
                finally:os.close(readfd)
                path=root/(member+'.enc');fd=os.open(path,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
                with os.fdopen(fd,'wb') as out:out.write(result.stdout)
                return path,backup.identity(path,key)
            role_path,role_id=encrypted('roles',['pg_dumpall','-U','source_admin','--roles-only','--no-role-passwords'])
            db_path,db_id=encrypted('extra_db',['pg_dump','-U','source_admin','-Fc','-d','extra_db'])
            roles=io.BytesIO();backup._decrypt_snapshot(os.open(role_path,os.O_RDONLY),key,role_id,roles)
            fixture.sql(fixture.target,'target_admin','postgres',roles.getvalue().decode())
            maintenance='voice_cohort_private_123'
            fixture.sql(fixture.target,'target_admin','postgres','CREATE DATABASE '+maintenance+' TEMPLATE template0')
            unmanaged.prepare_disposable_template(
                lambda db,sql:fixture.sql(fixture.target,'target_admin',db,sql),source)
            proc=subprocess.Popen(['docker','exec','-i',fixture.target,'pg_restore','-U','target_admin',
                '--exit-on-error','--create','-d','postgres'],stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
            backup._decrypt_snapshot(os.open(db_path,os.O_RDONLY),key,db_id,proc.stdin)
            proc.stdin.close();err=proc.stderr.read();self.assertEqual(proc.wait(timeout=120),0,err.decode())
            proc.stdout.close();proc.stderr.close()
            restored=json.loads(fixture.sql(fixture.target,'target_admin','extra_db',unmanaged.CATALOG_SQL))
            self.assertEqual(unmanaged.restore_result(source,restored),{'mode':'restore-only','from':None,'to':None})
            self.assertEqual(fixture.sql(fixture.target,'target_admin','extra_db','SELECT id,value FROM tenant.events'), '7|kept')
            self.assertEqual(fixture.sql(fixture.target,'target_admin','extra_db','SELECT version,dirty FROM public.schema_migrations'), '991|t')
            for db in ('postgres','template1','custom_template'):
                original=json.loads(fixture.sql(fixture.source,'source_admin',db,unmanaged.CATALOG_SQL))
                source_path,source_id=encrypted(db,['pg_dump','-U','source_admin','-Fc','-d',db])
                unmanaged.prepare_disposable_template(lambda name,sql:fixture.sql(fixture.target,'target_admin',name,sql),original,maintenance)
                if db in ('postgres','template1'):
                    unmanaged.prepare_disposable_database(lambda name,sql:fixture.sql(fixture.target,'target_admin',name,sql),db,maintenance)
                proc=subprocess.Popen(['docker','exec','-i',fixture.target,'pg_restore','-U','target_admin',
                    '--exit-on-error','--create','-d',maintenance],stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
                backup._decrypt_snapshot(os.open(source_path,os.O_RDONLY),key,source_id,proc.stdin)
                proc.stdin.close();err=proc.stderr.read();self.assertEqual(proc.wait(timeout=120),0,err.decode())
                proc.stdout.close();proc.stderr.close()
                if original['database_properties']['template']:
                    fixture.sql(fixture.target,'target_admin',maintenance,'ALTER DATABASE "'+db+'" IS_TEMPLATE true')
                restored=json.loads(fixture.sql(fixture.target,'target_admin',db,unmanaged.CATALOG_SQL))
                self.assertEqual(unmanaged.restore_result(original,restored),{'mode':'restore-only','from':None,'to':None})
                self.assertEqual(fixture.sql(fixture.target,'target_admin',db,'SELECT value FROM preserved.marker'),'kept')
            with db_path.open('ab') as f:f.write(b'tamper')
            with self.assertRaises(backup.BackupError):
                backup._decrypt_snapshot(os.open(db_path,os.O_RDONLY),key,db_id,io.BytesIO())


if __name__=='__main__':unittest.main()
