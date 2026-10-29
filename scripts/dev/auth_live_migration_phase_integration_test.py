"""Isolated real Flyway 26→32 history against a read-only copied source tree."""
import hashlib
import io
import json
import os
from pathlib import Path
import subprocess
import tarfile
import time
import unittest
import uuid
from unittest.mock import patch

import auth_live_migration_phase as live
from auth_user_adoption_runtime import QUALIFIED_TOOLS


@unittest.skipUnless(os.environ.get('RUN_DOCKER_INTEGRATION')=='1','explicit isolated Docker integration')
class RealHistoryTest(unittest.TestCase):
    def test_service_login_rejects_bad_password_even_if_loopback_trusts(self):
        name='auth-live-login-'+uuid.uuid4().hex
        network=None; pg=None
        def docker(*args,data=None,check=True):
            return subprocess.run(['docker',*args],input=data,stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,timeout=120,check=check)
        try:
            network=docker('network','create','--internal',name).stdout.decode().strip()
            pg=docker('create','--pull=never','--network',name,'--network-alias','voice-postgres',
                '-e','POSTGRES_USER=voice','-e','POSTGRES_PASSWORD=fixture','-e','POSTGRES_HOST_AUTH_METHOD=scram-sha-256',
                QUALIFIED_TOOLS['postgres']).stdout.decode().strip()
            docker('start',pg)
            for _ in range(100):
                if docker('exec',pg,'pg_isready','-h','voice-postgres','-U','voice',check=False).returncode==0: break
                time.sleep(.2)
            else: self.fail('isolated PostgreSQL unavailable')
            docker('exec','-e','PGPASSWORD=fixture',pg,'psql','-h','voice-postgres','-U','voice','-d','postgres',
                   '-X','-qAt','-v','ON_ERROR_STOP=1','-c','CREATE DATABASE auth_db OWNER voice')
            docker('exec',pg,'sh','-ec','sed -i "1i host all all 127.0.0.1/32 trust" "$PGDATA/pg_hba.conf"')
            docker('exec','-e','PGPASSWORD=fixture',pg,'psql','-h','voice-postgres','-U','voice','-d','postgres',
                   '-X','-qAt','-v','ON_ERROR_STOP=1','-c','SELECT pg_reload_conf()')
            self.assertEqual(docker('exec','-e','PGPASSWORD=wrong',pg,'psql','-h','127.0.0.1','-U','voice',
                '-d','auth_db','-X','-qAt','-c','SELECT 1',check=False).returncode,0)
            original=subprocess.run
            def local_exec(args,**kwargs):
                if args[0]=='kubectl': return original(['docker','exec','-i',pg,'sh','-ec',args[-1]],**kwargs)
                return original(args,**kwargs)
            with patch.object(live.probe.subprocess,'run',side_effect=local_exec):
                with self.assertRaises(subprocess.CalledProcessError): live.probe._login(b'wrong')
                result=live.probe._login(b'fixture')
            self.assertEqual(result['role'],'voice'); self.assertEqual(result['database'],'auth_db')
        finally:
            if pg and docker('rm','-fv',pg,check=False).returncode: raise RuntimeError('owned PG cleanup uncertain')
            if network and docker('network','rm',network,check=False).returncode: raise RuntimeError('owned network cleanup uncertain')

    def test_populated_source_26_to_32(self):
        root=Path(__file__).resolve().parents[2]/'src/backend/auth/src/main/resources'
        sources={p.name:p.read_bytes() for p in (root/'db/migration').glob('*.sql')}
        catalog={k:hashlib.sha256(v).hexdigest() for k,v in sources.items()}
        profile=(root/'roleidentityauthority/migration-history-linked-oauth-v1.json').read_bytes()
        archive=io.BytesIO()
        with tarfile.open(fileobj=archive,mode='w') as tar:
            for name,raw in sorted(sources.items()):
                entry=tarfile.TarInfo('sql/'+name); entry.mode=0o644; entry.size=len(raw)
                tar.addfile(entry,io.BytesIO(raw))
        ids=[]
        def docker(*args,data=None):
            result=subprocess.run(['docker',*args],input=data,stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,timeout=180,check=True)
            return result.stdout
        def create(*args):
            ident=docker('create','--pull=never',*args).decode().strip()
            self.assertRegex(ident,r'^[0-9a-f]{64}$'); ids.append(ident); return ident
        def query(pg,sql):
            return docker('exec','-i','-e','PGPASSWORD=fixture',pg,'psql','-h','127.0.0.1','-U','voice',
                          '-d','auth_db','-X','-qAt','-v','ON_ERROR_STOP=1',data=sql.encode())
        try:
            pg=create('--network','none','-e','POSTGRES_USER=voice','-e','POSTGRES_PASSWORD=fixture',QUALIFIED_TOOLS['postgres'])
            docker('start',pg)
            for _ in range(100):
                if subprocess.run(['docker','exec',pg,'pg_isready','-h','127.0.0.1','-U','voice'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL).returncode==0: break
                time.sleep(.2)
            else: self.fail('isolated PostgreSQL unavailable')
            docker('exec','-e','PGPASSWORD=fixture',pg,'psql','-h','127.0.0.1','-U','voice','-d','postgres',
                   '-X','-qAt','-v','ON_ERROR_STOP=1','-c','CREATE DATABASE auth_db OWNER voice')
            def migrate(target):
                container=create('--network','container:'+pg,'-e','FLYWAY_USER=voice','-e','FLYWAY_PASSWORD=fixture',
                    '-e','FLYWAY_URL=jdbc:postgresql://127.0.0.1:5432/auth_db',QUALIFIED_TOOLS['flyway'],
                    '-locations=filesystem:/flyway/sql','-target='+str(target),'migrate')
                docker('cp','-',container+':/flyway',data=archive.getvalue())
                docker('start','-a',container)
            migrate(26)
            query(pg,"INSERT INTO accounts(id,password_hash,type,status,deleted_at) VALUES "
                "('00000000-0000-0000-0000-000000000001','x','guest','active',NULL),"
                "('00000000-0000-0000-0000-000000000002','x','guest','active','2020-01-01');")
            sql="SELECT coalesce(json_agg(row_to_json(h) ORDER BY installed_rank),'[]'::json) FROM flyway_schema_history h"
            before=json.loads(query(pg,sql))
            self.assertTrue(live.matches_expected_history(before,live.expected_history(catalog,profile,26)))
            q=lambda text:json.loads(query(pg,text))
            columns=q("SELECT coalesce(json_object_agg(c.relname,(SELECT json_agg(a.attname ORDER BY a.attnum) "
                "FROM pg_attribute a WHERE a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped)),'{}'::json) "
                "FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind='r'")
            clone={'original_table_columns':columns,'before_markers':live.marker_snapshot(q,columns,before=True),
                   'expected_after_markers':live.marker_snapshot(q,columns,before=True,normalize=True),
                   'roles_sha256':live._digest(q(live.media_restore.role_catalog_sql())),
                   'before_catalog_sha256':live._digest(q(live.restore.AUTH_CATALOG))}
            with patch.object(live.probe.metadata,'kube_engine_snapshot',return_value=lambda:{'fixture':'pg'}), \
                 patch.object(live.probe,'_query',side_effect=q):
                live._c1_data_snapshot({'fixture':'pg'},clone,True,lambda:None)
            migrate(32)
            after=json.loads(query(pg,sql))
            self.assertTrue(live.matches_expected_history(after,live.expected_history(catalog,profile,32)))
            live.verify_transition(before,after,after)
            clone['after_catalog_sha256']=live._digest(q(live.restore.AUTH_CATALOG))
            with patch.object(live.probe.metadata,'kube_engine_snapshot',return_value=lambda:{'fixture':'pg'}), \
                 patch.object(live.probe,'_query',side_effect=q):
                live._c1_data_snapshot({'fixture':'pg'},clone,False,lambda:None)
            self.assertEqual(query(pg,'SELECT count(*) FROM accounts').strip(),b'2')
        finally:
            for ident in reversed(ids):
                result=subprocess.run(['docker','rm','-fv',ident],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=30)
                if result.returncode: raise RuntimeError('owned fixture cleanup uncertain')


if __name__=='__main__': unittest.main()
