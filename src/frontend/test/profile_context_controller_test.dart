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
import 'package:voice_frontend/state/mobile_opened_chat_strip.dart';
import 'package:voice_frontend/state/profile_context_controller.dart';
import 'package:voice_frontend/state/social_providers.dart';
import 'package:voice_frontend/state/shell_providers.dart';
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

    test(
      'logout clears auth-scoped navigation before account B logs in',
      () async {
        final harness = _ProfileContextHarness(
          spaces: const SpaceListData(spaces: []),
        );
        addTearDown(harness.dispose);
        harness.container.read(profileContextCoordinatorProvider);
        harness.container.read(shellNavigationProvider);
        _selectOldContext(harness.container);

        final auth = harness.container.read(authControllerProvider.notifier);
        await auth.logout();

        expect(harness.container.read(selectedSpaceIdProvider), isNull);
        expect(harness.container.read(selectedChatIdProvider), isNull);
        expect(harness.container.read(selectedChatFolderIdProvider), isNull);
        expect(harness.container.read(chatInboxProvider), 'main');
        expect(
          harness.container.read(shellSidePanelProvider),
          ShellSidePanel.none,
        );
        expect(harness.container.read(mobileOpenedChatStripProvider), isEmpty);
        expect(harness.container.read(chatListScrollOffsetProvider), 0);
        expect(harness.container.read(chatListScrollRestoreProvider), isFalse);

        await auth.applySession(
          _session('profile-b', 'token-b', accountId: 'account-b'),
        );

        expect(harness.container.read(selectedSpaceIdProvider), isNull);
        expect(harness.container.read(selectedChatIdProvider), isNull);
        expect(harness.container.read(selectedChatFolderIdProvider), isNull);
        expect(harness.container.read(chatInboxProvider), 'main');
        expect(harness.container.read(mobileOpenedChatStripProvider), isEmpty);
      },
    );

    test(
      'logout clears auth-scoped navigation before a guest starts',
      () async {
        final harness = _ProfileContextHarness(
          spaces: const SpaceListData(spaces: []),
        );
        addTearDown(harness.dispose);
        harness.container.read(profileContextCoordinatorProvider);
        harness.container.read(shellNavigationProvider);
        _selectOldContext(harness.container);

        final auth = harness.container.read(authControllerProvider.notifier);
        await auth.logout();
        final guest = _session(
          'guest-profile',
          'guest-token',
          accountId: 'guest-account',
        );
        await auth.applySession(guest);
        auth.state = auth.state.copyWith(isGuest: true);

        expect(harness.container.read(selectedSpaceIdProvider), isNull);
        expect(harness.container.read(selectedChatIdProvider), isNull);
        expect(harness.container.read(selectedChatFolderIdProvider), isNull);
        expect(harness.container.read(chatInboxProvider), 'main');
      },
    );

    test(
      'late old-account Space membership response cannot clear new account selection',
      () async {
        final requestStarted = Completer<void>();
        final spacesResponse = Completer<SpaceListData>();
        final harness = _ProfileContextHarness(
          spaces: const SpaceListData(spaces: []),
          onSpaces: () {
            if (!requestStarted.isCompleted) requestStarted.complete();
            return spacesResponse.future;
          },
        );
        addTearDown(harness.dispose);
        harness.container.read(profileContextCoordinatorProvider);
        harness.container.read(shellNavigationProvider);
        _selectOldContext(harness.container);

        final auth = harness.container.read(authControllerProvider.notifier);
        await harness.switchToB();
        await requestStarted.future;

        await auth.logout();
        await auth.applySession(
          _session('profile-c', 'token-c', accountId: 'account-c'),
        );
        harness.container.read(selectedSpaceIdProvider.notifier).state =
            'space-c';
        harness.container.read(selectedChatIdProvider.notifier).state =
            'chat-c';

        spacesResponse.complete(const SpaceListData(spaces: []));
        await _flush();

        expect(harness.container.read(selectedSpaceIdProvider), 'space-c');
        expect(harness.container.read(selectedChatIdProvider), 'chat-c');
      },
    );
  });
}

void _selectOldContext(ProviderContainer container) {
  container.read(selectedSpaceIdProvider.notifier).state = 'space-a';
  container.read(selectedChatIdProvider.notifier).state = 'chat-a';
  container.read(selectedChatFolderIdProvider.notifier).state = 'folder-a';
  container.read(chatInboxProvider.notifier).state = 'requests';
  container.read(shellSidePanelProvider.notifier).state =
      ShellSidePanel.members;
  container.read(mobileOpenedChatStripProvider.notifier).openChat('chat-a');
  container.read(chatListScrollOffsetProvider.notifier).state = 42;
  container.read(chatListScrollRestoreProvider.notifier).state = true;
}

Future<void> _flush() => Future<void>.delayed(Duration.zero);

class _ProfileContextHarness {
  _ProfileContextHarness({
    required SpaceListData spaces,
    Future<http.Response> Function(http.Request request)? onRequest,
    Future<SpaceListData> Function()? onSpaces,
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
        mySpacesProvider.overrideWith(
          (_) => onSpaces?.call() ?? Future.value(spaces),
        ),
      ],
    );
  }

  late final ProviderContainer container;

  Future<void> switchToB() => container
      .read(authControllerProvider.notifier)
      .applySession(_session('profile-b', 'token-b'));

  void dispose() => container.dispose();
}

AuthSession _session(
  String profileId,
  String token, {
  String accountId = 'account-a',
}) => AuthSession(
  accessToken: token,
  refreshToken: 'refresh-$profileId',
  accountId: accountId,
  activeProfileId: profileId,
  expiresInSeconds: 900,
);

http.Response _emptyResponse(int statusCode) => http.Response('', statusCode);
