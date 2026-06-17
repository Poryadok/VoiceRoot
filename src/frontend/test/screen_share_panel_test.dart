import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/voice_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/call_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/screen_share_providers.dart';
import 'package:voice_frontend/ui/call/screen_share_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets('dual screen share shows both streams with local self preview', (
    tester,
  ) async {
    const selfProfileId = 'prof-self';
    const peerProfileId = 'prof-peer';
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 200)),
        ),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        screenShareControllerProvider.overrideWith((ref) {
          final controller = ScreenShareController(ref);
          controller.state = const ScreenShareUiState(
            streams: [
              ActiveScreenShare(
                roomId: 'room-1',
                profileId: selfProfileId,
                streamId: 'stream-self',
              ),
              ActiveScreenShare(
                roomId: 'room-1',
                profileId: peerProfileId,
                streamId: 'stream-peer',
              ),
            ],
            selectedProfileId: selfProfileId,
            localStreamId: 'stream-self',
            isSharing: true,
          );
          return controller;
        }),
      ],
    );
    addTearDown(container.dispose);

    container.read(callControllerProvider.notifier).state = const CallState(
      phase: CallPhase.active,
      session: VoiceCallSession(
        roomId: 'room-1',
        livekitRoomName: 'lk-room',
        chatId: 'chat-1',
        initiatorProfileId: selfProfileId,
        calleeProfileId: peerProfileId,
        mediaKind: VoiceCallMediaKind.video,
        status: VoiceCallStatus.active,
      ),
    );

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: ScreenSharePanel()),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(ScreenSharePanel.streamPickerKey), findsOneWidget);
    expect(find.byKey(const Key('screen_share_local_preview')), findsOneWidget);
    expect(find.textContaining('Waiting for video'), findsNothing);
    expect(
      find.byKey(Key('screen_share_remote_$peerProfileId')),
      findsOneWidget,
    );

    await tester.tap(find.text('prof-pee'));
    await tester.pump();
    expect(find.byKey(const Key('screen_share_local_preview')), findsNothing);
  });

  testWidgets('screen share shows waiting text and limit error', (tester) async {
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 200)),
        ),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        screenShareControllerProvider.overrideWith((ref) {
          final controller = ScreenShareController(ref);
          controller.state = const ScreenShareUiState(
            streams: [
              ActiveScreenShare(
                roomId: 'room-1',
                profileId: 'prof-peer',
                streamId: 'stream-peer',
              ),
            ],
            errorMessage: 'screen_share_limit',
          );
          return controller;
        }),
      ],
    );
    addTearDown(container.dispose);

    container.read(callControllerProvider.notifier).state = const CallState(
      phase: CallPhase.active,
      session: VoiceCallSession(
        roomId: 'room-1',
        livekitRoomName: 'lk-room',
        chatId: 'chat-1',
        initiatorProfileId: 'prof-self',
        calleeProfileId: 'prof-peer',
        mediaKind: VoiceCallMediaKind.video,
        status: VoiceCallStatus.active,
      ),
    );

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: ScreenSharePanel()),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.textContaining('Waiting for screen video'), findsOneWidget);
    expect(find.textContaining('3 screen shares'), findsOneWidget);
  });

  testWidgets('screen share pause button toggles pause state', (tester) async {
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 200)),
        ),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        screenShareControllerProvider.overrideWith((ref) {
          final controller = ScreenShareController(ref);
          controller.state = const ScreenShareUiState(
            streams: [
              ActiveScreenShare(
                roomId: 'room-1',
                profileId: 'prof-self',
                streamId: 'stream-self',
              ),
            ],
            selectedProfileId: 'prof-self',
            localStreamId: 'stream-self',
            isSharing: true,
            isPaused: true,
          );
          return controller;
        }),
      ],
    );
    addTearDown(container.dispose);

    container.read(callControllerProvider.notifier).state = const CallState(
      phase: CallPhase.active,
      session: VoiceCallSession(
        roomId: 'room-1',
        livekitRoomName: 'lk-room',
        chatId: 'chat-1',
        initiatorProfileId: 'prof-self',
        calleeProfileId: 'prof-peer',
        mediaKind: VoiceCallMediaKind.video,
        status: VoiceCallStatus.active,
      ),
    );

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: Row(children: [ScreenSharePauseButton()]),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byIcon(Icons.play_arrow), findsOneWidget);
    await tester.tap(find.byIcon(Icons.play_arrow));
    await tester.pump();
  });

  testWidgets('screen share stop button and generic error message', (tester) async {
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 200)),
        ),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        screenShareControllerProvider.overrideWith((ref) {
          final controller = ScreenShareController(ref);
          controller.state = const ScreenShareUiState(
            isSharing: true,
            localStreamId: 'stream-self',
            errorMessage: 'upload_failed',
          );
          return controller;
        }),
      ],
    );
    addTearDown(container.dispose);

    container.read(callControllerProvider.notifier).state = const CallState(
      phase: CallPhase.active,
      session: VoiceCallSession(
        roomId: 'room-1',
        livekitRoomName: 'lk-room',
        chatId: 'chat-1',
        initiatorProfileId: 'prof-self',
        calleeProfileId: 'prof-peer',
        mediaKind: VoiceCallMediaKind.audio,
        status: VoiceCallStatus.active,
      ),
    );

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: Column(
              children: [
                ScreenSharePanel(),
                ScreenShareCallButton(),
              ],
            ),
          ),
        ),
      ),
    );
    await tester.pump();

    expect(find.text('upload_failed'), findsOneWidget);
    await tester.tap(find.byKey(ScreenSharePanel.shareButtonKey));
    await tester.pump();
  });

  testWidgets('screen share quality dialog returns selected fps', (tester) async {
    double? selectedFps;
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Builder(
          builder: (context) => Scaffold(
            body: FilledButton(
              onPressed: () async {
                selectedFps = await showScreenShareQualityDialog(context);
              },
              child: const Text('open-quality'),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('open-quality'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('720p · 15 FPS'));
    await tester.pumpAndSettle();

    expect(selectedFps, 15.0);
  });
}
