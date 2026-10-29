"""Three constructed receiver phases and real crypto; TT and Kubernetes are fixtures."""
import copy
import hashlib
import os
from pathlib import Path
import signal
import shutil
import socket
import subprocess
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import Mock, patch

import dev_peer_issuer_metadata as metadata
import dev_peer_issuer_client as issuer_client
import dev_peer_issuer_service as service
import frontend_runtime_executor as transport
import full_cohort_package_phase as package
import media_operation_journal as journals
import resource_fencing
import realtime_config_transition_authority as authority
import reset_credentials as reset
import space_creator_runtime_profile_delegated as delegated
import space_creator_runtime_profile_delegated_test as fixture
import space_creator_runtime_profile_fixed9_successor as successor
import space_creator_runtime_profile_target_trust as target
import space_role_peer_create as role
import writer_cut


class ThreeProfileTest(unittest.TestCase):
    def check_space_authority(self,case,trust,creator,space,role_phase):
        obj=case.actual['voice-space-media-server-tls']; meta=obj['metadata']
        raw=(meta['uid']+'\n'+meta['resourceVersion']+'\n'+obj['data']['tls.crt']).encode()
        roots={'fixture':'jdk'}; trust_id={'uid':'trust','resourceVersion':'1'}
        ca_id={'uid':creator.initial['ca']['uid'],'resourceVersion':creator.initial['ca']['resourceVersion']}
        receipt={'jdk_receipt_sha256':writer_cut.digest(roots),'preflight_receipt_sha256':writer_cut.digest(creator.initial),
                 'trust_secret':trust_id,'media_ca':copy.deepcopy(creator.initial['ca'])}
        trust.record={'status':'done','receipt_sha256':writer_cut.digest(receipt)}
        trust.journal.state['steps'][target.STEP]=copy.deepcopy(trust.record)|{'receipt':receipt}
        case.base.ledger.row[target.LEDGER]=copy.deepcopy(trust.record)
        trust.jdk=SimpleNamespace(); trust._locals=Mock()
        trust._secret=Mock(side_effect=lambda name,key:(trust_id,b'trust') if name=='voice-media-jvm-trust' else (ca_id,creator.media))
        with patch.object(authority.transport,'public_certificate',return_value=raw),patch.object(target.jdk,'verify_completed',return_value=roots):
            binding=authority.bind_delegated_space(case.base.cut,trust,creator,space,role_phase,'token')
            proof,jwks=authority._space(case.base.cut,binding,'token')
        self.assertEqual(proof['schema'],'voice-dev-space-delegated-v1')
        self.assertEqual(proof['tls']['uid'],meta['uid']); self.assertTrue(jwks)
    def test_three_constructed_preserve_phases_and_fixed9_drift(self):
        case=fixture.PhaseTest('test_unknown_issue_never_submits_kubernetes_write'); case.profile='creator'
        self.addCleanup(lambda:self.assertTrue(case.doCleanups())); case.setUp()
        material=reset.ResetMaterial(case.root); ca=(case.root/'fixture-ca.key',case.root/'fixture-ca.crt')
        for name in sorted(set(metadata.MEMBERS)-set(case.desired)):
            if name.endswith('-principal-signing'): material.signer(name,active=True)
            else: material.tls(name,ca,*delegated.packet.TLS[name])
        for obj in material.items:case.desired[obj['metadata']['name']]=copy.deepcopy(obj)
        for name,obj in case.desired.items():
            obj=copy.deepcopy(obj); obj['metadata'].update(uid='uid-'+name,resourceVersion='1'); case.actual[name]=obj
            case.identities[name]={'state':'present','uid':'uid-'+name,'resourceVersion':'1','keys':sorted(metadata.MEMBERS[name])}
        case.package.journal.state['steps'][package.STEP]={'status':'done','receipt':{'fixture':True}}
        def set_initial(pre,profile):
            initial={'profile':profile,'ca':copy.deepcopy(case.ca),'creator_secrets':copy.deepcopy(case.identities)}
            pre.record={'status':'done','ca':copy.deepcopy(case.ca),'creator_secrets':copy.deepcopy(case.identities),
                        'receipt_sha256':writer_cut.digest(initial)}
            pre.journal.state['steps'][pre.step]=copy.deepcopy(pre.record)|{'receipt':copy.deepcopy(initial)}
            case.base.ledger.row[pre.ledger]=copy.deepcopy(pre.record)
            return initial
        set_initial(case.preflight,'creator')
        client_type=type(case.base.client); backing=case.base.ledger
        class Ledger:
            def __init__(self,client):self.client=client
            def read(self):return backing.read()
            def write(self,revision,row):return backing.write(revision,row)
        def attach(name,intent):
            journal=journals.Journal(case.root/name,intent); self.addCleanup(journal.close)
            client=client_type(case.base.lock,journal); client.workloads=case.base.client.workloads
            fence=resource_fencing.Fence(case.base.context,lambda:copy.deepcopy(case.base.card),Ledger(client))
            return journal,client,fence
        base={'resource_context':copy.deepcopy(case.base.context),'namespace_uid':'ns'}
        def fresh(profile):
            intent=base|{'dev_peer_issuer_preflight':{'source_root':str(case.root),'source_sha':case.base.context['commit'],'profile':profile}}
            j,c,f=attach('pre-'+profile,intent)
            pre=delegated.preflight.PeerIssuerPreflightPhase(j,f,c,case.base.lock,'ns',case.package,case.base.cut,str(case.root),'token',profile)
            pre._ca=Mock(side_effect=lambda:copy.deepcopy(case.ca)); pre._creator=Mock(side_effect=lambda:copy.deepcopy(case.identities))
            set_initial(pre,profile); return pre
        with patch.object(role.canonical,'ROOT',case.root),patch.object(role.source,'verify'), \
             patch.object(role,'_get',side_effect=lambda name:copy.deepcopy(case.actual.get(name))):
            creator=case.phase; creator.run()
            pre=fresh('space'); intent=base|{'peer_receiver':{'profile':'space','source_sha':case.base.context['commit']}}
            j,c,f=attach('space-receiver',intent)
            space=delegated.SpaceReceiver(j,f,c,case.base.lock,'ns',case.package,case.base.cut,pre,case.root,'token'); space.run()
            pre=fresh('role'); j,c,f=attach('role-receiver',base)
            role_phase=role.Session(j,f,c,case.base.lock,'ns',case.package,pre,case.root,'token')
            self.assertEqual(role_phase.prepare()['status'],'done')
            trust=target.TargetTrustPhase.__new__(target.TargetTrustPhase)
            trust.package=case.package; trust.lock=case.base.lock; trust.namespace_uid='ns'; trust.fence=creator.fence
            trust.preflight=creator.preflight; trust._guard=Mock(); trust._local=Mock(); trust._ledger=backing.read
            trust.journal=SimpleNamespace(state={'steps':{target.STEP:{'receipt':{'preflight_receipt_sha256':writer_cut.digest(creator.initial)}}}})
            self.assertEqual(successor.verify(trust,'token',creator,space,role_phase),creator.initial)
            self.check_space_authority(case,trust,creator,space,role_phase)
            case.identities['voice-space-principal-signing']['resourceVersion']='drift'
            with self.assertRaises(target.metadata.GuardLost):successor.verify(trust,'token',creator,space,role_phase)
        self.assertEqual(case.writes,[])

    @unittest.skipUnless(os.name=='posix' and os.geteuid()==0,'isolated Linux root fixture')
    def test_missing_nine_through_service_and_three_real_receivers(self):
        with tempfile.TemporaryDirectory(dir='/root',prefix='fixed9-root-') as root_tmp, tempfile.TemporaryDirectory(dir='/tmp',prefix='fixed9-user-') as user_tmp:
            root=Path(root_tmp); user=Path(user_tmp); os.chown(user,1000,1000); os.chmod(user,0o700)
            archive=root/'archive'; archive.mkdir(mode=0o700); issue=root/'issue'; issue.mkdir(mode=0o700)
            subprocess.run(['openssl','req','-x509','-newkey','rsa:2048','-nodes','-days','120','-subj','/CN=fixture',
                '-addext','basicConstraints=critical,CA:TRUE','-keyout',str(archive/'media-ca.key'),'-out',str(archive/'media-ca.crt')],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
            (archive/'media-ca.key').chmod(0o600); media=(archive/'media-ca.crt').read_bytes()
            sha=hashlib.sha256(delegated.packet.crypto._openssl(['x509','-in','{0}','-outform','DER'],[media])).hexdigest()
            home=Path('/home/pmd'); home.mkdir(mode=0o700); os.chown(home,1000,1000)
            self.addCleanup(shutil.rmtree,home)
            base=home/'voice-dev-peer-issuer'; operations=base/'operations'; operation=operations/('a'*32)
            for path in (base,operations,operation):path.mkdir(mode=0o700); os.chown(path,1000,1000)
            socket_path=user/'service.sock'; listener=socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET)
            listener.bind(str(socket_path)); os.chown(socket_path,1000,1000); os.chmod(socket_path,0o600); listener.listen(1); listener.settimeout(45)
            pid=os.fork()
            if pid==0:
                case=None; ok=False
                try:
                    os.setgid(1000); os.setuid(1000); tempfile.tempdir=str(user)
                    case=fixture.PhaseTest('test_unknown_issue_never_submits_kubernetes_write'); case.profile='creator'; case.setUp()
                    case.ca['der_sha256']=sha; case.initial['ca']=copy.deepcopy(case.ca)
                    case.preflight.record['ca']=copy.deepcopy(case.ca); case.preflight.record['receipt_sha256']=writer_cut.digest(case.initial)
                    case.preflight.journal.state['steps'][case.preflight.step]=copy.deepcopy(case.preflight.record)|{'receipt':copy.deepcopy(case.initial)}
                    backing=case.base.ledger; backing.row[case.preflight.ledger]=copy.deepcopy(case.preflight.record)
                    case.package.journal.state['steps'][package.STEP]={'status':'done','receipt':{'fixture':True}}
                    case.package.verify_current.return_value={'run_id':1,'run_attempt':1}
                    client_type=type(case.base.client)
                    class Ledger:
                        def __init__(self,client):self.client=client
                        def read(self):return backing.read()
                        def write(self,revision,row):return backing.write(revision,row)
                    def attach(name,intent):
                        journal=journals.Journal(case.root/name,intent); case.addCleanup(journal.close)
                        client=client_type(case.base.lock,journal); client.workloads=case.base.client.workloads
                        fence=resource_fencing.Fence(case.base.context,lambda:copy.deepcopy(case.base.card),Ledger(client))
                        return journal,client,fence
                    base_intent={'resource_context':copy.deepcopy(case.base.context),'namespace_uid':'ns'}
                    def fresh(profile):
                        intent=base_intent|{'dev_peer_issuer_preflight':{'source_root':str(case.root),'source_sha':case.base.context['commit'],'profile':profile}}
                        j,c,f=attach('pre-'+profile,intent)
                        pre=delegated.preflight.PeerIssuerPreflightPhase(j,f,c,case.base.lock,'ns',case.package,case.base.cut,str(case.root),'token',profile)
                        pre._ca=Mock(side_effect=lambda:copy.deepcopy(case.ca)); pre._creator=Mock(side_effect=lambda:copy.deepcopy(case.identities))
                        initial={'profile':profile,'ca':copy.deepcopy(case.ca),'creator_secrets':copy.deepcopy(case.identities)}
                        pre.record={'status':'done','ca':copy.deepcopy(case.ca),'creator_secrets':copy.deepcopy(case.identities),'receipt_sha256':writer_cut.digest(initial)}
                        j.state['steps'][pre.step]=copy.deepcopy(pre.record)|{'receipt':copy.deepcopy(initial)}; backing.row[pre.ledger]=copy.deepcopy(pre.record)
                        return pre
                    with patch.object(delegated,'ROOT',operations),patch.object(role,'ROOT',operations),patch.object(role.canonical,'ROOT',case.root), \
                         patch.object(role.source,'verify'),patch.object(issuer_client,'SOCKET',str(socket_path)), \
                         patch.object(delegated.preflight.public,'_value',return_value=media), \
                         patch.object(role,'_get',side_effect=lambda name:copy.deepcopy(case.actual.get(name))), \
                         patch.object(role,'_create',side_effect=lambda obj,dry:case.create(obj,'role',dry)):
                        creator=case.phase; creator.run()
                        pre=fresh('space'); intent=base_intent|{'peer_receiver':{'profile':'space','source_sha':case.base.context['commit']}}
                        j,c,f=attach('space-receiver',intent)
                        space=delegated.SpaceReceiver(j,f,c,case.base.lock,'ns',case.package,case.base.cut,pre,case.root,'token'); space.run()
                        pre=fresh('role'); j,c,f=attach('role-receiver',base_intent)
                        role_phase=role.Session(j,f,c,case.base.lock,'ns',case.package,pre,case.root,'token')
                        role_phase.prepare(); role_phase.run()
                        trust=target.TargetTrustPhase.__new__(target.TargetTrustPhase)
                        trust.package=case.package; trust.lock=case.base.lock; trust.namespace_uid='ns'; trust.fence=creator.fence
                        trust.preflight=creator.preflight; trust._guard=Mock(); trust._local=Mock(); trust._ledger=backing.read
                        trust.journal=SimpleNamespace(state={'steps':{target.STEP:{'receipt':{'preflight_receipt_sha256':writer_cut.digest(creator.initial)}}}})
                        ok=(successor.verify(trust,'token',creator,space,role_phase)==creator.initial and
                            sum(not dry for _,dry in case.writes)==9)
                        self.check_space_authority(case,trust,creator,space,role_phase)
                        case.identities['voice-space-principal-signing']['resourceVersion']='drift'
                        try:successor.verify(trust,'token',creator,space,role_phase); ok=False
                        except target.metadata.GuardLost:pass
                except BaseException:
                    import traceback; traceback.print_exc(); ok=False
                finally:
                    if case is not None and not case.doCleanups():ok=False
                os._exit(0 if ok else 1)
            try:
                with patch.object(service,'ROOT',issue),patch.object(service,'BASE',base),patch.object(service.custody,'ARCHIVE',archive), \
                     patch.object(service.authority,'verify'),patch.object(service,'_guard'),patch.object(service.root_export,'_pmd_uid',return_value=1000):
                    for _ in range(3):
                        connection,_=listener.accept()
                        with connection:service.serve(connection)
                _,status=os.waitpid(pid,0); pid=None
                self.assertTrue(os.WIFEXITED(status) and os.WEXITSTATUS(status)==0)
            finally:
                listener.close()
                if pid is not None:
                    try:os.kill(pid,signal.SIGKILL)
                    except ProcessLookupError:pass
                    os.waitpid(pid,0)
