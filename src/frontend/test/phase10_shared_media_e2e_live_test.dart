import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/files_client.dart';
import 'package:voice_frontend/backend/messages_client.dart';

import 'support/live_gateway_harness.dart';

void main() {
  test('list shared media after file attachment', () async {
    final probe = await probeLiveGateway();
    expect(
      probe,
      isA<LiveGatewayReady>(),
      reason: probe is LiveGatewayUnavailable ? probe.reason : null,
    );
    final ctx = (probe as LiveGatewayReady).context;

    final sessionA = await ctx.registerUser('shared-media-a');
    final sessionB = await ctx.registerUser('shared-media-b');

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

    final bytes = Uint8List.fromList([115, 104, 97, 114, 101, 100]); // shared
    final files = ctx.filesClient();
    final ticketResult = await files.requestUpload(
      authorization: sessionA.authorizationHeader,
      originalName: 'shared.txt',
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

    final fileType = (confirm as FilesApiOk<FileMetadataData>).data.fileType;
    final attachmentType =
        fileType.trim().isEmpty ? 'document' : fileType.trim();

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

    final linkSend = await ctx.messagesClient().sendMessage(
      authorization: sessionA.authorizationHeader,
      chatId: chatId,
      content: 'see https://voice.app/docs',
      clientMessageId: qaClientMessageId(),
    );
    expect(linkSend, isA<MessagesApiOk<VoiceMessage>>());

    final filesList = await ctx.messagesClient().listSharedMedia(
      authorization: sessionA.authorizationHeader,
      chatId: chatId,
      kind: SharedMediaTabKind.files,
    );
    expect(filesList, isA<MessagesApiOk<SharedMediaListData>>());
    expect(
      (filesList as MessagesApiOk<SharedMediaListData>).data.items,
      isNotEmpty,
    );

    final linksList = await ctx.messagesClient().listSharedMedia(
      authorization: sessionA.authorizationHeader,
      chatId: chatId,
      kind: SharedMediaTabKind.links,
    );
    expect(linksList, isA<MessagesApiOk<SharedMediaListData>>());
    expect(
      (linksList as MessagesApiOk<SharedMediaListData>).data.items,
      isNotEmpty,
    );
  });
}
