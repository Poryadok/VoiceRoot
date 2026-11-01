"""Ownership and cleanup invariants for local disposable Docker fixtures."""
from pathlib import Path
import hashlib
import io
import json
import tempfile
import unittest
from unittest import mock

from local_docker_owned import DockerCLI, OwnedRun, OwnershipError
import local_rootless_profile as rootless


class FakeEngine:
    def __init__(self):
        self.identity = 'daemon-a'
        self.resources = {}
        self.removed = []
        self.uncertain_create = False
        self.uncertain_container_create = False
        self.uncertain_remove = False
        self.uncertain_start = False
        self.uncertain_exec = False
        self.uncertain_stop = False
        self.rootless_profile_valid = True
        self.rootless_verifications = 0

    def daemon(self):
        return self.identity

    def create(self, kind, name, labels):
        identifier = name if kind == 'volume' else ('a' if kind == 'network' else 'b') + name.encode().hex().ljust(63, '0')[:63]
        self.resources[(kind, name)] = {'id': identifier, 'name': name, 'labels': dict(labels), 'kind': kind,
                                        'internal': kind == 'network'}
        if self.uncertain_create:
            raise OwnershipError('create outcome unknown')
        return identifier

    def inspect(self, kind, name):
        return dict(self.resources[(kind, name)])

    def remove(self, kind, identifier):
        found = next(key for key, value in self.resources.items() if key[0] == kind and value['id'] == identifier)
        self.removed.append(found)
        del self.resources[found]
        if self.uncertain_remove:
            raise OwnershipError('remove outcome unknown')

    def exists(self, kind, identifier):
        return any(key[0] == kind and value['id'] == identifier for key, value in self.resources.items())

    def image(self, identifier):
        if identifier == rootless.IMAGE:
            return {'id': identifier, 'volumes': ['/home/rootless/.local/share/docker', '/var/lib/docker'],
                    'entrypoint': ['dockerd-entrypoint.sh']}
        return {'id': identifier, 'volumes': ['/data'], 'entrypoint': []}

    def verify_rootless(self, item, labels, running):
        self.rootless_verifications += 1
        if not self.rootless_profile_valid:
            raise OwnershipError('rootless profile differs')

    def create_container(self, name, labels, image_id, network_id, mounts, ports, command=None):
        identifier = 'c' + name.encode().hex().ljust(63, '0')[:63]
        self.resources[('container', name)] = {'id': identifier, 'name': name, 'kind': 'container',
                                               'labels': dict(labels), 'image': image_id, 'network': network_id,
                                               'mounts': dict(mounts), 'ports': list(ports), 'running': False,
                                               'command': command, 'entrypoint': []}
        if self.uncertain_container_create:
            raise OwnershipError('container create outcome unknown')
        return identifier

    def start(self, identifier):
        value = next(value for key, value in self.resources.items() if key[0] == 'container' and value['id'] == identifier)
        value['running'] = True
        if self.uncertain_start:
            raise OwnershipError('start outcome unknown')

    def execute(self, identifier, command, stdout, stderr, timeout, stdin=None):
        if self.uncertain_exec:
            raise OwnershipError('execution outcome unknown')
        if stdin is not None:
            self.last_input = stdin.read()
        return 7

    def stop(self, identifier):
        value = next(value for key, value in self.resources.items() if key[0] == 'container' and value['id'] == identifier)
        value['running'] = False
        if self.uncertain_stop:
            raise OwnershipError('stop outcome unknown')


