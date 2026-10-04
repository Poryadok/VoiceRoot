import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/stories/lfp_request_decision_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets('LFP request decision hides upstream failure details', (
    tester,
  ) async {
    const upstreamDetail = 'private matchmaking diagnostic';
    final client = MockClient(
      (_) async => http.Response(
        jsonEncode({'error_code': 'internal_error', 'message': upstreamDetail}),
        500,
      ),
    );
    await tester.pumpWidget(
      ProviderScope(
        overrides: voiceAppTestOverrides(client: client),
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: LfpRequestDecisionSheet(
              storyId: 'story-1',
              responderProfileId: 'profile-1',
              responseType: 'JOIN',
            ),
          ),
        ),
      ),
    );

    await tester.tap(find.byKey(LfpRequestDecisionSheet.acceptKey));
    await tester.pumpAndSettle();

    expect(find.byType(SnackBar), findsOneWidget);
    expect(find.text('Could not complete this action.'), findsOneWidget);
    expect(find.text(upstreamDetail), findsNothing);
  });
}
