import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:livekit_client/livekit_client.dart' as livekit;
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/livekit_room.dart';
import 'package:voice_frontend/backend/screen_share_sources.dart';
import 'package:voice_frontend/backend/voice_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/call_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/screen_share_providers.dart';
import 'package:voice_frontend/ui/call/screen_share_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

class _ScreenShareRoom extends Fake implements VoiceLiveKitRoom {
  bool failFirstCapture = false;
  final captureRates = <double>[];
  final captureAudioOptions = <bool>[];
  final captureSourceIds = <String?>[];
  int stopCalls = 0;

  @override
  void Function(bool)? onAudioPlaybackUnlockNeeded;

  @override
  void Function()? onTracksChanged;

  @override
  Future<void> connect({
    required String url,
    required String token,
    required bool video,
  }) async {}

  @override
  Future<void> disconnect() async {}

  @override
  Future<void> setMuted(bool muted) async {}

  @override
  livekit.LocalVideoTrack? localScreenShareTrack() => null;

  @override
  Future<void> startScreenShare({
    double maxFrameRate = 15,
    bool captureSystemAudio = false,
    String? sourceId,
  }) async {
    captureRates.add(maxFrameRate);
    captureAudioOptions.add(captureSystemAudio);
    captureSourceIds.add(sourceId);
    if (failFirstCapture && captureRates.length == 1) {
      throw StateError('private-screen-detail-6ab');
    }
  }

  @override
  Future<void> stopScreenShare() async {
    stopCalls++;
  }
}

void _testWindowsWidgets(String description, WidgetTesterCallback callback) {
  testWidgets(
    description,
    callback,
    variant: TargetPlatformVariant.only(TargetPlatform.windows),
  );
}

class _EmptySourceCatalog implements ScreenShareSourceCatalog {
  @override
  Future<List<ScreenShareSource>> listSources() async => const [];
}