class RootlessEndpointTests(unittest.TestCase):
    def test_verified_stream_hashes_exact_partial_pipe_writes(self):
        class Sink:
            def __init__(self):
                self.data = bytearray()
                self.closed = False
            def write(self, data):
                amount = min(3, len(data))
                self.data.extend(data[:amount])
                return amount
            def close(self):
                self.closed = True
        class Process:
            def __init__(self):
                self.stdin = Sink()
            def wait(self, timeout=None):
                return 0
            def poll(self):
                return 0
        engine = DockerCLI.__new__(DockerCLI)
        engine._env = {}
        payload = b'exact bytes across partial writes'
        process = Process()
        with mock.patch('local_docker_owned.subprocess.Popen', return_value=process):
            self.assertEqual(engine.execute_verified_stream('id', ['load'], None, None, 10,
                                                            io.BytesIO(payload), hashlib.sha256(payload).hexdigest()), 0)
        self.assertEqual(bytes(process.stdin.data), payload)
        process = Process()
        with mock.patch('local_docker_owned.subprocess.Popen', return_value=process), self.assertRaises(OwnershipError):
            engine.execute_verified_stream('id', ['load'], None, None, 10, io.BytesIO(payload), '0' * 64)

    def test_verified_stream_reaps_client_on_interruption_and_watchdog_start_failure(self):
        class InterruptedInput:
            def read(self, size):
                raise KeyboardInterrupt()
        class Process:
            def __init__(self):
                self.stdin = io.BytesIO()
                self.killed = False
                self.waited = False
            def poll(self):
                return -9 if self.killed else None
            def kill(self):
                self.killed = True
            def wait(self, timeout=None):
                self.waited = True
                return -9
        engine = DockerCLI.__new__(DockerCLI)
        engine._env = {}
        process = Process()
        with mock.patch('local_docker_owned.subprocess.Popen', return_value=process), self.assertRaises(KeyboardInterrupt):
            engine.execute_verified_stream('id', ['load'], None, None, 10, InterruptedInput(), '0' * 64)
        self.assertTrue(process.killed and process.waited)
        process = Process()
        with mock.patch('local_docker_owned.subprocess.Popen', return_value=process), \
                mock.patch('local_docker_owned.threading.Timer.start', side_effect=RuntimeError('unavailable')), \
                self.assertRaises(OwnershipError):
            engine.execute_verified_stream('id', ['load'], None, None, 10, io.BytesIO(b''), '0' * 64)
        self.assertTrue(process.killed and process.waited)
        with mock.patch('local_docker_owned.subprocess.Popen') as popen, \
                mock.patch('local_docker_owned.threading.Timer', side_effect=RuntimeError('unavailable')), \
                self.assertRaises(OwnershipError):
            engine.execute_verified_stream('id', ['load'], None, None, 10, io.BytesIO(b''), '0' * 64)
        popen.assert_not_called()

    def engine(self, env, context=None):
        engine = DockerCLI.__new__(DockerCLI)
        engine._env = dict(env)
        engine._call = lambda *args: json.dumps(context)
        return engine

    def test_context_is_resolved_to_explicit_local_socket(self):
        endpoint = 'npipe:////./pipe/dockerDesktopLinuxEngine'
        context = [{'Name': 'desktop-linux', 'Endpoints': {'docker': {'Host': endpoint, 'SkipTLSVerify': False}},
                    'TLSMaterial': {}}]
        env = self.engine({'DOCKER_CONTEXT': 'desktop-linux', 'PATH': 'keep'}, context).rootless_creator_env()
        self.assertEqual(env['DOCKER_HOST'], endpoint)
        self.assertNotIn('DOCKER_CONTEXT', env)
        self.assertEqual(env['PATH'], 'keep')

    def test_rejects_remote_tls_ambiguous_and_invalid_contexts(self):
        local = 'npipe:////./pipe/dockerDesktopLinuxEngine'
        cases = [
            ({'DOCKER_HOST': 'tcp://127.0.0.1:2375'}, None),
            ({'DOCKER_HOST': local, 'DOCKER_CONTEXT': 'desktop-linux'}, None),
            ({'DOCKER_HOST': local, 'DOCKER_TLS_VERIFY': '1'}, None),
            ({'DOCKER_HOST': 'unix:///tmp/../docker.sock'}, None),
            ({'DOCKER_CONTEXT': 'desktop-linux'}, [{'Name': 'other', 'Endpoints': {'docker': {'Host': local}}, 'TLSMaterial': {}}]),
            ({'DOCKER_CONTEXT': 'desktop-linux'}, [{'Name': 'desktop-linux', 'Endpoints': {'docker': {'Host': local, 'SkipTLSVerify': True}}, 'TLSMaterial': {}}]),
            ({'DOCKER_CONTEXT': 'desktop-linux'}, [{'Name': 'desktop-linux', 'Endpoints': {'docker': {'Host': local}}, 'TLSMaterial': {'docker': ['cert.pem']}}]),
        ]
        for env, context in cases:
            with self.subTest(env=env, context=context), self.assertRaises(OwnershipError):
                self.engine(env, context).rootless_creator_env()

    def test_explicit_local_unix_socket_is_retained(self):
        env = self.engine({'DOCKER_HOST': 'unix:///var/run/docker.sock'}).rootless_creator_env()
        self.assertEqual(env['DOCKER_HOST'], 'unix:///var/run/docker.sock')


