import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/sdk_authorization_client.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/l10n/app_localizations_en.dart';
import 'package:voice_frontend/routing/app_router.dart';
import 'package:voice_frontend/state/sdk_authorization_providers.dart';
import 'package:voice_frontend/ui/sdk/sdk_authorization_screens.dart';

void main() {
  final consent = SdkConsentView(
    requestId: 'request',
    applicationId: 'trusted-app',
    environmentId: 'production',
    displayName: 'Example Game',
    scopes: {'game.identity.read', 'game.chat.read'},
    gameSubject: 'player-42',
    policyRevision: 4,
    expiresAt: DateTime.utc(2030),
  );
  const profile = VoiceProfile(
    id: 'voice-profile',
    accountId: 'voice-account',
    username: 'player',
    discriminator: '1234',
    displayName: 'Player',
  );

  testWidgets('requires explicit profile choice before approval', (
    tester,
  ) async {
    var approvals = 0;
    String? chosen;
    await tester.pumpWidget(
      MaterialApp(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: StatefulBuilder(
          builder: (context, setState) => Scaffold(
            body: SdkConsentApprovalPanel(
              l10n: AppLocalizationsEn(),
              consent: consent,
              profiles: const [profile],
              profilesLoading: false,
              profilesFailed: false,
              canApprove: true,
              selectedProfileId: chosen,
              submitting: false,
              onProfileSelected: (id) => setState(() => chosen = id),
              onApprove: () => approvals++,
              onCancel: () {},
            ),
          ),
        ),
      ),
    );

    expect(find.text('Example Game'), findsOneWidget);
    expect(find.text('trusted-app'), findsOneWidget);
    expect(find.text('production'), findsOneWidget);
    expect(find.text('game.chat.read'), findsOneWidget);
    expect(
      tester.widget<FilledButton>(find.byType(FilledButton)).onPressed,
      isNull,
    );
    expect(find.byType(RadioListTile<String>), findsOneWidget);
    await tester.tap(find.byType(RadioListTile<String>));
    await tester.pump();
    expect(chosen, profile.id);
    expect(
      tester.widget<FilledButton>(find.byType(FilledButton)).onPressed,
      isNotNull,
    );
    await tester.tap(find.byType(FilledButton));
    expect(approvals, 1);
  });

  testWidgets('disables profile approval for non-regular sessions', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(
          body: SdkConsentApprovalPanel(
            l10n: AppLocalizationsEn(),
            consent: consent,
            profiles: const [profile],
            profilesLoading: false,
            profilesFailed: false,
            canApprove: false,
            selectedProfileId: profile.id,
            submitting: false,
            onProfileSelected: (_) {},
            onApprove: () {},
            onCancel: () {},
          ),
        ),
      ),
    );
    expect(
      tester.widget<FilledButton>(find.byType(FilledButton)).onPressed,
      isNull,
    );
  });

  testWidgets('router delivers callback URI to independent callback consumer', (
    tester,
  ) async {
    final router = createVoiceGoRouter(
      shellBuilder: (context, state) => const Scaffold(body: Text('home')),
    );
    await tester.pumpWidget(
      ProviderScope(
        child: MaterialApp.router(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          routerConfig: router,
        ),
      ),
    );
    router.go('/sdk/authorization/callback?error=denied');
    await tester.pumpAndSettle();
    expect(
      find.textContaining('authorization link is invalid or expired'),
      findsOneWidget,
    );
    router.dispose();
  });

  testWidgets(
    'retries linked-session resume without exchanging callback code again',
    (tester) async {
      const requestId = '11111111-1111-4111-8111-111111111111';
      const profileId = '22222222-2222-4222-8222-222222222222';
      const appId = '33333333-3333-4333-8333-333333333333';
      const envId = '44444444-4444-4444-8444-444444444444';
      const state = 'ssssssssssssssssssssssssssssssssssssssssssssssss';
      const code = 'ccccccccccccccccccccccccccccccccccccccccccc';
      const redirect = 'https://voice.example/sdk/authorization/callback';
      var exchangeCalls = 0;
      var linkedCalls = 0;
      final handoffs = _MemoryHandoffs(
        SdkAuthorizationHandoff(
          requestId: requestId,
          redirectUri: redirect,
          state: state,
          codeVerifier: List.filled(64, 'v').join(),
          authKeyId: 'auth-issued-kid',
          expiresAt: DateTime.now().toUtc().add(const Duration(minutes: 5)),
        ),
      );
      final sessions = _MemoryLinkedSessions();
      final client = SdkAuthorizationClient(
        gateway: GatewayHttpClient(
          httpClient: MockClient((request) async {
            if (request.url.path.endsWith('/$requestId/exchange')) {
              exchangeCalls++;
              return http.Response(
                jsonEncode({
                  'sourceAccountId': '55555555-5555-4555-8555-555555555555',
                  'accountId': '66666666-6666-4666-8666-666666666666',
                  'profileId': profileId,
                  'deviceId': '77777777-7777-4777-8777-777777777777',
                  'applicationId': appId,
                  'environmentId': envId,
                  'scopes': ['game.identity.read'],
                  'consentRevision': 2,
                  'policyRevision': 7,
                  'accessToken': 'linked-secret',
                  'expiresAt': '2030-10-01T00:05:00Z',
                }),
                200,
              );
            }
            if (request.url.path.endsWith('/linked-session')) {
              linkedCalls++;
              if (linkedCalls == 1) {
                return http.Response('{"error":"temporary"}', 503);
              }
              return http.Response(
                jsonEncode({
                  'sourceAccountId': '55555555-5555-4555-8555-555555555555',
                  'accountId': '66666666-6666-4666-8666-666666666666',
                  'profileId': profileId,
                  'deviceId': '77777777-7777-4777-8777-777777777777',
                  'applicationId': appId,
                  'environmentId': envId,
                  'scopes': ['game.identity.read'],
                  'consentRevision': 2,
                  'policyRevision': 7,
                  'expiresAt': '2030-10-01T00:05:00Z',
                }),
                200,
              );
            }
            return http.Response('{}', 404);
          }),
          config: const GatewayConfig(baseUrl: 'https://voice.example'),
        ),
        handoffStorage: handoffs,
        linkedSessionStorage: sessions,
        proofSigner: (kid, payload) async {
          expect(kid, 'auth-issued-kid');
          return 'signed-proof';
        },
      );
      final callback = Uri.parse('$redirect?code=$code&state=$state');
      await tester.pumpWidget(
        ProviderScope(
          overrides: [sdkAuthorizationClientProvider.overrideWithValue(client)],
          child: MaterialApp(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: SdkAuthorizationCallbackScreen(callback: callback),
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.text('Retry session resume'), findsOneWidget);
      expect(exchangeCalls, 1);
      expect(linkedCalls, 1);
      expect(await sessions.read(), isNotNull);

      await tester.tap(find.text('Retry session resume'));
      await tester.pumpAndSettle();
      expect(find.text('Connected to $appId'), findsOneWidget);
      expect(exchangeCalls, 1);
      expect(linkedCalls, 2);
      expect(await sessions.read(), isNull);
    },
  );

  testWidgets(
    'retries transient device signer failure without exchanging callback code again',
    (tester) async {
      const requestId = '11111111-1111-4111-8111-111111111111';
      const profileId = '22222222-2222-4222-8222-222222222222';
      const appId = '33333333-3333-4333-8333-333333333333';
      const envId = '44444444-4444-4444-8444-444444444444';
      const state = 'ssssssssssssssssssssssssssssssssssssssssssssssss';
      const code = 'ccccccccccccccccccccccccccccccccccccccccccc';
      const redirect = 'https://voice.example/sdk/authorization/callback';
      var exchangeCalls = 0;
      var linkedCalls = 0;
      var failLinkedSigningOnce = true;
      final handoffs = _MemoryHandoffs(
        SdkAuthorizationHandoff(
          requestId: requestId,
          redirectUri: redirect,
          state: state,
          codeVerifier: List.filled(64, 'v').join(),
          authKeyId: 'auth-issued-kid',
          expiresAt: DateTime.now().toUtc().add(const Duration(minutes: 5)),
        ),
      );
      final sessions = _MemoryLinkedSessions();
      final client = SdkAuthorizationClient(
        gateway: GatewayHttpClient(
          httpClient: MockClient((request) async {
            if (request.url.path.endsWith('/$requestId/exchange')) {
              exchangeCalls++;
              return http.Response(
                jsonEncode({
                  'sourceAccountId': '55555555-5555-4555-8555-555555555555',
                  'accountId': '66666666-6666-4666-8666-666666666666',
                  'profileId': profileId,
                  'deviceId': '77777777-7777-4777-8777-777777777777',
                  'applicationId': appId,
                  'environmentId': envId,
                  'scopes': ['game.identity.read'],
                  'consentRevision': 2,
                  'policyRevision': 7,
                  'accessToken': 'linked-secret',
                  'expiresAt': '2030-10-01T00:05:00Z',
                }),
                200,
              );
            }
            if (request.url.path.endsWith('/linked-session')) {
              linkedCalls++;
              return http.Response(
                jsonEncode({
                  'sourceAccountId': '55555555-5555-4555-8555-555555555555',
                  'accountId': '66666666-6666-4666-8666-666666666666',
                  'profileId': profileId,
                  'deviceId': '77777777-7777-4777-8777-777777777777',
                  'applicationId': appId,
                  'environmentId': envId,
                  'scopes': ['game.identity.read'],
                  'consentRevision': 2,
                  'policyRevision': 7,
                  'expiresAt': '2030-10-01T00:05:00Z',
                }),
                200,
              );
            }
            return http.Response('{}', 404);
          }),
          config: const GatewayConfig(baseUrl: 'https://voice.example'),
        ),
        handoffStorage: handoffs,
        linkedSessionStorage: sessions,
        proofSigner: (kid, payload) async {
          expect(kid, 'auth-issued-kid');
          if (payload.startsWith('voice-sdk-linked-v1\n') &&
              failLinkedSigningOnce) {
            failLinkedSigningOnce = false;
            throw const SdkDeviceProofSigningException(isRetryable: true);
          }
          return 'signed-proof';
        },
      );
      final callback = Uri.parse('$redirect?code=$code&state=$state');
      await tester.pumpWidget(
        ProviderScope(
          overrides: [sdkAuthorizationClientProvider.overrideWithValue(client)],
          child: MaterialApp(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: SdkAuthorizationCallbackScreen(callback: callback),
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.text('Retry session resume'), findsOneWidget);
      expect(exchangeCalls, 1);
      expect(linkedCalls, 0);
      expect(await sessions.read(), isNotNull);

      await tester.tap(find.text('Retry session resume'));
      await tester.pumpAndSettle();
      expect(find.text('Connected to $appId'), findsOneWidget);
      expect(exchangeCalls, 1);
      expect(linkedCalls, 1);
      expect(await sessions.read(), isNull);
    },
  );
}

class _MemoryHandoffs implements SdkAuthorizationHandoffStorage {
  _MemoryHandoffs(this.value);
  SdkAuthorizationHandoff? value;
  @override
  Future<void> pruneExpired() async {}
  @override
  Future<SdkAuthorizationHandoff?> readByRequestId(String requestId) async =>
      value?.requestId == requestId ? value : null;
  @override
  Future<SdkAuthorizationHandoff?> readByState(String state) async =>
      value?.state == state ? value : null;
  @override
  Future<void> write(SdkAuthorizationHandoff handoff) async => value = handoff;
  @override
  Future<void> deleteByState(String state) async {
    if (value?.state == state) value = null;
  }
}

class _MemoryLinkedSessions implements SdkLinkedSessionStorage {
  SdkLinkedSessionCredential? value;
  @override
  Future<void> pruneExpired() async {}
  @override
  Future<SdkLinkedSessionCredential?> read() async => value;
  @override
  Future<void> write(SdkLinkedSessionCredential credential) async =>
      value = credential;
  @override
  Future<void> clear() async => value = null;
}
