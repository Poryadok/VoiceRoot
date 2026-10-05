import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/notification_settings_models.dart';
import 'package:voice_frontend/backend/notifications_client.dart';

import 'support/live_gateway_harness.dart';

String notificationDebugBase() {
  const fromEnv = String.fromEnvironment('VOICE_NOTIFICATION_DEBUG_URL');
  if (fromEnv.isNotEmpty) return fromEnv;
  return 'http://127.0.0.1:18091';
}

const _diagnosticFile = String.fromEnvironment('VOICE_FCM_DIAGNOSTIC_FILE');
const _diagnosticStatusFile = String.fromEnvironment(
  'VOICE_FCM_DIAGNOSTIC_STATUS_FILE',
);

Future<void> _writeDiagnosticStatus(String field, String value) async {
  if (_diagnosticStatusFile.isEmpty) return;
  if ((field != 'pre' && field != 'post') ||
      (value != 'ok' && value != 'failed')) {
    fail('FCM diagnostic status write failed');
  }
  final file = File(_diagnosticStatusFile);
  try {
    final current = await file.readAsString();
    final match = RegExp(
      r'^pre=(unknown|ok|failed)\npost=(unknown|ok|failed)\n$',
    ).firstMatch(current);
    if (match == null) throw const FormatException();
    if ((field == 'pre' && (match[1] != 'unknown' || match[2] != 'unknown')) ||
        (field == 'post' && (match[1] != 'ok' || match[2] != 'unknown'))) {
      throw const FormatException();
    }
    final pre = field == 'pre' ? value : match[1]!;
    final post = field == 'post' ? value : match[2]!;
    await file.writeAsString('pre=$pre\npost=$post\n', flush: true);
  } on Object {
    fail('FCM diagnostic status write failed');
  }
}

Future<void> _writeFcmDiagnosticControl({
  required String chatId,
  required String senderProfileId,
  required String recipientProfileId,
  String messageId = '',
}) async {
  if (_diagnosticFile.isEmpty) return;
  try {
    await File(_diagnosticFile).writeAsString(
      jsonEncode({
        'message_id': messageId,
        'chat_id': chatId,
        'sender_profile_id': senderProfileId,
        'recipient_profile_id': recipientProfileId,
      }),
      flush: true,
    );
  } on Object {
    if (_diagnosticStatusFile.isNotEmpty) {
      await _writeDiagnosticStatus(
        messageId.isEmpty ? 'pre' : 'post',
        'failed',
      );
      fail('FCM diagnostic control write failed');
    }
  }
  if (_diagnosticStatusFile.isNotEmpty) {
    await _writeDiagnosticStatus(messageId.isEmpty ? 'pre' : 'post', 'ok');
  }
}

Future<T?> _bestEffortDiagnosticRead<T>(Future<T> Function() read) async {
  try {
    return await read().timeout(const Duration(seconds: 2));
  } on Object {
    return null;
  }
}

