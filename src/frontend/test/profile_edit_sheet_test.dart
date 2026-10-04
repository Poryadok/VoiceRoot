import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/profile/profile_edit_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets('keeps display name validation local and avoids a request', (
    tester,
  ) async {
    var patchRequests = 0;
    final client = MockClient((req) async {
      if (req.url.path == '/api/v1/users/me' && req.method == 'PATCH') {
        patchRequests++;
      }
      return http.Response('Not Found', 404);
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          discoverHintStorageProvider.overrideWithValue(
            testDiscoverHintStorage,
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(client),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: ProfileEditSheet(
              profile: VoiceProfile(
                id: 'prof-test',
                accountId: 'acc-test',
                username: 'voiceuser',
                discriminator: '4242',
                displayName: 'Voice User',
                bio: 'Old bio',
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(ProfileEditSheet.displayNameFieldKey),
      ' ',
    );
    await tester.tap(find.byKey(ProfileEditSheet.saveButtonKey));
    await tester.pumpAndSettle();

    final l10n = AppLocalizations.of(
      tester.element(find.byKey(ProfileEditSheet.displayNameFieldKey)),
    )!;
    expect(find.text(l10n.profileErrorDisplayNameRequired), findsOneWidget);
    expect(patchRequests, 0);
  });

  testWidgets('maps profile save diagnostics to neutral localized copy', (
    tester,
  ) async {
    const upstreamDiagnostic = 'private profile save diagnostic';
    final client = MockClient((req) async {
      if (req.url.path == '/api/v1/users/me' && req.method == 'PATCH') {
        return http.Response(jsonEncode({'message': upstreamDiagnostic}), 500);
      }
      return http.Response('Not Found', 404);
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          discoverHintStorageProvider.overrideWithValue(
            testDiscoverHintStorage,
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(client),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: ProfileEditSheet(
              profile: VoiceProfile(
                id: 'prof-test',
                accountId: 'acc-test',
                username: 'voiceuser',
                discriminator: '4242',
                displayName: 'Voice User',
                bio: 'Old bio',
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(ProfileEditSheet.displayNameFieldKey),
      'Updated Name',
    );
    await tester.tap(find.byKey(ProfileEditSheet.saveButtonKey));
    await tester.pumpAndSettle();

    expect(find.text('Could not complete this action.'), findsOneWidget);
    expect(find.textContaining(upstreamDiagnostic), findsNothing);
  });

  testWidgets('maps avatar presign diagnostics to neutral localized copy', (
    tester,
  ) async {
    const upstreamDiagnostic = 'private avatar presign diagnostic';
    var presignRequests = 0;
    final client = MockClient((req) async {
      if (req.url.path == '/api/v1/users/me/avatar/presigned-upload' &&
          req.method == 'POST') {
        presignRequests++;
        return http.Response(jsonEncode({'message': upstreamDiagnostic}), 500);
      }
      return http.Response('Not Found', 404);
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          discoverHintStorageProvider.overrideWithValue(
            testDiscoverHintStorage,
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(client),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(
            body: ProfileEditSheet(
              profile: const VoiceProfile(
                id: 'prof-test',
                accountId: 'acc-test',
                username: 'voiceuser',
                discriminator: '4242',
                displayName: 'Voice User',
                bio: 'Old bio',
              ),
              avatarPicker: () async => ProfileAvatarFile(
                bytes: base64Decode(
                  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR4nGNgYAAAAAMAASsJTYQAAAAASUVORK5CYII=',
                ),
                contentType: 'image/png',
                name: 'avatar.png',
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(ProfileEditSheet.avatarButtonKey));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(ProfileEditSheet.saveButtonKey));
    await tester.pumpAndSettle();

    expect(presignRequests, 1);
    expect(find.text('Could not complete this action.'), findsOneWidget);
    expect(find.textContaining(upstreamDiagnostic), findsNothing);
  });

  testWidgets('keeps missing authorization sentinel unchanged', (tester) async {
    var requests = 0;
    final client = MockClient((_) async {
      requests++;
      return http.Response('Not Found', 404);
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          discoverHintStorageProvider.overrideWithValue(
            testDiscoverHintStorage,
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          authorizationHeaderProvider.overrideWithValue(null),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(client),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(
            body: ProfileEditSheet(
              profile: const VoiceProfile(
                id: 'prof-test',
                accountId: 'acc-test',
                username: 'voiceuser',
                discriminator: '4242',
                displayName: 'Voice User',
                bio: 'Old bio',
              ),
              avatarPicker: () async => ProfileAvatarFile(
                bytes: base64Decode(
                  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR4nGNgYAAAAAMAASsJTYQAAAAASUVORK5CYII=',
                ),
                contentType: 'image/png',
                name: 'avatar.png',
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(ProfileEditSheet.avatarButtonKey));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(ProfileEditSheet.saveButtonKey));
    await tester.pumpAndSettle();

    final l10n = AppLocalizations.of(
      tester.element(find.byKey(ProfileEditSheet.saveButtonKey)),
    )!;
    expect(
      find.text(l10n.profileEditSaveError('not_authenticated')),
      findsOneWidget,
    );
    expect(requests, 0);
  });

  testWidgets('saves display name and bio through profile actions', (
    tester,
  ) async {
    Map<String, dynamic>? patchBody;
    final client = MockClient((req) async {
      if (req.url.path == '/api/v1/users/me' && req.method == 'PATCH') {
        patchBody = jsonDecode(req.body) as Map<String, dynamic>;
        return http.Response(
          jsonEncode({
            'profile': {
              'id': 'prof-test',
              'account_id': 'acc-test',
              'username': 'voiceuser',
              'discriminator': '4242',
              'display_name': patchBody!['display_name'],
              'bio': patchBody!['bio'],
              'locale': 'en',
              'theme': 'dark',
              'is_primary': true,
              'verification_type': 'none',
            },
          }),
          200,
        );
      }
      return http.Response('Not Found', 404);
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          discoverHintStorageProvider.overrideWithValue(
            testDiscoverHintStorage,
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(client),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: ProfileEditSheet(
              profile: VoiceProfile(
                id: 'prof-test',
                accountId: 'acc-test',
                username: 'voiceuser',
                discriminator: '4242',
                displayName: 'Voice User',
                bio: 'Old bio',
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(ProfileEditSheet.displayNameFieldKey),
      'Voice Renamed',
    );
    await tester.enterText(
      find.byKey(ProfileEditSheet.bioFieldKey),
      'Looking for a duo',
    );
    await tester.tap(find.byKey(ProfileEditSheet.saveButtonKey));
    await tester.pumpAndSettle();

    expect(patchBody, {
      'display_name': 'Voice Renamed',
      'bio': 'Looking for a duo',
    });
  });
}
