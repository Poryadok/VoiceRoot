import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/users_client.dart';

import 'support/live_gateway_harness.dart';

void main() {
  test('avatar presigned upload round-trip', () async {
    final probe = await probeLiveGateway();
    expect(probe, isA<LiveGatewayReady>());
    final ctx = (probe as LiveGatewayReady).context;
    final user = await ctx.registerUser('avatar-phase1');
    if (!await ctx.probeFileStorageAvailable(user)) {
      markTestSkipped('object storage not configured (MinIO/R2)');
    }
    final users = VoiceUsersClient(gateway: ctx.gatewayHttp());
    final png = Uint8List.fromList([
      0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
    ]);
    final ticket = await users.createAvatarPresignedUpload(
      authorization: user.authorizationHeader,
      contentType: 'image/png',
      contentLength: png.length,
    );
    expect(ticket, isA<UsersApiOk<AvatarPresignedUpload>>());
    final upload = (ticket as UsersApiOk<AvatarPresignedUpload>).data;
    final put = await users.uploadAvatarBytes(
      uploadUrl: Uri.parse(upload.uploadUrl),
      requiredHeaders: upload.requiredHeaders,
      bytes: png,
    );
    expect(put, isA<UsersApiOk<void>>());
  }, skip: runLiveIntegration ? null : 'opt-in live');
}
