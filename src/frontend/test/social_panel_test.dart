import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/core/voice_bottom_sheet.dart';
import 'package:voice_frontend/ui/core/voice_skeleton.dart';
import 'package:voice_frontend/ui/social/profile_detail_sheet.dart';
import 'package:voice_frontend/ui/social/social_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

const _backendUnavailableSnippet = 'Start the full API stack';

void main() {
  Widget socialTestApp({
    required Widget home,
    required http.Client client,
    AuthController Function(Ref ref)? authControllerFactory,
  }) {
    return ProviderScope(
      overrides: [
        ...voiceThemeTestOverrides(),
        profileAccentStorageProvider.overrideWithValue(
          testProfileAccentStorage,
        ),
        authSessionStorageProvider.overrideWithValue(
          InMemoryAuthSessionStorage(),
        ),
        authControllerProvider.overrideWith(
          authControllerFactory ?? authenticatedAuthController,
        ),
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
    expect(find.byKey(SocialPanel.tabContactsKey), findsOneWidget);
    expect(find.byKey(SocialPanel.tabFavoritesKey), findsOneWidget);
    expect(find.byKey(SocialPanel.tabRequestsKey), findsOneWidget);
    expect(find.byKey(SocialPanel.tabBlockedKey), findsOneWidget);
  });

  testWidgets('search works inside non-scrollable bottom sheet', (
    tester,
  ) async {
    await tester.pumpWidget(
      socialTestApp(
        home: Builder(
          builder: (context) => ElevatedButton(
            onPressed: () => showVoiceBottomSheet<void>(
              context: context,
              scrollable: false,
              child: const SocialPanel(),
            ),
            child: const Text('open'),
          ),
        ),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/users/search') {
            return http.Response(
              jsonEncode({
                'profile_list': {
                  'profiles': [
                    {
                      'id': 'p-sheet',
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

    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    await tester.enterText(find.byKey(SocialPanel.searchFieldKey), 'carol');
    await tester.tap(find.byKey(SocialPanel.searchSubmitKey));
    await tester.pumpAndSettle();

    expect(find.text('Carol'), findsOneWidget);
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

  testWidgets('search shows loading skeleton while request is in flight', (
    tester,
  ) async {
    final completer = Completer<http.Response>();
    addTearDown(() {
      if (!completer.isCompleted) {
        completer.complete(http.Response('{}', 500));
      }
    });

    await tester.pumpWidget(
      socialTestApp(
        home: const SocialPanel(),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/users/search') {
            return completer.future;
          }
          return http.Response('not found', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(find.byKey(SocialPanel.searchFieldKey), 'alice');
    await tester.tap(find.byKey(SocialPanel.searchSubmitKey));
    await tester.pump();

    expect(find.byKey(SocialPanel.searchLoadingKey), findsOneWidget);
    expect(find.byType(VoiceListSkeleton), findsOneWidget);

    completer.complete(
      http.Response(
        jsonEncode({
          'profile_list': {'profiles': []},
          'page': {'has_more': false},
        }),
        200,
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(SocialPanel.searchLoadingKey), findsNothing);
  });

  testWidgets('profile search shows only one loading skeleton', (tester) async {
    final completer = Completer<http.Response>();
    addTearDown(() {
      if (!completer.isCompleted) {
        completer.complete(http.Response('{}', 500));
      }
    });

    await tester.pumpWidget(
      socialTestApp(
        home: const SocialPanel(),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/users/search') {
            return completer.future;
          }
          return http.Response('not found', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(find.byKey(SocialPanel.searchFieldKey), 'alice');
    await tester.tap(find.byKey(SocialPanel.searchSubmitKey));
    await tester.pump();

    expect(find.byType(VoiceListSkeleton), findsOneWidget);
  });

  testWidgets('search shows a no-results state after an empty result', (
    tester,
  ) async {
    await tester.pumpWidget(
      socialTestApp(
        home: const SocialPanel(),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/users/search') {
            return http.Response(
              jsonEncode({
                'profile_list': {'profiles': []},
              }),
              200,
            );
          }
          return http.Response('{}', 200);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(find.byKey(SocialPanel.searchFieldKey), 'nobody');
    await tester.tap(find.byKey(SocialPanel.searchSubmitKey));
    await tester.pumpAndSettle();

    expect(find.text('No profiles found'), findsOneWidget);
  });

  testWidgets('search error state can retry the last query', (tester) async {
    var calls = 0;
    await tester.pumpWidget(
      socialTestApp(
        home: const SocialPanel(),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/users/search') {
            calls++;
            if (calls == 1) {
              return http.Response('temporary failure', 500);
            }
            return http.Response(
              jsonEncode({
                'profile_list': {
                  'profiles': [
                    {
                      'id': 'p-retry',
                      'account_id': 'a-retry',
                      'username': 'retry',
                      'discriminator': '0001',
                      'display_name': 'Retry User',
                      'locale': 'en',
                      'theme': 'dark',
                      'is_primary': true,
                      'verification_type': 'none',
                    },
                  ],
                },
              }),
              200,
            );
          }
          return http.Response('{}', 200);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(find.byKey(SocialPanel.searchFieldKey), 'retry');
    await tester.tap(find.byKey(SocialPanel.searchSubmitKey));
    await tester.pumpAndSettle();

    expect(find.text('Try again'), findsOneWidget);
    expect(find.text('Could not search for people.'), findsOneWidget);
    expect(find.text('temporary failure'), findsNothing);
    await tester.tap(find.text('Try again'));
    await tester.pumpAndSettle();

    expect(find.text('Retry User'), findsOneWidget);
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

  testWidgets('profile Block sends account ID and selected profile ID', (
    tester,
  ) async {
    const profileId = '11111111-1111-4111-8111-111111111111';
    const accountId = '22222222-2222-4222-8222-222222222222';
    Map<String, dynamic>? blockBody;
    var blockCalls = 0;
    await tester.pumpWidget(
      socialTestApp(
        home: const ProfileDetailSheet(profileId: profileId),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/users/profiles/$profileId') {
            return http.Response(
              jsonEncode({
                'profile': {
                  'id': profileId,
                  'account_id': accountId,
                  'username': 'xronos',
                  'discriminator': '0001',
                  'display_name': 'Xronos+1',
                  'locale': 'en',
                  'theme': 'dark',
                  'is_primary': true,
                  'verification_type': 'none',
                },
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/friends/blocks' &&
              req.method == 'POST') {
            blockCalls++;
            blockBody = Map<String, dynamic>.from(jsonDecode(req.body) as Map);
            return http.Response('{}', 500);
          }
          return http.Response('{}', 200);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.ensureVisible(find.byKey(ProfileDetailSheet.blockKey));
    await tester.tap(find.byKey(ProfileDetailSheet.blockKey));
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(FilledButton, 'Block user'));
    await tester.pumpAndSettle();

    expect(blockCalls, 1);
    expect(blockBody?['blocked_account_id'], accountId);
    expect(blockBody?['blocked_profile_id'], profileId);
  });

  testWidgets('friends tab shows empty state when no friends', (tester) async {
    await tester.pumpWidget(
      socialTestApp(
        home: const SocialPanel(initialTabIndex: 1),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/friends') {
            return http.Response(
              jsonEncode({
                'friend_list': {'friends': []},
              }),
              200,
            );
          }
          return http.Response('{}', 200);
        }),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('No friends yet'), findsOneWidget);
    expect(find.byKey(SocialPanel.friendsUnavailableKey), findsNothing);
  });

  testWidgets('requests tab shows empty state when no requests', (
    tester,
  ) async {
    await tester.pumpWidget(
      socialTestApp(
        home: const SocialPanel(initialTabIndex: 4),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/friends/requests') {
            return http.Response(
              jsonEncode({
                'friend_request_list': {'incoming': [], 'outgoing': []},
              }),
              200,
            );
          }
          return http.Response('{}', 200);
        }),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('No friend requests'), findsOneWidget);
    expect(find.byKey(SocialPanel.requestsUnavailableKey), findsNothing);
  });

  testWidgets('friends tab shows backend unavailable on 503', (tester) async {
    await tester.pumpWidget(
      socialTestApp(
        home: const SocialPanel(initialTabIndex: 1),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/friends') {
            return http.Response('unavailable', 503);
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

  testWidgets('search shows backend unavailable on 503', (tester) async {
    await tester.pumpWidget(
      socialTestApp(
        home: const SocialPanel(),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/users/search') {
            return http.Response('unavailable', 503);
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

  testWidgets('contacts tab shows empty state when no contacts', (
    tester,
  ) async {
    await tester.pumpWidget(
      socialTestApp(
        home: const SocialPanel(initialTabIndex: 2),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/friends/contacts') {
            return http.Response(
              jsonEncode({
                'contact_list': {'contacts': []},
              }),
              200,
            );
          }
          return http.Response('{}', 200);
        }),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('No contacts yet'), findsOneWidget);
    expect(find.byKey(SocialPanel.contactsUnavailableKey), findsNothing);
  });

  testWidgets(
    'contacts tab hides post-alpha phone book sync and does not call gateway',
    (tester) async {
      var syncCalled = false;
      await tester.pumpWidget(
        socialTestApp(
          home: const SocialPanel(initialTabIndex: 2),
          client: MockClient((req) async {
            if (req.url.path == '/api/v1/friends/contacts') {
              return http.Response(
                jsonEncode({
                  'contact_list': {'contacts': []},
                }),
                200,
              );
            }
            if (req.url.path == '/api/v1/friends/contacts/sync' &&
                req.method == 'POST') {
              syncCalled = true;
              return http.Response(
                jsonEncode({
                  'matched_profile_ids': ['p-1'],
                }),
                200,
              );
            }
            return http.Response('{}', 200);
          }),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.byKey(SocialPanel.syncPhoneContactsKey), findsNothing);
      expect(syncCalled, isFalse);
    },
  );

  testWidgets('favorites tab shows empty state when no favorites', (
    tester,
  ) async {
    await tester.pumpWidget(
      socialTestApp(
        home: const SocialPanel(initialTabIndex: 3),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/friends/favorites') {
            return http.Response(
              jsonEncode({
                'friend_list': {'friends': []},
              }),
              200,
            );
          }
          return http.Response('{}', 200);
        }),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('No favorite people yet'), findsOneWidget);
    expect(find.byKey(SocialPanel.favoritesUnavailableKey), findsNothing);
  });

  testWidgets('favorite failure hides upstream details and rolls back', (
    tester,
  ) async {
    await tester.pumpWidget(
      socialTestApp(
        home: const SocialPanel(initialTabIndex: 1),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/friends') {
            return http.Response(
              jsonEncode({
                'friend_list': {
                  'friends': [
                    {'profile_id': 'p-favorite'},
                  ],
                },
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/friends/favorites' &&
              req.method == 'GET') {
            return http.Response(
              jsonEncode({
                'friend_list': {'profile_ids': <String>[]},
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/friends/favorites' &&
              req.method == 'POST') {
            return http.Response(
              jsonEncode({
                'error': 'internal',
                'message': 'favorite-private-diagnostic',
              }),
              500,
            );
          }
          return http.Response('{}', 200);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(SocialPanel.favoriteToggleKey('p-favorite')));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.textContaining('favorite-private-diagnostic'), findsNothing);
    expect(find.text('Could not complete this action.'), findsOneWidget);
    expect(find.byIcon(Icons.star_border), findsOneWidget);
  });

  testWidgets('incoming request accept calls gateway', (tester) async {
    var accepted = false;
    await tester.pumpWidget(
      socialTestApp(
        home: const SocialPanel(initialTabIndex: 4),
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

  testWidgets('incoming request actions are disabled during profile switch', (
    tester,
  ) async {
    var acceptCalls = 0;
    await tester.pumpWidget(
      socialTestApp(
        home: const SocialPanel(initialTabIndex: 4),
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
          if (req.url.path == '/api/v1/friends/invitations/p-in/accept') {
            acceptCalls++;
            return http.Response('{}', 200);
          }
          return http.Response('{}', 200);
        }),
      ),
    );
    await tester.pumpAndSettle();

    final pendingSwitchAccept = tester
        .widget<IconButton>(find.byKey(SocialPanel.requestAcceptKey('p-in')))
        .onPressed;
    expect(pendingSwitchAccept, isNotNull);
    final container = ProviderScope.containerOf(
      tester.element(find.byKey(SocialPanel.panelKey)),
    );
    expect(
      container.read(authControllerProvider.notifier).gatewayRequestIdentity,
      isNotNull,
    );
    container.read(profileSwitchInProgressProvider.notifier).state = true;
    await tester.pump();

    final acceptButton = tester.widget<IconButton>(
      find.byKey(SocialPanel.requestAcceptKey('p-in')),
    );
    final declineButton = tester.widget<IconButton>(
      find.byKey(SocialPanel.requestDeclineKey('p-in')),
    );
    expect(acceptButton.onPressed, isNull);
    expect(declineButton.onPressed, isNull);

    // Exercise the callback captured before the switch began as well as the
    // now-disabled controls: the controller identity can still be the old one.
    pendingSwitchAccept!();
    await tester.pump();
    expect(acceptCalls, 0);
  });

  testWidgets('incoming request action stays retryable after a safe failure', (
    tester,
  ) async {
    final firstAccept = Completer<http.Response>();
    var acceptCalls = 0;
    var accepted = false;
    await tester.pumpWidget(
      socialTestApp(
        home: const SocialPanel(initialTabIndex: 4),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/friends/requests') {
            return http.Response(
              jsonEncode({
                'friend_request_list': {
                  'incoming': accepted
                      ? []
                      : [
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
            acceptCalls++;
            if (acceptCalls == 1) return firstAccept.future;
            accepted = true;
            return http.Response('{}', 200);
          }
          return http.Response('{}', 200);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(SocialPanel.requestAcceptKey('p-in')));
    await tester.pump();

    expect(
      tester
          .widget<IconButton>(find.byKey(SocialPanel.requestAcceptKey('p-in')))
          .onPressed,
      isNull,
    );
    expect(
      tester
          .widget<IconButton>(find.byKey(SocialPanel.requestDeclineKey('p-in')))
          .onPressed,
      isNull,
    );
    expect(
      find.byKey(SocialPanel.requestActionProgressKey('p-in')),
      findsOneWidget,
    );
    expect(acceptCalls, 1);

    firstAccept.complete(
      http.Response(jsonEncode({'message': 'private backend diagnostic'}), 503),
    );
    await tester.pumpAndSettle();

    expect(find.text('Could not complete this action.'), findsOneWidget);
    expect(find.textContaining('private backend diagnostic'), findsNothing);
    expect(find.byKey(SocialPanel.requestAcceptKey('p-in')), findsOneWidget);
    expect(find.byKey(SocialPanel.requestDeclineKey('p-in')), findsOneWidget);
    expect(
      find.byKey(SocialPanel.requestActionProgressKey('p-in')),
      findsNothing,
    );

    await tester.tap(find.byKey(SocialPanel.requestAcceptKey('p-in')));
    await tester.pumpAndSettle();

    expect(acceptCalls, 2);
    expect(find.byKey(SocialPanel.requestAcceptKey('p-in')), findsNothing);
    expect(find.text('No friend requests'), findsOneWidget);
  });

  testWidgets('pending request result is ignored after actor changes', (
    tester,
  ) async {
    final firstAccept = Completer<http.Response>();
    AuthController? testAuthController;
    var oldActorAcceptCalls = 0;
    await tester.pumpWidget(
      socialTestApp(
        home: const SocialPanel(initialTabIndex: 4),
        authControllerFactory: (ref) {
          final controller = authenticatedAuthController(ref);
          testAuthController = controller;
          return controller;
        },
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/friends/requests') {
            if (req.headers['authorization'] == 'Bearer account-b-token') {
              return http.Response(
                jsonEncode({
                  'friend_request_list': {'incoming': [], 'outgoing': []},
                }),
                200,
              );
            }
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
            oldActorAcceptCalls++;
            return firstAccept.future;
          }
          return http.Response('{}', 200);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(SocialPanel.requestAcceptKey('p-in')));
    await tester.pump();
    expect(oldActorAcceptCalls, 1);

    testAuthController!.state = const AuthState(
      session: AuthSession(
        accessToken: 'account-b-token',
        refreshToken: 'account-b-refresh',
        accountId: 'account-b',
        activeProfileId: 'profile-b',
        expiresInSeconds: 900,
      ),
    );
    await tester.pumpAndSettle();
    expect(find.byKey(SocialPanel.requestAcceptKey('p-in')), findsNothing);
    expect(find.text('No friend requests'), findsOneWidget);

    firstAccept.complete(http.Response('{}', 503));
    await tester.pumpAndSettle();

    expect(oldActorAcceptCalls, 1);
    expect(find.text('Could not complete this action.'), findsNothing);
    expect(find.text('No friend requests'), findsOneWidget);
  });

  testWidgets('blocked tab shows empty state when no blocks', (tester) async {
    await tester.pumpWidget(
      socialTestApp(
        home: const SocialPanel(initialTabIndex: 5),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/friends/blocks') {
            return http.Response(
              jsonEncode({
                'blocked_list': {'blocked': []},
              }),
              200,
            );
          }
          return http.Response('{}', 200);
        }),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('No blocked accounts'), findsOneWidget);
    expect(find.byKey(SocialPanel.blockedUnavailableKey), findsNothing);
  });

  testWidgets('blocked tab shows identity and unblocks by account ID', (
    tester,
  ) async {
    var unblocked = false;
    await tester.pumpWidget(
      socialTestApp(
        home: const SocialPanel(initialTabIndex: 5),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/friends/blocks') {
            return http.Response(
              jsonEncode({
                'blocked_list': {
                  'blocked': [
                    {
                      'blocked_account_id': 'acc-blocked',
                      'blocked_profile_id':
                          '00000000-0000-4000-8000-000000000002',
                      'display_name': 'Xronos+1',
                      'username': 'xronos',
                      'discriminator': '0001',
                    },
                  ],
                },
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/friends/blocks/acc-blocked' &&
              req.method == 'DELETE') {
            unblocked = true;
            return http.Response('', 204);
          }
          return http.Response('{}', 200);
        }),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Xronos+1'), findsOneWidget);
    expect(find.textContaining('acc-blocked'), findsNothing);

    await tester.tap(find.byKey(SocialPanel.unblockButtonKey('acc-blocked')));
    await tester.pumpAndSettle();

    expect(unblocked, isTrue);
  });

  testWidgets('unblock failure hides upstream details', (tester) async {
    await tester.pumpWidget(
      socialTestApp(
        home: const SocialPanel(initialTabIndex: 5),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/friends/blocks' && req.method == 'GET') {
            return http.Response(
              jsonEncode({
                'blocked_list': {
                  'blocked': [
                    {
                      'blocked_account_id': 'acc-blocked',
                      'blocked_profile_id':
                          '00000000-0000-4000-8000-000000000002',
                      'display_name': 'Blocked User',
                      'username': 'blocked',
                      'discriminator': '0001',
                    },
                  ],
                },
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/friends/blocks/acc-blocked' &&
              req.method == 'DELETE') {
            return http.Response(
              jsonEncode({
                'error': 'internal',
                'message': 'unblock-private-diagnostic',
              }),
              500,
            );
          }
          return http.Response('{}', 200);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(SocialPanel.unblockButtonKey('acc-blocked')));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.textContaining('unblock-private-diagnostic'), findsNothing);
    expect(find.text('Could not complete this action.'), findsOneWidget);
  });

  testWidgets('blocked tab hides account ID when identity is unavailable', (
    tester,
  ) async {
    await tester.pumpWidget(
      socialTestApp(
        home: const SocialPanel(initialTabIndex: 5),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/friends/blocks') {
            return http.Response(
              jsonEncode({
                'blocked_list': {
                  'blocked': [
                    {'blocked_account_id': 'acc-blocked'},
                  ],
                },
              }),
              200,
            );
          }
          return http.Response('{}', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('User unavailable'), findsOneWidget);
    expect(find.textContaining('acc-blocked'), findsNothing);
    expect(
      find.byKey(SocialPanel.unblockButtonKey('acc-blocked')),
      findsOneWidget,
    );
  });

  testWidgets('blocked tab never uses an ID-shaped snapshot as its label', (
    tester,
  ) async {
    const blockedId = '00000000-0000-4000-8000-000000000001';
    const selectedProfileId = '00000000-0000-4000-8000-000000000002';
    await tester.pumpWidget(
      socialTestApp(
        home: const SocialPanel(initialTabIndex: 5),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/friends/blocks') {
            return http.Response(
              jsonEncode({
                'blocked_list': {
                  'blocked': [
                    {
                      'blocked_account_id': blockedId,
                      'blocked_profile_id': selectedProfileId,
                      'display_name': blockedId,
                      'username': 'known',
                      'discriminator': '0001',
                    },
                  ],
                },
              }),
              200,
            );
          }
          return http.Response('{}', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('@known#0001'), findsOneWidget);
    expect(find.textContaining(blockedId), findsNothing);
    expect(find.textContaining(selectedProfileId), findsNothing);
    expect(find.byKey(SocialPanel.unblockButtonKey(blockedId)), findsOneWidget);
  });

  testWidgets('blocked tab uses neutral text when snapshot names are IDs', (
    tester,
  ) async {
    const blockedId = '00000000-0000-4000-8000-000000000001';
    const profileId = '00000000-0000-4000-8000-000000000002';
    await tester.pumpWidget(
      socialTestApp(
        home: const SocialPanel(initialTabIndex: 5),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/friends/blocks') {
            return http.Response(
              jsonEncode({
                'blocked_list': {
                  'blocked': [
                    {
                      'blocked_account_id': blockedId,
                      'blocked_profile_id': profileId,
                      'display_name': profileId,
                      'username': blockedId,
                    },
                  ],
                },
              }),
              200,
            );
          }
          return http.Response('{}', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('User unavailable'), findsOneWidget);
    expect(find.textContaining(blockedId), findsNothing);
    expect(find.textContaining(profileId), findsNothing);
    expect(find.byKey(SocialPanel.unblockButtonKey(blockedId)), findsOneWidget);
  });

  testWidgets('outgoing declined request shows declined label', (tester) async {
    await tester.pumpWidget(
      socialTestApp(
        home: const SocialPanel(initialTabIndex: 4),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/friends/requests') {
            return http.Response(
              jsonEncode({
                'friend_request_list': {
                  'incoming': [],
                  'outgoing': [
                    {'profile_id': 'p-out', 'status': 'declined'},
                  ],
                },
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/users/profiles/p-out') {
            return http.Response(
              jsonEncode({
                'profile': {
                  'id': 'p-out',
                  'account_id': 'a-out',
                  'username': 'outgoing',
                  'discriminator': '0001',
                  'display_name': 'Outgoing User',
                  'locale': 'en',
                  'theme': 'dark',
                  'is_primary': true,
                  'verification_type': 'none',
                },
              }),
              200,
            );
          }
          return http.Response('{}', 200);
        }),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Declined'), findsOneWidget);
    expect(find.text('Request pending'), findsNothing);
  });
}

class _NoopRealtimeHub extends RealtimeHub {
  _NoopRealtimeHub(super.ref);

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}
}
