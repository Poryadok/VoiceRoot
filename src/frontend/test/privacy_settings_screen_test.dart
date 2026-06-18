import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/ui/settings/privacy_settings_screen.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

Map<String, dynamic> _audience({
  bool friends = false,
  bool friendsOfFriends = false,
  bool spaceMembers = false,
  bool includeGuests = false,
}) {
  return {
    'friends': friends,
    'friends_of_friends': friendsOfFriends,
    'space_members': spaceMembers,
    'include_guests': includeGuests,
  };
}

void main() {
  testWidgets('privacy preset selector applies gaming defaults', (tester) async {
    final client = MockClient((req) async {
      if (req.url.path == '/api/v1/users/me/privacy' && req.method == 'GET') {
        return http.Response(
          jsonEncode({
            'privacy_settings': {
              'profile_id': 'prof-test',
              'preset': 'personal',
              'show_online': _audience(friends: true),
              'show_game_status': _audience(friends: true),
              'show_mm_rating': _audience(friends: true, friendsOfFriends: true),
              'show_phone': _audience(),
              'show_stories': _audience(friends: true, friendsOfFriends: true),
              'allow_dm': _audience(friends: true, friendsOfFriends: true),
              'allow_friend_requests': _audience(
                friends: true,
                friendsOfFriends: true,
                spaceMembers: true,
                includeGuests: true,
              ),
              'allow_guest_dm': false,
              'allow_phone_search': _audience(friends: true),
              'allow_calls': _audience(friends: true),
              'allow_chat_space_invites': _audience(friends: true),
              'allow_files': _audience(friends: true, friendsOfFriends: true),
              'allow_voice_messages': _audience(friends: true),
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
          locale: const Locale('ru'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const PrivacySettingsScreen(),
        ),
      ),
    );

    await tester.pumpAndSettle();

    expect(find.text('Игровой'), findsOneWidget);
    await tester.tap(find.text('Игровой'));
    await tester.pumpAndSettle();

    expect(find.text('Все'), findsWidgets);
  });
}
