import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/proto_mappers.dart';
import 'package:voice_frontend/gen/voice/messaging/v1/messaging.pb.dart' as pb;
import 'package:voice_frontend/gen/voice/messaging/v1/messaging.pbenum.dart';

void main() {
  test('message mapper preserves every known content type', () {
    for (final contentType in MessageContentType.values) {
      final message = voiceMessageFromProto(
        pb.Message(id: 'message', contentType: contentType),
      );

      expect(message.contentType, contentType);
    }
  });

  test('message mapper leaves an absent content type unspecified', () {
    final message = voiceMessageFromProto(pb.Message(id: 'message'));

    expect(message.contentType, isNull);
    expect(
      const VoiceMessage(
        id: 'legacy',
        chatId: 'chat',
        senderProfileId: 'sender',
        content: 'legacy constructor',
      ).contentType,
      isNull,
    );
  });

  test('unknown protobuf enum value remains absent from the UI model', () {
    // Field 21 is Message.content_type (varint enum); 99 is outside the
    // currently known MessageContentType values.
    final message = voiceMessageFromProto(
      pb.Message.fromBuffer(const [0xa8, 0x01, 0x63]),
    );

    expect(message.contentType, isNull);
  });
}
