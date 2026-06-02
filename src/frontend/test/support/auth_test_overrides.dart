import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:http/http.dart' as http;
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/discover_hint_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/profile_accent_storage.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';

import 'test_voice_token_catalog.dart';

final testDiscoverHintStorage = InMemoryDiscoverHintStorage();
final testProfileAccentStorage = InMemoryProfileAccentStorage();

List<Override> voiceAppTestOverrides({required http.Client client}) => [
      ...voiceThemeTestOverrides(),
      profileAccentStorageProvider.overrideWithValue(testProfileAccentStorage),
      authSessionStorageProvider.overrideWithValue(
        InMemoryAuthSessionStorage(),
      ),
      discoverHintStorageProvider.overrideWithValue(testDiscoverHintStorage),
      authControllerProvider.overrideWith(authenticatedAuthController),
      gatewayConfigProvider.overrideWithValue(
        const GatewayConfig(baseUrl: 'http://localhost:9999'),
      ),
      httpClientProvider.overrideWithValue(client),
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
