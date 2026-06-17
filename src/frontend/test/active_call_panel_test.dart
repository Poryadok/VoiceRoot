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
import 'package:voice_frontend/ui/call/active_call_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets('ActiveCallPanel shows mute and hangup when call active', (
    tester,
  ) async {
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(client: MockClient((_) async => http.Response('{}', 200))),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://127.0.0.1:18080'),
        ),
      ],
    );
    addTearDown(container.dispose);

    container.read(callControllerProvider.notifier).state = CallState(
      phase: CallPhase.active,
      session: const VoiceCallSession(
        roomId: 'room-1',
        livekitRoomName: 'lk-room',
        chatId: 'chat-1',
        initiatorProfileId: 'me',
        calleeProfileId: 'peer',
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
          home: const Scaffold(body: ActiveCallPanel()),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(ActiveCallPanel.panelKey), findsOneWidget);
    expect(find.byKey(ActiveCallPanel.muteKey), findsOneWidget);
    expect(find.byKey(ActiveCallPanel.hangupKey), findsOneWidget);
  });

  testWidgets('ActiveCallPanel video call uses fullscreen layout with minimize bar', (
    tester,
  ) async {
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(client: MockClient((_) async => http.Response('{}', 200))),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://127.0.0.1:18080'),
        ),
      ],
    );
    addTearDown(container.dispose);

    container.read(callControllerProvider.notifier).state = const CallState(
      phase: CallPhase.active,
      session: VoiceCallSession(
        roomId: 'room-1',
        livekitRoomName: 'lk-room',
        chatId: 'chat-1',
        initiatorProfileId: 'me',
        calleeProfileId: 'peer',
        mediaKind: VoiceCallMediaKind.video,
        status: VoiceCallStatus.active,
      ),
      isVideoEnabled: true,
    );

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: ActiveCallPanel()),
        ),
      ),
    );
    await tester.pumpAndSettle();

    final panelBox = tester.getSize(find.byKey(ActiveCallPanel.panelKey));
    final screenHeight = tester.view.physicalSize.height / tester.view.devicePixelRatio;
    expect(panelBox.height, greaterThan(screenHeight * 0.8));

    expect(find.byKey(const Key('active_call_minimize_bar')), findsOneWidget);
    expect(find.byKey(const Key('active_call_minimize')), findsOneWidget);

    await tester.tap(find.byKey(const Key('active_call_minimize')));
    await tester.pumpAndSettle();

    final minimizedBar = tester.getSize(find.byKey(const Key('active_call_minimize_bar')));
    expect(minimizedBar.height, lessThan(80));
    expect(find.byKey(ActiveCallPanel.videoPlaceholderKey), findsNothing);
  });

  testWidgets('ActiveCallPanel audio call shows unlock banner and control toggles', (
    tester,
  ) async {
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(client: MockClient((_) async => http.Response('{}', 200))),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://127.0.0.1:18080'),
        ),
      ],
    );
    addTearDown(container.dispose);

    container.read(callControllerProvider.notifier).state = const CallState(
      phase: CallPhase.active,
      session: VoiceCallSession(
        roomId: 'room-1',
        livekitRoomName: 'lk-room',
        chatId: 'chat-1',
        initiatorProfileId: 'me',
        calleeProfileId: 'peer',
        mediaKind: VoiceCallMediaKind.audio,
        status: VoiceCallStatus.active,
        sessionKind: VoiceSessionKind.groupVoice,
      ),
      needsAudioPlaybackUnlock: true,
      isMuted: true,
      isSpeakerMuted: true,
    );

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: ActiveCallPanel()),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(ActiveCallPanel.unlockAudioKey), findsOneWidget);
    expect(find.byKey(ActiveCallPanel.speakerKey), findsOneWidget);
    expect(find.byKey(ActiveCallPanel.videoKey), findsOneWidget);

    await tester.tap(find.byKey(ActiveCallPanel.muteKey));
    await tester.tap(find.byKey(ActiveCallPanel.speakerKey));
    await tester.tap(find.byKey(ActiveCallPanel.videoKey));
    await tester.tap(find.byKey(ActiveCallPanel.unlockAudioKey));
    await tester.pump();
  });

  testWidgets('ActiveCallPanel connecting shows spinner label', (tester) async {
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(client: MockClient((_) async => http.Response('{}', 200))),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://127.0.0.1:18080'),
        ),
      ],
    );
    addTearDown(container.dispose);

    container.read(callControllerProvider.notifier).state = const CallState(
      phase: CallPhase.connecting,
      session: VoiceCallSession(
        roomId: 'room-1',
        livekitRoomName: 'lk-room',
        chatId: 'chat-1',
        initiatorProfileId: 'me',
        calleeProfileId: 'peer',
        mediaKind: VoiceCallMediaKind.audio,
        status: VoiceCallStatus.ringing,
      ),
    );

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: ActiveCallPanel()),
        ),
      ),
    );
    await tester.pump();

    expect(find.byType(CircularProgressIndicator), findsOneWidget);
    expect(find.byKey(ActiveCallPanel.panelKey), findsOneWidget);
  });
}
