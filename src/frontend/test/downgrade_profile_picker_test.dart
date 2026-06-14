import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/profile/profile_downgrade_picker_screen.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets('downgrade profile picker keeps two profiles active', (tester) async {
    final client = MockClient((req) async {
      if (req.url.path == '/api/v1/users/me/profiles' && req.method == 'GET') {
        return http.Response(
          '{"profile_list":{"profiles":['
          '{"id":"p1","display_name":"Main","is_primary":true},'
          '{"id":"p2","display_name":"Alt A","is_primary":false},'
          '{"id":"p3","display_name":"Alt B","is_primary":false}'
          ']}}',
          200,
        );
      }
      if (req.url.path == '/api/v1/subscription/downgrade/profiles' && req.method == 'POST') {
        return http.Response('{"kept_profile_ids":["p1","p2"]}', 200);
      }
      return http.Response('Not Found', 404);
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: voiceAppTestOverrides(client: client),
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: ProfileDowngradePickerScreen()),
        ),
      ),
    );

    await tester.pumpAndSettle();

    expect(find.byKey(const Key('downgrade_profile_picker')), findsOneWidget);
    expect(find.text('Choose 2 profiles to keep'), findsOneWidget);
    expect(find.text('Main'), findsOneWidget);
    expect(find.text('Alt A'), findsOneWidget);
    expect(find.text('Alt B'), findsOneWidget);
  });
}