class OwnedRunTests(unittest.TestCase):
    def test_phase0_profile_requires_owned_readonly_source_and_no_network(self):
        with tempfile.TemporaryDirectory() as directory:
            engine = FakeEngine()
            engine.image = lambda identifier: {'id': identifier, 'volumes': [], 'entrypoint': []}
            def create(name, labels, image, volume, command):
                identifier = 'c' + name.encode().hex().ljust(63, '0')[:63]
                engine.resources[('container', name)] = {
                    'id': identifier, 'name': name, 'kind': 'container', 'labels': dict(labels),
                    'image': image, 'network': 'none', 'mounts': {'/workspace/repo': volume},
                    'readonly_mounts': ['/workspace/repo'], 'ports': [], 'running': False,
                    'command': command, 'entrypoint': [], 'user': '0', 'network_attachments': ['none']}
                return identifier
            engine.create_phase0_container = create
            owner = OwnedRun.create(Path(directory), engine)
            source = owner.create_resource('volume')
            name = owner.create_phase0_container('sha256:' + '1' * 64, source)
            item = next(row for row in owner.state['resources'] if row['name'] == name)
            self.assertEqual(item['network'], 'none')
            self.assertEqual(item['readonly_mounts'], ['/workspace/repo'])
            owner._inspect(item)
            engine.resources[('container', name)]['readonly_mounts'] = []
            with self.assertRaises(OwnershipError):
                owner._inspect(item)
            engine.resources[('container', name)]['readonly_mounts'] = ['/workspace/repo']
            engine.resources[('container', name)]['network_attachments'] = ['other']
            with self.assertRaises(OwnershipError):
                owner._inspect(item)
            engine.resources[('container', name)]['network_attachments'] = ['none']
            engine.resources[('container', name)]['user'] = '1000'
            with self.assertRaises(OwnershipError):
                owner._inspect(item)
            engine.resources[('container', name)]['user'] = '0'
            item['readonly_mounts'] = []
            with self.assertRaises(OwnershipError):
                owner._validate()
            item['readonly_mounts'] = ['/workspace/repo']
            item['network'] = 'host'
            with self.assertRaises(OwnershipError):
                owner._validate()
            item['network'] = 'none'
            engine.resources[('container', name)]['readonly_mounts'] = ['/workspace/repo']
            engine.resources[('container', name)]['network'] = 'host'
            with self.assertRaises(OwnershipError):
                owner._inspect(item)
            with self.assertRaises(OwnershipError):
                owner.create_phase0_container('sha256:' + '1' * 64, 'foreign-volume')
            owner.close()

    def test_phase0_lost_create_and_failed_inspection_reconcile_only_exact_profile(self):
        for outcome in ('lost', 'wrong_mount'):
            with self.subTest(outcome=outcome), tempfile.TemporaryDirectory() as directory:
                engine = FakeEngine()
                engine.image = lambda identifier: {'id': identifier, 'volumes': [], 'entrypoint': []}
                def create(name, labels, image, volume, command):
                    identifier = 'c' + name.encode().hex().ljust(63, '0')[:63]
                    engine.resources[('container', name)] = {
                        'id': identifier, 'name': name, 'kind': 'container', 'labels': dict(labels),
                        'image': image, 'network': 'none', 'mounts': {'/workspace/repo': volume},
                        'readonly_mounts': [] if outcome == 'wrong_mount' else ['/workspace/repo'],
                        'ports': [], 'running': False, 'command': command, 'entrypoint': [],
                        'user': '0', 'network_attachments': ['none']}
                    if outcome == 'lost':
                        raise OwnershipError('create outcome unknown')
                    return identifier
                engine.create_phase0_container = create
                owner = OwnedRun.create(Path(directory), engine)
                try:
                    source = owner.create_resource('volume')
                    with self.assertRaises(OwnershipError):
                        owner.create_phase0_container('sha256:' + '1' * 64, source)
                    item = owner.state['resources'][-1]
                    self.assertEqual((item['state'], item['id']), ('creating', ''))
                    if outcome == 'wrong_mount':
                        with self.assertRaises(OwnershipError):
                            owner.reconcile_creation(item['name'])
                        engine.resources[('container', item['name'])]['readonly_mounts'] = ['/workspace/repo']
                    owner.reconcile_creation(item['name'])
                    self.assertEqual(item['state'], 'ready')
                finally:
                    owner.close()
    def test_canonical_image_load_discards_output_on_success_and_failure(self):
        from unittest.mock import patch
        from local_image_archive_test import oci_fixture
        raw, digest, image_id = oci_fixture(extra_blob=True, pax=True)
        with tempfile.TemporaryDirectory() as directory:
            owner = OwnedRun.create(Path(directory), FakeEngine())
            try:
                archive = owner.directory / 'image.tar'
                archive.write_bytes(raw)
                def load(name, path, expected, expected_id, stdout, stderr, timeout):
                    self.assertEqual(hashlib.sha256(path.read_bytes()).hexdigest(), expected)
                    self.assertEqual(expected_id, image_id)
                    self.assertNotEqual(path.read_bytes(), raw)
                with patch.object(owner, 'load_rootless_image', side_effect=load) as operation:
                    owner.load_canonical_rootless_image('daemon', archive, digest, image_id,
                                                        'alpine:3.17', None, None)
                    self.assertEqual(operation.call_count, 1)
                    with self.assertRaises(OwnershipError):
                        owner.load_canonical_rootless_image('daemon', archive, '0' * 64, image_id,
                                                            'alpine:3.17', None, None)
                    self.assertEqual(operation.call_count, 1)
                    foreign, foreign_digest, foreign_id = oci_fixture(bad_tag=True)
                    archive.write_bytes(foreign)
                    with self.assertRaises(OwnershipError):
                        owner.load_canonical_rootless_image('daemon', archive, foreign_digest, foreign_id,
                                                            'alpine:3.17', None, None)
                    self.assertEqual(operation.call_count, 1)
                archive.write_bytes(raw)
                self.assertEqual(list(owner.directory.glob('canonical-*.tar')), [])
                def partial_failure(source, output, *args):
                    output.write(b'partial archive')
                    raise ValueError('source changed')
                with patch('local_image_archive.sanitize_archive', side_effect=partial_failure):
                    with patch.object(owner, 'load_rootless_image') as operation:
                        with self.assertRaises(OwnershipError):
                            owner.load_canonical_rootless_image('daemon', archive, digest, image_id,
                                                                'alpine:3.17', None, None)
                        operation.assert_not_called()
                self.assertEqual(list(owner.directory.glob('canonical-*.tar')), [])
                with patch.object(owner, 'load_rootless_image', side_effect=OwnershipError('load failed')):
                    with self.assertRaisesRegex(OwnershipError, 'load failed'):
                        owner.load_canonical_rootless_image('daemon', archive, digest, image_id,
                                                            'alpine:3.17', None, None)
                self.assertEqual(list(owner.directory.glob('canonical-*.tar')), [])
                with patch.object(owner, 'load_rootless_image', side_effect=OwnershipError('load failed')):
                    with patch.object(Path, 'unlink', side_effect=OSError('locked')):
                        with self.assertRaisesRegex(OwnershipError, 'load failed') as error:
                            owner.load_canonical_rootless_image('daemon', archive, digest, image_id,
                                                                'alpine:3.17', None, None)
                self.assertIn('cleanup failed', error.exception.__notes__[0])
            finally:
                owner.close()

    def test_rootless_image_load_rejects_preexisting_hidden_parent_image(self):
        class HiddenParentEngine(FakeEngine):
            def execute(self, identifier, command, stdout, stderr, timeout, stdin=None):
                if 'ls' not in command:
                    raise AssertionError('unexpected image operation')
                if '--all' in command:
                    stdout.write(('sha256:' + 'e' * 64 + '\n').encode())
                return 0

            def execute_verified_stream(self, *args):
                raise AssertionError('preexisting image must not be loaded')

        with tempfile.TemporaryDirectory() as directory:
            engine = HiddenParentEngine()
            owner = OwnedRun.create(Path(directory), engine)
            try:
                network = owner.create_resource('network')
                mounts = {target: owner.create_resource('volume') for target in sorted(rootless.TARGETS)}
                def creator(request):
                    item = owner.state['resources'][-1]
                    labels = {'voice.ci.run': owner.run_id, 'voice.ci.resource': item['nonce'],
                              'voice.ci.kind': 'container'}
                    return engine.create_container(item['name'], labels, rootless.IMAGE, item['network'], mounts, [])
                name = owner.create_rootless_container(network, mounts, creator)
                owner.start_container(name)
                archive = owner.directory / 'image.tar'
                archive.write_bytes(b'archive')
                with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err, self.assertRaises(OwnershipError):
                    owner.load_rootless_image(name, archive, hashlib.sha256(b'archive').hexdigest(),
                                              'sha256:' + 'e' * 64, out, err)
                self.assertEqual(owner.state['resources'][-1]['state'], 'executing')
                owner.cancel_execution(name)
                self.assertTrue(all(item['state'] == 'quarantined' for item in owner.state['resources']
                                    if item['kind'] == 'volume'))
                owner.cleanup()
            finally:
                owner.close()

    def test_rootless_image_load_is_journaled_and_verifies_content_and_image(self):
        class LoadEngine(FakeEngine):
            def __init__(self):
                super().__init__()
                self.calls = []
                self.loaded = None

            def execute(self, identifier, command, stdout, stderr, timeout, stdin=None):
                self.calls.append(command)
                if self.uncertain_exec:
                    raise OwnershipError('execution outcome unknown')
                if 'ls' in command:
                    if self.loaded is not None:
                        stdout.write(('sha256:' + 'e' * 64 + '\n').encode())
                    return 0
                if self.loaded is None:
                    return 1
                stdout.write(('sha256:' + 'e' * 64 + '\n').encode())
                return 0

            def execute_verified_stream(self, identifier, command, stdout, stderr, timeout, stdin, expected):
                self.calls.append(command)
                if self.uncertain_exec:
                    raise OwnershipError('execution outcome unknown')
                data = stdin.read()
                if hashlib.sha256(data).hexdigest() != expected:
                    raise OwnershipError('streamed image archive differs')
                self.loaded = data
                return 0

        with tempfile.TemporaryDirectory() as directory:
            engine = LoadEngine()
            owner = OwnedRun.create(Path(directory), engine)
            try:
                network = owner.create_resource('network')
                mounts = {target: owner.create_resource('volume') for target in sorted(rootless.TARGETS)}
                def creator(request):
                    item = owner.state['resources'][-1]
                    labels = {'voice.ci.run': owner.run_id, 'voice.ci.resource': item['nonce'],
                              'voice.ci.kind': 'container'}
                    return engine.create_container(item['name'], labels, rootless.IMAGE, item['network'], mounts, [])
                name = owner.create_rootless_container(network, mounts, creator)
                owner.start_container(name)
                archive = owner.directory / 'image.tar'
                archive.write_bytes(b'owned image archive')
                digest = hashlib.sha256(archive.read_bytes()).hexdigest()
                with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
                    with self.assertRaises(OwnershipError):
                        owner.load_rootless_image(name, archive, '0' * 64, 'sha256:' + 'e' * 64, out, err)
                    self.assertEqual(engine.calls, [])
                    owner.load_rootless_image(name, archive, digest, 'sha256:' + 'e' * 64, out, err)
                    self.assertEqual(engine.loaded, b'owned image archive')
                    engine.loaded = None
                    engine.uncertain_exec = True
                    with self.assertRaises(OwnershipError):
                        owner.load_rootless_image(name, archive, digest, 'sha256:' + 'e' * 64, out, err)
                self.assertEqual(owner.state['resources'][-1]['state'], 'executing')
                engine.uncertain_exec = False
                owner.cancel_execution(name)
                self.assertEqual(owner.state['resources'][-1]['state'], 'quarantined')
                self.assertTrue(all(item['state'] == 'quarantined' for item in owner.state['resources']
                                    if item['kind'] == 'volume'))
                with self.assertRaises(OwnershipError):
                    owner.start_container(name)
                with self.assertRaises(OwnershipError):
                    owner.create_rootless_container(network, mounts, creator)
                with self.assertRaises(OwnershipError):
                    owner.create_container('sha256:' + 'd' * 64, network,
                                           {'/data': mounts['/run/user/1000']}, [], command=['sleep', '120'])
                owner.cleanup()
            finally:
                owner.close()

    def test_failed_resource_inspection_keeps_reconcilable_creation(self):
        class ChangedNetworkEngine(FakeEngine):
            def create(self, kind, name, labels):
                identifier = super().create(kind, name, labels)
                self.resources[(kind, name)]['internal'] = False
                return identifier

        with tempfile.TemporaryDirectory() as directory:
            engine = ChangedNetworkEngine()
            owner = OwnedRun.create(Path(directory), engine)
            try:
                with self.assertRaises(OwnershipError):
                    owner.create_resource('network')
                item = owner.state['resources'][-1]
                self.assertEqual((item['state'], item['id']), ('creating', ''))
                engine.resources[('network', item['name'])]['internal'] = True
                owner.reconcile_creation(item['name'])
                owner.cleanup()
            finally:
                owner.close()

    def test_explicit_entrypoint_override_is_verified_before_start(self):
        class EntrypointEngine(FakeEngine):
            def image(self, identifier):
                return {'id': identifier, 'volumes': [], 'entrypoint': ['/original-entrypoint']}

            def create_container(self, name, labels, image_id, network_id, mounts, ports, command=None,
                                 clear_entrypoint=False):
                self.assert_override = clear_entrypoint
                return super().create_container(name, labels, image_id, network_id, mounts, ports, command)

        with tempfile.TemporaryDirectory() as directory:
            engine = EntrypointEngine()
            owner = OwnedRun.create(Path(directory), engine)
            try:
                network = owner.create_resource('network')
                image = 'sha256:' + 'd' * 64
                with self.assertRaises(OwnershipError):
                    owner.create_container(image, network, {}, [], clear_entrypoint=True)
                with self.assertRaises(OwnershipError):
                    owner.create_container(image, network, {}, [], command=['sleep', '120'],
                                           clear_entrypoint='yes')
                with self.assertRaises(OwnershipError):
                    owner.create_container(image, network, {}, [], command=['sleep', '120'])
                name = owner.create_container(image, network, {}, [], command=['sleep', '120'],
                                              clear_entrypoint=True)
                self.assertTrue(engine.assert_override)
                engine.resources[('container', name)]['entrypoint'] = ['/unexpected']
                with self.assertRaises(OwnershipError):
                    owner.start_container(name)
                engine.resources[('container', name)]['entrypoint'] = []
                owner.start_container(name)
                owner.cleanup()
            finally:
                owner.close()

    def test_entrypoint_override_recovery_rejects_unexpected_entrypoint(self):
        class EntrypointEngine(FakeEngine):
            def image(self, identifier):
                return {'id': identifier, 'volumes': [], 'entrypoint': ['/original-entrypoint']}

            def create_container(self, name, labels, image_id, network_id, mounts, ports, command=None,
                                 clear_entrypoint=False):
                if not clear_entrypoint:
                    raise AssertionError('explicit override was not requested')
                return super().create_container(name, labels, image_id, network_id, mounts, ports, command)

        with tempfile.TemporaryDirectory() as directory:
            engine = EntrypointEngine()
            owner = OwnedRun.create(Path(directory), engine)
            try:
                network = owner.create_resource('network')
                engine.uncertain_container_create = True
                with self.assertRaises(OwnershipError):
                    owner.create_container('sha256:' + 'd' * 64, network, {}, [],
                                           command=['sleep', '120'], clear_entrypoint=True)
                name = owner.state['resources'][-1]['name']
                engine.resources[('container', name)]['entrypoint'] = ['/unexpected']
                with self.assertRaises(OwnershipError):
                    owner.reconcile_creation(name)
                self.assertEqual(owner.state['resources'][-1]['state'], 'creating')
                engine.resources[('container', name)]['entrypoint'] = []
                owner.reconcile_creation(name)
                owner.cleanup()
            finally:
                owner.close()

    def test_entrypoint_inspection_failure_keeps_reconcilable_creation(self):
        class EntrypointEngine(FakeEngine):
            def image(self, identifier):
                return {'id': identifier, 'volumes': [], 'entrypoint': ['/original-entrypoint']}

            def create_container(self, name, labels, image_id, network_id, mounts, ports, command=None,
                                 clear_entrypoint=False):
                identifier = super().create_container(name, labels, image_id, network_id, mounts, ports, command)
                self.resources[('container', name)]['entrypoint'] = ['/unexpected']
                return identifier

        with tempfile.TemporaryDirectory() as directory:
            engine = EntrypointEngine()
            owner = OwnedRun.create(Path(directory), engine)
            try:
                network = owner.create_resource('network')
                with self.assertRaises(OwnershipError):
                    owner.create_container('sha256:' + 'd' * 64, network, {}, [],
                                           command=['sleep', '120'], clear_entrypoint=True)
                item = owner.state['resources'][-1]
                self.assertEqual((item['state'], item['id']), ('creating', ''))
                engine.resources[('container', item['name'])]['entrypoint'] = []
                owner.reconcile_creation(item['name'])
                owner.cleanup()
            finally:
                owner.close()

    def test_rootless_rejects_foreign_or_aliased_volumes_before_creator(self):
        with tempfile.TemporaryDirectory() as directory:
            engine = FakeEngine()
            owner = OwnedRun.create(Path(directory), engine)
            try:
                network = owner.create_resource('network')
                mounts = {target: owner.create_resource('volume') for target in sorted(rootless.TARGETS)}
                variants = []
                foreign = dict(mounts)
                foreign['/run/user/1000'] = 'foreign-volume'
                variants.append(foreign)
                aliased = dict(mounts)
                aliased['/run/user/1000'] = aliased['/var/lib/docker']
                variants.append(aliased)
                for variant in variants:
                    with self.subTest(mounts=variant), self.assertRaises(OwnershipError):
                        owner.create_rootless_container(network, variant,
                                                       lambda request: self.fail('creator must not run'))
                self.assertEqual(len(owner.state['resources']), 4)
                owner.cleanup()
            finally:
                owner.close()

    def test_rootless_creation_is_journaled_and_profile_checked_before_start(self):
        with tempfile.TemporaryDirectory() as directory:
            engine = FakeEngine()
            owner = OwnedRun.create(Path(directory), engine)
            try:
                network = owner.create_resource('network')
                mounts = {target: owner.create_resource('volume') for target in sorted(rootless.TARGETS)}

                def creator(request):
                    persisted = json.loads((owner.directory / 'ownership.json').read_text())
                    entry = persisted['resources'][-1]
                    self.assertEqual((entry['state'], entry['id'], entry['profile']),
                                     ('creating', '', 'rootless-unix-v1'))
                    self.assertEqual(request['daemon'], owner.state['daemon'])
                    labels = {'voice.ci.run': owner.run_id, 'voice.ci.resource': entry['nonce'],
                              'voice.ci.kind': 'container'}
                    return engine.create_container(entry['name'], labels, rootless.IMAGE,
                                                   entry['network'], mounts, [])

                name = owner.create_rootless_container(network, mounts, creator)
                self.assertEqual(owner.state['resources'][-1]['state'], 'ready')
                with self.assertRaises(OwnershipError):
                    owner.create_rootless_container(network, mounts,
                                                   lambda request: self.fail('shared daemon volumes must be rejected'))
                owner.start_container(name)
                self.assertGreaterEqual(engine.rootless_verifications, 3)
                owner.cleanup()
            finally:
                owner.close()

    def test_rootless_shared_workspace_requires_owned_distinct_volume_and_quarantines_it(self):
        with tempfile.TemporaryDirectory() as directory:
            engine = FakeEngine(); owner = OwnedRun.create(Path(directory), engine)
            try:
                network = owner.create_resource('network')
                mounts = {target: owner.create_resource('volume') for target in sorted(rootless.TARGETS)}
                mounts['/workspace'] = owner.create_resource('volume')
                def creator(request):
                    item = owner.state['resources'][-1]
                    self.assertEqual(item['profile'], 'rootless-workspace-v1')
                    self.assertEqual(request['workspace_volume'], mounts['/workspace'])
                    labels = {'voice.ci.run': owner.run_id, 'voice.ci.resource': item['nonce'], 'voice.ci.kind': 'container'}
                    return engine.create_container(item['name'], labels, rootless.IMAGE, item['network'], mounts, [])
                bad = dict(mounts, **{'/workspace': mounts['/run/user/1000']})
                with self.assertRaises(OwnershipError): owner.create_rootless_container(network, bad, creator)
                name = owner.create_rootless_container(network, mounts, creator)
                owner.start_container(name)
                item = owner.state['resources'][-1]; item['state'] = 'executing'; owner._save()
                owner.cancel_execution(name)
                self.assertEqual(item['state'], 'quarantined')
                self.assertTrue(all(r['state'] == 'quarantined' for r in owner.state['resources']
                                    if r['kind'] == 'volume'))
                owner.cleanup()
            finally:
                owner.close()

    def test_workspace_lost_create_and_quarantine_recover_exact_journal(self):
        with tempfile.TemporaryDirectory() as directory:
            engine = FakeEngine(); owner = OwnedRun.create(Path(directory), engine)
            path = owner.directory
            try:
                network = owner.create_resource('network')
                mounts = {target: owner.create_resource('volume') for target in sorted(rootless.WORKSPACE_TARGETS)}
                def lost(request):
                    item = owner.state['resources'][-1]
                    labels = {'voice.ci.run': owner.run_id, 'voice.ci.resource': item['nonce'], 'voice.ci.kind': 'container'}
                    engine.create_container(item['name'], labels, rootless.IMAGE, item['network'], mounts, [])
                    raise OwnershipError('lost create response')
                with self.assertRaises(OwnershipError): owner.create_rootless_container(network, mounts, lost)
                name = owner.state['resources'][-1]['name']
                self.assertEqual(owner.state['resources'][-1]['state'], 'creating')
            finally:
                owner.close()
            owner = OwnedRun.load(path, engine)
            try:
                engine.rootless_profile_valid = False
                with self.assertRaises(OwnershipError): owner.reconcile_creation(name)
                self.assertEqual(owner.state['resources'][-1]['state'], 'creating')
                engine.rootless_profile_valid = True
                owner.reconcile_creation(name)
                owner.start_container(name)
                owner.state['resources'][-1]['state'] = 'executing'; owner._save()
                owner.cancel_execution(name)
            finally:
                owner.close()
            owner = OwnedRun.load(path, engine)
            try:
                self.assertEqual(owner.state['resources'][-1]['state'], 'quarantined')
                self.assertTrue(all(r['state'] == 'quarantined' for r in owner.state['resources'] if r['kind'] == 'volume'))
                owner.cleanup()
                self.assertTrue(all(r['state'] == 'removed' for r in owner.state['resources']))
            finally:
                owner.close()

    def test_rootless_lost_create_response_recovery_requires_exact_profile(self):
        with tempfile.TemporaryDirectory() as directory:
            engine = FakeEngine()
            owner = OwnedRun.create(Path(directory), engine)
            try:
                network = owner.create_resource('network')
                mounts = {target: owner.create_resource('volume') for target in sorted(rootless.TARGETS)}

                def lost_response(request):
                    entry = owner.state['resources'][-1]
                    labels = {'voice.ci.run': owner.run_id, 'voice.ci.resource': entry['nonce'],
                              'voice.ci.kind': 'container'}
                    engine.create_container(entry['name'], labels, rootless.IMAGE, entry['network'], mounts, [])
                    raise OwnershipError('response lost')

                with self.assertRaises(OwnershipError):
                    owner.create_rootless_container(network, mounts, lost_response)
                name = owner.state['resources'][-1]['name']
                engine.rootless_profile_valid = False
                with self.assertRaises(OwnershipError):
                    owner.reconcile_creation(name)
                self.assertEqual(owner.state['resources'][-1]['state'], 'creating')
                engine.rootless_profile_valid = True
                owner.reconcile_creation(name)
                owner.cleanup()
            finally:
                owner.close()

    def test_rootless_failed_post_create_inspection_keeps_reconcilable_state(self):
        with tempfile.TemporaryDirectory() as directory:
            engine = FakeEngine()
            owner = OwnedRun.create(Path(directory), engine)
            try:
                network = owner.create_resource('network')
                mounts = {target: owner.create_resource('volume') for target in sorted(rootless.TARGETS)}

                def creator(request):
                    item = owner.state['resources'][-1]
                    labels = {'voice.ci.run': owner.run_id, 'voice.ci.resource': item['nonce'],
                              'voice.ci.kind': 'container'}
                    return engine.create_container(item['name'], labels, rootless.IMAGE,
                                                   item['network'], mounts, [])

                engine.rootless_profile_valid = False
                with self.assertRaises(OwnershipError):
                    owner.create_rootless_container(network, mounts, creator)
                item = owner.state['resources'][-1]
                self.assertEqual((item['state'], item['id']), ('creating', ''))
                engine.rootless_profile_valid = True
                owner.reconcile_creation(item['name'])
                owner.cleanup()
            finally:
                owner.close()
    def test_owned_execution_can_stream_input_without_a_host_bind_mount(self):
        with tempfile.TemporaryDirectory() as directory:
            engine = FakeEngine()
            owner = OwnedRun.create(Path(directory), engine)
            try:
                network = owner.create_resource('network')
                volume = owner.create_resource('volume')
                name = owner.create_container('sha256:' + 'd' * 64, network, {'/data': volume}, [],
                                              command=['sleep', 'infinity'])
                owner.start_container(name)
                with tempfile.TemporaryFile() as source:
                    source.write(b'snapshot bytes')
                    source.seek(0)
                    self.assertEqual(owner.execute(name, ['tar', '-x'], None, None, 30, stdin=source), 7)
                self.assertEqual(engine.last_input, b'snapshot bytes')
                owner.cleanup()
            finally:
                owner.close()

    def test_ambiguous_exec_status_does_not_release_execution(self):
        for status in (-1, 125, 256, True):
            with self.subTest(status=status), tempfile.TemporaryDirectory() as directory:
                engine = FakeEngine()
                owner = OwnedRun.create(Path(directory), engine)
                try:
                    network = owner.create_resource('network')
                    volume = owner.create_resource('volume')
                    name = owner.create_container('sha256:' + 'd' * 64, network, {'/data': volume}, [],
                                                  command=['sleep', 'infinity'])
                    owner.start_container(name)
                    engine.execute = lambda *args: status
                    with self.assertRaises(OwnershipError):
                        owner.execute(name, ['go', 'version'], None, None, 30)
                    self.assertEqual(owner.state['resources'][-1]['state'], 'executing')
                    owner.cancel_execution(name)
                    owner.cleanup()
                finally:
                    owner.close()

    def test_container_inspection_rejects_unknown_running_state(self):
        engine = DockerCLI.__new__(DockerCLI)
        container = {'Id': 'a' * 64, 'Name': '/fixture', 'Image': 'sha256:' + 'b' * 64,
                     'HostConfig': {'NetworkMode': 'c' * 64}, 'Config': {}, 'State': {}}
        engine._call = lambda *args: json.dumps([container])
        with self.assertRaises(OwnershipError):
            engine.inspect('container', 'fixture')
        container['State']['Running'] = False
        self.assertIs(engine.inspect('container', 'fixture')['running'], False)

    def test_cancel_uncertain_execution_rechecks_exact_identity_and_stopped_state(self):
        with tempfile.TemporaryDirectory() as directory:
            engine = FakeEngine()
            owner = OwnedRun.create(Path(directory), engine)
            try:
                network = owner.create_resource('network')
                volume = owner.create_resource('volume')
                name = owner.create_container('sha256:' + 'd' * 64, network, {'/data': volume}, [],
                                              command=['sleep', 'infinity'])
                owner.start_container(name)
                engine.uncertain_exec = True
                with self.assertRaises(OwnershipError):
                    owner.execute(name, ['sleep', '30'], None, None, 1)
                engine.uncertain_stop = True
                with self.assertRaises(OwnershipError):
                    owner.cancel_execution(name)
                self.assertEqual(owner.state['resources'][-1]['state'], 'stopping')
                with self.assertRaises(OwnershipError):
                    owner.cleanup()
                engine.uncertain_stop = False
                owner.cancel_execution(name)
                self.assertFalse(engine.resources[('container', name)]['running'])
                self.assertEqual(owner.state['resources'][-1]['state'], 'ready')
                owner.cleanup()
            finally:
                owner.close()

    def test_execution_is_journaled_and_uncertain_exit_blocks_cleanup(self):
        with tempfile.TemporaryDirectory() as directory:
            engine = FakeEngine()
            owner = OwnedRun.create(Path(directory), engine)
            try:
                network = owner.create_resource('network')
                volume = owner.create_resource('volume')
                name = owner.create_container('sha256:' + 'd' * 64, network, {'/data': volume}, [],
                                              command=['sleep', 'infinity'])
                owner.start_container(name)
                self.assertEqual(owner.execute(name, ['go', 'version'], None, None, 30), 7)
                self.assertEqual(owner.state['resources'][-1]['state'], 'ready')
                engine.uncertain_exec = True
                with self.assertRaises(OwnershipError):
                    owner.execute(name, ['go', 'version'], None, None, 30)
                self.assertEqual(owner.state['resources'][-1]['state'], 'executing')
                with self.assertRaises(OwnershipError):
                    owner.cleanup()
                self.assertEqual(engine.removed, [])
            finally:
                owner.close()

    def test_container_can_add_an_owned_workspace_volume(self):
        with tempfile.TemporaryDirectory() as directory:
            engine = FakeEngine()
            owner = OwnedRun.create(Path(directory), engine)
            try:
                network = owner.create_resource('network')
                image_volume = owner.create_resource('volume')
                workspace = owner.create_resource('volume')
                name = owner.create_container('sha256:' + 'd' * 64, network,
                                              {'/data': image_volume, '/workspace': workspace}, [],
                                              command=['sleep', 'infinity'])
                self.assertEqual(owner._inspect(owner.state['resources'][-1])['name'], name)
                owner.cleanup()
            finally:
                owner.close()

    def test_explicit_command_rejects_invalid_input_and_image_entrypoint(self):
        with tempfile.TemporaryDirectory() as directory:
            engine = FakeEngine()
            owner = OwnedRun.create(Path(directory), engine)
            try:
                network = owner.create_resource('network')
                volume = owner.create_resource('volume')
                for command in ([], [''], ['sleep\nother'], ['x'] * 65):
                    with self.subTest(command=command), self.assertRaises(OwnershipError):
                        owner.create_container('sha256:' + 'd' * 64, network, {'/data': volume}, [], command=command)
                engine.image = lambda identifier: {'id': identifier, 'volumes': ['/data'], 'entrypoint': ['sh']}
                with self.assertRaises(OwnershipError):
                    owner.create_container('sha256:' + 'd' * 64, network, {'/data': volume}, [], command=['sleep'])
                self.assertFalse(any(kind == 'container' for kind, _ in engine.resources))
                owner.cleanup()
            finally:
                owner.close()

    def test_explicit_container_command_is_bound_before_cleanup(self):
        with tempfile.TemporaryDirectory() as directory:
            engine = FakeEngine()
            owner = OwnedRun.create(Path(directory), engine)
            try:
                network = owner.create_resource('network')
                volume = owner.create_resource('volume')
                name = owner.create_container('sha256:' + 'd' * 64, network, {'/data': volume}, [],
                                              command=['sleep', 'infinity'])
                container = engine.resources[('container', name)]
                self.assertEqual(owner.state['resources'][-1]['command'], ['sleep', 'infinity'])
                container['command'] = ['sleep', '10']
                with self.assertRaises(OwnershipError):
                    owner.cleanup()
                self.assertEqual(engine.removed, [])
                container['command'] = ['sleep', 'infinity']
                container['entrypoint'] = ['unexpected']
                with self.assertRaises(OwnershipError):
                    owner.cleanup()
                self.assertEqual(engine.removed, [])
                container['entrypoint'] = []
                owner.cleanup()
            finally:
                owner.close()

    def test_changed_container_spec_and_external_network_fail_before_deletion(self):
        with tempfile.TemporaryDirectory() as directory:
            engine = FakeEngine()
            owner = OwnedRun.create(Path(directory), engine)
            try:
                network = owner.create_resource('network')
                volume = owner.create_resource('volume')
                name = owner.create_container('sha256:' + 'd' * 64, network, {'/data': volume}, [])
                container = engine.resources[('container', name)]
                original = container['network']
                container['network'] = 'e' * 64
                with self.assertRaises(OwnershipError):
                    owner.cleanup()
                self.assertEqual(engine.removed, [])
                container['network'] = original
                engine.resources[('network', network)]['internal'] = False
                with self.assertRaises(OwnershipError):
                    owner.cleanup()
                self.assertEqual(engine.removed, [])
            finally:
                owner.close()

    def test_uncertain_container_start_blocks_cleanup_and_retains_journal(self):
        with tempfile.TemporaryDirectory() as directory:
            engine = FakeEngine()
            owner = OwnedRun.create(Path(directory), engine)
            network = owner.create_resource('network')
            volume = owner.create_resource('volume')
            name = owner.create_container('sha256:' + 'd' * 64, network, {'/data': volume}, [])
            engine.uncertain_start = True
            with self.assertRaises(OwnershipError):
                owner.start_container(name)
            owner.close()
            restored = OwnedRun.load(owner.directory, engine)
            try:
                self.assertEqual(restored.state['resources'][-1]['state'], 'starting')
                with self.assertRaises(OwnershipError):
                    restored.cleanup()
                self.assertEqual(engine.removed, [])
            finally:
                restored.close()

    def test_owned_container_requires_own_network_and_all_image_volumes(self):
        with tempfile.TemporaryDirectory() as directory:
            engine = FakeEngine()
            first = OwnedRun.create(Path(directory), engine)
            second = OwnedRun.create(Path(directory), engine)
            try:
                network = first.create_resource('network')
                volume = first.create_resource('volume')
                foreign = second.create_resource('volume')
                image = 'sha256:' + 'd' * 64
                with self.assertRaises(OwnershipError):
                    first.create_container(image, network, {}, [])
                with self.assertRaises(OwnershipError):
                    first.create_container(image, network, {'/data': foreign}, [])
                with self.assertRaises(OwnershipError):
                    first.create_container(image, network, {'/data': volume}, ['6379/tcp'])
                name = first.create_container(image, network, {'/data': volume}, [])
                first.start_container(name)
                self.assertTrue(engine.resources[('container', name)]['running'])
                first.cleanup()
                self.assertEqual([kind for kind, _ in engine.removed], ['container', 'volume', 'network'])
                self.assertIn(('volume', foreign), engine.resources)
            finally:
                first.close()
                second.close()

    def test_cleanup_is_confined_to_the_exact_run(self):
        with tempfile.TemporaryDirectory() as directory:
            engine = FakeEngine()
            root = Path(directory)
            first = OwnedRun.create(root, engine)
            second = OwnedRun.create(root, engine)
            try:
                a = first.create_resource('network')
                b = first.create_resource('volume')
                c = second.create_resource('network')
                engine.resources[('volume', 'foreign')] = {'id': 'foreign', 'name': 'foreign',
                                                           'labels': {'voice.ci.run': 'someone-else'}, 'kind': 'volume'}
                with self.assertRaises(OwnershipError):
                    OwnedRun.load(first.directory, engine)
                first.cleanup()
                self.assertNotIn(('network', a), engine.resources)
                self.assertNotIn(('volume', b), engine.resources)
                self.assertIn(('network', c), engine.resources)
                self.assertIn(('volume', 'foreign'), engine.resources)
                self.assertEqual(engine.removed, [('volume', b), ('network', a)])
            finally:
                first.close()
                second.close()

    def test_changed_labels_or_daemon_never_delete_resources(self):
        with tempfile.TemporaryDirectory() as directory:
            engine = FakeEngine()
            owner = OwnedRun.create(Path(directory), engine)
            try:
                name = owner.create_resource('volume')
                engine.resources[('volume', name)]['labels']['voice.ci.run'] = 'foreign'
                with self.assertRaises(OwnershipError):
                    owner.cleanup()
                self.assertEqual(engine.removed, [])
                engine.resources[('volume', name)]['labels']['voice.ci.run'] = owner.run_id
                engine.identity = 'other-daemon'
                with self.assertRaises(OwnershipError):
                    owner.cleanup()
                self.assertEqual(engine.removed, [])
            finally:
                owner.close()

    def test_uncertain_create_is_persisted_and_cannot_be_pruned(self):
        with tempfile.TemporaryDirectory() as directory:
            engine = FakeEngine()
            owner = OwnedRun.create(Path(directory), engine)
            engine.uncertain_create = True
            with self.assertRaises(OwnershipError):
                owner.create_resource('volume')
            self.assertEqual(owner.state['resources'][0]['state'], 'creating')
            owner.close()
            restored = OwnedRun.load(owner.directory, engine)
            try:
                self.assertEqual(restored.state['resources'][0]['state'], 'creating')
                with self.assertRaises(OwnershipError):
                    restored.cleanup()
                self.assertEqual(engine.removed, [])
            finally:
                restored.close()

    def test_uncertain_remove_retains_intent_and_leaves_other_resources(self):
        with tempfile.TemporaryDirectory() as directory:
            engine = FakeEngine()
            owner = OwnedRun.create(Path(directory), engine)
            network = owner.create_resource('network')
            owner.create_resource('volume')
            engine.uncertain_remove = True
            with self.assertRaises(OwnershipError):
                owner.cleanup()
            owner.close()
            restored = OwnedRun.load(owner.directory, engine)
            try:
                self.assertEqual(restored.state['resources'][-1]['state'], 'removing')
                with self.assertRaises(OwnershipError):
                    restored.cleanup()
                self.assertIn(('network', network), engine.resources)
            finally:
                restored.close()


if __name__ == '__main__':
    unittest.main()
