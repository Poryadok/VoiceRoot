import 'dart:convert';
import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/routing/app_router.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/ui/chat/chat_archive_screen.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';

import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

Future<ThemeData> _theme(WidgetTester tester) async {
  const directory = String.fromEnvironment('VOICE_ARCHIVE_CAPTURE_DIR');
  if (directory.isEmpty) return voiceTestTheme();
  return (await tester.runAsync(() async {
    await (FontLoader(VoiceTheme.fontFamily)
          ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
          ..addFont(rootBundle.load('assets/fonts/NotoSans-Medium.ttf'))
          ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'))
          ..addFont(rootBundle.load('assets/fonts/NotoSans-Bold.ttf')))
        .load();
    final flutterRoot = Platform.environment['FLUTTER_ROOT'];
    if (flutterRoot == null) {
      throw StateError('FLUTTER_ROOT required for capture');
    }
    final bytes = await File(
      '$flutterRoot/bin/cache/artifacts/material_fonts/MaterialIcons-Regular.otf',
    ).readAsBytes();
    await (FontLoader(
      'MaterialIcons',
    )..addFont(Future.value(ByteData.sublistView(bytes)))).load();
    final catalog = await VoiceTokenCatalog.load();
    return VoiceTheme.build(
      catalog: catalog,
      mode: VoiceThemeMode.light,
      profileAccent: catalog.profileAccentAt(0),
    );
  }))!;
}

Future<void> _capture(WidgetTester tester, String name) async {
  const directory = String.fromEnvironment('VOICE_ARCHIVE_CAPTURE_DIR');
  if (directory.isEmpty) return;
  final boundary = tester.renderObject<RenderRepaintBoundary>(
    find.byKey(const Key('archive_capture')),
  );
  await tester.runAsync(() async {
    final image = await boundary.toImage();
    final bytes = (await image.toByteData(format: ui.ImageByteFormat.png))!;
    await Directory(directory).create(recursive: true);
    await File('$directory/$name.png').writeAsBytes(bytes.buffer.asUint8List());
    image.dispose();
  });
}

class _MemoryAuthStorage implements AuthSessionStorage {
  AuthSession? _session;

  @override
  Future<void> clear() async => _session = null;

  @override
  Future<AuthSession?> read() async => _session;

  @override
  Future<void> write(AuthSession session) async => _session = session;
}

class _ArchiveRealtime extends RealtimeHub {
  _ArchiveRealtime(super.ref);
  final reads = <String>[];
  @override
  Future<void> ensureConnected() async {}
  @override
  void markRead(String chatId, String messageId) =>
      reads.add('$chatId/$messageId');
}

