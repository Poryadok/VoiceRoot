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
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
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
