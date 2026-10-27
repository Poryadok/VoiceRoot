import 'dart:async';
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
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/backend/space_catalog_client.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/space_catalog_providers.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/ui/space/space_catalog_screen.dart';
import 'package:voice_frontend/ui/space/join_space_invite_sheet.dart';
import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  test(
    'membership proof reads every page before establishing nonmembership',
    () async {
      final container = ProviderContainer(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((req) async {
              expect(req.url.path, '/api/v1/spaces');
              expect(req.url.queryParameters['cursor'], 'next');
              return http.Response(
                '{"space_list":{"spaces":[{"id":"page2","name":"Second page"}]}}',
                200,
              );
            }),
          ),
          mySpacesProvider.overrideWith(
            (_) async => const SpaceListData(spaces: [], nextCursor: 'next'),
          ),
        ],
      );
      addTearDown(container.dispose);
      final subscription = container.listen(
        catalogMembershipsProvider,
        (_, _) {},
      );
      addTearDown(subscription.close);
      expect(await container.read(catalogMembershipsProvider.future), {
        'page2',
      });
    },
  );
  Future<void> mount(
    WidgetTester tester,
    MockClient client, {
    Size size = const Size(1280, 800),
    double textScale = 1,
    List<Override> overrides = const [],
  }) async {
    await tester.binding.setSurfaceSize(size);
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceAppTestOverrides(client: client),
          catalogMembershipsProvider.overrideWith((ref) async => {'joined'}),
          ...overrides,
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          builder: (context, child) => MediaQuery(
            data: MediaQuery.of(
              context,
            ).copyWith(textScaler: TextScaler.linear(textScale)),
            child: child!,
          ),
          home: const RepaintBoundary(
            key: Key('catalog_capture'),
            child: SpaceCatalogScreen(),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
  }

  http.Response cards({String next = ''}) => http.Response(
    jsonEncode({
      'space_list': {
        'spaces': [
          {
            'id': 'joined',
            'name': 'Joined space',
            'member_count': 9,
            'is_verified': true,
            'description': 'Existing community',
          },
          {
            'id': 'new',
            'name': 'New space',
            'description': 'Public card',
            'visibility': 'public',
          },
        ],
        'next_cursor': next,
      },
    }),
    200,
  );

  testWidgets(
    'public join handles denial then opens only confirmed membership; invite uses existing sheet',
    (tester) async {
      SharedPreferences.setMockInitialValues({});
      var joins = 0;
      await mount(
        tester,
        MockClient((req) async {
          if (req.method == 'POST') {
            joins++;
            if (joins == 1) return http.Response('{}', 403);
            return http.Response(
              '{"space_membership":{"space_id":"new"}}',
              200,
            );
          }
          return cards();
        }),
      );
      await tester.tap(find.byKey(SpaceCatalogScreen.joinKey('new')));
      await tester.pumpAndSettle();
      expect(joins, 1);
      expect(
        find.text(
          'Could not join this space. Check your memberships or try again.',
        ),
        findsOneWidget,
      );
      final container = ProviderScope.containerOf(
        tester.element(find.byType(SpaceCatalogScreen)),
      );
      expect(container.read(selectedSpaceIdProvider), isNull);
      await tester.tap(find.byKey(SpaceCatalogScreen.joinKey('new')));
      await tester.pumpAndSettle();
      expect(container.read(selectedSpaceIdProvider), 'new');
      expect(joins, 2);
      await mount(
        tester,
        MockClient((req) async {
          expect(req.method, 'GET');
          return http.Response(
            '{"space_list":{"spaces":[{"id":"invite","name":"Invite community","visibility":"invite_only"},{"id":"unknown","name":"Unknown admission"}]}}',
            200,
          );
        }),
      );
      expect(find.byKey(SpaceCatalogScreen.joinKey('unknown')), findsNothing);
      await tester.tap(find.byKey(SpaceCatalogScreen.joinKey('invite')));
      await tester.pumpAndSettle();
      expect(find.byType(JoinSpaceInviteSheet), findsOneWidget);
    },
  );

  testWidgets(
    'catalog previews safe data, shows verified only from server, and adapts H/V',
    (tester) async {
      for (final size in [const Size(1280, 800), const Size(390, 844)]) {
        await mount(tester, MockClient((_) async => cards()), size: size);
        expect(find.text('Joined space'), findsOneWidget);
        expect(find.byTooltip('Verified space'), findsOneWidget);
        final first = tester.getTopLeft(
          find.byKey(SpaceCatalogScreen.cardKey('joined')),
        );
        final second = tester.getTopLeft(
          find.byKey(SpaceCatalogScreen.cardKey('new')),
        );
        expect(
          size.width > 700 ? first.dy == second.dy : second.dy > first.dy,
          isTrue,
        );
        await tester.tap(find.text('New space'));
        await tester.pumpAndSettle();
        expect(find.byKey(SpaceCatalogScreen.previewKey), findsOneWidget);
        expect(find.text('Public card'), findsWidgets);
        expect(tester.takeException(), isNull);
        await tester.tap(find.byTooltip('Close').last);
        await tester.pumpAndSettle();
      }
    },
  );

  testWidgets('join completion from a replaced session cannot navigate', (
    tester,
  ) async {
    final token = StateProvider<String>((_) => 'Bearer old');
    final joined = Completer<http.Response>();
    await mount(
      tester,
      MockClient((req) async {
        if (req.method == 'POST') return joined.future;
        return cards();
      }),
      overrides: [
        authorizationHeaderProvider.overrideWith((ref) => ref.watch(token)),
      ],
    );
    final container = ProviderScope.containerOf(
      tester.element(find.byType(SpaceCatalogScreen)),
    );
    await tester.tap(find.byKey(SpaceCatalogScreen.joinKey('new')));
    await tester.pump();
    container.read(token.notifier).state = 'Bearer new';
    await tester.pump();
    joined.complete(
      http.Response('{"space_membership":{"space_id":"new"}}', 200),
    );
    await tester.pumpAndSettle();
    expect(container.read(selectedSpaceIdProvider), isNull);
  });

  testWidgets(
    'search replaces pending results and pagination retry retains cards',
    (tester) async {
      final stale = Completer<http.Response>();
      var failedPage = true;
      await mount(
        tester,
        MockClient((req) async {
          if (req.url.queryParameters['q'] == 'old') return stale.future;
          if (req.url.queryParameters['q'] == 'latest') {
            return http.Response('{"space_list":{}}', 200);
          }
          if (req.url.queryParameters['cursor'] == 'next') {
            if (failedPage) return http.Response('{}', 503);
            return http.Response(
              '{"space_list":{"spaces":[{"id":"page2","name":"Page two"}]}}',
              200,
            );
          }
          return cards(next: 'next');
        }),
      );
      await tester.tap(find.byKey(SpaceCatalogScreen.moreKey));
      await tester.pumpAndSettle();
      expect(find.text('Joined space'), findsOneWidget);
      failedPage = false;
      await tester.tap(find.text('Try again').last);
      await tester.pumpAndSettle();
      expect(find.text('Page two'), findsOneWidget);
      await tester.enterText(find.byKey(SpaceCatalogScreen.searchKey), 'old');
      await tester.pump(const Duration(milliseconds: 350));
      await tester.enterText(
        find.byKey(SpaceCatalogScreen.searchKey),
        'latest',
      );
      await tester.pump(const Duration(milliseconds: 350));
      await tester.pumpAndSettle();
      stale.complete(cards());
      await tester.pumpAndSettle();
      expect(find.text('Joined space'), findsNothing);
      expect(find.text('No spaces found'), findsOneWidget);
    },
  );

  testWidgets(
    'membership failure has recovery and never invents Open membership',
    (tester) async {
      var failed = true;
      await mount(
        tester,
        MockClient((_) async => cards()),
        overrides: [
          catalogMembershipsProvider.overrideWith((ref) async {
            if (failed) throw const SpaceCatalogFailure();
            return {'joined'};
          }),
        ],
      );
      expect(find.text('Could not load your memberships'), findsOneWidget);
      expect(find.text('Open space'), findsNothing);
      failed = false;
      await tester.tap(find.text('Try again'));
      await tester.pumpAndSettle();
      expect(find.text('Open space'), findsOneWidget);
    },
  );

  testWidgets(
    'auth change removes prior cards and closes preview while new request loads',
    (tester) async {
      final token = StateProvider<String>((ref) => 'Bearer old');
      final newPage = Completer<http.Response>();
      await mount(
        tester,
        MockClient((req) async {
          if (req.headers['authorization'] == 'Bearer new') {
            return newPage.future;
          }
          return cards();
        }),
        overrides: [
          authorizationHeaderProvider.overrideWith((ref) => ref.watch(token)),
        ],
      );
      await tester.tap(find.text('New space'));
      await tester.pumpAndSettle();
      final container = ProviderScope.containerOf(
        tester.element(find.byType(SpaceCatalogScreen)),
      );
      container.read(token.notifier).state = 'Bearer new';
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 400));
      expect(find.byKey(SpaceCatalogScreen.previewKey), findsNothing);
      expect(find.text('New space'), findsNothing);
      newPage.complete(http.Response('{"space_list":{}}', 200));
      await tester.pumpAndSettle();
    },
  );

  testWidgets('keyboard search, semantics and enlarged text retain controls', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    await mount(
      tester,
      MockClient((_) async => cards()),
      size: const Size(390, 844),
      textScale: 2,
    );
    expect(find.bySemanticsLabel(RegExp('Verified space')), findsWidgets);
    await tester.tap(find.byKey(SpaceCatalogScreen.searchKey));
    await tester.sendKeyEvent(LogicalKeyboardKey.tab);
    expect(FocusManager.instance.primaryFocus, isNotNull);
    await tester.pump();
    expect(tester.takeException(), isNull);
    semantics.dispose();
  });

  testWidgets('capture actual light token H and V widgets', (tester) async {
    final output = Platform.environment['VOICE_CATALOG_CAPTURE_DIR'];
    if (output == null) return;
    await tester.runAsync(() async {
      await (FontLoader(VoiceTheme.fontFamily)
            ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
            ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf')))
          .load();
      final root = Platform.environment['FLUTTER_ROOT']!;
      final bytes = await File(
        '$root/bin/cache/artifacts/material_fonts/MaterialIcons-Regular.otf',
      ).readAsBytes();
      await (FontLoader(
        'MaterialIcons',
      )..addFont(Future.value(ByteData.sublistView(bytes)))).load();
    });
    final catalog = await tester.runAsync(VoiceTokenCatalog.load);
    final theme = await VoiceTheme.build(
      catalog: catalog!,
      mode: VoiceThemeMode.light,
      profileAccent: catalog.profileAccentAt(0),
    );
    for (final size in [const Size(1280, 800), const Size(390, 844)]) {
      await mount(tester, MockClient((_) async => cards()), size: size);
      // Preserve the actual provider tree and replace only its inherited theme.
      final app = tester.widget<MaterialApp>(find.byType(MaterialApp));
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            ...voiceAppTestOverrides(client: MockClient((_) async => cards())),
            catalogMembershipsProvider.overrideWith((_) async => {'joined'}),
          ],
          child: MaterialApp(
            theme: theme,
            locale: const Locale('en'),
            localizationsDelegates: app.localizationsDelegates,
            supportedLocales: app.supportedLocales,
            home: const RepaintBoundary(
              key: Key('catalog_capture'),
              child: SpaceCatalogScreen(),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      final boundary = tester.renderObject<RenderRepaintBoundary>(
        find.byKey(const Key('catalog_capture')),
      );
      await tester.runAsync(() async {
        final image = await boundary.toImage(pixelRatio: 1);
        final data = await image.toByteData(format: ui.ImageByteFormat.png);
        final file = File('$output/catalog-${size.width.toInt()}.png');
        await file.parent.create(recursive: true);
        await file.writeAsBytes(data!.buffer.asUint8List());
        image.dispose();
      });
      expect(tester.takeException(), isNull);
    }
  });
}
