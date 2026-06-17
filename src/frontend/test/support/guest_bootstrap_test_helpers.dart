import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/onboarding_controller.dart';
import 'package:voice_frontend/state/guest_bootstrap_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';

import 'test_voice_token_catalog.dart';
import 'auth_test_overrides.dart';
import 'voice_test_theme.dart';

const guestBootstrapAccountId = 'a1b2c3d4-e5f6-7890-abcd-ef1234567890';

String guestRegisterResponseJson() => jsonEncode({
  'session': {
    'access_token': 'guest-access',
    'refresh_token': 'guest-refresh',
    'expires_in_seconds': 900,
    'account_id': guestBootstrapAccountId,
    'profile_id': 'guest-prof',
  },
});

List<Override> guestBootstrapOverrides({
  required Future<http.Response> Function(http.Request request) onRequest,
}) {
  return [
    voiceMaterialThemeProvider.overrideWith((ref) async => voiceTestTheme()),
    profileAccentStorageProvider.overrideWithValue(testProfileAccentStorage),
    authSessionStorageProvider.overrideWithValue(InMemoryAuthSessionStorage()),
    guestCredentialsStorageProvider.overrideWithValue(
      InMemoryGuestCredentialsStorage(),
    ),
    gatewayConfigProvider.overrideWithValue(
      const GatewayConfig(baseUrl: 'http://api.test'),
    ),
    onboardingControllerProvider.overrideWith(
      () => _CompletedOnboardingController(),
    ),
    realtimeAutoConnectProvider.overrideWithValue(false),
    webGuestAutoRegisterEnabledProvider.overrideWithValue(false),
    httpClientProvider.overrideWithValue(MockClient(onRequest)),
  ];
}

class _CompletedOnboardingController extends TestCompletedOnboardingController {}
