import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/social/profile_detail_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets('profile detail shows Remove from friends for existing friend', (
    tester,
  ) async {
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
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          realtimeAutoConnectProvider.overrideWithValue(false),
          httpClientProvider.overrideWithValue(
            MockClient((req) async {
              if (req.url.path == '/api/v1/users/profiles/p-friend') {
                return http.Response(
                  jsonEncode({
                    'profile': {
                      'id': 'p-friend',
                      'account_id': 'a-friend',
                      'username': 'bob',
                      'discriminator': '0001',
                      'display_name': 'Bob',
                      'locale': 'en',
                      'theme': 'dark',
                      'is_primary': true,
                      'verification_type': 'none',
                    },
                  }),
                  200,
                );
              }
              if (req.url.path == '/api/v1/users/profiles/p-friend/presence') {
                return http.Response(
                  jsonEncode({
                    'presenceStatus': {'profileId': 'p-friend', 'status': 'online'},
                  }),
                  200,
                );
              }
              if (req.url.path == '/api/v1/friends/requests') {
                return http.Response(
                  jsonEncode({
                    'friend_request_list': {'incoming': [], 'outgoing': []},
                  }),
                  200,
                );
              }
              if (req.url.path == '/api/v1/friends') {
                return http.Response(
                  jsonEncode({
                    'friend_list': {'profile_ids': ['p-friend']},
                  }),
                  200,
                );
              }
              return http.Response('{}', 200);
            }),
          ),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: ProfileDetailSheet(profileId: 'p-friend')),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('profile_remove_friend')), findsOneWidget);
    expect(find.text('Remove from friends'), findsOneWidget);
    expect(find.byKey(ProfileDetailSheet.addFriendKey), findsNothing);
  });
}
