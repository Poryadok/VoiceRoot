import copy
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

from controller import Blocked
from docker_runtime import DockerRuntime, LABEL, NATS_IMAGE


class DockerSafetyTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(); self.addCleanup(self.tmp.cleanup)
        self.calls = []
        self.row = {'Id': 'a'*64, 'Image': 'sha256:'+'b'*64,
            'Config': {'Labels': {LABEL: 'abcd1234'}, 'Image': NATS_IMAGE, 'User': '65532:65532'},
            'HostConfig': {'NetworkMode': 'none', 'Privileged': False,
                'ReadonlyRootfs': True, 'CapAdd': None, 'CapDrop': ['ALL'],
                'PortBindings': {}, 'SecurityOpt': ['no-new-privileges:true']},
            'Mounts': [], 'State': {'Running': False}}
        def run(args, timeout=60):
            self.calls.append(args)
            return json.dumps([self.row])
        self.runtime = DockerRuntime(self.tmp.name, 'abcd1234', run)
        self.runtime.owned['broker'] = {'id': 'a'*64, 'image': NATS_IMAGE,
            'image_id': 'sha256:'+'b'*64, 'network': 'none', 'mounts': set()}

    def test_foreign_container_never_stopped(self):
        self.row['Config']['Labels'][LABEL] = 'someoneelse'
        with self.assertRaises(Blocked): self.runtime.stop('broker')
        self.assertFalse(any(c[0]=='stop' for c in self.calls))

    def test_exposed_network_rejected_before_start(self):
        self.row['HostConfig']['NetworkMode'] = 'host'
        with self.assertRaises(Blocked): self.runtime.restart('broker')
        self.assertFalse(any(c[0]=='start' for c in self.calls))

    def test_unexpected_rw_mount_rejected_before_start(self):
        self.row['Mounts'] = [{'Type': 'bind', 'Source': '/var/lib/rancher/k3s', 'Destination': '/data', 'RW': True}]
        with self.assertRaises(Blocked): self.runtime.restart('broker')
        self.assertFalse(any(c[0]=='start' for c in self.calls))

    def test_running_broker_never_read_for_archive(self):
        self.row['State']['Running'] = True
        with patch('docker_runtime.archive_closed_store') as archive:
            with self.assertRaises(Blocked):
                self.runtime.cold_archive('broker', '/anything', '/archive')
            archive.assert_not_called()


if __name__ == '__main__': unittest.main()
