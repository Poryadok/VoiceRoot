"""Reject Compose configurations that can collide with unrelated local runs."""
import copy
from pathlib import Path
import tempfile
import unittest

from local_compose_audit import audit


class ComposeAuditTests(unittest.TestCase):
    def test_isolated_configuration_and_unsafe_variants(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            checkout = root / 'checkout'
            checkout.mkdir()
            project = 'voiceci-' + 'a' * 32
            scratch = checkout / 'tmp' / 'ci-local' / project
            scratch.mkdir(parents=True)
            config = {
                'name': project,
                'services': {'db': {
                    'image': 'postgres:16-bookworm',
                    'ports': [{'host_ip': '127.0.0.1', 'target': 5432, 'published': '0', 'protocol': 'tcp'}],
                    'volumes': [{'type': 'volume', 'source': 'data', 'target': '/data'},
                                {'type': 'bind', 'source': str(checkout), 'target': '/repo', 'read_only': True},
                                {'type': 'bind', 'source': str(scratch), 'target': '/scratch'}],
                }},
                'volumes': {'data': {'name': project + '_data'}},
                'networks': {'default': {'name': project + '_default'}},
            }
            self.assertEqual(audit(config, project, checkout, scratch), [])
            with self.assertRaises(ValueError):
                audit(config, project, checkout, root)
            built = copy.deepcopy(config)
            built['services']['db'].update(build={'context': str(checkout)}, image='voice-db:' + project)
            self.assertEqual(audit(built, project, checkout, scratch), [])
            built['services']['db']['build'].update(dockerfile='docker/Dockerfile', network='none')
            self.assertEqual(audit(built, project, checkout, scratch), [])
            variants = []
            def variant(change):
                candidate = copy.deepcopy(config)
                change(candidate)
                variants.append(candidate)
            variant(lambda c: c['services']['db']['ports'][0].update(published='5432'))
            variant(lambda c: c['services']['db']['ports'][0].pop('host_ip'))
            variant(lambda c: c['services']['db'].update(container_name='shared'))
            variant(lambda c: c['services']['db'].update(privileged=True))
            variant(lambda c: c['services']['db'].update(userns_mode='host'))
            variant(lambda c: c['services']['db'].update(uts='host'))
            variant(lambda c: c['services']['db'].update(cgroup='host'))
            variant(lambda c: c['services']['db'].update(runtime='custom'))
            variant(lambda c: c['services']['db'].update(security_opt=['seccomp=unconfined']))
            variant(lambda c: c['services']['db'].update(device_cgroup_rules=['c 1:3 rwm']))
            variant(lambda c: c['services']['db'].update(post_start=[{'command': 'true', 'privileged': True}]))
            variant(lambda c: c['services']['db'].update(pre_stop=[{'command': 'true', 'privileged': True}]))
            variant(lambda c: c['services']['db'].update(pre_start=[{'command': 'true', 'privileged': True}]))
            variant(lambda c: c['services']['db'].update(provider={'type': 'external'}))
            variant(lambda c: c['services']['db'].update(gpus='all'))
            variant(lambda c: c['services']['db'].update(cgroup_parent='host-group'))
            variant(lambda c: c['services']['db'].update(deploy={'resources': {'reservations': {'devices': [{'capabilities': ['gpu']}]}}}))
            variant(lambda c: c['services']['db'].update(network_mode='host'))
            variant(lambda c: c['services']['db'].update(pid='container:foreign'))
            variant(lambda c: c.update(secrets={'shared': {'external': True}}))
            variant(lambda c: c['volumes']['data'].update(external=True))
            variant(lambda c: c['volumes']['data'].update(name='shared-data'))
            variant(lambda c: c['volumes']['data'].update(driver_opts={'device': '/host'}))
            variant(lambda c: c['volumes']['data'].update(driver='nfs'))
            variant(lambda c: c['networks']['default'].update(name='shared-network'))
            variant(lambda c: c['networks']['default'].update(driver='macvlan'))
            variant(lambda c: c['services']['db']['volumes'].append(
                {'type': 'bind', 'source': '/var/run/docker.sock', 'target': '/var/run/docker.sock'}))
            variant(lambda c: c['services']['db']['volumes'][1].update(read_only=False))
            variant(lambda c: c['services']['db']['volumes'][1].update(source='relative/path'))
            variant(lambda c: c['services']['db'].update(build={'context': str(checkout)}, image='voice-db:local'))
            variant(lambda c: c['services']['db'].update(build={'context': str(root)}, image='voice-db:' + project))
            variant(lambda c: c['services']['db'].update(build={'context': str(checkout), 'cache_to': ['type=registry,ref=shared']}, image='voice-db:' + project))
            variant(lambda c: c['services']['db'].update(build={'context': str(checkout), 'network': 'host'}, image='voice-db:' + project))
            variant(lambda c: c['services']['db'].update(build={'context': str(checkout), 'privileged': True}, image='voice-db:' + project))
            variant(lambda c: c['services']['db'].update(build={'context': str(checkout), 'entitlements': ['security.insecure']}, image='voice-db:' + project))
            variant(lambda c: c['services']['db'].update(build={'context': str(checkout), 'ssh': ['default']}, image='voice-db:' + project))
            variant(lambda c: c['services']['db'].update(build={'context': str(checkout), 'secrets': ['credentials']}, image='voice-db:' + project))
            variant(lambda c: c['services']['db'].update(build={'context': str(checkout), 'dockerfile': '../outside/Dockerfile'}, image='voice-db:' + project))
            variant(lambda c: c['services']['db'].update(image='voice-db:local'))
            variant(lambda c: c.update(name='another-project'))
            for candidate in variants:
                with self.subTest(candidate=candidate):
                    self.assertTrue(audit(candidate, project, checkout, scratch))


if __name__ == '__main__':
    unittest.main()
