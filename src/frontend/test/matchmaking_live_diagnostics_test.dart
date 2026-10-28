import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

import 'support/matchmaking_live_diagnostics.dart';

void main() {
  test('cohort diagnostic distinguishes a foreign second participant', () {
    final result = matchmakingCohortDiagnostic(
      profileA: 'private-a',
      profileB: 'private-b',
      selectedMatchId: 'private-match',
      sessionA: {'matchId': 'private-match'},
      sessionB: {'match_id': 'other-match'},
      match: {
        'profile_ids': ['private-a', 'private-leftover'],
      },
    );
    expect(result, contains('identities_distinct=true'));
    expect(
      result,
      contains('session_a_selected=true session_b_selected=false'),
    );
    expect(result, contains('member_count=2 member_a=true member_b=false'));
    expect(result, isNot(contains('private')));
  });

  test('missing identities cannot look like valid distinct participants', () {
    final result = matchmakingCohortDiagnostic(
      profileA: '',
      profileB: '',
      selectedMatchId: '',
      sessionA: {},
      sessionB: {},
      match: {
        'profileIds': [''],
      },
    );
    expect(result, contains('identities_distinct=false'));
    expect(
      result,
      contains('session_a_selected=false session_b_selected=false'),
    );
    expect(result, contains('member_a=false member_b=false'));
  });

  test(
    'accept diagnostic labels the actor and only allowlists the known denial',
    () {
      expect(
        matchmakingAcceptFailureReason(
          http.Response(
            '{"error_code":"permission_denied","message":"not a match participant"}',
            403,
          ),
          actorA: false,
        ),
        'compose_mm_accept actor=b http=403 cause=not_match_participant',
      );
      for (final body in [
        'private-token',
        '{"message":"private-token"}',
        'x' * 1025,
      ]) {
        expect(
          matchmakingAcceptFailureReason(
            http.Response(body, 500),
            actorA: true,
          ),
          'compose_mm_accept actor=a http=500 cause=other',
        );
      }
    },
  );
}
