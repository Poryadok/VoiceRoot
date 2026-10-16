import copy
import json
import os
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

    def test_selected_store_requires_orderly_exit_not_only_running_false(self):
        self.runtime.owned['broker']['selected']={'path':'selected'}
        for code,oom in ((137,False),(1,False),(0,True)):
            with self.subTest(exit_code=code,oom=oom):
                self.row['State']={'Running':False,'ExitCode':code,'OOMKilled':oom}
                with patch.object(self.runtime,'inspect',return_value=self.row):
                    with self.assertRaisesRegex(Blocked,'broker_orderly_shutdown_failed'):
                        self.runtime.stop('broker')

    def test_selected_store_successful_shutdown_is_accepted(self):
        self.runtime.owned['broker']['selected']={'path':'selected'}
        self.row['State']={'Running':False,'ExitCode':0,'OOMKilled':False,'Error':''}
        with patch.object(self.runtime,'inspect',return_value=self.row):
            self.runtime.stop('broker')

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

    def test_second_runtime_kernel_preserves_previous_proof_output(self):
        base = Path(self.tmp.name)
        previous = base/'out-5'; previous.mkdir(mode=0o700)
        sentinel = previous/'census.json'; sentinel.write_bytes(b'previous-proof')
        outputs = []
        for _ in range(2):
            runtime = DockerRuntime(base, 'abcd1234', lambda args, timeout=60: '0')
            runtime.owned = {str(i): {} for i in range(5)}
            with patch.object(runtime, 'create', return_value='kernel'), patch.object(runtime, 'inspect', return_value={'State': {'Running': False}}):
                outputs.append(runtime.kernel('broker', 'census'))
        self.assertEqual(sentinel.read_bytes(), b'previous-proof')
        self.assertNotEqual(outputs[0], outputs[1])
        for output in outputs:
            self.assertEqual(output.parent, base)
            self.assertNotEqual(output, previous)
            self.assertEqual(output.stat().st_uid, 65532)
            self.assertEqual(output.stat().st_mode & 0o777, 0o700)

    def test_old_output_symlink_is_never_reused(self):
        base = Path(self.tmp.name)
        outside = base/'outside'; outside.mkdir()
        (base/'out-5').symlink_to(outside, target_is_directory=True)
        runtime = DockerRuntime(base, 'abcd1234', lambda args, timeout=60: '0')
        runtime.owned = {str(i): {} for i in range(5)}
        with patch.object(runtime, 'create', return_value='kernel'), patch.object(runtime, 'inspect', return_value={'State': {'Running': False}}):
            output = runtime.kernel('broker', 'census')
        self.assertEqual(output.parent, base)
        self.assertNotEqual(output.resolve(), outside)
        self.assertEqual(list(outside.iterdir()), [])


if __name__ == '__main__': unittest.main()
