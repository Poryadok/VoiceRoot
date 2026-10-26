import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
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
import 'support/voice_test_theme.dart';

const _urlLauncher = MethodChannel('plugins.flutter.io/url_launcher');
bool _goldenFontsLoaded = false;

Future<void> _loadGoldenFonts() async {
  // Each test needs a fresh Future in its own FakeAsync zone.
  if (_goldenFontsLoaded) return;
  await (FontLoader(
    'MaterialIcons',
  )..addFont(rootBundle.load('fonts/MaterialIcons-Regular.otf'))).load();
  await (FontLoader('Noto Sans')
        ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
        ..addFont(rootBundle.load('assets/fonts/NotoSans-Medium.ttf'))
        ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'))
        ..addFont(rootBundle.load('assets/fonts/NotoSans-Bold.ttf')))
      .load();
  _goldenFontsLoaded = true;
}

class _MemoryAuthStorage implements AuthSessionStorage {
  @override
  Future<void> clear() async {}

  @override
  Future<AuthSession?> read() async => null;

  @override
  Future<void> write(AuthSession session) async {}
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  for (final layout in [
    (name: 'horizontal', suffix: 'h', size: const Size(1280, 800)),
    (name: 'vertical', suffix: 'v', size: const Size(390, 844)),
  ]) {
    testWidgets('linked accounts ${layout.name} screenshot', (tester) async {
      await _verifyLinkedAccountsGolden(
        tester,
        name: layout.suffix,
        size: layout.size,
      );
    });
  }

  testWidgets('shows loading then the empty state and connect actions', (
    tester,
  ) async {
    final response = Completer<http.Response>();
    await tester.pumpWidget(
      _linkedAccountsTestApp(MockClient((_) => response.future)),
    );
    expect(find.byType(CircularProgressIndicator), findsOneWidget);
    expect(find.byKey(VerificationSettingsSheet.twitchLinkKey), findsNothing);
    expect(find.byKey(VerificationSettingsSheet.youtubeLinkKey), findsNothing);

    response.complete(http.Response(jsonEncode({'linked_accounts': []}), 200));
    await tester.pump();
    await tester.pumpAndSettle();

    final l10n = AppLocalizations.of(
      tester.element(find.byKey(VerificationSettingsSheet.sheetKey)),
    )!;
    expect(find.text(l10n.verificationLinkedAccountsEmpty), findsOneWidget);
    expect(find.byKey(VerificationSettingsSheet.twitchLinkKey), findsOneWidget);
    expect(
      find.byKey(VerificationSettingsSheet.youtubeLinkKey),
      findsOneWidget,
    );
  });

  testWidgets('shows and mutates links only for the selected profile', (
    tester,
  ) async {
    final launchCalls = <MethodCall>[];
    tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
      _urlLauncher,
      (call) async {
        launchCalls.add(call);
        return true;
      },
    );
    addTearDown(
      () => tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
        _urlLauncher,
        null,
      ),
    );
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
    final semantics = tester.ensureSemantics();

    expect(
      find.byKey(VerificationSettingsSheet.selectedProfileKey),
      findsOneWidget,
    );
    expect(
      tester
          .getSemantics(find.byKey(VerificationSettingsSheet.closeKey))
          .getSemanticsData()
          .flagsCollection
          .isButton,
      isTrue,
    );
    expect(find.textContaining('profile-selected'), findsOneWidget);
    expect(find.textContaining('selected-twitch'), findsOneWidget);
    expect(find.text('other-youtube'), findsNothing);
    expect(find.byKey(VerificationSettingsSheet.twitchLinkKey), findsNothing);
    expect(
      find.byKey(VerificationSettingsSheet.youtubeLinkKey),
      findsOneWidget,
    );
    expect(find.bySemanticsLabel('Link YouTube'), findsOneWidget);
    expect(find.bySemanticsLabel('Unlink'), findsOneWidget);

    await tester.tap(find.byKey(VerificationSettingsSheet.youtubeLinkKey));
    await tester.pumpAndSettle();
    expect(youtubeLinkStarted, isTrue);
    expect(launchCalls, hasLength(1));
    expect(launchCalls.single.arguments['url'], 'https://youtube.test/oauth');
    expect(find.textContaining('OAuth:'), findsNothing);

