import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/chat/chat_info_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  Widget testApp({required Widget home, required http.Client client}) {
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
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        httpClientProvider.overrideWithValue(client),
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

  testWidgets('chat info panel shows shared media tabs and empty state', (
    tester,
  ) async {
    const chatId = 'chat-shared-media-1';

    await tester.pumpWidget(
      testApp(
        home: SizedBox(
          height: 500,
          width: 400,
          child: ChatInfoPanel(chatId: chatId),
        ),
        client: MockClient((req) async {
          if (req.url.path.contains('/shared-media')) {
            return http.Response(
              jsonEncode({
                'shared_media_list': {
                  'items': [],
                  'next_cursor': '',
                  'has_more': false,
                },
              }),
              200,
            );
          }
          return http.Response('{}', 404);
        }),
      ),
    );

    await tester.pump();
    await tester.pumpAndSettle();

    expect(find.byKey(ChatInfoPanel.panelKey), findsOneWidget);
    expect(find.byKey(ChatInfoPanel.mediaTabKey), findsOneWidget);
    expect(find.byKey(ChatInfoPanel.filesTabKey), findsOneWidget);
    expect(find.byKey(ChatInfoPanel.linksTabKey), findsOneWidget);
    expect(find.byKey(ChatInfoPanel.voiceTabKey), findsOneWidget);
    expect(find.text('Nothing here yet'), findsOneWidget);
  });
}
