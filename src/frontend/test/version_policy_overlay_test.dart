import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/services/desktop_updater_service.dart';
import 'package:voice_frontend/state/version_policy_providers.dart';
import 'package:voice_frontend/state/version_update_launcher.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/ui/version/version_policy_overlay.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

class _RecordingLauncher implements VersionUpdateLauncher {
  int launchCount = 0;
  bool? lastImmediate;

  @override
  Future<void> launchUpdate({
    required String updateUrl,
    required bool immediate,
  }) async {
    launchCount++;
    lastImmediate = immediate;
  }
}

const _captureRootKey = ValueKey('version_update_capture_root');
var _captureFontsLoaded = false;

Future<void> _loadCaptureFonts(WidgetTester tester) async {
  if (_captureFontsLoaded) return;
  final loaded = await tester.runAsync(() async {
    final noto = FontLoader(VoiceTheme.fontFamily)
      ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
      ..addFont(rootBundle.load('assets/fonts/NotoSans-Medium.ttf'))
      ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'))
      ..addFont(rootBundle.load('assets/fonts/NotoSans-Bold.ttf'));
    await noto.load();

    final flutterRoot = Platform.environment['FLUTTER_ROOT'];
    if (flutterRoot == null || flutterRoot.isEmpty) {
      throw StateError(
        'FLUTTER_ROOT is required to load the SDK Material Icons font',
      );
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
    final bytes = await iconFont.readAsBytes();
    final iconData = ByteData.sublistView(Uint8List.fromList(bytes));
    await (FontLoader(
      'MaterialIcons',
    )..addFont(Future<ByteData>.value(iconData))).load();
    return true;
  });
  if (loaded == null) {
    throw StateError('Flutter app font loading did not complete');
  }
  _captureFontsLoaded = true;
}

Future<ThemeData> _applicationTheme(WidgetTester tester) async {
  if ((Platform.environment['VOICE_VERSION_CAPTURE_DIR'] ?? '').isNotEmpty) {
    await _loadCaptureFonts(tester);
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

Widget _versionOverlayApp({
  required ProviderContainer container,
  required ThemeData theme,
  required Widget body,
}) {
  return RepaintBoundary(
    key: _captureRootKey,
    child: UncontrolledProviderScope(
      container: container,
      child: MaterialApp(
        debugShowCheckedModeBanner: false,
        theme: theme,
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: VersionPolicyOverlay(child: Scaffold(body: body)),
      ),
    ),
  );
}

Future<void> _captureIfRequested(WidgetTester tester, String filename) async {
  final directory = Platform.environment['VOICE_VERSION_CAPTURE_DIR'];
  if (directory == null || directory.isEmpty) return;

  final boundary = tester.renderObject<RenderRepaintBoundary>(
    find.byKey(_captureRootKey),
  );
  final captured = await tester.runAsync(() async {
    final image = await boundary.toImage(pixelRatio: 1);
    try {
      final data = await image.toByteData(format: ui.ImageByteFormat.png);
      if (data == null) {
        throw StateError('Flutter image encoding returned null');
      }
      final output = File('$directory${Platform.pathSeparator}$filename');
      await output.parent.create(recursive: true);
      await output.writeAsBytes(data.buffer.asUint8List(), flush: true);
      return data.buffer.asUint8List();
    } finally {
      image.dispose();
    }
  });
  if (captured == null || captured.isEmpty) {
    throw StateError('Flutter screenshot capture did not complete');
  }
}

void main() {
  testWidgets('force update blocks interaction', (tester) async {
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 404)),
        ),
        versionUpdateLauncherProvider.overrideWithValue(_RecordingLauncher()),
        versionPolicyProvider.overrideWith(
          (ref) => VersionPolicyController(ref, enablePolling: false),
        ),
      ],
    );
    addTearDown(container.dispose);

    container
        .read(versionPolicyProvider.notifier)
        .state = const VersionPolicyState(
      phase: VersionPolicyPhase.forceUpdate,
      updateUrl: 'https://play.google.com/store/apps/details?id=voice',
      releaseNotes: 'Required fix',
    );

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const VersionPolicyOverlay(
            child: Scaffold(body: Text('App body')),
          ),
        ),
      ),
    );

    expect(
      find.byKey(const Key('version_force_update_barrier')),
      findsOneWidget,
    );
    expect(find.text('Required fix'), findsOneWidget);
    expect(find.text('App body'), findsOneWidget);
  });

  testWidgets('force update button launches store', (tester) async {
    final launcher = _RecordingLauncher();
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 404)),
        ),
        versionUpdateLauncherProvider.overrideWithValue(launcher),
        versionPolicyProvider.overrideWith(
          (ref) => VersionPolicyController(ref, enablePolling: false),
        ),
      ],
    );
    addTearDown(container.dispose);

    container
        .read(versionPolicyProvider.notifier)
        .state = const VersionPolicyState(
      phase: VersionPolicyPhase.forceUpdate,
      updateUrl: 'https://play.google.com/store/apps/details?id=voice',
    );

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const VersionPolicyOverlay(
            child: Scaffold(body: Text('App body')),
          ),
        ),
      ),
    );

    await tester.tap(find.byKey(const Key('version_force_update_button')));
    await tester.pump();

    expect(launcher.launchCount, 1);
    expect(launcher.lastImmediate, isTrue);
  });

  testWidgets('soft update button launches store', (tester) async {
    final launcher = _RecordingLauncher();
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 404)),
        ),
        versionUpdateLauncherProvider.overrideWithValue(launcher),
        versionPolicyProvider.overrideWith(
          (ref) => VersionPolicyController(ref, enablePolling: false),
        ),
      ],
    );
    addTearDown(container.dispose);

    container
        .read(versionPolicyProvider.notifier)
        .state = const VersionPolicyState(
      phase: VersionPolicyPhase.softUpdate,
      latestVersion: '1.2.0',
      updateUrl: 'https://play.google.com/store',
    );

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const VersionPolicyOverlay(
            child: Scaffold(body: Text('App body')),
          ),
        ),
      ),
    );

    await tester.tap(find.byKey(const Key('version_soft_update_button')));
    await tester.pump();

    expect(launcher.launchCount, 1);
    expect(launcher.lastImmediate, isFalse);
  });

  testWidgets('force update shows Update button', (tester) async {
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 404)),
        ),
        versionUpdateLauncherProvider.overrideWithValue(_RecordingLauncher()),
        versionPolicyProvider.overrideWith(
          (ref) => VersionPolicyController(ref, enablePolling: false),
        ),
      ],
    );
    addTearDown(container.dispose);

    container
        .read(versionPolicyProvider.notifier)
        .state = const VersionPolicyState(
      phase: VersionPolicyPhase.forceUpdate,
      updateUrl: 'https://updates.voice.example/windows/appcast.xml',
    );

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const VersionPolicyOverlay(
            child: Scaffold(body: Text('App body')),
          ),
        ),
      ),
    );

    expect(
      find.byKey(const Key('version_force_update_button')),
      findsOneWidget,
    );
  });

  testWidgets('soft update shows Update and Later buttons', (tester) async {
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 404)),
        ),
        versionUpdateLauncherProvider.overrideWithValue(_RecordingLauncher()),
        versionPolicyProvider.overrideWith(
          (ref) => VersionPolicyController(ref, enablePolling: false),
        ),
      ],
    );
    addTearDown(container.dispose);

    container
        .read(versionPolicyProvider.notifier)
        .state = const VersionPolicyState(
      phase: VersionPolicyPhase.softUpdate,
      latestVersion: '1.8.0',
      updateUrl: 'https://updates.voice.example/windows/appcast.xml',
    );

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const VersionPolicyOverlay(
            child: Scaffold(body: Text('App body')),
          ),
        ),
      ),
    );

    expect(find.byKey(const Key('version_soft_update_button')), findsOneWidget);
    expect(
      find.byKey(const Key('version_soft_update_dismiss')),
      findsOneWidget,
    );
  });

  testWidgets(
    'desktop ready soft patch shows policy notes and preserves nonblocking actions',
    (tester) async {
      tester.view.physicalSize = const Size(1280, 800);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      final desktopUpdater = RecordingDesktopUpdaterService();
      final container = ProviderContainer(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => http.Response('{}', 404)),
          ),
          versionUpdateLauncherProvider.overrideWithValue(_RecordingLauncher()),
          desktopUpdaterServiceProvider.overrideWithValue(desktopUpdater),
          versionPolicyProvider.overrideWith(
            (ref) => VersionPolicyController(ref, enablePolling: false),
          ),
        ],
      );
      addTearDown(container.dispose);

      const policy = VersionPolicyState(
        phase: VersionPolicyPhase.desktopReadyToRestart,
        latestVersion: '1.8.0',
        updateUrl: 'https://updates.voice.example/windows/appcast.xml',
        releaseNotes: 'Required fix',
      );
      container.read(versionPolicyProvider.notifier).state = policy;
      var backgroundActions = 0;

      await tester.pumpWidget(
        _versionOverlayApp(
          container: container,
          theme: await _applicationTheme(tester),
          body: Align(
            alignment: Alignment.topLeft,
            child: TextButton(
              key: const Key('version_background_action'),
              onPressed: () => backgroundActions++,
              child: const Text('App body action'),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(
        find.byKey(const Key('version_desktop_restart_button')),
        findsOneWidget,
      );
      expect(
        find.byKey(const Key('version_soft_update_dismiss')),
        findsOneWidget,
      );
      expect(find.text('Required fix'), findsOneWidget);
      await _captureIfRequested(tester, 'desktop-soft-patch.png');

      await tester.tap(find.byKey(const Key('version_background_action')));
      await tester.pump();
      expect(
        backgroundActions,
        1,
        reason: 'soft desktop update stays nonblocking',
      );

      await tester.tap(find.byKey(const Key('version_soft_update_dismiss')));
      await tester.pump();
      expect(
        container.read(versionPolicyProvider).phase,
        VersionPolicyPhase.ok,
      );

      container.read(versionPolicyProvider.notifier).state = policy;
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('version_desktop_restart_button')));
      await tester.pump();
      expect(desktopUpdater.restartCalls, 1);
    },
  );

  testWidgets(
    'forced mobile update scrolls long policy notes, blocks background, and has no Later',
    (tester) async {
      tester.view.physicalSize = const Size(390, 844);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      final launcher = _RecordingLauncher();
      final container = ProviderContainer(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => http.Response('{}', 404)),
          ),
          versionUpdateLauncherProvider.overrideWithValue(launcher),
          desktopUpdaterServiceProvider.overrideWithValue(
            RecordingDesktopUpdaterService(),
          ),
          versionPolicyProvider.overrideWith(
            (ref) => VersionPolicyController(ref, enablePolling: false),
          ),
        ],
      );
      addTearDown(container.dispose);

      final notes = List.filled(64, 'Required fix').join('\n');
      container.read(versionPolicyProvider.notifier).state = VersionPolicyState(
        phase: VersionPolicyPhase.forceUpdate,
        updateUrl: 'https://play.google.com/store/apps/details?id=voice',
        releaseNotes: notes,
      );
      var backgroundActions = 0;

      await tester.pumpWidget(
        _versionOverlayApp(
          container: container,
          theme: await _applicationTheme(tester),
          body: Align(
            alignment: Alignment.topLeft,
            child: TextButton(
              key: const Key('version_background_action'),
              onPressed: () => backgroundActions++,
              child: const Text('App body action'),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(
        find.byKey(const Key('version_force_update_barrier')),
        findsOneWidget,
      );
      expect(
        find.byKey(const Key('version_force_update_button')),
        findsOneWidget,
      );
      expect(find.text('Later'), findsNothing);
      expect(find.text(notes), findsOneWidget);
      expect(tester.takeException(), isNull);
      await _captureIfRequested(tester, 'mobile-forced-update-notes.png');

      await tester.tapAt(const Offset(4, 4));
      await tester.pump();
      expect(backgroundActions, 0);
      expect(
        container.read(versionPolicyProvider).phase,
        VersionPolicyPhase.forceUpdate,
      );

      final updateButton = find.byKey(const Key('version_force_update_button'));
      await tester.ensureVisible(updateButton);
      await tester.pumpAndSettle();
      await _captureIfRequested(tester, 'mobile-forced-update.png');
      final buttonRect = tester.getRect(updateButton);
      expect(buttonRect.top, greaterThanOrEqualTo(0));
      expect(buttonRect.bottom, lessThanOrEqualTo(844));
      final semantics = tester.ensureSemantics();
      try {
        expect(find.bySemanticsLabel('Update'), findsOneWidget);
      } finally {
        semantics.dispose();
      }

      await tester.tap(updateButton);
      await tester.pump();
      expect(launcher.launchCount, 1);
      expect(launcher.lastImmediate, isTrue);
      expect(
        container.read(versionPolicyProvider).phase,
        VersionPolicyPhase.forceUpdate,
      );
    },
  );

  testWidgets('forced update without a URL has no update action or deferral', (
    tester,
  ) async {
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 404)),
        ),
        versionUpdateLauncherProvider.overrideWithValue(_RecordingLauncher()),
        versionPolicyProvider.overrideWith(
          (ref) => VersionPolicyController(ref, enablePolling: false),
        ),
      ],
    );
    addTearDown(container.dispose);
    container
        .read(versionPolicyProvider.notifier)
        .state = const VersionPolicyState(
      phase: VersionPolicyPhase.forceUpdate,
      releaseNotes: 'Required fix',
    );

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const VersionPolicyOverlay(
            child: Scaffold(body: Text('App body')),
          ),
        ),
      ),
    );

    expect(
      find.byKey(const Key('version_force_update_barrier')),
      findsOneWidget,
    );
    expect(find.byKey(const Key('version_force_update_button')), findsNothing);
    expect(find.byKey(const Key('version_soft_update_dismiss')), findsNothing);
  });

  testWidgets('soft update banner dismisses', (tester) async {
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 404)),
        ),
        versionUpdateLauncherProvider.overrideWithValue(_RecordingLauncher()),
        versionPolicyProvider.overrideWith(
          (ref) => VersionPolicyController(ref, enablePolling: false),
        ),
      ],
    );
    addTearDown(container.dispose);

    container
        .read(versionPolicyProvider.notifier)
        .state = const VersionPolicyState(
      phase: VersionPolicyPhase.softUpdate,
      latestVersion: '1.2.0',
      updateUrl: 'https://play.google.com/store',
    );

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const VersionPolicyOverlay(
            child: Scaffold(body: Text('App body')),
          ),
        ),
      ),
    );

    expect(find.byKey(const Key('version_soft_update_banner')), findsOneWidget);
    await tester.tap(find.byKey(const Key('version_soft_update_dismiss')));
    await tester.pump();
    expect(container.read(versionPolicyProvider).phase, VersionPolicyPhase.ok);
  });
}
