import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/matchmaking_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/matchmaking/match_squad_screen.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  Widget testApp({required Widget home}) {
    return ProviderScope(
      overrides: [
        ...voiceThemeTestOverrides(),
        profileAccentStorageProvider.overrideWithValue(
          testProfileAccentStorage,
        ),
        authSessionStorageProvider.overrideWithValue(
          InMemoryAuthSessionStorage(),
        ),
        authControllerProvider.overrideWith(authenticatedAuthController),
        gatewayConfigProvider.overrideWithValue(const GatewayConfig(baseUrl: '')),
      ],
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: home,
      ),
    );
  }

  testWidgets('MatchSquadScreen shows voice section and chat when voiceRoomId set',
      (tester) async {
    await tester.pumpWidget(
      testApp(
        home: MatchSquadScreen(
          match: MatchData(
            id: 'match-1',
            gameId: 'game-1',
            mode: 'ranked',
            region: 'eu',
            status: 'active',
            profileIds: const ['profile-1'],
            chatId: 'chat-1',
            voiceRoomId: 'room-1',
          ),
        ),
      ),
    );
    await tester.pump();

    expect(find.byKey(MatchSquadScreen.voiceSectionKey), findsOneWidget);
    expect(find.byKey(MatchSquadScreen.leaveButtonKey), findsOneWidget);
  });
}
