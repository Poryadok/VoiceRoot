import unittest
import copy
import json
from unittest import mock
from pathlib import Path
import tempfile

import space_creator_runtime_profile_config as config
import frontend_runtime_executor as transport
import full_cohort_inventory_phase as inventory
import full_cohort_package_phase as package
import media_backup_phase_test as fixture
import media_operation_guard as guard
import media_operation_journal as journals
import resource_fencing


class ConfigTest(unittest.TestCase):
    def test_explicit_value_overrides_envfrom_and_values_stay_private(self):
        snap={'env':[{'name':'USER_GRPC_ADDR','value':'voice-user:9090'}],
              'envFrom':[{'configMapRef':{'name':'voice-app-config'}}]}
        def forbidden(*args): raise AssertionError('overridden envFrom must not be read')
        value,refs=config.resolve(snap,'USER_GRPC_ADDR',forbidden)
        self.assertEqual(config.category('space','USER_GRPC_ADDR',value),'expected')
        self.assertEqual(refs,[{'kind':'literal'}])

    def test_last_envfrom_wins_and_unknown_sources_hold(self):
        snap={'env':[],'envFrom':[{'configMapRef':{'name':'voice-app-config'}},
                                  {'secretRef':{'name':'voice-app-secrets'}}]}
        def reader(kind,name,key): return {'uid':kind,'resource_version':'1','value':kind}
        value,refs=config.resolve(snap,'USER_GRPC_ADDR',reader)
        self.assertEqual(value,'secret'); self.assertEqual(len(refs),2)
        snap['envFrom'].append({'secretRef':{'name':'foreign'}})
        with self.assertRaises(ValueError): config.resolve(snap,'USER_GRPC_ADDR',reader)

    def test_space_issuer_only_reports_fixed_category(self):
        self.assertEqual(config.category('auth','S2S_JWKS_URLS_JSON','{"space":"https://voice-space:8443/.well-known/jwks.json","private":"value"}'),'expected')
        self.assertEqual(config.category('auth','S2S_JWKS_URLS_JSON','{"space":"https://other"}'),'different')
        self.assertEqual(config.category('auth','AUTH_GRPC_TLS_CERT_FILE','/etc/voice/media/server/tls.crt'),'expected')
        self.assertEqual(config.category('auth','AUTH_GRPC_TLS_CERT_FILE','/elsewhere'),'different')

    def test_unknown_reference_and_value_expansion_are_rejected(self):
        def read(*_): raise AssertionError('unknown key must not be fetched')
        for entry in ({'name':'USER_GRPC_ADDR','value':'$(UNTRUSTED)'},
                      {'name':'USER_GRPC_ADDR','valueFrom':{'secretKeyRef':{'name':'voice-app-secrets','key':'OTHER_PRIVATE_KEY'}}}):
            with self.assertRaises(ValueError): config.resolve({'env':[entry],'envFrom':[]},'USER_GRPC_ADDR',read)

    def test_phase_rejects_drift_and_closed_journal(self):
        with tempfile.TemporaryDirectory() as root:
            context={'project':'voice','resource':'DEV','actor':'scheduled_devops',
                     'claim':'11111111-1111-4111-8111-111111111111','window':'22222222-2222-4222-8222-222222222222',
                     'revision':4,'generation':3,'operation_id':'media-c1-'+'a'*32,'commit':'b'*40,
                     'images':{'voice-space/space':'ghcr.io/x@sha256:'+'b'*64}}
            j=journals.Journal(Path(root)/'journal',{'resource_context':context,'namespace_uid':'ns'}); self.addCleanup(j.close)
            backing=fixture.Ledger(context); client=transport.KubectlTransport(j); ledger=resource_fencing.KubeLedger(client)
            ledger.read=backing.read; ledger.write=backing.write; fence=resource_fencing.Fence(context,lambda:{},ledger)
            pkg=package.PackagePhase.__new__(package.PackagePhase); inv=inventory.InventoryPhase.__new__(inventory.InventoryPhase)
            lock={'metadata':{'uid':'lock'}}
            for name,obj in (('pkg',pkg),('inv',inv)):
                other=journals.Journal(Path(root)/name,{'resource_context':context,'namespace_uid':'ns'}); self.addCleanup(other.close)
                obj.journal=other; obj.client=transport.KubectlTransport(other); other_ledger=resource_fencing.KubeLedger(obj.client)
                other_ledger.read=backing.read; other_ledger.write=backing.write
                obj.fence=resource_fencing.Fence(context,lambda:{},other_ledger); obj.lock=lock; obj.namespace_uid='ns'
            inv.package=pkg; inv.verify_current=mock.Mock(return_value={}); pkg.verify_current=mock.Mock(return_value={})
            phase=config.CreatorConfigPhase(j,fence,client,lock,'ns',pkg,inv)
            phase._observe=mock.Mock(return_value={'scope':'synthetic','operation_id':context['operation_id'],'source_sha':context['commit'],'namespace_uid':'ns'})
            with mock.patch.object(guard,'verify'):
                receipt=phase.run('token'); self.assertEqual(config.verify_completed(phase,'token'),receipt)
                phase._observe.side_effect=RuntimeError('pods already drained')
                self.assertEqual(config.verify_receipt(phase,'token'),receipt)
                phase._observe.side_effect=None
                original_ledger=phase._ledger; original_record=copy.deepcopy(phase.record); seen=[]
                def late_mutation():
                    answer=original_ledger(); seen.append(1)
                    if len(seen)==2:phase.record=None
                    return answer
                phase._ledger=late_mutation
                with self.assertRaises(config.metadata.GuardLost):config.verify_receipt(phase,'token')
                phase._ledger=original_ledger; phase.record=original_record
                phase._observe.return_value={'scope':'drift'}
                with self.assertRaises(config.ConfigError): config.verify_completed(phase,'token')
                inv.journal.close()
                with self.assertRaises(config.metadata.GuardLost): phase._guard()

    def test_observer_rejects_runtime_drift_and_redacts_literals(self):
        phase=config.CreatorConfigPhase.__new__(config.CreatorConfigPhase); phase.namespace_uid='ns'
        phase.fence=mock.Mock(context={'operation_id':'op','commit':'sha'}); phase._package=mock.Mock(); phase._ledger=mock.Mock()
        phase._guard=mock.Mock(); phase._inventory=mock.Mock(return_value={
            'pods':{'space':['voice-space-abc']},'projection':{'space':{'pods':[{'name':'voice-space-abc','uid':'pod',
            'containers':[{'name':'space','container_id':'cid','restart_count':0,'image_id':'digest'}]}]}}})
        before={'namespace_uid':'ns','pod_uid':'pod','pod_rv':'2','container_id':'cid','restart_count':0,
                'env':[{'name':'USER_GRPC_ADDR','value':'private-sentinel'}],'envFrom':[]}
        snap=mock.Mock(return_value=before); phase._runtime=mock.Mock(return_value='private-sentinel'); phase._key=mock.Mock()
        with mock.patch.dict(config.FIELDS,{'space':('USER_GRPC_ADDR',)},clear=True), mock.patch.object(config.metadata,'kube_workload_snapshot',return_value=snap):
            result=phase._observe('token')
            self.assertNotIn('private-sentinel',json.dumps(result))
            self.assertEqual(result['workloads']['space'][0]['fields']['USER_GRPC_ADDR']['state'],'different')
            phase._runtime.return_value='different'
            with self.assertRaises(ValueError): phase._observe('token')
