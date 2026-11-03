import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/backend/voice_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/call_providers.dart';
import 'package:voice_frontend/state/social_providers.dart';
import 'package:voice_frontend/ui/call/outgoing_call_overlay.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  setUpAll(() async {
    final icons = FontLoader('MaterialIcons')
      ..addFont(rootBundle.load('fonts/MaterialIcons-Regular.otf'));
    await icons.load();
    final font = FontLoader('Noto Sans')
      ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
      ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'));
    await font.load();
  });
  for (final size in [const Size(1280, 800), const Size(390, 844)]) {
    for (final video in [false, true]) {
      testWidgets('outgoing $size video=$video controls preserve intent', (
        tester,
      ) async {
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        addTearDown(tester.view.resetPhysicalSize);
        addTearDown(tester.view.resetDevicePixelRatio);
        final requests = <String>[];
        final container = ProviderContainer(
          overrides: [
            ...voiceAppTestOverrides(
              client: MockClient((request) async {
                requests.add(request.url.path);
                return http.Response('{}', 200);
              }),
            ),
            profileProvider('callee').overrideWith(
              (ref) async => const VoiceProfile(
                id: 'callee',
                accountId: 'account',
                username: 'maria',
                discriminator: '0001',
                displayName: 'Maria',
              ),
            ),
          ],
        );
        addTearDown(container.dispose);
        final controller = container.read(callControllerProvider.notifier);
        controller.state = CallState(
          phase: CallPhase.outgoing,
          isVideoEnabled: video,
          session: VoiceCallSession(
            roomId: 'room',
            livekitRoomName: 'voice-dm-room',
            chatId: 'chat',
            initiatorProfileId: 'prof-test',
            calleeProfileId: 'callee',
            mediaKind: video
                ? VoiceCallMediaKind.video
                : VoiceCallMediaKind.audio,
            status: VoiceCallStatus.ringing,
          ),
        );
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
              home: const Scaffold(
                body: Stack(children: [OutgoingCallOverlay()]),
              ),
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(find.text('Maria'), findsOneWidget);
        expect(find.byTooltip('Mute'), findsOneWidget);
        await tester.sendKeyEvent(LogicalKeyboardKey.tab);
        expect(
          FocusManager.instance.primaryFocus?.context
              ?.findAncestorWidgetOfExactType<OutgoingCallOverlay>(),
          isNotNull,
        );
        expect(
          find.byTooltip('Turn camera off'),
          video ? findsOneWidget : findsNothing,
        );
        await expectLater(
          find.byKey(OutgoingCallOverlay.overlayKey),
          matchesGoldenFile(
            'goldens/linux/outgoing_call_v3_${size.width > 600 ? 'h' : 'v'}_${video ? 'video' : 'audio'}.png',
          ),
        );
        tester.platformDispatcher.textScaleFactorTestValue = 2;
        addTearDown(tester.platformDispatcher.clearTextScaleFactorTestValue);
        await tester.pumpAndSettle();
        await tester.tap(find.byTooltip('Mute'));
        await tester.pumpAndSettle();
        expect(controller.state.isMuted, isTrue);
        expect(find.byTooltip('Unmute'), findsOneWidget);
        if (video) {
          await tester.tap(find.byTooltip('Turn camera off'));
          await tester.pumpAndSettle();
          expect(controller.state.isVideoEnabled, isFalse);
          expect(find.byTooltip('Turn camera on'), findsOneWidget);
        }
        expect(tester.takeException(), isNull);
        await tester.tap(find.byKey(OutgoingCallOverlay.cancelKey));
        await tester.pumpAndSettle();
        expect(requests, contains('/api/v1/voice/calls/room/end'));
        expect(find.byKey(OutgoingCallOverlay.overlayKey), findsNothing);
        await tester.pumpWidget(const SizedBox());
      });
    }
  }
}
