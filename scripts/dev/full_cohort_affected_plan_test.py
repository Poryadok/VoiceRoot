import copy
import hashlib
import json
import sys
from types import SimpleNamespace
from pathlib import Path
import unittest
from unittest.mock import patch
import full_cohort_affected_plan as affected
import media_backup_phase_test as fixture
import media_operation_journal as journal


class AffectedTests(unittest.TestCase):
    def setUp(self):
        engine={'namespace_uid':'ns'}; session={'database_oid':10,'server_start':'start'}
        def entry(db,oid):
            return {'binding':{'state':'resolved','database':db,'role':'voice'},
                    'metadata':{'database':db,'engine':engine,'session':session|{'database_oid':oid}},'secondary':[]}
        primary=entry('messaging_db',10); other=entry('chat_db',11)
        primary['secondary']=[{'binding':other['binding'],'metadata':other['metadata'],'owner_consistency':'matched'}]
        self.obs={'source_sha':'a'*40,'package_sha256':'b'*64,'projection':{'messaging':{
            'uid':'dep','generation':1,'spec_sha256':'c'*64,'pods':[{'containers':[{'name':'messaging',
            'image_id':'containerd://sha256:'+'1'*64}]}]}},'database_observations':{
            'workloads':{'messaging':[{'status':'observed','result':primary}]}}}
        self.candidate={'operation_id':'media-c1-'+'1'*32,'source_sha':'a'*40,'package_sha256':'b'*64,
            'platform_digests':{'voice-messaging/messaging':'sha256:'+'2'*64}}
        ledger=type('Ledger',(),{'read':lambda obj:({'status':'active'},('uid','rv'))})()
        fence=type('Fence',(),{'context':{},'ledger':ledger})()
        class Package:
            def __init__(obj): obj.fence=fence; obj.lock={}; obj.namespace_uid='ns'
            def verify_current(obj,token): return copy.deepcopy(self.candidate)
        self.package=Package()
        class Inventory:
            def __init__(obj): obj.fence=fence; obj.lock={}; obj.namespace_uid='ns'; obj.package=self.package
            def verify_current(obj,token): return copy.deepcopy(self.obs)
        self.inventory=Inventory()
        self.patches=[patch.object(affected.package,'PackagePhase',Package),patch.object(affected.inventory,'InventoryPhase',Inventory),
                      patch.object(affected.metadata,'CONTAINERS',{'messaging':'messaging'}),
                      patch.object(affected.metadata,'OWNERS',{'messaging':'messaging_db','chat':'chat_db'})]
        for p in self.patches: p.start(); self.addCleanup(p.stop)

    def test_primary_and_secondary_included(self):
        result=affected.build(self.package,self.inventory,'token')
        self.assertEqual(set(result['databases']),{'messaging_db','chat_db'})
        self.assertEqual(result['writers']['voice-messaging']['reason'],'candidate_image_changed')

    def test_unknown_route_or_secondary_holds(self):
        e=self.obs['database_observations']['workloads']['messaging'][0]
        e['result']['secondary'][0]['owner_consistency']='hold'
        with self.assertRaises(affected.AffectedError): affected.build(self.package,self.inventory,'token')
        e['status']='hold'
        with self.assertRaises(affected.AffectedError): affected.build(self.package,self.inventory,'token')

    def test_unchanged_writer_sharing_secondary_database_is_cut(self):
        entry=self.obs['database_observations']['workloads']['messaging'][0]['result']['secondary'][0]
        self.obs['projection']['chat']={'uid':'chat-dep','generation':1,'spec_sha256':'d'*64,
            'pods':[{'containers':[{'name':'chat','image_id':'containerd://sha256:'+'3'*64}]}]}
        self.obs['database_observations']['workloads']['chat']=[{'status':'observed','result':{
            'binding':entry['binding'],'metadata':entry['metadata'],'secondary':[]}}]
        self.candidate['platform_digests']['voice-chat/chat']='sha256:'+'3'*64
        with patch.object(affected.metadata,'CONTAINERS',{'messaging':'messaging','chat':'chat'}), \
             patch.object(affected,'PLANNED_CONFIG_UNKNOWN',frozenset()):
            result=affected.build(self.package,self.inventory,'token')
            self.assertEqual(result['writers']['voice-chat']['reason'],'shares_affected_database')
            self.obs['database_observations']['workloads']['chat'][0]['status']='hold'
            with self.assertRaises(affected.AffectedError): affected.build(self.package,self.inventory,'token')

    def test_unobserved_fixed_nonowner_is_cut_as_potential_writer(self):
        self.obs['projection']['analytics']={'uid':'analytics','generation':1,'spec_sha256':'e'*64,
            'pods':[{'containers':[{'name':'analytics','image_id':'sha256:'+'1'*64}]}]}
        self.obs['database_observations']['workloads']['analytics']=[{'status':'observed','result':{
            'binding':{'state':'not_observed'},'startup_qualified':False}}]
        self.candidate['platform_digests']['voice-analytics/analytics']='sha256:'+'1'*64
        with patch.object(affected.metadata,'CONTAINERS',{'messaging':'messaging','analytics':'analytics'}):
            class Catalog:
                def __init__(obj): obj.inventory=self.inventory
                def verify_current(obj,token): return {'engine':{'namespace_uid':'ns'},'server_start':'start',
                    'databases':{'messaging_db':10,'chat_db':11,'postgres':12},
                    'all_databases':{'messaging_db':10,'chat_db':11,'postgres':12,'template0':13},
                    'properties':{},'excluded':{'template0':{'oid':13}}}
            with patch.object(affected,'FullCatalogPhase',Catalog):
                with self.assertRaises(affected.AffectedError): affected.build(self.package,self.inventory,'token')
                result=affected.build(self.package,self.inventory,'token',Catalog())
                self.assertEqual(result['writers']['voice-analytics']['reason'],'unclassified_potential_writer')
                self.assertEqual(result['writers']['voice-analytics']['routes'],[])
                self.assertTrue(result['full_catalog'])
                self.assertIn('postgres',result['databases'])
                self.assertIn('postgres',result['unmanaged_databases'])
                self.obs['database_observations']['workloads']['analytics'][0]['status']='hold'
                with self.assertRaises(affected.AffectedError): affected.build(self.package,self.inventory,'token',Catalog())

    def test_realtime_create_only_requires_typed_current_absence(self):
        import realtime_absent_phase
        self.obs['projection']['realtime']={'uid':'rt','generation':1,'spec_sha256':'d'*64,
            'pods':[{'containers':[{'name':'realtime','image_id':'sha256:'+'1'*64}]}]}
        self.obs['database_observations']['workloads']['realtime']=[{'status':'hold','reason':'database_configuration_missing'}]
        self.candidate['platform_digests']['voice-realtime/realtime']='sha256:'+'1'*64
        class Catalog:
            def __init__(obj):obj.inventory=self.inventory
            def verify_current(obj,token):return {'engine':{'namespace_uid':'ns'},'server_start':'start',
                'databases':{'messaging_db':10,'chat_db':11,'postgres':12},
                'all_databases':{'messaging_db':10,'chat_db':11,'postgres':12},'properties':{},'excluded':{}}
        catalog=Catalog()
        class Absence:
            def __init__(obj):obj.inventory=self.inventory; obj.catalog=catalog
            def verify_current(obj,token):return {'source_sha':'a'*40,'deployment_uid':'rt',
                'generation':1,'spec_sha256':'d'*64}
        with patch.object(affected.metadata,'CONTAINERS',{'messaging':'messaging','realtime':'realtime'}), \
             patch.object(affected,'FullCatalogPhase',Catalog), \
             patch.object(realtime_absent_phase,'RealtimeAbsentPhase',Absence):
            with self.assertRaises(affected.AffectedError): affected.build(self.package,self.inventory,'token',catalog)
            plan=affected.build(self.package,self.inventory,'token',catalog,Absence())
            self.assertEqual(plan['writers']['voice-realtime']['reason'],'qualified_create_only_realtime')
            self.assertNotIn('realtime_db',plan['databases'])
            self.obs['database_observations']['workloads']['realtime'][0]['reason']='runtime_read_failed'
            with self.assertRaises(affected.AffectedError): affected.build(self.package,self.inventory,'token',catalog,Absence())

    def test_drain_invokes_only_matching_source_derived_scope(self):
        spec={'replicas':1}; self.obs['projection']['messaging']['spec_sha256']=hashlib.sha256(
            json.dumps(spec,sort_keys=True,separators=(',',':')).encode()).hexdigest()
        plan=affected.build(self.package,self.inventory,'token'); dbs=plan['databases']; first=next(iter(dbs.values()))
        class Cut:
            def __init__(obj):
                obj.fence=self.package.fence; obj.lock={}; obj.namespace_uid='ns'; obj.calls=0
                obj.client=type('Client',(),{'call':lambda inner,*args:{'items':[]}})()
                obj.admission={'controllers':[{'kind':'Deployment','metadata':{'name':'voice-messaging','uid':'dep','generation':1},'spec':spec}],
                    'session_scope':{'engine':first['engine'],'server_start':first['server_start'],'databases':sorted(dbs),
                                     'database_oids':{db:dbs[db]['database_oid'] for db in sorted(dbs)}},
                    'guards':{'jobs':[],'cronjobs':[]},'linked':{'jobs':[],'cronjobs':[]}}
            def drain(obj): obj.calls+=1; return {'status':'drained'}
        cut=Cut()
        with patch.object(affected.writer_cut,'WriterCut',Cut):
            self.assertEqual(affected.drain(self.package,self.inventory,'token',cut)['cut']['status'],'drained')
            cut.admission['session_scope']['database_oids']['chat_db']=999
            with self.assertRaises(affected.AffectedError): affected.drain(self.package,self.inventory,'token',cut)
            cut.admission['session_scope']['database_oids']['chat_db']=11
            job={'metadata':{'uid':'job','name':'job','namespace':'voice-dev'},'spec':{},'status':{'active':1}}
            cut.admission['guards']['jobs']=affected.writer_cut.inventory([job])
            cut.client.call=lambda *args:{'items':[job] if args[1]=='jobs' else []}
            with self.assertRaises(affected.AffectedError): affected.drain(self.package,self.inventory,'token',cut)
        self.assertEqual(cut.calls,1)


