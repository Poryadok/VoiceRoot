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

import realtime_p3_create_only_socket as channel


class ChannelTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(); self.addCleanup(self.tmp.cleanup)
        self.path = str(Path(self.tmp.name)/'packet.sock')
        self.listener = socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET); self.addCleanup(self.listener.close)
        self.listener.bind(self.path); self.listener.listen(1)
        self.operation = 'media-c2-'+'a'*32; self.source = 'b'*40
        self.envelope = {'operation_id':self.operation,'source_sha':self.source,'issuer_receipt':{},'objects':{}}

    def memfd(self, data, sealed=True):
        fd = os.memfd_create('fixture',os.MFD_ALLOW_SEALING); self.addCleanup(os.close,fd)
        os.fchmod(fd,0o600); os.write(fd,data)
        if sealed: fcntl.fcntl(fd,fcntl.F_ADD_SEALS,channel.REQUIRED_SEALS)
        return fd

    def send(self, fds, payload=channel.FRAME):
        def worker():
            with socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET) as sock:
                sock.connect(self.path)
                ancillary = [(socket.SOL_SOCKET,socket.SCM_RIGHTS,array.array('i',fds))] if fds else []
                sock.sendmsg([payload],ancillary)
        thread = threading.Thread(target=worker); thread.start(); self.addCleanup(thread.join); return thread

    def test_root_peer_single_sealed_fd(self):
        fd = self.memfd(json.dumps(self.envelope).encode()); self.send([fd])
        original = os.pread; inheritable = []
        def check(received, size, offset):
            inheritable.append(os.get_inheritable(received)); return original(received,size,offset)
        with patch.object(channel.os,'pread',side_effect=check):
            self.assertEqual(channel.receive(self.listener,self.operation,self.source),self.envelope)
        self.assertTrue(inheritable); self.assertFalse(any(inheritable))

    def test_unsealed_extra_fd_wrong_frame_and_rebinding_fail(self):
        for fault in ('unsealed','extra','frame','source'):
            with self.subTest(fault=fault):
                self.setUp(); fd = self.memfd(json.dumps(self.envelope).encode(),sealed=fault!='unsealed')
                self.send([fd,fd] if fault == 'extra' else [fd],b'WRONG' if fault == 'frame' else channel.FRAME)
                with self.assertRaises(channel.PacketError):
                    channel.receive(self.listener,self.operation,'f'*40 if fault == 'source' else self.source)

    def test_nonroot_peer_is_rejected(self):
        os.chmod(self.tmp.name,0o777); os.chmod(self.path,0o777)
        child = os.fork()
        if child == 0:
            try:
                os.setuid(65534)
                with socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET) as sock:
                    sock.connect(self.path); sock.send(channel.FRAME)
            finally: os._exit(0)
        try:
            with self.assertRaises(channel.PacketError): channel.receive(self.listener,self.operation,self.source)
        finally: os.waitpid(child,0)

    def test_truncated_ancillary_does_not_leak_received_descriptors(self):
        fd = self.memfd(json.dumps(self.envelope).encode())
        before = len(os.listdir('/proc/self/fd')); thread = self.send([fd]*12)
        with self.assertRaises(channel.PacketError): channel.receive(self.listener,self.operation,self.source)
        thread.join()
        self.assertEqual(len(os.listdir('/proc/self/fd')),before)


if __name__ == '__main__': unittest.main()
