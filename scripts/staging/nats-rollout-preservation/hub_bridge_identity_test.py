import ast
import copy
from pathlib import Path
import unittest
from unittest import mock
import hub_bridge_identity as identity
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

    def test_fixed_command_and_process_reject_drift(self):
        fields = {'Uid': '0 0 0 0', 'Gid': '0 0 0 0', 'NSpid': '100',
                  'CapInh': '0' * 16, 'CapAmb': '0' * 16, **identity.expected_process()}
        command = b'\0'.join(x.encode() for x in identity.COMMAND) + b'\0'
        identity.command(command); identity.process(fields)
        for changed in (command + b'extra\0', command[:-1], command.decode()):
            with self.subTest(command=changed), self.assertRaises(Blocked):identity.command(changed)
        changes = [('Uid', '1000 1000 1000 1000'), ('Gid', '1000 1000 1000 1000'),
                   ('CapAmb', '0000000000201000'), ('NoNewPrivs', '0'), ('Seccomp', '2'),
                   ('NSpid', '100 200')]
        mask = int(identity.CAPABILITY_MASK, 16)
        changes += [(key, format(mask ^ (1 << bit), '016x'))
                    for key in ('CapPrm', 'CapEff', 'CapBnd') for bit in (0, 1, 3, 12, 18, 19, 21)]
        changes += [('CapEff', format(mask | (1 << 13), '016x'))]
        for key, value in changes:
            with self.subTest(key=key, value=value), self.assertRaises(Blocked):
                identity.process({**fields, key: value})
