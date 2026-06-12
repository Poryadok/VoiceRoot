import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/notifications_client.dart';
import 'package:voice_frontend/state/push_notifications.dart';
import 'package:voice_frontend/state/push_notifications_bootstrap.dart';
import 'package:voice_frontend/state/push_platform.dart';

import 'support/gateway_test_client.dart';

void main() {
  group('VoiceNotificationsClient', () {
    test('unregisterDevice posts to gateway unregister-device endpoint', () async {
      String? capturedPath;
      Map<String, dynamic>? capturedBody;

      final client = VoiceNotificationsClient(
        gateway: gatewayHttpForTest(
          MockClient((request) async {
            capturedPath = request.url.path;
            capturedBody = jsonDecode(request.body) as Map<String, dynamic>;
            return http.Response('{}', 200);
          }),
        ),
      );

      final result = await client.unregisterDevice(
        authorization: 'Bearer test-jwt',
        deviceTokenId: 'device-token-uuid',
      );

      expect(result, isA<NotificationsApiOk<void>>());
      expect(capturedPath, '/api/v1/notifications/unregister-device');
      expect(capturedBody?['device_token_id'], 'device-token-uuid');
    });

    test('registerDevice posts to gateway register-device endpoint', () async {
      String? capturedPath;
      Map<String, dynamic>? capturedBody;

      final client = VoiceNotificationsClient(
        gateway: gatewayHttpForTest(
          MockClient((request) async {
            capturedPath = request.url.path;
            capturedBody = jsonDecode(request.body) as Map<String, dynamic>;
            return http.Response('{}', 200);
          }),
        ),
      );

      final result = await client.registerDevice(
        authorization: 'Bearer test-jwt',
        platform: 'web',
        token: 'fcm-token-abc',
      );

      expect(result, isA<NotificationsApiOk<void>>());
      expect(capturedPath, '/api/v1/notifications/register-device');
      expect(capturedBody?['platform'], 'web');
      expect(capturedBody?['token'], 'fcm-token-abc');
      expect(capturedBody?['push_service'], 'fcm');
    });

    test('registerDevice sends apns push_service when requested', () async {
      Map<String, dynamic>? capturedBody;

      final client = VoiceNotificationsClient(
        gateway: gatewayHttpForTest(
          MockClient((request) async {
            capturedBody = jsonDecode(request.body) as Map<String, dynamic>;
            return http.Response('{}', 200);
          }),
        ),
      );

      final result = await client.registerDevice(
        authorization: 'Bearer test-jwt',
        platform: 'ios',
        token: 'apns-token-xyz',
        pushService: 'apns',
      );

      expect(result, isA<NotificationsApiOk<void>>());
      expect(capturedBody?['platform'], 'ios');
      expect(capturedBody?['token'], 'apns-token-xyz');
      expect(capturedBody?['push_service'], 'apns');
    });

    test('registerDevice sends voip_apns push_service when requested', () async {
      Map<String, dynamic>? capturedBody;

      final client = VoiceNotificationsClient(
        gateway: gatewayHttpForTest(
          MockClient((request) async {
            capturedBody = jsonDecode(request.body) as Map<String, dynamic>;
            return http.Response('{}', 200);
          }),
        ),
      );

      final result = await client.registerDevice(
        authorization: 'Bearer test-jwt',
        platform: 'ios',
        token: 'voip-token-xyz',
        pushService: 'voip_apns',
      );

      expect(result, isA<NotificationsApiOk<void>>());
      expect(capturedBody?['push_service'], 'voip_apns');
    });
  });

  group('pushServiceForTarget', () {
    test('defaults to fcm on test VM', () {
      expect(pushServiceForTarget(), 'fcm');
    });
  });

  group('PushNotificationsBootstrap', () {
    test('registerToken posts FCM token to gateway after auth', () async {
      var posted = false;
      final client = VoiceNotificationsClient(
        gateway: gatewayHttpForTest(
          MockClient((request) async {
            posted = true;
            return http.Response('{}', 200);
          }),
        ),
      );
      const bootstrap = PushNotificationsBootstrap();
      await bootstrap.registerToken(
        client: client,
        authorization: 'Bearer jwt',
        platform: 'web',
        token: 'fcm-bootstrap-token',
      );
      expect(posted, isTrue);
    });
  });

  group('fcmDataToRealtimeNotification', () {
    test('maps new_message FCM data to notification WS frame', () {
      final frame = fcmDataToRealtimeNotification({
        'type': 'new_message',
        'chat_id': 'chat-1',
        'message_id': 'msg-1',
        'sender_profile_id': 'peer-1',
      });
      expect(frame, isNotNull);
      expect(frame!.op, 'notification');
      expect(frame.data?['type'], 'new_message');
      expect(frame.data?['chat_id'], 'chat-1');
      expect(frame.data?['message_id'], 'msg-1');
      expect(frame.data?['sender_profile_id'], 'peer-1');
    });

    test('maps mention FCM data to notification WS frame', () {
      final frame = fcmDataToRealtimeNotification({
        'type': 'mention',
        'chat_id': 'chat-2',
        'message_id': 'msg-2',
        'sender_profile_id': 'peer-2',
      });
      expect(frame, isNotNull);
      expect(frame!.data?['type'], 'mention');
    });

    test('maps match_found FCM data', () {
      final frame = fcmDataToRealtimeNotification({
        'type': 'match_found',
        'match_id': 'match-99',
      });
      expect(frame, isNotNull);
      expect(frame!.data?['type'], 'match_found');
      expect(frame.data?['match_id'], 'match-99');
    });
  });
}
