import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/social_providers.dart';
import 'package:voice_frontend/ui/settings/verification_settings_sheet.dart';

import 'support/test_voice_token_catalog.dart';

class _MemoryAuthStorage implements AuthSessionStorage {
  @override
  Future<void> clear() async {}

  @override
  Future<AuthSession?> read() async => null;

  @override
  Future<void> write(AuthSession session) async {}
}

void main() {
  for (final switchProfile in [true, false]) {
    testWidgets('linked-account completion preserves status identity after '
        '${switchProfile ? 'profile switch' : 'token refresh'}', (
      tester,
    ) async {
      final linkedResponse = Completer<http.Response>();
      var currentProfileId = 'profile-a';
      var linkedRequests = 0;
      var statusRequests = 0;
      late AuthController authController;
      final mock = MockClient((request) async {
        if (request.method == 'GET' &&
            request.url.path == '/api/v1/auth/linked-accounts') {
          linkedRequests++;
          return linkedResponse.future;
        }
        if (request.method == 'GET' &&
            request.url.path == '/api/v1/users/me/verification') {
          statusRequests++;
          return http.Response(
            jsonEncode({
              'verification_status': {
                'profile_id': currentProfileId,
                'verification_type': 'personal',
                'badge': 'badge-$currentProfileId',
              },
            }),
            200,
          );
        }
        return http.Response('not found', 404);
      });
      final gateway = GatewayHttpClient(
        httpClient: mock,
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      );
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            ...voiceThemeTestOverrides(),
            authSessionStorageProvider.overrideWithValue(_MemoryAuthStorage()),
            guestCredentialsStorageProvider.overrideWithValue(
              InMemoryGuestCredentialsStorage(),
            ),
            gatewayConfigProvider.overrideWithValue(
              const GatewayConfig(baseUrl: 'http://api.test'),
            ),
            gatewayHttpClientProvider.overrideWithValue(gateway),
            voiceAuthClientProvider.overrideWithValue(
              VoiceAuthClient(gateway: gateway),
            ),
            voiceUsersClientProvider.overrideWithValue(
              VoiceUsersClient(gateway: gateway),
            ),
            authControllerProvider.overrideWith((ref) {
              authController = AuthController(
                authClient: ref.watch(voiceAuthClientProvider),
                storage: ref.watch(authSessionStorageProvider),
                guestCredentialsStorage: ref.watch(
                  guestCredentialsStorageProvider,
                ),
              );
              authController.state = const AuthState(
                session: AuthSession(
                  accessToken: 'profile-a-token',
                  refreshToken: 'profile-a-refresh',
                  expiresInSeconds: 900,
                  accountId: 'account-1',
                  activeProfileId: 'profile-a',
                ),
              );
              return authController;
            }),
          ],
          child: const MaterialApp(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: Scaffold(body: VerificationSettingsSheet()),
          ),
        ),
      );
      await tester.pump();
      expect(linkedRequests, 1);
      await tester.tap(
        find.byKey(VerificationSettingsSheet.verificationStatusKey),
      );
      await tester.pumpAndSettle();
      expect(statusRequests, 1);

      currentProfileId = switchProfile ? 'profile-b' : 'profile-a';
      authController.state = authController.state.copyWith(
        session: AuthSession(
          accessToken: 'rotated-token',
          refreshToken: 'rotated-refresh',
          expiresInSeconds: 900,
          accountId: 'account-1',
          activeProfileId: currentProfileId,
        ),
      );
      await tester.pumpAndSettle();
      expect(statusRequests, switchProfile ? 2 : 1);
      expect(find.text('badge-$currentProfileId'), findsOneWidget);

      linkedResponse.complete(
        http.Response(
          jsonEncode({
            'linked_accounts': [
              {
                'platform': 'twitch',
                'profile_id': 'profile-a',
                'external_id': 'twitch-a',
                'external_login': 'profile-a-streamer',
              },
            ],
          }),
          200,
        ),
      );
      await tester.pumpAndSettle();
      final l10n = AppLocalizations.of(
        tester.element(
          find.byKey(VerificationSettingsSheet.verificationStatusViewKey),
        ),
      )!;
      expect(
        find.text(l10n.verificationSelectedProfile(currentProfileId)),
        findsOneWidget,
      );
      expect(find.text('badge-$currentProfileId'), findsOneWidget);
      expect(tester.takeException(), isNull);
    });
  }

  testWidgets(
    'late verification response from the prior profile cannot clear current status',
    (tester) async {
      final firstStatusResponse = Completer<http.Response>();
      var statusRequests = 0;
      final statusAuthorizations = <String?>[];
      late AuthController authController;
      final mock = MockClient((request) async {
        if (request.method == 'GET' &&
            request.url.path == '/api/v1/users/me/verification') {
          statusRequests++;
          statusAuthorizations.add(request.headers['Authorization']);
          if (statusRequests == 1) return firstStatusResponse.future;
          return http.Response(
            jsonEncode({
              'verification_status': {
                'profile_id': 'profile-b',
                'verification_type': 'none',
              },
              'pending_organization_challenge': {
                'domain': 'profile-b.example.org',
                'txt_record': 'voice-verify=profile-b-token',
                'expires_at': DateTime.now()
                    .add(const Duration(days: 1))
                    .toUtc()
                    .toIso8601String(),
              },
            }),
            200,
          );
        }
        if (request.method == 'GET' &&
            request.url.path == '/api/v1/auth/linked-accounts') {
          return http.Response(jsonEncode({'linked_accounts': []}), 200);
        }
        return http.Response('not found', 404);
      });
      final gateway = GatewayHttpClient(
        httpClient: mock,
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            ...voiceThemeTestOverrides(),
            authSessionStorageProvider.overrideWithValue(_MemoryAuthStorage()),
            guestCredentialsStorageProvider.overrideWithValue(
              InMemoryGuestCredentialsStorage(),
            ),
            gatewayConfigProvider.overrideWithValue(
              const GatewayConfig(baseUrl: 'http://api.test'),
            ),
            gatewayHttpClientProvider.overrideWithValue(gateway),
            voiceAuthClientProvider.overrideWithValue(
              VoiceAuthClient(gateway: gateway),
            ),
            voiceUsersClientProvider.overrideWithValue(
              VoiceUsersClient(gateway: gateway),
            ),
            authControllerProvider.overrideWith((ref) {
              authController = AuthController(
                authClient: ref.watch(voiceAuthClientProvider),
                storage: ref.watch(authSessionStorageProvider),
                guestCredentialsStorage: ref.watch(
                  guestCredentialsStorageProvider,
                ),
              );
              authController.state = const AuthState(
                session: AuthSession(
                  accessToken: 'profile-a-token',
                  refreshToken: 'profile-a-refresh',
                  expiresInSeconds: 900,
                  accountId: 'account-a',
                  activeProfileId: 'profile-a',
                ),
              );
              return authController;
            }),
          ],
          child: const MaterialApp(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: Scaffold(body: VerificationSettingsSheet()),
          ),
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(
        find.byKey(VerificationSettingsSheet.verificationStatusKey),
      );
      await tester.pump();
      expect(statusRequests, 1);

      authController.state = const AuthState(
        session: AuthSession(
          accessToken: 'profile-b-token',
          refreshToken: 'profile-b-refresh',
          expiresInSeconds: 900,
          accountId: 'account-b',
          activeProfileId: 'profile-b',
        ),
      );
      await tester.pumpAndSettle();

      expect(statusRequests, 2);
      expect(statusAuthorizations, [
        'Bearer profile-a-token',
        'Bearer profile-b-token',
      ]);
      expect(
        find.text('profile-b.example.org'),
        findsOneWidget,
        reason:
            'auth=$statusAuthorizations; text=${tester.widgetList<Text>(find.byType(Text)).map((widget) => widget.data).join(' | ')}',
      );
      expect(find.text('voice-verify=profile-b-token'), findsOneWidget);

      firstStatusResponse.complete(
        http.Response(jsonEncode({'message': 'stale profile A failure'}), 503),
      );
      await tester.pumpAndSettle();

      expect(find.text('profile-b.example.org'), findsOneWidget);
      expect(find.text('voice-verify=profile-b-token'), findsOneWidget);
      expect(find.textContaining('stale profile A failure'), findsNothing);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('verification status destination loads selected profile status', (
    tester,
  ) async {
    final requests = <http.Request>[];
    var statusAttempts = 0;
    late AuthController authController;
    final mock = MockClient((request) async {
      requests.add(request);
      if (request.method == 'GET' &&
          request.url.path == '/api/v1/users/me/verification') {
        statusAttempts++;
        if (statusAttempts == 1) {
          return http.Response(
            jsonEncode({'message': 'private verification diagnostic'}),
            503,
          );
        }
        return http.Response(
          jsonEncode({
            'verification_status': {
              'profile_id': statusAttempts == 2
                  ? 'profile-selected'
                  : 'profile-next',
              'verification_type': 'none',
            },
            if (statusAttempts == 2)
              'pending_organization_challenge': {
                'domain': 'example.org',
                'txt_record': 'voice-verify=restored-token',
                'expires_at': DateTime.now()
                    .add(const Duration(days: 1))
                    .toUtc()
                    .toIso8601String(),
              },
            if (statusAttempts == 3)
              'pending_organization_challenge': {
                'domain': 'old.example.org',
                'txt_record': 'voice-verify=expired-token',
                'expires_at': DateTime.now()
                    .subtract(const Duration(minutes: 1))
                    .toUtc()
                    .toIso8601String(),
              },
          }),
          200,
        );
      }
      if (request.method == 'GET' &&
          request.url.path == '/api/v1/auth/linked-accounts') {
        return http.Response(jsonEncode({'linked_accounts': []}), 200);
      }
      return http.Response('not found', 404);
    });
    final gateway = GatewayHttpClient(
      httpClient: mock,
      config: const GatewayConfig(baseUrl: 'http://api.test'),
    );
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          authSessionStorageProvider.overrideWithValue(_MemoryAuthStorage()),
          guestCredentialsStorageProvider.overrideWithValue(
            InMemoryGuestCredentialsStorage(),
          ),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          gatewayHttpClientProvider.overrideWithValue(gateway),
          voiceAuthClientProvider.overrideWithValue(
            VoiceAuthClient(gateway: gateway),
          ),
          voiceUsersClientProvider.overrideWithValue(
            VoiceUsersClient(gateway: gateway),
          ),
          authControllerProvider.overrideWith((ref) {
            authController = AuthController(
              authClient: ref.watch(voiceAuthClientProvider),
              storage: ref.watch(authSessionStorageProvider),
              guestCredentialsStorage: ref.watch(
                guestCredentialsStorageProvider,
              ),
            );
            authController.state = const AuthState(
              session: AuthSession(
                accessToken: 'access-token',
                refreshToken: 'refresh-token',
                expiresInSeconds: 900,
                accountId: 'account-1',
                activeProfileId: 'profile-selected',
              ),
            );
            return authController;
          }),
        ],
        child: const MaterialApp(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(body: VerificationSettingsSheet()),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(
      tester
          .widget<OutlinedButton>(
            find.byKey(VerificationSettingsSheet.organizationUnavailableKey),
          )
          .onPressed,
      isNull,
    );

    await tester.tap(
      find.byKey(VerificationSettingsSheet.verificationStatusKey),
    );
    await tester.pumpAndSettle();
    expect(
      find.byKey(VerificationSettingsSheet.verificationStatusRetryKey),
      findsOneWidget,
    );
    expect(
      find.textContaining('private verification diagnostic'),
      findsNothing,
    );
    await tester.tap(
      find.byKey(VerificationSettingsSheet.verificationStatusRetryKey),
    );
    await tester.pumpAndSettle();

    expect(
      requests.where(
        (request) => request.url.path == '/api/v1/users/me/verification',
      ),
      hasLength(2),
    );
    expect(
      requests
          .firstWhere(
            (request) => request.url.path == '/api/v1/users/me/verification',
          )
          .headers['Authorization'],
      'Bearer access-token',
    );
    expect(
      find.byKey(VerificationSettingsSheet.verificationStatusViewKey),
      findsOneWidget,
    );
    expect(find.text('Not verified'), findsOneWidget);
    expect(
      find.byKey(VerificationSettingsSheet.pendingChallengeKey),
      findsOneWidget,
    );
    expect(find.text('example.org'), findsOneWidget);
    expect(find.text('voice-verify=restored-token'), findsOneWidget);
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    for (final size in [const Size(1280, 800), const Size(390, 844)]) {
      tester.view.physicalSize = size;
      tester.view.devicePixelRatio = 1;
      await tester.pumpAndSettle();
      await tester.ensureVisible(
        find.byKey(VerificationSettingsSheet.pendingChallengeTxtRecordKey),
      );
      await tester.pumpAndSettle();
      expect(find.text('voice-verify=restored-token'), findsOneWidget);
      expect(tester.takeException(), isNull);
    }

    authController.state = const AuthState(
      session: AuthSession(
        accessToken: 'next-access-token',
        refreshToken: 'next-refresh-token',
        expiresInSeconds: 900,
        accountId: 'account-1',
        activeProfileId: 'profile-next',
      ),
    );
    await tester.pumpAndSettle();

    expect(find.textContaining('profile-next'), findsOneWidget);
    expect(
      find.byKey(VerificationSettingsSheet.pendingChallengeKey),
      findsNothing,
    );
    expect(find.text('old.example.org'), findsNothing);
    expect(statusAttempts, 3);
  });

  testWidgets('shows and mutates links only for the selected profile', (
    tester,
  ) async {
    var twitchLinked = true;
    var youtubeLinkStarted = false;
    var twitchUnlinked = false;
    final mock = MockClient((request) async {
      if (request.method == 'GET' &&
          request.url.path == '/api/v1/auth/linked-accounts') {
        return http.Response(
          jsonEncode({
            'linked_accounts': [
              if (twitchLinked)
                {
                  'platform': 'twitch',
                  'profile_id': 'profile-selected',
                  'external_id': 'tw-1',
                  'external_login': 'selected-twitch',
                },
              {
                'platform': 'youtube',
                'profile_id': 'profile-other',
                'external_id': 'yt-1',
                'external_login': 'other-youtube',
              },
            ],
          }),
          200,
        );
      }
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/auth/linked-accounts/youtube/link') {
        youtubeLinkStarted = true;
        return http.Response(
          jsonEncode({'authorization_url': 'https://youtube.test/oauth'}),
          200,
        );
      }
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/auth/linked-accounts/twitch/unlink') {
        twitchLinked = false;
        twitchUnlinked = true;
        return http.Response('', 204);
      }
      return http.Response('not found', 404);
    });
    final gateway = GatewayHttpClient(
      httpClient: mock,
      config: const GatewayConfig(baseUrl: 'http://api.test'),
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          authSessionStorageProvider.overrideWithValue(_MemoryAuthStorage()),
          guestCredentialsStorageProvider.overrideWithValue(
            InMemoryGuestCredentialsStorage(),
          ),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          gatewayHttpClientProvider.overrideWithValue(gateway),
          voiceAuthClientProvider.overrideWithValue(
            VoiceAuthClient(gateway: gateway),
          ),
          authControllerProvider.overrideWith((ref) {
            final controller = AuthController(
              authClient: ref.watch(voiceAuthClientProvider),
              storage: ref.watch(authSessionStorageProvider),
              guestCredentialsStorage: ref.watch(
                guestCredentialsStorageProvider,
              ),
            );
            controller.state = const AuthState(
              session: AuthSession(
                accessToken: 'access-token',
                refreshToken: 'refresh-token',
                expiresInSeconds: 900,
                accountId: 'account-1',
                activeProfileId: 'profile-selected',
              ),
            );
            return controller;
          }),
        ],
        child: const MaterialApp(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(body: VerificationSettingsSheet()),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(
      find.byKey(VerificationSettingsSheet.selectedProfileKey),
      findsOneWidget,
    );
    expect(find.textContaining('profile-selected'), findsOneWidget);
    expect(find.text('selected-twitch'), findsOneWidget);
    expect(find.text('other-youtube'), findsNothing);
    expect(find.byKey(VerificationSettingsSheet.twitchLinkKey), findsNothing);
    expect(
      find.byKey(VerificationSettingsSheet.youtubeLinkKey),
      findsOneWidget,
    );

    await tester.tap(find.byKey(VerificationSettingsSheet.youtubeLinkKey));
    await tester.pumpAndSettle();
    expect(youtubeLinkStarted, isTrue);

    await tester.tap(find.byKey(const ValueKey('verification_twitch_unlink')));
    await tester.pumpAndSettle();
    expect(twitchUnlinked, isTrue);
    expect(find.text('selected-twitch'), findsNothing);
    expect(find.byKey(VerificationSettingsSheet.twitchLinkKey), findsOneWidget);
  });

  for (final locale in ['en', 'ru']) {
    testWidgets(
      'linked accounts $locale no-session guard is localized without HTTP',
      (tester) async {
        final gateway = GatewayHttpClient(
          httpClient: MockClient((request) async {
            fail('No session must not make an HTTP request');
          }),
          config: const GatewayConfig(baseUrl: 'http://api.test'),
        );
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              ...voiceThemeTestOverrides(),
              authSessionStorageProvider.overrideWithValue(
                _MemoryAuthStorage(),
              ),
              guestCredentialsStorageProvider.overrideWithValue(
                InMemoryGuestCredentialsStorage(),
              ),
              gatewayHttpClientProvider.overrideWithValue(gateway),
              voiceAuthClientProvider.overrideWithValue(
                VoiceAuthClient(gateway: gateway),
              ),
              authControllerProvider.overrideWith((ref) {
                final controller = AuthController(
                  authClient: ref.watch(voiceAuthClientProvider),
                  storage: ref.watch(authSessionStorageProvider),
                  guestCredentialsStorage: ref.watch(
                    guestCredentialsStorageProvider,
                  ),
                );
                controller.state = const AuthState();
                return controller;
              }),
            ],
            child: MaterialApp(
              locale: Locale(locale),
              localizationsDelegates: AppLocalizations.localizationsDelegates,
              supportedLocales: AppLocalizations.supportedLocales,
              home: const Scaffold(body: VerificationSettingsSheet()),
            ),
          ),
        );
        await tester.pumpAndSettle();
        final l10n = AppLocalizations.of(
          tester.element(find.byKey(VerificationSettingsSheet.sheetKey)),
        )!;
        expect(find.text(l10n.settingsNotAuthenticated), findsOneWidget);
        expect(find.text('not_authenticated'), findsNothing);
        expect(tester.takeException(), isNull);
      },
    );
    for (final status in [403, 503]) {
      for (final operation in ['load', 'link', 'unlink']) {
        testWidgets(
          'linked accounts $operation $locale HTTP $status hides diagnostics and recovers',
          (tester) async {
            const diagnostic = 'private-linked-account-diagnostic';
            var attempts = 0;
            var linked = operation == 'unlink';
            final requests = <String>[];
            final mock = MockClient((request) async {
              requests.add('${request.method} ${request.url.path}');
              expect(request.headers['Authorization'], 'Bearer access-token');
              final isLoad =
                  request.method == 'GET' &&
                  request.url.path == '/api/v1/auth/linked-accounts';
              final isLink =
                  request.method == 'POST' &&
                  request.url.path ==
                      '/api/v1/auth/linked-accounts/twitch/link';
              final isUnlink =
                  request.method == 'POST' &&
                  request.url.path ==
                      '/api/v1/auth/linked-accounts/twitch/unlink';
              if ((operation == 'load' && isLoad) ||
                  (operation == 'link' && isLink) ||
                  (operation == 'unlink' && isUnlink)) {
                attempts++;
                if (attempts == 1) {
                  return http.Response(
                    jsonEncode({
                      'error_code': 'private_link_code',
                      'message': diagnostic,
                    }),
                    status,
                  );
                }
              }
              if (isLoad) {
                return http.Response(
                  jsonEncode({
                    'linked_accounts': [
                      if (linked)
                        {
                          'platform': 'twitch',
                          'profile_id': 'profile-selected',
                          'external_id': 'tw-1',
                          'external_login': 'selected-twitch',
                        },
                      {
                        'platform': 'youtube',
                        'profile_id': 'profile-other',
                        'external_id': 'yt-1',
                        'external_login': 'other-youtube',
                      },
                    ],
                  }),
                  200,
                );
              }
              if (isLink) {
                expect(jsonDecode(request.body), {
                  'redirect_uri': 'https://app.voice.test/oauth/twitch',
                });
                return http.Response(
                  jsonEncode({
                    'authorization_url': 'https://twitch.test/oauth',
                  }),
                  200,
                );
              }
              if (isUnlink) {
                linked = false;
                return http.Response('', 204);
              }
              fail('Unexpected request: ${request.method} ${request.url.path}');
            });
            final gateway = GatewayHttpClient(
              httpClient: mock,
              config: const GatewayConfig(baseUrl: 'http://api.test'),
            );
            await tester.pumpWidget(
              ProviderScope(
                overrides: [
                  ...voiceThemeTestOverrides(),
                  authSessionStorageProvider.overrideWithValue(
                    _MemoryAuthStorage(),
                  ),
                  guestCredentialsStorageProvider.overrideWithValue(
                    InMemoryGuestCredentialsStorage(),
                  ),
                  gatewayConfigProvider.overrideWithValue(
                    const GatewayConfig(baseUrl: 'http://api.test'),
                  ),
                  gatewayHttpClientProvider.overrideWithValue(gateway),
                  voiceAuthClientProvider.overrideWithValue(
                    VoiceAuthClient(gateway: gateway),
                  ),
                  authControllerProvider.overrideWith((ref) {
                    final controller = AuthController(
                      authClient: ref.watch(voiceAuthClientProvider),
                      storage: ref.watch(authSessionStorageProvider),
                      guestCredentialsStorage: ref.watch(
                        guestCredentialsStorageProvider,
                      ),
                    );
                    controller.state = const AuthState(
                      session: AuthSession(
                        accessToken: 'access-token',
                        refreshToken: 'refresh-token',
                        expiresInSeconds: 900,
                        accountId: 'account-1',
                        activeProfileId: 'profile-selected',
                      ),
                    );
                    return controller;
                  }),
                ],
                child: MaterialApp(
                  locale: Locale(locale),
                  localizationsDelegates:
                      AppLocalizations.localizationsDelegates,
                  supportedLocales: AppLocalizations.supportedLocales,
                  home: Scaffold(
                    body: Builder(
                      builder: (context) => TextButton(
                        key: const Key('open_linked_accounts'),
                        onPressed: () => showModalBottomSheet<void>(
                          context: context,
                          isScrollControlled: true,
                          builder: (_) => const VerificationSettingsSheet(),
                        ),
                        child: const Text('Open'),
                      ),
                    ),
                  ),
                ),
              ),
            );
            Future<void> open() async {
              await tester.tap(find.byKey(const Key('open_linked_accounts')));
              await tester.pumpAndSettle();
            }

            await open();
            final l10n = AppLocalizations.of(
              tester.element(find.byKey(VerificationSettingsSheet.sheetKey)),
            )!;
            expect(find.text('other-youtube'), findsNothing);
            final action = operation == 'unlink'
                ? find.byKey(const ValueKey('verification_twitch_unlink'))
                : find.byKey(VerificationSettingsSheet.twitchLinkKey);
            if (operation != 'load') {
              await tester.ensureVisible(action);
              await tester.tap(action);
              await tester.pumpAndSettle();
            }
            expect(attempts, 1);
            expect(
              find.text(
                status == 503
                    ? l10n.backendUnavailable
                    : l10n.commonActionFailed,
              ),
              findsOneWidget,
            );
            expect(find.textContaining(diagnostic), findsNothing);
            expect(find.textContaining('private_link_code'), findsNothing);
            if (operation == 'load') {
              // A successful independent link must not erase a load failure
              // that happens to use the same safe display string.
              await tester.ensureVisible(action);
              await tester.tap(action);
              await tester.pumpAndSettle();
              expect(attempts, 1);
              expect(
                find.text(
                  status == 503
                      ? l10n.backendUnavailable
                      : l10n.commonActionFailed,
                ),
                findsOneWidget,
              );
            }
            if (operation == 'link') {
              await tester.ensureVisible(action);
              await tester.tap(action);
              await tester.pumpAndSettle();
              expect(
                find.text('OAuth: https://twitch.test/oauth'),
                findsOneWidget,
              );
              expect(
                find.text(
                  status == 503
                      ? l10n.backendUnavailable
                      : l10n.commonActionFailed,
                ),
                findsNothing,
              );
            } else {
              // Existing error view has no reload/unlink control. Close and reopen
              // through the supported route before repeating the existing action.
              Navigator.of(
                tester.element(find.byKey(VerificationSettingsSheet.sheetKey)),
              ).pop();
              await tester.pumpAndSettle();
              await open();
              if (operation == 'unlink') {
                await tester.ensureVisible(action);
                await tester.tap(action);
                await tester.pumpAndSettle();
                expect(find.text('selected-twitch'), findsNothing);
              }
              expect(
                find.text(
                  status == 503
                      ? l10n.backendUnavailable
                      : l10n.commonActionFailed,
                ),
                findsNothing,
              );
            }
            expect(attempts, 2);
            expect(find.textContaining(diagnostic), findsNothing);
            expect(find.text('other-youtube'), findsNothing);
            expect(
              requests.where((r) => r.contains('/twitch/link')).length,
              operation == 'link'
                  ? 2
                  : operation == 'load'
                  ? 1
                  : 0,
            );
            expect(
              requests.where((r) => r.contains('/twitch/unlink')).length,
              operation == 'unlink' ? 2 : 0,
            );
          },
        );
      }
    }
  }
}
