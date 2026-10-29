"""Real delegated service/capability/Session; external TT, Kube and GitHub are fixtures."""
import base64
import copy
import hashlib
import os
from pathlib import Path
import shutil
import signal
import socket
import tempfile
import threading
import unittest
from unittest.mock import Mock, patch

import dev_peer_issuer_client as peer_client
import dev_peer_issuer_service as service
import dev_realtime_p3_issuer_capability as capability
import dev_realtime_p3_issuer_client as client
import dev_realtime_p3_issuer_service as p3_service
import media_operation_journal as journals
import realtime_p3_create_only as creation
import realtime_p3_create_only_packet as packet
import realtime_p3_create_only_packet_test as packet_fixture
import realtime_p3_create_only_source as source
import realtime_p3_missing_issuer as issuer
import realtime_p3_root_export as export
import resource_fencing
import space_creator_runtime_profile_delegated_test as fixture
import writer_cut


@unittest.skipUnless(os.name=='posix' and os.geteuid()==0,'isolated Linux root fixture')
class DelegatedP3JointTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        def cleanup():
            failed=bool(getattr(packet_fixture.PacketTest,'tearDown_exceptions',()))
            packet_fixture.PacketTest.doClassCleanups()
            if failed or packet_fixture.PacketTest.tearDown_exceptions:raise RuntimeError('nested class cleanup failed')
        cls.addClassCleanup(cleanup)
        packet_fixture.PacketTest.setUpClass()

    def exercise(self,lost):
        with tempfile.TemporaryDirectory(dir='/root',prefix='p3-delegated-root-') as root_tmp, tempfile.TemporaryDirectory(dir='/tmp',prefix='p3-delegated-user-') as user_tmp:
            root=Path(root_tmp); user=Path(user_tmp); os.chown(user,1000,1000); os.chmod(user,0o700)
            archive=root/'archive'; archive.mkdir(mode=0o700); issue=root/'issue'; issue.mkdir(mode=0o700)
            source_dir=Path(packet_fixture.PacketTest.temp.name)
            shutil.copyfile(source_dir/'principal.key',archive/'principal-ca.key'); os.chmod(archive/'principal-ca.key',0o600)
            shutil.copyfile(source_dir/'principal.crt',archive/'principal-ca.crt'); os.chmod(archive/'principal-ca.crt',0o600)
            principal=packet_fixture.PacketTest.principal; media=packet_fixture.PacketTest.media; space=packet_fixture.PacketTest.space
            def identity(name,value):
                return {'uid':'uid-'+name,'resourceVersion':'9','der_sha256':hashlib.sha256(packet.crypto.der(value)).hexdigest()}
            roots={'principal':identity('voice-principal-ca',principal),'media':identity('voice-media-ca',media),
                   'space':identity('voice-space-media-server-tls',space)}
            home=Path('/home/pmd'); home.mkdir(mode=0o700); os.chown(home,1000,1000); self.addCleanup(shutil.rmtree,home)
            peer_base=home/'voice-dev-peer-issuer'; pre_root=peer_base/'preflight'; suffix=pre_root/('a'*32); pre_dir=suffix/'realtime'
            p3_base=home/'voice-realtime-p3'; p3_ops=p3_base/'operations'; p3_dir=p3_ops/('a'*32)
            for path in (peer_base,pre_root,suffix,pre_dir,p3_base,p3_ops,p3_dir):path.mkdir(mode=0o700); os.chown(path,1000,1000)
            service_path=user/'issuer.sock'; listener=socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET)
            listener.bind(str(service_path)); os.chown(service_path,1000,1000); os.chmod(service_path,0o600); listener.listen(1); listener.settimeout(45)
            pid=os.fork()
            if pid==0:
                case=None; ok=False
                try:
                    os.setgid(1000); os.setuid(1000); tempfile.tempdir=str(user)
                    case=fixture.PhaseTest('test_unknown_issue_never_submits_kubernetes_write'); case.profile='creator'; case.setUp()
                    case.ca=copy.deepcopy(roots['media']); case.initial['ca']=copy.deepcopy(case.ca)
                    case.preflight.record['ca']=copy.deepcopy(case.ca); case.preflight.record['receipt_sha256']=writer_cut.digest(case.initial)
                    case.preflight.journal.state['steps'][case.preflight.step]=copy.deepcopy(case.preflight.record)|{'receipt':copy.deepcopy(case.initial)}
                    backing=case.base.ledger; backing.row[case.preflight.ledger]=copy.deepcopy(case.preflight.record)
                    case.package.verify_current.return_value={'run_id':1,'run_attempt':1}
                    case.package.journal.state['steps']['full-cohort-package']={'status':'done','receipt':{'fixture':True}}
                    cap=capability.RealtimeP3Capability(case.preflight,'token'); case.addCleanup(cap.close)
                    current={}; calls=[]
                    seven=lambda:{kind.lower()+'/'+name:({'state':'absent'} if (kind,name) not in current else {'state':'present'}) for kind,name in packet.MEMBERS}
                    cap._principal=Mock(side_effect=lambda:copy.deepcopy(roots['principal'])); cap._seven=Mock(side_effect=seven)
                    client_type=type(case.base.client)
                    class Ledger:
                        def __init__(self,client):self.client=client
                        def read(self):return backing.read()
                        def write(self,revision,row):return backing.write(revision,row)
                    intent={'resource_context':copy.deepcopy(case.base.context),'namespace_uid':'ns',
                            'realtime_p3_create':{'issuer_source_sha':source.REVIEWED_ISSUER_SHA,'source_repo':str(case.root),'issuer_suffix':'a'*32}}
                    j=journals.Journal(case.root/'p3-session',intent); case.addCleanup(j.close)
                    c=client_type(case.base.lock,j); c.workloads=case.base.client.workloads
                    f=resource_fencing.Fence(case.base.context,lambda:copy.deepcopy(case.base.card),Ledger(c))
                    session=creation.Session(j,f,c,case.base.lock,'ns'); case.addCleanup(session.close)
                    values={'voice-principal-ca':principal,'voice-media-ca':media,'voice-space-media-server-tls':space}
                    def public(name,key):return ('uid-'+name+'\n9\n'+base64.b64encode(values[name]).decode()).encode()
                    def get(kind,name):return copy.deepcopy(current.get((kind,name)))
                    def create(obj,dry):
                        calls.append((obj['metadata']['name'],dry)); result=copy.deepcopy(obj)
                        if not dry:
                            name=obj['metadata']['name']; result['metadata'].update(uid='uid-'+name,resourceVersion='10')
                            current[obj['kind'],name]=copy.deepcopy(result)
                        return result
                    with patch.object(capability,'SOCKET_ROOT',pre_root),patch.object(creation,'SOCKET_PARENT',p3_ops), \
                         patch.object(peer_client,'SOCKET',str(service_path)),patch.object(source,'verify'), \
                         patch.object(creation.transport,'public_certificate',side_effect=public), \
                         patch.object(creation.transport,'get',side_effect=get),patch.object(creation.transport,'create',side_effect=create):
                        cap.prepare(); handle=client.new_capability_trigger(cap,'token'); outcome=[]
                        def trigger():
                            try:handle.run(); outcome.append(True)
                            except BaseException:
                                import traceback; traceback.print_exc(); outcome.append(False)
                        thread=threading.Thread(target=trigger,daemon=True); thread.start(); cap.finish(); thread.join(45)
                        if thread.is_alive() or outcome!=[True]:raise ValueError('capability trigger')
                        session.prepare()
                        if lost:
                            try:session.finish_delegated(cap,'token')
                            except creation.CreateError:
                                ok=not calls and backing.row['status']=='uncertain'
                                try:session.prepare(); ok=False
                                except creation.CreateError:pass
                        else:
                            receipt=session.finish_delegated(cap,'token')
                            ok=(len(receipt['objects'])==7 and len(current)==7 and
                                [dry for _,dry in calls]==[True]*7+[False]*7 and session.completed)
                except BaseException:
                    import traceback; traceback.print_exc(); ok=False
                finally:
                    if case is not None and not case.doCleanups():ok=False
                os._exit(0 if ok else 1)
            try:
                def observe(namespace,context,lock,media_ref,principal_ref,space_ref=None):
                    objects={kind.lower()+'/'+name:{'state':'absent'} for kind,name in packet.MEMBERS}
                    return objects,{'principal':principal,'media':media,'space':space}
                original=p3_service.issue
                def issue_then_lose(*args):original(*args); raise OSError('synthetic terminal loss')
                with patch.object(service,'BASE',peer_base),patch.object(service.authority,'verify'), \
                     patch.object(service.root_export,'_pmd_uid',return_value=1000),patch.object(p3_service.metadata,'observe',side_effect=observe), \
                     patch.object(export,'ROOT',issue),patch.object(export,'_check_root_custody'), \
                     patch.object(issuer,'PARENT',issue),patch.object(issuer,'ARCHIVE',archive), \
                     patch.object(p3_service,'issue',side_effect=issue_then_lose if lost else original):
                    for index in range(2):
                        connection,_=listener.accept()
                        with connection:
                            if lost and index==1:
                                with self.assertRaises(OSError):service.serve(connection)
                            else:service.serve(connection)
                _,status=os.waitpid(pid,0); pid=None
                self.assertTrue(os.WIFEXITED(status) and os.WEXITSTATUS(status)==0)
            finally:
                listener.close()
                if pid is not None:
                    try:os.kill(pid,signal.SIGKILL)
                    except ProcessLookupError:pass
                    os.waitpid(pid,0)

    def test_actual_capability_service_and_seven_create(self):self.exercise(False)
    def test_lost_terminal_after_ack_never_dry_runs(self):self.exercise(True)
