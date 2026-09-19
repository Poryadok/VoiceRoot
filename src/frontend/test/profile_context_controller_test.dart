import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/backend/matchmaking_client.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/matchmaking_providers.dart';
import 'package:voice_frontend/state/matchmaking_search_controller.dart';
import 'package:voice_frontend/state/profile_context_controller.dart';
import 'package:voice_frontend/state/social_providers.dart';
import 'package:voice_frontend/state/space_providers.dart';

void main() {
  group('profileContextCoordinatorProvider', () {
    test(
      'profile switch cancels active MM search and clears local recovery',
      () async {
        final cancelled = <String>[];
        final harness = _ProfileContextHarness(
          spaces: const SpaceListData(spaces: []),
          onRequest: (request) async {
            if (request.url.path == '/api/v1/matchmaking/search/search-a') {
              cancelled.add(request.headers['authorization'] ?? '');
              return _emptyResponse(204);
            }
            return _emptyResponse(404);
          },
        );
        addTearDown(harness.dispose);
        harness.container.read(profileContextCoordinatorProvider);
        harness.container
            .read(activeSearchSessionProvider.notifier)
            .state = const SearchSessionData(
          id: 'search-a',
          profileId: 'profile-a',
          gameId: 'game-a',
          mode: 'ranked',
          criteriaJson: '{}',
          status: 'searching',
        );
        harness.container
            .read(matchmakingSearchControllerProvider.notifier)
            .showDeclinedRecovery();

        await harness.switchToB();
        await _flush();

        expect(cancelled, ['Bearer token-b']);
        expect(harness.container.read(activeSearchSessionProvider), isNull);
        expect(
          harness.container
              .read(matchmakingSearchControllerProvider)
              .recoveryReason,
          isNull,
        );
      },
    );

    test(
      'profile switch keeps a Space where next profile remains a member',
      () async {
        final harness = _ProfileContextHarness(
          spaces: const SpaceListData(
            spaces: [
              VoiceSpace(
                id: 'space-a',
                name: 'Shared',
                visibility: 'private',
                ownerProfileId: 'profile-a',
              ),
            ],
          ),
        );
        addTearDown(harness.dispose);
        harness.container.read(profileContextCoordinatorProvider);
        harness.container.read(selectedSpaceIdProvider.notifier).state =
            'space-a';
        harness.container.read(selectedChatIdProvider.notifier).state =
            'chat-a';

        await harness.switchToB();
        await _flush();

        expect(harness.container.read(selectedSpaceIdProvider), 'space-a');
        expect(harness.container.read(selectedChatIdProvider), 'chat-a');
      },
    );

    test(
      'profile switch exits a Space absent from next profile membership',
      () async {
        final harness = _ProfileContextHarness(
          spaces: const SpaceListData(spaces: []),
        );
        addTearDown(harness.dispose);
        harness.container.read(profileContextCoordinatorProvider);
        harness.container.read(selectedSpaceIdProvider.notifier).state =
            'space-a';
        harness.container.read(selectedChatIdProvider.notifier).state =
            'chat-a';

        await harness.switchToB();
        await _flush();

        expect(harness.container.read(selectedSpaceIdProvider), isNull);
        expect(harness.container.read(selectedChatIdProvider), isNull);
      },
    );
  });
}

Future<void> _flush() => Future<void>.delayed(Duration.zero);

class _ProfileContextHarness {
  _ProfileContextHarness({
    required SpaceListData spaces,
    Future<http.Response> Function(http.Request request)? onRequest,
  }) {
    final client = MockClient(onRequest ?? ((_) async => _emptyResponse(404)));
    final gateway = GatewayHttpClient(
      httpClient: client,
      config: const GatewayConfig(baseUrl: 'http://api.test'),
    );
    final controller = AuthController(
      authClient: VoiceAuthClient(gateway: gateway),
      storage: InMemoryAuthSessionStorage(),
      guestCredentialsStorage: InMemoryGuestCredentialsStorage(),
    )..state = AuthState(session: _session('profile-a', 'token-a'));
    container = ProviderContainer(
      overrides: [
        authControllerProvider.overrideWith((_) => controller),
        authSessionStorageProvider.overrideWithValue(
          InMemoryAuthSessionStorage(),
        ),
        guestCredentialsStorageProvider.overrideWithValue(
          InMemoryGuestCredentialsStorage(),
        ),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        httpClientProvider.overrideWithValue(client),
        activeProfileProvider.overrideWith((_) async => null),
        profileProvider.overrideWith(
          (_, profileId) async => VoiceProfile(
            id: profileId,
            accountId: 'account-a',
            username: profileId,
            discriminator: '0001',
            displayName: profileId,
            accentColor: '#123456',
          ),
        ),
        realtimeEventProvider.overrideWith((_) => const Stream.empty()),
        mySpacesProvider.overrideWith((_) async => spaces),
      ],
    );
  }

  late final ProviderContainer container;

  Future<void> switchToB() => container
      .read(authControllerProvider.notifier)
      .applySession(_session('profile-b', 'token-b'));

  void dispose() => container.dispose();
}

AuthSession _session(String profileId, String token) => AuthSession(
  accessToken: token,
  refreshToken: 'refresh-$profileId',
  accountId: 'account-a',
  activeProfileId: profileId,
  expiresInSeconds: 900,
);

http.Response _emptyResponse(int statusCode) => http.Response('', statusCode);
