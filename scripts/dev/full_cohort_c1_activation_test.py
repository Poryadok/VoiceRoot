import os
import copy
import fcntl
import hashlib
import json
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest import mock

import full_cohort_c1_activation as activation
import media_backup_phase_test as fixture
import media_operation_journal as journals


@unittest.skipUnless(os.name=='posix','Linux private directory fixture')
class ProfileDirectoryTest(unittest.TestCase):
    def setUp(self):
        self.root=Path('/home/pmd/voice-dev-peer-issuer/preflight'); self.suffix='a'*32
        self.root.mkdir(parents=True,mode=0o700,exist_ok=True); (self.root/self.suffix).mkdir(mode=0o700)
        self.addCleanup(self.cleanup_dirs)
        self.supervisor=activation.supervisor_module.Supervisor.__new__(activation.supervisor_module.Supervisor)
        self.supervisor.context={'operation_id':'media-c1-'+self.suffix,'project':'voice','resource':'DEV'}; self.supervisor._guard=mock.Mock()

    def cleanup_dirs(self):
        parent=self.root/self.suffix
        for name in ('space','role'):
            path=parent/name
            if path.is_symlink():path.unlink()
            elif path.exists():path.rmdir()
        parent.rmdir()

    def test_two_fixed_directories_are_exclusive(self):
        activation._profile_directory(self.supervisor,'space'); activation._profile_directory(self.supervisor,'role')
        self.assertEqual((self.root/self.suffix/'space').stat().st_mode & 0o777,0o700)
        with self.assertRaises(activation.ActivationError):activation._profile_directory(self.supervisor,'space')
        with self.assertRaises(activation.ActivationError):activation._profile_directory(self.supervisor,'creator')
        self.supervisor.context['resource']='STAGING'
        with self.assertRaises(activation.ActivationError):activation._profile_directory(self.supervisor,'role')

    def test_existing_symlink_is_never_adopted(self):
        (self.root/self.suffix/'role').symlink_to('/tmp',target_is_directory=True)
        with self.assertRaises(activation.ActivationError):activation._profile_directory(self.supervisor,'role')
        self.assertTrue((self.root/self.suffix/'role').is_symlink())

    def test_lost_guard_retains_reserved_directory_for_reconciliation(self):
        self.supervisor._guard.side_effect=[None,RuntimeError('guard lost')]
        with self.assertRaises(activation.ActivationError):activation._profile_directory(self.supervisor,'space')
        self.assertTrue((self.root/self.suffix/'space').is_dir())


