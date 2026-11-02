"""Private Chat diagnostic must not accept an incomplete Go event stream."""
import json
import unittest
from unittest import mock
from pathlib import Path
import tempfile

import local_chat_private


class ChatPrivateTests(unittest.TestCase):
    def test_selected_case_requires_exact_pass_and_no_skips(self):
        package = 'voice/backend/chat/internal/store'
        case = 'TestSpaceMembersStore_IsAndList'
        def events(actions):
            rows = [{'Package': package, 'Action': 'start'}]
            rows += [{'Package': package, 'Test': case, 'Action': action} for action in actions]
            rows.append({'Package': package, 'Action': 'pass'})
            return ''.join(json.dumps(row) + '\n' for row in rows).encode()
        summary = local_chat_private.validate_events(events(['run', 'pass']))
        self.assertEqual(summary['passed'], 1)
        for actions in (['run', 'skip'], ['run'], ['run', 'fail']):
            with self.subTest(actions=actions), self.assertRaises(ValueError):
                local_chat_private.validate_events(events(actions))

    def test_rejects_unowned_or_mismatched_socket_before_inspection(self):
        owner = mock.Mock()
        owner.state = {'resources': [
            {'kind': 'container', 'name': 'daemon', 'state': 'ready', 'profile': 'rootless-unix-v1',
             'network': 'n', 'mounts': {'/run/user/1000': 'socket'}},
            {'kind': 'container', 'name': 'client', 'state': 'ready', 'network': 'n',
             'mounts': {'/socket': 'other', '/workspace': 'workspace'}, 'command': ['sleep', '300']},
        ]}
        with self.assertRaises(ValueError):
            local_chat_private.inspect_pair(owner, 'daemon', 'client')
        self.assertEqual(owner.engine.mock_calls, [])

    def test_selected_run_is_source_bound_and_never_a_preflight_pass(self):
        package, case = local_chat_private.PACKAGE, local_chat_private.CASE
        events = [{'Package': package, 'Action': 'start'},
                  {'Package': package, 'Test': case, 'Action': 'run'},
                  {'Package': package, 'Test': case, 'Action': 'pass'},
                  {'Package': package, 'Action': 'pass'}]
        owner = mock.Mock()
        def execute(name, command, out, err, timeout):
            self.assertIn('TESTCONTAINERS_RYUK_DISABLED=true', command[2])
            self.assertIn('DOCKER_HOST=unix:///socket/docker.sock', command[2])
            self.assertNotIn('-short', command[2])
            out.write((''.join(json.dumps(row) + '\n' for row in events)).encode())
            return 0
        owner.execute.side_effect = execute
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'root'
            root.mkdir()
            source = {'head': 'a' * 40}
            with mock.patch.object(local_chat_private, 'inspect_pair', return_value='172.18.0.2'), \
                 mock.patch.object(local_chat_private.snapshot, 'verify_materialized'), \
                 mock.patch.object(local_chat_private.snapshot, 'capture', return_value=source), \
                 mock.patch.object(local_chat_private.local_go_execution, '_verify_modules'), \
                 mock.patch.object(local_chat_private.local_go_execution, '_verify_workspace'):
                result = local_chat_private.run_selected(owner, 'daemon', 'client', root, source,
                                                         root / 'snapshot', 'b' * 64, Path(directory) / 'out')
        self.assertEqual(result['status'], 'incomplete')
        self.assertEqual(result['tests']['passed'], 1)
        self.assertIn('Go dependency acquisition and cached ziphash source binding', result['unbound_inputs'])

    def test_pair_binds_exact_endpoints_and_rejects_a_foreign_peer(self):
        nid, did, cid = 'a' * 64, 'b' * 64, 'c' * 64
        deid, ceid = 'd' * 64, 'e' * 64
        owner = mock.Mock()
        owner.run_id = 'voiceci-' + '1' * 32
        daemon = {'kind': 'container', 'name': 'daemon', 'id': did, 'state': 'ready',
                  'profile': 'rootless-unix-v1', 'network': nid,
                  'mounts': {'/run/user/1000': 'socket', '/var/lib/docker': 'legacy',
                             '/home/rootless/.local/share/docker': 'data'}}
        client = {'kind': 'container', 'name': 'client', 'id': cid, 'state': 'ready', 'network': nid,
                  'mounts': {'/socket': 'socket', '/workspace': 'workspace'}, 'command': ['sleep', '300']}
        net = {'kind': 'network', 'id': nid, 'name': 'network', 'nonce': 'f' * 32, 'state': 'ready'}
        owner.state = {'resources': [net, *[{'kind': 'volume', 'name': n, 'state': 'ready'}
                                             for n in ('socket', 'legacy', 'data', 'workspace')], daemon, client]}
        owner._inspect.return_value = {'running': True}
        network = {'Id': nid, 'Name': 'network', 'Driver': 'bridge', 'Scope': 'local', 'Internal': True,
                   'Labels': {'voice.ci.run': owner.run_id, 'voice.ci.resource': net['nonce'], 'voice.ci.kind': 'network'},
                   'Containers': {did: {'Name': 'daemon', 'EndpointID': deid, 'IPv4Address': '172.18.0.2/16'},
                                  cid: {'Name': 'client', 'EndpointID': ceid, 'IPv4Address': '172.18.0.3/16'}}}
        def container(eid, ip):
            return {'State': {'Running': True}, 'NetworkSettings': {'Ports': {}, 'Networks': {
                'network': {'NetworkID': nid, 'EndpointID': eid, 'IPAddress': ip}}}}
        daemon_raw, client_raw = container(deid, '172.18.0.2'), container(ceid, '172.18.0.3')
        owner.engine._call.side_effect = lambda kind, action, ident: json.dumps([
            network if kind == 'network' else daemon_raw if ident == did else client_raw])
        self.assertEqual(local_chat_private.inspect_pair(owner, 'daemon', 'client'), '172.18.0.2')
        daemon['profile'] = 'rootless-workspace-v1'
        daemon['mounts']['/workspace'] = 'workspace'
        with self.assertRaises(ValueError):
            local_chat_private.inspect_pair(owner, 'daemon', 'client')
        self.assertEqual(local_chat_private.inspect_pair(owner, 'daemon', 'client', shared_workspace=True), '172.18.0.2')
        client['mounts']['/workspace'] = 'foreign'
        with self.assertRaises(ValueError):
            local_chat_private.inspect_pair(owner, 'daemon', 'client', shared_workspace=True)
        client['mounts']['/workspace'] = 'workspace'
        network['Containers']['f' * 64] = {'Name': 'foreign', 'EndpointID': '9' * 64}
        with self.assertRaises(ValueError):
            local_chat_private.inspect_pair(owner, 'daemon', 'client', shared_workspace=True)

    def test_source_change_and_failed_execution_never_report_success(self):
        owner = mock.Mock()
        source = {'head': 'a' * 40}
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'root'
            root.mkdir()
            with mock.patch.object(local_chat_private, 'inspect_pair', return_value='172.18.0.2'), \
                 mock.patch.object(local_chat_private.snapshot, 'verify_materialized'), \
                 mock.patch.object(local_chat_private.snapshot, 'capture', return_value={'head': 'b' * 40}), \
                 self.assertRaisesRegex(ValueError, 'source changed'):
                local_chat_private.run_selected(owner, 'daemon', 'client', root, source,
                                                root / 'snapshot', 'b' * 64, Path(directory) / 'first')
            owner.execute.assert_not_called()
            owner.execute.return_value = 1
            with mock.patch.object(local_chat_private, 'inspect_pair', return_value='172.18.0.2'), \
                 mock.patch.object(local_chat_private.snapshot, 'verify_materialized'), \
                 mock.patch.object(local_chat_private.snapshot, 'capture', return_value=source), \
                 mock.patch.object(local_chat_private.local_go_execution, '_verify_modules'), \
                 mock.patch.object(local_chat_private.local_go_execution, '_verify_workspace'), \
                 self.assertRaisesRegex(ValueError, 'diagnostic failed'):
                local_chat_private.run_selected(owner, 'daemon', 'client', root, source,
                                                root / 'snapshot', 'b' * 64, Path(directory) / 'second')


if __name__ == '__main__':
    unittest.main()
