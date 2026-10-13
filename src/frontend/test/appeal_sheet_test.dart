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
import 'package:voice_frontend/ui/core/voice_primary_button.dart';
import 'package:voice_frontend/ui/settings/appeal_sheet.dart';

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
  for (final locale in ['en', 'ru']) {
    for (final status in [403, 503]) {
      testWidgets(
        'appeal submit $locale HTTP $status is safe and manual retry recovers',
        (tester) async {
          const diagnostic = 'private-appeal-diagnostic';
          var submissions = 0;
          final mock = MockClient((request) async {
            expect(request.method, 'POST');
            expect(request.url.path, '/api/v1/moderation/appeals');
            expect(request.headers['Authorization'], 'Bearer access-token');
            expect(jsonDecode(request.body), {
              'sanction_id': 'sanction-1',
              'reason': 'mistake',
            });
            submissions++;
            if (submissions == 1) {
              return http.Response(
                jsonEncode({
                  'error_code': 'private_appeal_code',
                  'message': diagnostic,
                }),
                status,
              );
            }
            return http.Response(
              jsonEncode({
                'appeal': {
                  'id': 'appeal-1',
                  'sanction_id': 'sanction-1',
                  'status': 'pending',
                },
              }),
              201,
            );
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
                localizationsDelegates: AppLocalizations.localizationsDelegates,
                supportedLocales: AppLocalizations.supportedLocales,
                home: const Scaffold(
                  body: SingleChildScrollView(child: AppealSheet()),
                ),
              ),
            ),
          );
          await tester.pumpAndSettle();
          final l10n = AppLocalizations.of(
            tester.element(find.byKey(AppealSheet.sheetKey)),
          )!;
          final submit = find.byKey(AppealSheet.submitButtonKey);
          await tester.ensureVisible(submit);
          await tester.tap(submit);
          await tester.pumpAndSettle();
          expect(find.text(l10n.appealValidationError), findsOneWidget);
          expect(submissions, 0);
          await tester.enterText(
            find.byKey(AppealSheet.sanctionFieldKey),
            'sanction-1',
          );
          await tester.enterText(
            find.byKey(AppealSheet.reasonFieldKey),
            'mistake',
          );
          await tester.ensureVisible(submit);
          await tester.tap(submit);
          await tester.pumpAndSettle();
          expect(submissions, 1);
          expect(
            find.text(
              status == 503 ? l10n.backendUnavailable : l10n.commonActionFailed,
            ),
            findsOneWidget,
          );
          expect(find.textContaining(diagnostic), findsNothing);
          expect(find.textContaining('private_appeal_code'), findsNothing);
          expect(
            tester.widget<VoicePrimaryButton>(submit).onPressed,
            isNotNull,
          );
          expect(
            tester
                .widget<TextField>(find.byKey(AppealSheet.sanctionFieldKey))
                .controller!
                .text,
            'sanction-1',
          );
          expect(
            tester
                .widget<TextField>(find.byKey(AppealSheet.reasonFieldKey))
                .controller!
                .text,
            'mistake',
          );
          await tester.ensureVisible(submit);
          await tester.tap(submit);
          await tester.pumpAndSettle();
          expect(submissions, 2);
          expect(find.text(l10n.appealSubmittedMessage), findsOneWidget);
          expect(
            find.text(
              status == 503 ? l10n.backendUnavailable : l10n.commonActionFailed,
            ),
            findsNothing,
          );
          expect(tester.widget<VoicePrimaryButton>(submit).onPressed, isNull);
        },
      );
    }
  }
}
