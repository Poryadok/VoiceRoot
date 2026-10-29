import array
import fcntl
import json
import os
from pathlib import Path
import socket
import tempfile
import threading
import unittest
from unittest.mock import patch

import space_creator_runtime_profile_issuer_export as export

@unittest.skipUnless(os.geteuid()==0,'isolated root fixture required')
class ExportTest(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory(); self.addCleanup(self.tmp.cleanup); self.root=Path(self.tmp.name)
        self.op='media-c1-'+'a'*32; self.path=self.root/('space-creator-'+'a'*32); self.path.mkdir(mode=0o700)
        reservation=self.root/'space-creator-issuance.once'; reservation.write_text(self.path.name+'\n'); os.chmod(reservation,0o600)
        for name,data in {'complete':b'Creator profile issuance complete; runtime not qualified.\n',
                          'media-public.crt':b'fixture','objects.json':json.dumps({'issuer_receipt':{'object_names':['voice-role-media-server-tls']}}).encode()}.items():
            (self.path/name).write_bytes(data); os.chmod(self.path/name,0o600)
        self.sock=self.root/'packet.sock'; self.server=socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET)
        self.server.bind(str(self.sock)); self.server.listen(1); self.addCleanup(self.server.close)
        self.received=[]
        def receive():
            conn,_=self.server.accept()
            with conn:
                message,anc,_,_=conn.recvmsg(128,socket.CMSG_SPACE(array.array('i').itemsize))
                if not anc: return
                fds=array.array('i'); fds.frombytes(anc[0][2][:fds.itemsize]); fd=fds[0]
                try: self.received.append((message,fcntl.fcntl(fd,fcntl.F_GET_SEALS),os.read(fd,524288)))
                finally: os.close(fd)
        self.thread=threading.Thread(target=receive,daemon=True); self.thread.start()
        for change in (patch.object(export,'ROOT',self.root),patch.object(export,'_socket_path',return_value=self.sock),
                       patch.object(export.existing,'_check_root_custody'),patch.object(export.existing,'_pmd_uid',return_value=0),
                       patch.object(export.packet,'validate')):
            change.start(); self.addCleanup(change.stop)

    def test_real_sealed_fd_and_no_replay(self):
        start=export.existing._start_ticks(os.getpid())
        result=export.export(self.op,'b'*40,'namespace','c'*64,os.getpid(),start)
        self.thread.join(timeout=5); self.assertFalse(self.thread.is_alive())
        self.assertEqual(result['status'],'send_attempted'); self.assertEqual(self.received[0][0],export.FRAME)
        self.assertEqual(self.received[0][1]&export.existing.REQUIRED_SEALS,export.existing.REQUIRED_SEALS)
        self.assertEqual(json.loads(self.received[0][2])['issuer_receipt']['object_names'],['voice-role-media-server-tls'])
        with self.assertRaises(export.ExportError): export.export(self.op,'b'*40,'namespace','c'*64,os.getpid(),start)

    def test_peer_pid_mismatch_prevents_handoff(self):
        with self.assertRaises(export.ExportError): export.export(self.op,'b'*40,'namespace','c'*64,os.getpid()+1,1)
        self.thread.join(timeout=5); self.assertFalse((self.path/'handoff-started').exists())

if __name__=='__main__': unittest.main()