void main() {
  testWidgets('ChatArchiveScreen lists archived chats', (tester) async {
    final theme = await _theme(tester);
    tester.view.physicalSize = const Size(900, 560);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    final storage = _MemoryAuthStorage();
    const session = AuthSession(
      accessToken: 'token',
      refreshToken: 'refresh',
      expiresInSeconds: 900,
      accountId: 'account-1',
      activeProfileId: 'profile-1',
    );
    await storage.write(session);

    var readAttempts = 0;
    var unarchiveAttempts = 0;
    final mock = MockClient((req) async {
      if (req.url.path.endsWith('/archive') && req.method == 'POST') {
        unarchiveAttempts++;
        return http.Response('{}', 200);
      }
      if (req.url.path == '/api/v1/messages') {
        return http.Response(
          jsonEncode({
            'message_list': {
              'messages': [
                {
                  'id': 'latest-message',
                  'chat_id': 'chat-archived',
                  'sender_profile_id': 'profile-2',
                  'content': 'Old note',
                },
              ],
            },
          }),
          200,
        );
      }
      if (req.url.path == '/api/v1/messages/read') {
        expect(jsonDecode(req.body)['last_read_message_id'], 'latest-message');
        readAttempts++;
        return http.Response('{}', readAttempts == 1 ? 503 : 200);
      }
      if (req.url.path == '/api/v1/chats' &&
          req.url.queryParameters['inbox'] == 'archive') {
        return http.Response(
          jsonEncode({
            'chat_list': {
              'items': [
                {
                  'chat': {
                    'id': 'chat-archived',
                    'type': 'CHAT_TYPE_DM',
                    'creator_profile_id': 'profile-2',
                    'name': 'Design Crit',
                  },
                  'last_message_preview': 'Old note',
                  'unread_count': readAttempts >= 2 ? 0 : 2,
                },
              ],
            },
          }),
          200,
        );
      }
      return http.Response('not found', 404);
    });

    late _ArchiveRealtime hub;
    final container = ProviderContainer(
      overrides: [
        realtimeHubProvider.overrideWith((ref) => hub = _ArchiveRealtime(ref)),
        ...voiceThemeTestOverrides(),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        httpClientProvider.overrideWithValue(mock),
        authSessionStorageProvider.overrideWithValue(storage),
      ],
    );
    var disposed = false;
    void disposeContainer() {
      if (disposed) return;
      disposed = true;
      container.dispose();
    }

    addTearDown(disposeContainer);
    await container.read(authControllerProvider.notifier).applySession(session);

    final router = createVoiceGoRouter(
      shellBuilder: (context, state) =>
          const Scaffold(body: Text('home shell')),
    );

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: RepaintBoundary(
          key: const Key('archive_capture'),
          child: MaterialApp.router(
            debugShowCheckedModeBanner: false,
            theme: theme,
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            routerConfig: router,
          ),
        ),
      ),
    );
    router.go(VoiceAppRoutes.chatArchive);
    await tester.pumpAndSettle();

    expect(find.byKey(ChatArchiveScreen.screenKey), findsOneWidget);
    expect(find.text('Archived chats'), findsOneWidget);
    expect(
      find.byKey(ChatArchiveScreen.tileKey('chat-archived')),
      findsOneWidget,
    );
    expect(find.text('Old note'), findsOneWidget);
    expect(find.text('2'), findsOneWidget);
    expect(
      find.byKey(const Key('chat_archive_detail_placeholder')),
      findsOneWidget,
    );
    expect(
      tester.getSize(find.byKey(const Key('chat_archive_list_pane'))).width,
      320,
    );
    expect(find.text('Unarchive'), findsNothing);
    await _capture(tester, 'archive-h');
    await tester.longPress(
      find.byKey(ChatArchiveScreen.tileKey('chat-archived')),
    );
    await tester.pumpAndSettle();
    expect(find.text('Unarchive'), findsOneWidget);
    await _capture(tester, 'archive-h-context');
    expect(find.text('Mark as read'), findsOneWidget);
    await tester.tap(find.text('Mark as read'));
    await tester.pumpAndSettle();
    expect(readAttempts, 1);
    expect(find.text('2'), findsOneWidget);
    expect(find.text('Could not complete this action.'), findsOneWidget);
    await tester.longPress(
      find.byKey(ChatArchiveScreen.tileKey('chat-archived')),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('Mark as read'));
    await tester.pumpAndSettle();
    expect(readAttempts, 2);
    expect(hub.reads, [
      'chat-archived/latest-message',
    ], reason: 'Only durable REST success fans out the read position');
    expect(find.text('2'), findsNothing);
    final tile = find.byKey(ChatArchiveScreen.tileKey('chat-archived'));
    Focus.of(tester.element(tile)).requestFocus();
    await tester.pump();
    await tester.sendKeyDownEvent(LogicalKeyboardKey.shiftLeft);
    await tester.sendKeyEvent(LogicalKeyboardKey.f10);
    await tester.sendKeyUpEvent(LogicalKeyboardKey.shiftLeft);
    await tester.pumpAndSettle();
    expect(find.text('Unarchive'), findsOneWidget);
    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    await tester.pumpAndSettle();
    expect(find.text('Unarchive'), findsNothing);
    await tester.tap(tile, buttons: kSecondaryMouseButton);
    await tester.pumpAndSettle();
    expect(find.text('Unarchive'), findsOneWidget);
    await container
        .read(authControllerProvider.notifier)
        .applySession(
          const AuthSession(
            accessToken: 'token-other',
            refreshToken: 'refresh-other',
            expiresInSeconds: 900,
            accountId: 'account-1',
            activeProfileId: 'profile-other',
          ),
        );
    await tester.pumpAndSettle();
    await tester.tap(find.text('Unarchive'));
    await tester.pumpAndSettle();
    expect(
      unarchiveAttempts,
      0,
      reason:
          'A menu opened under another profile must not mutate the new profile',
    );
    await tester.tap(find.byType(BackButton));
    await tester.pumpAndSettle();
    expect(find.text('home shell'), findsOneWidget);
    await tester.pumpWidget(const SizedBox.shrink());
    await hub.dispose();
    disposeContainer();
  });

  testWidgets('mobile swipe unarchives an archived chat', (tester) async {
    final theme = await _theme(tester);
    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    final storage = _MemoryAuthStorage();
    const session = AuthSession(
      accessToken: 'token',
      refreshToken: 'refresh',
      expiresInSeconds: 900,
      accountId: 'account-1',
      activeProfileId: 'profile-1',
    );
    await storage.write(session);
    var unarchiveAttempts = 0;
    final unarchived = <String>[];
    final mock = MockClient((req) async {
      if (req.url.path == '/api/v1/chats' &&
          req.url.queryParameters['inbox'] == 'archive') {
        return http.Response(
          jsonEncode({
            'chat_list': {
              'items': [
                {
                  'chat': {
                    'id': 'design-crit',
                    'type': 'CHAT_TYPE_GROUP',
                    'name': 'Design Crit',
                  },
                  'last_message_preview': 'Final design review',
                  'unread_count': 2,
                },
                {
                  'chat': {
                    'id': 'chat-archived',
                    'type': 'CHAT_TYPE_GROUP',
                    'name': 'Voice HQ',
                  },
                  'last_message_preview': 'Photo',
                },
                {
                  'chat': {
                    'id': 'lobby-12',
                    'type': 'CHAT_TYPE_GROUP',
                    'name': 'Lobby #12',
                  },
                  'last_message_preview': 'Video message',
                },
              ],
            },
          }),
          200,
        );
      }
      if (req.url.path == '/api/v1/chats/chat-archived/archive' &&
          req.method == 'POST') {
        if (jsonDecode(req.body)['archived'] == false) {
          unarchiveAttempts++;
          if (unarchiveAttempts == 1) {
            return http.Response(
              jsonEncode({
                'code': 'ERROR_CODE_UNAVAILABLE',
                'message': 'private_archive_backend_detail',
              }),
              503,
              headers: {'content-type': 'application/json'},
            );
          }
          unarchived.add('chat-archived');
        }
        return http.Response('{}', 200);
      }
      return http.Response('not found', 404);
    });
    final container = ProviderContainer(
      overrides: [
        ...voiceThemeTestOverrides(),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        httpClientProvider.overrideWithValue(mock),
        authSessionStorageProvider.overrideWithValue(storage),
      ],
    );
    addTearDown(container.dispose);
    await container.read(authControllerProvider.notifier).applySession(session);
    final router = createVoiceGoRouter(
      shellBuilder: (context, state) =>
          const Scaffold(body: Text('home shell')),
    );
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: RepaintBoundary(
          key: const Key('archive_capture'),
          child: MaterialApp.router(
            debugShowCheckedModeBanner: false,
            theme: theme.copyWith(platform: TargetPlatform.android),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            routerConfig: router,
          ),
        ),
      ),
    );
    router.go(VoiceAppRoutes.chatArchive);
    await tester.pumpAndSettle();

    final row = find.byKey(ChatArchiveScreen.tileKey('chat-archived'));
    expect(row, findsOneWidget);
    expect(
      find.byKey(const Key('chat_archive_detail_placeholder')),
      findsNothing,
    );
    expect(
      tester.getSize(find.byKey(const Key('chat_archive_list_pane'))).width,
      390,
    );
    await _capture(tester, 'archive-v');
    final originalRowPosition = tester.getTopLeft(row);
    final swipe = await tester.startGesture(tester.getCenter(row));
    await swipe.moveBy(const Offset(-20, 0));
    await tester.pump();
    await swipe.moveBy(const Offset(-100, 0));
    await tester.pump(const Duration(milliseconds: 150));
    expect(tester.getTopLeft(row).dx, lessThan(originalRowPosition.dx));
    await _capture(tester, 'archive-v-swipe');
    await swipe.up();
    await tester.pumpAndSettle();
    await tester.drag(row, const Offset(-600, 0));
    await tester.pumpAndSettle();

    expect(row, findsOneWidget);
    expect(unarchiveAttempts, 1);
    expect(find.text('Could not complete this action.'), findsOneWidget);
    expect(find.text('private_archive_backend_detail'), findsNothing);
    expect(find.text('private_archive_backend_detail'), findsNothing);
    await tester.tap(find.text('Could not complete this action.'));
    await tester.pumpAndSettle();

    await tester.drag(row, const Offset(-600, 0));
    await tester.pumpAndSettle();

    expect(unarchiveAttempts, 2);
    expect(unarchived, ['chat-archived']);
    expect(row, findsNothing);
  });
}
