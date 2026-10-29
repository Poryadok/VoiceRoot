"""Real root service/client/receiver socket with fixture CA and synthetic authority."""
import copy
import hashlib
import os
from pathlib import Path
import signal
import socket
import subprocess
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import dev_peer_issuer_client as client
import dev_peer_issuer_metadata as metadata
import dev_peer_issuer_packet as packet
import dev_peer_issuer_service as service
import space_creator_runtime_profile_delegated as receiver
import space_creator_runtime_profile_delegated_test as fixture


@unittest.skipUnless(os.name=='posix' and os.geteuid()==0,'isolated root Linux fixture')
class ServiceJointTest(unittest.TestCase):
    def exercise(self,profile,lost,preserve_signer=False):
        with tempfile.TemporaryDirectory(prefix='peer-service-root-',dir='/root') as root_tmp, \
             tempfile.TemporaryDirectory(prefix='peer-service-user-',dir='/tmp') as user_tmp:
            root=Path(root_tmp); user=Path(user_tmp); os.chown(user,1000,1000); os.chmod(user,0o700)
            archive=root/'archive'; archive.mkdir(mode=0o700); issue=root/'issuance'; issue.mkdir(mode=0o700)
            subprocess.run(['openssl','req','-x509','-newkey','rsa:2048','-nodes','-days','120','-subj','/CN=fixture',
                '-addext','basicConstraints=critical,CA:TRUE','-keyout',str(archive/'media-ca.key'),
                '-out',str(archive/'media-ca.crt')],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
            (archive/'media-ca.key').chmod(0o600); media=(archive/'media-ca.crt').read_bytes()
            der=packet.crypto._openssl(['x509','-in','{0}','-outform','DER'],[media]); sha=hashlib.sha256(der).hexdigest()
            suffix='a'*32; base=user/'peer'; operations=base/'operations'; operation=operations/suffix
            for path in (base,operations,operation):path.mkdir(mode=0o700); os.chown(path,1000,1000)
            path=user/'service.sock'; listener=socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET)
            listener.bind(str(path)); os.chown(path,1000,1000); os.chmod(path,0o600); listener.listen(1); listener.settimeout(45)
            pid=os.fork()
            if pid==0:
                case=None; ok=False
                try:
                    os.setgid(1000); os.setuid(1000)
                    tempfile.tempdir=str(user)
                    case=fixture.PhaseTest('test_unknown_issue_never_submits_kubernetes_write'); case.profile=profile; case.setUp()
                    if preserve_signer:
                        name='voice-gateway-principal-signing'; original=copy.deepcopy(case.desired[name])
                        original['metadata'].update(uid='preserved',resourceVersion='9'); case.actual[name]=original
                        case.identities[name]={'state':'present','uid':'preserved','resourceVersion':'9','keys':sorted(metadata.MEMBERS[name])}
                        case.initial['creator_secrets']=copy.deepcopy(case.identities)
                        case.preflight.record['creator_secrets']=copy.deepcopy(case.identities)
                    case.ca['der_sha256']=sha; case.initial['ca']=copy.deepcopy(case.ca)
                    case.preflight.record['ca']=copy.deepcopy(case.ca)
                    case.preflight.record['receipt_sha256']=receiver.writer_cut.digest(case.initial)
                    case.preflight.journal.state['steps'][case.preflight.step]=copy.deepcopy(case.preflight.record)|{'receipt':copy.deepcopy(case.initial)}
                    case.base.ledger.row[case.preflight.ledger]=copy.deepcopy(case.preflight.record)
                    case.package.verify_current.return_value={'run_id':1,'run_attempt':1}
                    with patch.object(receiver,'ROOT',operations),patch.object(client,'SOCKET',str(path)), \
                         patch.object(receiver.preflight.public,'_value',return_value=media):
                        if lost:
                            try:case.phase.run()
                            except receiver.DelegatedError:
                                ok=not case.writes and case.base.ledger.row['status']=='uncertain'
                                try:case.phase.run(); ok=False
                                except receiver.DelegatedError:pass
                        else:
                            receipt=case.phase.run()
                            ok=(set(receipt['objects'])==set(metadata.PROFILES[profile]) and
                                sum(not dry for _,dry in case.writes)==len(metadata.PROFILES[profile])-int(preserve_signer)
                                and (not preserve_signer or receipt['objects']['voice-gateway-principal-signing']['uid']=='preserved')
                                and receiver.verify_completed(case.phase,'token')==receipt)
                except BaseException:ok=False
                finally:
                    if case is not None and not case.doCleanups():ok=False
                os._exit(0 if ok else 1)
            try:
                connection,_=listener.accept()
                original_issue=service.issue
                def lose_reply(*args):
                    original_issue(*args); raise OSError('synthetic lost terminal reply')
                with connection,patch.object(service,'ROOT',issue),patch.object(service,'BASE',base), \
                     patch.object(service.custody,'ARCHIVE',archive),patch.object(service.authority,'verify'), \
                     patch.object(service,'_guard'),patch.object(service.root_export,'_pmd_uid',return_value=1000), \
                     patch.object(service,'issue',side_effect=lose_reply if lost else original_issue):
                    if lost:
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

    def test_space_real_sealed_packet_and_trigger_completion(self):self.exercise('space',False)

    def test_creator_real_sealed_packet_and_trigger_completion(self):self.exercise('creator',False)

    def test_creator_partial_preserves_existing_signer(self):self.exercise('creator',False,True)

    def test_lost_terminal_response_never_mutates_kubernetes(self):self.exercise('space',True)


if __name__=='__main__': unittest.main()