class CatalogTests(unittest.TestCase):
    def test_completed_space_and_realtime_are_only_allowed_catalog_successors(self):
        import space_fresh_install_phase as space
        before={'engine':{'id':'engine'},'server_start':'start','all_databases':{'postgres':1,'space_db':2},
                'databases':{'postgres':1,'space_db':2},'excluded':{},'properties':{'postgres':{'p':1},'space_db':{'p':2}}}
        post={'all_databases':{'postgres':1,'space_db':3},'databases':{'postgres':1,'space_db':3},
              'excluded':{},'properties':{'postgres':{'p':1},'space_db':{'p':3}}}
        current=copy.deepcopy(post);current['all_databases']['realtime_db']=4
        current['databases']['realtime_db']=4;current['properties']['realtime_db']={'p':4}
        binding={'phase':object(),'token':'t','receipt_sha256':'f'*64}
        proof={'operation_id':'op','source_sha':'a'*40,'namespace_uid':'ns','receipt_sha256':'f'*64,
               'old_oid':2,'new_oid':3,'catalog':post,'original_catalog_sha256':affected.writer_cut.digest(before)}
        class Catalog:
            def verify_receipt(obj,_token):return copy.deepcopy(before)
        class Cut:
            def __init__(obj,probe):
                obj.probe=probe;obj.space_fresh=binding;obj.namespace_uid='ns'
                obj.fence=SimpleNamespace(context={'operation_id':'op','commit':'a'*40})
            def current_base_session_scope(obj):return {'engine':before['engine'],'server_start':'start',
                'databases':['postgres','space_db'],'database_oids':{'postgres':1,'space_db':3}}
        helper=SimpleNamespace(verify_binding=lambda *_args,**_kwargs:binding)
        scope={'engine':before['engine'],'server_start':'start','databases':['postgres','realtime_db','space_db'],
               'database_oids':{'postgres':1,'space_db':3,'realtime_db':4}}
        with patch.dict(sys.modules,{'writer_cut_space':helper}),patch.object(affected,'FullCatalogPhase',Catalog), \
             patch.object(affected.writer_cut,'WriterCut',Cut),patch.object(space,'verify_historical',return_value=proof), \
             patch.object(affected,'_full_catalog',return_value=current):
            probe=affected.CatalogScopedProbe(Catalog(),'t');cut=Cut(probe)
            self.assertEqual(affected._verify_catalog(probe,cut,scope),current)
            with patch.object(affected,'_full_catalog',return_value=post):
                self.assertEqual(affected.verify_space_catalog(probe,cut),post)
                post['all_databases']['foreign_db']=9
                with self.assertRaises(affected.AffectedError):affected.verify_space_catalog(probe,cut)
                del post['all_databases']['foreign_db']
            current['properties']['postgres']={'p':'changed'}
            with self.assertRaises(affected.AffectedError):affected._verify_catalog(probe,cut,scope)
            current['properties']['postgres']={'p':1};current['all_databases']['foreign_db']=9
            with self.assertRaises(affected.AffectedError):affected._verify_catalog(probe,cut,scope)

    def test_migration_catalog_uses_only_source_bound_execution(self):
        import writer_cut_migration
        before={'engine':{'id':'engine'},'server_start':'start','all_databases':{'postgres':1},
                'databases':{'postgres':1},'excluded':{},'properties':{'postgres':{'template':False}}}
        current={'all_databases':{'postgres':1,'realtime_db':2},'databases':{'postgres':1,'realtime_db':2},
                 'excluded':{},'properties':{'postgres':{'template':False},'realtime_db':{'template':False}}}
        state={'execution':{'database':'realtime_db','database_oid':2},'stable':{'operation_id':'op'}}
        class Catalog:
            def verify_receipt(obj,token): return copy.deepcopy(before)
        class Cut:
            def __init__(obj,probe):
                obj.probe=probe; obj.admission={'session_scope':{'engine':before['engine'],'server_start':'start',
                    'databases':['postgres'],'database_oids':{'postgres':1}}}
                obj.migration={'kind':'realtime','phase':object(),'root':'.','plan':copy.deepcopy(state)}
            def current_base_session_scope(obj):return copy.deepcopy(obj.admission['session_scope'])
        with patch.object(affected,'FullCatalogPhase',Catalog),patch.object(affected.writer_cut,'WriterCut',Cut), \
             patch.object(affected,'_full_catalog',return_value=current), \
             patch.object(writer_cut_migration,'normalized',return_value=(None,state)):
            probe=affected.CatalogScopedProbe(Catalog(),'token'); cut=Cut(probe)
            self.assertEqual(affected.verify_migration_catalog(probe,cut)['scope']['database_oids']['realtime_db'],2)
            current['all_databases']['foreign_db']=3
            with self.assertRaises(affected.AffectedError): affected.verify_migration_catalog(probe,cut)

    def test_effective_catalog_accepts_only_typed_realtime_successor(self):
        before={'engine':{'id':'engine'},'server_start':'start','all_databases':{'postgres':1},
                'databases':{'postgres':1},'excluded':{},'properties':{'postgres':{'template':False}}}
        current={'all_databases':{'postgres':1,'realtime_db':2},'databases':{'postgres':1,'realtime_db':2},
                 'excluded':{},'properties':{'postgres':{'template':False},'realtime_db':{'template':False}}}
        class Catalog:
            def verify_receipt(obj,token): return copy.deepcopy(before)
        class Cut:
            def __init__(obj,probe): obj.probe=probe
            def current_effective_session_scope(obj): return {'engine':before['engine'],'server_start':'start',
                'databases':['postgres','realtime_db'],'database_oids':{'postgres':1,'realtime_db':2}}
        with patch.object(affected,'FullCatalogPhase',Catalog), patch.object(affected.writer_cut,'WriterCut',Cut), \
             patch.object(affected,'_full_catalog',return_value=current):
            probe=affected.CatalogScopedProbe(Catalog(),'token'); cut=Cut(probe)
            self.assertEqual(affected.verify_effective_catalog(probe,cut),current)
            current['all_databases']['foreign_db']=3
            with self.assertRaises(affected.AffectedError): affected.verify_effective_catalog(probe,cut)

    def test_provisioned_sample_brackets_catalog_and_requires_zero(self):
        before={'engine':{'id':'engine'},'server_start':'start','all_databases':{'postgres':1},
                'databases':{'postgres':1},'excluded':{},'properties':{'postgres':{'template':False}}}
        current={'all_databases':{'postgres':1,'realtime_db':2},'databases':{'postgres':1,'realtime_db':2},
                 'excluded':{},'properties':{'postgres':{'template':False},'realtime_db':{'template':False}}}
        scope={'engine':before['engine'],'server_start':'start','databases':['postgres','realtime_db'],
               'database_oids':{'postgres':1,'realtime_db':2}}
        class Catalog:
            def verify_receipt(obj,token): return copy.deepcopy(before)
            def _guard(obj): pass
        class Cut:
            def __init__(obj,probe): obj.probe=probe
            def current_provisioned_session_scope(obj): return copy.deepcopy(scope)
        with patch.object(affected,'FullCatalogPhase',Catalog), patch.object(affected.writer_cut,'WriterCut',Cut), \
             patch.object(affected,'_full_catalog',return_value=current), \
             patch.object(affected.pg,'inspect',return_value={'scope':scope,'active_sessions':0,'active_transactions':0}) as sample:
            probe=affected.CatalogScopedProbe(Catalog(),'token'); cut=Cut(probe)
            self.assertEqual(affected.verify_provisioned_catalog(probe,cut),current)
            self.assertEqual(affected.inspect_provisioned(probe,cut)['scope'],scope)
            sample.return_value['active_sessions']=1
            with self.assertRaises(affected.AffectedError): affected.inspect_provisioned(probe,cut)
            sample.return_value['active_sessions']=0
            current['all_databases']['foreign_db']=3
            with self.assertRaises(affected.AffectedError): affected.inspect_provisioned(probe,cut)
            current['all_databases'].pop('foreign_db')
            def concurrent(*args):
                current['all_databases']['foreign_db']=3
                return {'scope':scope,'active_sessions':0,'active_transactions':0}
            sample.side_effect=concurrent
            with self.assertRaises(affected.AffectedError): affected.inspect_provisioned(probe,cut)

    def test_full_catalog_probe_holds_on_new_database(self):
        receipt={'engine':{'id':'engine'},'server_start':'start','all_databases':{'postgres':1},'databases':{'postgres':1}}
        changed=copy.deepcopy(receipt); changed['all_databases']['new_db']=2
        class Catalog:
            def __init__(obj): obj.values=[receipt,changed]
            def verify_current(obj,token): return obj.values.pop(0)
            def _guard(obj): pass
        with patch.object(affected,'FullCatalogPhase',Catalog), patch.object(affected.pg,'inspect',return_value={
                'scope':{'engine':{'id':'engine'},'server_start':'start','databases':['postgres'],'database_oids':{'postgres':1}},
                'active_sessions':0,'active_transactions':0}):
            probe=affected.CatalogScopedProbe(Catalog(),'token')
            with self.assertRaises(affected.AffectedError):
                probe({'engine':{'id':'engine'},'server_start':'start','databases':['postgres'],'database_oids':{'postgres':1}})

    def test_catalog_rejects_denied_or_changed_database(self):
        phase=type('Phase',(),{'_guard':lambda self:None})()
        row={'server_start':'start','can_read':True,'databases':[{'name':'postgres','oid':1,'connect':True,'allowconn':True,
            'template':False,'sessions':0,'prepared':0,'encoding':'UTF8','collate':'C','ctype':'C','locale_provider':'c',
            'icu_locale':None,'connection_limit':-1,'owner':'postgres','acl':None}]}
        changed=copy.deepcopy(row); changed['databases'].append(dict(row['databases'][0],name='other',oid=2))
        denied=copy.deepcopy(row); denied['databases'][0]['connect']=False
        with patch.object(affected.metadata,'kube_engine_snapshot',return_value=lambda:{'id':'engine'}), \
             patch.object(affected.metadata,'linux_psql',side_effect=[json.dumps(row),json.dumps(changed)]):
            with self.assertRaises(ValueError): affected._full_catalog(phase,{'id':'engine'},'start')
        with patch.object(affected.metadata,'kube_engine_snapshot',return_value=lambda:{'id':'engine'}), \
             patch.object(affected.metadata,'linux_psql',return_value=json.dumps(denied)):
            with self.assertRaises(ValueError): affected._full_catalog(phase,{'id':'engine'},'start')

    def test_fenced_catalog_receipt_and_drift(self):
        base=fixture.PhaseTest('test_exact_terminal_receipt_and_no_replay'); base.setUp(); self.addCleanup(base.doCleanups)
        j=journal.Journal(Path(base.temp.name)/'catalog',{'resource_context':copy.deepcopy(base.context),'namespace_uid':'ns'})
        self.addCleanup(j.close)
        class Client(fixture.Client): pass
        client=Client(base.lock); client.journal=j; base.ledger.client=client
        observed={'database_observations':{'workloads':{'user':[{'status':'observed','result':{'metadata':{
            'engine':{'namespace_uid':'ns'},'session':{'server_start':'start'}}}}]}}}
        class Inventory:
            def __init__(obj): obj.fence=base.fence; obj.lock=base.lock; obj.namespace_uid='ns'
            def verify_current(obj,token): return copy.deepcopy(observed)
            def verify_receipt(obj,token): return copy.deepcopy(observed)
        with patch.object(affected.frontend_runtime_executor,'KubectlTransport',Client), \
             patch.object(affected.inventory,'InventoryPhase',Inventory), \
             patch.object(affected,'_full_catalog',return_value={'databases':{'postgres':12},'all_databases':{'postgres':12},'excluded':{},'properties':{}}):
            phase=affected.FullCatalogPhase(j,base.fence,client,base.lock,'ns',Inventory())
            receipt=phase.run('token')
            self.assertEqual(phase.verify_current('token'),receipt)
            base.ledger.row['full_cohort_db_catalog']['nonce']='changed'
            with self.assertRaises(affected.AffectedError): phase.verify_receipt('token')


if __name__=='__main__': unittest.main()