class RealtimePreparationTest(unittest.TestCase):
    def test_actual_journal_preflight_and_historical_reader(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp); context={'project':'voice','resource':'DEV','actor':'scheduled_devops',
                'claim':'11111111-1111-4111-8111-111111111111','window':'22222222-2222-4222-8222-222222222222',
                'revision':4,'generation':3,'operation_id':'media-c1-'+'a'*32,'commit':'b'*40,
                'images':{'voice-realtime/realtime':'ghcr.io/x@sha256:'+'c'*64}}
            journal=journals.Journal(root/'journal',{'resource_context':context,'namespace_uid':'ns'}); self.addCleanup(journal.close)
            backing=fixture.Ledger(context); client=activation.frontend_runtime_executor.KubectlTransport(journal)
            ledger=activation.resource_fencing.KubeLedger(client); ledger.read=backing.read; ledger.write=backing.write
            fence=activation.resource_fencing.Fence(context,lambda:{},ledger); lock={'metadata':{'uid':'lock'}}
            node={'name':'node-1','uid':'node-uid','hostname':'host-1','architecture':'amd64'}
            pod={'metadata':{'name':'voice-postgres-0','namespace':'voice-dev','uid':'pod'},'spec':{'nodeName':'node-1'}}
            kube_node={'metadata':{'name':'node-1','uid':'node-uid','labels':{'kubernetes.io/hostname':'host-1',
                'kubernetes.io/arch':'amd64','kubernetes.io/os':'linux'}},'spec':{},'status':{'nodeInfo':{'architecture':'amd64',
                'operatingSystem':'linux'},'conditions':[{'type':'Ready','status':'True'}]}}
            client.call=mock.Mock(side_effect=lambda verb,kind,*a,**k:copy.deepcopy(pod if kind=='pod' else {'items':[kube_node]}))
            catalog={'all_databases':{'auth_db':1},'engine':{'pod_uid':'pod'},'scope':'all-databases-connectable-backup'}
            absent={'scope':'realtime-create-only-baseline'}; s=activation.supervisor_module.Supervisor.__new__(activation.supervisor_module.Supervisor)
            s.context=context; s.lock=lock; s.namespace_uid='ns'; s.fence=fence; s._guard=mock.Mock(); s._creator_guard=mock.Mock()
            def dependency(name,cls):
                obj=cls.__new__(cls); j=journals.Journal(root/name,{'resource_context':context,'namespace_uid':'ns'}); self.addCleanup(j.close)
                c=activation.frontend_runtime_executor.KubectlTransport(j); l=activation.resource_fencing.KubeLedger(c); l.read=backing.read; l.write=backing.write
                obj.journal,obj.client,obj.fence=j,c,activation.resource_fencing.Fence(context,lambda:{},l)
                obj.lock,obj.namespace_uid=lock,'ns'; return obj
            s.package=dependency('package',activation.full_cohort_package_phase.PackagePhase)
            s.inventory=dependency('inventory',activation.full_cohort_inventory_phase.InventoryPhase)
            s.catalog=dependency('catalog',activation.full_cohort_affected_plan.FullCatalogPhase)
            s.cut=dependency('cut',activation.writer_cut.WriterCut); s.cut.started=False; s.cut.drained=False
            s.package.verify_current=mock.Mock(return_value={'nodes':[node]})
            s.catalog.verify_current=mock.Mock(return_value=catalog); s.catalog.verify_receipt=mock.Mock(return_value=catalog)
            s.realtime_absent=dependency('absent',activation.realtime_absent_phase.RealtimeAbsentPhase)
            s.realtime_absent.inventory=s.inventory; s.realtime_absent.catalog=s.catalog
            s.realtime_absent.verify_current=mock.Mock(return_value=absent); s.realtime_absent.verify_receipt=mock.Mock(return_value=absent)
            prefix='src/backend/migrations/realtime_db/'; files={}
            for name in ('000001_admission.up.sql','000001_admission.down.sql'):
                path=root/prefix/name; path.parent.mkdir(parents=True,exist_ok=True); path.write_bytes(name.encode()); files[prefix+name]=hashlib.sha256(name.encode()).hexdigest()
            files['src/backend/realtime/internal/lifecycle/store.go']='1e34ba05a4831031409bb90e37b541b8adfbe233cb307618cbb6ba471fb5fdc9'
            with mock.patch.object(activation.canonical,'ROOT',root), mock.patch.object(activation.media_operation_guard,'verify'), \
                 mock.patch.object(activation.publish_candidate,'source_snapshot',return_value={'source_sha':context['commit'],'files':files}), \
                 mock.patch.object(activation,'_qualified_migrate',return_value=activation.migration.PLATFORMS['amd64']):
                phase=activation.RealtimePreparation(journal,fence,client,lock,'ns',s); receipt=phase.run('token')
                self.assertEqual(receipt['node'],node); s.cut.started=True; s.cut.drained=True
                s.realtime_absent.verify_current.side_effect=AssertionError('historical reader must not inspect old pods')
                self.assertEqual(activation.verify_realtime_receipt(phase,'token'),receipt)
                original_guard=phase._guard; calls=[]
                def late_change():
                    answer=original_guard(); calls.append(True)
                    if len(calls)==2:s.package.journal.state['steps']['foreign']={'status':'changed'}
                    return answer
                phase._guard=late_change
                with self.assertRaises(activation.ActivationError):activation.verify_realtime_receipt(phase,'token')
                self.assertEqual(len(calls),2); s.package.journal.state['steps'].pop('foreign'); phase._guard=original_guard
                journal.state['steps'][activation.REALTIME_STEP]['receipt']['node']['uid']='foreign'
                with self.assertRaises(activation.ActivationError):activation.verify_realtime_receipt(phase,'token')

    def test_oci_index_and_child_are_digest_bound(self):
        child=json.dumps({'schemaVersion':2},separators=(',',':')).encode(); child_digest='sha256:'+hashlib.sha256(child).hexdigest()
        index=json.dumps({'schemaVersion':2,'manifests':[{'digest':child_digest,'size':len(child),'platform':{'os':'linux','architecture':'amd64'}}]},separators=(',',':')).encode()
        index_digest='sha256:'+hashlib.sha256(index).hexdigest()
        with mock.patch.object(activation.migration,'MIGRATE_IMAGE','migrate/migrate@'+index_digest), \
             mock.patch.object(activation.migration,'PLATFORMS',{'amd64':child_digest}), \
             mock.patch.object(activation,'_migrate_manifest',side_effect=lambda digest:index if digest==index_digest else child):
            self.assertEqual(activation._qualified_migrate('amd64'),child_digest)
            with mock.patch.object(activation,'_migrate_manifest',return_value=b'{}'):
                with self.assertRaises(Exception):activation._qualified_migrate('amd64')
        ambiguous=json.loads(index); ambiguous['manifests'].append(copy.deepcopy(ambiguous['manifests'][0]))
        raw=json.dumps(ambiguous,separators=(',',':')).encode(); digest='sha256:'+hashlib.sha256(raw).hexdigest()
        with mock.patch.object(activation.migration,'MIGRATE_IMAGE','migrate/migrate@'+digest), \
             mock.patch.object(activation.migration,'PLATFORMS',{'amd64':child_digest}), \
             mock.patch.object(activation,'_migrate_manifest',return_value=raw):
            with self.assertRaises(Exception):activation._qualified_migrate('amd64')

    @unittest.skipUnless(hasattr(os,'memfd_create'),'requires Linux sealed memfd')
    def test_generated_credential_is_private_sealed_and_not_printable(self):
        credential=activation._RealtimeCredential(); self.addCleanup(credential.close)
        value=credential.read(); self.assertGreaterEqual(len(value),32); self.assertNotIn(value.decode(),repr(credential))
        with self.assertRaises(OSError):os.write(credential.fd,b'changed')
        self.assertEqual(fcntl.fcntl(credential.fd,fcntl.F_GET_SEALS)&activation._SEALS,activation._SEALS)

    def test_existing_realtime_role_stops_before_credential_or_secret(self):
        import full_cohort_c1_driver as driver_module
        s=activation.supervisor_module.Supervisor.__new__(activation.supervisor_module.Supervisor)
        s.context={'project':'voice','resource':'DEV','operation_id':'media-c1-'+'a'*32}
        s.cut=SimpleNamespace(verify_drained=mock.Mock()); s._guard=mock.Mock(); s.physical=object(); s._open=mock.Mock()
        driver=driver_module.C1Driver.__new__(driver_module.C1Driver); driver.supervisor=s
        prep=activation.RealtimePreparation.__new__(activation.RealtimePreparation); prep.supervisor=s
        node={'name':'node','uid':'uid','hostname':'host','architecture':'amd64'}; engine={'pod_uid':'p'}
        pg=SimpleNamespace(snapshot=mock.Mock(return_value=engine),catalog=mock.Mock(return_value={'role':{'oid':1},'database':None}))
        with mock.patch.object(activation,'verify_realtime_receipt',return_value={'node':node}), \
             mock.patch.object(activation,'_recheck_realtime_node'), \
             mock.patch.object(activation.physical,'verify_receipt',return_value={'physical':{'node':{'name':'node','uid':'uid'},'engine':engine}}), \
             mock.patch.object(activation.realtime_db_provision,'KubePostgres',return_value=pg), \
             mock.patch.object(activation,'_RealtimeCredential') as credential:
            with self.assertRaises(activation.ActivationError):activation._realtime_database(driver,prep,'token')
            credential.assert_not_called(); s._open.assert_not_called()


if __name__=='__main__': unittest.main()
