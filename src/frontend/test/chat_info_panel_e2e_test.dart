import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/chat/chat_info_panel.dart';
import 'package:voice_frontend/ui/chat/e2e_chat_settings.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

/// Batch E2E-A red test: DM chat info exposes E2E opt-in UI (docs/features/encryption.md).
void main() {
  const chatId = 'dm-e2e-info-1';

  Widget dmInfoTestApp({required http.Client client}) {
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
        chatListControllerProvider.overrideWith(_DmE2eChatListController.new),
      ],
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(
          body: SizedBox(
            height: 500,
            width: 400,
            child: ChatInfoPanel(chatId: chatId, isGroup: false),
          ),
        ),
      ),
    );
  }

  testWidgets('DM chat info shows E2E toggle for direct messages', (tester) async {
    await tester.pumpWidget(
      dmInfoTestApp(
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

    expect(find.byKey(ChatInfoPanel.e2eToggleKey), findsOneWidget);
    expect(find.textContaining('encryption', findRichText: true), findsWidgets);
  });

  testWidgets('tapping E2E toggle opens enable confirmation dialog', (tester) async {
    await tester.pumpWidget(
      dmInfoTestApp(
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

    await tester.tap(find.byKey(ChatInfoPanel.e2eToggleKey));
    await tester.pumpAndSettle();

    expect(find.byKey(E2eEnableConfirmDialog.dialogKey), findsOneWidget);
    expect(
      find.textContaining('Global search', findRichText: true),
      findsOneWidget,
    );
  });
}

class _DmE2eChatListController extends ChatListController {
  _DmE2eChatListController(super.ref) : super() {
    state = ChatListState(
      items: [
        ChatListItem(
          chat: VoiceChat(
            id: 'dm-e2e-info-1',
            type: 'CHAT_TYPE_DM',
            creatorProfileId: 'prof-test',
            e2eEnabled: false,
          ),
          dmPeerProfileId: 'peer-b',
        ),
      ],
    );
  }

  @override
  Future<void> loadInitial() async {}

  @override
  Future<void> loadMore() async {}
}
