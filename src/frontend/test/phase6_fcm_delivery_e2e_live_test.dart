import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/notifications_client.dart';

import 'support/live_gateway_harness.dart';

String notificationDebugBase() {
  const fromEnv = String.fromEnvironment('VOICE_NOTIFICATION_DEBUG_URL');
  if (fromEnv.isNotEmpty) return fromEnv;
  return 'http://127.0.0.1:18091';
}

void main() {
  test('offline DM triggers recorded FCM push payload', () async {
    final probe = await probeLiveGateway();
    expect(probe, isA<LiveGatewayReady>());
    final ctx = (probe as LiveGatewayReady).context;
    final a = await ctx.registerUser('fcm-del-a');
    final b = await ctx.registerUser('fcm-del-b');
    final notifications = VoiceNotificationsClient(gateway: ctx.gatewayHttp());

    await notifications.registerDevice(
      authorization: b.authorizationHeader,
      platform: 'web',
      token: 'qa-fcm-delivery-${b.activeProfileId}',
    );

    final dm = await ctx.chatsClient().createDm(
      authorization: a.authorizationHeader,
      otherProfileId: b.activeProfileId,
    );
    final chatId = (dm as ChatsApiOk<VoiceChat>).data.id;

    final send = await ctx.messagesClient().sendMessage(
      authorization: a.authorizationHeader,
      chatId: chatId,
      content: 'fcm delivery probe ${DateTime.now().millisecondsSinceEpoch}',
      clientMessageId: qaClientMessageId(),
    );
    expect(send, isA<MessagesApiOk<VoiceMessage>>());

    final uri = Uri.parse(
      '${notificationDebugBase()}/debug/recorded-pushes?profile_id=${b.activeProfileId}',
    );
    RecordedPush? recorded;
    for (var i = 0; i < 20; i++) {
      await Future<void>.delayed(const Duration(milliseconds: 500));
      final resp = await http.get(uri);
      if (resp.statusCode == 200) {
        final map = jsonDecode(resp.body) as Map<String, dynamic>;
        recorded = RecordedPush.fromJson(map);
        break;
      }
    }
    expect(recorded, isNotNull);
    expect(recorded!.body, isNotEmpty);
  }, skip: runLiveIntegration ? null : 'opt-in live');
}

class RecordedPush {
  RecordedPush({required this.body, required this.type});
  final String body;
  final String type;

  factory RecordedPush.fromJson(Map<String, dynamic> json) => RecordedPush(
        body: json['Body'] as String? ?? json['body'] as String? ?? '',
        type: json['Type'] as String? ?? json['type'] as String? ?? '',
      );
}
