"""Reject unsafe changes to a private rootless daemon container profile."""
import copy
import unittest

import local_rootless_profile as profile


class RootlessProfileTests(unittest.TestCase):
    def test_owned_network_has_exactly_the_expected_peers(self):
        network_id = 'c' * 64
        labels = {'voice.ci.run': 'voiceci-' + 'a' * 32, 'voice.ci.resource': 'b' * 32,
                  'voice.ci.kind': 'network'}
        peers = {'d' * 64: 'daemon', 'e' * 64: 'client'}
        item = {'Id': network_id, 'Name': 'owned', 'Driver': 'bridge', 'Scope': 'local',
                'Internal': True, 'Labels': labels,
                'Containers': {key: {'Name': name, 'EndpointID': 'f' * 64} for key, name in peers.items()}}
        profile.verify_owned_network(item, network_id, 'owned', labels, peers)
        variants = []
        for field, value in [('Id', 'f' * 64), ('Name', 'foreign'), ('Internal', False),
                             ('Driver', 'overlay'), ('Scope', 'swarm'), ('Labels', {})]:
            altered = copy.deepcopy(item)
            altered[field] = value
            variants.append(altered)
        missing = copy.deepcopy(item)
        del missing['Containers']['e' * 64]
        variants.append(missing)
        extra = copy.deepcopy(item)
        extra['Containers']['f' * 64] = {'Name': 'foreign', 'EndpointID': 'f' * 64}
        variants.append(extra)
        renamed = copy.deepcopy(item)
        renamed['Containers']['d' * 64]['Name'] = 'foreign'
        variants.append(renamed)
        for altered in variants:
            with self.subTest(altered=altered), self.assertRaises(ValueError):
                profile.verify_owned_network(altered, network_id, 'owned', labels, peers)
        with self.assertRaises(ValueError):
            profile.verify_owned_network(item, network_id, 'owned', labels, {})

    def fixture(self):
        labels = {'voice.ci.run': 'voiceci-' + 'a' * 32, 'voice.ci.kind': 'container'}
        volumes = {'/home/rootless/.local/share/docker': 'data-volume', '/var/lib/docker': 'legacy-volume',
                   '/run/user/1000': 'socket-volume'}
        host = copy.deepcopy(profile.HOST_DEFAULTS)
        host.update({'NetworkMode': 'owned-network', 'MaskedPaths': list(profile.MASKED_PATHS),
                     'ReadonlyPaths': list(profile.READONLY_PATHS),
                     'Mounts': [{'Type': 'volume', 'Source': value, 'Target': key} for key, value in volumes.items()]})
        config = copy.deepcopy(profile.CONFIG_DEFAULTS)
        env = [entry for entry in profile.ENV if not entry.startswith('DOCKERD_ROOTLESS_ROOTLESSKIT_FLAGS=')]
        env.append('DOCKERD_ROOTLESS_ROOTLESSKIT_FLAGS=')
        config.update({'Hostname': 'b' * 12, 'Image': profile.IMAGE, 'User': 'rootless', 'Env': env,
                       'Cmd': ['dockerd', '--storage-driver=vfs', '--iptables=false', '--ip-masq=false',
                               '--host=unix:///run/user/1000/docker.sock'],
                       'Entrypoint': ['dockerd-entrypoint.sh'], 'Labels': labels})
        item = {'Id': 'b' * 64, 'Name': '/owned-daemon', 'Image': profile.IMAGE,
                'Config': config,
                'HostConfig': host,
                'Mounts': [{'Type': 'volume', 'Name': value, 'Destination': key, 'RW': True}
                           for key, value in volumes.items()]}
        return item, labels, volumes

    def test_accepts_exact_profile_and_rejects_host_escape_variants(self):
        item, labels, volumes = self.fixture()
        profile.verify_selected_fields(item, 'b' * 64, 'owned-daemon', 'owned-network', volumes, labels)
        mutations = [
            ('Privileged', True), ('NetworkMode', 'host'), ('PidMode', 'host'), ('IpcMode', 'host'),
            ('UTSMode', 'host'), ('UsernsMode', 'host'), ('CgroupnsMode', 'host'), ('Runtime', 'custom'),
            ('CapAdd', ['SYS_ADMIN']), ('SecurityOpt', ['seccomp=unconfined']),
            ('Binds', ['/var/run/docker.sock:/var/run/docker.sock']), ('PortBindings', {'2375/tcp': [{}]}),
            ('PublishAllPorts', True), ('Devices', [{'PathOnHost': '/dev/kmsg'}]),
            ('DeviceRequests', [{'Driver': 'x'}]), ('DeviceCgroupRules', ['a *:* rwm']),
            ('Sysctls', {'net.ipv4.ip_forward': '1'}), ('GroupAdd', ['0']),
            ('ReadonlyPaths', []), ('MaskedPaths', []), ('Memory', 0), ('PidsLimit', 0),
            ('RestartPolicy', {'Name': 'always', 'MaximumRetryCount': 0}), ('AutoRemove', True),
        ]
        for key, value in mutations:
            with self.subTest(field=key):
                changed = copy.deepcopy(item)
                changed['HostConfig'][key] = value
                with self.assertRaises(ValueError):
                    profile.verify_selected_fields(changed, 'b' * 64, 'owned-daemon', 'owned-network', volumes, labels)

    def test_rejects_changed_image_identity_command_environment_and_mounts(self):
        item, labels, volumes = self.fixture()
        for section, key, value in [('root', 'Id', 'c' * 64), ('root', 'Image', 'sha256:' + 'd' * 64),
                                    ('Config', 'User', 'root'), ('Config', 'Cmd', ['sh']),
                                    ('Config', 'Env', list(profile.ENV) + ['DOCKER_HOST=unix:///host']),
                                    ('Config', 'Labels', {'voice.ci.run': 'foreign'})]:
            with self.subTest(field=key):
                changed = copy.deepcopy(item)
                (changed if section == 'root' else changed[section])[key] = value
                with self.assertRaises(ValueError):
                    profile.verify_selected_fields(changed, 'b' * 64, 'owned-daemon', 'owned-network', volumes, labels)
        changed = copy.deepcopy(item)
        changed['Mounts'][0]['Name'] = 'foreign-volume'
        with self.assertRaises(ValueError):
            profile.verify_selected_fields(changed, 'b' * 64, 'owned-daemon', 'owned-network', volumes, labels)

    def test_workspace_profile_requires_exact_fourth_owned_mount(self):
        item, labels, volumes = self.fixture()
        volumes['/workspace'] = 'workspace-volume'
        item['HostConfig']['Mounts'].append({'Type': 'volume', 'Source': 'workspace-volume', 'Target': '/workspace'})
        item['Mounts'].append({'Type': 'volume', 'Name': 'workspace-volume', 'Destination': '/workspace', 'RW': True})
        profile.verify_selected_fields(item, 'b' * 64, 'owned-daemon', 'owned-network', volumes, labels)
        for mutation in ('name', 'read_only', 'unlisted', 'alias'):
            changed, expected = copy.deepcopy(item), dict(volumes)
            if mutation == 'name': changed['Mounts'][-1]['Name'] = 'foreign'
            if mutation == 'read_only': changed['Mounts'][-1]['RW'] = False
            if mutation == 'unlisted': changed['HostConfig']['Mounts'].append({'Type': 'bind', 'Source': '/', 'Target': '/host'})
            if mutation == 'alias': expected['/workspace'] = expected['/run/user/1000']
            with self.subTest(mutation=mutation), self.assertRaises(ValueError):
                profile.verify_selected_fields(changed, 'b' * 64, 'owned-daemon', 'owned-network', expected, labels)

    def test_rejects_missing_security_fields(self):
        item, labels, volumes = self.fixture()
        for key in ('Privileged', 'CapAdd', 'Binds', 'PortBindings', 'Devices', 'MaskedPaths'):
            with self.subTest(field=key):
                changed = copy.deepcopy(item)
                del changed['HostConfig'][key]
                with self.assertRaises(ValueError):
                    profile.verify_selected_fields(changed, 'b' * 64, 'owned-daemon', 'owned-network', volumes, labels)

    def test_rejects_unreviewed_host_configuration(self):
        item, labels, volumes = self.fixture()
        for key, value in [('CgroupParent', '/host'), ('Dns', ['1.1.1.1']),
                           ('LogConfig', {'Type': 'custom', 'Config': {}}),
                           ('ContainerIDFile', '/host/path'), ('UnknownFutureField', True)]:
            with self.subTest(field=key):
                changed = copy.deepcopy(item)
                changed['HostConfig'][key] = value
                with self.assertRaises(ValueError):
                    profile.verify_selected_fields(changed, 'b' * 64, 'owned-daemon', 'owned-network', volumes, labels)

    def test_rejects_unreviewed_container_configuration(self):
        item, labels, volumes = self.fixture()
        for key, value in [('WorkingDir', '/unreviewed'), ('Tty', True),
                           ('ExposedPorts', {'22/tcp': {}}), ('UnknownFutureField', True)]:
            with self.subTest(field=key):
                changed = copy.deepcopy(item)
                changed['Config'][key] = value
                with self.assertRaises(ValueError):
                    profile.verify_selected_fields(changed, 'b' * 64, 'owned-daemon', 'owned-network', volumes, labels)

    def test_running_daemon_must_have_one_owned_network_and_no_published_ports(self):
        item = {'State': {'Running': True}, 'NetworkSettings': {
            'Networks': {'owned': {'NetworkID': 'c' * 64, 'EndpointID': 'd' * 64, 'IPAddress': '172.18.0.2'}},
            'Ports': {'2375/tcp': None, '2376/tcp': None}}}
        self.assertEqual(profile.verify_runtime_network(item, 'c' * 64), '172.18.0.2')
        for change in ('foreign_network', 'extra_network', 'host_port', 'unknown_port',
                       'public_address', 'unspecified_address', 'stopped'):
            with self.subTest(change=change):
                changed = copy.deepcopy(item)
                if change == 'foreign_network':
                    changed['NetworkSettings']['Networks']['owned']['NetworkID'] = 'f' * 64
                elif change == 'extra_network':
                    changed['NetworkSettings']['Networks']['foreign'] = {}
                elif change == 'host_port':
                    changed['NetworkSettings']['Ports']['2375/tcp'] = [{'HostIp': '0.0.0.0', 'HostPort': '2375'}]
                elif change == 'unknown_port':
                    changed['NetworkSettings']['Ports']['22/tcp'] = None
                elif change == 'public_address':
                    changed['NetworkSettings']['Networks']['owned']['IPAddress'] = '8.8.8.8'
                elif change == 'unspecified_address':
                    changed['NetworkSettings']['Networks']['owned']['IPAddress'] = '0.0.0.0'
                else:
                    changed['State']['Running'] = False
                with self.assertRaises(ValueError):
                    profile.verify_runtime_network(changed, 'c' * 64)


if __name__ == '__main__':
    unittest.main()
