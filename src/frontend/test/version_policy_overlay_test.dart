import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/version_policy_providers.dart';
import 'package:voice_frontend/state/version_update_launcher.dart';
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

void main() {
  testWidgets('force update blocks interaction', (tester) async {
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(client: MockClient((_) async => http.Response('{}', 404))),
        versionUpdateLauncherProvider.overrideWithValue(_RecordingLauncher()),
        versionPolicyProvider.overrideWith(
          (ref) => VersionPolicyController(ref, enablePolling: false),
        ),
      ],
    );
    addTearDown(container.dispose);

    container.read(versionPolicyProvider.notifier).state = const VersionPolicyState(
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

    expect(find.byKey(const Key('version_force_update_barrier')), findsOneWidget);
    expect(find.text('Required fix'), findsOneWidget);
    expect(find.text('App body'), findsOneWidget);
  });

  testWidgets('force update button launches store', (tester) async {
    final launcher = _RecordingLauncher();
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(client: MockClient((_) async => http.Response('{}', 404))),
        versionUpdateLauncherProvider.overrideWithValue(launcher),
        versionPolicyProvider.overrideWith(
          (ref) => VersionPolicyController(ref, enablePolling: false),
        ),
      ],
    );
    addTearDown(container.dispose);

    container.read(versionPolicyProvider.notifier).state = const VersionPolicyState(
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
        ...voiceAppTestOverrides(client: MockClient((_) async => http.Response('{}', 404))),
        versionUpdateLauncherProvider.overrideWithValue(launcher),
        versionPolicyProvider.overrideWith(
          (ref) => VersionPolicyController(ref, enablePolling: false),
        ),
      ],
    );
    addTearDown(container.dispose);

    container.read(versionPolicyProvider.notifier).state = const VersionPolicyState(
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
        ...voiceAppTestOverrides(client: MockClient((_) async => http.Response('{}', 404))),
        versionUpdateLauncherProvider.overrideWithValue(_RecordingLauncher()),
        versionPolicyProvider.overrideWith(
          (ref) => VersionPolicyController(ref, enablePolling: false),
        ),
      ],
    );
    addTearDown(container.dispose);

    container.read(versionPolicyProvider.notifier).state = const VersionPolicyState(
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

    expect(find.byKey(const Key('version_force_update_button')), findsOneWidget);
  });

  testWidgets('soft update shows Update and Later buttons', (tester) async {
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(client: MockClient((_) async => http.Response('{}', 404))),
        versionUpdateLauncherProvider.overrideWithValue(_RecordingLauncher()),
        versionPolicyProvider.overrideWith(
          (ref) => VersionPolicyController(ref, enablePolling: false),
        ),
      ],
    );
    addTearDown(container.dispose);

    container.read(versionPolicyProvider.notifier).state = const VersionPolicyState(
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
    expect(find.byKey(const Key('version_soft_update_dismiss')), findsOneWidget);
  });

  testWidgets('windows desktop soft update shows restart and update button', (tester) async {
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(client: MockClient((_) async => http.Response('{}', 404))),
        versionUpdateLauncherProvider.overrideWithValue(_RecordingLauncher()),
        versionPolicyProvider.overrideWith(
          (ref) => VersionPolicyController(ref, enablePolling: false),
        ),
      ],
    );
    addTearDown(container.dispose);

    container.read(versionPolicyProvider.notifier).state = const VersionPolicyState(
      phase: VersionPolicyPhase.desktopReadyToRestart,
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

    expect(find.byKey(const Key('version_desktop_restart_button')), findsOneWidget);
    expect(find.byKey(const Key('version_soft_update_dismiss')), findsOneWidget);
  });

  testWidgets('soft update banner dismisses', (tester) async {
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(client: MockClient((_) async => http.Response('{}', 404))),
        versionUpdateLauncherProvider.overrideWithValue(_RecordingLauncher()),
        versionPolicyProvider.overrideWith(
          (ref) => VersionPolicyController(ref, enablePolling: false),
        ),
      ],
    );
    addTearDown(container.dispose);

    container.read(versionPolicyProvider.notifier).state = const VersionPolicyState(
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
    expect(
      container.read(versionPolicyProvider).phase,
      VersionPolicyPhase.ok,
    );
  });
}
