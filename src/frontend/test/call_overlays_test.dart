import 'package:flutter/material.dart';
import 'package:flutter/foundation.dart'
    show TargetPlatform, debugDefaultTargetPlatformOverride, kIsWeb;
import 'package:flutter/services.dart';
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
import 'package:voice_frontend/ui/a11y/focus_trap.dart';
import 'package:voice_frontend/ui/call/call_error_listener.dart';
import 'package:voice_frontend/ui/call/incoming_call_overlay.dart';
import 'package:voice_frontend/ui/call/outgoing_call_overlay.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

const _callerProfileId = 'caller-prof';
const _calleeProfileId = 'callee-prof';

ProviderContainer _incomingContainer({
  required http.Client client,
  GatewayConfig config = const GatewayConfig(baseUrl: 'http://127.0.0.1:18080'),
  CallPhase phase = CallPhase.incoming,
  VoiceCallMediaKind mediaKind = VoiceCallMediaKind.audio,
  String callerName = 'Caller',
  VoiceCallSession? session,
}) {
  final container = ProviderContainer(
    overrides: [
      ...voiceAppTestOverrides(client: client),
      gatewayConfigProvider.overrideWithValue(config),
      profileProvider(_callerProfileId).overrideWith(
        (ref) async => VoiceProfile(
          id: _callerProfileId,
          accountId: 'acc-caller',
          username: 'caller',
          discriminator: '0001',
          displayName: callerName,
        ),
      ),
    ],
  );
  container.read(callControllerProvider.notifier).state = CallState(
    phase: phase,
    session:
        session ??
        VoiceCallSession(
          roomId: 'room-1',
          livekitRoomName: 'lk-room',
          chatId: 'chat-1',
          initiatorProfileId: _callerProfileId,
          calleeProfileId: 'me',
          mediaKind: mediaKind,
          status: VoiceCallStatus.ringing,
        ),
  );
  return container;
}

Future<void> _pumpIncomingOverlay(
  WidgetTester tester,
  ProviderContainer container,
) async {
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: MaterialApp(
        theme: voiceTestTheme().copyWith(
          textTheme: voiceTestTheme().textTheme.apply(fontFamily: 'Noto Sans'),
        ),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: const Scaffold(body: Stack(children: [IncomingCallOverlay()])),
      ),
    ),
  );
}

Future<void> _loadNotoSans() async {
  final materialIcons = FontLoader('MaterialIcons')
    ..addFont(rootBundle.load('fonts/MaterialIcons-Regular.otf'));
  await materialIcons.load();
  final loader = FontLoader('Noto Sans')
    ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
    ..addFont(rootBundle.load('assets/fonts/NotoSans-Medium.ttf'))
    ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'))
    ..addFont(rootBundle.load('assets/fonts/NotoSans-Bold.ttf'));
  await loader.load();
}

