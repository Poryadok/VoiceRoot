import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/state/subscription_providers.dart';
import 'package:voice_frontend/ui/core/voice_skeleton.dart';
import 'package:voice_frontend/ui/profile/profile_downgrade_picker_screen.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets(
    'downgrade profile picker shows list skeleton while profiles load',
    (tester) async {
      final pending = Completer<List<VoiceProfile>>();
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            ...voiceAppTestOverrides(
              client: MockClient((_) async => http.Response('', 500)),
            ),
            myProfilesProvider.overrideWith((ref) => pending.future),
          ],
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const Scaffold(body: ProfileDowngradePickerScreen()),
          ),
        ),
      );
      await tester.pump();

      expect(find.byType(VoiceListSkeleton), findsOneWidget);

      pending.complete(const []);
    },
  );

  testWidgets(
    'downgrade profile picker shows localized error on profile load failure',
    (tester) async {
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            ...voiceAppTestOverrides(
              client: MockClient((_) async => http.Response('', 500)),
            ),
            myProfilesProvider.overrideWith(
              (ref) async => throw Exception('private detail'),
            ),
          ],
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

      final l10n = AppLocalizations.of(
        tester.element(find.byType(ProfileDowngradePickerScreen)),
      )!;
      expect(find.text(l10n.subscriptionProfilesLoadError), findsOneWidget);
      expect(find.text('private detail'), findsNothing);
      expect(find.byType(CircularProgressIndicator), findsNothing);
    },
  );

  testWidgets('downgrade profile picker keeps two profiles active', (
    tester,
  ) async {
    final client = MockClient((req) async {
      if (req.url.path == '/api/v1/users/profiles' && req.method == 'GET') {
        return http.Response(
          '{"profile_list":{"profiles":['
          '{"id":"p1","display_name":"Main","is_primary":true},'
          '{"id":"p2","display_name":"Alt A","is_primary":false},'
          '{"id":"p3","display_name":"Alt B","is_primary":false}'
          ']}}',
          200,
        );
      }
      if (req.url.path == '/api/v1/subscription/downgrade/profiles' &&
          req.method == 'POST') {
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
