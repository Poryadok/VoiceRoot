import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/routing/app_router.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/ui/chat/chat_archive_screen.dart';

import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

class _MemoryAuthStorage implements AuthSessionStorage {
  AuthSession? _session;

  @override
  Future<void> clear() async => _session = null;

  @override
  Future<AuthSession?> read() async => _session;

  @override
  Future<void> write(AuthSession session) async => _session = session;
}

void main() {
  testWidgets('ChatArchiveScreen lists archived chats', (tester) async {
    final storage = _MemoryAuthStorage();
    const session = AuthSession(
      accessToken: 'token',
      refreshToken: 'refresh',
      expiresInSeconds: 900,
      accountId: 'account-1',
      activeProfileId: 'profile-1',
    );
    await storage.write(session);

    final mock = MockClient((req) async {
      if (req.url.path == '/api/v1/chats' &&
          req.url.queryParameters['inbox'] == 'archive') {
        return http.Response(
          jsonEncode({
            'chat_list': {
              'items': [
                {
                  'chat': {
                    'id': 'chat-archived',
                    'type': 'CHAT_TYPE_DM',
                    'creator_profile_id': 'profile-2',
                    'name': 'Archived DM',
                  },
                  'last_message_preview': 'Old note',
                },
              ],
            },
          }),
          200,
        );
      }
      return http.Response('not found', 404);
    });

    final container = ProviderContainer(
      overrides: [
        ...voiceThemeTestOverrides(),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        httpClientProvider.overrideWithValue(mock),
        authSessionStorageProvider.overrideWithValue(storage),
      ],
    );
    addTearDown(container.dispose);
    await container.read(authControllerProvider.notifier).applySession(session);

    final router = createVoiceGoRouter(
      shellBuilder: (context, state) =>
          const Scaffold(body: Text('home shell')),
    );

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp.router(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          routerConfig: router,
        ),
      ),
    );
    router.go(VoiceAppRoutes.chatArchive);
    await tester.pumpAndSettle();

    expect(find.byKey(ChatArchiveScreen.screenKey), findsOneWidget);
    expect(find.text('Archived chats'), findsOneWidget);
    expect(find.byKey(ChatArchiveScreen.tileKey('chat-archived')), findsOneWidget);
    expect(find.text('Old note'), findsOneWidget);
  });
}
