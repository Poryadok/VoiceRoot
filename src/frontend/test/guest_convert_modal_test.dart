import 'dart:convert';
import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/app.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/ui/a11y/focus_trap.dart';
import 'package:voice_frontend/ui/auth/guest_convert_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

const _captureBoundaryKey = ValueKey<String>('guest_convert_capture');

void main() {
  testWidgets(
    'guest reminder opens a centered desktop conversion dialog',
    (tester) async {
      final requests = await _openGuestConversionFromReminder(
        tester,
        viewport: const Size(1280, 800),
      );

      // This fails against the original implementation: it always opens a sheet.
      expect(find.byType(Dialog), findsOneWidget);
      expect(find.byType(BottomSheet), findsNothing);
      final dialogSurface = find
          .descendant(of: find.byType(Dialog), matching: find.byType(Material))
          .first;
      final dialogRect = tester.getRect(dialogSurface);
      expect(dialogRect.center.dx, closeTo(640, 1));
      expect(dialogRect.center.dy, closeTo(400, 1));
      expect(dialogRect.width, lessThanOrEqualTo(420));
      expect(find.byKey(const Key('guest_convert_modal')), findsOneWidget);
      expect(find.byKey(const Key('guest_convert_email')), findsOneWidget);
      expect(find.byKey(const Key('guest_convert_password')), findsOneWidget);
      expect(
        find.descendant(
          of: find.byType(Dialog),
          matching: find.byType(VoiceFocusTrap),
        ),
        findsOneWidget,
      );
      expect(_primaryFocusIsWithinGuestTrap(tester), isTrue);
      await _captureGuestConvert(tester, 'guest-convert-H-1280x800');

      final cancelFocus = Focus.of(
        tester.element(
          find.byKey(const Key('guest_save_account_reminder_cta')),
        ),
      );
      await tester.tap(find.text('Cancel'));
      await tester.pumpAndSettle();
      expect(find.byKey(GuestConvertSheet.modalKey), findsNothing);
      expect(
        find.byKey(const Key('guest_save_account_reminder')),
        findsOneWidget,
      );
      expect(tester.binding.focusManager.primaryFocus, same(cancelFocus));
      expect(_conversionRequests(requests), isEmpty);

      // Re-entry and Escape use the same route/focus trap; dismissal does not submit.
      await tester.tap(
        find.byKey(const Key('guest_save_account_reminder_cta')),
      );
      await tester.pumpAndSettle();
      expect(find.byType(Dialog), findsOneWidget);
      expect(_primaryFocusIsWithinGuestTrap(tester), isTrue);
      await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      await tester.pumpAndSettle();
      expect(find.byKey(GuestConvertSheet.modalKey), findsNothing);
      expect(_conversionRequests(requests), isEmpty);
    },
    variant: const TargetPlatformVariant({TargetPlatform.windows}),
  );

  testWidgets(
    'guest reminder keeps the mobile conversion bottom sheet',
    (tester) async {
      final requests = await _openGuestConversionFromReminder(
        tester,
        viewport: const Size(390, 844),
      );

      expect(find.byType(BottomSheet), findsOneWidget);
      expect(find.byType(Dialog), findsNothing);
      final sheetRect = tester.getRect(find.byType(BottomSheet));
      expect(sheetRect.width, closeTo(390, 1));
      expect(sheetRect.bottom, closeTo(844, 1));
      expect(find.byKey(GuestConvertSheet.modalKey), findsOneWidget);
      expect(
        find.descendant(
          of: find.byType(BottomSheet),
          matching: find.byType(VoiceFocusTrap),
        ),
        findsOneWidget,
      );
      expect(find.byKey(GuestConvertSheet.emailFieldKey), findsOneWidget);
      expect(find.byKey(GuestConvertSheet.passwordFieldKey), findsOneWidget);
      await _captureGuestConvert(tester, 'guest-convert-V-390x844');

      await tester.tap(find.text('Cancel'));
      await tester.pumpAndSettle();
      expect(find.byKey(GuestConvertSheet.modalKey), findsNothing);
      expect(
        find.byKey(const Key('guest_save_account_reminder')),
        findsOneWidget,
      );
      expect(_conversionRequests(requests), isEmpty);
    },
    variant: const TargetPlatformVariant({TargetPlatform.windows}),
  );

  testWidgets('convert sheet shows localized client validation errors', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          discoverHintStorageProvider.overrideWithValue(
            testDiscoverHintStorage,
          ),
          authControllerProvider.overrideWith((ref) {
            final c = authenticatedAuthController(ref);
            c.state = c.state.copyWith(isGuest: true);
            return c;
          }),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((_) async => throw UnimplementedError()),
          ),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: GuestConvertSheet()),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('Create account'));
    await tester.pump();

    expect(find.text('Enter your email and password.'), findsNWidgets(2));
    expect(find.byKey(GuestConvertSheet.errorKey), findsNothing);

    await tester.enterText(
      find.byKey(GuestConvertSheet.emailFieldKey),
      'guest@example.com',
    );
    await tester.tap(find.text('Create account'));
    await tester.pump();

    expect(find.text('Enter your email and password.'), findsOneWidget);
    expect(find.byKey(GuestConvertSheet.errorKey), findsNothing);

    await tester.enterText(
      find.byKey(GuestConvertSheet.passwordFieldKey),
      'short',
    );
    await tester.tap(find.text('Create account'));
    await tester.pump();

    expect(
      find.text('Password must be at least 8 characters.'),
      findsOneWidget,
    );
    expect(find.byKey(GuestConvertSheet.errorKey), findsNothing);
  });

  testWidgets('convert sheet shows localized validation_failed from API', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          discoverHintStorageProvider.overrideWithValue(
            testDiscoverHintStorage,
          ),
          authControllerProvider.overrideWith((ref) {
            final c = authenticatedAuthController(ref);
            c.state = c.state.copyWith(isGuest: true);
            return c;
          }),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((request) async {
              if (request.url.path == '/api/v1/auth/convert-guest') {
                return http.Response(
                  jsonEncode({'error': 'validation_failed'}),
                  400,
                );
              }
              return http.Response('not found', 404);
            }),
          ),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: GuestConvertSheet()),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(GuestConvertSheet.emailFieldKey),
      'guest@example.com',
    );
    await tester.enterText(
      find.byKey(GuestConvertSheet.passwordFieldKey),
      'validpass',
    );
    await tester.tap(find.text('Create account'));
    await tester.pumpAndSettle();

    expect(
      find.text('Use a valid email and a password of at least 8 characters.'),
      findsOneWidget,
    );
    expect(find.byKey(GuestConvertSheet.errorKey), findsOneWidget);
    expect(find.text('validation_failed'), findsNothing);
  });

  testWidgets('convert sheet hides unknown Auth diagnostics', (tester) async {
    const diagnostic = 'private-conversion-stack-detail';
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          discoverHintStorageProvider.overrideWithValue(
            testDiscoverHintStorage,
          ),
          authControllerProvider.overrideWith((ref) {
            final c = authenticatedAuthController(ref);
            c.state = c.state.copyWith(isGuest: true);
            return c;
          }),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((request) async {
              if (request.url.path == '/api/v1/auth/convert-guest') {
                return http.Response(
                  jsonEncode({
                    'error': 'unknown_conversion_failure',
                    'message': diagnostic,
                  }),
                  500,
                );
              }
              return http.Response('not found', 404);
            }),
          ),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: GuestConvertSheet()),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(GuestConvertSheet.emailFieldKey),
      'guest@example.com',
    );
    await tester.enterText(
      find.byKey(GuestConvertSheet.passwordFieldKey),
      'validpass',
    );
    await tester.tap(find.text('Create account'));
    await tester.pumpAndSettle();

    expect(find.text('Could not complete this action.'), findsOneWidget);
    expect(find.text('unknown_conversion_failure'), findsNothing);
    expect(find.text(diagnostic), findsNothing);
  });
}

