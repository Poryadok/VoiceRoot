import 'dart:io';
import 'dart:ui' show ImageByteFormat, Tristate;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/app.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/routing/deep_link_listener.dart';
import 'package:voice_frontend/state/chat_navigation_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';
import 'package:voice_frontend/state/social_providers.dart';
import 'package:voice_frontend/state/shell_providers.dart';
import 'package:voice_frontend/state/presence_providers.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/ui/shell/chat_list_body.dart';
import 'package:voice_frontend/ui/shell/desktop_shell_rail.dart';
import 'package:voice_frontend/ui/shell/message_requests_folder.dart';
import 'package:voice_frontend/ui/shell/mobile_shell_drawer.dart';

import 'support/auth_test_overrides.dart';
import 'support/fake_voice_api_clients.dart';

const _captureBoundaryKey = Key('message_requests_shell_capture');
const _captureEnv = 'VOICE_CONTACT_REQUESTS_CAPTURE_DIR';
const _voipChannel = MethodChannel('voice/voip');
var _captureFontsLoaded = false;

void main() {
  testWidgets(
    'real shell opens and returns from message requests on wide and narrow layouts',
    (tester) async {
      final captureDir = Platform.environment[_captureEnv];
      final capturing = captureDir != null && captureDir.isNotEmpty;
      final captureTheme = await _themeForCapture(tester, capturing);
      final semantics = tester.ensureSemantics();
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(_voipChannel, (_) async => null);
      addTearDown(
        () => TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
            .setMockMethodCallHandler(_voipChannel, null),
      );
      addTearDown(() {
        tester.view.resetPhysicalSize();
        tester.view.resetDevicePixelRatio();
      });

      final wideContainer = await _pumpVoiceShell(
        tester,
        const Size(1280, 800),
        captureTheme,
      );
      expect(find.byKey(DesktopShellRail.railKey), findsOneWidget);
      expect(find.byKey(MessageRequestsFolderRailButton.keyId), findsOneWidget);
      expect(find.text('2'), findsWidgets);
      await tester.tap(find.byKey(MessageRequestsFolderRailButton.keyId));
      await tester.pumpAndSettle();
      _expectRequestsWorkspace(tester);
      await _captureIfRequested(tester, 'contact-requests-wide.png');
      await tester.tap(find.byIcon(Icons.arrow_back));
      await tester.pumpAndSettle();
      expect(wideContainer.read(selectedChatFolderIdProvider), isNull);
      expect(find.text('Direct messages'), findsWidgets);
      expect(find.text('Rowan'), findsNothing);

      final narrowContainer = await _pumpVoiceShell(
        tester,
        const Size(390, 844),
        captureTheme,
      );
      expect(find.byKey(const Key('mobile_shell_drawer_open')), findsOneWidget);
      await tester.tap(find.byKey(const Key('mobile_shell_drawer_open')));
      await tester.pumpAndSettle();
      expect(find.byKey(MobileShellDrawer.drawerKey), findsOneWidget);
      expect(find.byKey(MessageRequestsFolderDrawerTile.keyId), findsOneWidget);
      expect(find.text('2'), findsWidgets);
      expect(
        tester
            .getSemantics(find.byKey(MessageRequestsFolderDrawerTile.keyId))
            .getSemanticsData()
            .hasAction(SemanticsAction.tap),
        isTrue,
      );
      await tester.tap(find.byKey(MessageRequestsFolderDrawerTile.keyId));
      await tester.pumpAndSettle();
      expect(find.byKey(MobileShellDrawer.drawerKey), findsNothing);
      _expectRequestsWorkspace(tester);
      await _captureIfRequested(tester, 'contact-requests-narrow.png');
      await tester.tap(find.byKey(const Key('mobile_shell_drawer_open')));
      await tester.pumpAndSettle();
      final requestTile = tester.widget<ListTile>(
        find.byKey(MessageRequestsFolderDrawerTile.keyId),
      );
      expect(requestTile.selected, isTrue);
      expect(
        tester
                .getSemantics(find.byKey(MessageRequestsFolderDrawerTile.keyId))
                .getSemanticsData()
                .flagsCollection
                .isSelected ==
            Tristate.isTrue,
        isTrue,
      );
      await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      await tester.pumpAndSettle();
      if (find.byKey(MobileShellDrawer.drawerKey).evaluate().isNotEmpty) {
        await tester.tapAt(const Offset(380, 400));
        await tester.pumpAndSettle();
      }
      expect(find.byKey(MobileShellDrawer.drawerKey), findsNothing);
      await tester.tap(find.byIcon(Icons.arrow_back));
      await tester.pumpAndSettle();
      expect(narrowContainer.read(selectedChatFolderIdProvider), isNull);
      expect(find.text('Direct messages'), findsWidgets);
      expect(find.text('Rowan'), findsNothing);

      await tester.tap(find.byKey(const Key('mobile_shell_drawer_open')));
      await tester.pumpAndSettle();
      var requestTileFocused = false;
      for (var attempt = 0; attempt < 10 && !requestTileFocused; attempt++) {
        await tester.sendKeyEvent(LogicalKeyboardKey.tab);
        await tester.pump();
        requestTileFocused =
            tester
                .getSemantics(find.byKey(MessageRequestsFolderDrawerTile.keyId))
                .getSemanticsData()
                .flagsCollection
                .isFocused ==
            Tristate.isTrue;
      }
      expect(requestTileFocused, isTrue);
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      await tester.pumpAndSettle();
      expect(find.byKey(MobileShellDrawer.drawerKey), findsNothing);
      expect(narrowContainer.read(chatInboxProvider), 'requests');
      _expectRequestsWorkspace(tester);
      await tester.tap(find.byIcon(Icons.arrow_back));
      await tester.pumpAndSettle();
      expect(narrowContainer.read(chatInboxProvider), 'main');
      expect(find.text('Direct messages'), findsWidgets);
      semantics.dispose();
    },
  );
}

