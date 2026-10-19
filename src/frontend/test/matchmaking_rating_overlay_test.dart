import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/matchmaking_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/matchmaking_search_controller.dart';
import 'package:voice_frontend/ui/matchmaking/match_rating_overlay.dart';

import 'support/voice_test_theme.dart';

MatchData _completedMatch() {
  return const MatchData(
    id: 'match-1',
    gameId: 'g-val',
    mode: 'Duo',
    region: 'eu',
    status: 'completed',
    profileIds: ['p1', 'p2'],
    gameName: 'Valorant',
  );
}

void main() {
  testWidgets(
    'retry preserves accepted votes and retries only failed teammate',
    (tester) async {
      final votes = <String>[];
      var done = 0;
      await tester.pumpWidget(
        _ratingFixture(
          teammates: const [
            RatedTeammate(profileId: 'p2', displayName: 'First'),
            RatedTeammate(profileId: 'p3', displayName: 'Second'),
          ],
          onRate: (id, stars) async {
            votes.add(id);
            return id == 'p2' || votes.where((value) => value == id).length > 1;
          },
          onDone: () => done++,
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byTooltip('4').at(0));
      await tester.tap(find.byTooltip('5').at(1));
      await tester.tap(find.byKey(MatchRatingOverlay.submitButtonKey));
      await tester.pumpAndSettle();
      expect(done, 0);
      expect(votes, ['p2', 'p3']);

      await tester.tap(find.byKey(MatchRatingOverlay.submitButtonKey));
      await tester.pumpAndSettle();
      expect(votes, ['p2', 'p3', 'p3']);
      expect(done, 1);
    },
  );

  testWidgets(
    'confirmed failed ban retries without resending accepted rating',
    (tester) async {
      var votes = 0;
      var bans = 0;
      var done = 0;
      await tester.pumpWidget(
        _ratingFixture(
          onRate: (_, _) async {
            votes++;
            return true;
          },
          onBan: (_) async => ++bans > 1,
          onDone: () => done++,
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(MatchRatingOverlay.starButtonKey(1)));
      await tester.tap(find.byKey(MatchRatingOverlay.submitButtonKey));
      await tester.pump();
      await tester.pump(const Duration(seconds: 1));
      final l10n = AppLocalizations.of(
        tester.element(find.byType(AlertDialog)),
      )!;
      await tester.tap(find.text(l10n.matchRatingBanConfirm));
      await tester.pumpAndSettle();
      expect(done, 0);
      expect(votes, 1);
      expect(bans, 1);

      await tester.tap(find.byKey(MatchRatingOverlay.submitButtonKey));
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsNothing);
      expect(votes, 1);
      expect(bans, 2);
      expect(done, 1);
    },
  );

  testWidgets('pending submission blocks skip and independent ban actions', (
    tester,
  ) async {
    final vote = Completer<bool>();
    var bans = 0;
    var skipped = 0;
    await tester.pumpWidget(
      _ratingFixture(
        onRate: (_, _) => vote.future,
        onBan: (_) async {
          bans++;
          return true;
        },
        onSkipAll: () => skipped++,
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(MatchRatingOverlay.starButtonKey(4)));
    await tester.tap(find.byKey(MatchRatingOverlay.submitButtonKey));
    await tester.pump();
    await tester.tap(find.byKey(MatchRatingOverlay.skipAllButtonKey));
    await tester.tap(find.byKey(MatchRatingOverlay.skipTeammateKey('p2')));
    await tester.tap(find.byKey(MatchRatingOverlay.banButtonKey('p2')));
    expect(bans, 0);
    expect(skipped, 0);
    vote.complete(true);
    await tester.pumpAndSettle();
  });

  testWidgets('rating overlay shows star buttons for each teammate', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(
          body: MatchRatingOverlay(
            match: _completedMatch(),
            raterProfileId: 'p1',
            teammates: const [
              RatedTeammate(profileId: 'p2', displayName: 'Teammate'),
            ],
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(MatchRatingOverlay.starButtonKey(5)), findsOneWidget);
    expect(find.byKey(MatchRatingOverlay.submitButtonKey), findsOneWidget);
    expect(find.text('Teammate'), findsOneWidget);
  });

  testWidgets('submit sends rateMatch with selected stars', (tester) async {
    int? submittedStars;
    String? ratedProfileId;

    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(
          body: MatchRatingOverlay(
            match: _completedMatch(),
            raterProfileId: 'p1',
            teammates: const [
              RatedTeammate(profileId: 'p2', displayName: 'Teammate'),
            ],
            onRate: (profileId, stars) async {
              ratedProfileId = profileId;
              submittedStars = stars;
              return true;
            },
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(MatchRatingOverlay.starButtonKey(4)));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(MatchRatingOverlay.submitButtonKey));
    await tester.pumpAndSettle();

    expect(ratedProfileId, 'p2');
    expect(submittedStars, 4);
  });

  testWidgets('ban button triggers banFromMM for teammate', (tester) async {
    String? bannedProfileId;

    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(
          body: MatchRatingOverlay(
            match: _completedMatch(),
            raterProfileId: 'p1',
            teammates: const [
              RatedTeammate(profileId: 'p2', displayName: 'Teammate'),
            ],
            onBan: (profileId) async {
              bannedProfileId = profileId;
              return true;
            },
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(MatchRatingOverlay.banButtonKey('p2')));
    await tester.pumpAndSettle();

    expect(bannedProfileId, 'p2');
  });

  testWidgets('recovery card shows timeout copy and return action', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(
          body: MatchmakingRecoveryCard(
            reason: SearchRecoveryReason.timeout,
            onAction: () {},
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(MatchmakingRecoveryCard.timeoutStateKey), findsOneWidget);
    expect(find.text('Return to queue'), findsOneWidget);
  });

  testWidgets('recovery card shows decline copy and continue action', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(
          body: MatchmakingRecoveryCard(
            reason: SearchRecoveryReason.declined,
            onAction: () {},
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(
      find.byKey(MatchmakingRecoveryCard.declinedStateKey),
      findsOneWidget,
    );
    expect(find.text('Continue searching'), findsOneWidget);
  });
}

Widget _ratingFixture({
  List<RatedTeammate> teammates = const [
    RatedTeammate(profileId: 'p2', displayName: 'Teammate'),
  ],
  Future<bool> Function(String, int)? onRate,
  Future<bool> Function(String)? onBan,
  VoidCallback? onDone,
  VoidCallback? onSkipAll,
}) => MaterialApp(
  theme: voiceTestTheme(),
  localizationsDelegates: AppLocalizations.localizationsDelegates,
  supportedLocales: AppLocalizations.supportedLocales,
  home: Scaffold(
    body: MatchRatingOverlay(
      match: _completedMatch(),
      raterProfileId: 'p1',
      teammates: teammates,
      onRate: onRate,
      onBan: onBan,
      onDone: onDone,
      onSkipAll: onSkipAll,
    ),
  ),
);
