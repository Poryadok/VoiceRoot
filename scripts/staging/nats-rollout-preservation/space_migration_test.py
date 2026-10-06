import copy
import unittest
import space_migration as space

class SpaceTests(unittest.TestCase):
    def row(self,version=15,count=2,default='true'):
        return {'database':'space_db','version':version,'dirty':False,
            'allow_guests_true':count,'column_default':default,'column_type':'boolean'}

    def test_crossing_requires_backup_and_preserves_precount(self):
        state=space.requirement(self.row(),24)
        self.assertTrue(state['backup_required']);self.assertEqual(state['before']['allow_guests_true'],2)
        with self.assertRaises(space.SpaceError):space.require_backup(state,None)

    def test_already_applied_preserves_legitimate_later_opt_in(self):
        state=space.requirement(self.row(24,7,'false'),24)
        self.assertFalse(state['backup_required'])
        self.assertEqual(state['before']['allow_guests_true'],7)

    def test_dirty_unknown_and_non_boolean_schema_refused(self):
        for key,value in (('dirty',True),('version',None),('version',True),('allow_guests_true',-1),('column_type','text')):
            row=self.row();row[key]=value
            with self.subTest(key=key,value=value),self.assertRaises(space.SpaceError):space.requirement(row,24)

    def test_post_crossing_requires_zero_default_false_clean_target(self):
        state=space.requirement(self.row(),24)
        self.assertEqual(space.verify_after(state,self.row(24,0,'false'))['allow_guests_true'],0)
        for key,value in (('version',23),('dirty',True),('allow_guests_true',1),('column_default','true')):
            row=self.row(24,0,'false');row[key]=value
            with self.subTest(key=key),self.assertRaises(space.SpaceError):space.verify_after(state,row)

    def test_restore_or_offnode_cannot_be_asserted_without_exact_dump_binding(self):
        state=space.requirement(self.row(),24)
        backup={'schema':'voice-space-backup-v1','before':copy.deepcopy(state['before']),
            'snapshot':'00000003-0000001B-1','dump_sha256':'a'*64,'dump_bytes':128,
            'restored':True,'offnode_verified':True}
        self.assertEqual(space.require_backup(state,backup),backup)
        for key,value in (('restored',False),('offnode_verified',False),('dump_sha256','x'),('dump_bytes',0),('snapshot','arbitrary SQL')):
            bad=copy.deepcopy(backup);bad[key]=value
            with self.subTest(key=key),self.assertRaises(space.SpaceError):space.require_backup(state,bad)
        bad=copy.deepcopy(backup);bad['before']['allow_guests_true']=1
        with self.assertRaises(space.SpaceError):space.require_backup(state,bad)
