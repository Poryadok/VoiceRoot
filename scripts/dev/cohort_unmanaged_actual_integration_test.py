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
import unittest
from unittest.mock import patch

import cohort_unmanaged_actual as unmanaged
import full_cohort_affected_plan as affected
import story_media_backup_cohort as cohort
import story_media_backup_cohort_restore as restore
import story_media_backup_restore as media
import story_media_backup as backup
import story_media_backup_integration_test as fixtures
from story_media_backup_capture_integration_test import cleanup_fixture
from story_media_backup_restore_integration_test import Journal, POSTGRES, MIGRATE


class UnmanagedIntegrationTest(unittest.TestCase):
    def test_synthetic_receipt_full_encrypted_cohort_uses_real_restore(self):
        """Actual dump/crypto/restore; receipt verifier is explicitly synthetic."""
        fixture=fixtures.PopulatedRestoreTest();self.addCleanup(cleanup_fixture,fixture);fixture.setUp()
        fixture.sql(fixture.source,'source_admin','postgres','CREATE ROLE app_writer LOGIN')
        fixture.sql(fixture.source,'source_admin','postgres','CREATE DATABASE extra_db')
        fixture.sql(fixture.source,'source_admin','postgres','CREATE DATABASE custom_template TEMPLATE template0')
        fixture.sql(fixture.source,'source_admin','postgres','ALTER DATABASE custom_template IS_TEMPLATE true')
        for db in ('extra_db','postgres','template1','custom_template'):
            fixture.sql(fixture.source,'source_admin',db,
                "CREATE SCHEMA tenant AUTHORIZATION app_writer; CREATE TABLE tenant.events(id bigint PRIMARY KEY, value text); "
                "ALTER TABLE tenant.events OWNER TO app_writer; INSERT INTO tenant.events VALUES (7,'kept'); "
                "CREATE TABLE public.schema_migrations(version bigint,dirty boolean); INSERT INTO public.schema_migrations VALUES (991,true)")
        databases=sorted(('extra_db','postgres','template1','custom_template'))
        catalogs={db:json.loads(fixture.sql(fixture.source,'source_admin',db,unmanaged.CATALOG_SQL)) for db in databases}
        properties={}
        for db in databases:
            value=catalogs[db]['database_properties']
            properties[db]={key:value[key] for key in ('template','allowconn','encoding','collate','ctype','locale_provider','icu_locale','connection_limit')}
            properties[db].update(owner=catalogs[db]['database_owner'],acl=value['acl_raw'])
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
        metadata={'catalog':catalogs,'roles':roles,'migrations':plans}
        binding={'operation_id':event['operation_id'],'snapshot_id':'capture-'+'c'*32,'source_sha':event['source_sha'],
                 'databases':databases,'sources':{db:{'engine':engine,'server_start':start,'database_oid':oids[db]} for db in databases},
                 'full_catalog':{'receipt_sha256':digest(event),'properties':properties},
                 'dump_receipts':{name:'d'*64 for name in databases+['roles']},
                 'restore_inputs':{'catalog_sha256':digest(catalogs),'roles_catalog_sha256':digest(roles),
                    'migration_catalog_sha256':digest(plans),'postgres_image':POSTGRES,'migrate_image':MIGRATE,
                    'migrate_binary_sha256':'e'*64,'postgres_major':16}}
        binding['sources']['roles']={'engine':engine,'server_start':start}
        phase=object.__new__(affected.FullCatalogPhase)
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
            observed=[];original=media._psql
            def psql(cid,admin,db,sql,timeout=120):
                result=original(cid,admin,db,sql,timeout=timeout)
                if sql==unmanaged.CATALOG_SQL:
                    self.assertEqual(original(cid,admin,db,'SELECT id,value FROM tenant.events').strip(),b'7|kept')
                    self.assertEqual(original(cid,admin,db,'SELECT version,dirty FROM public.schema_migrations').strip(),b'991|t')
                    observed.append(db)
                return result
            with patch.object(affected.FullCatalogPhase,'verify_current',return_value=event),patch.object(media,'_psql',side_effect=psql):
                receipt=restore.qualify(root,key,binding,databases,manifest,metadata,Path('/work'),POSTGRES,MIGRATE,Journal(),phase,'synthetic')
            self.assertEqual(receipt['cleanup'],'verified')
            self.assertEqual(receipt['databases'],{db:{'mode':'restore-only','from':None,'to':None} for db in databases})
            self.assertEqual(sorted(observed),sorted(databases*2))
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
