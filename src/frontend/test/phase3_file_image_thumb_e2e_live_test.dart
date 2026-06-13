import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/files_client.dart';

import 'support/live_gateway_harness.dart';

void main() {
  test('image upload returns thumb metadata', () async {
    final probe = await probeLiveGateway();
    expect(probe, isA<LiveGatewayReady>());
    final ctx = (probe as LiveGatewayReady).context;
    final a = await ctx.registerUser('img-thumb-a');
    final b = await ctx.registerUser('img-thumb-b');
    if (!await ctx.probeFileStorageAvailable(a)) {
      markTestSkipped('object storage not configured');
    }
    final dm = await ctx.chatsClient().createDm(
      authorization: a.authorizationHeader,
      otherProfileId: b.activeProfileId,
    );
    final chatId = (dm as ChatsApiOk<VoiceChat>).data.id;
    final png = Uint8List.fromList([
      0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
      0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
      0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00,
      0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
      0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49,
      0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
    ]);
    final files = ctx.filesClient();
    final ticketResult = await files.requestUpload(
      authorization: a.authorizationHeader,
      originalName: 'thumb.png',
      mimeType: 'image/png',
      sizeBytes: png.length,
      chatId: chatId,
      chatType: 'CHAT_TYPE_DM',
    );
    expect(ticketResult, isA<FilesApiOk<FileUploadTicket>>());
    final ticket = (ticketResult as FilesApiOk<FileUploadTicket>).data;
    await files.putBytes(
      uploadUrl: ticket.presignedPutUrl,
      bytes: png,
      mimeType: 'image/png',
    );
    final confirm = await files.confirmUpload(
      authorization: a.authorizationHeader,
      fileId: ticket.fileId,
      bytes: png,
    );
    expect(confirm, isA<FilesApiOk<FileMetadataData>>());
    final meta = (confirm as FilesApiOk<FileMetadataData>).data;
    expect(meta.previewUrl, isNotEmpty);
  }, skip: runLiveIntegration ? null : 'opt-in live');
}