void main() {
  setUpAll(_loadNotoSans);

  testWidgets(
    'IncomingCallOverlay shows accept and decline for incoming call',
    (tester) async {
      final container = _incomingContainer(
        client: MockClient((_) async => http.Response('{}', 200)),
      );
      addTearDown(container.dispose);
      await _pumpIncomingOverlay(tester, container);
      await tester.pumpAndSettle();

      expect(find.byKey(IncomingCallOverlay.overlayKey), findsOneWidget);
      expect(find.byKey(IncomingCallOverlay.acceptKey), findsOneWidget);
      expect(find.byKey(IncomingCallOverlay.declineKey), findsOneWidget);
      expect(find.textContaining('Caller'), findsWidgets);
      expect(find.text('Audio call'), findsOneWidget);
      expect(find.byIcon(Icons.call), findsOneWidget);
      expect(find.byTooltip('Accept'), findsOneWidget);
      expect(find.byTooltip('Decline'), findsOneWidget);
      expect(find.byType(VoiceFocusTrap), findsOneWidget);
    },
  );

  testWidgets('IncomingCallOverlay leaves native iOS call chrome to CallKit', (
    tester,
  ) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.iOS;
    final container = _incomingContainer(
      client: MockClient((_) async => http.Response('{}', 200)),
    );
    addTearDown(container.dispose);

    try {
      await _pumpIncomingOverlay(tester, container);

      expect(find.byKey(IncomingCallOverlay.overlayKey), findsNothing);
      expect(find.byKey(IncomingCallOverlay.acceptKey), findsNothing);
      expect(find.byKey(IncomingCallOverlay.declineKey), findsNothing);
    } finally {
      debugDefaultTargetPlatformOverride = null;
    }
  });

  testWidgets('IncomingCallOverlay remains available on web and non-iOS', (
    tester,
  ) async {
    // On a web test target this exercises iOS web specifically; on the DartVM
    // target it verifies the Windows presentation path.
    debugDefaultTargetPlatformOverride = kIsWeb
        ? TargetPlatform.iOS
        : TargetPlatform.windows;
    final container = _incomingContainer(
      client: MockClient((_) async => http.Response('{}', 200)),
    );
    addTearDown(container.dispose);

    try {
      await _pumpIncomingOverlay(tester, container);
      await tester.pump();

      expect(find.byKey(IncomingCallOverlay.overlayKey), findsOneWidget);
      expect(find.byKey(IncomingCallOverlay.acceptKey), findsOneWidget);
      expect(find.byKey(IncomingCallOverlay.declineKey), findsOneWidget);
    } finally {
      debugDefaultTargetPlatformOverride = null;
    }
  });

  testWidgets('IncomingCallOverlay respects voice capability and call state', (
    tester,
  ) async {
    final disabled = _incomingContainer(
      client: MockClient((_) async => http.Response('{}', 200)),
      config: const GatewayConfig(baseUrl: ''),
    );
    addTearDown(disabled.dispose);
    await _pumpIncomingOverlay(tester, disabled);
    expect(find.byKey(IncomingCallOverlay.overlayKey), findsNothing);

    final inactive = _incomingContainer(
      client: MockClient((_) async => http.Response('{}', 200)),
      phase: CallPhase.outgoing,
    );
    addTearDown(inactive.dispose);
    await _pumpIncomingOverlay(tester, inactive);
    expect(find.byKey(IncomingCallOverlay.overlayKey), findsNothing);

    final missingSession = _incomingContainer(
      client: MockClient((_) async => http.Response('{}', 200)),
      session: null,
    );
    // The factory supplies its normal ringing session unless explicitly cleared.
    addTearDown(missingSession.dispose);
    missingSession.read(callControllerProvider.notifier).state =
        const CallState(phase: CallPhase.incoming);
    await _pumpIncomingOverlay(tester, missingSession);
    expect(find.byKey(IncomingCallOverlay.overlayKey), findsNothing);
  });

  testWidgets('IncomingCallOverlay reflects video caller and accept action', (
    tester,
  ) async {
    final requests = <String>[];
    final container = _incomingContainer(
      client: MockClient((request) async {
        requests.add(request.url.path);
        return http.Response('unavailable', 503);
      }),
      mediaKind: VoiceCallMediaKind.video,
    );
    addTearDown(container.dispose);

    await _pumpIncomingOverlay(tester, container);
    await tester.pumpAndSettle();

    expect(find.text('Caller'), findsOneWidget);
    expect(find.byIcon(Icons.videocam), findsOneWidget);
    expect(find.byTooltip('Accept'), findsOneWidget);
    await tester.tap(find.byKey(IncomingCallOverlay.acceptKey));
    await tester.pumpAndSettle();

    expect(requests, contains('/api/v1/voice/calls/room-1/accept'));
    expect(container.read(callControllerProvider).phase, CallPhase.failed);
  });

  testWidgets('IncomingCallOverlay decline invokes the existing action', (
    tester,
  ) async {
    final requests = <String>[];
    final container = _incomingContainer(
      client: MockClient((request) async {
        requests.add(request.url.path);
        return http.Response('unavailable', 503);
      }),
    );
    addTearDown(container.dispose);

    await _pumpIncomingOverlay(tester, container);
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(IncomingCallOverlay.declineKey));
    await tester.pumpAndSettle();

    expect(requests, contains('/api/v1/voice/calls/room-1/decline'));
  });

  testWidgets('IncomingCallOverlay returns focus after each dismissal', (
    tester,
  ) async {
    final firstTrigger = FocusNode(debugLabel: 'first call trigger');
    final secondTrigger = FocusNode(debugLabel: 'second call trigger');
    addTearDown(firstTrigger.dispose);
    addTearDown(secondTrigger.dispose);

    final container = _incomingContainer(
      client: MockClient((_) async => http.Response('{}', 200)),
      phase: CallPhase.outgoing,
    );
    addTearDown(container.dispose);
    final callController = container.read(callControllerProvider.notifier);
    final session = container.read(callControllerProvider).session!;
    var overlayMounted = true;
    late StateSetter updateHost;

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme().copyWith(
            textTheme: voiceTestTheme().textTheme.apply(
              fontFamily: 'Noto Sans',
            ),
          ),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: StatefulBuilder(
            builder: (context, setState) {
              updateHost = setState;
              return Scaffold(
                body: Stack(
                  children: [
                    Align(
                      alignment: Alignment.topLeft,
                      child: Column(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          TextButton(
                            focusNode: firstTrigger,
                            onPressed: () {},
                            child: const Text('First trigger'),
                          ),
                          TextButton(
                            focusNode: secondTrigger,
                            onPressed: () {},
                            child: const Text('Second trigger'),
                          ),
                        ],
                      ),
                    ),
                    if (overlayMounted) const IncomingCallOverlay(),
                  ],
                ),
              );
            },
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    firstTrigger.requestFocus();
    await tester.pump();
    expect(FocusManager.instance.primaryFocus, same(firstTrigger));

    callController.state = CallState(
      phase: CallPhase.incoming,
      session: session,
    );
    await tester.pumpAndSettle();
    expect(find.byKey(IncomingCallOverlay.overlayKey), findsOneWidget);

    callController.state = CallState(
      phase: CallPhase.outgoing,
      session: session,
    );
    await tester.pumpAndSettle();
    expect(FocusManager.instance.primaryFocus, same(firstTrigger));

    secondTrigger.requestFocus();
    await tester.pump();
    expect(FocusManager.instance.primaryFocus, same(secondTrigger));

    callController.state = CallState(
      phase: CallPhase.incoming,
      session: session,
    );
    await tester.pumpAndSettle();
    expect(find.byKey(IncomingCallOverlay.overlayKey), findsOneWidget);

    callController.state = CallState(
      phase: CallPhase.outgoing,
      session: session,
    );
    await tester.pumpAndSettle();
    expect(FocusManager.instance.primaryFocus, same(secondTrigger));

    // The app shell keeps the consumer mounted while no call is incoming. If
    // the host removes it later, it must not restore the first call's target.
    updateHost(() => overlayMounted = false);
    await tester.pumpAndSettle();
    expect(FocusManager.instance.primaryFocus, same(secondTrigger));
  });

  testWidgets('IncomingCallOverlay fits the horizontal s19 viewport', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(1280, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    final container = _incomingContainer(
      client: MockClient((_) async => http.Response('{}', 200)),
      mediaKind: VoiceCallMediaKind.video,
      callerName: 'Alex',
    );
    addTearDown(container.dispose);

    await _pumpIncomingOverlay(tester, container);
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
    await expectLater(
      find.byKey(IncomingCallOverlay.overlayKey),
      matchesGoldenFile('goldens/incoming_call_overlay_h.png'),
    );
  });

  testWidgets('IncomingCallOverlay fits the vertical s19 viewport', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    final container = _incomingContainer(
      client: MockClient((_) async => http.Response('{}', 200)),
      mediaKind: VoiceCallMediaKind.video,
      callerName: 'Alex',
    );
    addTearDown(container.dispose);

    await _pumpIncomingOverlay(tester, container);
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
    await expectLater(
      find.byKey(IncomingCallOverlay.overlayKey),
      matchesGoldenFile('goldens/incoming_call_overlay_v.png'),
    );
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
          home: const Scaffold(body: Stack(children: [OutgoingCallOverlay()])),
        ),
      ),
    );
    await tester.pump();

    expect(find.byKey(OutgoingCallOverlay.overlayKey), findsOneWidget);
    expect(find.byKey(OutgoingCallOverlay.cancelKey), findsOneWidget);
    expect(find.textContaining('Callee'), findsWidgets);
  });

  testWidgets('CallErrorListener shows snackbar on call failure', (
    tester,
  ) async {
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
                      ref
                          .read(callControllerProvider.notifier)
                          .state = const CallState(
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