Future<List<http.Request>> _openGuestConversionFromReminder(
  WidgetTester tester, {
  required Size viewport,
}) async {
  tester.view.physicalSize = viewport;
  tester.view.devicePixelRatio = 1;
  await tester.binding.setSurfaceSize(viewport);
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(() async => tester.binding.setSurfaceSize(null));

  final requests = <http.Request>[];
  final client = MockClient((request) async {
    if (request.url.path == '/api/v1/auth/convert-guest') {
      requests.add(request);
    }
    return http.Response('not found', 404);
  });
  await tester.pumpWidget(
    RepaintBoundary(
      key: _captureBoundaryKey,
      child: ProviderScope(
        overrides: guestShellTestOverrides(client: client)
          ..add(
            authControllerProvider.overrideWith((ref) {
              final controller = authenticatedAuthController(ref);
              controller.state = controller.state.copyWith(isGuest: true);
              return controller;
            }),
          )
          ..add(connectivityWatcherProvider.overrideWith((ref) {})),
        child: const VoiceApp(locale: Locale('en')),
      ),
    ),
  );
  await tester.pumpAndSettle();
  expect(find.byKey(const Key('guest_save_account_reminder')), findsOneWidget);
  await tester.tap(find.byKey(const Key('guest_save_account_reminder_cta')));
  await tester.pumpAndSettle();
  return requests;
}

List<http.Request> _conversionRequests(List<http.Request> requests) => requests
    .where((request) => request.url.path == '/api/v1/auth/convert-guest')
    .toList();

bool _primaryFocusIsWithinGuestTrap(WidgetTester tester) {
  final focus = tester.binding.focusManager.primaryFocus;
  return focus?.debugLabel == 'VoiceFocusTrap' ||
      focus?.ancestors.any((node) => node.debugLabel == 'VoiceFocusTrap') ==
          true;
}

Future<void> _captureGuestConvert(WidgetTester tester, String name) async {
  final captureDirectory = Platform.environment['UIV3_CAPTURE_DIR'];
  if (captureDirectory == null || captureDirectory.isEmpty) return;

  final boundary = tester.renderObject<RenderRepaintBoundary>(
    find.byKey(_captureBoundaryKey),
  );
  final captured = await tester.runAsync(() async {
    final image = await boundary.toImage(pixelRatio: 1);
    try {
      final png = await image.toByteData(format: ui.ImageByteFormat.png);
      if (png == null) throw StateError('Flutter did not encode the capture');
      final directory = Directory(captureDirectory);
      await directory.create(recursive: true);
      await File(
        '${directory.path}${Platform.pathSeparator}$name.png',
      ).writeAsBytes(png.buffer.asUint8List(), flush: true);
      return true;
    } finally {
      image.dispose();
    }
  });
  if (captured != true) throw StateError('Flutter capture did not complete');
}
