import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/app.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/discover_hint_storage.dart';
import 'package:voice_frontend/backend/friends_client.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/message_cache/in_memory_message_cache_store.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/l10n/app_localizations_en.dart';
import 'package:voice_frontend/services/windows_desktop_host.dart';
import 'package:voice_frontend/settings/voice_input_settings.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/message_cache_providers.dart';
import 'package:voice_frontend/state/social_providers.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/state/version_policy_providers.dart';
import 'package:voice_frontend/routing/deep_link_listener.dart';
import 'package:voice_frontend/shell/three_column_shell.dart';
import 'package:voice_frontend/theme/profile_accent_storage.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/auth/auth_screen.dart';
import 'package:voice_frontend/ui/shell/chat_list_body.dart';

import 'support/fake_voice_api_clients.dart';
import 'support/voice_test_theme.dart';

/// Mounted regular-registration proof for docs/features/onboarding.md.
///
/// The live variant is intentionally not encoded yet: the repository plan
/// requires an admission invite for email registration, while AuthScreen has
/// no invite input, and the auth feature docs describe a restricted pending
/// session before email verification. The offline contract below therefore
/// proves the client-visible flow without inventing either bypass.
void main() {
  testWidgets('visible registration opens onboarding and enters chat list', (
    tester,
  ) async {
    final requests = <http.Request>[];
    var onboardingDismissCalls = 0;
    final client = MockClient((request) async {
      requests.add(request);
      final path = request.url.path;

      if (path == '/api/v1/auth/register' && request.method == 'POST') {
        return _jsonResponse({
          'session': {
            'access_token': 'registration-access',
            'refresh_token': 'registration-refresh',
            'expires_in_seconds': 900,
            'account_id': 'account-visible-registration',
            'profile_id': 'profile-visible-registration',
          },
        });
      }
      if (path == '/api/v1/users/profiles/profile-visible-registration') {
        return _jsonResponse({
          'profile': {
            'id': 'profile-visible-registration',
            'account_id': 'account-visible-registration',
            'username': 'visibleuser',
            'discriminator': '0001',
            'display_name': 'Visible User',
            'locale': 'en',
            'theme': 'dark',
            'is_primary': true,
            'verification_type': 'none',
          },
        });
      }
      if (path == '/api/v1/users/me/onboarding' && request.method == 'GET') {
        return _jsonResponse(_onboardingJson(completed: false));
      }
      if (path == '/api/v1/users/me/onboarding/steps' &&
          request.method == 'POST') {
        final body = jsonDecode(request.body) as Map<String, dynamic>;
        expect(body['step_id'], 'dismiss');
        onboardingDismissCalls++;
        return _jsonResponse(
          _onboardingJson(completed: true, completedSteps: const ['dismiss']),
        );
      }
      if (path == '/health') {
        return http.Response('ok', 200);
      }
      return http.Response(
        '{}',
        200,
        headers: {'content-type': 'application/json'},
      );
    });

    await tester.binding.setSurfaceSize(const Size(1280, 800));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final container = ProviderContainer(
      overrides: [
        voiceMaterialThemeProvider.overrideWith(
          (ref) async => voiceTestTheme(),
        ),
        profileAccentStorageProvider.overrideWithValue(
          InMemoryProfileAccentStorage(),
        ),
        authSessionStorageProvider.overrideWithValue(
          InMemoryAuthSessionStorage(),
        ),
        guestCredentialsStorageProvider.overrideWithValue(
          InMemoryGuestCredentialsStorage(),
        ),
        discoverHintStorageProvider.overrideWithValue(
          InMemoryDiscoverHintStorage(),
        ),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        httpClientProvider.overrideWithValue(client),
        voiceChatsClientProvider.overrideWithValue(FakeVoiceChatsClient()),
        voiceMessagesClientProvider.overrideWithValue(
          FakeVoiceMessagesClient(),
        ),
        realtimeAutoConnectProvider.overrideWithValue(false),
        messageCacheStoreProvider.overrideWithValue(
          InMemoryMessageCacheStore(),
        ),
        connectivityWatcherProvider.overrideWith((ref) {}),
        isDeviceOfflineProvider.overrideWith((ref) => false),
        mySpacesProvider.overrideWith(
          (ref) async => const SpaceListData(spaces: []),
        ),
        deepLinkListenerProvider.overrideWith(_NoopDeepLinkListener.new),
        versionPolicyProvider.overrideWith(
          (ref) => VersionPolicyController(ref, enablePolling: false),
        ),
        windowsDesktopHostProvider.overrideWithValue(
          const NoopWindowsDesktopHost(),
        ),
        voiceInputSettingsProvider.overrideWith(
          _TestVoiceInputSettingsNotifier.new,
        ),
        friendRequestsProvider.overrideWith(
          (ref) => const FriendRequestsData(incoming: [], outgoing: []),
        ),
      ],
    );
    addTearDown(() async {
      await container.read(authControllerProvider.notifier).logout();
      container.dispose();
      client.close();
    });

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: const VoiceApp(locale: Locale('en')),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.byKey(AuthScreen.screenKey), findsOneWidget);
    expect(container.read(authControllerProvider).isAuthenticated, isFalse);

    await tester.enterText(
      find.byKey(AuthScreen.emailFieldKey),
      'visible-registration@example.test',
    );
    await tester.enterText(
      find.byKey(AuthScreen.passwordFieldKey),
      'VoiceQaTest1!',
    );
    await tester.tap(find.byKey(AuthScreen.registerButtonKey));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.byKey(AuthScreen.screenKey), findsNothing);
    expect(
      find.text(AppLocalizationsEn().onboardingSaveAccountTitle),
      findsOneWidget,
    );

    await tester.tap(
      find.widgetWithText(TextButton, AppLocalizationsEn().onboardingSkip),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    await tester.pump(const Duration(milliseconds: 100));

    expect(onboardingDismissCalls, 1);
    expect(
      find.text(AppLocalizationsEn().onboardingSaveAccountTitle),
      findsNothing,
    );
    expect(find.byKey(ThreeColumnShell.navActiveRail), findsOneWidget);
    expect(find.byKey(ChatListBody.listKey), findsOneWidget);
    expect(find.bySemanticsLabel('Chat list'), findsOneWidget);
    expect(find.text(AppLocalizationsEn().chatListTitle), findsWidgets);
    expect(
      requests.where((request) => request.url.path == '/api/v1/auth/register'),
      hasLength(1),
    );
  });
}

http.Response _jsonResponse(Map<String, dynamic> body) => http.Response(
  jsonEncode(body),
  200,
  headers: {'content-type': 'application/json'},
);

Map<String, dynamic> _onboardingJson({
  required bool completed,
  List<String> completedSteps = const [],
}) => {
  'onboarding_state': {
    'profile_id': 'profile-visible-registration',
    'completed_steps': completedSteps,
    'completed': completed,
  },
};

class _NoopDeepLinkListener extends DeepLinkListener {
  _NoopDeepLinkListener(super.ref);

  @override
  Future<void> start() async {}
}

class _TestVoiceInputSettingsNotifier extends VoiceInputSettingsNotifier {
  @override
  VoiceInputSettings build() => const VoiceInputSettings();

  @override
  Future<void> setMode(VoiceInputMode mode) async {
    state = state.copyWith(mode: mode);
  }

  @override
  Future<void> setPttKey(LogicalKeyboardKey key) async {
    state = state.copyWith(pttKey: key);
  }
}
