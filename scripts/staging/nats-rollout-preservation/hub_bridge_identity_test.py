import ast
import copy
from pathlib import Path
import unittest
from unittest import mock
import hub_bridge_identity as identity
import hub_namespace_service as service
import hub_bridge_tool_policy as policy
from controller import Blocked


class BridgeIdentityTests(unittest.TestCase):
    def test_installer_and_loaded_identity_share_fixed_contract_and_reread(self):
        tree = ast.parse(Path(__file__).with_name('installer.py').read_text(encoding='utf-8'))
        node = next(n.value for n in tree.body if isinstance(n, ast.Assign)
                    and any(isinstance(t, ast.Name) and t.id == 'SERVICE' for t in n.targets))
        unit = eval(compile(ast.Expression(node), '<installer SERVICE>', 'eval'), {'hub_bridge_identity': identity})
        for directive in ('User=root', 'Group=root', 'NoNewPrivileges=true', 'AmbientCapabilities=',
                          'ExecStart=' + ' '.join(identity.COMMAND)):
            self.assertIn(directive + '\n', unit)
        expected = {**identity.UNIT_IDENTITY, 'ExecStart': list(identity.COMMAND)}
        calls = []
        self.assertEqual(identity.loaded(lambda: calls.append('read') or copy.deepcopy(expected),
                                         lambda: calls.append('guard')), expected)
        self.assertEqual(calls, ['guard', 'read', 'guard', 'read', 'guard'])
        for key, value in [('User', '1000'), ('Group', '1000'), ('NoNewPrivileges', 'no'),
                           ('AmbientCapabilities', ['CAP_SYS_ADMIN']), ('ExecStart', ['/usr/bin/python3'])]:
            bad = {**expected, key: value}
            with self.subTest(key=key), self.assertRaises(Blocked):
                identity.loaded(mock.Mock(side_effect=[expected, bad]), lambda: None)

    def test_actual_service_consumer_refuses_changed_command_or_process_before_namespace_read(self):
        fields = {'Uid': '0 0 0 0', 'Gid': '0 0 0 0', 'NSpid': '100', 'CapInh': '0' * 16,
                  'CapAmb': '0' * 16, **identity.expected_process()}
        boot = '4e5ebb62-d69d-4782-8706-0808287c5707'
        command = b'\0'.join(x.encode() for x in identity.COMMAND) + b'\0'
        for change in ('command', 'uid', 'gid', 'ambient', 'nnp', 'seccomp', 'valid'):
            current = copy.deepcopy(fields)
            if change == 'uid': current['Uid'] = '1000 1000 1000 1000'
            if change == 'gid': current['Gid'] = '1000 1000 1000 1000'
            if change == 'ambient': current['CapAmb'] = '0000000000201000'
            if change == 'nnp': current['NoNewPrivs'] = '0'
            if change == 'seccomp': current['Seccomp'] = '2'
            obj = service.ServiceCustody.__new__(service.ServiceCustody)
            obj.mount_fd = 10; obj.guard = lambda: None
            obj.profile = {'boot_id': boot, 'process': {k: fields[k] for k in
                           ('CapPrm', 'CapEff', 'CapBnd', 'NoNewPrivs', 'Seccomp')}}
            raw = ''.join(k + ': ' + v + '\n' for k, v in current.items()).encode()
            def read(path, limit):
                return boot.encode() if path.endswith('boot_id') else raw if path.endswith('status') else command + (b'extra\0' if change == 'command' else b'')
            with mock.patch.object(service, '_read', side_effect=read), \
                    mock.patch.object(service.os, 'open', side_effect=RuntimeError('next namespace boundary')) as opened:
                if change == 'valid':
                    with self.assertRaisesRegex(RuntimeError, 'next namespace boundary'): obj.verify()
                    opened.assert_called_once()
                else:
                    with self.assertRaises(Blocked): obj.verify()
                    opened.assert_not_called()

    def test_actual_constructor_refuses_each_missing_or_extra_capability_and_caller_tuple(self):
        value = {'schema': 'voice-approved-namespace-service-v1',
                 'boot_id': '4e5ebb62-d69d-4782-8706-0808287c5707',
                 'mount_namespace': {'dev': 1, 'inode': 2, 'owner_inode': 3},
                 'root_directory': {'dev': 4, 'inode': 5}, 'nft': policy.nft_member(),
                 'process': identity.expected_process()}
        mask = int(identity.CAPABILITY_MASK, 16)
        changes = [(key, format(mask ^ (1 << bit), '016x'))
                   for key in ('CapPrm', 'CapEff', 'CapBnd') for bit in (0, 1, 3, 12, 18, 19, 21)]
        changes += [('CapEff', format(mask | (1 << 13), '016x')), ('Seccomp', '2'), ('NoNewPrivs', '0')]
        with mock.patch.object(service.sys, 'platform', 'linux'), \
                mock.patch.object(service.os, 'geteuid', return_value=0, create=True), \
                mock.patch.object(service.os, 'open', side_effect=RuntimeError('held namespace boundary')) as opened:
            for key, changed in changes:
                bad = copy.deepcopy(value); bad['process'][key] = changed
                with self.subTest(key=key, changed=changed), self.assertRaises(Blocked):
                    service.ServiceCustody(bad, lambda: None)
            opened.assert_not_called()
            with self.assertRaisesRegex(RuntimeError, 'held namespace boundary'):
                service.ServiceCustody(value, lambda: None)
