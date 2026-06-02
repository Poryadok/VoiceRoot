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
import 'package:voice_frontend/ui/social/social_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

const _backendUnavailableSnippet = 'Start the full API stack';

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

  testWidgets('SocialPanel shows search, friends, and requests tabs', (
    tester,
  ) async {
    await tester.pumpWidget(
      socialTestApp(
        client: MockClient((_) async => http.Response('{}', 200)),
        home: const SocialPanel(),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(SocialPanel.panelKey), findsOneWidget);
    expect(find.byKey(SocialPanel.tabSearchKey), findsOneWidget);
    expect(find.byKey(SocialPanel.tabFriendsKey), findsOneWidget);
    expect(find.byKey(SocialPanel.tabRequestsKey), findsOneWidget);
  });

  testWidgets('search submits query and shows profile result', (tester) async {
    await tester.pumpWidget(
      socialTestApp(
        home: const SocialPanel(),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/users/search') {
            return http.Response(
              jsonEncode({
                'profile_list': {
                  'profiles': [
                    {
                      'id': 'p-search',
                      'account_id': 'a-1',
                      'username': 'carol',
                      'discriminator': '0001',
                      'display_name': 'Carol',
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
            );
          }
          return http.Response('not found', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(find.byKey(SocialPanel.searchFieldKey), 'carol');
    await tester.tap(find.byKey(SocialPanel.searchSubmitKey));
    await tester.pumpAndSettle();

    expect(find.text('Carol'), findsOneWidget);
    expect(find.textContaining('@carol'), findsOneWidget);
  });

  testWidgets('profile detail shows online indicator from presence', (
    tester,
  ) async {
    final client = MockClient((req) async {
      if (req.url.path == '/api/v1/users/profiles/p-1') {
        return http.Response(
          jsonEncode({
            'profile': {
              'id': 'p-1',
              'account_id': 'a-1',
              'username': 'dana',
              'discriminator': '0001',
              'display_name': 'Dana',
              'locale': 'en',
              'theme': 'dark',
              'is_primary': true,
              'verification_type': 'none',
            },
          }),
          200,
        );
      }
      if (req.url.path == '/api/v1/users/profiles/p-1/presence') {
        return http.Response(
          jsonEncode({
            'presenceStatus': {'profileId': 'p-1', 'status': 'online'},
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
      return http.Response('{}', 200);
    });

    await tester.pumpWidget(
      socialTestApp(
        client: client,
        home: const ProfileDetailSheet(profileId: 'p-1'),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(ProfileDetailSheet.sheetKey), findsOneWidget);
    expect(find.text('Dana'), findsOneWidget);
    expect(find.byKey(ProfileDetailSheet.onlineIndicatorKey), findsOneWidget);
  });

  testWidgets('profile detail shows last seen for offline presence', (
    tester,
  ) async {
    final client = MockClient((req) async {
      if (req.url.path == '/api/v1/users/profiles/p-2') {
        return http.Response(
          jsonEncode({
            'profile': {
              'id': 'p-2',
              'account_id': 'a-2',
              'username': 'erin',
              'discriminator': '0002',
              'display_name': 'Erin',
              'locale': 'en',
              'theme': 'dark',
              'is_primary': true,
              'verification_type': 'none',
            },
          }),
          200,
        );
      }
      if (req.url.path == '/api/v1/users/profiles/p-2/presence') {
        return http.Response(
          jsonEncode({
            'presenceStatus': {
              'profileId': 'p-2',
              'status': 'invisible',
              'lastSeen': '2026-06-02T18:30:00Z',
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
      return http.Response('{}', 200);
    });

    await tester.pumpWidget(
      socialTestApp(
        client: client,
        home: const ProfileDetailSheet(profileId: 'p-2'),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Erin'), findsOneWidget);
    expect(find.textContaining('Last seen'), findsOneWidget);
  });

  testWidgets('friends tab shows backend unavailable on 404', (tester) async {
    await tester.pumpWidget(
      socialTestApp(
        home: const SocialPanel(initialTabIndex: 1),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/friends') {
            return http.Response('not found', 404);
          }
          return http.Response('{}', 200);
        }),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(SocialPanel.friendsUnavailableKey), findsOneWidget);
    expect(find.textContaining(_backendUnavailableSnippet), findsOneWidget);
    expect(find.text('No friends yet'), findsNothing);
  });

  testWidgets('search shows backend unavailable on 404', (tester) async {
    await tester.pumpWidget(
      socialTestApp(
        home: const SocialPanel(),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/users/search') {
            return http.Response('not found', 404);
          }
          return http.Response('{}', 200);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(find.byKey(SocialPanel.searchFieldKey), 'alice');
    await tester.tap(find.byKey(SocialPanel.searchSubmitKey));
    await tester.pumpAndSettle();

    expect(find.byKey(SocialPanel.searchUnavailableKey), findsOneWidget);
    expect(find.textContaining(_backendUnavailableSnippet), findsOneWidget);
  });

  testWidgets('incoming request accept calls gateway', (tester) async {
    var accepted = false;
    await tester.pumpWidget(
      socialTestApp(
        home: const SocialPanel(initialTabIndex: 2),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/friends/requests') {
            return http.Response(
              jsonEncode({
                'friend_request_list': {
                  'incoming': [
                    {'profile_id': 'p-in'},
                  ],
                  'outgoing': [],
                },
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/users/profiles/p-in') {
            return http.Response(
              jsonEncode({
                'profile': {
                  'id': 'p-in',
                  'account_id': 'a-in',
                  'username': 'incoming',
                  'discriminator': '0001',
                  'display_name': 'Incoming User',
                  'locale': 'en',
                  'theme': 'dark',
                  'is_primary': true,
                  'verification_type': 'none',
                },
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/friends/invitations/p-in/accept') {
            accepted = true;
            return http.Response('{}', 200);
          }
          return http.Response('{}', 200);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(SocialPanel.requestAcceptKey('p-in')));
    await tester.pumpAndSettle();

    expect(accepted, isTrue);
  });
}

class _NoopRealtimeHub extends RealtimeHub {
  _NoopRealtimeHub(super.ref);

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}
}
