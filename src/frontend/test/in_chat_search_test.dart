import 'dart:io';
import 'dart:convert';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/shell_providers.dart';
import 'package:voice_frontend/settings/voice_input_settings.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';
import 'package:voice_frontend/ui/shell/side_panel.dart';
import 'package:voice_frontend/ui/search/in_chat_search.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

const _e2eChatId = 'e2e-chat-1';
const _captureDirectoryKey = 'VOICE_CHAT_INFO_SEARCH_CAPTURE_DIR';
const _captureBoundaryKey = Key('chat_info_search_capture');

var _captureFontsLoaded = false;

class _E2eChatListController extends ChatListController {
  _E2eChatListController(super.ref) : super() {
    state = const ChatListState(
      items: [
        ChatListItem(
          chat: VoiceChat(
            id: _e2eChatId,
            type: 'CHAT_TYPE_DM',
            creatorProfileId: 'peer-1',
            e2eEnabled: true,
          ),
        ),
      ],
    );
  }

  @override
  Future<void> loadInitial() async {}

  @override
  Future<void> loadMore() async {}
}

class _E2eChatRoomController extends ChatRoomController {
  _E2eChatRoomController(super.ref, super.chatId) : super() {
    state = const ChatRoomState(
      messages: [
        VoiceMessage(
          id: 'msg-local-1',
          chatId: _e2eChatId,
          senderProfileId: 'peer-1',
          content: 'loaded local secret needle',
          isE2e: true,
        ),
      ],
    );
  }

  @override
  Future<void> loadInitial() async {}
}

class _FakeRealtimeHub extends RealtimeHub {
  _FakeRealtimeHub(super.ref);

  @override
  Stream<RealtimeFrame> get events => const Stream.empty();

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}
}

