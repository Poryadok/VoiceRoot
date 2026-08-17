import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:integration_test/integration_test.dart';
import 'package:voice_frontend/app.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/notifications_client.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/routing/app_router.dart';
import 'package:voice_frontend/routing/deep_link_controller.dart';
import 'package:voice_frontend/routing/deep_link_parser.dart';
import 'package:voice_frontend/shell/three_column_shell.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';
import 'package:voice_frontend/state/deep_link_navigation.dart';
import 'package:voice_frontend/state/push_notifications.dart';
import 'package:voice_frontend/state/push_notifications_bootstrap.dart';
import 'package:voice_frontend/state/shared_media_providers.dart';
import 'package:voice_frontend/state/shell_providers.dart';
import 'package:voice_frontend/state/social_providers.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/state/voip_push_platform.dart';
import 'package:voice_frontend/ui/social/profile_detail_sheet.dart';

import '../test/support/auth_test_overrides.dart';
import '../test/support/fake_voice_api_clients.dart';
import '../test/support/gateway_test_client.dart';
import '../test/support/voice_test_theme.dart';

/// P3.1 device-driver scaffold (inventory PL-04 / DL-04).
///
/// Runs under [IntegrationTestWidgetsFlutterBinding] on the host
/// `flutter_tester` (CI job `flutter-device-driver`). Covers:
/// - deep-link matrix via `apply(parseDeepLinkUrl)` (invite / space / profile /
///   DM / `voice://` / pending-after-login)
/// - push / VoIP token register contracts via Gateway mock
///
/// Host tester does **not** close DL-04 (App Links / AASA on-device).
///
/// Still out of CI (honest gaps):
/// - NT-05 real-device alert delivery (staging secrets)
/// - DL-04 mobile App Links / AASA on-device acceptance
/// - CallKit / PushKit / LiveKit media paths on a physical device
void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  group('device driver — deep link', () {
    testWidgets('chat message deep link opens conversation region', (
      tester,
    ) async {
      await bindDesktopTestViewport(tester);
      final env = await _pumpDriverShell(tester);

      await env.container
          .read(deepLinkNavigatorProvider)
          .apply(
            parseDeepLinkUrl(
              'https://voice.gg/ch/device-driver-chat/m/device-driver-msg',
            ),
          );
      await _pumpShellReady(tester);

      expect(env.container.read(selectedChatIdProvider), 'device-driver-chat');
      expect(
        env.container.read(
          pendingChatMessageScrollProvider('device-driver-chat'),
        ),
        'device-driver-msg',
      );
      expect(find.byKey(ThreeColumnShell.navOpenChat), findsOneWidget);
      expect(find.bySemanticsLabel('Conversation'), findsOneWidget);
    });

    testWidgets('space deep link selects space', (tester) async {
      await bindDesktopTestViewport(tester);
      final env = await _pumpDriverShell(tester);

      await env.container
          .read(deepLinkNavigatorProvider)
          .apply(parseDeepLinkUrl('https://voice.gg/s/device-driver-space'));
      await _pumpShellReady(tester);

      expect(
        env.container.read(selectedSpaceIdProvider),
        'device-driver-space',
      );
    });

    testWidgets('invite deep link joins by invite code', (tester) async {
      await bindDesktopTestViewport(tester);
      final spaces = _RecordingSpacesClient();
      final env = await _pumpDriverShell(
        tester,
        extraOverrides: [voiceSpacesClientProvider.overrideWithValue(spaces)],
      );

      await env.container
          .read(deepLinkNavigatorProvider)
          .apply(
            parseDeepLinkUrl('https://voice.gg/invite/device-driver-code'),
          );
      await _pumpShellReady(tester);

      expect(spaces.lastJoinCode, 'device-driver-code');
    });

    testWidgets('DM deep link opens conversation', (tester) async {
      await bindDesktopTestViewport(tester);
      final env = await _pumpDriverShell(
        tester,
        extraOverrides: [
          voiceChatsClientProvider.overrideWithValue(
            FakeVoiceChatsClient(dmChatId: 'device-driver-dm'),
          ),
        ],
      );

      await env.container
          .read(deepLinkNavigatorProvider)
          .apply(parseDeepLinkUrl('https://voice.gg/dm/device-driver-peer'));
      await _pumpShellReady(tester);

      expect(env.container.read(selectedChatIdProvider), 'device-driver-dm');
      expect(
        env.container.read(navigationSectionProvider),
        NavigationSection.chats,
      );
      expect(find.bySemanticsLabel('Conversation'), findsOneWidget);
    });

    testWidgets('profile deep link opens social profile sheet', (tester) async {
      await bindDesktopTestViewport(tester);
      final users = _FakeUsersClient();
      final env = await _pumpDriverShell(
        tester,
        extraOverrides: [voiceUsersClientProvider.overrideWithValue(users)],
      );

      final applyFuture = env.container
          .read(deepLinkNavigatorProvider)
          .apply(parseDeepLinkUrl('https://voice.gg/u/alice'));
      await _pumpShellReady(tester);

      expect(users.lastSearchQuery, 'alice');
      expect(
        env.container.read(navigationSectionProvider),
        NavigationSection.social,
      );
      expect(find.byKey(ProfileDetailSheet.sheetKey), findsOneWidget);

      final nav = rootNavigatorKey.currentState;
      if (nav != null && nav.canPop()) {
        nav.pop();
      }
      await applyFuture;
    });

    testWidgets('voice:// custom scheme opens conversation', (tester) async {
      await bindDesktopTestViewport(tester);
      final env = await _pumpDriverShell(tester);

      await env.container
          .read(deepLinkNavigatorProvider)
          .apply(
            parseDeepLinkUrl('voice://ch/voice-scheme-chat/m/voice-scheme-msg'),
          );
      await _pumpShellReady(tester);

      expect(env.container.read(selectedChatIdProvider), 'voice-scheme-chat');
      expect(
        env.container.read(
          pendingChatMessageScrollProvider('voice-scheme-chat'),
        ),
        'voice-scheme-msg',
      );
      expect(find.bySemanticsLabel('Conversation'), findsOneWidget);
    });

    testWidgets('pending deep link applies after login', (tester) async {
      await bindDesktopTestViewport(tester);
      final env = await _pumpDriverShell(
        tester,
        extraOverrides: [
          authControllerProvider.overrideWith(_loggedOutAuthController),
        ],
      );

      final incoming = parseDeepLinkUrl(
        'https://voice.gg/ch/pending-after-login/m/pending-msg',
      );
      await env.container
          .read(deepLinkControllerProvider.notifier)
          .onIncomingLink(incoming);
      expect(env.container.read(deepLinkControllerProvider).pending, incoming);
      expect(env.container.read(deepLinkControllerProvider).resolved, isNull);

      env.container
          .read(authControllerProvider.notifier)
          .state = const AuthState(
        session: AuthSession(
          accessToken: 'test-access',
          refreshToken: 'test-refresh',
          accountId: 'acc-test',
          activeProfileId: 'prof-test',
          expiresInSeconds: 900,
        ),
      );
      await env.container
          .read(deepLinkControllerProvider.notifier)
          .flushPendingAfterAuth();
      final resolved = env.container.read(deepLinkControllerProvider).resolved;
      expect(resolved, incoming);
      expect(env.container.read(deepLinkControllerProvider).pending, isNull);

      await env.container.read(deepLinkNavigatorProvider).apply(resolved!);
      await _pumpShellReady(tester);

      expect(env.container.read(selectedChatIdProvider), 'pending-after-login');
      expect(
        env.container.read(
          pendingChatMessageScrollProvider('pending-after-login'),
        ),
        'pending-msg',
      );
    });
  });

  group('device driver — push register contract', () {
    testWidgets('FCM bootstrap posts register-device to Gateway', (
      tester,
    ) async {
      String? capturedPath;
      Map<String, dynamic>? capturedBody;

      final client = VoiceNotificationsClient(
        gateway: gatewayHttpForTest(
          MockClient((request) async {
            capturedPath = request.url.path;
            capturedBody = jsonDecode(request.body) as Map<String, dynamic>;
            return http.Response('{"device_token_id":"tok-1"}', 200);
          }),
        ),
      );

      const bootstrap = PushNotificationsBootstrap();
      final id = await bootstrap.registerToken(
        client: client,
        authorization: 'Bearer device-driver-jwt',
        platform: 'android',
        token: 'fcm-device-driver-token',
      );

      expect(id, 'tok-1');
      expect(capturedPath, '/api/v1/notifications/register-device');
      expect(capturedBody?['platform'], 'android');
      expect(capturedBody?['token'], 'fcm-device-driver-token');
      expect(capturedBody?['push_service'], 'fcm');
    });

    testWidgets('FCM data maps to realtime notification frame', (tester) async {
      final frame = fcmDataToRealtimeNotification({
        'type': 'new_message',
        'chat_id': 'device-driver-chat',
        'message_id': 'device-driver-msg',
        'sender_profile_id': 'peer-1',
      });
      expect(frame, isNotNull);
      expect(frame!.op, 'notification');
      expect(frame.data?['type'], 'new_message');
      expect(frame.data?['chat_id'], 'device-driver-chat');
    });
  });

  group('device driver — VoIP register contract', () {
    testWidgets('VoIP PushKit service is unavailable on host tester', (
      tester,
    ) async {
      // Physical iOS + PushKit required for isVoIPPushSupported == true.
      expect(voipPushServiceForTarget(), isNull);
      expect(isVoIPPushSupported, isFalse);
    });

    testWidgets('voip_apns bootstrap posts register-device to Gateway', (
      tester,
    ) async {
      Map<String, dynamic>? capturedBody;

      final client = VoiceNotificationsClient(
        gateway: gatewayHttpForTest(
          MockClient((request) async {
            capturedBody = jsonDecode(request.body) as Map<String, dynamic>;
            return http.Response('{}', 200);
          }),
        ),
      );

      const bootstrap = PushNotificationsBootstrap();
      await bootstrap.registerToken(
        client: client,
        authorization: 'Bearer device-driver-jwt',
        platform: 'ios',
        token: 'voip-device-driver-token',
        pushService: 'voip_apns',
      );

      expect(capturedBody?['platform'], 'ios');
      expect(capturedBody?['token'], 'voip-device-driver-token');
      expect(capturedBody?['push_service'], 'voip_apns');
    });
  });
}

