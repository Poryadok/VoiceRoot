import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/ui/profile/manage_profiles_sheet.dart';

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
  testWidgets('manage profiles deletes secondary profile via REST', (
    tester,
  ) async {
    var deleteCalled = false;
    final mock = MockClient((req) async {
      if (req.method == 'DELETE' &&
          req.url.path == '/api/v1/users/profiles/profile-secondary') {
        deleteCalled = true;
        return http.Response('', 200);
      }
      if (req.url.path == '/api/v1/users/profiles') {
        return http.Response(
          jsonEncode({
            'profile_list': {
              'profiles': [
                {
                  'id': 'profile-primary',
                  'account_id': 'account-1',
                  'username': 'main',
                  'discriminator': '0001',
                  'display_name': 'Main',
                  'is_primary': true,
                },
                {
                  'id': 'profile-secondary',
                  'account_id': 'account-1',
                  'username': 'alt',
                  'discriminator': '0002',
                  'display_name': 'Alt',
                  'is_primary': false,
                },
              ],
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
      authorizationProvider: () => 'Bearer token',
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
                accessToken: 'token',
                refreshToken: 'refresh',
                expiresInSeconds: 900,
                accountId: 'account-1',
                activeProfileId: 'profile-primary',
              ),
            );
            return controller;
          }),
        ],
        child: MaterialApp(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: ManageProfilesSheet()),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(
      find.byKey(const Key('manage_profiles_delete_profile-secondary')),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('Delete'));
    await tester.pumpAndSettle();

    expect(deleteCalled, isTrue);
  });

  for (final locale in ['en', 'ru']) {
    for (final status in [403, 503]) {
      testWidgets(
        'profile delete $locale HTTP $status is safe and manual retry recovers',
        (tester) async {
          const diagnostic = 'private-profile-delete-diagnostic';
          var deletes = 0;
          var deleted = false;
          final mock = MockClient((request) async {
            if (request.method == 'DELETE') {
              expect(
                request.url.path,
                '/api/v1/users/profiles/profile-secondary',
              );
              expect(request.headers['Authorization'], 'Bearer token');
              deletes++;
              if (deletes == 1) {
                return http.Response(
                  jsonEncode({
                    'error_code': 'private_delete_code',
                    'message': diagnostic,
                  }),
                  status,
                );
              }
              deleted = true;
              return http.Response('', 204);
            }
            if (request.url.path == '/api/v1/users/profiles') {
              return http.Response(
                jsonEncode({
                  'profile_list': {
                    'profiles': [
                      {
                        'id': 'profile-primary',
                        'account_id': 'account-1',
                        'username': 'main',
                        'discriminator': '0001',
                        'display_name': 'Main',
                        'is_primary': true,
                      },
                      if (!deleted)
                        {
                          'id': 'profile-secondary',
                          'account_id': 'account-1',
                          'username': 'alt',
                          'discriminator': '0002',
                          'display_name': 'Alt',
                          'is_primary': false,
                        },
                      {
                        'id': 'profile-active',
                        'account_id': 'account-1',
                        'username': 'active',
                        'discriminator': '0003',
                        'display_name': 'Active',
                        'is_primary': false,
                      },
                    ],
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
                      accessToken: 'token',
                      refreshToken: 'refresh',
                      expiresInSeconds: 900,
                      accountId: 'account-1',
                      activeProfileId: 'profile-active',
                    ),
                  );
                  return controller;
                }),
              ],
              child: MaterialApp(
                locale: Locale(locale),
                localizationsDelegates: AppLocalizations.localizationsDelegates,
                supportedLocales: AppLocalizations.supportedLocales,
                home: const Scaffold(body: ManageProfilesSheet()),
              ),
            ),
          );
          await tester.pumpAndSettle();
          final l10n = AppLocalizations.of(
            tester.element(find.byKey(ManageProfilesSheet.sheetKey)),
          )!;
          expect(
            find.byKey(const Key('manage_profiles_delete_profile-primary')),
            findsNothing,
          );
          await tester.tap(
            find.byKey(const Key('manage_profiles_delete_profile-active')),
          );
          await tester.pumpAndSettle();
          expect(
            find.text(l10n.manageProfilesDeleteActiveBlocked),
            findsOneWidget,
          );
          expect(deletes, 0);

          final deleteButton = find.byKey(
            const Key('manage_profiles_delete_profile-secondary'),
          );
          await tester.tap(deleteButton);
          await tester.pumpAndSettle();
          expect(deletes, 0);
          await tester.tap(
            find.widgetWithText(FilledButton, l10n.commonDelete),
          );
          await tester.pumpAndSettle();
          expect(deletes, 1);
          expect(
            find.text(
              status == 503 ? l10n.backendUnavailable : l10n.commonActionFailed,
            ),
            findsOneWidget,
          );
          expect(find.textContaining(diagnostic), findsNothing);
          expect(find.textContaining('private_delete_code'), findsNothing);
          expect(tester.widget<IconButton>(deleteButton).onPressed, isNotNull);

          await tester.tap(deleteButton);
          await tester.pumpAndSettle();
          await tester.tap(find.widgetWithText(TextButton, l10n.commonCancel));
          await tester.pumpAndSettle();
          expect(deletes, 1);
          await tester.tap(deleteButton);
          await tester.pumpAndSettle();
          await tester.tap(
            find.widgetWithText(FilledButton, l10n.commonDelete),
          );
          await tester.pumpAndSettle();
          expect(deletes, 2);
          expect(find.text('Alt'), findsNothing);
          expect(
            find.text(
              status == 503 ? l10n.backendUnavailable : l10n.commonActionFailed,
            ),
            findsNothing,
          );
        },
      );
    }
  }
}
