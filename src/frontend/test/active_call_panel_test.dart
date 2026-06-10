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
}
