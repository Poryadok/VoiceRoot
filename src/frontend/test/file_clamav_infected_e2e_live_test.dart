import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/files_client.dart';

import 'support/live_gateway_harness.dart';

void main() {
  test('infected file rejected on confirm', () async {
    final probe = await probeLiveGateway();
    expect(probe, isA<LiveGatewayReady>());
    final ctx = (probe as LiveGatewayReady).context;
    final a = await ctx.registerUser('clamav-a');
    final b = await ctx.registerUser('clamav-b');
    if (!await ctx.probeFileStorageAvailable(a)) {
      markTestSkipped('object storage not configured');
    }
    final dm = await ctx.chatsClient().createDm(
      authorization: a.authorizationHeader,
      otherProfileId: b.activeProfileId,
    );
    final chatId = (dm as ChatsApiOk<VoiceChat>).data.id;
    final eicar = Uint8List.fromList(utf8.encode(
      r'X5O!P%@AP[4\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*',
    ));
    final files = ctx.filesClient();
    final ticketResult = await files.requestUpload(
      authorization: a.authorizationHeader,
      originalName: 'eicar.com',
      mimeType: 'text/plain',
      sizeBytes: eicar.length,
      chatId: chatId,
      chatType: 'CHAT_TYPE_DM',
    );
    expect(ticketResult, isA<FilesApiOk<FileUploadTicket>>());
    final ticket = (ticketResult as FilesApiOk<FileUploadTicket>).data;
    await files.putBytes(
      uploadUrl: ticket.presignedPutUrl,
      bytes: eicar,
      mimeType: 'text/plain',
    );
    final confirm = await files.confirmUpload(
      authorization: a.authorizationHeader,
      fileId: ticket.fileId,
      bytes: eicar,
    );
    expect(confirm, isA<FilesApiFailure>());
  }, skip: runLiveIntegration ? null : 'opt-in live');
}
