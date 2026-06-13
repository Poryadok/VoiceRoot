import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/proto_mappers.dart';
import 'package:voice_frontend/gen/voice/messaging/v1/messaging.pb.dart' as messaging_pb;

void main() {
  test('sharedMediaListFromProto maps items and pagination', () {
    final list = messaging_pb.SharedMediaList(
      items: [
        messaging_pb.SharedMediaItem(
          messageId: 'msg-1',
          senderProfileId: 'prof-1',
          fileId: 'file-1',
          attachmentType: 'image',
          sortOrder: 0,
        ),
        messaging_pb.SharedMediaItem(
          messageId: 'msg-2',
          senderProfileId: 'prof-1',
          externalUrl: 'https://example.com',
          title: 'Example',
          sortOrder: 0,
        ),
      ],
      nextCursor: 'cursor-1',
      hasMore: true,
    );

    final data = sharedMediaListFromProto(list);
    expect(data.items, hasLength(2));
    expect(data.items.first.fileId, 'file-1');
    expect(data.items.last.externalUrl, 'https://example.com');
    expect(data.nextCursor, 'cursor-1');
    expect(data.hasMore, isTrue);
  });

  test('SharedMediaTabKind wire values', () {
    expect(SharedMediaTabKind.media.wireValue, 'media');
    expect(SharedMediaTabKind.links.wireValue, 'links');
  });
}
