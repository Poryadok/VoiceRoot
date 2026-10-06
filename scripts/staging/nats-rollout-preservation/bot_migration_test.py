import unittest
from unittest.mock import Mock,patch
import bot_migration

class BotPrerequisiteTest(unittest.TestCase):
    def row(self,**values):
        return dict(database='bot_db',version=3,dirty=False,duplicate_token_groups=0,**values)
    def test_empty_token_collision_vetoes_forward_unique_index(self):
        row=self.row();row['duplicate_token_groups']=1
        with self.assertRaises(bot_migration.PrerequisiteError):bot_migration.verify(row)
        self.assertIn('interaction_token IS NOT NULL',bot_migration.SQL)
        self.assertNotIn("interaction_token <> ''",bot_migration.SQL)
    def test_clean_bound_schema_retains_exact_observation(self):
        row=self.row()
        self.assertEqual(bot_migration.verify(row),row)
        for field,value in [('database','space_db'),('dirty',True),('version',True),
                            ('version',0),('duplicate_token_groups',False),
                            ('duplicate_token_groups',-1)]:
            with self.subTest(field=field,value=value):
                changed=dict(row);changed[field]=value
                with self.assertRaises(bot_migration.PrerequisiteError):bot_migration.verify(changed)
        with self.assertRaises(bot_migration.PrerequisiteError):bot_migration.verify(dict(row,extra=0))
    def test_only_named_canonical_index_file_selects_preflight(self):
        row={'database':'bot','files':{'000004_slash_interaction_outbox.up.sql':'sql'}}
        self.assertTrue(bot_migration.required([row]))
        self.assertFalse(bot_migration.required([dict(row,database='search')]))
        self.assertFalse(bot_migration.required([dict(row,files={'000001_init.up.sql':'sql'})]))
    def test_fixed_database_query_binds_authority_and_rejects_drift(self):
        kube=Mock();kube.run.return_value=self.row()
        with patch('space_authority.capture',return_value={'pod':'fixed'}) as authority:
            receipt=bot_migration.capture(kube)
        self.assertEqual(receipt,{'authority':{'pod':'fixed'},'observation':self.row()})
        self.assertEqual(authority.call_count,2)
        argv=kube.run.call_args.args[0]
        self.assertEqual(argv[:4],['exec','voice-postgres-0','--','sh'])
        self.assertEqual(argv[-1],bot_migration.SQL)
        self.assertIn('-d bot_db',argv[5])
        with patch('space_authority.capture',side_effect=[{'pod':'old'},{'pod':'new'}]):
            with self.assertRaises(bot_migration.PrerequisiteError):bot_migration.capture(kube)

if __name__=='__main__':unittest.main()
