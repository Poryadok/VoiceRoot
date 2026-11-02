import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:fixnum/fixnum.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/proto_mappers.dart';
import 'package:voice_frontend/backend/message_cache/in_memory_message_cache_store.dart';
import 'package:voice_frontend/gen/voice/messaging/v1/messaging.pb.dart' as pb;
import 'package:voice_frontend/ui/chat/message_view_count.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/state/chat_providers.dart';

import 'support/auth_test_overrides.dart';

void main() {
  for (final messageId in ['text-message', 'offline-file-message']) {
    test(
      '$messageId stays unread on sender own read synchronization',
      () async {
        late _ReceiptHub hub;
        final container = ProviderContainer(
          overrides: [
            ...voiceAppTestOverrides(
              client: MockClient((_) async => http.Response('{}', 404)),
            ),
            realtimeHubProvider.overrideWith((ref) => hub = _ReceiptHub(ref)),
          ],
        );
        addTearDown(container.dispose);
        final subscription = container.listen(
          chatRoomControllerProvider('dm'),
          (_, _) {},
        );
        addTearDown(subscription.close);
        await pumpEventQueue();
        final controller = container.read(
          chatRoomControllerProvider('dm').notifier,
        );
        controller.state = ChatRoomState(
          historyProfileId: 'prof-test',
          messages: [
            VoiceMessage(
              id: messageId,
              chatId: 'dm',
              senderProfileId: 'prof-test',
              content: 'outbound',
              attachments: messageId == 'offline-file-message'
                  ? const [
                      MessageAttachment(
                        fileId: 'file',
                        type: 'file',
                        name: 'qa-upload.txt',
                      ),
                    ]
                  : const [],
            ),
          ],
        );
        void receipt(String? reader) => hub.add(
          RealtimeFrame(
            op: 'message_read',
            data: {
              'chat_id': 'dm',
              'message_id': messageId,
              'profile_id': ?reader,
            },
          ),
        );
        receipt('prof-test');
        await pumpEventQueue();
        expect(
          controller.state.readMessageIds,
          isEmpty,
          reason: 'Sender read synchronization is not a peer receipt',
        );
        expect(controller.state.deliveredMessageIds, isEmpty);
        receipt(null);
        await pumpEventQueue();
        expect(controller.state.readMessageIds, isEmpty);
        receipt('peer-profile');
        await pumpEventQueue();
        expect(controller.state.readMessageIds, contains(messageId));
        expect(controller.state.deliveredMessageIds, contains(messageId));
      },
    );
  }
  testWidgets(
    'failed viewport record retries on reentry and stops after success',
    (tester) async {
      var calls = 0;
      final scroll = ScrollController();
      await tester.pumpWidget(
        MaterialApp(
          home: ListView(
            cacheExtent: 2000,
            controller: scroll,
            children: [
              MessageViewportView(
                onVisible: () async {
                  calls++;
                  return calls > 1;
                },
                child: const SizedBox(height: 80, child: Text('target')),
              ),
              const SizedBox(height: 1200),
            ],
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(calls, 1);
      await tester.drag(find.byType(ListView), const Offset(0, -700));
      await tester.pumpAndSettle();
      await tester.drag(find.byType(ListView), const Offset(0, 700));
      await tester.pumpAndSettle();
      expect(calls, 2);
      await tester.drag(find.byType(ListView), const Offset(0, -700));
      await tester.pumpAndSettle();
      await tester.drag(find.byType(ListView), const Offset(0, 700));
      await tester.pumpAndSettle();
      expect(calls, 2);
      await tester.pumpWidget(const SizedBox());
      scroll.dispose();
    },
  );
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
                onVisible: () async {
                  views++;
                  return true;
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

class _ReceiptHub extends RealtimeHub {
  _ReceiptHub(super.ref);
  final _frames = StreamController<RealtimeFrame>.broadcast();
  @override
  Stream<RealtimeFrame> get events => _frames.stream;
  @override
  Future<void> ensureConnected() async {}
  @override
  void ensureSubscribed(String chatId) {}
  void add(RealtimeFrame frame) => _frames.add(frame);
  @override
  Future<void> dispose() => _frames.close();
}
