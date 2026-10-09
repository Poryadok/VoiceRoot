import 'dart:convert';

import 'package:flutter/foundation.dart'
    show debugDefaultTargetPlatformOverride;
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/app.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';

import 'support/auth_test_overrides.dart';

void main() {
  testWidgets('hides gateway status bar when /health returns 200', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: voiceAppTestOverrides(
          client: MockClient((request) async {
            if (request.url.path == '/health') {
              return http.Response('OK', 200);
            }
            return http.Response('Not Found', 404);
          }),
        ),
        child: const VoiceApp(locale: Locale('en')),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.textContaining('Gateway: ok'), findsNothing);
    expect(find.byKey(const Key('gateway_status_text')), findsNothing);
    expect(find.byKey(const Key('settings_open')), findsOneWidget);
  });

  testWidgets('shell exposes network banner when device is offline', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => http.Response('OK', 200)),
          ),
          connectivityWatcherProvider.overrideWith((ref) {}),
          isDeviceOfflineProvider.overrideWith((ref) => true),
          realtimeLinkStatusProvider.overrideWith(
            (ref) => RealtimeLinkStatus.disconnected,
          ),
        ],
        child: const VoiceApp(locale: Locale('en')),
      ),
    );
    await tester.pumpAndSettle();

    expect(
      find.byKey(const Key('global_reconnect_banner')),
      findsOneWidget,
      reason: 'the shell should show offline status before Realtime connects',
    );
    final banner = find.byKey(const Key('global_reconnect_banner'));
    expect(
      find.descendant(
        of: banner,
        matching: find.byIcon(Icons.cloud_off_outlined),
      ),
      findsOneWidget,
      reason: 'device-offline status stays a static offline mark',
    );
    expect(
      find.descendant(
        of: banner,
        matching: find.byKey(const Key('network_reconnecting_progress')),
      ),
      findsNothing,
      reason: 'device-offline status must not imply a reconnect spinner',
    );
    final offlineMark = find.descendant(
      of: banner,
      matching: find.byKey(const Key('network_offline_mark')),
    );
    expect(offlineMark, findsOneWidget);
    expect(tester.getSize(offlineMark), const Size(36, 36));
  });

  testWidgets('shell banner can be dismissed and returns on a new failure', (
    tester,
  ) async {
    final previousTargetPlatform = debugDefaultTargetPlatformOverride;
    debugDefaultTargetPlatformOverride = TargetPlatform.windows;
    try {
      const voipChannel = MethodChannel('voice/voip');
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(voipChannel, (_) async => null);
      addTearDown(
        () => TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
            .setMockMethodCallHandler(voipChannel, null),
      );
      var retryCalls = 0;
      final container = ProviderContainer(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => http.Response('OK', 200)),
          ),
          connectivityWatcherProvider.overrideWith((ref) {}),
          realtimeHubProvider.overrideWith(
            (ref) =>
                _RetryRecordingRealtimeHub(ref, onRetry: () => retryCalls++),
          ),
        ],
      );
      addTearDown(container.dispose);
      container.read(realtimeLinkStatusProvider.notifier).state =
          RealtimeLinkStatus.connected;

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: const VoiceApp(locale: Locale('en')),
        ),
      );
      await tester.pump();
      container.read(realtimeLinkStatusProvider.notifier).state =
          RealtimeLinkStatus.reconnecting;
      await tester.pump(reconnectBannerShowDelay);

      expect(find.byKey(const Key('global_reconnect_banner')), findsOneWidget);
      expect(
        find.text(
          'Drafts stay on this device. Realtime updates resume after reconnection.',
        ),
        findsOneWidget,
      );
      final banner = find.byKey(const Key('global_reconnect_banner'));
      final retryButton = find.descendant(
        of: banner,
        matching: find.widgetWithText(OutlinedButton, 'Try again'),
      );
      expect(retryButton, findsOneWidget);
      expect(
        tester.getSemantics(retryButton).flagsCollection.isButton,
        isTrue,
      );
      bool retryOwnsPrimaryFocus() {
        final focusContext = FocusManager.instance.primaryFocus?.context;
        if (focusContext == null) return false;
        return find
            .descendant(
              of: retryButton,
              matching: find.byWidget(focusContext.widget),
            )
            .evaluate()
            .isNotEmpty;
      }

      var tabCount = 0;
      while (!retryOwnsPrimaryFocus() && tabCount < 40) {
        await tester.sendKeyEvent(LogicalKeyboardKey.tab);
        await tester.pump();
        tabCount++;
      }
      expect(
        retryOwnsPrimaryFocus(),
        isTrue,
        reason:
            'bounded Tab traversal should focus the Retry TextButton itself',
      );
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      await tester.pump();
      final retryCallsAfterEnter = retryCalls;
      await tester.tap(retryButton);
      await tester.pump();
      expect(retryCalls, retryCallsAfterEnter + 1);
      expect(retryCallsAfterEnter, 1);
      final dismiss = find.descendant(
        of: banner,
        matching: find.byTooltip('Close'),
      );
      expect(dismiss, findsOneWidget);
      await tester.tap(dismiss);
      await tester.pump();
      expect(find.byKey(const Key('global_reconnect_banner')), findsNothing);

      container.read(realtimeLinkStatusProvider.notifier).state =
          RealtimeLinkStatus.connected;
      await tester.pump(reconnectBannerHideDelay);
      container.read(realtimeLinkStatusProvider.notifier).state =
          RealtimeLinkStatus.reconnecting;
      await tester.pump(reconnectBannerShowDelay);
      expect(find.byKey(const Key('global_reconnect_banner')), findsOneWidget);
    } finally {
      debugDefaultTargetPlatformOverride = previousTargetPlatform;
    }
  });

  testWidgets('shows failure when base URL missing', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => http.Response('x', 404)),
          ),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: ''),
          ),
        ],
        child: const VoiceApp(locale: Locale('en')),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.textContaining('missing base URL'), findsOneWidget);
  });

  testWidgets('session bar shows @handle when profile loads', (tester) async {
    const profileJson = {
      'id': 'prof-test',
      'account_id': 'acc-test',
      'username': 'voiceuser',
      'discriminator': '4242',
      'display_name': 'Voice User',
      'locale': 'en',
      'theme': 'dark',
      'is_primary': true,
      'verification_type': 'none',
    };
    await tester.pumpWidget(
      ProviderScope(
        overrides: voiceAppTestOverrides(
          client: MockClient((request) async {
            if (request.url.path == '/health') {
              return http.Response('OK', 200);
            }
            if (request.url.path == '/api/v1/users/profiles') {
              return http.Response(
                jsonEncode({
                  'profile_list': {
                    'profiles': [profileJson],
                  },
                }),
                200,
              );
            }
            if (request.url.path == '/api/v1/users/profiles/prof-test') {
              return http.Response(jsonEncode({'profile': profileJson}), 200);
            }
            return http.Response('Not Found', 404);
          }),
        ),
        child: const VoiceApp(locale: Locale('en')),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('@voiceuser#4242'), findsOneWidget);
    expect(find.textContaining('Profile:'), findsNothing);
  });

  testWidgets('chat list shows backend unavailable on 503', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((request) async {
              if (request.url.path == '/health') {
                return http.Response('OK', 200);
              }
              if (request.url.path == '/api/v1/chats') {
                return http.Response('unavailable', 503);
              }
              return http.Response('Not Found', 404);
            }),
          ),
          voiceChatsClientProvider.overrideWith(
            (ref) =>
                VoiceChatsClient(gateway: ref.watch(gatewayHttpClientProvider)),
          ),
        ],
        child: const VoiceApp(locale: Locale('en')),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('chat_list_unavailable')), findsOneWidget);
    expect(find.textContaining('Start the full API stack'), findsOneWidget);
    expect(find.text('No conversations yet'), findsNothing);
  });
}

class _RetryRecordingRealtimeHub extends RealtimeHub {
  _RetryRecordingRealtimeHub(super.ref, {required this.onRetry});

  final VoidCallback onRetry;

  @override
  Stream<RealtimeFrame> get events => const Stream.empty();

  @override
  bool get canRetryCurrentSession => true;

  @override
  Future<void> ensureConnected() async {}

  @override
  Future<void> retryCurrentSession() async => onRetry();

  @override
  void ensureSubscribed(String chatId) {}

  @override
  Future<void> dispose() async {}
}
