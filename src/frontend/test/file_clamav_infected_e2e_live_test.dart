import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/files_client.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/gen/voice/file/v1/file.pb.dart' as file_pb;

import 'support/live_gateway_harness.dart';

// Keep the standard 68 bytes exact; a malformed signature tests a clean upload.
Uint8List _eicarBytes() => Uint8List.fromList(
  utf8.encode(
    r'X5O!P%@AP[4\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*',
  ),
);

void main() {
  test('EICAR fixture is the exact 68-byte antivirus test file', () {
    final bytes = _eicarBytes();
    expect(bytes, hasLength(68));
    expect(
      VoiceFilesClient.sha256Hex(bytes),
      '275a021bbfb6489e54d471899f7db9d1663fc695ec2fe2a2c4538aabf651fd0f',
    );
  });

  test(
    'infected file rejected on confirm',
    () async {
      final probe = await probeLiveGateway();
      expect(probe, isA<LiveGatewayReady>());
      final ctx = (probe as LiveGatewayReady).context;
      final a = await ctx.registerUser('clamav-a');
      final b = await ctx.registerUser('clamav-b');
      if (!await ctx.probeFileStorageAvailable(a)) {
        markTestSkipped('object storage not configured');
        return;
      }
      final dm = await ctx.chatsClient().createDm(
        authorization: a.authorizationHeader,
        otherProfileId: b.activeProfileId,
      );
      final chatId = (dm as ChatsApiOk<VoiceChat>).data.id;
      final eicar = _eicarBytes();
      final files = ctx.filesClient();
      final ticketResult = await files.requestUpload(
        authorization: a.authorizationHeader,
        originalName: 'eicar.exe',
        mimeType: 'application/x-msdownload',
        sizeBytes: eicar.length,
        chatId: chatId,
        chatType: 'CHAT_TYPE_DM',
      );
      expect(ticketResult, isA<FilesApiOk<FileUploadTicket>>());
      final ticket = (ticketResult as FilesApiOk<FileUploadTicket>).data;
      expect(
        await files.putBytes(
          uploadUrl: ticket.presignedPutUrl,
          bytes: eicar,
          mimeType: 'application/x-msdownload',
        ),
        isA<FilesApiOk<void>>(),
      );
      final confirm = await files.confirmUpload(
        authorization: a.authorizationHeader,
        fileId: ticket.fileId,
        bytes: eicar,
      );
      // Gateway maps infected metadata to this domain error. A generic failure
      // could be a storage/scanner outage and does not prove virus detection.
      expect(confirm, isA<FilesApiFailure>());
      final rejection = confirm as FilesApiFailure;
      expect(rejection.statusCode, 412);
      expect(rejection.errorCode, 'file_infected');
      final gateway = ctx.gatewayHttp();
      final metadata = await gateway.getProto(
        gateway.resolve('/api/v1/files/${ticket.fileId}'),
        authorization: a.authorizationHeader,
        createEmpty: file_pb.GetFileMetadataResponse.create,
      );
      expect(metadata, isA<GatewayHttpOk<file_pb.GetFileMetadataResponse>>());
      final file = (metadata as GatewayHttpOk<file_pb.GetFileMetadataResponse>)
          .data
          .fileMetadata;
      expect(file.id, ticket.fileId);
      expect(file.status, 'failed');
      expect(file.scanResult, 'infected');
      final download = await files.getFileUrl(
        authorization: a.authorizationHeader,
        fileId: ticket.fileId,
      );
      expect(download, isA<FilesApiFailure>());
      expect((download as FilesApiFailure).statusCode, 412);
    },
    timeout: const Timeout(Duration(minutes: 2)),
    skip: runLiveIntegration ? null : 'opt-in live',
  );
}
