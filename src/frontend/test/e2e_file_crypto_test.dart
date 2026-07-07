import 'package:fixnum/fixnum.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/proto_mappers.dart';
import 'package:voice_frontend/gen/voice/chat/v1/chat.pbenum.dart';
import 'package:voice_frontend/gen/voice/file/v1/file.pb.dart' as file_pb;

/// app stack5 E2E-B red tests: encrypted file upload wire mapping (docs/features/encryption.md).
void main() {
  group('requestUploadToProto', () {
    test('sets isE2e when requested for E2E DM attachment', () {
      final proto = requestUploadToProto(
        originalName: 'cipher.bin',
        mimeType: 'application/octet-stream',
        sizeBytes: 1024,
        chatId: 'chat-dm-1',
        chatType: ChatType.CHAT_TYPE_DM,
        isE2e: true,
      );

      // RED: mapper must accept isE2e and forward to RequestUploadRequest.
      expect(proto.hasIsE2e(), isTrue);
      expect(proto.isE2e, isTrue);
      expect(proto.contextChat.id, 'chat-dm-1');
    });

    test('omits isE2e for plaintext uploads', () {
      final proto = requestUploadToProto(
        originalName: 'plain.txt',
        mimeType: 'text/plain',
        sizeBytes: 64,
        chatId: 'chat-dm-2',
        chatType: ChatType.CHAT_TYPE_DM,
      );

      expect(proto.hasIsE2e(), isFalse);
    });

    test('sets contextStory when storyId provided', () {
      final proto = requestUploadToProto(
        originalName: 'clip.mp4',
        mimeType: 'video/mp4',
        sizeBytes: 2048,
        storyId: 'story-42',
      );

      expect(proto.hasContextStory(), isTrue);
      expect(proto.contextStory.storyId, 'story-42');
    });
  });

  group('fileMetadataFromProto', () {
    test('preserves isE2e on metadata roundtrip', () {
      final meta = file_pb.FileMetadata(
        id: 'file-1',
        fileType: 'other',
        status: 'ready',
        originalName: 'cipher.bin',
        isE2e: true,
        sizeBytes: Int64(1024),
      );
      final mapped = fileMetadataFromProto(meta);
      expect(mapped.fileId, 'file-1');
      expect(meta.isE2e, isTrue);
    });
  });
}
