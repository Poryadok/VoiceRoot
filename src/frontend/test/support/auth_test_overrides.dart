import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:http/http.dart' as http;
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/discover_hint_storage.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/backend/message_cache/in_memory_message_cache_store.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/message_cache_providers.dart';
import 'package:voice_frontend/state/onboarding_controller.dart';
import 'package:voice_frontend/state/guest_save_account_reminder.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/theme/profile_accent_storage.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';

import 'fake_voice_api_clients.dart';
import 'test_voice_token_catalog.dart';

final testDiscoverHintStorage = InMemoryDiscoverHintStorage();
final testProfileAccentStorage = InMemoryProfileAccentStorage();
final _testRealtimeHub = _NoopRealtimeHub();

List<Override> voiceAppTestOverrides({required http.Client client}) => [
  ...voiceThemeTestOverrides(),
  profileAccentStorageProvider.overrideWithValue(testProfileAccentStorage),
  authSessionStorageProvider.overrideWithValue(InMemoryAuthSessionStorage()),
  guestCredentialsStorageProvider.overrideWithValue(
    InMemoryGuestCredentialsStorage(),
  ),
  discoverHintStorageProvider.overrideWithValue(testDiscoverHintStorage),
  authControllerProvider.overrideWith(authenticatedAuthController),
  gatewayConfigProvider.overrideWithValue(
    const GatewayConfig(baseUrl: 'http://localhost:9999'),
  ),
  httpClientProvider.overrideWithValue(client),
  voiceChatsClientProvider.overrideWithValue(FakeVoiceChatsClient()),
  voiceMessagesClientProvider.overrideWithValue(FakeVoiceMessagesClient()),
  realtimeAutoConnectProvider.overrideWithValue(false),
  realtimeHubProvider.overrideWithValue(_testRealtimeHub),
  messageCacheStoreProvider.overrideWithValue(InMemoryMessageCacheStore()),
  isDeviceOfflineProvider.overrideWith((ref) => false),
  mySpacesProvider.overrideWith((_) async => const SpaceListData(spaces: [])),
];

/// Onboarding already completed — avoids overlay dialogs in shell widget tests.
class TestCompletedOnboardingController extends OnboardingController {
  @override
  OnboardingUiState build() => const OnboardingUiState(completed: true);
}

List<Override> guestShellTestOverrides({required http.Client client}) => [
  ...voiceAppTestOverrides(client: client),
  onboardingControllerProvider.overrideWith(TestCompletedOnboardingController.new),
  guestSaveAccountReminderVisibleProvider.overrideWith((ref) async => true),
];

/// Pre-authenticated [AuthController] for widget tests of the main shell.
AuthController authenticatedAuthController(Ref ref) {
  final controller = AuthController(
    authClient: ref.watch(voiceAuthClientProvider),
    storage: ref.watch(authSessionStorageProvider),
    guestCredentialsStorage: ref.watch(guestCredentialsStorageProvider),
  );
  controller.state = const AuthState(
    session: AuthSession(
      accessToken: 'test-access',
      refreshToken: 'test-refresh',
      accountId: 'acc-test',
      activeProfileId: 'prof-test',
      expiresInSeconds: 900,
    ),
  );
  return controller;
}

class _NoopRealtimeHub extends RealtimeHub {
  _NoopRealtimeHub() : super(_UnwiredRef());

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}
}

class _UnwiredRef implements Ref {
  @override
  dynamic noSuchMethod(Invocation invocation) => throw UnimplementedError();
}