Future<ProviderContainer> _pumpSourcePicker(
  WidgetTester tester, {
  bool emptySources = false,
}) async {
  final container = ProviderContainer(
    overrides: [
      ...voiceAppTestOverrides(
        client: MockClient((_) async => http.Response('{}', 200)),
      ),
      if (emptySources)
        screenShareSourceCatalogProvider.overrideWithValue(
          _EmptySourceCatalog(),
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
      initiatorProfileId: 'prof-test',
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
          body: Column(children: [ScreenSharePanel(), ScreenShareCallButton()]),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
  return container;
}

void main() {
  _testWindowsWidgets('unavailable desktop sources show a safe error', (
    tester,
  ) async {
    final container = await _pumpSourcePicker(tester, emptySources: true);
    await tester.tap(find.byKey(ScreenSharePanel.shareButtonKey));
    await tester.pumpAndSettle();
    final l10n = AppLocalizations.of(
      tester.element(find.byKey(ScreenSharePanel.panelKey)),
    )!;
    expect(find.text(l10n.screenShareSourceUnavailable), findsOneWidget);
    expect(find.byType(AlertDialog), findsNothing);
    expect(container.read(screenShareControllerProvider).isSharing, isFalse);
  });

  _testWindowsWidgets(
    'cancelling source selection does not open quality or share',
    (tester) async {
      final container = await _pumpSourcePicker(tester);
      await tester.tap(find.byKey(ScreenSharePanel.shareButtonKey));
      await tester.pumpAndSettle();
      expect(find.text('Test screen'), findsOneWidget);
      expect(find.text('Test window'), findsOneWidget);
      await tester.tapAt(const Offset(5, 5));
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsNothing);
      expect(container.read(screenShareControllerProvider).isSharing, isFalse);
      expect(
        container.read(screenShareControllerProvider).errorMessage,
        isNull,
      );
    },
  );

  testWidgets(
    'unsupported desktop audio stays disabled and skips source picker',
    (tester) async {
      await _pumpSourcePicker(tester);
      await tester.tap(find.byKey(ScreenSharePanel.shareButtonKey));
      await tester.pumpAndSettle();
      expect(find.text('Test screen'), findsNothing);
      expect(find.text('Test window'), findsNothing);
      final toggle = tester.widget<SwitchListTile>(find.byType(SwitchListTile));
      expect(toggle.value, isFalse);
      expect(toggle.onChanged, isNull);
    },
    variant: TargetPlatformVariant.only(TargetPlatform.macOS),
  );

  _testWindowsWidgets(
    'dual screen share shows both streams with local self preview',
    (tester) async {
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
      expect(
        find.byKey(const Key('screen_share_local_preview')),
        findsOneWidget,
      );
      expect(find.textContaining('Waiting for video'), findsNothing);
      expect(
        find.byKey(Key('screen_share_remote_$peerProfileId')),
        findsOneWidget,
      );

      await tester.tap(find.text('prof-pee'));
      await tester.pump();
      expect(find.byKey(const Key('screen_share_local_preview')), findsNothing);
    },
  );

  _testWindowsWidgets('screen share shows waiting text and limit error', (
    tester,
  ) async {
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

  _testWindowsWidgets('screen share pause button toggles pause state', (
    tester,
  ) async {
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
          home: const Scaffold(body: Row(children: [ScreenSharePauseButton()])),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byIcon(Icons.play_arrow), findsOneWidget);
    await tester.tap(find.byIcon(Icons.play_arrow));
    await tester.pump();
  });

  _testWindowsWidgets('screen share stop button and generic error message', (
    tester,
  ) async {
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
              children: [ScreenSharePanel(), ScreenShareCallButton()],
            ),
          ),
        ),
      ),
    );
    await tester.pump();

    final l10n = AppLocalizations.of(
      tester.element(find.byKey(ScreenSharePanel.panelKey)),
    )!;
    expect(find.text(l10n.commonActionFailed), findsOneWidget);
    expect(find.text('upload_failed'), findsNothing);
    await tester.tap(find.byKey(ScreenSharePanel.shareButtonKey));
    await tester.pump();
  });

  for (final language in ['en', 'ru']) {
    for (final failure in [
      ('API 500', 500),
      ('API 503', 503),
      ('capture', 200),
      ('limit', 429),
    ]) {
      _testWindowsWidgets(
        'ScreenSharePanel $language ${failure.$1} safely recovers on explicit retry',
        (tester) async {
          final room = _ScreenShareRoom()
            ..failFirstCapture = failure.$1 == 'capture';
          final requests = <http.Request>[];
          var shareAttempts = 0;
          final container = ProviderContainer(
            overrides: [
              ...voiceAppTestOverrides(
                client: MockClient((request) async {
                  requests.add(request);
                  if (request.method == 'POST' &&
                      request.url.path == '/api/v1/voice/calls/room-1/accept') {
                    return http.Response(
                      jsonEncode({
                        'call_session': {
                          'room_id': 'room-1',
                          'livekit_room_name': 'lk-room',
                          'linked_chat': {'id': 'chat-1'},
                          'initiator_profile_id': 'prof-peer',
                          'callee_profile_id': 'prof-test',
                          'media_kind': 'CALL_MEDIA_KIND_AUDIO',
                          'status': 'CALL_STATUS_ACTIVE',
                        },
                      }),
                      200,
                    );
                  }
                  if (request.method == 'GET' &&
                      request.url.path == '/api/v1/voice/calls/room-1/token') {
                    return http.Response(
                      jsonEncode({
                        'jwt': 'fake-media-token',
                        'livekit_url': 'wss://media.test',
                      }),
                      200,
                    );
                  }
                  if (request.method == 'POST' &&
                      request.url.path ==
                          '/api/v1/voice/calls/room-1/screen-share/start') {
                    shareAttempts++;
                    if (shareAttempts == 1 && failure.$2 != 200) {
                      return http.Response(
                        jsonEncode({'message': 'private-screen-detail-6ab'}),
                        failure.$2,
                      );
                    }
                    return http.Response(
                      jsonEncode({
                        'screen_share_session': {
                          'stream_id': 'stream-$shareAttempts',
                        },
                      }),
                      200,
                    );
                  }
                  if (request.method == 'POST' &&
                      request.url.path ==
                          '/api/v1/voice/calls/room-1/screen-share/stop') {
                    return http.Response('', 204);
                  }
                  return http.Response('{}', 404);
                }),
              ),
              liveKitRoomFactoryProvider.overrideWithValue(() => room),
            ],
          );
          addTearDown(container.dispose);
          final controller = container.read(callControllerProvider.notifier);
          controller.state = const CallState(
            phase: CallPhase.incoming,
            session: VoiceCallSession(
              roomId: 'room-1',
              livekitRoomName: 'lk-room',
              chatId: 'chat-1',
              initiatorProfileId: 'prof-peer',
              calleeProfileId: 'prof-test',
              mediaKind: VoiceCallMediaKind.audio,
              status: VoiceCallStatus.ringing,
            ),
          );
          await controller.acceptCall();
          expect(
            container.read(callControllerProvider).phase,
            CallPhase.active,
          );
          expect(controller.liveKitRoom, same(room));
          requests.clear();

          await tester.pumpWidget(
            UncontrolledProviderScope(
              container: container,
              child: MaterialApp(
                theme: voiceTestTheme(),
                locale: Locale(language),
                localizationsDelegates: AppLocalizations.localizationsDelegates,
                supportedLocales: AppLocalizations.supportedLocales,
                home: const Scaffold(
                  body: Column(
                    children: [ScreenSharePanel(), ScreenShareCallButton()],
                  ),
                ),
              ),
            ),
          );
          await tester.pumpAndSettle();
          final l10n = AppLocalizations.of(
            tester.element(find.byKey(ScreenSharePanel.panelKey)),
          )!;

          Future<void> startShare() async {
            await tester.tap(find.byKey(ScreenSharePanel.shareButtonKey));
            await tester.pumpAndSettle();
            await tester.tap(find.text('Test window'));
            await tester.pumpAndSettle();
            await tester.tap(find.text(l10n.screenShareQuality720p15));
            await tester.pumpAndSettle();
          }

          await startShare();
          expect(
            find.text(
              failure.$1 == 'limit'
                  ? l10n.screenShareLimitReached
                  : l10n.commonActionFailed,
            ),
            findsOneWidget,
          );
          expect(
            find.textContaining('private-screen-detail-6ab'),
            findsNothing,
          );
          expect(find.textContaining('Bad state:'), findsNothing);
          expect(find.textContaining('Exception'), findsNothing);
          expect(
            container.read(screenShareControllerProvider).isSharing,
            isFalse,
          );
          expect(
            container.read(callControllerProvider).phase,
            CallPhase.active,
          );
          expect(
            container.read(callControllerProvider).session!.roomId,
            'room-1',
          );
          expect(shareAttempts, 1);
          expect(room.captureRates, failure.$1 == 'capture' ? [15.0] : isEmpty);
          expect(requests.map((request) => request.url.path).toList(), [
            '/api/v1/voice/calls/room-1/screen-share/start',
            if (failure.$1 == 'capture')
              '/api/v1/voice/calls/room-1/screen-share/stop',
          ]);
          if (failure.$1 == 'capture') {
            expect(jsonDecode(requests.last.body), {'stream_id': 'stream-1'});
          }

          await startShare();
          expect(shareAttempts, 2);
          final sharing = container.read(screenShareControllerProvider);
          expect(sharing.isSharing, isTrue);
          expect(sharing.localStreamId, 'stream-2');
          expect(sharing.errorMessage, isNull);
          expect(find.text(l10n.commonActionFailed), findsNothing);
          expect(find.text(l10n.screenShareLimitReached), findsNothing);
          expect(
            room.captureRates,
            failure.$1 == 'capture' ? [15.0, 15.0] : [15.0],
          );
          expect(
            room.captureAudioOptions,
            failure.$1 == 'capture' ? [true, true] : [true],
          );
          expect(
            room.captureSourceIds,
            failure.$1 == 'capture'
                ? ['test-window-id', 'test-window-id']
                : ['test-window-id'],
          );
          for (final request in requests) {
            expect(request.method, 'POST');
            expect(request.headers['Authorization'], 'Bearer test-access');
            if (request.url.path.endsWith('/start')) {
              expect(jsonDecode(request.body), {'room_id': 'room-1'});
            }
          }

          await tester.tap(find.byKey(ScreenSharePanel.shareButtonKey));
          await tester.pumpAndSettle();
          expect(room.stopCalls, 1);
          expect(
            container.read(screenShareControllerProvider).isSharing,
            isFalse,
          );
          expect(
            container.read(callControllerProvider).phase,
            CallPhase.active,
          );
          expect(requests.map((request) => request.url.path).toList(), [
            '/api/v1/voice/calls/room-1/screen-share/start',
            if (failure.$1 == 'capture')
              '/api/v1/voice/calls/room-1/screen-share/stop',
            '/api/v1/voice/calls/room-1/screen-share/start',
            '/api/v1/voice/calls/room-1/screen-share/stop',
          ]);
          expect(jsonDecode(requests.last.body), {'stream_id': 'stream-2'});
        },
      );
    }
  }

  _testWindowsWidgets('screen share quality dialog returns selected fps', (
    tester,
  ) async {
    ScreenShareStartOptions? selectedOptions;
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Builder(
          builder: (context) => Scaffold(
            body: FilledButton(
              onPressed: () async {
                selectedOptions = await showScreenShareQualityDialog(context);
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
    final l10n = AppLocalizations.of(
      tester.element(find.text('open-quality')),
    )!;
    expect(find.text(l10n.screenShareSystemAudio), findsOneWidget);
    await tester.tap(find.text('720p · 15 FPS'));
    await tester.pumpAndSettle();

    expect(selectedOptions?.maxFrameRate, 15.0);
    expect(selectedOptions?.captureSystemAudio, isTrue);
  });

  _testWindowsWidgets(
    'screen share quality dialog allows opting out of system audio',
    (tester) async {
      ScreenShareStartOptions? selectedOptions;
      await tester.pumpWidget(
        MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Builder(
            builder: (context) => Scaffold(
              body: FilledButton(
                onPressed: () async {
                  selectedOptions = await showScreenShareQualityDialog(context);
                },
                child: const Text('open-quality'),
              ),
            ),
          ),
        ),
      );
      await tester.tap(find.text('open-quality'));
      await tester.pumpAndSettle();
      final l10n = AppLocalizations.of(
        tester.element(find.text('open-quality')),
      )!;
      await tester.tap(find.text(l10n.screenShareSystemAudio));
      await tester.tap(find.text('720p · 30 FPS'));
      await tester.pumpAndSettle();

      expect(selectedOptions?.maxFrameRate, 30.0);
      expect(selectedOptions?.captureSystemAudio, isFalse);
    },
  );
}
