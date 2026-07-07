import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/e2e_client.dart';
import 'package:voice_frontend/backend/files_client.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/e2e/e2e_crypto_adapter.dart';
import 'package:voice_frontend/e2e/e2e_file_crypto.dart';
import 'package:voice_frontend/e2e/e2e_message_service.dart';

import 'support/live_gateway_harness.dart';

/// app stack5 live E2E file: encrypted upload in E2E-enabled DM skips server image processing.
///
/// Run when full compose stack is up:
/// ```text
/// flutter test test/e2e_file_live_test.dart ^
///   --dart-define=VOICE_RUN_LIVE_INTEGRATION=true ^
///   --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:18080
/// ```
void main() {
  test(
    'E2E DM encrypted file upload has no server thumbnail',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final sessionA = await ctx.registerUser('e2e-file-a');
      final sessionB = await ctx.registerUser('e2e-file-b');
      if (!await ctx.probeFileStorageAvailable(sessionA)) {
        markTestSkipped('object storage not configured');
      }

      final gateway = GatewayHttpClient(
        httpClient: ctx.httpClient,
        config: ctx.config,
      );
      final chats = VoiceChatsClient(gateway: gateway);
      final adapterA = E2eCryptoAdapter.inMemoryForTest();
      final adapterB = E2eCryptoAdapter.inMemoryForTest();
      final e2eA = VoiceE2eClient(gateway: gateway, adapter: adapterA);
      final e2eB = VoiceE2eClient(gateway: gateway, adapter: adapterB);
      final files = VoiceFilesClient(gateway: gateway);
      final messages = VoiceMessagesClient(gateway: gateway);
      final messageServiceA = E2eMessageService(
        adapter: adapterA,
        e2eClient: e2eA,
      );
      const fileCrypto = E2eFileCrypto();

      final dmResult = await chats.createDm(
        authorization: sessionA.authorizationHeader,
        otherProfileId: sessionB.activeProfileId,
      );
      expect(dmResult, isA<ChatsApiOk<VoiceChat>>());
      final chatId = (dmResult as ChatsApiOk<VoiceChat>).data.id;

      expect(
        await e2eA.uploadPreKeyBundle(
          authorization: sessionA.authorizationHeader,
        ),
        isA<E2eApiOk<void>>(),
      );
      expect(
        await e2eB.uploadPreKeyBundle(
          authorization: sessionB.authorizationHeader,
        ),
        isA<E2eApiOk<void>>(),
      );
      expect(
        await e2eA.enableChatE2e(
          authorization: sessionA.authorizationHeader,
          chatId: chatId,
        ),
        isA<E2eApiOk<void>>(),
      );
      expect(
        await e2eB.enableChatE2e(
          authorization: sessionB.authorizationHeader,
          chatId: chatId,
        ),
        isA<E2eApiOk<void>>(),
      );

      // Minimal PNG header bytes — server must not thumbnail/process when is_e2e.
      final plaintext = Uint8List.fromList([
        0x89,
        0x50,
        0x4e,
        0x47,
        0x0d,
        0x0a,
        0x1a,
        0x0a,
        0x00,
        0x00,
        0x00,
        0x0d,
      ]);
      final encrypted = await fileCrypto.encryptBytes(
        plaintext: plaintext,
        messageService: messageServiceA,
        localProfileId: sessionA.activeProfileId,
        peerProfileId: sessionB.activeProfileId,
        authorization: sessionA.authorizationHeader,
        chatId: chatId,
      );
      expect(encrypted.ciphertext, isNot(equals(plaintext)));

      final ticketResult = await files.requestUpload(
        authorization: sessionA.authorizationHeader,
        originalName: 'cipher.png',
        mimeType: 'image/png',
        sizeBytes: encrypted.ciphertext.length,
        chatId: chatId,
        chatType: 'CHAT_TYPE_DM',
        isE2e: true,
      );
      expect(ticketResult, isA<FilesApiOk<FileUploadTicket>>());
      final ticket = (ticketResult as FilesApiOk<FileUploadTicket>).data;

      final put = await files.putBytes(
        uploadUrl: ticket.presignedPutUrl,
        bytes: encrypted.ciphertext,
        mimeType: 'image/png',
      );
      expect(put, isA<FilesApiOk<void>>());

      final confirm = await files.confirmUpload(
        authorization: sessionA.authorizationHeader,
        fileId: ticket.fileId,
        bytes: encrypted.ciphertext,
      );
      expect(confirm, isA<FilesApiOk<FileMetadataData>>());
      final metadata = (confirm as FilesApiOk<FileMetadataData>).data;
      expect(metadata.isE2e, isTrue);
      expect(metadata.status, 'ready');
      expect(metadata.previewUrl, isNull);

      final attachmentType =
          metadata.fileType.trim().isEmpty ? 'file' : metadata.fileType.trim();
      final send = await messages.sendMessage(
        authorization: sessionA.authorizationHeader,
        chatId: chatId,
        content: '',
        isE2e: true,
        clientMessageId: qaClientMessageId(),
        attachments: [
          MessageAttachment(
            fileId: metadata.fileId,
            type: attachmentType,
            name: metadata.originalName,
            sizeBytes: metadata.sizeBytes,
            e2eKeyWire: encrypted.keyWire,
          ),
        ],
      );
      expect(send, isA<MessagesApiOk<VoiceMessage>>());
      final sent = (send as MessagesApiOk<VoiceMessage>).data;
      expect(sent.isE2e, isTrue);
      expect(sent.attachments, isNotEmpty);
      expect(sent.attachments.first.e2eKeyWire, isNotEmpty);

      final bMessages = await messages.getMessages(
        authorization: sessionB.authorizationHeader,
        chatId: chatId,
      );
      expect(bMessages, isA<MessagesApiOk<MessageListData>>());
      final received = (bMessages as MessagesApiOk<MessageListData>)
          .data
          .messages
          .firstWhere((m) => m.id == sent.id);
      final attachment = received.attachments.first;
      expect(attachment.e2eKeyWire, isNotEmpty);

      final downloaded = await files.fetchFileBytes(
        authorization: sessionB.authorizationHeader,
        fileId: attachment.fileId,
      );
      expect(downloaded, isA<FilesApiOk<Uint8List>>());
      final ciphertext = (downloaded as FilesApiOk<Uint8List>).data;

      final messageServiceB = E2eMessageService(
        adapter: adapterB,
        e2eClient: e2eB,
      );
      final decrypted = await fileCrypto.decryptBytes(
        ciphertext: ciphertext,
        keyWire: attachment.e2eKeyWire!,
        messageService: messageServiceB,
        localProfileId: sessionB.activeProfileId,
        peerProfileId: sessionA.activeProfileId,
        authorization: sessionB.authorizationHeader,
      );
      expect(decrypted, equals(plaintext));

      // Stored attachment wire must carry e2e_key_wire for peer decrypt.
      final attachmentsJson = jsonEncode(
        received.attachments.map((a) => a.toJson()).toList(),
      );
      expect(attachmentsJson, contains('e2e_key_wire'));
    },
    skip: runLiveIntegration ? null : 'opt-in live',
    timeout: const Timeout(Duration(minutes: 2)),
  );
}
