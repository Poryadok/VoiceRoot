import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/app.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
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
  });

  testWidgets('shell banner can be dismissed and returns on a new failure', (
    tester,
  ) async {
    const voipChannel = MethodChannel('voice/voip');
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(voipChannel, (_) async => null);
    addTearDown(
      () => TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(voipChannel, null),
    );
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('OK', 200)),
        ),
        connectivityWatcherProvider.overrideWith((ref) {}),
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
    final retryButton = find.text('Try again');
    expect(retryButton, findsOneWidget);
    expect(
      tester.getSemantics(retryButton).hasFlag(SemanticsFlag.isButton),
      isTrue,
    );
    Focus.of(tester.element(retryButton)).requestFocus();
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.pump();
    expect(find.byTooltip('Close'), findsOneWidget);
    await tester.tap(find.byTooltip('Close'));
    await tester.pump();
    expect(find.byKey(const Key('global_reconnect_banner')), findsNothing);

    container.read(realtimeLinkStatusProvider.notifier).state =
        RealtimeLinkStatus.connected;
    await tester.pump(reconnectBannerHideDelay);
    container.read(realtimeLinkStatusProvider.notifier).state =
        RealtimeLinkStatus.reconnecting;
    await tester.pump(reconnectBannerShowDelay);
    expect(find.byKey(const Key('global_reconnect_banner')), findsOneWidget);
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