void _expectRequestsWorkspace(WidgetTester tester) {
  expect(find.text('Message requests'), findsOneWidget);
  expect(find.text('Rowan'), findsOneWidget);
  expect(find.byKey(ChatListBody.listKey), findsOneWidget);
}

Future<ProviderContainer> _pumpVoiceShell(
  WidgetTester tester,
  Size size,
  ThemeData? captureTheme,
) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = size;
  final container = ProviderContainer(
    overrides: [
      ...voiceAppTestOverrides(
        client: MockClient((_) async => http.Response('{}', 404)),
      ),
      voiceChatsClientProvider.overrideWithValue(_RequestsChatsClient()),
      activeProfileProvider.overrideWith(
        (_) async => const VoiceProfile(
          id: 'prof-test',
          accountId: 'acc-test',
          username: 'voiceuser',
          discriminator: '4242',
          displayName: 'Voice User',
          isPrimary: true,
        ),
      ),
      chatFoldersProvider.overrideWith(
        (_) async => const FolderListData(
          folders: [
            VoiceFolder(id: 'all', name: 'All chats', folderType: 'system'),
          ],
        ),
      ),
      quickAccessListProvider.overrideWith(
        (_) async => const QuickAccessListData(items: []),
      ),
      presenceProvider.overrideWith((_, _) => null),
      connectivityWatcherProvider.overrideWith((_) {}),
      deepLinkListenerProvider.overrideWith(_NoopDeepLinkListener.new),
      if (captureTheme != null)
        voiceMaterialThemeProvider.overrideWith((_) async => captureTheme),
    ],
  );
  addTearDown(container.dispose);
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: RepaintBoundary(
        key: _captureBoundaryKey,
        child: const VoiceApp(locale: Locale('en')),
      ),
    ),
  );
  await tester.pumpAndSettle();
  return container;
}

class _RequestsChatsClient extends FakeVoiceChatsClient {
  _RequestsChatsClient() : super();

