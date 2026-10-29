"""Actual guarded PG16 cohort archive/clone for populated Story 4→8.

External source, Kube engine and paused admission are synthetic. Opt in only
on the already-authorized disposable Linux Docker test runner; this is not live
STAGING admission, a four-hour proof, or Story runtime compatibility evidence.
"""
import copy
import hashlib
import importlib
import json
import os
from pathlib import Path
import secrets
import shutil
import subprocess
import sys
import tempfile
import time
import unittest
from unittest.mock import patch

sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'nats-known-baseline'))
sys.path.insert(0,str(Path(__file__).resolve().parents[2]/'dev'))
import commands
import guard
import resource_claim
import staging_cohort_preservation as owner
import staging_cohort_preservation_pg as pg
import staging_cohort_preservation_test as fixtures
import story_media_backup_restore as media_restore
from staging_cohort_preservation_runtime import DOCKER
from auth_user_adoption_runtime import QUALIFIED_TOOLS


@unittest.skipUnless(os.environ.get('VOICE_COHORT_DOCKER')=='1','owned Linux Docker fixture required')
class StoryGuardedRestoreTest(unittest.TestCase):
    def test_populated_story4_archive_and_guarded_restore_migrate8(self):
        original_popen=subprocess.Popen
        def docker(*args,body=None):
            result=subprocess.run(DOCKER+list(args),input=body,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=120)
            if result.returncode: self.fail('owned Docker fixture command failed: '+str(args[:2]))
            return result.stdout
        name='voice-story-cohort-source-'+secrets.token_hex(12)
        cid=docker('create','--name',name,'--network','none','--label','voice.test.owner='+name,
                   '-e','POSTGRES_PASSWORD=isolated-test-only','-e','POSTGRES_USER=postgres',QUALIFIED_TOOLS['postgres']).decode().strip()
        try:
            docker('start',cid);deadline=time.monotonic()+60
            while subprocess.run(DOCKER+['exec',cid,'pg_isready','-h','127.0.0.1','-U','postgres'],
                                 stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL).returncode:
                if time.monotonic()>deadline:self.fail('owned PostgreSQL unavailable')
                time.sleep(.2)
            def sql(database,body,user='postgres'):
                return docker('exec','-i',cid,'psql','-X','-qAt','-v','ON_ERROR_STOP=1','-U',user,'-d',database,
                              body=body if isinstance(body,bytes) else body.encode()).decode().strip()
            self.assertTrue(sql('postgres','SHOW server_version_num;').startswith('16'))
            sql('postgres','CREATE ROLE story_owner LOGIN; CREATE DATABASE story_db OWNER story_owner;')
            sql('story_db','ALTER SCHEMA public OWNER TO story_owner;')
            source=Path(__file__).resolve().parents[3]
            files=sorted((source/'src/backend/migrations/story_db').glob('*.up.sql'))
            self.assertEqual([int(p.name[:6]) for p in files],list(range(1,9)))
            for path in files[:4]:sql('story_db',path.read_bytes(),'story_owner')
            sql('story_db',"""CREATE TABLE schema_migrations(version bigint NOT NULL PRIMARY KEY,dirty boolean NOT NULL);
                INSERT INTO schema_migrations VALUES(4,false);
                INSERT INTO stories(id,author_profile_id,type,text_content,visibility,expires_at,archived_until)
                VALUES('11111111-1111-4111-8111-111111111111','22222222-2222-4222-8222-222222222222',
                       'text','preserved content','everyone','2030-01-01Z','2030-02-01Z');""",'story_owner')
            self.assertEqual(sql('story_db','SELECT version||\'|\'||dirty FROM schema_migrations;','story_owner'),'4|false')
            approved={'source_files':{p.relative_to(source).as_posix():hashlib.sha256(p.read_bytes()).hexdigest()
                for prefix in ('src/backend/migrations','src/backend/auth/src/main/resources/db/migration')
                for p in (source/prefix).rglob('*.sql')}}
            actual_capture=commands.capture
            def capture(argv,**kwargs):
                if argv[:len(pg.EXEC)]==pg.EXEC:
                    commands.authority_guard(argv)
                    return docker('exec','-i',cid,*argv[len(pg.EXEC):],body=kwargs.get('body'))
                return actual_capture(argv,**kwargs)
            def popen(argv,*args,**kwargs):
                if argv[:len(pg.EXEC)]==pg.EXEC:argv=DOCKER+['exec','-i',cid,*argv[len(pg.EXEC):]]
                return original_popen(argv,*args,**kwargs)
            namespace='11111111-1111-4111-8111-111111111111'
            engine={'pod':{'uid':'22222222-2222-4222-8222-222222222222','container_id':'containerd://'+'c'*64,'restart_count':0},
                    'pvc':{'uid':'33333333-3333-4333-8333-333333333333'},'pv':{'uid':'44444444-4444-4444-8444-444444444444'}}
            with tempfile.TemporaryDirectory(dir='/root') as directory:
                root=Path(directory);(root/'installed').mkdir(mode=0o700)
                before={'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':'voice-user','namespace':'voice-staging',
                    'uid':'user','resourceVersion':'3','generation':1},'spec':{'replicas':1}}
                observed=[]; original_psql=media_restore._psql
                def inspect_clone(cid,admin,database,statement,timeout=120):
                    result=original_psql(cid,admin,database,statement,timeout=timeout)
                    try:value=json.loads(result)
                    except (ValueError,UnicodeError):value=None
                    if database=='story_db' and isinstance(value,dict) and value.get('history')==[{'version':8,'dirty':False}]:
                        query="""SELECT json_build_object(
                            'text',(SELECT text_content FROM stories WHERE id='11111111-1111-4111-8111-111111111111'),
                            'owner',(SELECT pg_get_userbyid(relowner) FROM pg_class WHERE oid='public.stories'::regclass),
                            'owner_super',(SELECT rolsuper FROM pg_roles WHERE rolname='story_owner'),
                            'gate_count',(SELECT count(*) FROM story_reference_inventory_gate));"""
                        observed.append(json.loads(original_psql(cid,admin,database,query)))
                    return result
                with patch.object(guard,'ROOT',root),patch.object(owner,'_source',return_value=(approved,{},{})), \
                     patch.object(owner,'revalidate_paused'),patch.object(owner,'shared_module',side_effect=lambda _session,module:importlib.import_module(module)), \
                     patch('realtime_bootstrap.engine_snapshot',return_value=(namespace,engine)), \
                     patch.object(commands,'capture',side_effect=capture),patch.object(subprocess,'Popen',side_effect=popen), \
                     patch.object(media_restore,'_psql',side_effect=inspect_clone):
                    preflight={'schema':'voice-staging-cohort-preflight-v1','operation':'a'*12,
                               'services':[],'actor':None,'nats':{},'enrollment':{},'contract':{}}
                    session,_=fixtures.CutEffectTests().fixture(root,before,namespace,preflight=preflight)
                    owner.private._private_directory(root/'installed'/'sources')
                    owner.private._private_directory(root/'installed'/'sources'/session.operation)
                    shutil.copytree(source/'src/backend/migrations',root/'installed'/'sources'/session.operation/'src/backend/migrations')
                    with resource_claim.command_scope(session.check):
                        archive=owner._capture_archive(session)
                        self.assertIn('story_db',archive['databases'])
                        self.assertIn('story_db',owner.dump_receipts(session))
                        receipt=owner._qualify_restore(session,'local')
                        self.assertEqual(receipt['status'],'COMPLETE')
                        self.assertEqual(receipt['proof']['cleanup'],'verified')
                        self.assertEqual(receipt['proof']['databases']['story_db'],{'mode':'migrate','from':4,'to':8})
                        self.assertTrue(observed)
                        self.assertTrue(all(value=={'text':'preserved content','owner':'story_owner',
                                                   'owner_super':False,'gate_count':1} for value in observed))
            self.assertEqual(sql('story_db',"SELECT text_content FROM stories WHERE id='11111111-1111-4111-8111-111111111111';",'story_owner'),
                             'preserved content')
            self.assertEqual(sql('story_db','SELECT version||\'|\'||dirty FROM schema_migrations;','story_owner'),'4|false')
        finally:
            if cid:
                label=docker('inspect',cid,'--format','{{index .Config.Labels "voice.test.owner"}}').decode().strip()
                if label!=name:raise AssertionError('fixture ownership changed')
                docker('rm','-f',cid)


if __name__=='__main__': unittest.main()
