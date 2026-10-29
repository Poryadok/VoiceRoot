import threading
import unittest
from http.server import ThreadingHTTPServer
from unittest.mock import patch

import core_postrollout_smoke_test as fixture
import staging_core_smoke_http as http


class ExistingAccountHTTPTest(unittest.TestCase):
    def setUp(self):
        self.server = ThreadingHTTPServer(('127.0.0.1', 0), fixture.Server)
        self.server.calls = []
        self.server.registered = {'owned+2@example.test': 0, 'owned+3@example.test': 1}
        thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        thread.start()
        self.addCleanup(self.server.server_close)
        self.addCleanup(self.server.shutdown)
        self.origin = patch.object(http, 'ORIGIN', 'http://127.0.0.1:' + str(self.server.server_port))
        self.origin.start()
        self.addCleanup(self.origin.stop)

    def test_existing_logins_do_not_register_or_send_mail(self):
        client = http.StagingHTTP()
        first = http.login(client, 'owned+2@example.test', 'fixture-only-password')
        second = http.login(client, 'owned+3@example.test', 'fixture-only-password')
        self.assertEqual(first['profile_id'], fixture.Server.accounts[0][1])
        self.assertEqual(second['profile_id'], fixture.Server.accounts[1][1])
        self.assertEqual([row[0] for row in self.server.calls], ['/api/v1/auth/login'] * 2)

    def test_proxy_environment_is_ignored(self):
        observed = []
        original = http.urllib.request.build_opener
        def build(*handlers):
            observed.append(handlers[0].proxies)
            return original(*handlers)
        with patch.dict('os.environ', {'HTTP_PROXY': 'http://foreign.invalid:9000', 'http_proxy': 'http://foreign.invalid:9000'}), \
                patch.object(http.urllib.request, 'build_opener', side_effect=build):
            http.login(http.StagingHTTP(), 'owned+2@example.test', 'fixture-only-password')
        self.assertEqual(observed, [{}])

    def test_mfa_or_non_session_response_holds_without_another_request(self):
        class Challenge(http.StagingHTTP):
            def request(self, *args, **kwargs):
                return {'mfa_required': True}
        with self.assertRaises(http.SmokeHTTPError):
            http.login(Challenge(), 'owned+2@example.test', 'fixture-only-password')

    def test_owned_space_invite_join_and_readback(self):
        client = http.StagingHTTP()
        owner = http.login(client, 'owned+2@example.test', 'fixture-only-password')
        member = http.login(client, 'owned+3@example.test', 'fixture-only-password')
        space = http.create_space(client, 'fixture', owner)
        invite = http.create_invite(client, space['space_id'], owner)
        self.assertEqual(http.join(client, invite, member)['profile_id'], member['profile_id'])
        result = http.read_membership(client, space['space_id'], member)
        self.assertEqual(result, {'space_id': fixture.Server.space, 'profile_id': member['profile_id'], 'matches': 1})
        self.assertEqual([row[0] for row in self.server.calls], ['/api/v1/auth/login'] * 2 + [
            '/api/v1/spaces', '/api/v1/spaces/' + fixture.Server.space + '/invites',
            '/api/v1/invites/fixture-code/join', '/api/v1/spaces'])
        self.assertNotIn('unrelated-not-recorded', repr(result))

    def test_foreign_membership_and_missing_readback_hold(self):
        client = http.StagingHTTP()
        member = http.login(client, 'owned+3@example.test', 'fixture-only-password')
        invite = {'code': 'fixture-code', 'space_id': fixture.Server.space}
        with patch.object(client, 'request',
                          return_value={'space_membership': {'space_id': fixture.Server.space,
                                                               'profile_id': fixture.Server.accounts[0][1]}}):
            with self.assertRaises(http.SmokeHTTPError): http.join(client, invite, member)
        self.server.hide_membership = True
        with self.assertRaises(http.SmokeHTTPError): http.read_membership(client, fixture.Server.space, member)


if __name__ == '__main__':
    unittest.main()
