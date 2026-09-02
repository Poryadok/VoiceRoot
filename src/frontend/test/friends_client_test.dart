import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/friends_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';

import 'support/gateway_test_client.dart';

void main() {
  const config = GatewayConfig(baseUrl: 'http://api.test');
  const auth = 'Bearer access-token';

  group('VoiceFriendsClient.listFriends', () {
    test('GET /api/v1/friends', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'GET');
        expect(req.url.path, '/api/v1/friends');
        expect(req.headers['Authorization'], auth);
        return http.Response(
          jsonEncode({
            'friend_list': {
              'friends': [
                {'profile_id': 'friend-1'},
              ],
              'next_cursor': '',
            },
          }),
          200,
        );
      });
      final client = VoiceFriendsClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.listFriends(authorization: auth);
      expect(r, isA<FriendsApiOk<FriendsListData>>());
      expect((r as FriendsApiOk<FriendsListData>).data.friends, ['friend-1']);
    });
  });

  group('VoiceFriendsClient.listFriendRequests', () {
    test('GET /api/v1/friends/requests', () async {
      final mock = MockClient((req) async {
        expect(req.url.path, '/api/v1/friends/requests');
        return http.Response(
          jsonEncode({
            'friend_request_list': {
              'incoming': [
                {'profile_id': 'req-in-1'},
              ],
              'outgoing': [
                {'profile_id': 'req-out-1'},
              ],
            },
          }),
          200,
        );
      });
      final client = VoiceFriendsClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.listFriendRequests(authorization: auth);
      expect(r, isA<FriendsApiOk<FriendRequestsData>>());
      final data = (r as FriendsApiOk<FriendRequestsData>).data;
      expect(data.incoming, ['req-in-1']);
      expect(data.outgoing, ['req-out-1']);
    });
  });

  group('VoiceFriendsClient.sendFriendInvitation', () {
    test('POST /api/v1/friends/invitations', () async {
      String? capturedBody;
      final mock = MockClient((req) async {
        expect(req.method, 'POST');
        expect(req.url.path, '/api/v1/friends/invitations');
        capturedBody = req.body;
        return http.Response('{}', 200);
      });
      final client = VoiceFriendsClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.sendFriendInvitation(
        authorization: auth,
        targetProfileId: 'target-p',
      );
      expect(r, isA<FriendsApiEmpty>());
      final body = jsonDecode(capturedBody!) as Map<String, dynamic>;
      expect(body['target_profile_id'], 'target-p');
    });
  });

  group('VoiceFriendsClient.acceptFriendInvitation', () {
    test('POST /api/v1/friends/invitations/{id}/accept', () async {
      final mock = MockClient((req) async {
        expect(req.url.path, '/api/v1/friends/invitations/req-1/accept');
        expect(req.method, 'POST');
        return http.Response('{}', 200);
      });
      final client = VoiceFriendsClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.acceptFriendInvitation(
        authorization: auth,
        requesterProfileId: 'req-1',
      );
      expect(r, isA<FriendsApiEmpty>());
    });
  });

  group('VoiceFriendsClient.declineFriendInvitation', () {
    test('POST /api/v1/friends/invitations/{id}/decline', () async {
      final mock = MockClient((req) async {
        expect(req.url.path, '/api/v1/friends/invitations/req-2/decline');
        return http.Response('{}', 200);
      });
      final client = VoiceFriendsClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.declineFriendInvitation(
        authorization: auth,
        requesterProfileId: 'req-2',
      );
      expect(r, isA<FriendsApiEmpty>());
    });
  });

  group('VoiceFriendsClient.listContacts', () {
    test('GET /api/v1/friends/contacts', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'GET');
        expect(req.url.path, '/api/v1/friends/contacts');
        return http.Response(
          jsonEncode({
            'contact_list': {
              'contacts': [
                {
                  'profile_id': 'contact-1',
                  'source': 'manual',
                  'is_favorite': true,
                },
              ],
              'next_cursor': 'c2',
            },
          }),
          200,
        );
      });
      final client = VoiceFriendsClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.listContacts(authorization: auth);
      expect(r, isA<FriendsApiOk<ContactsListData>>());
      final data = (r as FriendsApiOk<ContactsListData>).data;
      expect(data.contacts.single.profileId, 'contact-1');
      expect(data.contacts.single.isFavorite, isTrue);
      expect(data.nextCursor, 'c2');
    });
  });

  group('VoiceFriendsClient.addContact', () {
    test('POST /api/v1/friends/contacts', () async {
      String? capturedBody;
      final mock = MockClient((req) async {
        expect(req.method, 'POST');
        expect(req.url.path, '/api/v1/friends/contacts');
        capturedBody = req.body;
        return http.Response('{}', 200);
      });
      final client = VoiceFriendsClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.addContact(
        authorization: auth,
        targetProfileId: 'p-1',
      );
      expect(r, isA<FriendsApiEmpty>());
      final body = jsonDecode(capturedBody!) as Map<String, dynamic>;
      expect(body['target_profile_id'], 'p-1');
      expect(body['source'], 'manual');
    });
  });

  group('VoiceFriendsClient.listFavorites', () {
    test('GET /api/v1/friends/favorites', () async {
      final mock = MockClient((req) async {
        expect(req.url.path, '/api/v1/friends/favorites');
        return http.Response(
          jsonEncode({
            'friend_list': {
              'friends': [
                {'profile_id': 'fav-1'},
              ],
            },
          }),
          200,
        );
      });
      final client = VoiceFriendsClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.listFavorites(authorization: auth);
      expect(r, isA<FriendsApiOk<FavoritesListData>>());
      expect((r as FriendsApiOk<FavoritesListData>).data.favorites, ['fav-1']);
    });
  });

  group('VoiceFriendsClient.setFavorite', () {
    test('POST /api/v1/friends/favorites', () async {
      String? capturedBody;
      final mock = MockClient((req) async {
        expect(req.method, 'POST');
        expect(req.url.path, '/api/v1/friends/favorites');
        capturedBody = req.body;
        return http.Response('{}', 200);
      });
      final client = VoiceFriendsClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.setFavorite(
        authorization: auth,
        friendProfileId: 'p-fav',
        favorite: true,
      );
      expect(r, isA<FriendsApiEmpty>());
      final body = jsonDecode(capturedBody!) as Map<String, dynamic>;
      expect(body['friend_profile_id'], 'p-fav');
      expect(body['favorite'], isTrue);
    });
  });

  group('VoiceFriendsClient.listBlocked', () {
    test('GET /api/v1/friends/blocks', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'GET');
        expect(req.url.path, '/api/v1/friends/blocks');
        return http.Response(
          jsonEncode({
            'blocked_list': {
              'blocked': [
                {'blocked_account_id': 'acc-1'},
              ],
            },
          }),
          200,
        );
      });
      final client = VoiceFriendsClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.listBlocked(authorization: auth);
      expect(r, isA<FriendsApiOk<BlockedListData>>());
      final data = (r as FriendsApiOk<BlockedListData>).data;
      expect(data.blocked.single.blockedAccountId, 'acc-1');
    });
  });

  group('VoiceFriendsClient.unblockAccount', () {
    test('DELETE /api/v1/friends/blocks/{id}', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'DELETE');
        expect(req.url.path, '/api/v1/friends/blocks/acc-2');
        return http.Response('', 204);
      });
      final client = VoiceFriendsClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.unblockAccount(
        authorization: auth,
        blockedAccountId: 'acc-2',
      );
      expect(r, isA<FriendsApiEmpty>());
    });
  });
}
