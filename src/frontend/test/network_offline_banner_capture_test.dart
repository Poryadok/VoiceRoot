import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/ui/core/voice_compact_banner.dart';

const _captureDirectoryVariable = 'VOICE_NETWORK_OFFLINE_CAPTURE_DIR';
const _captureBoundaryKey = Key('network_offline_banner_capture');

void main() {
  testWidgets('renders the reconnect banner at H/V reference viewports', (
    tester,
  ) async {
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });
    final captureDirectory = Platform.environment[_captureDirectoryVariable]
        ?.trim();
    final shouldCapture = captureDirectory?.isNotEmpty ?? false;

    if (shouldCapture) {
      await _loadProductionFonts(tester);
    }
    final catalog = await VoiceTokenCatalog.load();
    final theme = await VoiceTheme.build(
      catalog: catalog,
      mode: VoiceThemeMode.dark,
      profileAccent: catalog.profileAccentAt(0),
    );

    for (final viewport in [
      (name: 'h', size: const Size(1280, 800)),
      (name: 'v', size: const Size(390, 844)),
    ]) {
      tester.view.devicePixelRatio = 1;
      tester.view.physicalSize = viewport.size;
      await tester.pumpWidget(
        MaterialApp(
          theme: theme,
          locale: const Locale('en'),
          supportedLocales: AppLocalizations.supportedLocales,
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          home: Builder(
            builder: (context) {
              final media = MediaQuery.of(context);
              final l10n = AppLocalizations.of(context)!;
              return MediaQuery(
                data: media.copyWith(textScaler: const TextScaler.linear(1.5)),
                child: Scaffold(
                  body: RepaintBoundary(
                    key: _captureBoundaryKey,
                    child: ColoredBox(
                      color: theme.scaffoldBackgroundColor,
                      child: SafeArea(
                        child: Align(
                          alignment: Alignment.topCenter,
                          child: VoiceCompactBanner(
                            message: l10n.chatRealtimeReconnecting,
                            detail: l10n.networkReconnectDetails,
                            actionLabel: l10n.commonRetry,
                            onAction: () {},
                            onDismiss: () {},
                          ),
                        ),
                      ),
                    ),
                  ),
                ),
              );
            },
          ),
        ),
      );

      expect(find.text('Reconnecting…'), findsOneWidget);
      expect(
        find.text(
          'Drafts stay on this device. Realtime updates resume after reconnection.',
        ),
        findsOneWidget,
      );
      expect(find.text('Try again'), findsOneWidget);
      expect(find.byTooltip('Close'), findsOneWidget);
      expect(tester.takeException(), isNull);

      if (shouldCapture) {
        await _writeCapture(
          tester,
          '${captureDirectory!}${Platform.pathSeparator}network-offline-${viewport.name}.png',
          expectedSize: viewport.size,
        );
      }
    }
  });
}

Future<void> _loadProductionFonts(WidgetTester tester) async {
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
    final iconBytes = await iconFont.readAsBytes();
    final iconData = ByteData.sublistView(iconBytes);
    await (FontLoader(
      'MaterialIcons',
    )..addFont(Future<ByteData>.value(iconData))).load();
    return true;
  });
  if (loaded != true) {
    throw StateError('Production capture fonts did not load');
  }
}

Future<void> _writeCapture(
  WidgetTester tester,
  String path, {
  required Size expectedSize,
}) async {
  final boundary = tester.renderObject<RenderRepaintBoundary>(
    find.byKey(_captureBoundaryKey),
  );
  final written = await tester.runAsync(() async {
    final image = await boundary.toImage(pixelRatio: 1);
    try {
      if (image.width != expectedSize.width.toInt() ||
          image.height != expectedSize.height.toInt()) {
        throw StateError('Capture did not match the requested viewport size');
      }
      final png = await image.toByteData(format: ui.ImageByteFormat.png);
      if (png == null) throw StateError('PNG encoding returned null');
      await File(path).writeAsBytes(png.buffer.asUint8List(), flush: true);
      return true;
    } finally {
      image.dispose();
    }
  });
  if (written != true) {
    throw StateError('Network banner capture did not complete');
  }
}
