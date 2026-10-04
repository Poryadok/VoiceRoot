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
                    'presenceStatus': {
                      'profileId': 'p-friend',
                      'status': 'online',
                    },
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
                    'friend_list': {
                      'profile_ids': ['p-friend'],
                    },
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

  testWidgets('profile detail shows unavailable when profile returns 404', (
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
              if (req.url.path == '/api/v1/users/profiles/p-blocked') {
                return http.Response(
                  jsonEncode({
                    'error': 'not_found',
                    'message': 'profile not found',
                  }),
                  404,
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
          home: const Scaffold(
            body: ProfileDetailSheet(profileId: 'p-blocked'),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('User unavailable'), findsOneWidget);
    expect(find.text('Could not load profile'), findsNothing);
  });

  testWidgets('opening a DM hides upstream details and keeps profile open', (
    tester,
  ) async {
    const diagnostic = 'dm-private-diagnostic';
    final dmRequests = <http.Request>[];
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
              if (req.url.path == '/api/v1/users/profiles/p-dm') {
                return http.Response(
                  jsonEncode({
                    'profile': {
                      'id': 'p-dm',
                      'account_id': 'a-dm',
                      'username': 'dm-target',
                      'discriminator': '0001',
                      'display_name': 'DM Target',
                      'locale': 'en',
                      'theme': 'dark',
                      'is_primary': true,
                      'verification_type': 'none',
                    },
                  }),
                  200,
                );
              }
              if (req.url.path == '/api/v1/users/profiles/p-dm/presence') {
                return http.Response(
                  jsonEncode({
                    'presenceStatus': {'profileId': 'p-dm', 'status': 'online'},
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
                    'friend_list': {'profile_ids': <String>[]},
                  }),
                  200,
                );
              }
              if (req.url.path == '/api/v1/chats/dm' && req.method == 'POST') {
                dmRequests.add(req);
                return http.Response(
                  jsonEncode({'error': 'internal', 'message': diagnostic}),
                  500,
                );
              }
              return http.Response('{}', 404);
            }),
          ),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: ProfileDetailSheet(profileId: 'p-dm')),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(ProfileDetailSheet.messageKey));
    await tester.pumpAndSettle();

    expect(dmRequests, hasLength(1));
    expect(dmRequests.single.method, 'POST');
    expect(dmRequests.single.url.path, '/api/v1/chats/dm');
    expect(
      (jsonDecode(dmRequests.single.body)
          as Map<String, dynamic>)['other_profile_id'],
      'p-dm',
    );
    final l10n = AppLocalizations.of(
      tester.element(find.byKey(ProfileDetailSheet.sheetKey)),
    )!;
    expect(find.text(l10n.commonActionFailed), findsOneWidget);
    expect(find.textContaining(diagnostic), findsNothing);
    expect(find.byKey(ProfileDetailSheet.sheetKey), findsOneWidget);
  });

  testWidgets('friend action failure hides upstream details', (tester) async {
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
                    'friend_list': {'profile_ids': <String>[]},
                  }),
                  200,
                );
              }
              if (req.url.path == '/api/v1/friends/invitations' &&
                  req.method == 'POST') {
                return http.Response(
                  jsonEncode({
                    'error': 'internal',
                    'message': 'friend-private-diagnostic',
                  }),
                  500,
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

    await tester.tap(find.byKey(ProfileDetailSheet.addFriendKey));
    await tester.pumpAndSettle();

    expect(find.textContaining('friend-private-diagnostic'), findsNothing);
    expect(find.text('Could not complete this action.'), findsOneWidget);
    expect(find.byKey(ProfileDetailSheet.sheetKey), findsOneWidget);
  });

  testWidgets('block failure hides upstream details and keeps the sheet open', (
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
              if (req.url.path == '/api/v1/users/profiles/p-block') {
                return http.Response(
                  jsonEncode({
                    'profile': {
                      'id': 'p-block',
                      'account_id': 'a-block',
                      'username': 'blocked',
                      'discriminator': '0001',
                      'display_name': 'Block Target',
                      'locale': 'en',
                      'theme': 'dark',
                      'is_primary': true,
                      'verification_type': 'none',
                    },
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
                    'friend_list': {'profile_ids': <String>[]},
                  }),
                  200,
                );
              }
              if (req.url.path == '/api/v1/friends/blocks' &&
                  req.method == 'POST') {
                return http.Response(
                  jsonEncode({
                    'error': 'internal',
                    'message': 'block-private-diagnostic',
                  }),
                  500,
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
          home: const Scaffold(body: ProfileDetailSheet(profileId: 'p-block')),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(ProfileDetailSheet.blockKey));
    await tester.pumpAndSettle();
    await tester.tap(
      find.descendant(
        of: find.byType(AlertDialog),
        matching: find.text('Block user'),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.textContaining('block-private-diagnostic'), findsNothing);
    expect(find.text('Could not complete this action.'), findsOneWidget);
    expect(find.byKey(ProfileDetailSheet.sheetKey), findsOneWidget);
  });
}
