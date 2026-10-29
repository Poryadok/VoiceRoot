"""Actual root exporter to unprivileged socket receiver; synthetic packet only."""
import json
import os
from pathlib import Path
import signal
import socket
import tempfile
import unittest
from unittest.mock import patch
from types import SimpleNamespace
import sys

import realtime_p3_root_export as exporter
import realtime_p3_create_only_socket as receiver
import realtime_p3_create_only as create
import realtime_p3_create_only_test as fixture


def cleanup_fixture(case, classes):
    ok = True
    if case is not None:
        try: ok = case.doCleanups() and ok
        except BaseException: ok = False
    for cls in classes:
        try:
            ok = not getattr(cls,'tearDown_exceptions',[]) and ok
            cls.doClassCleanups()
            ok = not getattr(cls,'tearDown_exceptions',[]) and ok
        except BaseException: ok = False
    return ok


class ExportIntegrationTest(unittest.TestCase):
    def test_cleanup_failure_is_reported_and_all_levels_attempted(self):
        calls = []
        class Case:
            def doCleanups(self): calls.append('case'); return False
        class Outer:
            tearDown_exceptions = []
            @classmethod
            def doClassCleanups(cls): calls.append('outer'); cls.tearDown_exceptions = [RuntimeError()]
        class Inner:
            tearDown_exceptions = []
            @classmethod
            def doClassCleanups(cls): calls.append('inner')
        self.assertFalse(cleanup_fixture(Case(),(Outer,Inner)))
        self.assertEqual(calls,['case','outer','inner'])
        self.assertTrue(cleanup_fixture(None,(Inner,)))

    def test_nested_unittest_cleanup_error_survives_second_cleanup(self):
        class Inner(unittest.TestCase): pass
        class Outer(unittest.TestCase): pass
        def fail(): raise RuntimeError('fixture cleanup failure')
        Inner.addClassCleanup(fail)
        Outer.addClassCleanup(Inner.doClassCleanups)
        self.assertFalse(cleanup_fixture(None,(Outer,Inner)))

    @unittest.skipUnless(os.geteuid() == 0, 'requires isolated Linux root fixture')
    def test_root_sender_to_unprivileged_consumer(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary); root.chmod(0o755)
            issuance = root/'issuance'; issuance.mkdir(mode=0o700)
            op = 'media-c1-' + '1'*32; packet = issuance/('p3-'+'1'*32); packet.mkdir(mode=0o700)
            (issuance/'p3-issuance.once').write_text(packet.name+'\n')
            (packet/'complete').write_text('Packet complete; runtime and cluster are not qualified.\n')
            receipt = {'status':'staged','namespace':'voice-dev','cluster_applied':False,'runtime_qualified':False,
                       'object_names':sorted(exporter.ASSET_KEYS),'principal_ca_der_sha256':'a'*64,
                       'media_ca_der_sha256':'b'*64,'space_jwks_cert_der_sha256':'c'*64}
            (packet/'receipt.json').write_text(json.dumps(receipt))
            items = []
            for name,keys in exporter.ASSET_KEYS.items():
                public = name == 'voice-p3-principal-public'
                items.append({'apiVersion':'v1','kind':'ConfigMap' if public else 'Secret',
                    'metadata':{'name':name,'namespace':'voice-dev'},'data':dict.fromkeys(keys,''),
                    **({'immutable':True} if public else {'type':'Opaque'})})
            objects = {'apiVersion':'v1','kind':'List','items':items}
            (packet/'objects.json').write_text(json.dumps(objects))
            for path in (issuance/'p3-issuance.once',*packet.iterdir()): path.chmod(0o600)
            private = root/'consumer'; private.mkdir(mode=0o700); os.chown(private,1000,1000)
            path = private/'packet.sock'; read_fd,write_fd = os.pipe()
            pid = os.fork()
            if pid == 0:
                try:
                    os.close(read_fd); os.setgid(1000); os.setuid(1000)
                    with socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET) as listener:
                        listener.bind(str(path)); os.chmod(path,0o600); listener.listen(1)
                        os.write(write_fd,b'R')
                        value = receiver.receive(listener,op,'a'*40)
                        os.write(write_fd,b'Y' if value['objects'] == objects and value['issuer_receipt'] == receipt else b'N')
                    os._exit(0)
                except BaseException:
                    os._exit(1)
            os.close(write_fd)
            finished = False
            try:
                self.assertEqual(os.read(read_fd,1),b'R')
                ticks = exporter._start_ticks(pid)
                with patch.object(exporter,'ROOT',issuance), patch.object(exporter,'_socket_path',return_value=path), \
                     patch.object(exporter,'_check_root_custody'), patch.object(exporter,'_pmd_uid',return_value=1000):
                    result = exporter.export(op,'a'*40,pid,ticks)
                    self.assertEqual(result['status'],'send_attempted')
                    with self.assertRaises(exporter.ExportError): exporter.export(op,'a'*40,pid,ticks)
                self.assertEqual(os.read(read_fd,1),b'Y')
                finished = True
            finally:
                os.close(read_fd)
                if not finished:
                    try: os.kill(pid,signal.SIGTERM)
                    except ProcessLookupError: pass
                _, status = os.waitpid(pid,0)
            self.assertEqual(status,0)

    @unittest.skipUnless(os.geteuid() == 0, 'requires isolated Linux root fixture')
    def test_export_to_real_journaled_session(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary); root.chmod(0o755)
            issuance = root/'issuance'; issuance.mkdir(mode=0o700)
            child_tmp = root/'child'; child_tmp.mkdir(mode=0o700); os.chown(child_tmp,1000,1000)
            read_fd,write_fd = os.pipe(); pid = os.fork()
            if pid == 0:
                case = None; ok = False
                try:
                    os.close(read_fd); os.setgid(1000); os.setuid(1000)
                    tempfile.tempdir = str(child_tmp)
                    fixture.CreateOnlyTest.setUpClass()
                    case = fixture.CreateOnlyTest('test_actual_crypto_dryrun_then_seven_create_and_exact_readback')
                    case.setUp()
                    os.write(write_fd,json.dumps({'envelope':case.envelope,'socket':str(case.parent/case.suffix/'packet.sock')}).encode()+b'\n')
                    actual_receive = receiver.receive
                    def receive(*args):
                        os.write(write_fd,b'R'); return actual_receive(*args)
                    with patch.dict(sys.modules,{'realtime_p3_create_only_source':SimpleNamespace(verify=lambda *args:None)}), \
                         patch.object(create,'SOCKET_PARENT',case.parent), patch.object(create.transport,'public_certificate',side_effect=case.public), \
                         patch.object(create.transport,'get',side_effect=case.get), patch.object(create.transport,'create',side_effect=case.create), \
                         patch.object(create.channel,'receive',side_effect=receive):
                        # Keep the actual receiver before installing the wrapper.
                        ready = case.session.prepare()
                        result = case.session.finish()
                    ok = ready['status'] == 'ready' and len(result['objects']) == 7 and len(case.created) == 7
                    ok = ok and case.base.ledger.row['realtime_p3_handoff']['status'] == 'done'
                except BaseException:
                    ok = False
                finally:
                    ok = cleanup_fixture(case,(fixture.CreateOnlyTest,fixture.packet_fixture.PacketTest)) and ok
                    try: os.write(write_fd,b'Y' if ok else b'N')
                    except OSError: pass
                    os._exit(0 if ok else 1)
            os.close(write_fd); finished = False
            try:
                with os.fdopen(read_fd,'rb',buffering=0) as stream:
                    line = bytearray()
                    while not line.endswith(b'\n') and len(line) <= 280000:
                        chunk = stream.read(1)
                        if not chunk: break
                        line.extend(chunk)
                    incoming = json.loads(line); envelope = incoming['envelope']; op = envelope['operation_id']
                    packet = issuance/('p3-'+op[-32:]); packet.mkdir(mode=0o700)
                    (issuance/'p3-issuance.once').write_text(packet.name+'\n')
                    (packet/'complete').write_text('Packet complete; runtime and cluster are not qualified.\n')
                    (packet/'receipt.json').write_text(json.dumps(envelope['issuer_receipt']))
                    (packet/'objects.json').write_text(json.dumps(envelope['objects']))
                    for path in (issuance/'p3-issuance.once',*packet.iterdir()): path.chmod(0o600)
                    self.assertEqual(stream.read(1),b'R')
                    with patch.object(exporter,'ROOT',issuance), patch.object(exporter,'_socket_path',return_value=Path(incoming['socket'])), \
                         patch.object(exporter,'_check_root_custody'), patch.object(exporter,'_pmd_uid',return_value=1000):
                        exporter.export(op,envelope['source_sha'],pid,exporter._start_ticks(pid))
                    self.assertEqual(stream.read(1),b'Y'); finished = True
            finally:
                if not finished:
                    try: os.kill(pid,signal.SIGTERM)
                    except ProcessLookupError: pass
                _, status = os.waitpid(pid,0)
            self.assertEqual(status,0)


if __name__ == '__main__': unittest.main()
