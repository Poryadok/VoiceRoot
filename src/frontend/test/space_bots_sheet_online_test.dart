import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/bots_client.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/bot_providers.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/space/space_bots_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  const auth = 'Bearer access-token';

  testWidgets('SpaceBotsSheet shows online indicator for installed bots', (
    tester,
  ) async {
    const installed = [
      InstalledBotInfo(
        bot: VoiceBotSummary(
          id: 'bot-online',
          name: 'OnlineBot',
          description: 'live',
          scopesJson: '["TEXT_CHAT_SEND_MESSAGES"]',
        ),
        installationId: 'inst-1',
        allowedChatIds: ['chat-1'],
        online: true,
      ),
      InstalledBotInfo(
        bot: VoiceBotSummary(
          id: 'bot-offline',
          name: 'OfflineBot',
          description: 'down',
          scopesJson: '["TEXT_CHAT_SEND_MESSAGES"]',
        ),
        installationId: 'inst-2',
        allowedChatIds: ['chat-1'],
        online: false,
      ),
    ];

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
          authorizationHeaderProvider.overrideWithValue(auth),
          installedBotsProvider('space-1').overrideWith(
            (ref) async => installed,
          ),
          spacePermissionProvider((
            spaceId: 'space-1',
            permission: 'SPACE_MANAGE_BOTS',
            chatId: null,
            voiceRoomId: null,
          )).overrideWith((ref) async => true),
          discoverableBotsProvider.overrideWith((ref) async => const []),
          spaceTreeProvider('space-1').overrideWith(
            (ref) async => const SpaceTreeData(
              categories: [],
              nodes: [],
              voiceRooms: [],
            ),
          ),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: SpaceBotsSheet(spaceId: 'space-1'),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(
      find.byKey(const Key('space_bots_online_indicator_bot-online')),
      findsOneWidget,
      reason: 'installed bot with online:true must show online indicator',
    );
    expect(
      find.byKey(const Key('space_bots_offline_indicator_bot-offline')),
      findsOneWidget,
      reason: 'installed bot with online:false must show offline indicator',
    );
  });
}
