import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/profile/profile_edit_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets('saves display name and bio through profile actions', (
    tester,
  ) async {
    Map<String, dynamic>? patchBody;
    final client = MockClient((req) async {
      if (req.url.path == '/api/v1/users/me' && req.method == 'PATCH') {
        patchBody = jsonDecode(req.body) as Map<String, dynamic>;
        return http.Response(
          jsonEncode({
            'profile': {
              'id': 'prof-test',
              'account_id': 'acc-test',
              'username': 'voiceuser',
              'discriminator': '4242',
              'display_name': patchBody!['display_name'],
              'bio': patchBody!['bio'],
              'locale': 'en',
              'theme': 'dark',
              'is_primary': true,
              'verification_type': 'none',
            },
          }),
          200,
        );
      }
      return http.Response('Not Found', 404);
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          discoverHintStorageProvider.overrideWithValue(
            testDiscoverHintStorage,
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(client),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: ProfileEditSheet(
              profile: VoiceProfile(
                id: 'prof-test',
                accountId: 'acc-test',
                username: 'voiceuser',
                discriminator: '4242',
                displayName: 'Voice User',
                bio: 'Old bio',
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(ProfileEditSheet.displayNameFieldKey),
      'Voice Renamed',
    );
    await tester.enterText(
      find.byKey(ProfileEditSheet.bioFieldKey),
      'Looking for a duo',
    );
    await tester.tap(find.byKey(ProfileEditSheet.saveButtonKey));
    await tester.pumpAndSettle();

    expect(patchBody, {
      'display_name': 'Voice Renamed',
      'bio': 'Looking for a duo',
    });
  });
}
