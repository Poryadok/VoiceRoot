import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/matchmaking_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/matchmaking/match_found_overlay.dart';

import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

MatchData _pendingMatch() {
  return const MatchData(
    id: 'match-1',
    gameId: 'g-val',
    mode: 'Duo',
    region: 'eu',
    status: 'pending_accept',
    profileIds: ['p1', 'p2'],
    gameName: 'Valorant',
  );
}

void main() {
  testWidgets('match found overlay shows accept and decline actions', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: voiceThemeTestOverrides(),
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(
            body: Stack(
              children: [
                MatchFoundOverlay(match: _pendingMatch()),
              ],
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(MatchFoundOverlay.acceptButtonKey), findsOneWidget);
    expect(find.byKey(MatchFoundOverlay.declineButtonKey), findsOneWidget);
    expect(find.textContaining('Valorant'), findsOneWidget);
  });

  testWidgets('accept button triggers respondToMatch with accept true', (tester) async {
    bool? accepted;
    var showOverlay = true;
    await tester.pumpWidget(
      ProviderScope(
        overrides: voiceThemeTestOverrides(),
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: StatefulBuilder(
            builder: (context, setState) {
              return Scaffold(
                body: Stack(
                  children: [
                    if (showOverlay)
                      MatchFoundOverlay(
                        match: _pendingMatch(),
                        onRespond: (accept) async {
                          accepted = accept;
                          setState(() => showOverlay = false);
                          return const RespondToMatchData(
                            match: MatchData(
                              id: 'match-1',
                              gameId: 'g-val',
                              mode: 'Duo',
                              region: 'eu',
                              status: 'active',
                              profileIds: ['p1', 'p2'],
                            ),
                            searchSession: SearchSessionData(
                              id: 'sess-1',
                              profileId: 'p1',
                              gameId: 'g-val',
                              mode: 'Duo',
                              criteriaJson: '{}',
                              status: 'matched',
                            ),
                          );
                        },
                      ),
                  ],
                ),
              );
            },
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(MatchFoundOverlay.acceptButtonKey));
    await tester.pumpAndSettle();

    expect(accepted, isTrue);
    expect(find.byType(MatchFoundOverlay), findsNothing);
  });
}
