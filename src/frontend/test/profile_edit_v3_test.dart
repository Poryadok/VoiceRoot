import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/profile/profile_edit_sheet.dart';
import 'package:voice_frontend/theme/voice_theme.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';
import 'support/test_voice_token_catalog.dart';

ThemeData? _captureTheme;

const _profile = VoiceProfile(
  id: 'prof-test',
  accountId: 'acc-test',
  username: 'voiceuser',
  discriminator: '4242',
  displayName: 'Voice User',
  bio: 'Old bio',
  accentColor: '#7EC8E3',
);

Future<void> _pump(
  WidgetTester tester,
  http.Client client, {
  ProfileAvatarPicker? avatarPicker,
}) async {
  await tester.pumpWidget(
    ProviderScope(
      overrides: voiceAppTestOverrides(client: client),
      child: MaterialApp(
        theme: _captureTheme ?? voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: RepaintBoundary(
          key: const Key('profile_edit_capture'),
          child: Scaffold(
            body: ProfileEditSheet(
              profile: _profile,
              avatarPicker: avatarPicker,
            ),
          ),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  setUpAll(() async {
    if (!const bool.fromEnvironment('VOICE_PROFILE_EDIT_SCREENSHOTS')) return;
    await (FontLoader(VoiceTheme.fontFamily)
          ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
          ..addFont(rootBundle.load('assets/fonts/NotoSans-Medium.ttf'))
          ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'))
          ..addFont(rootBundle.load('assets/fonts/NotoSans-Bold.ttf')))
        .load();
    var flutterRoot = File(Platform.resolvedExecutable).parent;
    for (var i = 0; i < 5; i++) {
      flutterRoot = flutterRoot.parent;
    }
    final icons = await File(
      [
        flutterRoot.path,
        'bin',
        'cache',
        'artifacts',
        'material_fonts',
        'MaterialIcons-Regular.otf',
      ].join(Platform.pathSeparator),
    ).readAsBytes();
    await (FontLoader(
      'MaterialIcons',
    )..addFont(Future.value(ByteData.sublistView(icons)))).load();
    _captureTheme = await VoiceTheme.build(
      catalog: testVoiceTokenCatalog,
      mode: VoiceThemeMode.light,
      profileAccent: testVoiceTokenCatalog.profileAccentAt(0),
    );
  });
  testWidgets(
    'saved avatar preview survives privacy failure without repeating upload',
    (tester) async {
      final bytes = base64Decode(
        'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR4nGNgYAAAAAMAASsJTYQAAAAASUVORK5CYII=',
      );
      var presigns = 0;
      var uploads = 0;
      var profilePatches = 0;
      var privacyPatches = 0;
      await _pump(
        tester,
        MockClient((req) async {
          if (req.url.path.endsWith('/presigned-upload')) {
            presigns++;
            return http.Response(
              jsonEncode({
                'upload_url': 'http://upload.test/avatar',
                'public_url': 'http://cdn.test/avatar',
                'required_headers': {},
              }),
              200,
            );
          }
          if (req.method == 'PUT') {
            uploads++;
            return http.Response('', 200);
          }
          if (req.url.path.endsWith('/privacy')) {
            if (req.method == 'GET') {
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
            privacyPatches++;
            if (privacyPatches == 1) return http.Response('{}', 503);
            return http.Response(
              jsonEncode({
                'privacy_settings': {
                  'profile_id': 'prof-test',
                  'preset': 'work',
                },
              }),
              200,
            );
          }
          if (req.method == 'PATCH') {
            profilePatches++;
            return http.Response(
              jsonEncode({
                'profile': {
                  'id': 'prof-test',
                  'account_id': 'acc-test',
                  'display_name': 'Voice User',
                  'avatar_url': 'http://cdn.test/avatar',
                },
              }),
              200,
            );
          }
          return http.Response('{}', 404);
        }),
        avatarPicker: () async => ProfileAvatarFile(
          bytes: bytes,
          contentType: 'image/png',
          name: 'saved.png',
        ),
      );
      await tester.tap(find.byKey(ProfileEditSheet.avatarButtonKey));
      await tester.pumpAndSettle();
      await tester.ensureVisible(find.text('Work'));
      await tester.tap(find.text('Work'));
      await tester.pump();
      await tester.ensureVisible(find.byKey(ProfileEditSheet.saveButtonKey));
      await tester.tap(find.byKey(ProfileEditSheet.saveButtonKey));
      await tester.pumpAndSettle();
      expect(find.text('Could not complete this action.'), findsOneWidget);
      final preview = tester
          .widget<CircleAvatar>(find.byType(CircleAvatar))
          .backgroundImage;
      expect(preview, isA<MemoryImage>());
      expect((preview! as MemoryImage).bytes, bytes);
      await tester.tap(find.byKey(ProfileEditSheet.saveButtonKey));
      await tester.pumpAndSettle();
      expect(presigns, 1);
      expect(uploads, 1);
      expect(profilePatches, 1);
      expect(privacyPatches, 2);
    },
  );

  for (final viaOtherPreset in [false, true]) {
    testWidgets(
      'explicit Work defaults reset custom audiences even with same label (via other: $viaOtherPreset)',
      (tester) async {
        Map<String, dynamic>? saved;
        var profilePatches = 0;
        await _pump(
          tester,
          MockClient((req) async {
            if (req.url.path.endsWith('/privacy')) {
              if (req.method == 'GET') {
                return http.Response(
                  jsonEncode({
                    'privacy_settings': {
                      'profile_id': 'prof-test',
                      'preset': 'work',
                      'show_online': {'friends': true},
                      'show_game_status': {'friends': true},
                    },
                  }),
                  200,
                );
              }
              saved =
                  (jsonDecode(req.body) as Map<String, dynamic>)['settings']
                      as Map<String, dynamic>;
              return http.Response(
                jsonEncode({'privacy_settings': saved}),
                200,
              );
            }
            if (req.method == 'PATCH') profilePatches++;
            return http.Response('{}', 404);
          }),
        );
        expect(
          tester
              .widget<FilledButton>(find.byKey(ProfileEditSheet.saveButtonKey))
              .onPressed,
          isNull,
        );
        if (viaOtherPreset) {
          await tester.ensureVisible(find.text('Personal'));
          await tester.tap(find.text('Personal'));
          await tester.pump();
        }
        await tester.ensureVisible(find.text('Work'));
        await tester.tap(find.text('Work'));
        await tester.pump();
        await tester.ensureVisible(find.byKey(ProfileEditSheet.saveButtonKey));
        expect(
          tester
              .widget<FilledButton>(find.byKey(ProfileEditSheet.saveButtonKey))
              .onPressed,
          isNotNull,
        );
        await tester.tap(find.byKey(ProfileEditSheet.saveButtonKey));
        await tester.pumpAndSettle();
        expect(saved?['preset'], 'work');
        expect(saved?['profile_id'], 'prof-test');
        expect(saved?['show_game_status'], {
          'friends': false,
          'friends_of_friends': false,
          'space_members': false,
          'include_guests': false,
        });
        expect((saved?['show_online'] as Map)['space_members'], isTrue);
        expect((saved?['show_online'] as Map)['friends'], isFalse);
        expect(profilePatches, 0);
      },
    );
  }

  testWidgets(
    'profile switch during avatar presign cancels follow-up mutations',
    (tester) async {
      final presign = Completer<http.Response>();
      var uploads = 0;
      var patches = 0;
      await _pump(
        tester,
        MockClient((req) async {
          if (req.url.path.endsWith('/presigned-upload')) return presign.future;
          if (req.method == 'PUT') uploads++;
          if (req.method == 'PATCH') patches++;
          return http.Response('{}', 500);
        }),
        avatarPicker: () async => ProfileAvatarFile(
          bytes: base64Decode(
            'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR4nGNgYAAAAAMAASsJTYQAAAAASUVORK5CYII=',
          ),
          contentType: 'image/png',
          name: 'avatar.png',
        ),
      );
      await tester.tap(find.byKey(ProfileEditSheet.avatarButtonKey));
      await tester.pumpAndSettle();
      await tester.ensureVisible(find.byKey(ProfileEditSheet.saveButtonKey));
      await tester.tap(find.byKey(ProfileEditSheet.saveButtonKey));
      await tester.pump();
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
      presign.complete(
        http.Response(
          jsonEncode({
            'upload_url': 'http://upload.test/avatar',
            'public_url': 'http://cdn.test/avatar',
            'required_headers': {},
          }),
          200,
        ),
      );
      await tester.pumpAndSettle();
      expect(uploads, 0);
      expect(patches, 0);
      expect(find.byType(ProfileEditSheet), findsOneWidget);
      expect(
        tester
            .widget<FilledButton>(find.byKey(ProfileEditSheet.saveButtonKey))
            .onPressed,
        isNotNull,
      );
    },
  );

  testWidgets('privacy retry does not repeat a successful identity save', (
    tester,
  ) async {
    var profilePatches = 0;
    var privacyPatches = 0;
    await _pump(
      tester,
      MockClient((req) async {
        if (req.url.path.endsWith('/privacy')) {
          if (req.method == 'GET') {
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
          privacyPatches++;
          if (privacyPatches == 1) return http.Response('{}', 503);
          return http.Response(
            jsonEncode({
              'privacy_settings': {'profile_id': 'prof-test', 'preset': 'work'},
            }),
            200,
          );
        }
        if (req.method == 'PATCH') {
          profilePatches++;
          return http.Response(
            jsonEncode({
              'profile': {
                'id': 'prof-test',
                'account_id': 'acc-test',
                'display_name': 'Updated',
              },
            }),
            200,
          );
        }
        return http.Response('{}', 404);
      }),
    );
    await tester.enterText(
      find.byKey(ProfileEditSheet.displayNameFieldKey),
      'Updated',
    );
    await tester.pump();
    await tester.ensureVisible(find.text('Work'));
    await tester.tap(find.text('Work'));
    await tester.ensureVisible(find.byKey(ProfileEditSheet.saveButtonKey));
    await tester.tap(find.byKey(ProfileEditSheet.saveButtonKey));
    await tester.pumpAndSettle();
    expect(find.text('Could not complete this action.'), findsOneWidget);
    await tester.tap(find.byKey(ProfileEditSheet.saveButtonKey));
    await tester.pumpAndSettle();
    expect(profilePatches, 1);
    expect(privacyPatches, 2);
  });

  testWidgets('save requires a draft change and handle remains visible', (
    tester,
  ) async {
    await _pump(tester, MockClient((_) async => http.Response('{}', 500)));
    expect(
      tester
          .widget<FilledButton>(find.byKey(ProfileEditSheet.saveButtonKey))
          .onPressed,
      isNull,
    );
    expect(find.text('@voiceuser#4242'), findsOneWidget);
    await tester.enterText(
      find.byKey(ProfileEditSheet.displayNameFieldKey),
      'Updated',
    );
    await tester.pump();
    expect(
      tester
          .widget<FilledButton>(find.byKey(ProfileEditSheet.saveButtonKey))
          .onPressed,
      isNotNull,
    );
  });

  testWidgets(
    'accent uses profile API and privacy changes use preset defaults',
    (tester) async {
      Map<String, dynamic>? profileBody;
      Map<String, dynamic>? privacyBody;
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
          if (req.method == 'PATCH') {
            final body = jsonDecode(req.body) as Map<String, dynamic>;
            if (req.url.path.endsWith('/privacy')) {
              privacyBody = body;
              return http.Response(
                jsonEncode({'privacy_settings': body['settings']}),
                200,
              );
            }
            profileBody = body;
            return http.Response(
              jsonEncode({
                'profile': {
                  'id': 'prof-test',
                  'account_id': 'acc-test',
                  'display_name': 'Voice User',
                  'accent_color': body['accent_color'],
                },
              }),
              200,
            );
          }
          return http.Response('{}', 404);
        }),
      );
      await tester.ensureVisible(
        find.byKey(const Key('profile_edit_accent_1')),
      );
      await tester.tap(find.byKey(const Key('profile_edit_accent_1')));
      await tester.pump();
      await tester.ensureVisible(find.text('Work'));
      await tester.tap(find.text('Work'));
      await tester.pump();
      await tester.ensureVisible(find.byKey(ProfileEditSheet.saveButtonKey));
      await tester.tap(find.byKey(ProfileEditSheet.saveButtonKey));
      await tester.pumpAndSettle();
      expect(profileBody?['accent_color'], '#9ED9A6');
      final settings = privacyBody?['settings'] as Map<String, dynamic>?;
      expect(settings?['preset'], 'work');
      expect(settings?['profile_id'], 'prof-test');
      expect(settings?['show_game_status'], {
        'friends': false,
        'friends_of_friends': false,
        'space_members': false,
        'include_guests': false,
      });
      expect((settings?['show_online'] as Map)['space_members'], isTrue);
    },
  );

  testWidgets('wide and narrow layouts wrap controls without overflow', (
    tester,
  ) async {
    addTearDown(() => tester.view.resetPhysicalSize());
    addTearDown(() => tester.view.resetDevicePixelRatio());
    tester.view.devicePixelRatio = 1;
    for (final width in [390.0, 1280.0]) {
      tester.view.physicalSize = Size(width, 844);
      await _pump(
        tester,
        MockClient(
          (_) async => http.Response(
            jsonEncode({
              'privacy_settings': {
                'profile_id': 'prof-test',
                'preset': 'personal',
              },
            }),
            200,
          ),
        ),
      );
      if (const bool.fromEnvironment('VOICE_PROFILE_EDIT_SCREENSHOTS')) {
        await expectLater(
          find.byKey(const Key('profile_edit_capture')),
          matchesGoldenFile(
            '../../../tmp/swarm/anton137-profile-edit/profile-edit-${width.toInt()}-top.png',
          ),
        );
      }
      await tester.ensureVisible(find.byKey(ProfileEditSheet.saveButtonKey));
      await tester.pumpAndSettle();
      if (const bool.fromEnvironment('VOICE_PROFILE_EDIT_SCREENSHOTS')) {
        await expectLater(
          find.byKey(const Key('profile_edit_capture')),
          matchesGoldenFile(
            '../../../tmp/swarm/anton137-profile-edit/profile-edit-${width.toInt()}-bottom.png',
          ),
        );
      }
      expect(tester.takeException(), isNull);
      expect(find.byTooltip('Close'), findsOneWidget);
    }
  });
}
