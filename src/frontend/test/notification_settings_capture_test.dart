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

import 'package:voice_frontend/backend/notification_settings_models.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/settings/notification_quiet_hours_storage.dart';
import 'package:voice_frontend/state/push_notifications_controller.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/ui/settings/notification_settings_screen.dart';

import 'support/auth_test_overrides.dart';

const _captureBoundaryKey = Key('notification_settings_capture');
const _captureDirectoryVariable = 'VOICE_NOTIFICATION_CAPTURE_DIR';

void main() {
  final captureDirectory = Platform.environment[_captureDirectoryVariable];
  testWidgets(
    'capture notification settings at desktop and mobile sizes',
    (tester) async {
      final directory = Directory(captureDirectory!);
      final theme = await tester.runAsync(() async {
        await directory.create(recursive: true);
        await _loadCaptureFonts();
        final catalog = await VoiceTokenCatalog.load();
        return VoiceTheme.build(
          catalog: catalog,
          mode: VoiceThemeMode.dark,
          profileAccent: catalog.profileAccentAt(0),
        );
      });
      if (theme == null) throw StateError('Voice theme capture did not finish');
      final client = MockClient((request) async {
        if (request.url.path == '/api/v1/notifications/settings' &&
            request.method == 'GET') {
          return http.Response(
            jsonEncode({
              'notification_settings': {
                'profile_id': 'capture-profile',
                'scope_type': 'global',
                'enabled': true,
                'suppress_types_json': jsonEncode(['reaction']),
              },
            }),
            200,
          );
        }
        if (request.url.path == '/api/v1/notifications/quiet-hours' &&
            request.method == 'GET') {
          return http.Response(
            jsonEncode({
              'quiet_hours': {
                'enabled': true,
                'start_time': '23:00',
                'end_time': '08:00',
                'timezone': 'UTC',
                'override_mentions': true,
              },
            }),
            200,
          );
        }
        return http.Response('', 204);
      });

      for (final viewport in [
        (name: 'desktop', size: const Size(1280, 800)),
        (name: 'mobile', size: const Size(390, 844)),
      ]) {
        tester.view.physicalSize = viewport.size;
        tester.view.devicePixelRatio = 1;
        await tester.pumpWidget(
          RepaintBoundary(
            key: _captureBoundaryKey,
            child: ProviderScope(
              overrides: [
                ...voiceAppTestOverrides(client: client),
                notificationQuietHoursStorageProvider.overrideWithValue(
                  _CaptureQuietHoursStorage(),
                ),
                pushNotificationsControllerProvider.overrideWith(
                  _CapturePushNotificationsController.new,
                ),
              ],
              child: MaterialApp(
                debugShowCheckedModeBanner: false,
                theme: theme,
                locale: const Locale('en'),
                localizationsDelegates: AppLocalizations.localizationsDelegates,
                supportedLocales: AppLocalizations.supportedLocales,
                home: const NotificationSettingsScreen(),
              ),
            ),
          ),
        );
        await tester.pumpAndSettle();
        final screenScrollable = find.descendant(
          of: find.byKey(NotificationSettingsScreen.screenKey),
          matching: find.byType(Scrollable),
        );
        tester
            .state<ScrollableState>(screenScrollable.first)
            .position
            .jumpTo(0);
        await tester.pumpAndSettle();
        await _capture(
          tester,
          directory,
          '${viewport.name}-top',
          viewport.size,
        );
        await tester.ensureVisible(
          find.byKey(const Key('notification_type_voice_member_joined')),
        );
        await tester.pumpAndSettle();
        await _capture(
          tester,
          directory,
          '${viewport.name}-events',
          viewport.size,
        );
      }
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    },
    skip: captureDirectory == null || captureDirectory.isEmpty,
  );
}

class _CaptureQuietHoursStorage extends NotificationQuietHoursStorage {
  @override
  Future<VoiceQuietHours> read(String profileId) async =>
      VoiceQuietHours.defaults;

  @override
  Future<void> write(String profileId, VoiceQuietHours quietHours) async {}
}

class _CapturePushNotificationsController extends PushNotificationsController {
  _CapturePushNotificationsController(super.ref);

  @override
  Future<PushPermissionStatus> getPermissionStatus() async =>
      PushPermissionStatus.granted;

  @override
  Future<PushPermissionStatus> requestPermissionAndRegister() async =>
      PushPermissionStatus.granted;
}

Future<void> _loadCaptureFonts() async {
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
  final bytes = await File(
    [
      flutterRoot,
      'bin',
      'cache',
      'artifacts',
      'material_fonts',
      'MaterialIcons-Regular.otf',
    ].join(Platform.pathSeparator),
  ).readAsBytes();
  final icons = ByteData.sublistView(Uint8List.fromList(bytes));
  await (FontLoader('MaterialIcons')..addFont(Future.value(icons))).load();
}

Future<void> _capture(
  WidgetTester tester,
  Directory directory,
  String name,
  Size expectedSize,
) async {
  final boundary = tester.renderObject<RenderRepaintBoundary>(
    find.byKey(_captureBoundaryKey),
  );
  final written = await tester.runAsync(() async {
    final image = await boundary.toImage(pixelRatio: 1);
    try {
      if (image.width != expectedSize.width.toInt() ||
          image.height != expectedSize.height.toInt()) {
        throw StateError(
          'Capture does not match requested viewport $expectedSize',
        );
      }
      final data = await image.toByteData(format: ui.ImageByteFormat.png);
      if (data == null) throw StateError('PNG encoding returned null');
      await File(
        '${directory.path}/notifications-$name.png',
      ).writeAsBytes(data.buffer.asUint8List(), flush: true);
      return true;
    } finally {
      image.dispose();
    }
  });
  if (written != true) {
    throw StateError('Notification screenshot did not finish');
  }
}
