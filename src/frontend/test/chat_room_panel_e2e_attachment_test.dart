import 'dart:convert';
import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/e2e/e2e_file_crypto.dart';
import 'package:voice_frontend/e2e/e2e_image_thumb.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/e2e_attachment_test_fixture.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

/// Widget coverage for E2E attachment thumb + download via real decrypt providers.
void main() {
  const chatId = 'chat-e2e-attach';
  const imageFileId = 'file-e2e-image';
  const docFileId = 'file-e2e-doc';
  late Uint8List pngBytes;
  final docBytes = Uint8List.fromList(List<int>.generate(32, (i) => i));

  late E2eAttachmentTestFixture fixture;
  late Uint8List imageCiphertext;
  late String imageKeyWire;
  late Uint8List decryptedImageBytes;
  late Uint8List imageThumbBytes;
  late Uint8List docCiphertext;
  late String docKeyWire;

  setUpAll(() async {
    TestWidgetsFlutterBinding.ensureInitialized();
    pngBytes = await _solidPng(32, 32);
    fixture = await E2eAttachmentTestFixture.create();
    final image = await fixture.encryptFileForPeer(plaintext: pngBytes);
    imageCiphertext = image.ciphertext;
    imageKeyWire = image.keyWire;
    const crypto = E2eFileCrypto();
    decryptedImageBytes = await crypto.decryptBytes(
      ciphertext: imageCiphertext,
      keyWire: imageKeyWire,
      messageService: fixture.messageService,
      localProfileId: fixture.localProfileId,
      peerProfileId: fixture.peerProfileId,
    );
    imageThumbBytes = (await resizeImageBytesForThumb(decryptedImageBytes))!;
    final doc = await fixture.encryptFileForPeer(plaintext: docBytes);
    docCiphertext = doc.ciphertext;
    docKeyWire = doc.keyWire;
  });

  http.Client clientForMessages({
    required List<Map<String, dynamic>> messages,
    required String fileId,
    required Uint8List ciphertext,
  }) {
    return MockClient((req) async {
      if (req.url.path == '/api/v1/messages') {
        return http.Response(
          jsonEncode({
            'message_list': {'messages': messages},
          }),
          200,
        );
      }
      if (req.url.path == '/api/v1/messages/read') {
        return http.Response('{}', 200);
      }
      if (req.url.path == '/api/v1/files/$fileId/url') {
        return http.Response(
          jsonEncode({'presigned_get_url': 'http://files.test/$fileId'}),
          200,
        );
      }
      if (req.url.host == 'files.test') {
        return http.Response.bytes(ciphertext, 200);
      }
      return http.Response('{}', 404);
    });
  }

  Widget attachmentApp({
    required http.Client client,
    List<Override> extraOverrides = const [],
  }) {
    return ProviderScope(
      overrides: [
        ...voiceThemeTestOverrides(),
        ...fixture.providerOverrides(),
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
        chatListControllerProvider.overrideWith(_E2eChatListController.new),
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

  testWidgets('E2E image attachment decrypts thumb via provider chain', (
    tester,
  ) async {
    await tester.runAsync(() async {
      await tester.pumpWidget(
        attachmentApp(
          client: clientForMessages(
            fileId: imageFileId,
            ciphertext: imageCiphertext,
            messages: [
              {
                'id': 'msg-e2e-1',
                'chat': {'id': chatId},
                'sender_profile_id': fixture.peerProfileId,
                'content': '',
                'attachments_json': jsonEncode([
                  {
                    'file_id': imageFileId,
                    'type': 'image',
                    'name': 'secret.png',
                    'e2e_key_wire': imageKeyWire,
                  },
                ]),
                'created_at': '2024-01-01T00:00:00Z',
              },
            ],
          ),
          extraOverrides: [
            e2eDecryptedAttachmentThumbProvider.overrideWith(
              (ref, request) async {
                if (request.fileId == imageFileId) return imageThumbBytes;
                return null;
              },
            ),
          ],
        ),
      );
      for (var i = 0; i < 40; i++) {
        await tester.pump(const Duration(milliseconds: 100));
      }
    });
    await tester.pump();

    expect(
      find.byKey(ChatRoomPanel.attachmentPreviewKey(imageFileId)),
      findsOneWidget,
    );
    expect(find.text('Could not decrypt attachment'), findsNothing);
    expect(find.byType(Image), findsOneWidget);
  });

  testWidgets('E2E document attachment decrypts on download tap', (
    tester,
  ) async {
    await tester.pumpWidget(
      attachmentApp(
        client: clientForMessages(
          fileId: docFileId,
          ciphertext: docCiphertext,
          messages: [
            {
              'id': 'msg-e2e-2',
              'chat': {'id': chatId},
              'sender_profile_id': fixture.peerProfileId,
              'content': '',
              'attachments_json': jsonEncode([
                {
                  'file_id': docFileId,
                  'type': 'document',
                  'name': 'report.pdf',
                  'size_bytes': 2048,
                  'e2e_key_wire': docKeyWire,
                },
              ]),
              'created_at': '2024-01-01T00:00:01Z',
            },
          ],
        ),
      ),
    );
    await tester.pump();
    await tester.pumpAndSettle(const Duration(seconds: 3));

    expect(find.text('report.pdf'), findsOneWidget);
    expect(
      find.textContaining('Tap to download', findRichText: true),
      findsOneWidget,
    );

    await tester.tap(find.text('report.pdf'));
    await tester.pumpAndSettle();

    expect(find.text('Could not decrypt attachment'), findsNothing);
  });
}

class _NoopRealtimeHub extends RealtimeHub {
  _NoopRealtimeHub(super.ref);

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}
}

Future<Uint8List> _solidPng(int width, int height) async {
  final recorder = ui.PictureRecorder();
  final canvas = Canvas(recorder);
  canvas.drawRect(
    Rect.fromLTWH(0, 0, width.toDouble(), height.toDouble()),
    Paint()..color = const Color(0xFF336699),
  );
  final picture = recorder.endRecording();
  final image = await picture.toImage(width, height);
  final byteData = await image.toByteData(format: ui.ImageByteFormat.png);
  image.dispose();
  return byteData!.buffer.asUint8List();
}

class _E2eChatListController extends ChatListController {
  _E2eChatListController(super.ref) : super() {
    state = ChatListState(
      items: [
        ChatListItem(
          chat: VoiceChat(
            id: 'chat-e2e-attach',
            type: 'CHAT_TYPE_DM',
            creatorProfileId: 'prof-test',
            e2eEnabled: true,
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