Widget e2eInChatSearchTestApp({
  required http.Client client,
  required String chatId,
  Widget? home,
  ShellSidePanel sidePanel = ShellSidePanel.none,
  void Function(ProviderContainer)? onContainer,
  ThemeData? theme,
}) {
  return ProviderScope(
    overrides: [
      ...voiceThemeTestOverrides(),
      profileAccentStorageProvider.overrideWithValue(testProfileAccentStorage),
      authSessionStorageProvider.overrideWithValue(
        InMemoryAuthSessionStorage(),
      ),
      authControllerProvider.overrideWith(authenticatedAuthController),
      gatewayConfigProvider.overrideWithValue(
        const GatewayConfig(baseUrl: 'http://api.test'),
      ),
      httpClientProvider.overrideWithValue(client),
      chatListControllerProvider.overrideWith(_E2eChatListController.new),
      voiceInputSettingsProvider.overrideWith(
        TestVoiceInputSettingsNotifier.new,
      ),
      realtimeHubProvider.overrideWith(_FakeRealtimeHub.new),
      selectedChatIdProvider.overrideWith((ref) => chatId),
      shellSidePanelProvider.overrideWith((ref) => sidePanel),
      chatRoomControllerProvider(
        chatId,
      ).overrideWith((ref) => _E2eChatRoomController(ref, chatId)),
    ],
    child: MaterialApp(
      theme: theme ?? voiceTestTheme(),
      locale: const Locale('en'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: Builder(
        builder: (context) {
          onContainer?.call(ProviderScope.containerOf(context));
          return RepaintBoundary(
            key: _captureBoundaryKey,
            child: home ?? Scaffold(body: InChatSearch(chatId: chatId)),
          );
        },
      ),
    ),
  );
}

Future<ThemeData?> _loadCaptureTheme(WidgetTester tester) async {
  final directory = Platform.environment[_captureDirectoryKey];
  if (directory == null || directory.isEmpty) return null;

  if (!_captureFontsLoaded) {
    final loaded = await tester.runAsync(() async {
      final noto = FontLoader(VoiceTheme.fontFamily)
        ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
        ..addFont(rootBundle.load('assets/fonts/NotoSans-Medium.ttf'))
        ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'))
        ..addFont(rootBundle.load('assets/fonts/NotoSans-Bold.ttf'));
      await noto.load();
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
      final iconData = ByteData.sublistView(
        Uint8List.fromList(await iconFont.readAsBytes()),
      );
      await (FontLoader(
        'MaterialIcons',
      )..addFont(Future<ByteData>.value(iconData))).load();
      return true;
    });
    if (loaded != true) throw StateError('Production font loading failed');
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
  if (theme == null) throw StateError('Production tokens did not load');
  return theme;
}

Future<void> _captureSearchState(WidgetTester tester, String filename) async {
  final directory = Platform.environment[_captureDirectoryKey];
  if (directory == null || directory.isEmpty) return;
  final boundary = tester.renderObject<RenderRepaintBoundary>(
    find.byKey(_captureBoundaryKey),
  );
  final captured = await tester.runAsync(() async {
    final image = await boundary.toImage(
      pixelRatio: tester.view.devicePixelRatio,
    );
    try {
      final bytes = await image.toByteData(format: ui.ImageByteFormat.png);
      if (bytes == null) throw StateError('PNG encoding returned null');
      final output = File('$directory${Platform.pathSeparator}$filename');
      await output.parent.create(recursive: true);
      await output.writeAsBytes(bytes.buffer.asUint8List(), flush: true);
      return true;
    } finally {
      image.dispose();
    }
  });
  if (captured != true) throw StateError('Chat search capture failed');
}

void main() {
  Widget inChatSearchTestApp({
    required http.Client client,
    required String chatId,
  }) {
    return ProviderScope(
      overrides: [
        ...voiceThemeTestOverrides(),
        profileAccentStorageProvider.overrideWithValue(
          testProfileAccentStorage,
        ),
        authSessionStorageProvider.overrideWithValue(
          InMemoryAuthSessionStorage(),
        ),
        authControllerProvider.overrideWith(authenticatedAuthController),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        httpClientProvider.overrideWithValue(client),
      ],
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(body: InChatSearch(chatId: chatId)),
      ),
    );
  }

  testWidgets('InChatSearch shows field and navigation controls', (
    tester,
  ) async {
    await tester.pumpWidget(
      inChatSearchTestApp(
        chatId: 'chat-1',
        client: MockClient((_) async => http.Response('{}', 200)),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(InChatSearch.panelKey), findsOneWidget);
    expect(find.byKey(InChatSearch.searchFieldKey), findsOneWidget);
    expect(find.byKey(InChatSearch.prevHitKey), findsOneWidget);
    expect(find.byKey(InChatSearch.nextHitKey), findsOneWidget);
  });

  testWidgets('in-chat search queries scoped chat_id', (tester) async {
    String? chatIdParam;
    await tester.pumpWidget(
      inChatSearchTestApp(
        chatId: 'chat-scoped',
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/search/in-chat') {
            chatIdParam = req.url.queryParameters['chat_id'];
            return http.Response(
              jsonEncode({
                'searchResults': {
                  'hits': [
                    {
                      'messageId': 'msg-1',
                      'snippet': 'alpha match',
                      'score': 1.0,
                    },
                    {
                      'messageId': 'msg-2',
                      'snippet': 'beta match',
                      'score': 0.9,
                    },
                  ],
                },
              }),
              200,
            );
          }
          return http.Response('not found', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(find.byKey(InChatSearch.searchFieldKey), 'match');
    await tester.pump(const Duration(milliseconds: 350));
    await tester.pumpAndSettle();

    expect(chatIdParam, 'chat-scoped');
    expect(find.textContaining('alpha match'), findsOneWidget);
  });

  testWidgets('in-chat search up/down navigates between hits', (tester) async {
    await tester.pumpWidget(
      inChatSearchTestApp(
        chatId: 'chat-1',
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/search/in-chat') {
            return http.Response(
              jsonEncode({
                'searchResults': {
                  'hits': [
                    {
                      'messageId': 'msg-1',
                      'snippet': 'first hit',
                      'score': 1.0,
                    },
                    {
                      'messageId': 'msg-2',
                      'snippet': 'second hit',
                      'score': 0.9,
                    },
                  ],
                },
              }),
              200,
            );
          }
          return http.Response('not found', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(find.byKey(InChatSearch.searchFieldKey), 'hit');
    await tester.pump(const Duration(milliseconds: 350));
    await tester.pumpAndSettle();

    expect(find.byKey(InChatSearch.activeHitKey('msg-1')), findsOneWidget);

    await tester.tap(find.byKey(InChatSearch.nextHitKey));
    await tester.pumpAndSettle();
    expect(find.byKey(InChatSearch.activeHitKey('msg-2')), findsOneWidget);

    await tester.tap(find.byKey(InChatSearch.prevHitKey));
    await tester.pumpAndSettle();
    expect(find.byKey(InChatSearch.activeHitKey('msg-1')), findsOneWidget);
  });

  testWidgets('in-chat search highlights snippet match', (tester) async {
    await tester.pumpWidget(
      inChatSearchTestApp(
        chatId: 'chat-1',
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/search/in-chat') {
            return http.Response(
              jsonEncode({
                'searchResults': {
                  'hits': [
                    {
                      'messageId': 'msg-1',
                      'snippet': 'find the <mark>needle</mark> here',
                      'score': 1.0,
                    },
                  ],
                },
              }),
              200,
            );
          }
          return http.Response('not found', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(find.byKey(InChatSearch.searchFieldKey), 'needle');
    await tester.pump(const Duration(milliseconds: 350));
    await tester.pumpAndSettle();

    expect(find.byKey(InChatSearch.highlightKey), findsOneWidget);
  });

  testWidgets('E2E chat shows local-only search banner', (tester) async {
    await tester.pumpWidget(
      e2eInChatSearchTestApp(
        chatId: _e2eChatId,
        client: MockClient((_) async => http.Response('not found', 404)),
      ),
    );
    await tester.pumpAndSettle();

    expect(
      find.textContaining('loaded history on this device', findRichText: true),
      findsOneWidget,
    );
  });

  testWidgets('E2E in-chat search uses local cache without server search', (
    tester,
  ) async {
    var serverSearchCalls = 0;
    await tester.pumpWidget(
      e2eInChatSearchTestApp(
        chatId: _e2eChatId,
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/search/in-chat') {
            serverSearchCalls++;
          }
          return http.Response('not found', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(find.byKey(InChatSearch.searchFieldKey), 'needle');
    await tester.pump(const Duration(milliseconds: 350));
    await tester.pumpAndSettle();

    expect(serverSearchCalls, 0);
    expect(find.textContaining('loaded local secret needle'), findsOneWidget);
    expect(
      find.byKey(InChatSearch.activeHitKey('msg-local-1')),
      findsOneWidget,
    );
  });

  testWidgets(
    'in-chat search expands inline in header instead of bottom sheet',
    (tester) async {
      await tester.pumpWidget(
        chatRoomInChatSearchTestApp(
          client: MockClient((_) async => http.Response('{}', 200)),
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(ChatRoomPanel.inChatSearchKey));
      await tester.pumpAndSettle();

      expect(find.byType(BottomSheet), findsNothing);
      expect(
        find.byKey(const Key('in_chat_search_inline_header')),
        findsOneWidget,
      );
      expect(find.byKey(InChatSearch.searchFieldKey), findsOneWidget);
    },
  );

  testWidgets('Chat Info opens existing search and closing returns to room', (
    tester,
  ) async {
    final captureTheme = await _loadCaptureTheme(tester);
    tester.view.physicalSize = const Size(1280, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(
      e2eInChatSearchTestApp(
        chatId: _e2eChatId,
        theme: captureTheme,
        sidePanel: ShellSidePanel.chatInfo,
        client: MockClient((_) async => http.Response('not found', 404)),
        home: LayoutBuilder(
          builder: (context, constraints) => Scaffold(
            body: Row(
              children: [
                const Expanded(child: ChatRoomPanel(chatId: _e2eChatId)),
                if (constraints.maxWidth >= 600)
                  SizedBox(width: 320, child: SidePanelHost()),
              ],
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Search messages'), findsOneWidget);
    final semantics = tester.ensureSemantics();
    expect(find.bySemanticsLabel('Search messages'), findsOneWidget);
    await tester.tap(find.text('Search messages'));
    await tester.pumpAndSettle();

    expect(find.byKey(InChatSearch.searchFieldKey), findsOneWidget);
    expect(
      tester
          .widget<TextField>(find.byKey(InChatSearch.searchFieldKey))
          .focusNode!
          .hasFocus,
      isTrue,
    );
    await tester.enterText(find.byKey(InChatSearch.searchFieldKey), 'needle');
    await tester.pump(const Duration(milliseconds: 350));
    await tester.pumpAndSettle();

    expect(find.textContaining('loaded local secret needle'), findsOneWidget);
    expect(
      find.byKey(InChatSearch.activeHitKey('msg-local-1')),
      findsOneWidget,
    );
    await _captureSearchState(tester, 'chat-info-search-h.png');

    await tester.tap(find.byKey(ChatRoomPanel.inChatSearchKey));
    await tester.pumpAndSettle();

    expect(find.byKey(InChatSearch.searchFieldKey), findsNothing);
    expect(
      tester
          .widget<IconButton>(find.byKey(ChatRoomPanel.inChatSearchKey))
          .focusNode!
          .hasFocus,
      isTrue,
    );
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.pumpAndSettle();
    expect(find.byKey(InChatSearch.searchFieldKey), findsOneWidget);
    expect(
      tester
          .widget<TextField>(find.byKey(InChatSearch.searchFieldKey))
          .focusNode!
          .hasFocus,
      isTrue,
    );
    tester.view.physicalSize = const Size(390, 844);
    await tester.pumpAndSettle();
    expect(find.byKey(InChatSearch.searchFieldKey), findsOneWidget);
    await tester.enterText(find.byKey(InChatSearch.searchFieldKey), 'needle');
    await tester.pump(const Duration(milliseconds: 350));
    await tester.pumpAndSettle();
    expect(find.textContaining('loaded local secret needle'), findsOneWidget);
    expect(
      tester.getSize(find.byKey(InChatSearch.searchFieldKey)).width,
      lessThan(390),
    );
    await _captureSearchState(tester, 'chat-info-search-v.png');
    semantics.dispose();
  });

  testWidgets('Chat Info search ignores stale chat and profile requests', (
    tester,
  ) async {
    late ProviderContainer container;
    await tester.pumpWidget(
      e2eInChatSearchTestApp(
        chatId: _e2eChatId,
        sidePanel: ShellSidePanel.chatInfo,
        onContainer: (value) => container = value,
        client: MockClient((_) async => http.Response('not found', 404)),
        home: Scaffold(
          body: Row(
            children: [
              const Expanded(child: ChatRoomPanel(chatId: _e2eChatId)),
              SizedBox(width: 320, child: SidePanelHost()),
            ],
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    final viewerProfileId = container
        .read(authControllerProvider)
        .activeProfileId!;
    container.read(selectedChatIdProvider.notifier).state = 'another-chat';
    container
        .read(chatInfoSearchRequestProvider.notifier)
        .state = ChatInfoSearchRequest(
      chatId: _e2eChatId,
      viewerProfileId: viewerProfileId,
    );
    await tester.pumpAndSettle();

    expect(find.byKey(InChatSearch.searchFieldKey), findsNothing);
    expect(container.read(chatInfoSearchRequestProvider), isNull);

    container.read(selectedChatIdProvider.notifier).state = _e2eChatId;
    container
        .read(chatInfoSearchRequestProvider.notifier)
        .state = ChatInfoSearchRequest(
      chatId: _e2eChatId,
      viewerProfileId: 'different-profile',
    );
    await tester.pumpAndSettle();

    expect(find.byKey(InChatSearch.searchFieldKey), findsNothing);
    expect(container.read(chatInfoSearchRequestProvider), isNull);
  });

  testWidgets(
    'unconsumed Chat Info search request expires with room disposal',
    (tester) async {
      late ProviderContainer container;
      var showRoom = true;
      late StateSetter updateHome;
      Widget roomAndInfo() => Scaffold(
        body: Row(
          children: [
            const Expanded(child: ChatRoomPanel(chatId: _e2eChatId)),
            SizedBox(width: 320, child: SidePanelHost()),
          ],
        ),
      );

      await tester.pumpWidget(
        e2eInChatSearchTestApp(
          chatId: _e2eChatId,
          sidePanel: ShellSidePanel.chatInfo,
          onContainer: (value) => container = value,
          client: MockClient((_) async => http.Response('not found', 404)),
          home: StatefulBuilder(
            builder: (context, setState) {
              updateHome = setState;
              return showRoom ? roomAndInfo() : const SizedBox.shrink();
            },
          ),
        ),
      );
      await tester.pumpAndSettle();

      updateHome(() => showRoom = false);
      await tester.pumpAndSettle();
      final viewerProfileId = container
          .read(authControllerProvider)
          .activeProfileId!;
      container
          .read(chatInfoSearchRequestProvider.notifier)
          .state = ChatInfoSearchRequest(
        chatId: _e2eChatId,
        viewerProfileId: viewerProfileId,
      );
      await tester.pump();
      await tester.pump(const Duration(seconds: 1));

      expect(container.read(chatInfoSearchRequestProvider), isNull);
      updateHome(() => showRoom = true);
      await tester.pumpAndSettle();
      expect(find.byKey(InChatSearch.searchFieldKey), findsNothing);
    },
  );

  testWidgets('in-chat search shows error when API fails', (tester) async {
    await tester.pumpWidget(
      chatRoomInChatSearchTestApp(
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/search/in-chat') {
            return http.Response(
              jsonEncode({
                'error_code': 'internal',
                'message': 'search failed',
              }),
              500,
            );
          }
          if (req.url.path == '/api/v1/messages') {
            return http.Response(
              jsonEncode({
                'message_list': {'messages': []},
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/messages/read') {
            return http.Response('{}', 200);
          }
          return http.Response('{}', 200);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(ChatRoomPanel.inChatSearchKey));
    await tester.pumpAndSettle();

    await tester.enterText(find.byKey(InChatSearch.searchFieldKey), 'needle');
    await tester.pump(const Duration(milliseconds: 350));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('in_chat_search_error')), findsOneWidget);
    expect(find.textContaining('Could not search this chat'), findsOneWidget);
  });
}

Widget chatRoomInChatSearchTestApp({required http.Client client}) {
  return ProviderScope(
    overrides: [
      ...voiceThemeTestOverrides(),
      profileAccentStorageProvider.overrideWithValue(testProfileAccentStorage),
      authSessionStorageProvider.overrideWithValue(
        InMemoryAuthSessionStorage(),
      ),
      authControllerProvider.overrideWith(authenticatedAuthController),
      gatewayConfigProvider.overrideWithValue(
        const GatewayConfig(baseUrl: 'http://api.test'),
      ),
      httpClientProvider.overrideWithValue(client),
      realtimeHubProvider.overrideWith(_FakeRealtimeHub.new),
      chatListControllerProvider.overrideWith(
        _ChatRoomSearchListController.new,
      ),
      chatRoomControllerProvider('chat-search').overrideWith(
        (ref) => _ChatRoomSearchRoomController(ref, 'chat-search'),
      ),
    ],
    child: MaterialApp(
      theme: voiceTestTheme(),
      locale: const Locale('en'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: const Scaffold(body: ChatRoomPanel(chatId: 'chat-search')),
    ),
  );
}

class _ChatRoomSearchListController extends ChatListController {
  _ChatRoomSearchListController(super.ref) : super() {
    state = const ChatListState(
      items: [
        ChatListItem(
          chat: VoiceChat(
            id: 'chat-search',
            type: 'CHAT_TYPE_DM',
            creatorProfileId: 'peer-1',
          ),
        ),
      ],
    );
  }

  @override
  Future<void> loadInitial() async {}

  @override
  Future<void> loadMore() async {}
}

class _ChatRoomSearchRoomController extends ChatRoomController {
  _ChatRoomSearchRoomController(super.ref, super.chatId) : super() {
    state = const ChatRoomState(messages: []);
  }
}
