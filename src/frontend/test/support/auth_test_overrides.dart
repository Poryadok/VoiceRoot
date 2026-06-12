import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:http/http.dart' as http;
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/discover_hint_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/backend/message_cache/in_memory_message_cache_store.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/message_cache_providers.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/theme/profile_accent_storage.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';

import 'test_voice_token_catalog.dart';

final testDiscoverHintStorage = InMemoryDiscoverHintStorage();
final testProfileAccentStorage = InMemoryProfileAccentStorage();

List<Override> voiceAppTestOverrides({required http.Client client}) => [
  ...voiceThemeTestOverrides(),
  profileAccentStorageProvider.overrideWithValue(testProfileAccentStorage),
  authSessionStorageProvider.overrideWithValue(InMemoryAuthSessionStorage()),
  discoverHintStorageProvider.overrideWithValue(testDiscoverHintStorage),
  authControllerProvider.overrideWith(authenticatedAuthController),
  gatewayConfigProvider.overrideWithValue(
    const GatewayConfig(baseUrl: 'http://localhost:9999'),
  ),
  httpClientProvider.overrideWithValue(client),
  realtimeAutoConnectProvider.overrideWithValue(false),
  messageCacheStoreProvider.overrideWithValue(InMemoryMessageCacheStore()),
  isDeviceOfflineProvider.overrideWith((ref) => false),
  mySpacesProvider.overrideWith((_) async => const SpaceListData(spaces: [])),
];

/// Pre-authenticated [AuthController] for widget tests of the main shell.
AuthController authenticatedAuthController(Ref ref) {
  final controller = AuthController(
    authClient: ref.watch(voiceAuthClientProvider),
    storage: ref.watch(authSessionStorageProvider),
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
