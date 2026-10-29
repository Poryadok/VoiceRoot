import copy
import hashlib
from pathlib import Path
import tempfile
import unittest
from unittest import mock

import frontend_runtime_executor as transport
import full_cohort_inventory_phase as inventory
import full_cohort_package_phase as package
import media_backup_phase_test as fixture
import media_operation_guard as guard
import media_operation_journal as journals
import resource_fencing
import space_creator_runtime_profile_auth as auth
from space_creator_runtime_profile_trust_test import listing


class AuthTest(unittest.TestCase):
    def test_options_and_mount_reject_shadow_and_overrides(self):
        value='-XX:MaxRAMPercentage=75.0 -XX:InitialRAMPercentage=25.0 -Djavax.net.ssl.trustStore='+auth.TRUST+' -Djavax.net.ssl.trustStorePassword=changeit -Djavax.net.ssl.trustStoreType=PKCS12'
        auth._options(value)
        with self.assertRaises(ValueError): auth._options(value+' -Djavax.net.ssl.trustStore=/evil')
        pod={'spec':{'containers':[{'name':'auth','volumeMounts':[{'name':'media-trust','mountPath':'/etc/voice/media/trust','readOnly':True}]}],
                     'volumes':[{'name':'media-trust','secret':{'secretName':'voice-media-jvm-trust','defaultMode':288,'items':[{'key':'truststore.p12','path':'truststore.p12'}]}}]}}
        auth._mount(pod)
        for mount in ({'name':'evil','mountPath':'/opt/java'}, {'name':'evil','mountPath':'/opt/java/openjdk/lib/server'},
                      {'name':'evil','mountPath':'//opt/java'}, {'name':'evil','mountPath':'/etc/voice/media/trust/x'}):
            bad=copy.deepcopy(pod); bad['spec']['containers'][0]['volumeMounts'].append(mount)
            with self.assertRaises(ValueError): auth._mount(bad)

    def test_real_journal_fence_detects_drift_and_no_replay(self):
        with tempfile.TemporaryDirectory() as root:
            context={'project':'voice','resource':'DEV','actor':'scheduled_devops',
                     'claim':'11111111-1111-4111-8111-111111111111','window':'22222222-2222-4222-8222-222222222222',
                     'revision':4,'generation':3,'operation_id':'media-c1-'+'a'*32,'commit':'b'*40,
                     'images':{'voice-space/space':'ghcr.io/x@sha256:'+'b'*64}}
            j=journals.Journal(Path(root)/'journal',{'resource_context':context,'namespace_uid':'ns'})
            self.addCleanup(j.close)
            client=transport.KubectlTransport(j); backing=fixture.Ledger(context)
            ledger=resource_fencing.KubeLedger(client); ledger.read=backing.read; ledger.write=backing.write
            fence=resource_fencing.Fence(context,lambda:{},ledger); lock={'metadata':{'uid':'lock'}}
            pkg=package.PackagePhase.__new__(package.PackagePhase); inv=inventory.InventoryPhase.__new__(inventory.InventoryPhase)
            for name,obj in (('package',pkg),('inventory',inv)):
                other=journals.Journal(Path(root)/name,{'resource_context':context,'namespace_uid':'ns'})
                self.addCleanup(other.close); other_client=transport.KubectlTransport(other)
                other_ledger=resource_fencing.KubeLedger(other_client); other_ledger.read=backing.read; other_ledger.write=backing.write
                obj.fence=resource_fencing.Fence(context,lambda:{},other_ledger)
                obj.client=other_client; obj.journal=other; obj.lock=lock; obj.namespace_uid='ns'
            inv.package=pkg; inv.verify_current=mock.Mock(return_value={})
            phase=auth.AuthTrustPhase(j,fence,client,lock,'ns',pkg,inv)
            observation={'scope':'synthetic'}; phase._observe=mock.Mock(return_value=observation)
            with mock.patch.object(guard,'verify'):
                self.assertEqual(phase.run('token'),observation)
                self.assertEqual(auth.verify_completed(phase,'token'),observation)
                phase._observe.return_value={'scope':'drift'}
                with self.assertRaises(auth.AuthTrustError): auth.verify_completed(phase,'token')
                with self.assertRaises(auth.AuthTrustError): phase.run('token')
                inv.journal.close()
                with self.assertRaises(auth.metadata.GuardLost): phase._guard()

    def test_observation_binds_mounted_bytes_and_pod_restarts(self):
        phase=auth.AuthTrustPhase.__new__(auth.AuthTrustPhase)
        phase.namespace_uid='ns'; phase.fence=mock.Mock(context={'operation_id':'op','commit':'sha'})
        phase._guard=mock.Mock(); phase._ledger=mock.Mock(); phase._package=mock.Mock()
        observation={'pods':{'auth':['voice-auth-abc']},'projection':{'auth':{'pods':[{'name':'voice-auth-abc','uid':'pod',
             'containers':[{'name':'auth','container_id':'cid','restart_count':0,'image_id':'digest'}]}]}}}
        phase._inventory=mock.Mock(return_value=observation)
        identity={'namespace_uid':'ns','pod_uid':'pod','pod_rv':'2','container_id':'cid','restart_count':0}
        pod={'metadata':{'uid':'pod','resourceVersion':'2'},'spec':{'containers':[{'name':'auth','volumeMounts':[
            {'name':'media-trust','mountPath':'/etc/voice/media/trust','readOnly':True}]}],
            'volumes':[{'name':'media-trust','secret':{'secretName':'voice-media-jvm-trust','defaultMode':288,
              'items':[{'key':'truststore.p12','path':'truststore.p12'}]}}]}}
        phase.client=mock.Mock(); phase.client.call.return_value=pod
        values={'voice-media-jvm-trust':({'uid':'trust','resourceVersion':'1'},b'store'),
                'voice-media-ca':({'uid':'ca','resourceVersion':'1'},b'ca')}
        phase._secret=mock.Mock(side_effect=lambda name,key:copy.deepcopy(values[name]))
        media=hashlib.sha256(b'der').hexdigest()
        def row(fingerprint): return ':'.join(fingerprint[i:i+2].upper() for i in range(0,64,2))
        def exec_value(_, *args):
            if args[0]=='sha256sum': return (hashlib.sha256(b'store').hexdigest()+'  '+auth.TRUST+'\n').encode()
            if args[0]=='bash': return b'M'
            root_line=listing(('trustedCertEntry','AA'))
            if auth.JDK in args: return root_line
            return b'Keystore type: PKCS12\nYour keystore contains 2 entries\nEntry type: trustedCertEntry\nSHA256: '+row('aa'*32).encode()+b'\nEntry type: trustedCertEntry\nSHA256: '+row(media).encode()+b'\n'
        phase._exec=mock.Mock(side_effect=exec_value)
        reader=mock.Mock(return_value='-XX:MaxRAMPercentage=75.0 -XX:InitialRAMPercentage=25.0 -Djavax.net.ssl.trustStore='+auth.TRUST+' -Djavax.net.ssl.trustStorePassword=changeit -Djavax.net.ssl.trustStoreType=PKCS12')
        snapshot=mock.Mock(return_value=identity)
        with mock.patch.object(auth.metadata,'kube_workload_snapshot',return_value=snapshot), mock.patch.object(auth.metadata,'kube_runtime_reader',return_value=reader), mock.patch.object(auth.material,'_cert_der',return_value=b'der'), mock.patch.object(auth.material,'_openssl',return_value=b'CA:TRUE'):
            self.assertEqual(phase._observe('token')['pods'][0]['image_id'],'digest')
            snapshot.side_effect=[identity,identity|{'restart_count':1}]
            with self.assertRaises(ValueError): phase._observe('token')
