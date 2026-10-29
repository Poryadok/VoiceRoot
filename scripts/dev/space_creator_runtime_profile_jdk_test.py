import copy
import json
from pathlib import Path
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
from space_creator_runtime_profile_trust_test import listing


class JdkTest(unittest.TestCase):
    def test_last_ledger_read_cannot_tamper_done_local_record(self):
        phase=jdk.TargetJdkPhase.__new__(jdk.TargetJdkPhase); row={}; rev=('uid','rv')
        phase.journal=SimpleNamespace(state={'steps':{}},save=mock.Mock()); phase.poisoned=False
        phase.fence=SimpleNamespace(context={'operation_id':'op','commit':'sha'},ledger=SimpleNamespace(),uncertain=mock.Mock())
        phase._guard=mock.Mock(); phase._local=mock.Mock(); phase._inventory=mock.Mock(); phase._package=mock.Mock()
        def read():
            result=copy.deepcopy(row)
            if result.get(jdk.LEDGER,{}).get('status')=='done': phase.journal.state['steps'].pop(jdk.STEP,None)
            return result,rev
        def write(_,new): row.clear(); row.update(copy.deepcopy(new))
        phase._ledger=read; phase.fence.ledger.write=write
        def capture(*_):
            marker=copy.deepcopy(row[jdk.LEDGER]); marker['tool']={'status':'removed'}
            row[jdk.LEDGER]=copy.deepcopy(marker); phase.journal.state['steps'][jdk.STEP]=copy.deepcopy(marker); phase.record=marker
            return {'scope':'synthetic'}
        phase._capture=capture
        with self.assertRaises(jdk.JdkError): phase.run('token','actor')
        self.assertTrue(phase.poisoned)

    def test_declared_volumes_and_wrong_image_are_rejected_before_create(self):
        info=[{'Id':'correct','Os':'linux','Architecture':'amd64','RepoDigests':['image'],'Config':{'Volumes':None}}]
        jdk._image_info(info,'image','correct')
        wrong=copy.deepcopy(info); wrong[0]['Config']['Volumes']={'/unexpected':{}}
        with self.assertRaises(ValueError): jdk._image_info(wrong,'image','correct')
        with self.assertRaises(ValueError): jdk._image_info(info,'image','other')

    def test_protected_capture_cleanup_and_receipt_drift(self):
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp); context={'project':'voice','resource':'DEV','actor':'scheduled_devops',
              'claim':'11111111-1111-4111-8111-111111111111','window':'22222222-2222-4222-8222-222222222222',
              'revision':4,'generation':3,'operation_id':'media-c1-'+'a'*32,'commit':'b'*40,
              'images':{'voice-auth/auth':'ghcr.io/poryadok/voiceroot/auth@sha256:'+'c'*64}}
            (root/context['operation_id']).mkdir(mode=0o700)
            journal=journals.Journal(root/'journal',{'resource_context':context,'namespace_uid':'ns'}); self.addCleanup(journal.close)
            backing=fixture.Ledger(context); client=transport.KubectlTransport(journal); ledger=resource_fencing.KubeLedger(client)
            ledger.read=backing.read; ledger.write=backing.write; fence=resource_fencing.Fence(context,lambda:{},ledger)
            pkg=package.PackagePhase.__new__(package.PackagePhase); inv=inventory.InventoryPhase.__new__(inventory.InventoryPhase); lock={'metadata':{'uid':'lock'}}
            for name,obj in (('pkg',pkg),('inv',inv)):
                other=journals.Journal(root/name,{'resource_context':context,'namespace_uid':'ns'}); self.addCleanup(other.close)
                obj.journal=other; obj.client=transport.KubectlTransport(other); other_ledger=resource_fencing.KubeLedger(obj.client)
                other_ledger.read=backing.read; other_ledger.write=backing.write
                obj.fence=resource_fencing.Fence(context,lambda:{},other_ledger); obj.lock=lock; obj.namespace_uid='ns'
            inv.package=pkg; inv.verify_current=mock.Mock(return_value={})
            pkg.verify_current=mock.Mock(return_value={'images':context['images'],'platform_digests':{'voice-auth/auth':'sha256:'+'d'*64}})
            phase=jdk.TargetJdkPhase(journal,fence,client,lock,'ns',pkg,inv); commands=[]; cid='e'*64; image_id='sha256:'+'f'*64
            def docker(args,limit=1048576):
                commands.append(args)
                if args[0]=='pull': return b''
                if args[:2]==['image','inspect']: return json.dumps([{'Id':image_id,'Os':'linux','Architecture':'amd64','RepoDigests':[context['images']['voice-auth/auth']],'Config':{'Volumes':None}}]).encode()
                if args[0]=='create': return cid.encode()
                if args[0]=='inspect':
                    name=next(a.split('=',1)[1] for c in commands if c[0]=='create' for a in c if a.startswith('voice.test.owner='))
                    args=['-list','-v','-keystore','/opt/java/openjdk/lib/security/cacerts','-storepass','changeit','-J-Duser.language=en','-J-Duser.country=US']
                    return json.dumps([{'Id':cid,'Image':image_id,'Config':{'Labels':{'voice.test.owner':name},'Entrypoint':['/opt/java/openjdk/bin/keytool'],'Cmd':args},'HostConfig':{'NetworkMode':'none'},'Mounts':[],
                                        'State':{'Status':'exited','ExitCode':0}}]).encode()
                if args[0]=='start': return listing(('trustedCertEntry','AA'))
                if args[0] in ('rm','ps'): return b''
                raise AssertionError(args)
            phase._docker=mock.Mock(side_effect=docker)
            with mock.patch.object(guard,'verify'), mock.patch.object(jdk.eligibility,'read_image_config',return_value={'config_digest':image_id,'env':{}}), \
                 mock.patch.object(jdk.publish_candidate,'Registry'):
                receipt=phase.run('token','actor'); verified=jdk.verify_completed(phase,'token')
                self.assertEqual(verified,receipt); self.assertEqual(receipt['root_fingerprints'],['aa'*32])
                self.assertTrue(any(x[0]=='rm' and cid in x for x in commands))
                before=len(commands)
                with self.assertRaises(jdk.JdkError): phase.run('token','actor')
                self.assertEqual(len(commands),before)
                phase.journal.state['steps'][jdk.STEP]['receipt']['root_fingerprints']=['bb'*32]
                with self.assertRaises(jdk.JdkError): jdk.verify_completed(phase,'token')
                phase.journal.state['steps'][jdk.STEP]['receipt']=copy.deepcopy(receipt)
                original=phase._ledger; calls=[]
                def late_close():
                    result=original(); calls.append(1)
                    if len(calls)==2: inv.journal.close()
                    return result
                phase._ledger=late_close
                with self.assertRaises(jdk.metadata.GuardLost): jdk.verify_completed(phase,'token')
