import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/subscription_client.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/settings/chat_theme_preference.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/presence_providers.dart';
import 'package:voice_frontend/state/social_providers.dart';
import 'package:voice_frontend/state/subscription_providers.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';
import 'package:voice_frontend/ui/core/voice_chat_bubble.dart';
import 'package:voice_frontend/ui/settings/chat_themes_settings_screen.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  testWidgets(
    'an entitled chat renders its own saved palette in message bubbles',
    (tester) async {
      SharedPreferences.setMockInitialValues({
        chatThemePreferencePrefKey: '{"chat-a":"violet","chat-b":"sunset"}',
      });
      final container = ProviderContainer(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => http.Response('{}', 404)),
          ),
          subscriptionProvider.overrideWith(
            (ref) async => const VoiceSubscription(
              id: 'subscription-1',
              accountId: 'acc-test',
              plan: 'premium',
              billingPeriod: 'month',
              status: 'active',
            ),
          ),
          chatRoomControllerProvider(
            'chat-a',
          ).overrideWith((ref) => _ThemeRoomController(ref, 'chat-a')),
          presenceProvider('peer-1').overrideWith((ref) => null),
          profileProvider('peer-1').overrideWith(
            (ref) async => const VoiceProfile(
              id: 'peer-1',
              accountId: 'peer-account',
              username: 'peer',
              discriminator: '1234',
              displayName: 'Theme Preview Peer',
            ),
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
            home: const Scaffold(body: ChatRoomPanel(chatId: 'chat-a')),
          ),
        ),
      );
      await tester.pumpAndSettle();

      final bubble = tester.widget<VoiceChatBubble>(
        find.byType(VoiceChatBubble).first,
      );
      expect(bubble.palette, ChatThemePalette.forTheme(ChatTheme.violet));
      expect(
        container.read(effectiveChatThemeProvider('chat-b')),
        ChatTheme.sunset,
      );
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('saved overrides render with default palette after Plus lapses', (
    tester,
  ) async {
    SharedPreferences.setMockInitialValues({
      chatThemePreferencePrefKey: '{"chat-a":"midnight"}',
    });
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 404)),
        ),
        subscriptionProvider.overrideWith((ref) async => null),
        chatRoomControllerProvider(
          'chat-a',
        ).overrideWith((ref) => _ThemeRoomController(ref, 'chat-a')),
        presenceProvider('peer-1').overrideWith((ref) => null),
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
          home: const Scaffold(body: ChatRoomPanel(chatId: 'chat-a')),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(
      tester
          .widget<VoiceChatBubble>(find.byType(VoiceChatBubble).first)
          .palette,
      isNull,
    );
    expect(tester.takeException(), isNull);
  });

  testWidgets('captures production-font picker and conversation in H and V', (
    tester,
  ) async {
    final captureTheme = await _loadProductionTheme(tester);
    final captureDirectory = Directory(
      Platform.environment['VOICE_CHAT_THEMES_CAPTURE_DIR'] ??
          '${Directory.systemTemp.path}${Platform.pathSeparator}voice-chat-themes',
    );
    await tester.runAsync(() => captureDirectory.create(recursive: true));

    for (final viewport in [const Size(1280, 800), const Size(390, 844)]) {
      tester.view.physicalSize = viewport;
      tester.view.devicePixelRatio = 1;
      tester.binding.handleMetricsChanged();
      SharedPreferences.setMockInitialValues({
        chatThemePreferencePrefKey: '{"chat-a":"violet"}',
      });
      final container = ProviderContainer(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => http.Response('{}', 404)),
          ),
          subscriptionProvider.overrideWith(
            (ref) async => const VoiceSubscription(
              id: 'subscription-1',
              accountId: 'acc-test',
              plan: 'premium',
              billingPeriod: 'month',
              status: 'active',
            ),
          ),
          chatRoomControllerProvider(
            'chat-a',
          ).overrideWith((ref) => _ThemeRoomController(ref, 'chat-a')),
          presenceProvider('peer-1').overrideWith((ref) => null),
          profileProvider('peer-1').overrideWith(
            (ref) async => const VoiceProfile(
              id: 'peer-1',
              accountId: 'peer-account',
              username: 'peer',
              discriminator: '1234',
              displayName: 'Theme Preview Peer',
            ),
          ),
        ],
      );
      container.read(selectedChatIdProvider.notifier).state = 'chat-a';
      final orientation = viewport.width > viewport.height ? 'h' : 'v';

      await tester.pumpWidget(
        RepaintBoundary(
          key: _captureBoundaryKey,
          child: UncontrolledProviderScope(
            container: container,
            child: MaterialApp(
              theme: captureTheme,
              debugShowCheckedModeBanner: false,
              localizationsDelegates: AppLocalizations.localizationsDelegates,
              supportedLocales: AppLocalizations.supportedLocales,
              home: const ChatThemesSettingsScreen(chatId: 'chat-a'),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.byKey(ChatThemesSettingsScreen.screenKey), findsOneWidget);
      await _capture(tester, captureDirectory, 'picker-$orientation.png');

      await tester.pumpWidget(
        RepaintBoundary(
          key: _captureBoundaryKey,
          child: UncontrolledProviderScope(
            container: container,
            child: MaterialApp(
              theme: captureTheme,
              debugShowCheckedModeBanner: false,
              localizationsDelegates: AppLocalizations.localizationsDelegates,
              supportedLocales: AppLocalizations.supportedLocales,
              home: const Scaffold(body: ChatRoomPanel(chatId: 'chat-a')),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      final bubble = tester.widget<VoiceChatBubble>(
        find.byType(VoiceChatBubble).first,
      );
      expect(bubble.palette, ChatThemePalette.forTheme(ChatTheme.violet));
      await _capture(tester, captureDirectory, 'conversation-$orientation.png');
      expect(tester.takeException(), isNull);

      await tester.pumpWidget(const SizedBox.shrink());
      container.dispose();
    }

    tester.view.resetPhysicalSize();
    tester.view.resetDevicePixelRatio();
  });
}

const _captureBoundaryKey = Key('chat_theme_capture_boundary');

Future<void> _capture(
  WidgetTester tester,
  Directory directory,
  String name,
) async {
  final file = File('${directory.path}${Platform.pathSeparator}$name');
  final bytes = await tester.runAsync(() async {
    final boundary = tester.renderObject<RenderRepaintBoundary>(
      find.byKey(_captureBoundaryKey),
    );
    final image = await boundary.toImage(
      pixelRatio: tester.view.devicePixelRatio,
    );
    try {
      final data = await image.toByteData(format: ui.ImageByteFormat.png);
      if (data == null) throw StateError('PNG encoding returned null');
      await file.writeAsBytes(data.buffer.asUint8List(), flush: true);
      return await file.length();
    } finally {
      image.dispose();
    }
  });
  expect(bytes, greaterThan(0));
  // ignore: avoid_print
  print('CHAT_THEME_CAPTURE ${file.path} $bytes bytes');
}

Future<ThemeData> _loadProductionTheme(WidgetTester tester) async {
  final loaded = await tester.runAsync(() async {
    final noto = FontLoader(VoiceTheme.fontFamily)
      ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
      ..addFont(rootBundle.load('assets/fonts/NotoSans-Medium.ttf'))
      ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'))
      ..addFont(rootBundle.load('assets/fonts/NotoSans-Bold.ttf'));
    await noto.load();

    var flutterRoot = File(Platform.resolvedExecutable).parent;
    File iconFontForRoot(Directory root) => File(
      [
        root.path,
        'bin',
        'cache',
        'artifacts',
        'material_fonts',
        'MaterialIcons-Regular.otf',
      ].join(Platform.pathSeparator),
    );
    var iconFont = iconFontForRoot(flutterRoot);
    while (!iconFont.existsSync() &&
        flutterRoot.path != flutterRoot.parent.path) {
      flutterRoot = flutterRoot.parent;
      iconFont = iconFontForRoot(flutterRoot);
    }
    if (!iconFont.existsSync()) {
      throw StateError('Flutter MaterialIcons font is not cached');
    }
    final iconData = ByteData.sublistView(
      Uint8List.fromList(await iconFont.readAsBytes()),
    );
    await (FontLoader('MaterialIcons')..addFont(Future.value(iconData))).load();
    return true;
  });
  if (loaded != true) throw StateError('Production font loading failed');

  final theme = await tester.runAsync(() async {
    final catalog = await VoiceTokenCatalog.load();
    return VoiceTheme.build(
      catalog: catalog,
      mode: VoiceThemeMode.dark,
      profileAccent: catalog.profileAccentAt(0),
    );
  });
  if (theme == null) throw StateError('Production tokens did not load');
  return theme;
}

class _ThemeRoomController extends ChatRoomController {
  _ThemeRoomController(super.ref, super.chatId) : super() {
    state = ChatRoomState(
      messages: [
        VoiceMessage(
          id: '$chatId-message',
          chatId: chatId,
          senderProfileId: 'peer-1',
          content: 'A real room message',
          createdAt: DateTime.utc(2026, 10, 7),
        ),
      ],
      historyProfileId: 'prof-test',
    );
  }

  @override
  Future<void> loadInitial() async {}
}
