import copy
from pathlib import Path
import unittest
from unittest.mock import Mock,patch
import space_safeguards as safeguards
import space_migration

BEFORE={'database':'space_db','version':15,'dirty':False,'allow_guests_true':2,
    'column_default':'true','column_type':'boolean'}
AFTER=dict(BEFORE,version=24,allow_guests_true=0,column_default='false')

class Exporter:
    def __init__(self,order,before=BEFORE):self.order=order;self.before=copy.deepcopy(before)
    def __enter__(self):self.order.append('snapshot-open');return self
    def __exit__(self,*args):self.order.append('snapshot-closed')
    def dump(self,path,limit):
        self.order.append('dump-held-snapshot')
        return {'schema':'voice-space-backup-v1','before':self.before,'snapshot':'00000003-00000008-1',
            'dump_sha256':'a'*64,'dump_bytes':10,'restored':False,'offnode_verified':False}

class Tests(unittest.TestCase):
    def binding(self):
        return {'authority':{'image':'docker.io/library/postgres@sha256:'+'b'*64},
            'requirement':space_migration.requirement(BEFORE,24),'max_dump_bytes':1000,
            'source_sha':'c'*40,'migration_plan_sha256':'d'*64}

    def test_combined_capacity_vetoes_cipher_or_either_disk_before_fence(self):
        from types import SimpleNamespace
        native={'pvc_capacity_bytes':1<<30,'max_files':1000,'required_free_bytes':2<<30}
        binding=self.binding();binding['max_dump_bytes']=1<<30
        def disk(p):return SimpleNamespace(free=100<<30)
        with patch.object(safeguards.shutil,'disk_usage',side_effect=disk),patch.object(safeguards,'command',return_value=b'/var/lib/docker\n'),patch.object(safeguards,'directory',side_effect=lambda p:p):
            self.assertLess(safeguards.combined_capacity(Path('/private'),native,binding)['cipher_max_bytes'],64<<30)
            binding['max_dump_bytes']=64<<30
            with self.assertRaisesRegex(safeguards.Blocked,'cipher_capacity'):safeguards.combined_capacity(Path('/private'),native,binding)
            binding['max_dump_bytes']=1<<30
            with patch.object(safeguards.shutil,'disk_usage',return_value=SimpleNamespace(free=1)):
                with self.assertRaisesRegex(safeguards.Blocked,'backup_disk'):safeguards.combined_capacity(Path('/private'),native,binding)
            with patch.object(safeguards.shutil,'disk_usage',side_effect=lambda p:SimpleNamespace(free=1 if str(p)=='/var/lib/docker' else 100<<30)):
                with self.assertRaisesRegex(safeguards.Blocked,'restore_disk'):safeguards.combined_capacity(Path('/private'),native,binding)
    def test_target_is_derived_from_bound_space_up_files(self):
        plan=[{'database':'space','files':{'000016_fail_closed.up.sql':'source','000024_auth.up.sql':'source',
            '000099_unused.down.sql':'source'}}]
        self.assertEqual(safeguards.target(plan),24)
        self.assertIsNone(safeguards.target([]))
    def test_exporter_closes_before_restore_and_offnode_remains_false(self):
        order=[];binding=self.binding();writes=[]
        def restore(path,backup,image,op):
            self.assertEqual(order[-2:],['snapshot-closed','fence'])
            self.assertFalse(backup['restored']);self.assertFalse(backup['offnode_verified'])
            order.append('isolated-restore');return {'restored':True,'image':image}
        with patch.object(safeguards.space_authority,'revalidate'),patch.object(safeguards,'Postgres'),\
             patch.object(safeguards,'Snapshot',side_effect=lambda b:Exporter(order)),\
             patch.object(safeguards,'capacity',return_value=1000),patch.object(safeguards,'restore',side_effect=restore):
            receipt=safeguards.capture(None,Path('/private'),binding,'a'*12,lambda:order.append('fence'),
                lambda p,r:writes.append((p,r)))
        self.assertTrue(receipt['exporter_closed']);self.assertTrue(receipt['backup']['restored'])
        self.assertFalse(receipt['backup']['offnode_verified'])
        self.assertEqual(receipt['operation'],'a'*12)
        self.assertEqual(receipt['source_sha'],binding['source_sha'])
        self.assertEqual(receipt['migration_plan_sha256'],binding['migration_plan_sha256'])
        self.assertEqual(writes[0][0].name,'space-before-manifest.json')
    def test_closed_counts_may_advance_before_fence_but_schema_cannot(self):
        for field,value,allowed in [('allow_guests_true',3,True),('version',17,False)]:
            before=dict(BEFORE,**{field:value});order=[]
            with patch.object(safeguards.space_authority,'revalidate'),patch.object(safeguards,'Postgres'),\
                 patch.object(safeguards,'Snapshot',side_effect=lambda b:Exporter(order,before)),\
                 patch.object(safeguards,'capacity',return_value=1000),patch.object(safeguards,'restore',return_value={'restored':True}):
                if allowed:
                    row=safeguards.capture(None,Path('/private'),self.binding(),'a'*12,lambda:None,lambda *a:None)
                    self.assertEqual(row['backup']['before'][field],value)
                else:
                    with self.assertRaises(safeguards.Blocked):
                        safeguards.capture(None,Path('/private'),self.binding(),'a'*12,lambda:None,lambda *a:None)
    def test_missing_offnode_or_changed_closed_snapshot_prevents_migration(self):
        binding=self.binding()
        with patch.object(safeguards.space_authority,'revalidate'),patch.object(safeguards,'Postgres'),\
             patch.object(safeguards,'Snapshot') as snapshot:
            with self.assertRaises(space_migration.SpaceError):safeguards.before_migrate(None,binding,None)
            snapshot.assert_not_called()
        backup=Exporter([]).dump(None,1000);backup.update(restored=True,offnode_verified=True)
        with patch.object(safeguards.space_authority,'revalidate'),patch.object(safeguards,'Postgres'),\
             patch.object(safeguards,'Snapshot',side_effect=lambda b:Exporter([],dict(BEFORE,allow_guests_true=3))):
            with self.assertRaises(safeguards.Blocked):safeguards.before_migrate(None,binding,backup)
    def test_postcondition_fails_for_wrong_version_count_or_default(self):
        for bad in (dict(AFTER,version=23),dict(AFTER,allow_guests_true=1),dict(AFTER,column_default='true')):
            with patch.object(safeguards.space_authority,'revalidate'),patch.object(safeguards,'Postgres'),\
                 patch.object(safeguards,'Snapshot',side_effect=lambda b:Exporter([],bad)):
                with self.assertRaises(space_migration.SpaceError):safeguards.after_migrate(None,self.binding())

if __name__=='__main__':unittest.main()
