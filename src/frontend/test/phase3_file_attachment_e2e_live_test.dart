import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/files_client.dart';
import 'package:voice_frontend/backend/messages_client.dart';

import 'support/live_gateway_harness.dart';

void main() {
  test(
    'upload file and send attachment message',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final sessionA = await ctx.registerUser('file-a');
      final sessionB = await ctx.registerUser('file-b');

      if (!await ctx.probeFileStorageAvailable(sessionA)) {
        markTestSkipped(
          'object storage not configured (MinIO/R2); set FILE_R2_* in .env',
        );
      }

      final dm = await ctx.chatsClient().createDm(
        authorization: sessionA.authorizationHeader,
        otherProfileId: sessionB.activeProfileId,
      );
      final chatId = (dm as ChatsApiOk<VoiceChat>).data.id;

      final bytes = Uint8List.fromList([101, 50, 101, 10]); // "e2e\n"
      final files = ctx.filesClient();
      final ticketResult = await files.requestUpload(
        authorization: sessionA.authorizationHeader,
        originalName: 'e2e.txt',
        mimeType: 'text/plain',
        sizeBytes: bytes.length,
        chatId: chatId,
        chatType: 'CHAT_TYPE_DM',
      );
      expect(ticketResult, isA<FilesApiOk<FileUploadTicket>>());
      final ticket = (ticketResult as FilesApiOk<FileUploadTicket>).data;

      final put = await files.putBytes(
        uploadUrl: ticket.presignedPutUrl,
        bytes: bytes,
        mimeType: 'text/plain',
      );
      expect(put, isA<FilesApiOk<void>>());

      final confirm = await files.confirmUpload(
        authorization: sessionA.authorizationHeader,
        fileId: ticket.fileId,
        bytes: bytes,
      );
      expect(confirm, isA<FilesApiOk<FileMetadataData>>());

      final wsB = await ctx.connectSubscribed(sessionB, chatId);
      addTearDown(wsB.dispose);

      final fileType = (confirm as FilesApiOk<FileMetadataData>).data.fileType;
      final attachmentType =
          fileType.trim().isEmpty ? 'file' : fileType.trim();

      final frameFuture = waitForOp(wsB.events, 'message_create');
      final attachmentsJson = jsonEncode([
        {'file_id': ticket.fileId, 'type': attachmentType},
      ]);
      final send = await ctx.messagesClient().sendMessage(
        authorization: sessionA.authorizationHeader,
        chatId: chatId,
        content: '',
        clientMessageId: qaClientMessageId(),
        attachments: MessageAttachment.listFromWire(attachmentsJson),
      );
      expect(send, isA<MessagesApiOk<VoiceMessage>>());
      final msgId = (send as MessagesApiOk<VoiceMessage>).data.id;

      final frame = await frameFuture;
      expect(frame.data?['message_id'], msgId);
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
