import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/ui/core/verified_badge.dart';
import 'package:voice_frontend/ui/social/profile_detail_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets(
    'profile detail renders its personal system verification badge and visible status',
    (tester) async {
      await tester.pumpWidget(
        _profileDetailTestApp(
          profileId: 'p-target',
          client: _profilePresentationClient(
            verificationType: 'personal',
            visibleCustomStatus: 'Available for testing',
          ),
        ),
      );
      await tester.pumpAndSettle();

      final context = tester.element(find.byKey(ProfileDetailSheet.sheetKey));
      final l10n = AppLocalizations.of(context)!;
      expect(find.byKey(VerifiedBadge.personalKey), findsOneWidget);
      expect(find.bySemanticsLabel(l10n.verifiedBadgePersonal), findsOneWidget);
      expect(find.text('Available for testing'), findsOneWidget);
      expect(
        find.byKey(const Key('profile_not_in_contacts_warning')),
        findsOneWidget,
      );
      expect(find.text(l10n.profileNotInContactsWarning), findsOneWidget);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets(
    'profile detail uses organization badge and does not expose profile status fallback',
    (tester) async {
      await tester.pumpWidget(
        _profileDetailTestApp(
          profileId: 'p-target',
          client: _profilePresentationClient(
            verificationType: 'organization',
            profileCustomStatus: 'Unfiltered profile status',
          ),
        ),
      );
      await tester.pumpAndSettle();

      final context = tester.element(find.byKey(ProfileDetailSheet.sheetKey));
      final l10n = AppLocalizations.of(context)!;
      expect(find.byKey(VerifiedBadge.organizationKey), findsOneWidget);
      expect(
        find.bySemanticsLabel(l10n.verifiedBadgeOrganization),
        findsOneWidget,
      );
      expect(find.text('Unfiltered profile status'), findsNothing);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets(
    'profile detail does not render a verification badge when absent',
    (tester) async {
      await tester.pumpWidget(
        _profileDetailTestApp(
          profileId: 'p-target',
          client: _profilePresentationClient(verificationType: 'none'),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.byKey(VerifiedBadge.personalKey), findsNothing);
      expect(find.byKey(VerifiedBadge.organizationKey), findsNothing);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('profile detail hides outsider warning for a contact and self', (
    tester,
  ) async {
    await tester.pumpWidget(
      _profileDetailTestApp(
        profileId: 'p-target',
        client: _profilePresentationClient(
          verificationType: 'none',
          contactProfileIds: const ['p-target'],
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(
      find.byKey(const Key('profile_not_in_contacts_warning')),
      findsNothing,
    );

    await tester.pumpWidget(
      _profileDetailTestApp(
        profileId: 'prof-test',
        key: const ValueKey('self-profile-warning-case'),
        client: _profilePresentationClient(
          profileId: 'prof-test',
          verificationType: 'none',
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(
      find.byKey(const Key('profile_not_in_contacts_warning')),
      findsNothing,
    );
    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'profile detail hides outsider warning while contact state is unknown or failing',
    (tester) async {
      final contactList = Completer<http.Response>();
      await tester.pumpWidget(
        _profileDetailTestApp(
          profileId: 'p-target',
          client: _profilePresentationClient(
            verificationType: 'none',
            contactListResponse: contactList.future,
          ),
        ),
      );
      await tester.pump();
      expect(
        find.byKey(const Key('profile_not_in_contacts_warning')),
        findsNothing,
      );

      contactList.complete(
        http.Response(
          jsonEncode({
            'contact_list': {'contacts': <Object?>[]},
          }),
          503,
        ),
      );
      await tester.pumpAndSettle();
      expect(
        find.byKey(const Key('profile_not_in_contacts_warning')),
        findsNothing,
      );
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('profile detail resolves contacts beyond the first page', (
    tester,
  ) async {
    await tester.pumpWidget(
      _profileDetailTestApp(
        profileId: 'p-target',
        client: _profilePresentationClient(
          verificationType: 'none',
          contactPages: const {
            '': {'contacts': <Object?>[], 'next_cursor': 'contacts-page-2'},
            'contacts-page-2': {
              'contacts': [
                {'profile_id': 'p-target', 'source': 'manual'},
              ],
            },
          },
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(
      find.byKey(const Key('profile_not_in_contacts_warning')),
      findsNothing,
    );
    expect(tester.takeException(), isNull);
  });

  testWidgets('profile detail hides outsider warning during profile switch', (
    tester,
  ) async {
    await tester.pumpWidget(
      _profileDetailTestApp(
        profileId: 'p-target',
        isProfileSwitching: true,
        client: _profilePresentationClient(verificationType: 'none'),
      ),
    );
    await tester.pumpAndSettle();
    expect(
      find.byKey(const Key('profile_not_in_contacts_warning')),
      findsNothing,
    );
    expect(tester.takeException(), isNull);
  });

  testWidgets('contact profile hides message when DM permission is denied', (
    tester,
  ) async {
    final requests = <http.Request>[];
    final client = MockClient((request) async {
      requests.add(request);
      if (request.url.path == '/api/v1/users/profiles/p-contact') {
        return http.Response(
          jsonEncode({
            'profile': {
              'id': 'p-contact',
              'account_id': 'a-contact',
              'username': 'contact',
              'discriminator': '0001',
              'display_name': 'Contact',
              'locale': 'en',
              'theme': 'dark',
              'is_primary': true,
              'verification_type': 'none',
            },
          }),
          200,
        );
      }
      if (request.url.path == '/api/v1/chats/dm-permission/p-contact') {
        return http.Response(jsonEncode({'allowed': false}), 200);
      }
      if (request.url.path.endsWith('/presence')) {
        final profileId = request.url.path.split('/').reversed.skip(1).first;
        return http.Response(
          jsonEncode({
            'presenceStatus': {'profileId': profileId, 'status': 'online'},
          }),
          200,
        );
      }
      if (request.url.path == '/api/v1/friends/requests') {
        return http.Response(
          jsonEncode({
            'friend_request_list': {'incoming': [], 'outgoing': []},
          }),
          200,
        );
      }
      if (request.url.path == '/api/v1/friends') {
        return http.Response(
          jsonEncode({
            'friend_list': {'profile_ids': <String>[]},
          }),
          200,
        );
      }
      return http.Response('{}', 200);
    });

    await tester.pumpWidget(
      _profileDetailTestApp(profileId: 'p-contact', client: client),
    );
    await tester.pumpAndSettle();

    expect(
      requests.where(
        (request) =>
            request.url.path == '/api/v1/chats/dm-permission/p-contact',
      ),
      hasLength(1),
    );
    expect(find.byKey(ProfileDetailSheet.messageKey), findsNothing);
    expect(
      requests.where(
        (request) =>
            request.url.path == '/api/v1/chats/dm' && request.method == 'POST',
      ),
      isEmpty,
    );
  });

  testWidgets('contact profile retries permission before allowing a DM', (
    tester,
  ) async {
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(390, 844);
    final captureTheme = await _captureProfileTheme(tester);
    var permissionReads = 0;
    var dmCreates = 0;
    final client = MockClient((request) async {
      if (request.url.path == '/api/v1/users/profiles/p-contact') {
        return http.Response(
          jsonEncode({
            'profile': {
              'id': 'p-contact',
              'account_id': 'a-contact',
              'username': 'contact',
              'discriminator': '0001',
              'display_name': 'Contact',
              'locale': 'en',
              'theme': 'dark',
              'is_primary': true,
              'verification_type': 'none',
            },
          }),
          200,
        );
      }
      if (request.url.path == '/api/v1/chats/dm-permission/p-contact') {
        permissionReads++;
        if (permissionReads == 1) {
          return http.Response(
            jsonEncode({'error': 'unavailable', 'message': 'private-reason'}),
            503,
          );
        }
        if (permissionReads == 2) {
          return http.Response(jsonEncode({'allowed': 'yes'}), 200);
        }
        return http.Response(jsonEncode({'allowed': true}), 200);
      }
      if (request.url.path == '/api/v1/chats/dm' && request.method == 'POST') {
        dmCreates++;
        return http.Response('{}', 200);
      }
      if (request.url.path.endsWith('/presence')) {
        final profileId = request.url.path.split('/').reversed.skip(1).first;
        return http.Response(
          jsonEncode({
            'presenceStatus': {'profileId': profileId, 'status': 'online'},
          }),
          200,
        );
      }
      if (request.url.path == '/api/v1/friends/requests') {
        return http.Response(
          jsonEncode({
            'friend_request_list': {'incoming': [], 'outgoing': []},
          }),
          200,
        );
      }
      if (request.url.path == '/api/v1/friends') {
        return http.Response(
          jsonEncode({
            'friend_list': {'profile_ids': <String>[]},
          }),
          200,
        );
      }
      return http.Response('{}', 200);
    });

    await tester.pumpWidget(
      _profileDetailTestApp(
        profileId: 'p-contact',
        client: client,
        theme: captureTheme,
      ),
    );
    await tester.pumpAndSettle();

    final sheetContext = tester.element(
      find.byKey(ProfileDetailSheet.sheetKey),
    );
    final l10n = AppLocalizations.of(sheetContext)!;
    final semantics = tester.ensureSemantics();
    expect(find.byKey(ProfileDetailSheet.messageKey), findsNothing);
    expect(find.text(l10n.chatListLoadError), findsOneWidget);
    expect(find.text(l10n.commonRetry), findsOneWidget);
    expect(find.bySemanticsLabel(l10n.chatListLoadError), findsOneWidget);
    expect(find.bySemanticsLabel(l10n.commonRetry), findsOneWidget);
    semantics.dispose();
    expect(find.textContaining('private-reason'), findsNothing);
    expect(dmCreates, 0);
    expect(tester.takeException(), isNull);
    await _captureProfileFavorites(
      tester,
      'contact-dm-permission-error-portrait.png',
    );

    tester.view.physicalSize = const Size(844, 390);
    await tester.pumpAndSettle();
    expect(find.text(l10n.chatListLoadError), findsOneWidget);
    expect(find.text(l10n.commonRetry), findsOneWidget);
    expect(tester.takeException(), isNull);
    await _captureProfileFavorites(
      tester,
      'contact-dm-permission-error-landscape.png',
    );
    tester.view.physicalSize = const Size(390, 844);
    await tester.pumpAndSettle();

    await tester.tap(find.text(l10n.commonRetry));
    await tester.pumpAndSettle();

    expect(permissionReads, 2);
    expect(find.byKey(ProfileDetailSheet.messageKey), findsNothing);
    expect(find.text(l10n.chatListLoadError), findsOneWidget);
    await tester.tap(find.text(l10n.commonRetry));
    await tester.pumpAndSettle();

    expect(permissionReads, 3);
    expect(find.byKey(ProfileDetailSheet.messageKey), findsOneWidget);
    expect(find.bySemanticsLabel(l10n.profileMessage), findsOneWidget);
    expect(tester.takeException(), isNull);
    await tester.ensureVisible(find.byKey(ProfileDetailSheet.messageKey));
    await tester.pumpAndSettle();
    await _captureProfileFavorites(tester, 'contact-dm-allowed-portrait.png');
    tester.view.physicalSize = const Size(844, 390);
    await tester.pumpAndSettle();
    expect(find.byKey(ProfileDetailSheet.messageKey), findsOneWidget);
    expect(tester.takeException(), isNull);
    await tester.ensureVisible(find.byKey(ProfileDetailSheet.messageKey));
    await tester.pumpAndSettle();
    await _captureProfileFavorites(tester, 'contact-dm-allowed-landscape.png');
    await tester.tap(find.byKey(ProfileDetailSheet.messageKey));
    await tester.pumpAndSettle();
    expect(dmCreates, 1);
  });

  testWidgets('stale DM permission cannot enable contact after viewer switch', (
    tester,
  ) async {
    final firstPermission = Completer<http.Response>();
    final firstPermissionStarted = Completer<void>();
    var permissionReads = 0;
    var dmCreates = 0;
    AuthController? authController;
    final client = MockClient((request) async {
      if (request.url.path == '/api/v1/users/profiles/p-contact') {
        return http.Response(
          jsonEncode({
            'profile': {
              'id': 'p-contact',
              'account_id': 'a-contact',
              'username': 'contact',
              'discriminator': '0001',
              'display_name': 'Contact',
              'locale': 'en',
              'theme': 'dark',
              'is_primary': true,
              'verification_type': 'none',
            },
          }),
          200,
        );
      }
      if (request.url.path == '/api/v1/chats/dm-permission/p-contact') {
        permissionReads++;
        if (permissionReads == 1) {
          firstPermissionStarted.complete();
          return firstPermission.future;
        }
        return http.Response(jsonEncode({'allowed': false}), 200);
      }
      if (request.url.path == '/api/v1/chats/dm' && request.method == 'POST') {
        dmCreates++;
        return http.Response('{}', 200);
      }
      if (request.url.path.endsWith('/presence')) {
        final profileId = request.url.path.split('/').reversed.skip(1).first;
        return http.Response(
          jsonEncode({
            'presenceStatus': {'profileId': profileId, 'status': 'online'},
          }),
          200,
        );
      }
      if (request.url.path == '/api/v1/friends/requests') {
        return http.Response(
          jsonEncode({
            'friend_request_list': {'incoming': [], 'outgoing': []},
          }),
          200,
        );
      }
      if (request.url.path == '/api/v1/friends') {
        return http.Response(
          jsonEncode({
            'friend_list': {'profile_ids': <String>[]},
          }),
          200,
        );
      }
      return http.Response('{}', 200);
    });

    await tester.pumpWidget(
      _profileDetailTestApp(
        profileId: 'p-contact',
        client: client,
        onAuthController: (controller) => authController = controller,
      ),
    );
    await tester.pump();
    await firstPermissionStarted.future;
    expect(find.byKey(ProfileDetailSheet.messageKey), findsNothing);

    authController!.state = const AuthState(
      session: AuthSession(
        accessToken: 'replacement-access',
        refreshToken: 'replacement-refresh',
        accountId: 'acc-test',
        activeProfileId: 'viewer-b',
        expiresInSeconds: 900,
      ),
    );
    await tester.pumpAndSettle();
    firstPermission.complete(http.Response(jsonEncode({'allowed': true}), 200));
    await tester.pumpAndSettle();

    expect(permissionReads, 2);
    expect(find.byKey(ProfileDetailSheet.messageKey), findsNothing);
    expect(dmCreates, 0);
  });

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
    final permissionRequests = <http.Request>[];
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
              if (req.url.path == '/api/v1/chats/dm-permission/p-dm') {
                permissionRequests.add(req);
                return http.Response(jsonEncode({'allowed': true}), 200);
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

    expect(permissionRequests, hasLength(1));
    expect(permissionRequests.single.method, 'GET');
    expect(
      permissionRequests.single.url.path,
      '/api/v1/chats/dm-permission/p-dm',
    );
    expect(find.byKey(ProfileDetailSheet.messageKey), findsOneWidget);
    await tester.tap(find.byKey(ProfileDetailSheet.messageKey));
    await tester.pumpAndSettle();

    expect(dmRequests, hasLength(1));
    expect(permissionRequests, hasLength(1));
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

  testWidgets('friend profile adds and removes favorites through the API', (
    tester,
  ) async {
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(390, 844);
    final captureTheme = await _captureProfileTheme(tester);
    var isFavorite = false;
    var isFriend = true;
    var selfProfileId = 'p-friend';
    var failFavoritesLoad = false;
    var failNextFavoriteUpdate = false;
    var deferNextFavoriteUpdate = false;
    final favoriteUpdateStarted = Completer<void>();
    final deferredFavoriteResponse = Completer<http.Response>();
    late AuthController authController;
    final favoriteRequests = <Map<String, dynamic>>[];
    final client = MockClient((request) async {
      if (request.url.path.startsWith('/api/v1/users/profiles/') &&
          !request.url.path.endsWith('/presence')) {
        final profileId = request.url.path.split('/').last;
        return http.Response(
          jsonEncode({
            'profile': {
              'id': profileId,
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
      if (request.url.path.endsWith('/presence')) {
        final profileId = request.url.path.split('/').reversed.skip(1).first;
        return http.Response(
          jsonEncode({
            'presenceStatus': {'profileId': profileId, 'status': 'online'},
          }),
          200,
        );
      }
      if (request.url.path == '/api/v1/friends/requests') {
        return http.Response(
          jsonEncode({
            'friend_request_list': {'incoming': [], 'outgoing': []},
          }),
          200,
        );
      }
      if (request.url.path == '/api/v1/friends') {
        return http.Response(
          jsonEncode({
            'friend_list': {
              'profile_ids': isFriend ? [selfProfileId] : <String>[],
            },
          }),
          200,
        );
      }
      if (request.url.path == '/api/v1/chats/dm-permission/p-friend') {
        return http.Response(jsonEncode({'allowed': true}), 200);
      }
      if (request.url.path == '/api/v1/friends/favorites' &&
          request.method == 'GET') {
        if (failFavoritesLoad) {
          return http.Response(
            jsonEncode({
              'error': 'internal',
              'message': 'favorites-load-private-diagnostic',
            }),
            500,
          );
        }
        return http.Response(
          jsonEncode({
            'friend_list': {
              'profile_ids': isFavorite ? ['p-friend'] : <String>[],
            },
          }),
          200,
        );
      }
      if (request.url.path == '/api/v1/friends/favorites' &&
          request.method == 'POST') {
        final body = jsonDecode(request.body) as Map<String, dynamic>;
        favoriteRequests.add(body);
        if (failNextFavoriteUpdate) {
          failNextFavoriteUpdate = false;
          return http.Response(
            jsonEncode({
              'error': 'internal',
              'message': 'favorite-private-diagnostic',
            }),
            500,
          );
        }
        if (deferNextFavoriteUpdate) {
          deferNextFavoriteUpdate = false;
          favoriteUpdateStarted.complete();
          return deferredFavoriteResponse.future;
        }
        isFavorite = body['favorite'] as bool;
        return http.Response('{}', 200);
      }
      return http.Response('{}', 200);
    });

    await tester.pumpWidget(
      _profileDetailTestApp(
        profileId: 'p-friend',
        client: client,
        theme: captureTheme,
        onAuthController: (controller) => authController = controller,
      ),
    );
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
    expect(find.byTooltip('Add to favorites'), findsOneWidget);
    expect(find.bySemanticsLabel('Add to favorites'), findsOneWidget);
    await _captureProfileFavorites(tester, 'profile-favorites-portrait.png');
    tester.view.physicalSize = const Size(844, 390);
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
    expect(find.byTooltip('Add to favorites'), findsOneWidget);
    await tester.ensureVisible(find.byTooltip('Add to favorites'));
    await tester.pumpAndSettle();
    await _captureProfileFavorites(tester, 'profile-favorites-landscape.png');
    final favoriteButton = find.byKey(const Key('profile_favorite_toggle'));
    final favoriteFocus = tester
        .widget<OutlinedButton>(favoriteButton)
        .focusNode!;
    favoriteFocus.requestFocus();
    await tester.pump();
    expect(favoriteFocus.hasPrimaryFocus, isTrue);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.pumpAndSettle();
    await tester.pumpAndSettle();

    expect(favoriteRequests, [
      {'friend_profile_id': 'p-friend', 'favorite': true},
    ]);
    expect(find.byTooltip('Remove from favorites'), findsOneWidget);
    tester.view.physicalSize = const Size(390, 844);
    await tester.pumpAndSettle();

    await tester.tap(find.byTooltip('Remove from favorites'));
    await tester.pumpAndSettle();

    expect(favoriteRequests, [
      {'friend_profile_id': 'p-friend', 'favorite': true},
      {'friend_profile_id': 'p-friend', 'favorite': false},
    ]);
    expect(find.byTooltip('Add to favorites'), findsOneWidget);

    failNextFavoriteUpdate = true;
    await tester.tap(find.byTooltip('Add to favorites'));
    await tester.pumpAndSettle();
    expect(find.byTooltip('Add to favorites'), findsOneWidget);
    expect(find.text('Could not complete this action.'), findsOneWidget);
    expect(find.textContaining('favorite-private-diagnostic'), findsNothing);

    await tester.tap(find.byTooltip('Add to favorites'));
    await tester.pumpAndSettle();
    expect(find.byTooltip('Remove from favorites'), findsOneWidget);

    isFriend = false;
    await tester.pumpWidget(
      _profileDetailTestApp(
        profileId: 'p-friend',
        client: client,
        theme: captureTheme,
        key: const ValueKey('not-friend'),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.byTooltip('Add to favorites'), findsNothing);

    isFriend = true;
    failFavoritesLoad = true;
    await tester.pumpWidget(
      _profileDetailTestApp(
        profileId: 'p-friend',
        client: client,
        theme: captureTheme,
        key: const ValueKey('favorite-load-error'),
        onAuthController: (controller) => authController = controller,
      ),
    );
    await tester.pumpAndSettle();
    expect(find.byTooltip('Add to favorites'), findsNothing);
    expect(find.text('Try again'), findsOneWidget);
    expect(
      find.textContaining('favorites-load-private-diagnostic'),
      findsNothing,
    );
    failFavoritesLoad = false;
    await tester.ensureVisible(find.text('Try again'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Try again'));
    await tester.pumpAndSettle();
    expect(find.byTooltip('Remove from favorites'), findsOneWidget);

    deferNextFavoriteUpdate = true;
    await tester.tap(find.byTooltip('Remove from favorites'));
    await tester.pump();
    await favoriteUpdateStarted.future;
    authController.state = const AuthState(
      session: AuthSession(
        accessToken: 'replacement-access',
        refreshToken: 'replacement-refresh',
        accountId: 'acc-test',
        activeProfileId: 'prof-test',
        expiresInSeconds: 900,
      ),
    );
    await tester.pump();
    deferredFavoriteResponse.complete(
      http.Response(
        jsonEncode({
          'error': 'internal',
          'message': 'stale-session-private-diagnostic',
        }),
        500,
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('Could not complete this action.'), findsNothing);
    expect(
      find.textContaining('stale-session-private-diagnostic'),
      findsNothing,
    );
    expect(find.byTooltip('Remove from favorites'), findsOneWidget);

    selfProfileId = 'prof-test';
    await tester.pumpWidget(
      _profileDetailTestApp(
        profileId: 'prof-test',
        client: client,
        theme: captureTheme,
        key: const ValueKey('self-profile'),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.byTooltip('Add to favorites'), findsNothing);
  });
}

MockClient _profilePresentationClient({
  required String verificationType,
  String profileId = 'p-target',
  String? profileCustomStatus,
  String? visibleCustomStatus,
  List<String> contactProfileIds = const [],
  String? nextContactCursor,
  Map<String, Map<String, dynamic>> contactPages = const {},
  Future<http.Response>? contactListResponse,
}) {
  return MockClient((request) async {
    if (request.url.path == '/api/v1/users/profiles/$profileId') {
      return http.Response(
        jsonEncode({
          'profile': {
            'id': profileId,
            'account_id': 'a-target',
            'username': 'target',
            'discriminator': '0001',
            'display_name': 'Target',
            'locale': 'en',
            'theme': 'dark',
            'is_primary': true,
            'verification_type': verificationType,
            if (profileCustomStatus != null)
              'custom_status': profileCustomStatus,
          },
        }),
        200,
      );
    }
    if (request.url.path == '/api/v1/users/profiles/$profileId/presence') {
      return http.Response(
        jsonEncode({
          'presenceStatus': {
            'profileId': profileId,
            'status': 'online',
            if (visibleCustomStatus != null)
              'customStatus': visibleCustomStatus,
          },
        }),
        200,
      );
    }
    if (request.url.path == '/api/v1/chats/dm-permission/$profileId') {
      return http.Response(jsonEncode({'allowed': false}), 200);
    }
    if (request.url.path == '/api/v1/friends/requests') {
      return http.Response(
        jsonEncode({
          'friend_request_list': {'incoming': [], 'outgoing': []},
        }),
        200,
      );
    }
    if (request.url.path == '/api/v1/friends/contacts') {
      if (contactListResponse != null) return contactListResponse;
      final cursor = request.url.queryParameters['cursor'] ?? '';
      final page =
          contactPages[cursor] ??
          {
            'contacts': [
              for (final contactProfileId in contactProfileIds)
                {'profile_id': contactProfileId, 'source': 'manual'},
            ],
            if (nextContactCursor != null) 'next_cursor': nextContactCursor,
          };
      return http.Response(jsonEncode({'contact_list': page}), 200);
    }
    if (request.url.path == '/api/v1/friends') {
      return http.Response(
        jsonEncode({
          'friend_list': {'profile_ids': <String>[]},
        }),
        200,
      );
    }
    return http.Response('{}', 200);
  });
}

Widget _profileDetailTestApp({
  required String profileId,
  required MockClient client,
  Key? key,
  ThemeData? theme,
  bool isProfileSwitching = false,
  void Function(AuthController controller)? onAuthController,
}) {
  AuthController createAuthController(Ref ref) {
    final controller = authenticatedAuthController(ref);
    onAuthController?.call(controller);
    return controller;
  }

  return ProviderScope(
    key: key,
    overrides: [
      ...voiceThemeTestOverrides(),
      profileAccentStorageProvider.overrideWithValue(testProfileAccentStorage),
      authSessionStorageProvider.overrideWithValue(
        InMemoryAuthSessionStorage(),
      ),
      authControllerProvider.overrideWith(createAuthController),
      profileSwitchInProgressProvider.overrideWith((ref) => isProfileSwitching),
      gatewayConfigProvider.overrideWithValue(
        const GatewayConfig(baseUrl: 'http://api.test'),
      ),
      realtimeAutoConnectProvider.overrideWithValue(false),
      httpClientProvider.overrideWithValue(client),
    ],
    child: MaterialApp(
      theme: theme ?? voiceTestTheme(),
      locale: const Locale('en'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: RepaintBoundary(
        key: const Key('profile_favorites_capture'),
        child: Scaffold(body: ProfileDetailSheet(profileId: profileId)),
      ),
    ),
  );
}

bool _profileFavoritesCaptureFontsLoaded = false;

const _profileFavoritesCaptureEnvironmentKey =
    'VOICE_PROFILE_FAVORITES_CAPTURE_DIR';

Future<ThemeData?> _captureProfileTheme(WidgetTester tester) async {
  final captureDirectory =
      Platform.environment[_profileFavoritesCaptureEnvironmentKey];
  if (captureDirectory == null || captureDirectory.isEmpty) return null;

  if (!_profileFavoritesCaptureFontsLoaded) {
    final loaded = await tester.runAsync(() async {
      final noto = FontLoader(VoiceTheme.fontFamily)
        ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
        ..addFont(rootBundle.load('assets/fonts/NotoSans-Medium.ttf'))
        ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'))
        ..addFont(rootBundle.load('assets/fonts/NotoSans-Bold.ttf'));
      await noto.load();

      final flutterRoot = Platform.environment['FLUTTER_ROOT'];
      if (flutterRoot == null || flutterRoot.isEmpty) {
        throw StateError('FLUTTER_ROOT is required to load Material Icons');
      }
      final iconFont = File(
        [
          flutterRoot,
          'bin',
          'cache',
          'artifacts',
          'material_fonts',
          'MaterialIcons-Regular.otf',
        ].join(Platform.pathSeparator),
      );
      final bytes = await iconFont.readAsBytes();
      final iconData = ByteData.sublistView(Uint8List.fromList(bytes));
      await (FontLoader(
        'MaterialIcons',
      )..addFont(Future<ByteData>.value(iconData))).load();
      return true;
    });
    if (loaded != true) {
      throw StateError('Production font loading did not complete');
    }
    _profileFavoritesCaptureFontsLoaded = true;
  }

  final theme = await tester.runAsync(() async {
    final catalog = await VoiceTokenCatalog.load();
    return VoiceTheme.build(
      catalog: catalog,
      mode: VoiceThemeMode.dark,
      profileAccent: catalog.profileAccentAt(0),
    );
  });
  if (theme == null) {
    throw StateError('Voice design tokens did not load for capture');
  }
  return theme;
}

Future<void> _captureProfileFavorites(
  WidgetTester tester,
  String filename,
) async {
  final captureDirectory =
      Platform.environment[_profileFavoritesCaptureEnvironmentKey];
  if (captureDirectory == null || captureDirectory.isEmpty) return;
  final boundary = tester.renderObject<RenderRepaintBoundary>(
    find.byKey(const Key('profile_favorites_capture')),
  );
  final captured = await tester.runAsync(() async {
    final image = await boundary.toImage(
      pixelRatio: tester.view.devicePixelRatio,
    );
    try {
      final png = await image.toByteData(format: ui.ImageByteFormat.png);
      if (png == null) throw StateError('PNG encoding returned null');
      final output = File(
        '$captureDirectory${Platform.pathSeparator}$filename',
      );
      await output.parent.create(recursive: true);
      await output.writeAsBytes(png.buffer.asUint8List(), flush: true);
      return true;
    } finally {
      image.dispose();
    }
  });
  if (captured != true) {
    throw StateError('Profile Favorites capture did not complete');
  }
}
