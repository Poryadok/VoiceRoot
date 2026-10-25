import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:fixnum/fixnum.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/proto_mappers.dart';
import 'package:voice_frontend/backend/message_cache/in_memory_message_cache_store.dart';
import 'package:voice_frontend/gen/voice/messaging/v1/messaging.pb.dart' as pb;
import 'package:voice_frontend/ui/chat/message_view_count.dart';

void main() {
  test('durable optional views survive protobuf, cache and copy', () {
    final message = voiceMessageFromProto(
      pb.Message(id: 'message', viewCount: Int64(42)),
    );
    expect(message.viewCount, 42);
    expect(
      InMemoryMessageCacheStore.decodePayload(
        InMemoryMessageCacheStore.encodePayload(message),
      ).copyWith(content: 'edited').viewCount,
      42,
    );
    expect(voiceMessageFromProto(pb.Message(id: 'legacy')).viewCount, isNull);
    expect(
      const VoiceMessage(
        id: 'dm',
        chatId: 'chat',
        senderProfileId: 'peer',
        content: 'text',
      ).viewCount,
      isNull,
    );
  });
  testWidgets('viewport qualification ignores prefetched offscreen bubble', (
    tester,
  ) async {
    var views = 0;
    final controller = ScrollController();
    await tester.pumpWidget(
      MaterialApp(
        home: SizedBox(
          height: 200,
          child: ListView(
            controller: controller,
            children: [
              const SizedBox(height: 1200),
              MessageViewportView(
                onVisible: () {
                  views++;
                },
                child: const SizedBox(height: 60, child: Text('target')),
              ),
            ],
          ),
        ),
      ),
    );
    await tester.pump();
    expect(views, 0);
    await tester.drag(find.byType(ListView), const Offset(0, -900));
    await tester.pumpAndSettle();
    expect(views, 1);
    await tester.pump();
    expect(views, 1);
    await tester.pumpWidget(const SizedBox());
    controller.dispose();
  });
  testWidgets('view count footer displays an eye and exact count', (
    tester,
  ) async {
    await tester.pumpWidget(
      const MaterialApp(home: Scaffold(body: MessageViewCount(count: 42))),
    );
    expect(find.byIcon(Icons.visibility_outlined), findsOneWidget);
    expect(find.text('42'), findsOneWidget);
  });
}