    await tester.tap(find.byKey(const ValueKey('verification_twitch_unlink')));
    await tester.pumpAndSettle();
    expect(twitchUnlinked, isFalse);
    await tester.tap(find.text('Cancel'));
    await tester.pumpAndSettle();
    expect(twitchUnlinked, isFalse);
    await tester.tap(find.byKey(const ValueKey('verification_twitch_unlink')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('verification_unlink_confirm')));
    await tester.pumpAndSettle();
    expect(twitchUnlinked, isTrue);
    expect(find.textContaining('selected-twitch'), findsNothing);
    expect(find.byKey(VerificationSettingsSheet.twitchLinkKey), findsOneWidget);
    semantics.dispose();
  });

  testWidgets('discards a delayed linked-account list after profile change', (
    tester,
  ) async {
    final oldResponse = Completer<http.Response>();
    var listRequests = 0;
    late AuthController authController;
    await tester.pumpWidget(
      _linkedAccountsTestApp(
        MockClient((request) async {
          if (request.method == 'GET' &&
              request.url.path == '/api/v1/auth/linked-accounts') {
            listRequests++;
            if (listRequests == 1) return oldResponse.future;
            return http.Response(
              jsonEncode({
                'linked_accounts': [
                  {
                    'platform': 'youtube',
                    'profile_id': 'profile-next',
                    'external_id': 'next-1',
                    'external_login': 'next-profile-link',
                  },
                ],
              }),
              200,
            );
          }
          return http.Response('{}', 404);
        }),
        onAuthController: (controller) => authController = controller,
      ),
    );
    await tester.pump();

    await authController.applySession(_testSession('profile-next'));
    await tester.pumpAndSettle();
    expect(find.textContaining('profile-next'), findsOneWidget);
    expect(find.textContaining('next-profile-link'), findsOneWidget);

    oldResponse.complete(
      http.Response(
        jsonEncode({
          'linked_accounts': [
            {
              'platform': 'twitch',
              'profile_id': 'profile-selected',
              'external_id': 'old-1',
              'external_login': 'old-profile-link',
            },
          ],
        }),
        200,
      ),
    );
    await tester.pump();
    await tester.pumpAndSettle();
    expect(find.textContaining('profile-next'), findsOneWidget);
    expect(find.textContaining('next-profile-link'), findsOneWidget);
    expect(find.textContaining('old-profile-link'), findsNothing);
    expect(listRequests, 2);
  });

  testWidgets('does not launch a delayed OAuth URL after profile change', (
    tester,
  ) async {
    final linkResponse = Completer<http.Response>();
    final launchCalls = <MethodCall>[];
    tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
      _urlLauncher,
      (call) async {
        launchCalls.add(call);
        return true;
      },
    );
    addTearDown(
      () => tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
        _urlLauncher,
        null,
      ),
    );
    late AuthController authController;
    await tester.pumpWidget(
      _linkedAccountsTestApp(
        MockClient((request) async {
          if (request.method == 'GET' &&
              request.url.path == '/api/v1/auth/linked-accounts') {
            return http.Response(jsonEncode({'linked_accounts': []}), 200);
          }
          if (request.method == 'POST' &&
              request.url.path == '/api/v1/auth/linked-accounts/twitch/link') {
            return linkResponse.future;
          }
          return http.Response('{}', 404);
        }),
        onAuthController: (controller) => authController = controller,
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(VerificationSettingsSheet.twitchLinkKey));
    await tester.pump();
    await authController.applySession(_testSession('profile-next'));
    await tester.pumpAndSettle();
    expect(find.textContaining('profile-next'), findsOneWidget);

    linkResponse.complete(
      http.Response(
        jsonEncode({'authorization_url': 'https://twitch.test/old-profile'}),
        200,
      ),
    );
    await tester.pump();
    await tester.pumpAndSettle();
    expect(launchCalls, isEmpty);
    expect(find.textContaining('old-profile'), findsNothing);
  });

  testWidgets('discards delayed unlink completion after profile change', (
    tester,
  ) async {
    final unlinkResponse = Completer<http.Response>();
    var listRequests = 0;
    late AuthController authController;
    await tester.pumpWidget(
      _linkedAccountsTestApp(
        MockClient((request) async {
          if (request.method == 'GET' &&
              request.url.path == '/api/v1/auth/linked-accounts') {
            listRequests++;
            return http.Response(
              jsonEncode({
                'linked_accounts': listRequests == 1
                    ? [
                        {
                          'platform': 'twitch',
                          'profile_id': 'profile-selected',
                          'external_id': 'old-1',
                          'external_login': 'old-profile-link',
                        },
                      ]
                    : <Map<String, String>>[],
              }),
              200,
            );
          }
          if (request.method == 'POST' &&
              request.url.path ==
                  '/api/v1/auth/linked-accounts/twitch/unlink') {
            return unlinkResponse.future;
          }
          return http.Response('{}', 404);
        }),
        onAuthController: (controller) => authController = controller,
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('verification_twitch_unlink')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(VerificationSettingsSheet.unlinkConfirmKey));
    await tester.pump();

    await authController.applySession(_testSession('profile-next'));
    await tester.pumpAndSettle();
    expect(find.textContaining('profile-next'), findsOneWidget);
    expect(find.textContaining('old-profile-link'), findsNothing);

    unlinkResponse.complete(http.Response('', 204));
    await tester.pump();
    await tester.pumpAndSettle();
    expect(find.textContaining('profile-next'), findsOneWidget);
    expect(find.textContaining('old-profile-link'), findsNothing);
    expect(listRequests, 2);
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
            var linked = operation == 'unlink' || operation == 'load';
            final requests = <String>[];
            final launchCalls = <MethodCall>[];
            tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
              _urlLauncher,
              (call) async {
                launchCalls.add(call);
                return true;
              },
            );
            addTearDown(
              () => tester.binding.defaultBinaryMessenger
                  .setMockMethodCallHandler(_urlLauncher, null),
            );
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
              if (operation == 'unlink') {
                await tester.tap(
                  find.byKey(const Key('verification_unlink_confirm')),
                );
                await tester.pumpAndSettle();
              }
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
              await tester.tap(find.byKey(const Key('verification_retry')));
              await tester.pumpAndSettle();
              expect(attempts, 2);
              expect(find.textContaining('selected-twitch'), findsOneWidget);
              expect(find.textContaining(diagnostic), findsNothing);
            }
            if (operation != 'load' && operation == 'link') {
              await tester.ensureVisible(action);
              await tester.tap(action);
              await tester.pumpAndSettle();
              expect(find.textContaining('OAuth:'), findsNothing);
              expect(launchCalls, hasLength(1));
              expect(
                launchCalls.single.arguments['url'],
                'https://twitch.test/oauth',
              );
              expect(
                find.text(
                  status == 503
                      ? l10n.backendUnavailable
                      : l10n.commonActionFailed,
                ),
                findsNothing,
              );
            } else if (operation == 'unlink') {
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
                await tester.tap(
                  find.byKey(const Key('verification_unlink_confirm')),
                );
                await tester.pumpAndSettle();
                expect(find.textContaining('selected-twitch'), findsNothing);
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
              operation == 'link' ? 2 : 0,
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

Widget _linkedAccountsTestApp(
  http.Client client, {
  AuthSession? session,
  void Function(AuthController controller)? onAuthController,
}) {
  final gateway = GatewayHttpClient(
    httpClient: client,
    config: const GatewayConfig(baseUrl: 'http://api.test'),
  );
  return ProviderScope(
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
          guestCredentialsStorage: ref.watch(guestCredentialsStorageProvider),
        );
        controller.state = AuthState(
          session:
              session ??
              const AuthSession(
                accessToken: 'access-token',
                refreshToken: 'refresh-token',
                expiresInSeconds: 900,
                accountId: 'account-1',
                activeProfileId: 'profile-selected',
              ),
        );
        onAuthController?.call(controller);
        return controller;
      }),
    ],
    child: MaterialApp(
      theme: voiceTestTheme().copyWith(
        textTheme: voiceTestTheme().textTheme.apply(fontFamily: 'Noto Sans'),
        primaryTextTheme: voiceTestTheme().primaryTextTheme.apply(
          fontFamily: 'Noto Sans',
        ),
      ),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: Scaffold(body: VerificationSettingsSheet()),
    ),
  );
}

