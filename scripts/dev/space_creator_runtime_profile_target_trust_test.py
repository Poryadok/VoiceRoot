import copy
import hashlib
import json
from pathlib import Path
import sys
import tempfile
from types import SimpleNamespace
import unittest
from unittest import mock

import frontend_runtime_executor as transport
import full_cohort_inventory_phase as inventory
import full_cohort_package_phase as package
import media_backup_phase_test as fixture
import media_operation_guard as guard
import media_operation_journal as journals
import resource_fencing
import space_creator_runtime_profile_jdk as jdk
import space_creator_runtime_profile_create as creator
import space_creator_runtime_profile_target_trust as target


class TargetTrustTest(unittest.TestCase):
    def test_timed_out_input_cleans_exact_owned_container(self):
        phase=target.TargetTrustPhase.__new__(target.TargetTrustPhase); ca_sha=hashlib.sha256(b'der').hexdigest(); cid='e'*64
        image='ghcr.io/poryadok/voiceroot/auth@sha256:'+'a'*64; image_id='sha256:'+'f'*64
        roots={'image':image,'config_digest':image_id,'root_fingerprints':['aa'*32]}
        source={'ca':{'uid':'ca','resourceVersion':'1','der_sha256':ca_sha}}
        phase._dependencies=mock.Mock(return_value=(roots,source)); phase._guard=mock.Mock(); phase._state=mock.Mock()
        phase._secret=mock.Mock(side_effect=lambda name,key:({'uid':'ca','resourceVersion':'1'},b'ca') if name=='voice-media-ca' else ({'uid':'trust','resourceVersion':'1'},b'private'))
        commands=[]
        def docker(args,limit=1048576):
            commands.append(args)
            if args[:2]==['image','inspect']:return json.dumps([{'Id':image_id,'Os':'linux','Architecture':'amd64','RepoDigests':[image],'Config':{'Volumes':None}}]).encode()
            if args[0]=='create':return cid.encode()
            if args[0]=='inspect':
                name=next(a.split('=',1)[1] for c in commands if c[0]=='create' for a in c if a.startswith('voice.test.owner='))
                return json.dumps([{'Id':cid,'Image':image_id,'Config':{'Labels':{'voice.test.owner':name},'Entrypoint':['/bin/sh'],
                    'Cmd':['-ec',target.SCRIPT],'User':'65532:65532','OpenStdin':True},'HostConfig':{'NetworkMode':'none','ReadonlyRootfs':True,
                    'CapDrop':['ALL'],'SecurityOpt':['no-new-privileges'],'Tmpfs':{'/scratch':target.TMPFS}},'Mounts':[]}]).encode()
            if args[0] in ('rm','ps'):return b''
            raise AssertionError(args)
        phase._docker=mock.Mock(side_effect=docker)
        with mock.patch.object(target.material,'_cert_der',return_value=b'der'), mock.patch.object(target,'_input',side_effect=TimeoutError()):
            with self.assertRaises(TimeoutError):phase._capture('token')
        self.assertTrue(any(args[0]=='rm' and cid in args for args in commands))
        self.assertEqual(phase._state.call_args.args[0]['status'],'removed')
        commands.clear()
        with mock.patch.object(target.material,'_cert_der',return_value=b'der'), mock.patch.object(target,'_input',side_effect=target.metadata.GuardLost('lost')):
            with self.assertRaises(target.metadata.GuardLost):phase._capture('token')
        self.assertFalse(any(args[0]=='rm' for args in commands))

    def test_preflight_ledger_read_failure_is_guard_lost(self):
        phase=target.TargetTrustPhase.__new__(target.TargetTrustPhase); row={'status':'active'}; rev=('uid','rv')
        phase._guard=mock.Mock(); phase._local=mock.Mock(); phase._ledger=mock.Mock(return_value=(row,rev))
        phase.jdk=SimpleNamespace(_ledger=mock.Mock(return_value=(row,rev)),_local=mock.Mock())
        jdk_receipt={'roots':'safe'}; phase.jdk.record={'status':'done','receipt_sha256':target.writer_cut.digest(jdk_receipt)}
        phase.jdk.journal=SimpleNamespace(state={'steps':{jdk.STEP:phase.jdk.record|{'receipt':jdk_receipt}}})
        ledger=SimpleNamespace(read=mock.Mock(side_effect=[OSError('private'),(row,rev)]))
        phase.preflight=SimpleNamespace(fence=SimpleNamespace(ledger=ledger),_local=mock.Mock())
        pre_receipt={'ca':'safe'}; phase.preflight.record={'status':'done','receipt_sha256':target.writer_cut.digest(pre_receipt)}
        phase.preflight.journal=SimpleNamespace(state={'steps':{'pre-step':phase.preflight.record|{'receipt':pre_receipt}}})
        row[jdk.LEDGER]=phase.jdk.record; row['pre-ledger']=phase.preflight.record
        phase._target_refs=(phase.jdk,phase.preflight)
        preflight=SimpleNamespace(STEP='pre-step',LEDGER='pre-ledger',verify_current=mock.Mock(return_value=pre_receipt))
        with mock.patch.object(target,'_preflight_layout',return_value=(preflight,'pre-step','pre-ledger')), mock.patch.object(target.jdk,'verify_completed',return_value={'roots':'safe'}):
            with self.assertRaises(target.metadata.GuardLost): phase._dependencies('token')
            self.assertEqual(phase._dependencies('token'),({'roots':'safe'},{'ca':'safe'}))

    def test_fixed9_layout_accepts_only_creator_profile(self):
        import dev_peer_issuer_preflight as peer
        phase=peer.PeerIssuerPreflightPhase.__new__(peer.PeerIssuerPreflightPhase)
        phase.profile='creator'; phase.step='creator-step'; phase.ledger='creator-ledger'
        self.assertEqual(target._preflight_layout(phase),(peer,'creator-step','creator-ledger'))
        phase.profile='role'
        with self.assertRaises(ValueError): target._preflight_layout(phase)

    def test_fixed9_creator_dependencies_bind_exact_ledger_and_receipt(self):
        import dev_peer_issuer_preflight as peer
        phase=target.TargetTrustPhase.__new__(target.TargetTrustPhase); row={'status':'active'}; rev=('uid','rv')
        phase._guard=mock.Mock(); phase._local=mock.Mock(); phase._ledger=mock.Mock(return_value=(row,rev))
        phase.jdk=SimpleNamespace(_ledger=mock.Mock(return_value=(row,rev)),_local=mock.Mock())
        roots={'roots':'safe'}; phase.jdk.record={'status':'done','receipt_sha256':target.writer_cut.digest(roots)}
        phase.jdk.journal=SimpleNamespace(state={'steps':{jdk.STEP:phase.jdk.record|{'receipt':roots}}})
        pre=peer.PeerIssuerPreflightPhase.__new__(peer.PeerIssuerPreflightPhase); pre.profile='creator'
        pre.step='fixed9-creator'; pre.ledger='fixed9-ledger'; pre._local=mock.Mock()
        source={'profile':'creator','ca':'safe'}; pre.record={'status':'done','receipt_sha256':target.writer_cut.digest(source)}
        pre.journal=SimpleNamespace(state={'steps':{pre.step:pre.record|{'receipt':source}}})
        pre.fence=SimpleNamespace(ledger=SimpleNamespace(read=mock.Mock(return_value=(row,rev))))
        phase.preflight=pre; phase._target_refs=(phase.jdk,pre); row[jdk.LEDGER]=phase.jdk.record; row[pre.ledger]=pre.record
        with mock.patch.object(peer,'verify_current',return_value=source), mock.patch.object(jdk,'verify_completed',return_value=roots):
            self.assertEqual(phase._dependencies('token'),(roots,source))
            row[pre.ledger]={'status':'tampered'}
            with self.assertRaises(target.metadata.GuardLost):phase._dependencies('token')

    def test_fixed9_successor_keeps_exact_original_ca_and_jdk(self):
        import space_creator_runtime_profile_fixed9_successor as successor
        phase=target.TargetTrustPhase.__new__(target.TargetTrustPhase); phase._guard=mock.Mock(); phase._locals=mock.Mock(); phase.jdk=object()
        roots={'roots':'safe'}; original={'ca':{'uid':'ca','resourceVersion':'1','der_sha256':hashlib.sha256(b'der').hexdigest()}}
        receipt={'jdk_receipt_sha256':target.writer_cut.digest(roots),'preflight_receipt_sha256':target.writer_cut.digest(original),
                 'trust_secret':{'uid':'trust','resourceVersion':'1'},'media_ca':original['ca']}
        marker={'status':'done','receipt_sha256':target.writer_cut.digest(receipt)}; phase.record=copy.deepcopy(marker)
        phase.journal=SimpleNamespace(state={'steps':{target.STEP:marker|{'receipt':receipt}}})
        row={target.LEDGER:marker}; phase._ledger=mock.Mock(return_value=(row,('uid','rv')))
        phase._secret=mock.Mock(side_effect=lambda name,key:({'uid':'ca','resourceVersion':'1'},b'ca') if name=='voice-media-ca' else ({'uid':'trust','resourceVersion':'1'},b'store'))
        with mock.patch.object(jdk,'verify_completed',return_value=roots), mock.patch.object(successor,'verify',return_value=original), \
             mock.patch.object(target.material,'_cert_der',return_value=b'der'):
            self.assertEqual(target.verify_fixed9_successor(phase,'token',object(),object(),object()),receipt)
            phase._secret.side_effect=lambda name,key:({'uid':'ca','resourceVersion':'2'},b'ca') if name=='voice-media-ca' else ({'uid':'trust','resourceVersion':'1'},b'store')
            with self.assertRaises(target.TargetTrustError):target.verify_fixed9_successor(phase,'token',object(),object(),object())

    def test_phase_rejects_identity_drift_and_redacts_keystore(self):
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp); context={'project':'voice','resource':'DEV','actor':'scheduled_devops',
              'claim':'11111111-1111-4111-8111-111111111111','window':'22222222-2222-4222-8222-222222222222',
              'revision':4,'generation':3,'operation_id':'media-c1-'+'a'*32,'commit':'b'*40,
              'images':{'voice-auth/auth':'ghcr.io/poryadok/voiceroot/auth@sha256:'+'c'*64}}
            backing=fixture.Ledger(context); lock={'metadata':{'uid':'lock'}}
            def binding(name,obj):
                other=journals.Journal(root/name,{'resource_context':context,'namespace_uid':'ns'}); self.addCleanup(other.close)
                obj.journal=other; obj.client=transport.KubectlTransport(other); ledger=resource_fencing.KubeLedger(obj.client)
                ledger.read=backing.read; ledger.write=backing.write; obj.fence=resource_fencing.Fence(context,lambda:{},ledger)
                obj.lock=lock; obj.namespace_uid='ns'
            pkg=package.PackagePhase.__new__(package.PackagePhase); inv=inventory.InventoryPhase.__new__(inventory.InventoryPhase)
            binding('pkg',pkg); binding('inv',inv); inv.package=pkg; inv.verify_current=mock.Mock(return_value={})
            pkg.verify_current=mock.Mock(return_value={'images':context['images'],'platform_digests':{'voice-auth/auth':'sha256:'+'d'*64}})
            journal=journals.Journal(root/'target',{'resource_context':context,'namespace_uid':'ns'}); self.addCleanup(journal.close)
            client=transport.KubectlTransport(journal); ledger=resource_fencing.KubeLedger(client); ledger.read=backing.read; ledger.write=backing.write
            fence=resource_fencing.Fence(context,lambda:{},ledger)
            jp=jdk.TargetJdkPhase.__new__(jdk.TargetJdkPhase); jp.package=pkg; jp.inventory=inv; jp.fence=fence; jp.lock=lock; jp.namespace_uid='ns'; jp._local=mock.Mock()
            jdk_receipt={'scope':'synthetic'}; jp.record={'status':'done','receipt_sha256':target.writer_cut.digest(jdk_receipt)}
            jp.journal=SimpleNamespace(state={'steps':{jdk.STEP:jp.record|{'receipt':jdk_receipt}}})
            class Preflight: pass
            pp=Preflight(); pp.package=pkg; pp.fence=fence; pp.lock=lock; pp.namespace_uid='ns'; pp._local=mock.Mock()
            fake=SimpleNamespace(IssuerPreflightPhase=Preflight,STEP='pre-step',LEDGER='pre-ledger')
            with mock.patch.dict(sys.modules,{'space_p3_issuer_preflight':fake}), mock.patch.object(guard,'verify'):
                phase=target.TargetTrustPhase(journal,fence,client,lock,'ns',pkg,inv,jp,pp)
                ca_sha=hashlib.sha256(b'der').hexdigest(); image_id='sha256:'+'f'*64
                roots={'image':context['images']['voice-auth/auth'],'platform_digest':'sha256:'+'d'*64,
                       'config_digest':image_id,'root_fingerprints':['aa'*32]}
                source={'ca':{'uid':'ca','resourceVersion':'1','der_sha256':ca_sha}}
                jp.record={'status':'done','receipt_sha256':target.writer_cut.digest(roots)}
                jp.journal.state['steps'][jdk.STEP]=jp.record|{'receipt':roots}
                pp.record={'status':'done','receipt_sha256':target.writer_cut.digest(source)}
                pp.journal=SimpleNamespace(state={'steps':{'pre-step':pp.record|{'receipt':source}}})
                backing.row[jdk.LEDGER]=copy.deepcopy(jp.record); backing.row['pre-ledger']=copy.deepcopy(pp.record)
                phase._dependencies=mock.Mock(return_value=(roots,source)); values={
                    'voice-media-jvm-trust':({'uid':'trust','resourceVersion':'1'},b'PRIVATE-SENTINEL'),
                    'voice-media-ca':({'uid':'ca','resourceVersion':'1'},b'ca')}
                phase._secret=mock.Mock(side_effect=lambda name,key:copy.deepcopy(values[name])); cid='e'*64; commands=[]
                def docker(args,limit=1048576):
                    commands.append(args)
                    if args[:2]==['image','inspect']: return json.dumps([{'Id':image_id,'Os':'linux','Architecture':'amd64','RepoDigests':[roots['image']],'Config':{'Volumes':None}}]).encode()
                    if args[0]=='create': return cid.encode()
                    if args[0]=='inspect':
                        name=next(a.split('=',1)[1] for c in commands if c[0]=='create' for a in c if a.startswith('voice.test.owner='))
                        return json.dumps([{'Id':cid,'Image':image_id,'Config':{'Labels':{'voice.test.owner':name},'Entrypoint':['/bin/sh'],
                           'Cmd':['-ec',target.SCRIPT],'User':'65532:65532','OpenStdin':True},'HostConfig':{'NetworkMode':'none','ReadonlyRootfs':True,
                           'CapDrop':['ALL'],'SecurityOpt':['no-new-privileges'],'Tmpfs':{'/scratch':target.TMPFS}},'Mounts':[],'State':{'Status':'exited','ExitCode':0}}]).encode()
                    if args[0] in ('rm','ps'): return b''
                    raise AssertionError(args)
                phase._docker=mock.Mock(side_effect=docker)
                def line(value):return ':'.join(value[i:i+2].upper() for i in range(0,64,2)).encode()
                listing=(b'Keystore type: PKCS12\nYour keystore contains 2 entries\nEntry type: trustedCertEntry\nSHA256: '+line('aa'*32)+
                         b'\nEntry type: trustedCertEntry\nSHA256: '+line(ca_sha)+b'\n')
                with mock.patch.object(target,'_input',return_value=listing), mock.patch.object(target.material,'_cert_der',return_value=b'der'):
                    receipt=phase.run('token'); self.assertNotIn('PRIVATE-SENTINEL',json.dumps(receipt))
                    self.assertEqual(target.verify_current(phase,'token'),receipt)
                    successor=creator.Session.__new__(creator.Session); successor.preflight=pp; successor.package=pkg
                    successor.fence=fence; successor.lock=lock; successor.namespace_uid='ns'; successor.effects={}; successor._local=mock.Mock()
                    issued={'operation_id':context['operation_id'],'source_sha':context['commit'],'namespace_uid':'ns',
                            'preflight_sha256':target.writer_cut.digest(source)}
                    successor.record={'status':'done','receipt':issued}; backing.row[creator.LEDGER]=copy.deepcopy(successor.record)
                    backing.row[creator.EFFECTS]={}
                    with mock.patch.object(creator,'verify_completed',return_value=issued) as verify_creator, mock.patch.object(jdk,'verify_completed',return_value=roots):
                        self.assertEqual(target.verify_successor(phase,'token',successor),receipt)
                        verify_creator.side_effect=creator.CreateError('private')
                        with self.assertRaises(target.metadata.GuardLost):target.verify_successor(phase,'token',successor)
                        verify_creator.side_effect=None
                        backing.row[creator.LEDGER]={'status':'tampered'}
                        with self.assertRaises(target.metadata.GuardLost):target.verify_successor(phase,'token',successor)
                        backing.row[creator.LEDGER]=copy.deepcopy(successor.record); original_fence=successor.fence; original_record=copy.deepcopy(successor.record)
                        def late_mutation():
                            answer=backing.read(); successor.record={'status':'tampered'}; return answer
                        successor.fence=SimpleNamespace(context=context,ledger=SimpleNamespace(read=late_mutation))
                        with self.assertRaises(target.metadata.GuardLost):target.verify_successor(phase,'token',successor)
                        successor.fence=original_fence; successor.record=original_record
                    backing.row[creator.LEDGER]=copy.deepcopy(successor.record)
                    pp._local.side_effect=[ValueError('private'),None]
                    with self.assertRaises(target.metadata.GuardLost): target.verify_current(phase,'token')
                    self.assertEqual(target.verify_current(phase,'token'),receipt)
                    changed={'tampered':True}; original_record=copy.deepcopy(jp.record); original_entry=copy.deepcopy(jp.journal.state['steps'][jdk.STEP])
                    jp.record={'status':'done','receipt_sha256':target.writer_cut.digest(changed)}
                    jp.journal.state['steps'][jdk.STEP]=jp.record|{'receipt':changed}
                    with self.assertRaises(target.metadata.GuardLost): target.verify_current(phase,'token')
                    jp.record=original_record; jp.journal.state['steps'][jdk.STEP]=original_entry
                    phase.preflight=object()
                    with self.assertRaises(target.metadata.GuardLost): target.verify_current(phase,'token')
                    phase.preflight=pp
                    original_ledger=phase._ledger; original_marker=copy.deepcopy(phase.record); seen=[]
                    def last_read_tamper():
                        answer=original_ledger(); seen.append(1)
                        if len(seen)==2:phase.record=None
                        return answer
                    phase._ledger=last_read_tamper
                    with self.assertRaises(target.metadata.GuardLost): target.verify_current(phase,'token')
                    phase._ledger=original_ledger; phase.record=original_marker
                    values['voice-media-jvm-trust']=({'uid':'trust','resourceVersion':'2'},b'PRIVATE-SENTINEL')
                    with self.assertRaises(target.TargetTrustError): target.verify_current(phase,'token')
