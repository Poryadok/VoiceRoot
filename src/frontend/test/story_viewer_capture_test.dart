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
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/social_providers.dart';
import 'package:voice_frontend/state/stories_providers.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/ui/stories/story_viewer_screen.dart';

import 'support/auth_test_overrides.dart';

const _directory = String.fromEnvironment('VOICE_STORY_VIEWER_CAPTURE_DIR');
const _root = Key('story_viewer_capture_root');

void main() {
  testWidgets(
    'story viewer H/V actual fonts, controls and readable scaled text',
    (tester) async {
      final setup = await tester.runAsync(() async {
        await (FontLoader(VoiceTheme.fontFamily)
              ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
              ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf')))
            .load();
        final flutterRoot = Platform.environment['FLUTTER_ROOT']!;
        final icons = await File(
          '$flutterRoot/bin/cache/artifacts/material_fonts/MaterialIcons-Regular.otf',
        ).readAsBytes();
        await (FontLoader(
          'MaterialIcons',
        )..addFont(Future.value(ByteData.sublistView(icons)))).load();
        final catalog = await VoiceTokenCatalog.load();
        final theme = await VoiceTheme.build(
          catalog: catalog,
          mode: VoiceThemeMode.light,
          profileAccent: catalog.profileAccentAt(0),
        );
        return (catalog: catalog, theme: theme);
      });
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      for (final mobile in [false, true]) {
        tester.view.physicalSize = mobile
            ? const Size(390, 844)
            : const Size(1280, 800);
        tester.view.devicePixelRatio = 1;
        for (final scenario in ['text', 'lfp', 'inactive', 'scaled']) {
          final story = StoryData(
            id: 'capture-story',
            authorProfileId: 'capture-author',
            type: 'text',
            textContent: scenario == 'text'
                ? 'Design voice после релиза'
                : 'Ищем пятого до 22:00',
            isLookingForParty: scenario != 'text',
            lfpCriteriaJson: '{"mode":"Ranked","region":"Europe"}',
            createdAt: DateTime.utc(2026, 11, 3, 12),
            expiresAt: scenario == 'inactive'
                ? DateTime.utc(2020)
                : DateTime.utc(2099),
            textStyleJson: '{"background":"accent"}',
          );
          final container = ProviderContainer(
            overrides: [
              ...voiceAppTestOverrides(
                client: MockClient((_) async => http.Response('', 204)),
              ),
              storyDetailProvider(
                'capture-story',
              ).overrideWith((ref) async => story),
              voiceTokenCatalogProvider.overrideWith(
                (ref) async => setup!.catalog,
              ),
              profileProvider('capture-author').overrideWith(
                (ref) async => const VoiceProfile(
                  id: 'capture-author',
                  accountId: 'capture-account',
                  username: 'alex',
                  discriminator: '0001',
                  displayName: 'Alex',
                ),
              ),
            ],
          );
          await tester.pumpWidget(
            RepaintBoundary(
              key: _root,
              child: UncontrolledProviderScope(
                container: container,
                child: MaterialApp(
                  debugShowCheckedModeBanner: false,
                  theme: setup!.theme,
                  locale: const Locale('ru'),
                  localizationsDelegates:
                      AppLocalizations.localizationsDelegates,
                  supportedLocales: AppLocalizations.supportedLocales,
                  builder: (context, child) => MediaQuery(
                    data: MediaQuery.of(context).copyWith(
                      textScaler: TextScaler.linear(
                        scenario == 'scaled' ? 1.5 : 1,
                      ),
                    ),
                    child: child!,
                  ),
                  home: const StoryViewerScreen(
                    storyIds: [
                      'capture-story',
                      'capture-story',
                      'capture-story',
                    ],
                  ),
                ),
              ),
            ),
          );
          await tester.pumpAndSettle();
          expect(tester.takeException(), isNull);
          expect(
            find.byKey(StoryViewerScreen.replyButtonKey).hitTestable(),
            findsOneWidget,
          );
          expect(
            find.byKey(const Key('story_viewer_author')).hitTestable(),
            findsOneWidget,
          );
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
                  '$_directory/viewer-$scenario-${mobile ? 'v' : 'h'}.png',
                );
                await file.parent.create(recursive: true);
                await file.writeAsBytes(
                  bytes!.buffer.asUint8List(),
                  flush: true,
                );
              } finally {
                image.dispose();
              }
            });
          }
          await tester.pumpWidget(const SizedBox.shrink());
          container.dispose();
        }
      }
    },
    skip: _directory.isEmpty,
  );
}
