"""Ownership and cleanup invariants for local disposable Docker fixtures."""
from pathlib import Path
import tempfile
import unittest

from local_docker_owned import OwnedRun, OwnershipError


class FakeEngine:
    def __init__(self):
        self.identity = 'daemon-a'
        self.resources = {}
        self.removed = []
        self.uncertain_create = False
        self.uncertain_remove = False
        self.uncertain_start = False

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
        return {'id': identifier, 'volumes': ['/data']}

    def create_container(self, name, labels, image_id, network_id, mounts, ports):
        identifier = 'c' + name.encode().hex().ljust(63, '0')[:63]
        self.resources[('container', name)] = {'id': identifier, 'name': name, 'kind': 'container',
                                               'labels': dict(labels), 'image': image_id, 'network': network_id,
                                               'mounts': dict(mounts), 'ports': list(ports), 'running': False}
        return identifier

    def start(self, identifier):
        value = next(value for key, value in self.resources.items() if key[0] == 'container' and value['id'] == identifier)
        value['running'] = True
        if self.uncertain_start:
            raise OwnershipError('start outcome unknown')


class OwnedRunTests(unittest.TestCase):
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
