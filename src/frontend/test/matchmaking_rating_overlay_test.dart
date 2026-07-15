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
  testWidgets('rating overlay shows star buttons for each teammate', (tester) async {
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

  testWidgets('recovery card shows timeout copy and return action', (tester) async {
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

  testWidgets('recovery card shows decline copy and continue action', (tester) async {
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

    expect(find.byKey(MatchmakingRecoveryCard.declinedStateKey), findsOneWidget);
    expect(find.text('Continue searching'), findsOneWidget);
  });
}
