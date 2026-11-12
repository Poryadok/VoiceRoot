import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/matchmaking_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/matchmaking/game_detail_screen.dart';
import 'package:voice_frontend/ui/matchmaking/queue_search_screen.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

CatalogGame _game({bool rolesRequired = true, bool rankRequired = true}) {
  return CatalogGame(
    id: 'g-v3',
    name: 'Detail Game',
    status: 'active',
    config: GameConfig.fromJson({
      'genre': 'MOBA',
      'regions': const ['eu', 'na'],
      'modes': [
        {
          'name': 'Ranked',
          'slots': 10,
          'party_size_min': 1,
          'party_size_max': 5,
          'roles_required': rolesRequired,
          'rank_required': rankRequired,
          'roles': [
            {'name': 'Carry', 'required': true},
            {'name': 'Support', 'required': false},
          ],
          'ranks': [
            {'name': 'Bronze', 'value': 0},
            {'name': 'Silver', 'value': 100},
            {'name': 'Gold', 'value': 200},
          ],
        },
      ],
    }),
  );
}

http.Response _json(Object body, [int status = 200]) => http.Response(
  jsonEncode(body),
  status,
  headers: {'content-type': 'application/json'},
);

Widget _app(GameDetailScreen screen, http.Client client) {
  return ProviderScope(
    overrides: voiceAppTestOverrides(client: client),
    child: MaterialApp(
      debugShowCheckedModeBanner: false,
      theme: voiceTestTheme(),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: screen,
    ),
  );
}

