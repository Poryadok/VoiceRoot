import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart' as http_testing;
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/settings/settings_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

const _urlLauncher = MethodChannel('plugins.flutter.io/url_launcher');

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    SharedPreferences.setMockInitialValues({});
  });

  testWidgets(
    'Settings Help searches the static guide and opens official links',
    (tester) async {
      tester.view.physicalSize = const Size(1280, 800);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      final launchCalls = <MethodCall>[];
      tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
        _urlLauncher,
        (call) async {
          launchCalls.add(call);
          return true;
        },
      );
      addTearDown(
        () => tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
          _urlLauncher,
          null,
        ),
      );

      final container = ProviderContainer(
        overrides: voiceAppTestOverrides(
          client: http_testing.MockClient(
            (_) async => http.Response('{}', 200),
          ),
        ),
      );
      addTearDown(container.dispose);
      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp(
            theme: voiceTestTheme(),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: Builder(
              builder: (context) => Scaffold(
                body: TextButton(
                  onPressed: () => showModalBottomSheet<void>(
                    context: context,
                    isScrollControlled: true,
                    builder: (_) => const SettingsSheet(),
                  ),
                  child: const Text('Open settings'),
                ),
              ),
            ),
          ),
        ),
      );

      await tester.tap(find.text('Open settings'));
      await tester.pumpAndSettle();
      await tester.ensureVisible(find.byKey(const Key('settings_help')));
      await tester.tap(find.byKey(const Key('settings_help')));
      await tester.pumpAndSettle();

      expect(find.byIcon(Icons.help_outline), findsOneWidget);
      expect(find.text('How can we help?'), findsOneWidget);
      expect(
        find.text('Find an answer or contact the Voice team.'),
        findsOneWidget,
      );
      expect(find.bySemanticsLabel('How can we help?'), findsOneWidget);
      expect(
        find.bySemanticsLabel('Find an answer or contact the Voice team.'),
        findsOneWidget,
      );
      final semantics = tester.ensureSemantics();
      expect(
        tester
            .getSemantics(find.byTooltip('Close help'))
            .getSemanticsData()
            .flagsCollection
            .isButton,
        isTrue,
      );
      expect(
        tester
            .getSemantics(find.text('How can we help?'))
            .getSemanticsData()
            .flagsCollection
            .isHeader,
        isTrue,
      );
      expect(
        tester
            .getSemantics(
              find.descendant(
                of: find.byKey(const Key('settings_help_search')),
                matching: find.byType(EditableText),
              ),
            )
            .getSemanticsData()
            .flagsCollection
            .isTextField,
        isTrue,
      );
      expect(
        tester
            .getSemantics(find.byKey(const Key('settings_help_docs')))
            .getSemanticsData()
            .flagsCollection
            .isButton,
        isTrue,
      );
      expect(
        tester
            .getSemantics(find.byKey(const Key('settings_help_support')))
            .getSemanticsData()
            .flagsCollection
            .isButton,
        isTrue,
      );
      expect(
        find.bySemanticsLabel('Search help (e.g. voice rooms)'),
        findsOneWidget,
      );
      expect(find.bySemanticsLabel('Project documentation'), findsOneWidget);
      expect(find.bySemanticsLabel('Contact support'), findsOneWidget);

      final close = find.byTooltip('Close help');
      final search = find.byKey(const Key('settings_help_search'));
      final docs = find.byKey(const Key('settings_help_docs'));
      final support = find.byKey(const Key('settings_help_support'));
      await _tabUntilPrimaryFocusWithin(tester, close);
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.pump();
      expect(_primaryFocusWithin(tester, search), isTrue);
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.pump();
      expect(_primaryFocusWithin(tester, docs), isTrue);
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.pump();
      expect(_primaryFocusWithin(tester, support), isTrue);
      await tester.sendKeyDownEvent(LogicalKeyboardKey.shiftLeft);
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.sendKeyUpEvent(LogicalKeyboardKey.shiftLeft);
      await tester.pump();
      expect(_primaryFocusWithin(tester, docs), isTrue);
      await tester.sendKeyDownEvent(LogicalKeyboardKey.shiftLeft);
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.sendKeyUpEvent(LogicalKeyboardKey.shiftLeft);
      await tester.pump();
      expect(_primaryFocusWithin(tester, search), isTrue);
      await tester.sendKeyDownEvent(LogicalKeyboardKey.shiftLeft);
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.sendKeyUpEvent(LogicalKeyboardKey.shiftLeft);
      await tester.pump();
      expect(_primaryFocusWithin(tester, close), isTrue);
      semantics.dispose();

      for (final title in ['Chats', 'Spaces', 'Matchmaking', 'Voice']) {
        expect(find.text(title), findsOneWidget);
      }

      await tester.enterText(
        find.byKey(const Key('settings_help_search')),
        'voice rooms',
      );
      await tester.pumpAndSettle();
      expect(find.text('Spaces'), findsOneWidget);
      expect(find.text('Voice'), findsOneWidget);
      expect(find.text('Chats'), findsNothing);
      expect(find.text('Matchmaking'), findsNothing);

      await tester.enterText(
        find.byKey(const Key('settings_help_search')),
        'e2e',
      );
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('settings_help_no_results')), findsOneWidget);

      await tester.tap(find.byKey(const Key('settings_help_clear_search')));
      await tester.pumpAndSettle();
      expect(find.text('Chats'), findsOneWidget);

      await tester.tap(find.byKey(const Key('settings_help_docs')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('settings_help_support')));
      await tester.pumpAndSettle();

      expect(launchCalls, hasLength(2));
      expect(
        (launchCalls[0].arguments as Map)['url'],
        'https://github.com/Poryadok/VoiceRoot/blob/master/README.md',
      );
      expect(
        (launchCalls[1].arguments as Map)['url'],
        'https://github.com/Poryadok/VoiceRoot/issues',
      );

      await tester.ensureVisible(close);
      await tester.tap(close);
      await tester.pumpAndSettle();
      expect(search, findsNothing);
    },
  );

  testWidgets('Settings Help shows recovery feedback when a link cannot open', (
    tester,
  ) async {
    var launchAttempts = 0;
    tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
      _urlLauncher,
      (_) async {
        launchAttempts++;
        return false;
      },
    );
    addTearDown(
      () => tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
        _urlLauncher,
        null,
      ),
    );

    final container = ProviderContainer(
      overrides: voiceAppTestOverrides(
        client: http_testing.MockClient((_) async => http.Response('{}', 200)),
      ),
    );
    addTearDown(container.dispose);
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Builder(
            builder: (context) => Scaffold(
              body: TextButton(
                onPressed: () => showModalBottomSheet<void>(
                  context: context,
                  isScrollControlled: true,
                  builder: (_) => const SettingsSheet(),
                ),
                child: const Text('Open settings'),
              ),
            ),
          ),
        ),
      ),
    );

    await tester.tap(find.text('Open settings'));
    await tester.pumpAndSettle();
    await tester.ensureVisible(find.byKey(const Key('settings_help')));
    await tester.tap(find.byKey(const Key('settings_help')));
    await tester.pumpAndSettle();
    final close = find.byTooltip('Close help');
    final search = find.byKey(const Key('settings_help_search'));
    final docs = find.byKey(const Key('settings_help_docs'));
    await _tabUntilPrimaryFocusWithin(tester, close);
    await tester.sendKeyEvent(LogicalKeyboardKey.tab);
    await tester.pump();
    expect(_primaryFocusWithin(tester, search), isTrue);
    await tester.sendKeyEvent(LogicalKeyboardKey.tab);
    await tester.pump();
    expect(_primaryFocusWithin(tester, docs), isTrue);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.pumpAndSettle();

    expect(find.text('Could not open link.'), findsOneWidget);
    expect(find.text('Try again'), findsOneWidget);
    final semantics = tester.ensureSemantics();
    expect(find.bySemanticsLabel('Try again'), findsOneWidget);
    semantics.dispose();
    expect(launchAttempts, 1);
    final support = find.byKey(const Key('settings_help_support'));
    final retryLabel = find.text('Try again');
    final retry = find.ancestor(
      of: retryLabel,
      matching: find.byType(TextButton),
    );
    expect(_primaryFocusWithin(tester, docs), isTrue);
    await tester.sendKeyEvent(LogicalKeyboardKey.tab);
    await tester.pump();
    expect(_primaryFocusWithin(tester, support), isTrue);
    await tester.sendKeyEvent(LogicalKeyboardKey.tab);
    await tester.pump();
    expect(_primaryFocusWithin(tester, retry), isTrue);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.pumpAndSettle();
    expect(launchAttempts, 2);
  });
}

bool _primaryFocusWithin(WidgetTester tester, Finder target) {
  final focusContext = FocusManager.instance.primaryFocus?.context;
  if (focusContext == null) return false;
  final targetElement = tester.element(target);
  var found = false;
  focusContext.visitAncestorElements((ancestor) {
    if (identical(ancestor, targetElement)) {
      found = true;
      return false;
    }
    return true;
  });
  return found;
}

Future<void> _tabUntilPrimaryFocusWithin(
  WidgetTester tester,
  Finder target,
) async {
  for (var attempt = 0; attempt < 8; attempt++) {
    if (_primaryFocusWithin(tester, target)) return;
    await tester.sendKeyEvent(LogicalKeyboardKey.tab);
    await tester.pump();
  }
  expect(_primaryFocusWithin(tester, target), isTrue);
}
