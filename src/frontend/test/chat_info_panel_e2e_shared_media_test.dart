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
import 'package:voice_frontend/ui/chat/chat_info_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/e2e_attachment_test_fixture.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  const chatId = 'dm-e2e-shared-media';
  const imageFileId = 'file-shared-e2e-img';
  late Uint8List pngBytes;
  late E2eAttachmentTestFixture fixture;
  late Uint8List imageCiphertext;
  late String imageKeyWire;
  late Uint8List decryptedImageBytes;
  late Uint8List imageThumbBytes;

  setUpAll(() async {
    TestWidgetsFlutterBinding.ensureInitialized();
    pngBytes = await _solidPng(32, 32);
    fixture = await E2eAttachmentTestFixture.create();
    final image = await fixture.encryptFileForPeer(
      plaintext: pngBytes,
      chatId: chatId,
    );
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
  });

  Widget sharedMediaApp({
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
        chatListControllerProvider.overrideWith(_E2eDmChatListController.new),
        ...extraOverrides,
      ],
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(
          body: SizedBox(
            height: 600,
            width: 400,
            child: ChatInfoPanel(chatId: chatId, isGroup: false),
          ),
        ),
      ),
    );
  }

  http.Client mockSharedMediaClient({
    required String kind,
    required String fileId,
    required Uint8List ciphertext,
  }) {
    return MockClient((req) async {
      if (req.url.path.contains('/shared-media') &&
          req.url.queryParameters['kind'] == kind) {
        return http.Response(
          jsonEncode({
            'shared_media_list': {
              'items': [
                {
                  'message_id': 'msg-shared-1',
                  'sender_profile_id': fixture.peerProfileId,
                  'file_id': fileId,
                  'attachment_type': kind == 'media' ? 'image' : 'document',
                  'original_name': kind == 'media' ? 'photo.png' : 'doc.pdf',
                  'sort_order': 0,
                  'e2eKeyWire': imageKeyWire,
                },
              ],
              'next_cursor': '',
              'has_more': false,
            },
          }),
          200,
        );
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

  testWidgets('E2E shared media tab shows decrypted image thumb', (
    tester,
  ) async {
    await tester.runAsync(() async {
      await tester.pumpWidget(
        sharedMediaApp(
          client: mockSharedMediaClient(
            kind: 'media',
            fileId: imageFileId,
            ciphertext: imageCiphertext,
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

    expect(find.byKey(ChatInfoPanel.mediaTabKey), findsOneWidget);
    expect(find.byIcon(Icons.lock_outline), findsNothing);
    expect(find.byType(Image), findsOneWidget);
  });

  testWidgets('E2E shared files tab shows download affordance not lock icon', (
    tester,
  ) async {
    const docFileId = 'file-shared-e2e-doc';
    await tester.pumpWidget(
      sharedMediaApp(
        client: mockSharedMediaClient(
          kind: 'files',
          fileId: docFileId,
          ciphertext: imageCiphertext,
        ),
      ),
    );
    await tester.pump();
    await tester.tap(find.byKey(ChatInfoPanel.filesTabKey));
    await tester.pump();
    await tester.pumpAndSettle(const Duration(seconds: 3));

    expect(find.text('doc.pdf'), findsOneWidget);
    expect(find.byIcon(Icons.lock_outline), findsNothing);
    expect(find.byIcon(Icons.download_outlined), findsOneWidget);
  });
}

class _E2eDmChatListController extends ChatListController {
  _E2eDmChatListController(super.ref) : super() {
    state = ChatListState(
      items: [
        ChatListItem(
          chat: VoiceChat(
            id: 'dm-e2e-shared-media',
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
