import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/bots_client.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/l10n/app_localizations_en.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/bot_providers.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/bots/bot_install_page.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  const auth = 'Bearer access-token';
  const slug = 'statsbot';

  const sampleBot = VoiceBotSummary(
    id: 'bot-stats',
    name: 'StatsBot',
    slug: slug,
    description: 'Player stats for CS2, Valorant, and Dota 2',
    scopesJson:
        '["TEXT_CHAT_SEND_MESSAGES","SPACE_VIEW_MEMBER_LIST","MEMBER_ASSIGN_ROLES"]',
  );

  testWidgets('BotInstallPage shows description, scopes, and commands section',
      (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
          authorizationHeaderProvider.overrideWithValue(auth),
          botBySlugProvider(slug).overrideWith((ref) async => sampleBot),
          mySpacesProvider.overrideWith(
            (ref) async => const SpaceListData(spaces: []),
          ),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const BotInstallPage(slug: slug),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(BotInstallPage.pageKey), findsOneWidget);
    expect(find.text('StatsBot'), findsOneWidget);
    expect(find.text('@statsbot'), findsOneWidget);

    final l10n = AppLocalizationsEn();
    expect(find.text(l10n.botInstallDescriptionHeading), findsOneWidget);
    expect(find.text(sampleBot.description), findsOneWidget);

    expect(find.text(l10n.botInstallScopesHeading), findsOneWidget);
    expect(
      find.text('• Send messages in allowed text chats'),
      findsOneWidget,
    );
    expect(find.text('• View space member list'), findsOneWidget);
    expect(find.text('• Assign roles below the bot'), findsOneWidget);

    expect(find.text(l10n.botInstallCommandsHeading), findsOneWidget);
    expect(find.text(l10n.botInstallCommandsEmpty), findsOneWidget);

    expect(find.text(l10n.botInstallWhitelistHeading), findsOneWidget);
    expect(find.byKey(const Key('bot_install_confirm')), findsOneWidget);
  });

  testWidgets(
    'BotInstallPage requires privileged ack for SPACE_MANAGE_ROLES (BOT-C)',
    (tester) async {
      const privilegedBot = VoiceBotSummary(
        id: 'bot-roles',
        name: 'RolesBot',
        slug: 'rolesbot',
        description: 'Manages roles',
        scopesJson: '["TEXT_CHAT_SEND_MESSAGES","SPACE_MANAGE_ROLES"]',
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            ...voiceThemeTestOverrides(),
            profileAccentStorageProvider.overrideWithValue(
              testProfileAccentStorage,
            ),
            authorizationHeaderProvider.overrideWithValue(auth),
            botBySlugProvider('rolesbot').overrideWith(
              (ref) async => privilegedBot,
            ),
            mySpacesProvider.overrideWith(
              (ref) async => const SpaceListData(
                spaces: [
                  VoiceSpace(
                    id: 'space-1',
                    name: 'Test Space',
                    visibility: 'private',
                    ownerProfileId: 'owner-1',
                  ),
                ],
              ),
            ),
            spaceTreeProvider('space-1').overrideWith(
              (ref) async => const SpaceTreeData(
                categories: [],
                voiceRooms: [],
                nodes: [
                  SpaceTreeNodeData(
                    id: 'node-1',
                    spaceId: 'space-1',
                    kind: 'text_chat',
                    sortOrder: 0,
                    displayName: 'general',
                    linkedChatId: 'chat-1',
                    chatType: 'CHAT_TYPE_CHANNEL',
                  ),
                ],
              ),
            ),
          ],
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const BotInstallPage(slug: 'rolesbot'),
          ),
        ),
      );
      await tester.pumpAndSettle();

      final l10n = AppLocalizationsEn();
      expect(
        find.text('• Create and manage roles below the bot (privileged)'),
        findsOneWidget,
      );

      await tester.tap(find.byKey(const Key('bot_install_space_picker')));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Test Space').last);
      await tester.pumpAndSettle();

      expect(find.text(l10n.spaceBotsPrivilegedAck), findsOneWidget);

      await tester.tap(find.byKey(const Key('bot_install_chat_chat-1')));
      await tester.pumpAndSettle();

      final installButton = tester.widget<FilledButton>(
        find.byKey(const Key('bot_install_confirm')),
      );
      expect(
        installButton.onPressed,
        isNull,
        reason:
            'SPACE_MANAGE_ROLES install must stay disabled until acknowledged (BOT-C)',
      );
    },
  );
}
