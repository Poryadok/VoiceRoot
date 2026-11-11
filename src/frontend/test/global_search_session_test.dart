import 'dart:async';
import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/search/global_search_panel.dart';
import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

const _a = AuthSession(accessToken:'test-access', refreshToken:'test-refresh', accountId:'acc-test', activeProfileId:'prof-test', expiresInSeconds:900);
const _b = AuthSession(accessToken:'b-access', refreshToken:'b-refresh', accountId:'other-account', activeProfileId:'other-profile', expiresInSeconds:900);

void main() {
  for (final replacement in ['account', 'aba', 'same-token reinstall', 'controller replacement']) {
    for (final success in [true, false]) {
      test('shared DM action rejects $replacement delayed ${success ? 'success' : 'failure'} without panel guard', () async {
        final started = Completer<void>();
        final pending = Completer<http.Response>();
        final client = MockClient((request) async {
          if (request.url.path == '/api/v1/chats/dm') { started.complete(); return pending.future; }
          return http.Response('{}', 200);
        });
        final container = ProviderContainer(overrides:[
          authSessionStorageProvider.overrideWithValue(InMemoryAuthSessionStorage()),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(const GatewayConfig(baseUrl:'http://api.test')),
          httpClientProvider.overrideWithValue(client),
        ]);
        addTearDown(() { container.dispose(); client.close(); });
        final auth = container.read(authControllerProvider.notifier);
        final result = container.read(chatActionsProvider).openDmWithProfileResult('target');
        await started.future;
        switch (replacement) {
          case 'account': await auth.applySession(_b);
          case 'aba': await auth.applySession(_b); await auth.applySession(_a);
          case 'same-token reinstall': await auth.applySession(_a);
          case 'controller replacement': container.invalidate(authControllerProvider); container.read(authControllerProvider);
        }
        pending.complete(success ? http.Response('{"chat":{"id":"stale-dm","type":"CHAT_TYPE_DM"}}', 200) : http.Response('{"message":"denied"}', 403));
        final actual = await result;
        expect(actual, isA<ChatsApiFailure>());
        expect((actual as ChatsApiFailure).message, kChatActionStaleContext);
        expect(container.read(selectedChatIdProvider), isNull);
      });
    }
  }

  for (final replacement in ['account', 'aba', 'same-token reinstall', 'controller replacement', 'refresh']) {
    for (final success in [true, false]) {
      testWidgets('contact $replacement ${replacement == 'refresh' ? 'preserves' : 'rejects'} delayed ${success ? 'success' : 'failure'}', (tester) async {
        final pending = Completer<http.Response>();
        var dmRequests = 0;
        var closes = 0;
        final client = MockClient((request) async {
          if (request.url.path == '/api/v1/auth/refresh') return http.Response(jsonEncode({'session': {
            'access_token':'refreshed-access','refresh_token':'refreshed-refresh',
            'account_id':'acc-test','profile_id':'prof-test','expires_in_seconds':'900','account_type':'regular',
          }}), 200);
          if (request.url.path == '/api/v1/chats/dm') {
            dmRequests++;
            return pending.future;
          }
          if (request.url.path == '/api/v1/search/global') return http.Response(jsonEncode({'global_search_results': {
            'profile_ids': request.url.queryParameters['q'] == 'target' ? ['target'] : [],
            'messages': request.url.queryParameters['q'] == 'current' ? [{'message_id':'current','snippet':'current session result'}] : [],
          }}), 200);
          if (request.url.path == '/api/v1/users/profiles/target') return http.Response(jsonEncode({'profile': {
            'id':'target','account_id':'target-account','display_name':'Target','username':'target','discriminator':'0001',
          }}), 200);
          return http.Response('{}', 200);
        });
        final container = ProviderContainer(overrides:[
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(testProfileAccentStorage),
          authSessionStorageProvider.overrideWithValue(InMemoryAuthSessionStorage()),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(const GatewayConfig(baseUrl:'http://api.test')),
          httpClientProvider.overrideWithValue(client),
        ]);
        addTearDown(() { container.dispose(); client.close(); });
        await tester.pumpWidget(UncontrolledProviderScope(container:container, child:MaterialApp(
          theme:voiceTestTheme(), locale:const Locale('en'),
          localizationsDelegates:AppLocalizations.localizationsDelegates,
          supportedLocales:AppLocalizations.supportedLocales,
          home:Scaffold(body:GlobalSearchPanel(onClose:() { closes++; })),
        )));
        await tester.pumpAndSettle();
        await tester.enterText(find.byKey(GlobalSearchPanel.searchFieldKey), 'target');
        await tester.pump(const Duration(milliseconds:350));
        await tester.pumpAndSettle();
        await tester.tap(find.text('Target'));
        await tester.pump();
        expect(dmRequests, 1);
        final auth = container.read(authControllerProvider.notifier);
        await tester.runAsync(() async {
          switch (replacement) {
            case 'account': await auth.applySession(_b);
            case 'aba': await auth.applySession(_b); await auth.applySession(_a);
            case 'same-token reinstall': await auth.applySession(_a);
            case 'controller replacement': container.invalidate(authControllerProvider); container.read(authControllerProvider);
            case 'refresh':
              final identity = auth.gatewayRequestIdentity!;
              final installation = auth.sessionInstallGeneration;
              expect(await auth.refreshOn401ForIdentity(identity), isTrue);
              expect(auth.gatewayRequestIdentity, identity);
              expect(auth.sessionInstallGeneration, installation);
          }
        });
        await tester.pumpAndSettle();
        if (replacement != 'refresh') {
          await tester.enterText(find.byKey(GlobalSearchPanel.searchFieldKey), 'current');
          await tester.pump(const Duration(milliseconds:350));
          await tester.pumpAndSettle();
        }
        pending.complete(success
          ? http.Response('{"chat":{"id":"old-dm","type":"CHAT_TYPE_DM"}}', 200)
          : http.Response('{"error_code":"permission_denied","message":"old action denied"}', 403));
        await tester.pumpAndSettle();
        expect(container.read(selectedChatIdProvider), replacement == 'refresh' && success ? 'old-dm' : isNull);
        expect(closes, replacement == 'refresh' && success ? 1 : 0);
        expect(find.byType(SnackBar), replacement == 'refresh' && !success ? findsOneWidget : findsNothing);
        expect(find.text(replacement == 'refresh' ? 'Target' : 'current session result'), findsOneWidget);
        expect(tester.takeException(), isNull);
        await tester.pumpWidget(const SizedBox.shrink());
      });
    }
  }
}