  @override
  Future<ChatsApiResult<ChatListData>> listChats({
    required String authorization,
    String? cursor,
    int? pageSize,
    String? inbox,
    String? folderId,
  }) async {
    if (inbox == 'requests') {
      return const ChatsApiOk(
        ChatListData(
          items: [
            ChatListItem(
              chat: VoiceChat(
                id: 'request-row',
                type: 'CHAT_TYPE_DM',
                creatorProfileId: 'profile-rowan',
              ),
              unreadCount: 2,
              inbox: 'requests',
              isStranger: true,
              dmPeerProfileId: 'profile-rowan',
              dmPeerDisplayName: 'Rowan',
            ),
          ],
        ),
      );
    }
    return const ChatsApiOk(ChatListData(items: []));
  }

  @override
  Future<ChatsApiResult<FolderListData>> listFolders({
    required String authorization,
  }) async => const ChatsApiOk(
    FolderListData(
      folders: [
        VoiceFolder(id: 'all', name: 'All chats', folderType: 'system'),
      ],
    ),
  );

  @override
  Future<ChatsApiResult<QuickAccessListData>> listQuickAccess({
    required String authorization,
  }) async => const ChatsApiOk(QuickAccessListData(items: []));
}

class _NoopDeepLinkListener extends DeepLinkListener {
  _NoopDeepLinkListener(super.ref);

  @override
  Future<void> start() async {}
}

Future<ThemeData?> _themeForCapture(WidgetTester tester, bool capturing) async {
  if (!capturing) return null;
  if (!_captureFontsLoaded) {
    final loaded = await tester.runAsync(() async {
      final notoSans = FontLoader(VoiceTheme.fontFamily)
        ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
        ..addFont(rootBundle.load('assets/fonts/NotoSans-Medium.ttf'))
        ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'))
        ..addFont(rootBundle.load('assets/fonts/NotoSans-Bold.ttf'));
      await notoSans.load();

      final flutterRoot = Platform.environment['FLUTTER_ROOT'];
      if (flutterRoot == null || flutterRoot.isEmpty) {
        throw StateError('FLUTTER_ROOT is required to load Material Icons');
      }
      final iconFont = File(
        [
          flutterRoot,
          'bin',
          'cache',
          'artifacts',
          'material_fonts',
          'MaterialIcons-Regular.otf',
        ].join(Platform.pathSeparator),
      );
      final iconBytes = await iconFont.readAsBytes();
      final iconData = ByteData.sublistView(Uint8List.fromList(iconBytes));
      await (FontLoader(
        'MaterialIcons',
      )..addFont(Future<ByteData>.value(iconData))).load();
      return true;
    });
    if (loaded != true) {
      throw StateError('Capture font loading did not finish');
    }
    _captureFontsLoaded = true;
  }

  final theme = await tester.runAsync(() async {
    final catalog = await VoiceTokenCatalog.load();
    return VoiceTheme.build(
      catalog: catalog,
      mode: VoiceThemeMode.dark,
      profileAccent: catalog.profileAccentAt(0),
    );
  });
  if (theme == null) {
    throw StateError('Voice theme asset loading did not finish');
  }
  return theme;
}

Future<void> _captureIfRequested(WidgetTester tester, String filename) async {
  final directory = Platform.environment[_captureEnv];
  if (directory == null || directory.isEmpty) return;
  final boundary = tester.renderObject<RenderRepaintBoundary>(
    find.byKey(_captureBoundaryKey),
  );
  final bytes = await tester.runAsync(() async {
    final image = await boundary.toImage(pixelRatio: 1);
    try {
      final data = await image.toByteData(format: ImageByteFormat.png);
      if (data == null) throw StateError('Capture PNG encoding failed');
      final output = File('$directory${Platform.pathSeparator}$filename');
      await output.parent.create(recursive: true);
      final png = data.buffer.asUint8List();
      await output.writeAsBytes(png, flush: true);
      return png;
    } finally {
      image.dispose();
    }
  });
  if (bytes == null || bytes.isEmpty) {
    throw StateError('Capture did not finish');
  }
}
