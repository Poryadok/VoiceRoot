import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/social/profile_detail_sheet.dart';
import 'package:voice_frontend/ui/social/qr_add_friend_sheet.dart';
import 'package:voice_frontend/ui/social/social_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  Widget socialTestApp({required Widget home, required http.Client client}) {
    return ProviderScope(
      overrides: [
        ...voiceThemeTestOverrides(),
        profileAccentStorageProvider.overrideWithValue(
          testProfileAccentStorage,
        ),
        authSessionStorageProvider.overrideWithValue(
          InMemoryAuthSessionStorage(),
        ),
        authControllerProvider.overrideWith(authenticatedAuthController),
        discoverHintStorageProvider.overrideWithValue(testDiscoverHintStorage),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        httpClientProvider.overrideWithValue(client),
        realtimeLinkStatusProvider.overrideWith(
          (ref) => RealtimeLinkStatus.disconnected,
        ),
        realtimeEventProvider.overrideWith((ref) => const Stream.empty()),
        realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
      ],
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(body: home),
      ),
    );
  }

  testWidgets('SocialPanel search tab opens QR add-friend sheet', (
    tester,
  ) async {
    await tester.pumpWidget(
      socialTestApp(
        client: MockClient((request) async {
          if (request.url.path == '/api/v1/users/profiles/prof-test') {
            return http.Response(
              jsonEncode({
                'profile': {
                  'id': 'prof-test',
                  'account_id': 'a-self',
                  'username': 'alice',
                  'display_name': 'Alice',
                  'discriminator': '0001',
                  'locale': 'en',
                  'theme': 'dark',
                  'is_primary': true,
                  'verification_type': 'none',
                },
              }),
              200,
              headers: {'content-type': 'application/json'},
            );
          }
          if (request.url.path.contains('/profiles/prof-test/presence')) {
            return http.Response('{}', 200);
          }
          return http.Response('{}', 200);
        }),
        home: const SocialPanel(),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(SocialPanel.qrAddFriendKey));
    await tester.pumpAndSettle();

    expect(find.byKey(QrAddFriendSheet.sheetKey), findsOneWidget);
    expect(find.byKey(QrAddFriendSheet.myQrKey), findsOneWidget);
  });

  testWidgets('QR scan opens profile sheet for valid profile link', (
    tester,
  ) async {
    await tester.pumpWidget(
      socialTestApp(
        client: MockClient((request) async {
          if (request.url.path == '/api/v1/users/search') {
            return http.Response(
              jsonEncode({
                'profile_list': {
                  'profiles': [
                    {
                      'id': 'profile-bob',
                      'account_id': 'a-bob',
                      'username': 'bob',
                      'display_name': 'Bob',
                      'discriminator': '0002',
                      'locale': 'en',
                      'theme': 'dark',
                      'is_primary': true,
                      'verification_type': 'none',
                    },
                  ],
                },
                'page': {'has_more': false},
              }),
              200,
              headers: {'content-type': 'application/json'},
            );
          }
          if (request.url.path == '/api/v1/users/profiles/profile-bob') {
            return http.Response(
              jsonEncode({
                'profile': {
                  'id': 'profile-bob',
                  'account_id': 'a-bob',
                  'username': 'bob',
                  'display_name': 'Bob',
                  'discriminator': '0002',
                  'locale': 'en',
                  'theme': 'dark',
                  'is_primary': true,
                  'verification_type': 'none',
                },
              }),
              200,
              headers: {'content-type': 'application/json'},
            );
          }
          if (request.url.path.contains('/profiles/profile-bob/presence')) {
            return http.Response('{}', 200);
          }
          return http.Response('{}', 200);
        }),
        home: Builder(
          builder: (context) => Center(
            child: FilledButton(
              onPressed: () => QrAddFriendSheet.show(context),
              child: const Text('open'),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Scan'));
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(QrAddFriendSheet.scanFieldKey),
      'https://voice.gg/u/bob',
    );
    await tester.tap(find.byKey(QrAddFriendSheet.scanSubmitKey));
    await tester.pumpAndSettle();

    expect(find.byType(ProfileDetailSheet), findsOneWidget);
  });
}

class _NoopRealtimeHub extends RealtimeHub {
  _NoopRealtimeHub(super.ref);

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}
}
