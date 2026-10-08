import 'dart:async';
import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/routing/app_router.dart';
import 'package:voice_frontend/routing/deep_link_parser.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/deep_link_navigation.dart';
import 'package:voice_frontend/state/shared_media_providers.dart';
import 'package:voice_frontend/state/shell_providers.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

const _captureBoundaryKey = ValueKey('deep-link-error-capture');
var _captureFontsLoaded = false;

void main() {
  test('applyDeepLinkNavigation selects chat and message', () async {
    final container = ProviderContainer(
      overrides: voiceAppTestOverrides(
        client: MockClient((_) async => throw UnimplementedError()),
      ),
    );
    addTearDown(container.dispose);

    container.read(chatListControllerProvider);
    await pumpEventQueue();

    await container
        .read(deepLinkNavigatorProvider)
        .apply(parseDeepLinkUrl('https://voice.gg/ch/chat-1/m/msg-1'));
    await pumpEventQueue();

    expect(container.read(selectedChatIdProvider), 'chat-1');
    expect(container.read(pendingChatMessageScrollProvider('chat-1')), 'msg-1');
    expect(
      container.read(pendingChatMessageHighlightProvider('chat-1')),
      'msg-1',
    );
    expect(container.read(navigationSectionProvider), NavigationSection.chats);
  });

  test('applyDeepLinkNavigation selects space chat', () async {
    final container = ProviderContainer(
      overrides: voiceAppTestOverrides(
        client: MockClient((_) async => throw UnimplementedError()),
      ),
    );
    addTearDown(container.dispose);

    container.read(chatListControllerProvider);
    await pumpEventQueue();

    await container
        .read(deepLinkNavigatorProvider)
        .apply(parseDeepLinkUrl('https://voice.gg/s/space-1/c/chat-2'));
    await pumpEventQueue();

    expect(container.read(selectedSpaceIdProvider), 'space-1');
    expect(container.read(selectedChatIdProvider), 'chat-2');
  });

  for (final statusCode in [403, 404]) {
    testWidgets(
      'a $statusCode resolve failure shows safe copy and does not apply the target',
      (tester) async {
        final mockClient = MockClient((request) async {
          expect(request.url.path, '/api/v1/links/resolve');
          return http.Response(
            '{"code":"PRIVATE_DETAIL","message":"private resolver detail"}',
            statusCode,
            headers: const {'content-type': 'application/json'},
          );
        });
        final container = ProviderContainer(
          overrides: voiceAppTestOverrides(client: mockClient),
        );
        addTearDown(container.dispose);

        final router = createVoiceGoRouter(
          shellBuilder: (context, state) => Consumer(
            builder: (context, ref, _) => Scaffold(
              body: Column(
                children: [
                  const Text('home shell'),
                  TextButton(
                    onPressed: () => resolveAndNavigateDeepLink(
                      ref,
                      parseDeepLinkUrl('https://voice.gg/ch/denied-chat'),
                    ),
                    child: const Text('resolve'),
                  ),
                ],
              ),
            ),
          ),
        );
        await tester.pumpWidget(
          UncontrolledProviderScope(
            container: container,
            child: MaterialApp.router(
              locale: const Locale('en'),
              localizationsDelegates: AppLocalizations.localizationsDelegates,
              supportedLocales: AppLocalizations.supportedLocales,
              routerConfig: router,
            ),
          ),
        );

        await tester.tap(find.text('resolve'));
        await tester.pumpAndSettle();

        expect(container.read(selectedChatIdProvider), isNull);
        expect(find.text('private resolver detail'), findsNothing);
        expect(
          find.text(statusCode == 403 ? 'Access denied' : 'Not found'),
          findsOneWidget,
        );
        expect(find.text('Go to Home'), findsOneWidget);
        expect(find.bySemanticsLabel('Go to Home'), findsOneWidget);

        await tester.tap(find.text('Go to Home'));
        await tester.pumpAndSettle();

        expect(find.text('home shell'), findsOneWidget);
        expect(find.text('Access denied'), findsNothing);
        expect(find.text('Not found'), findsNothing);
      },
    );
  }

  testWidgets('non-403/404 resolve failures retain existing fallback routing', (
    tester,
  ) async {
    final mockClient = MockClient(
      (_) async => http.Response(
        '{"code":"private","message":"private detail"}',
        500,
        headers: const {'content-type': 'application/json'},
      ),
    );
    final container = ProviderContainer(
      overrides: voiceAppTestOverrides(client: mockClient),
    );
    addTearDown(container.dispose);
    final router = _createResolverRouter();
    await tester.pumpWidget(_resolverApp(container: container, router: router));

    await tester.tap(find.text('resolve'));
    await tester.pumpAndSettle();

    expect(container.read(selectedChatIdProvider), 'denied-chat');
    expect(find.text('private detail'), findsNothing);
    expect(find.text('Access denied'), findsNothing);
    expect(find.text('Not found'), findsNothing);
  });

  testWidgets('a successfully resolved space keeps resolved navigation', (
    tester,
  ) async {
    final mockClient = MockClient(
      (_) async => http.Response(
        '{"kind":"space","space_id":"resolved-space"}',
        200,
        headers: const {'content-type': 'application/json'},
      ),
    );
    final container = ProviderContainer(
      overrides: voiceAppTestOverrides(client: mockClient),
    );
    addTearDown(container.dispose);
    final router = _createResolverRouter();
    await tester.pumpWidget(_resolverApp(container: container, router: router));

    await tester.tap(find.text('resolve'));
    await tester.pumpAndSettle();

    expect(container.read(selectedSpaceIdProvider), 'resolved-space');
    expect(find.text('Access denied'), findsNothing);
    expect(find.text('Not found'), findsNothing);
  });

  testWidgets('a resolve result is ignored after the caller is disposed', (
    tester,
  ) async {
    final pendingResponse = Completer<http.Response>();
    final mockClient = MockClient((_) => pendingResponse.future);
    final container = ProviderContainer(
      overrides: voiceAppTestOverrides(client: mockClient),
    );
    addTearDown(container.dispose);
    final router = _createResolverRouter();
    await tester.pumpWidget(_resolverApp(container: container, router: router));

    await tester.tap(find.text('resolve'));
    await tester.pump();
    await tester.pumpWidget(const SizedBox.shrink());
    pendingResponse.complete(
      http.Response(
        '{"code":"PRIVATE_DETAIL","message":"private resolver detail"}',
        404,
        headers: const {'content-type': 'application/json'},
      ),
    );
    await tester.pumpAndSettle();

    expect(container.read(selectedChatIdProvider), isNull);
    expect(find.byKey(const ValueKey('deep-link-error-screen')), findsNothing);
  });

  testWidgets('a resolve result is ignored after the active session changes', (
    tester,
  ) async {
    final pendingResponse = Completer<http.Response>();
    final mockClient = MockClient((_) => pendingResponse.future);
    final container = ProviderContainer(
      overrides: voiceAppTestOverrides(client: mockClient),
    );
    addTearDown(container.dispose);
    final router = _createResolverRouter();
    await tester.pumpWidget(_resolverApp(container: container, router: router));

    await tester.tap(find.text('resolve'));
    await tester.pump();
    container.read(authControllerProvider.notifier).state = const AuthState();
    pendingResponse.complete(
      http.Response(
        '{"code":"PRIVATE_DETAIL","message":"private resolver detail"}',
        403,
        headers: const {'content-type': 'application/json'},
      ),
    );
    await tester.pumpAndSettle();

    expect(container.read(selectedChatIdProvider), isNull);
    expect(find.byKey(const ValueKey('deep-link-error-screen')), findsNothing);
  });

  testWidgets(
    'a resolve result is ignored after the active profile changes within the account',
    (tester) async {
      final pendingResponse = Completer<http.Response>();
      final mockClient = MockClient((_) => pendingResponse.future);
      final container = ProviderContainer(
        overrides: voiceAppTestOverrides(client: mockClient),
      );
      addTearDown(container.dispose);
      final router = _createResolverRouter();
      await tester.pumpWidget(
        _resolverApp(container: container, router: router),
      );

      await tester.tap(find.text('resolve'));
      await tester.pump();
      container.read(authControllerProvider.notifier).state = const AuthState(
        session: AuthSession(
          accessToken: 'next-profile-access',
          refreshToken: 'next-profile-refresh',
          accountId: 'acc-test',
          activeProfileId: 'prof-next',
          expiresInSeconds: 900,
        ),
      );
      pendingResponse.complete(
        http.Response(
          '{"code":"PRIVATE_DETAIL","message":"private resolver detail"}',
          403,
          headers: const {'content-type': 'application/json'},
        ),
      );
      await tester.pumpAndSettle();

      expect(
        container.read(authControllerProvider).session?.accountId,
        'acc-test',
      );
      expect(
        container.read(authControllerProvider).session?.activeProfileId,
        'prof-next',
      );
      expect(container.read(selectedChatIdProvider), isNull);
      expect(
        find.byKey(const ValueKey('deep-link-error-screen')),
        findsNothing,
      );
      expect(find.text('private resolver detail'), findsNothing);
    },
  );

  testWidgets('the Home CTA is reachable and activatable from the keyboard', (
    tester,
  ) async {
    final mockClient = MockClient(
      (_) async => http.Response(
        '{"code":"PRIVATE_DETAIL","message":"private resolver detail"}',
        404,
        headers: const {'content-type': 'application/json'},
      ),
    );
    final container = ProviderContainer(
      overrides: voiceAppTestOverrides(client: mockClient),
    );
    addTearDown(container.dispose);
    final router = _createResolverRouter();
    await tester.pumpWidget(_resolverApp(container: container, router: router));

    await tester.tap(find.text('resolve'));
    await tester.pumpAndSettle();
    expect(find.text('Not found'), findsOneWidget);

    await tester.sendKeyEvent(LogicalKeyboardKey.tab);
    await tester.pump();
    expect(find.byKey(const ValueKey('deep-link-error-home')), findsOneWidget);
    expect(find.text('Go to Home'), findsOneWidget);

    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.pumpAndSettle();

    expect(find.text('home shell'), findsOneWidget);
    expect(find.byKey(const ValueKey('deep-link-error-screen')), findsNothing);
    expect(find.text('Not found'), findsNothing);
    expect(find.text('private resolver detail'), findsNothing);
  });

  testWidgets('a newer resolve supersedes an older pending result', (
    tester,
  ) async {
    final olderResponse = Completer<http.Response>();
    var requestCount = 0;
    final mockClient = MockClient((_) {
      requestCount++;
      if (requestCount == 1) return olderResponse.future;
      return Future.value(
        http.Response(
          '{"kind":"space","space_id":"resolved-space"}',
          200,
          headers: const {'content-type': 'application/json'},
        ),
      );
    });
    final container = ProviderContainer(
      overrides: voiceAppTestOverrides(client: mockClient),
    );
    addTearDown(container.dispose);
    final router = _createResolverRouter();
    await tester.pumpWidget(_resolverApp(container: container, router: router));

    await tester.tap(find.text('resolve'));
    await tester.pump();
    await tester.tap(find.text('resolve'));
    await tester.pumpAndSettle();
    olderResponse.complete(
      http.Response(
        '{"code":"PRIVATE_DETAIL","message":"private resolver detail"}',
        404,
        headers: const {'content-type': 'application/json'},
      ),
    );
    await tester.pumpAndSettle();

    expect(requestCount, 2);
    expect(container.read(selectedSpaceIdProvider), 'resolved-space');
    expect(find.byKey(const ValueKey('deep-link-error-screen')), findsNothing);
  });

  for (final (orientation, viewport, statusCode) in [
    ('H', const Size(1280, 800), 403),
    ('V', const Size(390, 844), 404),
  ]) {
    testWidgets(
      'deep-link error fits the production-font $orientation viewport',
      (tester) async {
        tester.view.physicalSize = viewport;
        tester.view.devicePixelRatio = 1;
        tester.binding.handleMetricsChanged();
        addTearDown(tester.view.resetPhysicalSize);
        addTearDown(tester.view.resetDevicePixelRatio);

        final captureDirectory =
            Platform.environment['VOICE_DEEPLINK_CAPTURE_DIR'];
        final theme = captureDirectory == null || captureDirectory.isEmpty
            ? voiceTestTheme()
            : await _captureTheme(tester);
        final mockClient = MockClient(
          (_) async => http.Response(
            '{"code":"PRIVATE_DETAIL","message":"private resolver detail"}',
            statusCode,
            headers: const {'content-type': 'application/json'},
          ),
        );
        final container = ProviderContainer(
          overrides: voiceAppTestOverrides(client: mockClient),
        );
        addTearDown(container.dispose);
        final router = _createResolverRouter();
        await tester.pumpWidget(
          _resolverApp(
            container: container,
            router: router,
            theme: theme,
            capture: true,
          ),
        );

        await tester.tap(find.text('resolve'));
        await tester.pumpAndSettle();

        expect(
          find.text(statusCode == 403 ? 'Access denied' : 'Not found'),
          findsOneWidget,
        );
        expect(find.text('Go to Home'), findsOneWidget);
        expect(find.text('private resolver detail'), findsNothing);
        expect(tester.takeException(), isNull);
        if (captureDirectory != null && captureDirectory.isNotEmpty) {
          await _writeCapture(
            tester,
            captureDirectory,
            'deep-link-error-${orientation.toLowerCase()}',
          );
        }
      },
    );
  }
}