void main() {
  http.Client clientForTests() => MockClient((request) async => _json({}));

  Finder elementIs(Object? element) => find.byElementPredicate((e) => e == element);

  Element? findMenuRow(String value) {
    for (final element in find
        .descendant(of: find.byType(InkWell), matching: find.text(value))
        .evaluate()) {
      final inChip = find
          .ancestor(of: elementIs(element), matching: find.byType(Chip))
          .evaluate()
          .toList()
          .length;
      if (inChip == 0) {
        return element;
      }
    }
    return null;
  }

  Future<void> openMenu(WidgetTester tester, String key) async {
    await tester.tap(find.byKey(Key(key)));
    await tester.pumpAndSettle();
  }

  Future<void> select(WidgetTester tester, String key, String value) async {
    var row = findMenuRow(value);
    if (row == null) {
      await openMenu(tester, key);
      row = findMenuRow(value);
    }
    expect(row, isNotNull, reason: 'menu row for $value');
    await tester.tap(elementIs(row));
    await tester.pumpAndSettle();
  }

  String startError() => 'Could not start search';

  Finder startButton() => find.byKey(
    Key(GameDetailScreen.startButtonKey('Ranked')),
  );

  testWidgets(
    'the region, own rank, bounds and role pickers render per mode card in H/V',
    (tester) async {
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      for (final size in [const Size(1280, 800), const Size(390, 844)]) {
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        await tester.binding.setSurfaceSize(size);
        final client = clientForTests();
        addTearDown(client.close);
        await tester.pumpWidget(
          _app(GameDetailScreen(game: _game(), spaceId: 'space-v3'), client),
        );
        await tester.pumpAndSettle();
        expect(
          find.byKey(GameDetailScreen.regionPickerKey),
          findsOneWidget,
          reason: '$size region picker',
        );
        for (final key in [
          GameDetailScreen.rolePickerKey('Ranked'),
          GameDetailScreen.ownRankPickerKey('Ranked'),
          GameDetailScreen.soughtRankMinKey('Ranked'),
          GameDetailScreen.soughtRankMaxKey('Ranked'),
        ]) {
          expect(
            find.byKey(Key(key)),
            findsOneWidget,
            reason: '$size $key',
          );
        }
        expect(
          startButton().hitTestable(),
          findsOneWidget,
          reason: '$size start',
        );
        for (final label in [
          'Region',
          'Your role',
          'Your rank',
          'Min rank sought',
          'Max rank sought',
        ]) {
          expect(find.text(label), findsOneWidget, reason: '$size $label');
        }
        expect(find.text('Start queue'), findsOneWidget);
        expect(find.text('Carry'), findsOneWidget);
        expect(find.text('Gold (200)'), findsOneWidget);
        expect(find.text(startError()), findsNothing);
      }
      await tester.binding.setSurfaceSize(null);
    },
  );

  testWidgets(
    'Start routes to QueueSearch even with no criterion picked yet',
    (tester) async {
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      for (final size in [const Size(1280, 800), const Size(390, 844)]) {
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        await tester.binding.setSurfaceSize(size);
        final client = clientForTests();
        addTearDown(client.close);
        await tester.pumpWidget(
          _app(GameDetailScreen(game: _game(), spaceId: 'space-v3'), client),
        );
        await tester.pumpAndSettle();
        await tester.tap(startButton());
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 350));
        await tester.pumpAndSettle();
        expect(
          find.byType(QueueSearchScreen),
          findsOneWidget,
          reason: '$size unselected criteria route stays open',
        );
        expect(find.byType(GameDetailScreen), findsNothing);
        await tester.pumpWidget(const SizedBox.shrink());
      }
      await tester.binding.setSurfaceSize(null);
    },
  );

  testWidgets(
    'role and own-rank pickers persist values and Start stays open',
    (tester) async {
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      for (final size in [const Size(1280, 800), const Size(390, 844)]) {
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        await tester.binding.setSurfaceSize(size);
        final client = clientForTests();
        addTearDown(client.close);
        await tester.pumpWidget(
          _app(GameDetailScreen(game: _game(), spaceId: 'space-v3'), client),
        );
        await tester.pumpAndSettle();
        await select(
          tester,
          GameDetailScreen.regionPickerKeyName,
          'eu',
        );
        await select(
          tester,
          GameDetailScreen.rolePickerKey('Ranked'),
          'Carry',
        );
        expect(
          find.text('Carry'),
          findsNWidgets(2),
          reason: '$size role readout',
        );
        await select(
          tester,
          GameDetailScreen.ownRankPickerKey('Ranked'),
          'Silver',
        );
        expect(
          find.text('Silver'),
          findsOneWidget,
          reason: '$size own rank readout',
        );
        expect(
          find.byKey(
            Key(GameDetailScreen.soughtRankMinKey('Ranked')),
          ),
          findsOneWidget,
          reason: '$size bounds stay optional',
        );
        await tester.tap(startButton());
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 350));
        await tester.pumpAndSettle();
        expect(
          find.byType(QueueSearchScreen),
          findsOneWidget,
          reason: '$size route',
        );
        expect(find.byType(GameDetailScreen), findsNothing);
        await tester.pumpWidget(const SizedBox.shrink());
      }
      await tester.binding.setSurfaceSize(null);
    },
  );

  testWidgets(
    'out-of-order sought bounds stay accepted and Start still routes',
    (tester) async {
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      for (final size in [const Size(1280, 800), const Size(390, 844)]) {
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        await tester.binding.setSurfaceSize(size);
        final client = clientForTests();
        addTearDown(client.close);
        await tester.pumpWidget(
          _app(GameDetailScreen(game: _game(), spaceId: 'space-v3'), client),
        );
        await tester.pumpAndSettle();
        await select(
          tester,
          GameDetailScreen.regionPickerKeyName,
          'eu',
        );
        await select(
          tester,
          GameDetailScreen.rolePickerKey('Ranked'),
          'Carry',
        );
        await select(
          tester,
          GameDetailScreen.ownRankPickerKey('Ranked'),
          'Silver',
        );
        await select(
          tester,
          GameDetailScreen.soughtRankMinKey('Ranked'),
          'Silver',
        );
        await select(
          tester,
          GameDetailScreen.soughtRankMaxKey('Ranked'),
          'Bronze',
        );
        expect(
          find.text('Bronze'),
          findsOneWidget,
          reason: '$size max readout stays below min',
        );
        await tester.tap(startButton());
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 350));
        await tester.pumpAndSettle();
        expect(
          find.byType(QueueSearchScreen),
          findsOneWidget,
          reason: '$size route open with out-of-order bounds',
        );
        await tester.pumpWidget(const SizedBox.shrink());
      }
      await tester.binding.setSurfaceSize(null);
    },
  );

  testWidgets(
    'a mode without required role or rank starts with the region and the '
    'no-preference option',
    (tester) async {
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      for (final size in [const Size(1280, 800), const Size(390, 844)]) {
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        await tester.binding.setSurfaceSize(size);
        final client = clientForTests();
        addTearDown(client.close);
        await tester.pumpWidget(
          _app(
            GameDetailScreen(
              game: _game(rolesRequired: false, rankRequired: false),
              spaceId: 'space-v3',
            ),
            client,
          ),
        );
        await tester.pumpAndSettle();
        for (final key in [
          GameDetailScreen.ownRankPickerKey('Ranked'),
          GameDetailScreen.soughtRankMinKey('Ranked'),
          GameDetailScreen.soughtRankMaxKey('Ranked'),
        ]) {
          expect(
            find.byKey(Key(key)),
            findsNothing,
            reason: '$size rank controls only when required',
          );
        }
        await openMenu(
          tester,
          GameDetailScreen.rolePickerKey('Ranked'),
        );
        expect(
          find.text('No role preference'),
          findsOneWidget,
          reason: '$size role clear option',
        );
        expect(
          find.text('Carry'),
          findsAtLeastNWidgets(2),
          reason: '$size role rows',
        );
        expect(
          find.text('Support'),
          findsAtLeastNWidgets(2),
          reason: '$size role rows',
        );
        await select(
          tester,
          GameDetailScreen.rolePickerKey('Ranked'),
          'No role preference',
        );
        await select(
          tester,
          GameDetailScreen.regionPickerKeyName,
          'eu',
        );
        await tester.tap(startButton());
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 350));
        await tester.pumpAndSettle();
        expect(
          find.byType(QueueSearchScreen),
          findsOneWidget,
          reason: '$size route',
        );
        await tester.pumpWidget(const SizedBox.shrink());
      }
      await tester.binding.setSurfaceSize(null);
    },
  );
}