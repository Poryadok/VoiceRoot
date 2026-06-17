import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/backend/voice_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/call_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/social_providers.dart';
import 'package:voice_frontend/ui/call/call_error_listener.dart';
import 'package:voice_frontend/ui/call/incoming_call_overlay.dart';
import 'package:voice_frontend/ui/call/outgoing_call_overlay.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

const _callerProfileId = 'caller-prof';
const _calleeProfileId = 'callee-prof';

void main() {
  testWidgets('IncomingCallOverlay shows accept and decline for incoming call', (
    tester,
  ) async {
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 200)),
        ),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://127.0.0.1:18080'),
        ),
        profileProvider(_callerProfileId).overrideWith(
          (ref) async => const VoiceProfile(
            id: _callerProfileId,
            accountId: 'acc-caller',
            username: 'caller',
            discriminator: '0001',
            displayName: 'Caller',
          ),
        ),
      ],
    );
    addTearDown(container.dispose);

    container.read(callControllerProvider.notifier).state = const CallState(
      phase: CallPhase.incoming,
      session: VoiceCallSession(
        roomId: 'room-1',
        livekitRoomName: 'lk-room',
        chatId: 'chat-1',
        initiatorProfileId: _callerProfileId,
        calleeProfileId: 'me',
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
          home: const Scaffold(
            body: Stack(children: [IncomingCallOverlay()]),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(IncomingCallOverlay.overlayKey), findsOneWidget);
    expect(find.byKey(IncomingCallOverlay.acceptKey), findsOneWidget);
    expect(find.byKey(IncomingCallOverlay.declineKey), findsOneWidget);
    expect(find.textContaining('Caller'), findsWidgets);
  });

  testWidgets('OutgoingCallOverlay shows cancel while dialing', (tester) async {
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 200)),
        ),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://127.0.0.1:18080'),
        ),
        profileProvider(_calleeProfileId).overrideWith(
          (ref) async => const VoiceProfile(
            id: _calleeProfileId,
            accountId: 'acc-callee',
            username: 'callee',
            discriminator: '0002',
            displayName: 'Callee',
          ),
        ),
      ],
    );
    addTearDown(container.dispose);

    container.read(callControllerProvider.notifier).state = const CallState(
      phase: CallPhase.outgoing,
      session: VoiceCallSession(
        roomId: 'room-2',
        livekitRoomName: 'lk-room-2',
        chatId: 'chat-2',
        initiatorProfileId: 'me',
        calleeProfileId: _calleeProfileId,
        mediaKind: VoiceCallMediaKind.video,
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
          home: const Scaffold(
            body: Stack(children: [OutgoingCallOverlay()]),
          ),
        ),
      ),
    );
    await tester.pump();

    expect(find.byKey(OutgoingCallOverlay.overlayKey), findsOneWidget);
    expect(find.byKey(OutgoingCallOverlay.cancelKey), findsOneWidget);
    expect(find.textContaining('Callee'), findsWidgets);
  });

  testWidgets('CallErrorListener shows snackbar on call failure', (tester) async {
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 200)),
        ),
      ],
    );
    addTearDown(container.dispose);

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(
            body: CallErrorListener(
              child: Consumer(
                builder: (context, ref, _) {
                  return FilledButton(
                    onPressed: () {
                      ref.read(callControllerProvider.notifier).state =
                          const CallState(
                        phase: CallPhase.failed,
                        errorMessage: 'livekit_connect_failed',
                      );
                    },
                    child: const Text('fail'),
                  );
                },
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('fail'));
    await tester.pump();
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('call_error_snackbar')), findsOneWidget);
  });
}