GoRouter _createResolverRouter() => createVoiceGoRouter(
  shellBuilder: (context, state) => Consumer(
    builder: (context, ref, _) => Scaffold(
      body: Column(
        children: [
          const Text('home shell'),
          TextButton(
            onPressed: () => resolveAndNavigateDeepLink(
              ref,
              parseDeepLinkUrl('https://voice.gg/ch/denied-chat'),
            ),
            child: const Text('resolve'),
          ),
        ],
      ),
    ),
  ),
);

Widget _resolverApp({
  required ProviderContainer container,
  required GoRouter router,
  ThemeData? theme,
  bool capture = false,
}) => UncontrolledProviderScope(
  container: container,
  child: RepaintBoundary(
    key: capture ? _captureBoundaryKey : null,
    child: MaterialApp.router(
      theme: theme,
      debugShowCheckedModeBanner: false,
      locale: const Locale('en'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      routerConfig: router,
    ),
  ),
);

Future<ThemeData> _captureTheme(WidgetTester tester) async {
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
      final iconPath = [
        flutterRoot,
        'bin',
        'cache',
        'artifacts',
        'material_fonts',
        'MaterialIcons-Regular.otf',
      ].join(Platform.pathSeparator);
      final iconBytes = await File(iconPath).readAsBytes();
      final iconData = ByteData.sublistView(Uint8List.fromList(iconBytes));
      await (FontLoader(
        'MaterialIcons',
      )..addFont(Future<ByteData>.value(iconData))).load();
      return true;
    });
    if (loaded != true) {
      throw StateError('Production font loading did not complete');
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
    throw StateError('Voice design tokens did not load for capture');
  }
  return theme;
}

Future<void> _writeCapture(
  WidgetTester tester,
  String directory,
  String filename,
) async {
  final boundary = tester.renderObject<RenderRepaintBoundary>(
    find.byKey(_captureBoundaryKey),
  );
  final captured = await tester.runAsync(() async {
    final image = await boundary.toImage(pixelRatio: 1);
    try {
      final data = await image.toByteData(format: ui.ImageByteFormat.png);
      if (data == null) throw StateError('PNG encoding returned null');
      final output = File('$directory${Platform.pathSeparator}$filename.png');
      await output.parent.create(recursive: true);
      await output.writeAsBytes(data.buffer.asUint8List(), flush: true);
      return data.buffer.asUint8List();
    } finally {
      image.dispose();
    }
  });
  if (captured == null || captured.isEmpty) {
    throw StateError('Deep-link error screenshot capture did not complete');
  }
}