String _diagnosticHttpStatus(int? statusCode) =>
    statusCode == null ? 'unavailable' : 'http_$statusCode';

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

      await _writeFcmDiagnosticControl(
        chatId: chatId,
        senderProfileId: a.activeProfileId,
        recipientProfileId: b.activeProfileId,
      );

      final send = await ctx.messagesClient().sendMessage(
      authorization: a.authorizationHeader,
      chatId: chatId,
      content: 'fcm delivery probe ${DateTime.now().millisecondsSinceEpoch}',
      clientMessageId: qaClientMessageId(),
      );
      expect(send, isA<MessagesApiOk<VoiceMessage>>());
      final sentMessage = (send as MessagesApiOk<VoiceMessage>).data;
      await _writeFcmDiagnosticControl(
        chatId: chatId,
        senderProfileId: a.activeProfileId,
        recipientProfileId: b.activeProfileId,
        messageId: sentMessage.id,
      );

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
    var failureDiagnostics = '';
    if (recorded == null && _diagnosticFile.isNotEmpty) {
      final reads = await Future.wait<Object?>([
        _bestEffortDiagnosticRead(
          () => ctx.chatsClient().listGroupMembers(
            authorization: a.authorizationHeader,
            chatId: chatId,
            pageSize: 500,
          ),
        ),
        _bestEffortDiagnosticRead(
          () => notifications.getSettings(authorization: b.authorizationHeader),
        ),
        _bestEffortDiagnosticRead(
          () => notifications.getSettings(
            authorization: b.authorizationHeader,
            scopeType: 'chat',
            scopeId: chatId,
          ),
        ),
        _bestEffortDiagnosticRead(
          () => notifications.getQuietHours(authorization: b.authorizationHeader),
        ),
      ]);

      final memberResult = reads[0];
      final memberData = memberResult is ChatsApiOk<MemberListData>
          ? memberResult.data
          : null;
      final membersStatus = memberData != null
          ? 'ok'
          : memberResult is ChatsApiFailure
              ? _diagnosticHttpStatus(memberResult.statusCode)
              : 'unavailable';
      final recipientPresent = memberData == null
          ? 'unknown'
          : memberData.members.any((member) => member.profileId == b.activeProfileId).toString();
      final memberCount = memberData?.members.length.toString() ?? 'unknown';

      final globalResult = reads[1];
      final globalSettings = globalResult is NotificationsApiOk<VoiceNotificationSettings>
          ? globalResult.data
          : null;
      final globalStatus = globalSettings == null
          ? globalResult is NotificationsApiFailure
              ? _diagnosticHttpStatus(globalResult.statusCode)
              : 'unavailable'
          : globalSettings.profileId == b.activeProfileId &&
                  globalSettings.scopeType == 'global' &&
                  globalSettings.scopeId == null
              ? 'ok'
              : 'scope_mismatch';
      final globalScopeMatches = globalStatus == 'ok';

      final chatResult = reads[2];
      final chatSettings = chatResult is NotificationsApiOk<VoiceNotificationSettings>
          ? chatResult.data
          : null;
      final chatStatus = chatSettings == null
          ? chatResult is NotificationsApiFailure
              ? _diagnosticHttpStatus(chatResult.statusCode)
              : 'unavailable'
          : chatSettings.profileId == b.activeProfileId &&
                  chatSettings.scopeType == 'chat' &&
                  chatSettings.scopeId == chatId
              ? 'ok'
              : 'scope_mismatch';
      final chatScopeMatches = chatStatus == 'ok';

      final quietResult = reads[3];
      final quietHours = quietResult is NotificationsApiOk<VoiceQuietHours>
          ? quietResult.data
          : null;
      final quietStatus = quietHours == null
          ? quietResult is NotificationsApiFailure
              ? _diagnosticHttpStatus(quietResult.statusCode)
              : 'unavailable'
          : 'ok';
      final globalEnabled = globalScopeMatches ? globalSettings!.enabled.toString() : 'unknown';
      final globalSuppressesNewMessage = globalScopeMatches
          ? globalSettings!.suppressedTypes.contains(NotificationEventTypes.newMessage).toString()
          : 'unknown';
      final globalSuppressesMessageRequest = globalScopeMatches
          ? globalSettings!.suppressedTypes.contains(NotificationEventTypes.messageRequest).toString()
          : 'unknown';
      final globalMuteUntilPresent = globalScopeMatches
          ? (globalSettings!.muteUntil != null).toString()
          : 'unknown';
      final chatEnabled = chatScopeMatches ? chatSettings!.enabled.toString() : 'unknown';
      final chatSuppressesNewMessage = chatScopeMatches
          ? chatSettings!.suppressedTypes.contains(NotificationEventTypes.newMessage).toString()
          : 'unknown';
      final chatSuppressesMessageRequest = chatScopeMatches
          ? chatSettings!.suppressedTypes.contains(NotificationEventTypes.messageRequest).toString()
          : 'unknown';
      final chatMuteUntilPresent = chatScopeMatches
          ? (chatSettings!.muteUntil != null).toString()
          : 'unknown';
      final quietHoursEnabled = quietStatus == 'ok' ? quietHours!.enabled.toString() : 'unknown';

      failureDiagnostics =
          '; members_api=$membersStatus member_count_returned=$memberCount recipient_present=$recipientPresent'
          ' global_settings_api=$globalStatus global_enabled=$globalEnabled'
          ' global_suppresses_new_message=$globalSuppressesNewMessage'
          ' global_suppresses_message_request=$globalSuppressesMessageRequest'
          ' global_mute_until_present=$globalMuteUntilPresent'
          ' chat_settings_api=$chatStatus chat_enabled=$chatEnabled'
          ' chat_suppresses_new_message=$chatSuppressesNewMessage'
          ' chat_suppresses_message_request=$chatSuppressesMessageRequest'
          ' chat_mute_until_present=$chatMuteUntilPresent'
          ' quiet_hours_api=$quietStatus quiet_hours_enabled=$quietHoursEnabled';
    }
    expect(
      recorded,
      isNotNull,
      reason: 'no recorded push; final recorder HTTP status was $lastStatusCode$failureDiagnostics',
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
