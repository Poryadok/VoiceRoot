import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/onboarding_controller.dart';
import 'package:voice_frontend/state/presence_providers.dart';
import 'package:voice_frontend/state/social_providers.dart';
import 'package:voice_frontend/ui/shell/chat_list_body.dart';

import 'support/auth_test_overrides.dart';
import 'support/fake_voice_api_clients.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets(
    'DM row prefers its supplied peer title when profile is unavailable',
    (tester) async {
      const chatId = 'existing-dm-1';
      final chats = FakeVoiceChatsClient(
        pages: [
          ChatListData(
            items: [
              ChatListItem(
                chat: const VoiceChat(
                  id: chatId,
                  type: 'CHAT_TYPE_DM',
                  creatorProfileId: 'peer-profile',
                ),
                dmPeerProfileId: 'peer-profile',
                dmPeerDisplayName: 'Actual Peer Nickname',
              ),
            ],
          ),
        ],
      );
      final container = ProviderContainer(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => throw UnimplementedError()),
          ),
          onboardingControllerProvider.overrideWith(
            TestCompletedOnboardingController.new,
          ),
          voiceChatsClientProvider.overrideWith((ref) => chats),
          profileProvider('peer-profile').overrideWith((ref) async => null),
          presenceProvider('peer-profile').overrideWith((ref) => null),
        ],
      );
      addTearDown(container.dispose);

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const Scaffold(body: ChatListBody(showHeader: false)),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Actual Peer Nickname'), findsOneWidget);
      expect(find.text('peer-profile'), findsNothing);
    },
  );

  testWidgets(
    'DM row uses a generic title when peer title and profile are unavailable',
    (tester) async {
      const chatId = 'existing-dm-private';
      final container = ProviderContainer(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => throw UnimplementedError()),
          ),
          onboardingControllerProvider.overrideWith(
            TestCompletedOnboardingController.new,
          ),
          voiceChatsClientProvider.overrideWith(
            (ref) => FakeVoiceChatsClient(
              pages: [
                ChatListData(
                  items: [
                    ChatListItem(
                      chat: const VoiceChat(
                        id: chatId,
                        type: 'CHAT_TYPE_DM',
                        creatorProfileId: 'private-peer-profile',
                      ),
                      dmPeerProfileId: 'private-peer-profile',
                    ),
                  ],
                ),
              ],
            ),
          ),
          profileProvider(
            'private-peer-profile',
          ).overrideWith((ref) async => null),
          presenceProvider('private-peer-profile').overrideWith((ref) => null),
        ],
      );
      addTearDown(container.dispose);

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const Scaffold(body: ChatListBody(showHeader: false)),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Chat existing'), findsOneWidget);
      expect(find.text('private-peer-profile'), findsNothing);
    },
  );
}
