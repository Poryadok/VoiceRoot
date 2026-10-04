import 'dart:convert';
import 'dart:io';

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

const _diagnosticFile = String.fromEnvironment('VOICE_FCM_DIAGNOSTIC_FILE');

void main() {
  test('offline DM triggers recorded FCM push payload', () async {
    final probe = await probeLiveGateway();
    expect(probe, isA<LiveGatewayReady>());
    final ctx = (probe as LiveGatewayReady).context;
    final a = await ctx.registerUser('fcm-del-a');
    final b = await ctx.registerUser('fcm-del-b');
    final notifications = VoiceNotificationsClient(gateway: ctx.gatewayHttp());

    final registration = await notifications.registerDevice(
      authorization: b.authorizationHeader,
      platform: 'web',
      token: 'qa-fcm-delivery-${b.activeProfileId}',
    );
    if (registration case NotificationsApiFailure(:final statusCode)) {
      fail('synthetic FCM device registration failed with HTTP ${statusCode ?? 0}');
    }

    final recorderEndpoint = Uri.parse('${notificationDebugBase()}/debug/recorded-pushes');
    final routeProbe = await http.get(recorderEndpoint);
    expect(
      routeProbe.statusCode,
      400,
      reason: 'expected enabled debug recorder route (HTTP ${routeProbe.statusCode})',
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

    final uri = recorderEndpoint.replace(
      queryParameters: {'profile_id': b.activeProfileId},
    );
    RecordedPush? recorded;
    int? lastStatusCode;
    for (var i = 0; i < 20; i++) {
      await Future<void>.delayed(const Duration(milliseconds: 500));
      final resp = await http.get(uri);
      lastStatusCode = resp.statusCode;
      if (resp.statusCode == 200) {
        final map = jsonDecode(resp.body) as Map<String, dynamic>;
        recorded = RecordedPush.fromJson(map);
        break;
      }
    }
    if (recorded == null && _diagnosticFile.isNotEmpty) {
      try {
        final sentMessage = (send as MessagesApiOk<VoiceMessage>).data;
        await File(_diagnosticFile).writeAsString(
          jsonEncode({
            'message_id': sentMessage.id,
            'chat_id': chatId,
            'sender_profile_id': a.activeProfileId,
          }),
          flush: true,
        );
      } on Object {
        // Correlation is best-effort; the original assertion remains authoritative.
      }
    }
    expect(
      recorded,
      isNotNull,
      reason: 'no recorded push; final recorder HTTP status was $lastStatusCode',
    );
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
