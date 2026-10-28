import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/proto_mappers.dart';
import 'package:voice_frontend/gen/voice/file/v1/file.pb.dart' as file_pb;
import 'package:voice_frontend/gen/voice/messaging/v1/messaging.pb.dart'
    as msg_pb;

void main() {
  test('voice upload binds AUDIO intent, MIME, byte length and chat', () {
    final request = requestUploadToProto(
      originalName: 'voice.wav',
      mimeType: 'audio/wav',
      sizeBytes: 48120,
      chatId: 'chat-1',
      voiceNote: true,
    );
    expect(request.intent, file_pb.UploadIntent.UPLOAD_INTENT_AUDIO);
    expect(request.intent.value, 7);
    expect(request.mimeType, 'audio/wav');
    expect(request.sizeBytes.toInt(), 48120);
    expect(request.contextChat.id, 'chat-1');
  });

  test('voice send binds typed VOICE and returned file ID with duration', () {
    final request = sendMessageRequestToProto(
      chatId: 'chat-1',
      content: '',
      attachments: const [
        MessageAttachment(
          fileId: 'confirmed-file',
          type: 'voice_message',
          name: 'voice.wav',
          sizeBytes: 48120,
          durationSeconds: 3,
        ),
      ],
    );
    expect(
      request.contentType,
      msg_pb.MessageContentType.MESSAGE_CONTENT_TYPE_VOICE,
    );
    final attachment =
        (jsonDecode(request.attachmentsJson) as List).single as Map;
    expect(attachment['file_id'], 'confirmed-file');
    expect(attachment['type'], 'voice_message');
    expect(attachment['duration_seconds'], 3);
  });

  test(
    'ordinary attachments keep legacy intent and untyped message behavior',
    () {
      final upload = requestUploadToProto(
        originalName: 'report.pdf',
        mimeType: 'application/pdf',
        sizeBytes: 4,
        chatId: 'chat-1',
      );
      expect(upload.intent, file_pb.UploadIntent.UPLOAD_INTENT_UNSPECIFIED);
      final message = sendMessageRequestToProto(
        chatId: 'chat-1',
        content: 'caption',
        attachments: const [
          MessageAttachment(fileId: 'file', type: 'document'),
        ],
      );
      expect(message.hasContentType(), isFalse);
      expect(message.content, 'caption');
    },
  );
}
