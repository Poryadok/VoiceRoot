import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  const chatId = 'chat-e2e-attach';
  const imageFileId = 'file-e2e-image';
  const docFileId = 'file-e2e-doc';
  final thumbBytes = Uint8List.fromList([137, 80, 78, 71, 13]);
  final docBytes = Uint8List.fromList(List<int>.generate(32, (i) => i));

  Widget previewApp({
    required http.Client client,
    List<Override> extraOverrides = const [],
  }) {
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
        realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
        selectedChatIdProvider.overrideWith((ref) => chatId),
        e2eDecryptedAttachmentThumbProvider.overrideWith(
          (ref, request) async {
            if (request.fileId == imageFileId) return thumbBytes;
            return null;
          },
        ),
        e2eDecryptedAttachmentBytesProvider.overrideWith(
          (ref, request) async {
            if (request.fileId == docFileId) return docBytes;
            return null;
          },
        ),
        ...extraOverrides,
      ],
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: const Scaffold(body: ChatRoomPanel(chatId: chatId)),
      ),
    );
  }

  testWidgets('E2E image attachment uses decrypted thumb provider', (
    tester,
  ) async {
    await tester.pumpWidget(
      previewApp(
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/messages') {
            return http.Response(
              jsonEncode({
                'message_list': {
                  'messages': [
                    {
                      'id': 'msg-e2e-1',
                      'chat': {'id': chatId},
                      'sender_profile_id': 'peer-b',
                      'content': '',
                      'attachments_json': jsonEncode([
                        {
                          'file_id': imageFileId,
                          'type': 'image',
                          'name': 'secret.png',
                          'e2e_key_wire': 'wire-image',
                        },
                      ]),
                      'created_at': '2024-01-01T00:00:00Z',
                    },
                  ],
                },
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/messages/read') {
            return http.Response('{}', 200);
          }
          return http.Response('{}', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    expect(
      find.byKey(ChatRoomPanel.attachmentPreviewKey(imageFileId)),
      findsOneWidget,
    );
    expect(find.byType(Image), findsOneWidget);
  });

  testWidgets('E2E non-image attachment shows download affordance', (
    tester,
  ) async {
    await tester.pumpWidget(
      previewApp(
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/messages') {
            return http.Response(
              jsonEncode({
                'message_list': {
                  'messages': [
                    {
                      'id': 'msg-e2e-2',
                      'chat': {'id': chatId},
                      'sender_profile_id': 'peer-b',
                      'content': '',
                      'attachments_json': jsonEncode([
                        {
                          'file_id': docFileId,
                          'type': 'document',
                          'name': 'report.pdf',
                          'size_bytes': 2048,
                          'e2e_key_wire': 'wire-doc',
                        },
                      ]),
                      'created_at': '2024-01-01T00:00:01Z',
                    },
                  ],
                },
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/messages/read') {
            return http.Response('{}', 200);
          }
          return http.Response('{}', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('report.pdf'), findsOneWidget);
    expect(
      find.textContaining('Tap to download', findRichText: true),
      findsOneWidget,
    );
    expect(find.byIcon(Icons.lock_outline), findsOneWidget);

    await tester.tap(find.text('report.pdf'));
    await tester.pumpAndSettle();

    expect(find.text('Could not decrypt attachment'), findsNothing);
    expect(find.text('Could not save attachment'), findsOneWidget);
  });
}

class _NoopRealtimeHub extends RealtimeHub {
  _NoopRealtimeHub(super.ref);

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}
}
