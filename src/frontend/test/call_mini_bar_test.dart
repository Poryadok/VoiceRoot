import 'dart:io';
import 'dart:ui' as ui;
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/livekit_room.dart';
import 'package:voice_frontend/backend/voice_client.dart';
import 'package:voice_frontend/backend/proto_mappers.dart';
import 'package:voice_frontend/gen/voice/chat/v1/chat.pb.dart' as chat_pb;
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/call_presentation_providers.dart';
import 'package:voice_frontend/state/call_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/ui/call/active_call_panel.dart';
import 'package:voice_frontend/ui/call/call_mini_bar.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  Future<ProviderContainer> mount(
    WidgetTester tester, {
    bool mobile = false,
    bool active = true,
    bool connecting = false,
    double textScale = 1,
    Locale locale = const Locale('en'),
  }) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = Size(mobile ? 390 : 320, 844);
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    final theme = Platform.environment['VOICE_MINIBAR_CAPTURE_DIR'] == null
        ? voiceTestTheme()
        : (await tester.runAsync(() async {
            await (FontLoader(VoiceTheme.fontFamily)
                  ..addFont(
                    rootBundle.load('assets/fonts/NotoSans-Regular.ttf'),
                  )
                  ..addFont(
                    rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'),
                  ))
                .load();
            await (FontLoader('MaterialIcons')
                  ..addFont(rootBundle.load('fonts/MaterialIcons-Regular.otf')))
                .load();
            final catalog = await VoiceTokenCatalog.load();
            return VoiceTheme.build(
              catalog: catalog,
              mode: VoiceThemeMode.light,
              profileAccent: catalog.profileAccentAt(0),
            );
          }))!;
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 200)),
        ),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://127.0.0.1:18080'),
        ),
        chatListProvider.overrideWith(
          (ref) async => chatListFromProto(
            chat_pb.ChatList(
              items: [
                chat_pb.ChatListItem(
                  chat: chat_pb.Chat(id: 'chat', name: 'Design voice'),
                ),
              ],
            ),
          ),
        ),
        callParticipantsProvider.overrideWithValue(const [
          VoiceCallParticipant(
            identity: 'maria',
            name: 'Maria',
            isSpeaking: true,
          ),
          VoiceCallParticipant(
            identity: 'alex',
            name: 'Alex',
            isSpeaking: false,
          ),
          VoiceCallParticipant(
            identity: 'daria',
            name: 'Daria',
            isSpeaking: false,
          ),
        ]),
      ],
    );
    addTearDown(container.dispose);
    if (active) {
      container.read(callControllerProvider.notifier).state = CallState(
        phase: connecting ? CallPhase.connecting : CallPhase.active,
        session: VoiceCallSession(
          roomId: 'room',
          livekitRoomName: 'room',
          chatId: 'chat',
          initiatorProfileId: 'self',
          calleeProfileId: 'peer',
          mediaKind: VoiceCallMediaKind.audio,
          status: VoiceCallStatus.active,
          startedAt: DateTime.now().subtract(const Duration(seconds: 42)),
        ),
      );
    }
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: theme,
          builder: (context, child) => MediaQuery(
            data: MediaQuery.of(
              context,
            ).copyWith(textScaler: TextScaler.linear(textScale)),
            child: child!,
          ),
          locale: locale,
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(
            body: RepaintBoundary(
              key: const Key('capture'),
              child: ColoredBox(
                color: theme.scaffoldBackgroundColor,
                child: Stack(
                  children: [
                    Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        if (!mobile)
                          const Expanded(
                            child: Center(child: Text('Chat list')),
                          ),
                        CallMiniBar(mobile: mobile),
                        if (mobile)
                          const Expanded(
                            child: Center(child: Text('Chat list')),
                          ),
                      ],
                    ),
                    const ActiveCallPanel(shellHosted: true),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    return container;
  }

  testWidgets('idle does not occupy a shell slot', (tester) async {
    await mount(tester, active: false);
    expect(find.byKey(CallMiniBar.panelKey), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'desktop dock shows duration and real control state; hangup clears it',
    (tester) async {
      final container = await mount(tester);
      expect(tester.getSize(find.byKey(CallMiniBar.panelKey)).height, 144);
      expect(find.textContaining('00:42'), findsOneWidget);
      expect(find.text('Maria is speaking'), findsOneWidget);
      expect(find.byKey(CallMiniBar.deafenKey), findsOneWidget);
      await tester.tap(find.byKey(CallMiniBar.muteKey));
      await tester.pump();
      expect(container.read(callControllerProvider).isMuted, isTrue);
      await tester.tap(find.byKey(CallMiniBar.deafenKey));
      await tester.pump();
      expect(container.read(callControllerProvider).isSpeakerMuted, isTrue);
      await tester.tap(find.byKey(CallMiniBar.hangupKey));
      await tester.pump();
      expect(container.read(callControllerProvider).isActive, isFalse);
      expect(find.byKey(CallMiniBar.panelKey), findsNothing);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets(
    'keyboard return expands call; minimize restores dock without ending session',
    (tester) async {
      final container = await mount(tester);
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      await tester.pump();
      expect(container.read(callExpandedRoomProvider), 'room');
      expect(find.byKey(CallMiniBar.panelKey), findsNothing);
      expect(find.byKey(ActiveCallPanel.panelKey), findsOneWidget);
      await tester.tap(find.byKey(ActiveCallPanel.minimizeKey));
      await tester.pump();
      expect(find.byKey(CallMiniBar.panelKey), findsOneWidget);
      expect(container.read(callControllerProvider).isActive, isTrue);
      await tester.pump();
      expect(
        FocusManager.instance.primaryFocus?.debugLabel,
        'call mini return',
      );
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('mobile enlarged text retains reachable controls and semantics', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    try {
      await mount(tester, mobile: true, textScale: 2);
      expect(find.byKey(CallMiniBar.muteKey), findsOneWidget);
      expect(find.byKey(CallMiniBar.hangupKey), findsOneWidget);
      expect(
        tester.getRect(find.byKey(CallMiniBar.hangupKey)).right,
        lessThanOrEqualTo(390),
      );
      expect(find.byTooltip('Hang up'), findsOneWidget);
      expect(tester.takeException(), isNull);
    } finally {
      semantics.dispose();
    }
  });

  for (final mobile in [false, true]) {
    for (final connecting in [false, true]) {
      testWidgets(
        '${mobile ? 'mobile' : 'desktop'} connecting=$connecting RU layout and capture',
        (tester) async {
          await mount(
            tester,
            mobile: mobile,
            connecting: connecting,
            locale: const Locale('ru'),
          );
          final panel = find.byKey(CallMiniBar.panelKey);
          expect(tester.getSize(panel).height, mobile ? 64 : 144);
          expect(tester.getTopLeft(panel).dy, mobile ? 0 : 700);
          expect(
            find.textContaining(
              connecting ? 'Подключение к звонку…' : 'Подключено',
            ),
            findsOneWidget,
          );
          expect(tester.takeException(), isNull);
          final directory = Platform.environment['VOICE_MINIBAR_CAPTURE_DIR'];
          if (directory != null) {
            await tester.runAsync(() async {
              final boundary = tester.renderObject<RenderRepaintBoundary>(
                find.byKey(const Key('capture')),
              );
              final image = await boundary.toImage(pixelRatio: 1);
              final bytes = await image.toByteData(
                format: ui.ImageByteFormat.png,
              );
              final folder = Directory(directory)..createSync(recursive: true);
              File(
                '${folder.path}/minibar-${mobile ? 'v' : 'h'}-${connecting ? 'connecting' : 'active'}.png',
              ).writeAsBytesSync(bytes!.buffer.asUint8List());
              image.dispose();
            });
          }
        },
      );
    }
  }
}