class _DriverShell {
  _DriverShell(this.container);
  final ProviderContainer container;
}

Future<_DriverShell> _pumpDriverShell(
  WidgetTester tester, {
  List<Override> extraOverrides = const [],
}) async {
  late ProviderContainer container;
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container = ProviderContainer(
        overrides: [
          ...guestShellTestOverrides(
            client: MockClient((_) async => throw UnimplementedError()),
          ),
          connectivityWatcherProvider.overrideWith((ref) {}),
          ...extraOverrides,
        ],
      ),
      child: const VoiceApp(locale: Locale('en')),
    ),
  );
  await _pumpShellReady(tester);
  return _DriverShell(container);
}

AuthController _loggedOutAuthController(Ref ref) {
  return AuthController(
    authClient: ref.watch(voiceAuthClientProvider),
    storage: ref.watch(authSessionStorageProvider),
    guestCredentialsStorage: ref.watch(guestCredentialsStorageProvider),
    onAuthenticated: () async {
      await ref
          .read(deepLinkControllerProvider.notifier)
          .flushPendingAfterAuth();
    },
  );
}

class _RecordingSpacesClient extends VoiceSpacesClient {
  _RecordingSpacesClient()
    : super(
        gateway: gatewayHttpForTest(
          MockClient((_) async => http.Response('{}', 404)),
        ),
      );