Future<void> _verifyLinkedAccountsGolden(
  WidgetTester tester, {
  required String name,
  required Size size,
}) async {
  await _loadGoldenFonts();
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1;
  await tester.pumpWidget(
    _linkedAccountsTestApp(
      MockClient((request) async {
        if (request.method == 'GET' &&
            request.url.path == '/api/v1/auth/linked-accounts') {
          return http.Response(
            jsonEncode({
              'linked_accounts': [
                {
                  'platform': 'twitch',
                  'profile_id': 'profile-selected',
                  'external_id': 'tw-1',
                  'external_login': 'selected-streamer',
                },
              ],
            }),
            200,
          );
        }
        return http.Response('{}', 404);
      }),
    ),
  );
  await _pumpUntil(tester, find.textContaining('selected-streamer'));

  expect(tester.takeException(), isNull);
  expect(find.textContaining('selected-streamer'), findsOneWidget);
  final goldenPath = Platform.isLinux && name == 'h'
      ? 'goldens/linux/linked_accounts_h.png'
      : 'goldens/linked_accounts_$name.png';
  await expectLater(
    find.byKey(VerificationSettingsSheet.sheetKey),
    matchesGoldenFile(goldenPath),
  );
}

AuthSession _testSession(String profileId) => AuthSession(
  accessToken: 'access-token-$profileId',
  refreshToken: 'refresh-token-$profileId',
  expiresInSeconds: 900,
  accountId: 'account-1',
  activeProfileId: profileId,
);

Future<void> _pumpUntil(WidgetTester tester, Finder finder) async {
  for (var attempt = 0; attempt < 20; attempt++) {
    if (finder.evaluate().isNotEmpty) return;
    await tester.pump(const Duration(milliseconds: 50));
  }
}
