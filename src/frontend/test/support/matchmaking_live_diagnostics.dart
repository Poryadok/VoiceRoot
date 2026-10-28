import 'dart:convert';

import 'package:http/http.dart' as http;

/// Only relationships and finite categories leave this diagnostic boundary.
String matchmakingCohortDiagnostic({
  required String profileA,
  required String profileB,
  required String selectedMatchId,
  required Map<String, dynamic> sessionA,
  required Map<String, dynamic> sessionB,
  required Map<String, dynamic> match,
}) {
  final rawMembers = match['profileIds'] ?? match['profile_ids'];
  final members = rawMembers is List ? rawMembers : const [];
  final count = members.length > 9 ? 'many' : '${members.length}';
  bool selected(Map<String, dynamic> session) =>
      selectedMatchId.isNotEmpty &&
      (session['matchId'] ?? session['match_id']) == selectedMatchId;
  return 'compose_mm_cohort '
      'identity_a_present=${profileA.isNotEmpty} '
      'identity_b_present=${profileB.isNotEmpty} '
      'identities_distinct=${profileA.isNotEmpty && profileB.isNotEmpty && profileA != profileB} '
      'session_a_selected=${selected(sessionA)} '
      'session_b_selected=${selected(sessionB)} '
      'member_count=$count '
      'member_a=${profileA.isNotEmpty && members.contains(profileA)} '
      'member_b=${profileB.isNotEmpty && members.contains(profileB)}';
}

String matchmakingAcceptFailureReason(
  http.Response response, {
  required bool actorA,
}) {
  var cause = 'other';
  if (response.body.length <= 1024) {
    try {
      final body = jsonDecode(response.body);
      if (body is Map<String, dynamic> &&
          body['error_code'] == 'permission_denied' &&
          body['message'] == 'not a match participant') {
        cause = 'not_match_participant';
      }
    } on FormatException {
      // Never include arbitrary upstream payloads in CI logs.
    }
  }
  return 'compose_mm_accept actor=${actorA ? 'a' : 'b'} '
      'http=${response.statusCode} cause=$cause';
}
