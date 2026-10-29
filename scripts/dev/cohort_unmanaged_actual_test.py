"""Bounded unmanaged source catalog checks; no live authority."""
import copy
import hashlib
import json
import unittest
from unittest.mock import patch
from types import SimpleNamespace

import cohort_unmanaged_actual as unmanaged
import full_cohort_affected_plan as affected
import story_media_backup_cohort as cohort
import story_media_backup_cohort_restore as restore
import cohort_backup_phases as phases


class UnmanagedActualTest(unittest.TestCase):
    def setUp(self):
        self.catalog = {'database_owner':'source_admin', 'database_properties':{'encoding':'UTF8'}, 'schemas':[], 'relations':[],
                        'columns':[], 'constraints':[], 'indexes':[], 'routines':[],
                        'types':[], 'extensions':[], 'defaults':[], 'policies':[], 'triggers':[], 'sequences':[]}

    def test_unmanaged_catalog_is_restore_only_even_when_ledger_exists(self):
        self.catalog['relations'] = [{'schema':'public','name':'schema_migrations','kind':'r',
                                      'owner':'source_admin','acl':None}]
        self.assertEqual(unmanaged.restore_result(self.catalog,copy.deepcopy(self.catalog)),
                         {'mode':'restore-only','from':None,'to':None})

    def test_missing_or_changed_catalog_is_not_accepted(self):
        for changed in (None, {}, dict(self.catalog,database_properties={'encoding':'LATIN1'}),dict(self.catalog,schemas=[{'name':'lost'}]),
                        dict(self.catalog,relations='bad')):
            with self.subTest(changed=changed), self.assertRaises(unmanaged.UnmanagedError):
                unmanaged.restore_result(self.catalog,changed)

    def test_arbitrary_ledger_is_not_a_migration_assertion(self):
        result=unmanaged.restore_result(self.catalog,self.catalog)
        self.assertNotIn('version',result)
        self.assertNotIn('compatibility',result)

    def test_template_uses_the_exact_source_plpgsql_owner(self):
        self.catalog['extensions']=[{'name':'plpgsql','version':'1.0','owner':'source_admin'}]
        commands=[]
        unmanaged.prepare_disposable_template(lambda db,sql: commands.append((db,sql)),self.catalog)
        self.assertEqual(commands[0],('postgres','ALTER DATABASE template0 WITH ALLOW_CONNECTIONS true'))
        self.assertIn('SET ROLE "source_admin"',commands[1][1])
        self.assertEqual(commands[-1],('postgres','ALTER DATABASE template0 WITH ALLOW_CONNECTIONS false'))

    def test_builtin_targets_are_recreated_only_from_private_maintenance_database(self):
        commands=[]
        unmanaged.prepare_disposable_database(lambda db,sql:commands.append((db,sql)),
                                               'template1','voice_cohort_private_123')
        self.assertEqual(commands,[('voice_cohort_private_123','ALTER DATABASE "template1" IS_TEMPLATE false'),
                                   ('voice_cohort_private_123','DROP DATABASE "template1"')])
        with self.assertRaises(unmanaged.UnmanagedError):
            unmanaged.prepare_disposable_database(lambda *args:None,'template0','postgres')

    def test_full_scope_needs_typed_current_receipt_and_matching_oids(self):
        engine={'namespace_uid':'fixture'}
        receipt={'operation_id':'op','source_sha':'a'*40,'namespace_uid':'fixture','engine':engine,
                 'server_start':'now','databases':{'extra_db':11,'postgres':12},
                 'all_databases':{'extra_db':11,'postgres':12,'template0':13},
                 'properties':{'extra_db':{'allowconn':True},'postgres':{'allowconn':True},'template0':{'allowconn':False}},
                 'excluded':{'template0':{'oid':13}},'scope':'all-databases-connectable-backup'}
        digest=hashlib.sha256(json.dumps(receipt,sort_keys=True,separators=(',',':')).encode()).hexdigest()
        binding={'operation_id':'media-c1-'+'a'*32,'source_sha':'a'*40,'snapshot_id':'capture-'+'b'*32,'databases':['extra_db','postgres'],
                 'sources':{'roles':{'engine':engine,'server_start':'now'},
                            'extra_db':{'engine':engine,'server_start':'now','database_oid':11},
                            'postgres':{'engine':engine,'server_start':'now','database_oid':12}},
                 'full_catalog':{'receipt_sha256':digest,'properties':copy.deepcopy(receipt['properties'])}}
        receipt['operation_id']=binding['operation_id']
        binding['full_catalog']['receipt_sha256']=hashlib.sha256(json.dumps(receipt,sort_keys=True,separators=(',',':')).encode()).hexdigest()
        phase=object.__new__(affected.FullCatalogPhase)
        with patch.object(affected.FullCatalogPhase,'verify_current',return_value=receipt):
            self.assertEqual(unmanaged.verify_catalog_phase(phase,'token',binding),receipt)
            self.assertEqual(cohort._members(binding,binding['databases']),{'extra_db','postgres','roles'})
            unbound=copy.deepcopy(binding);del unbound['full_catalog']
            with self.assertRaises(cohort.CohortError):cohort._members(unbound,unbound['databases'])
            changed=copy.deepcopy(binding);changed['sources']['extra_db']['database_oid']=99
            with self.assertRaises(unmanaged.UnmanagedError):unmanaged.verify_catalog_phase(phase,'token',changed)
            with self.assertRaises(unmanaged.UnmanagedError):unmanaged.verify_catalog_phase(object(),'token',binding)
            changed=copy.deepcopy(binding);changed['full_catalog']['receipt_sha256']='0'*64
            with self.assertRaises(unmanaged.UnmanagedError):unmanaged.verify_catalog_phase(phase,'token',changed)
            changed=copy.deepcopy(binding);changed['databases']=['extra_db']
            with self.assertRaises(unmanaged.UnmanagedError):unmanaged.verify_catalog_phase(phase,'token',changed)
        with self.assertRaises(phases.CohortPhaseError):
            phases.CohortPhases(SimpleNamespace(intent={'cohort_binding':binding}),object(),'token')
        cut=object.__new__(affected.writer_cut.WriterCut)
        probe=object.__new__(affected.CatalogScopedProbe)
        probe.catalog=probe._bound_catalog=phase;probe.token=probe._bound_token='token'
        cut.probe=probe;cut.provision_successor={'fixture':True};cut.effective_session=None
        with patch.object(affected.FullCatalogPhase,'verify_receipt',return_value=receipt), \
             patch.object(affected.FullCatalogPhase,'verify_current',side_effect=AssertionError('original cannot be current')), \
             patch.object(affected,'verify_provisioned_catalog',return_value={'fixture':'successor'},create=True) as successor:
            self.assertEqual(unmanaged.verify_catalog_phase(phase,'token',binding,cut),receipt)
            successor.assert_called_once_with(probe,cut)
            probe.token='different'
            with self.assertRaises(unmanaged.UnmanagedError):unmanaged.verify_catalog_phase(phase,'token',binding,cut)
        probe.token='token';cut.effective_session={'fixture':True}
        with patch.object(affected.FullCatalogPhase,'verify_receipt',return_value=receipt), \
             patch.object(affected,'verify_effective_catalog',return_value={'fixture':'effective'}) as successor:
            self.assertEqual(unmanaged.verify_catalog_phase(phase,'token',binding,cut),receipt)
            successor.assert_called_once_with(probe,cut)
        with patch.object(affected.FullCatalogPhase,'verify_receipt',return_value=receipt), \
             patch.object(affected,'verify_effective_catalog',side_effect=RuntimeError('changed')):
            with self.assertRaises(unmanaged.UnmanagedError):unmanaged.verify_catalog_phase(phase,'token',binding,cut)

        cut.effective_session=None;cut.provision_successor=None;cut.space_fresh={'fixture':True}
        with patch.object(affected.FullCatalogPhase,'verify_receipt',return_value=receipt), \
             patch.object(affected.FullCatalogPhase,'verify_current',side_effect=AssertionError('original cannot be current')), \
             patch.object(affected,'verify_space_catalog',return_value={'fixture':'space'},create=True) as successor:
            self.assertEqual(unmanaged.verify_catalog_phase(phase,'token',binding,cut),receipt)
            successor.assert_called_once_with(probe,cut)

    def test_extra_plan_is_only_restore_only_in_full_catalog_mode(self):
        plan={'engine':'unmanaged','from':None,'to':None,'files':{}}
        with self.assertRaises(restore.CohortRestoreError):restore.classify_plan('extra_db',plan)
        self.assertEqual(restore.classify_plan('extra_db',plan,True),'restore-only')
        for change in ({'from':1},{'to':1},{'files':{'001.sql':'x'}},{'engine':'golang-migrate'}):
            with self.subTest(change=change),self.assertRaises(restore.CohortRestoreError):
                restore.classify_plan('extra_db',plan|change,True)

    def test_service_databases_restore_before_mutating_disposable_template(self):
        self.assertEqual(restore.restore_order(['extra_db','postgres','template1','voice_db']),
                         ['voice_db','extra_db','postgres','template1'])

    def test_cut_binding_requires_original_scope_and_matching_ledger(self):
        owner=object.__new__(phases.CohortPhases)
        ledger=SimpleNamespace(read=lambda:({'status':'active'},('uid','rv')))
        fence=SimpleNamespace(context={'operation_id':'fixture'},ledger=ledger)
        owner.phases=SimpleNamespace(fence=fence,lock={'uid':'lock'},namespace_uid='ns')
        owner.binding={'full_catalog':{}};owner.catalog_phase=object();owner.catalog_token='token'
        owner._catalog_cut=owner._bound_catalog_cut=None
        cut=object.__new__(affected.writer_cut.WriterCut)
        cut.fence=fence;cut.lock={'uid':'lock'};cut.namespace_uid='ns'
        cut.provision_successor=cut.effective_session=cut.migration=None
        receipt={'engine':{'identity':'fixture'},'server_start':'now','all_databases':{'postgres':7,'template0':8}}
        scope={'engine':receipt['engine'],'server_start':'now','databases':['postgres','template0'],
               'database_oids':receipt['all_databases']}
        cut.admission={'session_scope':scope}
        with patch.object(phases.CohortPhases,'_guard'),patch.object(affected.writer_cut.WriterCut,'_guard'), \
             patch.object(unmanaged,'verify_catalog_phase',return_value=receipt):
            owner.bind_catalog_cut(cut)
            self.assertIs(owner._catalog_cut,cut)
            with self.assertRaises(phases.CohortPhaseError):owner.bind_catalog_cut(cut)
            owner._catalog_cut=owner._bound_catalog_cut=None
            cut.admission={'session_scope':scope|{'database_oids':{'postgres':7}}}
            with self.assertRaises(phases.CohortPhaseError):owner.bind_catalog_cut(cut)


if __name__=='__main__':unittest.main()
