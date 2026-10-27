import 'dart:async';
import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/stories_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/stories_providers.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/ui/stories/story_archive_screen.dart';
import 'support/auth_test_overrides.dart';

const _root = Key('archive_capture_root');
const _directory = String.fromEnvironment('VOICE_ARCHIVE_CAPTURE_DIR');

void main() {
  testWidgets('archive H/V actual layout, states and scaled text', (
    tester,
  ) async {
    final theme = await tester.runAsync(() async {
      await (FontLoader(VoiceTheme.fontFamily)
            ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
            ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf')))
          .load();
      final flutterRoot =
          Platform.environment['FLUTTER_ROOT'] ??
          File(
            Platform.resolvedExecutable,
          ).parent.parent.parent.parent.parent.path;
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
    });
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    for (final mobile in [false, true]) {
      tester.view.physicalSize = mobile
          ? const Size(390, 844)
          : const Size(1280, 800);
      tester.view.devicePixelRatio = 1;
      for (final scenario in [
        'populated',
        'scaled',
        'empty',
        'error',
        'loading',
      ]) {
        final pending = Completer<List<StoryData>>();
        final container = ProviderContainer(
          overrides: [
            ...voiceAppTestOverrides(
              client: MockClient((_) async => http.Response('{}', 200)),
            ),
            storyArchiveProvider.overrideWith((ref) async {
              if (scenario == 'loading') return pending.future;
              if (scenario == 'error') throw StateError('private diagnostic');
              if (scenario == 'empty') return [];
              return [
                for (final (index, title) in [
                  'Рейд в пятницу',
                  'Победа 13:9',
                  'Design voice',
                  'Ищем саппорта',
                  'Стендап',
                  'Лучший момент',
                ].indexed)
                  StoryData(
                    id: 'archive-$index',
                    authorProfileId: 'prof-test',
                    type: 'text',
                    textContent: title,
                  ),
              ];
            }),
          ],
        );
        await tester.pumpWidget(
          RepaintBoundary(
            key: _root,
            child: UncontrolledProviderScope(
              container: container,
              child: MaterialApp(
                debugShowCheckedModeBanner: false,
                theme: theme,
                locale: const Locale('ru'),
                localizationsDelegates: AppLocalizations.localizationsDelegates,
                supportedLocales: AppLocalizations.supportedLocales,
                builder: (context, child) => MediaQuery(
                  data: MediaQuery.of(context).copyWith(
                    textScaler: TextScaler.linear(
                      scenario == 'scaled' ? 1.5 : 1,
                    ),
                  ),
                  child: child!,
                ),
                home: const StoryArchiveScreen(),
              ),
            ),
          ),
        );
        if (scenario == 'loading') {
          await tester.pump();
        } else {
          await tester.pumpAndSettle();
        }
        expect(tester.takeException(), isNull);
        if (scenario == 'populated' || scenario == 'scaled') {
          final grid = tester.widget<GridView>(find.byType(GridView));
          expect(
            (grid.gridDelegate as SliverGridDelegateWithFixedCrossAxisCount)
                .crossAxisCount,
            mobile ? 2 : 3,
          );
          expect(
            find
                .byKey(const Key('story_archive_add_highlight_archive-0'))
                .hitTestable(),
            findsOneWidget,
          );
        }
        expect(find.textContaining('private diagnostic'), findsNothing);
        if (_directory.isNotEmpty) {
          final boundary = tester.renderObject<RenderRepaintBoundary>(
            find.byKey(_root),
          );
          await tester.runAsync(() async {
            final image = await boundary.toImage(pixelRatio: 1);
            try {
              final bytes = await image.toByteData(
                format: ui.ImageByteFormat.png,
              );
              final file = File(
                '$_directory/archive-$scenario-${mobile ? 'v' : 'h'}.png',
              );
              await file.parent.create(recursive: true);
              await file.writeAsBytes(bytes!.buffer.asUint8List(), flush: true);
            } finally {
              image.dispose();
            }
          });
        }
        await tester.pumpWidget(const SizedBox.shrink());
        container.dispose();
        if (!pending.isCompleted) pending.complete([]);
      }
    }
  });
}
