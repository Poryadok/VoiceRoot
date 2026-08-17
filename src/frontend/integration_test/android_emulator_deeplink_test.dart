import 'dart:io' show Platform;

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/testing.dart';
import 'package:integration_test/integration_test.dart';
import 'package:voice_frontend/app.dart';
import 'package:voice_frontend/routing/deep_link_parser.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';
import 'package:voice_frontend/state/deep_link_navigation.dart';
import 'package:voice_frontend/state/shared_media_providers.dart';

import '../test/support/auth_test_overrides.dart';
import '../test/support/voice_test_theme.dart';

/// P3.1 A3 — Android emulator driver (custom scheme + https App Link URL shape).
///
/// Runs on an Android emulator/device, **not** `flutter-tester`.
/// Host tester CI stays on [device_driver_smoke_test.dart].
///
/// Does **not** require Play SHA or prod `voice.gg` well-known. OS-level
/// `am start` / debug-SHA App Links verification: see README.md.
///
/// Skip without a device: `Platform.isAndroid` is false on flutter-tester.
bool get _runningOnAndroid => !kIsWeb && Platform.isAndroid;

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  if (!_runningOnAndroid) {
    test(
      'skipped: requires Android emulator (not flutter-tester); see integration_test/README.md',
      () {},
      skip: true,
    );
    return;
  }

  group('Android emulator deep links', () {
    testWidgets('custom scheme voice:// opens conversation', (tester) async {
      await bindDesktopTestViewport(tester);
      final container = await _pumpAndroidShell(tester);

      await container
          .read(deepLinkNavigatorProvider)
          .apply(parseDeepLinkUrl('voice://ch/emu-chat/m/emu-msg'));
      await _pumpShellReady(tester);

      expect(container.read(selectedChatIdProvider), 'emu-chat');
      expect(
        container.read(pendingChatMessageScrollProvider('emu-chat')),
        'emu-msg',
      );
    });

    testWidgets('https App Link URL shape opens conversation', (tester) async {
      await bindDesktopTestViewport(tester);
      final container = await _pumpAndroidShell(tester);

      await container
          .read(deepLinkNavigatorProvider)
          .apply(parseDeepLinkUrl('https://voice.gg/ch/emu-applink-chat'));
      await _pumpShellReady(tester);

      expect(container.read(selectedChatIdProvider), 'emu-applink-chat');
    });
  });
}

Future<ProviderContainer> _pumpAndroidShell(WidgetTester tester) async {
  late ProviderContainer container;
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container = ProviderContainer(
        overrides: [
          ...guestShellTestOverrides(
            client: MockClient((_) async => throw UnimplementedError()),
          ),
          connectivityWatcherProvider.overrideWith((ref) {}),
        ],
      ),
      child: const VoiceApp(locale: Locale('en')),
    ),
  );
  await _pumpShellReady(tester);
  return container;
}

Future<void> _pumpShellReady(WidgetTester tester) async {
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 100));
  await tester.pump(const Duration(milliseconds: 400));
}
