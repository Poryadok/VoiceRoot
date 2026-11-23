import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/subscription_providers.dart';
import 'package:voice_frontend/ui/profile/profile_edit_sheet.dart';
import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

final _editedProfile = VoiceProfile(
  id: 'prof-test',
  accountId: 'acc-test',
  username: 'voiceuser',
  discriminator: '4242',
  displayName: 'Voice User',
  bio: 'Old bio',
);

const _otherProfile = VoiceProfile(
  id: 'other-profile',
  accountId: 'acc-test',
  username: 'other',
  discriminator: '0002',
  displayName: 'Other',
);

Future<void> _pump(
  WidgetTester tester,
  http.Client client, {
  required List<VoiceProfile> profiles,
}) async {
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        ...voiceAppTestOverrides(client: client),
        myProfilesProvider.overrideWith((_) async => profiles),
      ],
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(body: ProfileEditSheet(profile: _editedProfile)),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

/// Moves the active profile away from the edited profile, the same way the
/// profile-switch guard is exercised in profile_edit_v3_test.dart.
void _switchActiveProfileAway(WidgetTester tester) {
  final container = ProviderScope.containerOf(
    tester.element(find.byType(ProfileEditSheet)),
  );
  container.read(authControllerProvider.notifier).state = const AuthState(
    session: AuthSession(
      accessToken: 'other-access',
      refreshToken: 'other-refresh',
      accountId: 'acc-test',
      activeProfileId: 'other-profile',
      expiresInSeconds: 3600,
    ),
  );
}

void main() {
  testWidgets(
    'delete of a non-active secondary profile confirms and sends REST',
    (tester) async {
      var deletes = 0;
      await _pump(
        tester,
        MockClient((req) async {
          if (req.method == 'DELETE' &&
              req.url.path == '/api/v1/users/profiles/prof-test') {
            deletes++;
            return http.Response('', 200);
          }
          if (req.url.path.endsWith('/privacy') && req.method == 'GET') {
            return http.Response(
              jsonEncode({
                'privacy_settings': {
                  'profile_id': 'prof-test',
                  'preset': 'personal',
                },
              }),
              200,
            );
          }
          return http.Response('{}', 404);
        }),
        profiles: [_editedProfile, _otherProfile],
      );
      _switchActiveProfileAway(tester);
      await tester.pumpAndSettle();

      await tester.ensureVisible(find.byKey(ProfileEditSheet.deleteButtonKey));
      await tester.tap(find.byKey(ProfileEditSheet.deleteButtonKey));
      await tester.pumpAndSettle();
      await tester.ensureVisible(find.widgetWithText(FilledButton, 'Delete'));
      await tester.tap(find.widgetWithText(FilledButton, 'Delete'));
      await tester.pumpAndSettle();

      expect(deletes, 1);
      expect(find.text('Could not complete this action.'), findsNothing);
    },
  );

  testWidgets('delete dialog cancel keeps the profile and sends no request', (
    tester,
  ) async {
    var deletes = 0;
    await _pump(
      tester,
      MockClient((req) async {
        if (req.method == 'DELETE') {
          deletes++;
          return http.Response('', 200);
        }
        if (req.url.path.endsWith('/privacy') && req.method == 'GET') {
          return http.Response(
            jsonEncode({
              'privacy_settings': {
                'profile_id': 'prof-test',
                'preset': 'personal',
              },
            }),
            200,
          );
        }
        return http.Response('{}', 404);
      }),
      profiles: [_editedProfile, _otherProfile],
    );
    _switchActiveProfileAway(tester);
    await tester.pumpAndSettle();

    await tester.ensureVisible(find.byKey(ProfileEditSheet.deleteButtonKey));
    await tester.tap(find.byKey(ProfileEditSheet.deleteButtonKey));
    await tester.pumpAndSettle();
    await tester.ensureVisible(find.widgetWithText(TextButton, 'Cancel'));
    await tester.tap(find.widgetWithText(TextButton, 'Cancel'));
    await tester.pumpAndSettle();

    expect(deletes, 0);
    expect(find.byType(ProfileEditSheet), findsOneWidget);
  });

  testWidgets('active profile cannot be deleted from its own edit sheet', (
    tester,
  ) async {
    var deletes = 0;
    await _pump(
      tester,
      MockClient((req) async {
        if (req.method == 'DELETE') deletes++;
        if (req.url.path.endsWith('/privacy') && req.method == 'GET') {
          return http.Response(
            jsonEncode({
              'privacy_settings': {
                'profile_id': 'prof-test',
                'preset': 'personal',
              },
            }),
            200,
          );
        }
        return http.Response('{}', 404);
      }),
      profiles: [_editedProfile, _otherProfile],
    );
    await tester.ensureVisible(find.byKey(ProfileEditSheet.deleteButtonKey));
    await tester.tap(find.byKey(ProfileEditSheet.deleteButtonKey));
    await tester.pumpAndSettle();

    expect(deletes, 0);
    expect(
      find.text('Switch to another profile before removing this one.'),
      findsOneWidget,
    );
  });

  testWidgets('primary profile cannot be deleted from its own edit sheet', (
    tester,
  ) async {
    var deletes = 0;
    await _pump(
      tester,
      MockClient((req) async {
        if (req.method == 'DELETE') deletes++;
        if (req.url.path.endsWith('/privacy') && req.method == 'GET') {
          return http.Response(
            jsonEncode({
              'privacy_settings': {
                'profile_id': 'prof-test',
                'preset': 'personal',
              },
            }),
            200,
          );
        }
        return http.Response('{}', 404);
      }),
      profiles: [
        VoiceProfile(
          id: 'prof-test',
          accountId: 'acc-test',
          username: 'voiceuser',
          discriminator: '4242',
          displayName: 'Voice User',
          isPrimary: true,
        ),
        _otherProfile,
      ],
    );
    await tester.ensureVisible(find.byKey(ProfileEditSheet.deleteButtonKey));
    await tester.tap(find.byKey(ProfileEditSheet.deleteButtonKey));
    await tester.pumpAndSettle();

    expect(deletes, 0);
    expect(find.text('The primary profile cannot be removed.'), findsOneWidget);
  });

  testWidgets('single-profile account shows no delete affordance', (
    tester,
  ) async {
    await _pump(
      tester,
      MockClient((_) async => http.Response('{}', 404)),
      profiles: [_editedProfile],
    );
    expect(find.byKey(ProfileEditSheet.deleteButtonKey), findsNothing);
  });

  testWidgets(
    'delete row renders danger-coloured at narrow width without overflow',
    (tester) async {
      addTearDown(() => tester.view.resetPhysicalSize());
      addTearDown(() => tester.view.resetDevicePixelRatio());
      tester.view.devicePixelRatio = 1;
      for (final width in [390.0, 1280.0]) {
        tester.view.physicalSize = Size(width, 844);
        await _pump(
          tester,
          MockClient((req) async {
            if (req.url.path.endsWith('/privacy') && req.method == 'GET') {
              return http.Response(
                jsonEncode({
                  'privacy_settings': {
                    'profile_id': 'prof-test',
                    'preset': 'personal',
                  },
                }),
                200,
              );
            }
            return http.Response('{}', 404);
          }),
          profiles: [_editedProfile, _otherProfile],
        );
        expect(tester.takeException(), isNull);
        expect(find.byKey(ProfileEditSheet.deleteButtonKey), findsOneWidget);
      }
    },
  );
}
