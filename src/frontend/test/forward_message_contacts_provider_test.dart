import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/social_providers.dart';
import 'package:voice_frontend/ui/chat/forward_message_contacts_provider.dart';

import 'support/auth_test_overrides.dart';

void main() {
  ProviderContainer createContainer(http.Client client) {
    return ProviderContainer(
      overrides: [
        authSessionStorageProvider.overrideWithValue(
          InMemoryAuthSessionStorage(),
        ),
        authControllerProvider.overrideWith(authenticatedAuthController),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        httpClientProvider.overrideWithValue(client),
      ],
    );
  }

  test('loads and deduplicates accepted friend IDs across every cursor page', () async {
    final requestedCursors = <String?>[];
    final container = createContainer(
      MockClient((req) async {
        expect(req.url.path, '/api/v1/friends');
        requestedCursors.add(req.url.queryParameters['cursor']);
        final page = requestedCursors.length;
        return http.Response(
          jsonEncode({
            'friend_list': {
              'friends': page == 1
                  ? [
                      {'profile_id': 'friend-a'},
                      {'profile_id': 'friend-b'},
                    ]
                  : [
                      {'profile_id': 'friend-b'},
                      {'profile_id': 'friend-c'},
                    ],
              'next_cursor': page == 1 ? 'cursor-2' : '',
            },
          }),
          200,
        );
      }),
    );
    addTearDown(container.dispose);

    final ids = await container.read(
      forwardMessageAcceptedFriendIdsProvider.future,
    );

    expect(requestedCursors, [null, 'cursor-2']);
    expect(ids, ['friend-a', 'friend-b', 'friend-c']);
  });

  test('does not present a partial contact list when a later page fails', () async {
    var page = 0;
    final container = createContainer(
      MockClient((req) async {
        page++;
        return page == 1
            ? http.Response(
                jsonEncode({
                  'friend_list': {
                    'friends': [
                      {'profile_id': 'friend-a'},
                    ],
                    'next_cursor': 'cursor-2',
                  },
                }),
                200,
              )
            : http.Response('{}', 503);
      }),
    );
    addTearDown(container.dispose);

    await expectLater(
      container.read(forwardMessageAcceptedFriendIdsProvider.future),
      throwsA(isA<StateError>()),
    );
    expect(page, 2);
  });

  test('rejects a repeated pagination cursor instead of looping', () async {
    var page = 0;
    final container = createContainer(
      MockClient((_) async {
        page++;
        return http.Response(
          jsonEncode({
            'friend_list': {
              'friends': [
                {'profile_id': 'friend-$page'},
              ],
              'next_cursor': 'cursor-same',
            },
          }),
          200,
        );
      }),
    );
    addTearDown(container.dispose);

    await expectLater(
      container.read(forwardMessageAcceptedFriendIdsProvider.future),
      throwsA(isA<StateError>()),
    );
    expect(page, 2);
  });

}
