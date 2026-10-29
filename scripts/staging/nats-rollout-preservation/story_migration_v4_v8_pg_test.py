"""Disposable PostgreSQL 16 populated-v4 dump/restore and target-v8 test.

Opt in with VOICE_STORY_TEST_PG_CONTAINER naming an owned, network-none test
container labelled voice.test.owner equal to its exact name. No live endpoint,
STAGING credential, production restore authority or Story runtime is used.
"""
import os
from pathlib import Path
import subprocess
import unittest


@unittest.skipUnless(os.environ.get('VOICE_STORY_TEST_PG_CONTAINER'),'owned PG16 fixture required')
class StoryMigrationTest(unittest.TestCase):
    def test_populated_v4_dump_restore_upgrade_to_v8_preserves_rows_and_ownership(self):
        name=os.environ['VOICE_STORY_TEST_PG_CONTAINER']
        self.assertRegex(name,r'^voice-story-v4v8-pg-[a-f0-9]{16}$')
        def docker(*args,body=None):
            result=subprocess.run(['docker',*args],input=body,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=90)
            if result.returncode:
                self.fail('owned PG fixture command failed: '+str(args[:3]))
            return result.stdout
        self.assertEqual(docker('inspect',name,'--format','{{index .Config.Labels "voice.test.owner"}}').decode().strip(),name)
        self.assertEqual(docker('inspect',name,'--format','{{.HostConfig.NetworkMode}}').decode().strip(),'none')
        def sql(database,statement,role='story_owner'):
            return docker('exec','-i',name,'psql','-X','-qAt','-v','ON_ERROR_STOP=1','-U',role,'-d',database,
                          body=statement if isinstance(statement,bytes) else statement.encode()).decode().strip()
        self.assertEqual(sql('postgres','SHOW server_version_num;','postgres')[:2],'16')
        sql('postgres','CREATE ROLE story_owner LOGIN; CREATE ROLE story_other LOGIN;','postgres')
        docker('exec',name,'createdb','-U','postgres','-O','story_owner','story_v4')
        docker('exec',name,'createdb','-U','postgres','-O','story_owner','story_restore')
        root=Path(__file__).resolve().parents[3]/'src/backend/migrations/story_db'
        files=sorted(root.glob('*.up.sql'))
        self.assertEqual([int(p.name[:6]) for p in files],list(range(1,9)))
        sql('story_v4','CREATE TABLE schema_migrations(version bigint NOT NULL PRIMARY KEY,dirty boolean NOT NULL);')
        for file in files[:4]: sql('story_v4',file.read_bytes())
        sql('story_v4',"""INSERT INTO schema_migrations VALUES(4,false);
            INSERT INTO stories(id,author_profile_id,type,text_content,visibility,expires_at,archived_until)
            VALUES('11111111-1111-4111-8111-111111111111','22222222-2222-4222-8222-222222222222',
                   'text','preserved content','everyone','2030-01-01Z','2030-02-01Z');
            INSERT INTO story_views(story_id,viewer_profile_id) VALUES
            ('11111111-1111-4111-8111-111111111111','33333333-3333-4333-8333-333333333333');""")
        before=sql('story_v4',"SELECT id||'|'||author_profile_id||'|'||text_content FROM stories;")
        self.assertEqual(sql('story_v4','SELECT version||\'|\'||dirty FROM schema_migrations;'),'4|false')
        docker('exec',name,'pg_dump','-U','story_owner','-d','story_v4','--format=custom','--no-owner',
               '--file=/tmp/story-v4-owned-fixture.dump')
        docker('exec',name,'pg_restore','-U','story_owner','-d','story_restore','--exit-on-error','--no-owner',
               '/tmp/story-v4-owned-fixture.dump')
        self.assertEqual(sql('story_restore',"SELECT id||'|'||author_profile_id||'|'||text_content FROM stories;"),before)
        self.assertEqual(sql('story_restore','SELECT version||\'|\'||dirty FROM schema_migrations;'),'4|false')
        for version,file in enumerate(files[4:],5):
            sql('story_restore','UPDATE schema_migrations SET version='+str(version)+',dirty=true;')
            sql('story_restore',file.read_bytes())
            sql('story_restore','UPDATE schema_migrations SET dirty=false;')
        self.assertEqual(sql('story_restore','SELECT version||\'|\'||dirty FROM schema_migrations;'),'8|false')
        self.assertEqual(sql('story_restore',"SELECT id||'|'||author_profile_id||'|'||text_content FROM stories;"),before)
        self.assertEqual(sql('story_restore','SELECT count(*) FROM story_views;'),'1')
        self.assertEqual(sql('story_restore','SELECT count(*) FROM story_reference_inventory_gate;'),'1')
        self.assertEqual(sql('story_restore',"SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace "
            "WHERE n.nspname='public' AND c.relkind IN ('r','p') AND pg_get_userbyid(c.relowner)<>'story_owner';"),'0')
        self.assertEqual(sql('story_restore',"SELECT rolsuper FROM pg_roles WHERE rolname='story_owner';"),'f')
        self.assertEqual(sql('story_restore',"SELECT has_table_privilege('story_owner','stories','INSERT'),"
            "has_table_privilege('story_other','stories','INSERT');"),'t|f')


if __name__=='__main__': unittest.main()
