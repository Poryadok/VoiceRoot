import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/app.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/auth/auth_screen.dart';
import 'package:voice_frontend/ui/chat/chat_list_panel.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';
import 'package:voice_frontend/ui/social/social_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  group('Phase 1 screens locale ru', () {
    testWidgets('AuthScreen shows Russian labels', (tester) async {
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
          ],
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('ru'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const AuthScreen(),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Вход в Voice'), findsOneWidget);
      expect(find.text('Пароль'), findsOneWidget);
      expect(find.text('Войти'), findsOneWidget);
      expect(find.text('Регистрация'), findsOneWidget);
    });

    testWidgets('authenticated shell shows Russian chat chrome', (tester) async {
      await tester.pumpWidget(
        ProviderScope(
          overrides: voiceAppTestOverrides(
            client: MockClient((req) async {
              if (req.url.path == '/health') {
                return http.Response('OK', 200);
              }
              if (req.url.path == '/api/v1/chats') {
                return http.Response(
                  jsonEncode({'chat_list': {'items': []}}),
                  200,
                );
              }
              return http.Response('{}', 404);
            }),
          ),
          child: const VoiceApp(locale: Locale('ru')),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Личные сообщения'), findsOneWidget);
      expect(find.text('Пока нет диалогов'), findsOneWidget);
      expect(find.text('Выберите диалог'), findsOneWidget);
      expect(find.text('Выйти'), findsOneWidget);
    });

    testWidgets('SocialPanel shows Russian tab labels', (tester) async {
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
            httpClientProvider.overrideWithValue(
              MockClient((_) async => http.Response('{}', 200)),
            ),
            realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
          ],
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('ru'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const Scaffold(body: SocialPanel()),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Поиск'), findsOneWidget);
      expect(find.text('Друзья'), findsOneWidget);
      expect(find.text('Заявки'), findsOneWidget);
    });

    testWidgets('ChatRoomPanel shows Russian empty state and input hint',
        (tester) async {
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
            httpClientProvider.overrideWithValue(
              MockClient((req) async {
                if (req.url.path == '/api/v1/messages') {
                  return http.Response(
                    jsonEncode({'message_list': {'messages': []}}),
                    200,
                  );
                }
                return http.Response('{}', 404);
              }),
            ),
            realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
            realtimeLinkStatusProvider.overrideWith(
              (ref) => RealtimeLinkStatus.disconnected,
            ),
            realtimeEventProvider.overrideWith((ref) => const Stream.empty()),
          ],
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('ru'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const Scaffold(
              body: ChatRoomPanel(chatId: 'chat-abc'),
            ),
          ),
        ),
      );
      await tester.pump();
      await tester.pump(const Duration(seconds: 1));

      expect(find.text('Сообщений пока нет'), findsOneWidget);
      final field = tester.widget<TextField>(find.byKey(ChatRoomPanel.inputKey));
      expect(field.decoration?.hintText, 'Сообщение');
    });
  });

  group('Phase 1 screens locale en', () {
    testWidgets('ChatListPanel shows English chrome', (tester) async {
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
            httpClientProvider.overrideWithValue(
              MockClient((req) async {
                if (req.url.path == '/api/v1/chats') {
                  return http.Response(
                    jsonEncode({'chat_list': {'items': []}}),
                    200,
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
            home: const Scaffold(body: ChatListPanel()),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Direct messages'), findsOneWidget);
      expect(find.text('No conversations yet'), findsOneWidget);
    });
  });
}

class _NoopRealtimeHub extends RealtimeHub {
  _NoopRealtimeHub(super.ref);

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}
}
