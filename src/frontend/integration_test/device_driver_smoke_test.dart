import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:integration_test/integration_test.dart';
import 'package:voice_frontend/app.dart';
import 'package:voice_frontend/backend/notifications_client.dart';
import 'package:voice_frontend/routing/deep_link_parser.dart';
import 'package:voice_frontend/shell/three_column_shell.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';
import 'package:voice_frontend/state/deep_link_navigation.dart';
import 'package:voice_frontend/state/push_notifications.dart';
import 'package:voice_frontend/state/push_notifications_bootstrap.dart';
import 'package:voice_frontend/state/shared_media_providers.dart';
import 'package:voice_frontend/state/voip_push_platform.dart';

import '../test/support/auth_test_overrides.dart';
import '../test/support/gateway_test_client.dart';
import '../test/support/voice_test_theme.dart';

/// P3.1 device-driver scaffold (inventory PL-04 / DL-04).
///
/// Runs under [IntegrationTestWidgetsFlutterBinding] on the host
/// `flutter_tester` (CI job `flutter-device-driver`). Covers:
/// - deep-link → conversation navigation
/// - push / VoIP token register contracts via Gateway mock
///
/// Still out of CI (honest gaps):
/// - NT-05 real-device alert delivery (staging secrets)
/// - DL-04 mobile App Links / AASA on-device acceptance
/// - CallKit / PushKit / LiveKit media paths on a physical device
void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  group('device driver — deep link', () {
    testWidgets('chat message deep link opens conversation region', (
      tester,
    ) async {
      await bindDesktopTestViewport(tester);

      late ProviderContainer container;

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container = ProviderContainer(
            overrides: [
              ...guestShellTestOverrides(
                client: MockClient((_) async => throw UnimplementedError()),
              ),
              // flutter-tester has no connectivity_plus plugin; keep offline=false.
              connectivityWatcherProvider.overrideWith((ref) {}),
            ],
          ),
          child: const VoiceApp(locale: Locale('en')),
        ),
      );
      await _pumpShellReady(tester);

      await container.read(deepLinkNavigatorProvider).apply(
        parseDeepLinkUrl(
          'https://voice.gg/ch/device-driver-chat/m/device-driver-msg',
        ),
      );
      await _pumpShellReady(tester);

      expect(container.read(selectedChatIdProvider), 'device-driver-chat');
      expect(
        container.read(
          pendingChatMessageScrollProvider('device-driver-chat'),
        ),
        'device-driver-msg',
      );
      expect(find.byKey(ThreeColumnShell.navOpenChat), findsOneWidget);
      expect(find.bySemanticsLabel('Conversation'), findsOneWidget);
    });
  });

  group('device driver — push register contract', () {
    testWidgets('FCM bootstrap posts register-device to Gateway', (
      tester,
    ) async {
      String? capturedPath;
      Map<String, dynamic>? capturedBody;

      final client = VoiceNotificationsClient(
        gateway: gatewayHttpForTest(
          MockClient((request) async {
            capturedPath = request.url.path;
            capturedBody = jsonDecode(request.body) as Map<String, dynamic>;
            return http.Response('{"device_token_id":"tok-1"}', 200);
          }),
        ),
      );

      const bootstrap = PushNotificationsBootstrap();
      final id = await bootstrap.registerToken(
        client: client,
        authorization: 'Bearer device-driver-jwt',
        platform: 'android',
        token: 'fcm-device-driver-token',
      );

      expect(id, 'tok-1');
      expect(capturedPath, '/api/v1/notifications/register-device');
      expect(capturedBody?['platform'], 'android');
      expect(capturedBody?['token'], 'fcm-device-driver-token');
      expect(capturedBody?['push_service'], 'fcm');
    });

    testWidgets('FCM data maps to realtime notification frame', (tester) async {
      final frame = fcmDataToRealtimeNotification({
        'type': 'new_message',
        'chat_id': 'device-driver-chat',
        'message_id': 'device-driver-msg',
        'sender_profile_id': 'peer-1',
      });
      expect(frame, isNotNull);
      expect(frame!.op, 'notification');
      expect(frame.data?['type'], 'new_message');
      expect(frame.data?['chat_id'], 'device-driver-chat');
    });
  });

  group('device driver — VoIP register contract', () {
    testWidgets('VoIP PushKit service is unavailable on host tester', (
      tester,
    ) async {
      // Physical iOS + PushKit required for isVoIPPushSupported == true.
      expect(voipPushServiceForTarget(), isNull);
      expect(isVoIPPushSupported, isFalse);
    });

    testWidgets('voip_apns bootstrap posts register-device to Gateway', (
      tester,
    ) async {
      Map<String, dynamic>? capturedBody;

      final client = VoiceNotificationsClient(
        gateway: gatewayHttpForTest(
          MockClient((request) async {
            capturedBody = jsonDecode(request.body) as Map<String, dynamic>;
            return http.Response('{}', 200);
          }),
        ),
      );

      const bootstrap = PushNotificationsBootstrap();
      await bootstrap.registerToken(
        client: client,
        authorization: 'Bearer device-driver-jwt',
        platform: 'ios',
        token: 'voip-device-driver-token',
        pushService: 'voip_apns',
      );

      expect(capturedBody?['platform'], 'ios');
      expect(capturedBody?['token'], 'voip-device-driver-token');
      expect(capturedBody?['push_service'], 'voip_apns');
    });
  });
}

Future<void> _pumpShellReady(WidgetTester tester) async {
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 100));
  await tester.pump(const Duration(milliseconds: 400));
}