  String? lastJoinCode;

  @override
  Future<SpacesApiResult<SpaceMembershipData>> joinByInvite({
    required String authorization,
    required String code,
  }) async {
    lastJoinCode = code;
    return SpacesApiOk(
      SpaceMembershipData(
        spaceId: 'invite-joined-space',
        profileId: 'prof-test',
        joinedAt: DateTime.utc(2026, 1, 1),
      ),
    );
  }
}

class _FakeUsersClient extends VoiceUsersClient {
  _FakeUsersClient()
    : super(
        gateway: gatewayHttpForTest(
          MockClient((_) async => http.Response('{}', 404)),
        ),
      );

  String? lastSearchQuery;

  static const _alice = VoiceProfile(
    id: 'prof-alice',
    accountId: 'acc-alice',
    username: 'alice',
    discriminator: '0001',
    displayName: 'Alice',
  );

  @override
  Future<UsersApiResult<SearchProfilesData>> searchProfiles({
    required String authorization,
    required String query,
    String? cursor,
    int? pageSize,
  }) async {
    lastSearchQuery = query;
    return const UsersApiOk(SearchProfilesData(profiles: [_alice]));
  }

  @override
  Future<UsersApiResult<VoiceProfile>> getProfile({
    required String authorization,
    required String profileId,
  }) async {
    return const UsersApiOk(_alice);
  }
}

Future<void> _pumpShellReady(WidgetTester tester) async {
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 100));
  await tester.pump(const Duration(milliseconds: 400));
}
