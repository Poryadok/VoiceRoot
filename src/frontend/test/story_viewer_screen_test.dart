import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/stories_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/matchmaking_providers.dart';
import 'package:voice_frontend/backend/matchmaking_client.dart';
import 'package:voice_frontend/state/stories_providers.dart';
import 'package:voice_frontend/ui/stories/story_game_tag_chip.dart';
import 'package:voice_frontend/ui/stories/story_viewer_screen.dart';
import 'package:voice_frontend/ui/stories/story_viewers_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  const textStoryAccent = StoryData(
    id: 'story-text-accent',
    authorProfileId: 'author-1',
    type: 'text',
    textContent: 'Styled story',
    textStyleJson: '{"background":"accent"}',
    visibility: 'everyone',
    viewCount: 0,
  );

  const storyWithGameTag = StoryData(
    id: 'story-photo-tag',
    authorProfileId: 'author-2',
    type: 'text',
    textContent: 'Playing tonight',
    gameTag: 'game-dota',
    visibility: 'everyone',
    viewCount: 0,
  );

  const story = StoryData(
    id: 'story-1',
    authorProfileId: 'author-1',
    type: 'text',
    textContent: 'Hello story',
    visibility: 'everyone',
    viewCount: 3,
  );

  Widget wrap(Widget child, {String activeProfileId = 'prof-test'}) {
    return ProviderScope(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => throw UnimplementedError()),
        ),
        authControllerProvider.overrideWith((ref) {
          final controller = AuthController(
            authClient: ref.watch(voiceAuthClientProvider),
            storage: ref.watch(authSessionStorageProvider),
            guestCredentialsStorage: ref.watch(guestCredentialsStorageProvider),
          );
          controller.state = AuthState(
            session: AuthSession(
              accessToken: 'test-access',
              refreshToken: 'test-refresh',
              accountId: 'acc-test',
              activeProfileId: activeProfileId,
              expiresInSeconds: 900,
            ),
          );
          return controller;
        }),
        storyDetailProvider('story-1').overrideWith((ref) async => story),
      ],
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: child,
      ),
    );
  }

  testWidgets('StoryViewerScreen shows private reply action', (tester) async {
    await tester.pumpWidget(
      wrap(const StoryViewerScreen(storyIds: ['story-1'])),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('story_viewer_reply')), findsOneWidget);
  });

  testWidgets('StoryViewerScreen shows view count for author', (tester) async {
    await tester.pumpWidget(
      wrap(
        const StoryViewerScreen(storyIds: ['story-1']),
        activeProfileId: 'author-1',
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('story_viewer_view_count')), findsOneWidget);
    expect(find.textContaining('3'), findsOneWidget);
  });

  testWidgets('StoryViewerScreen applies text background from textStyleJson',
      (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => throw UnimplementedError()),
          ),
          authControllerProvider.overrideWith((ref) {
            final controller = AuthController(
              authClient: ref.watch(voiceAuthClientProvider),
              storage: ref.watch(authSessionStorageProvider),
              guestCredentialsStorage:
                  ref.watch(guestCredentialsStorageProvider),
            );
            controller.state = AuthState(
              session: AuthSession(
                accessToken: 'test-access',
                refreshToken: 'test-refresh',
                accountId: 'acc-test',
                activeProfileId: 'prof-test',
                expiresInSeconds: 900,
              ),
            );
            return controller;
          }),
          storyDetailProvider('story-text-accent')
              .overrideWith((ref) async => textStoryAccent),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const StoryViewerScreen(storyIds: ['story-text-accent']),
        ),
      ),
    );
    await tester.pumpAndSettle();

    final colored = tester.widget<ColoredBox>(
      find.descendant(
        of: find.byType(StoryViewerScreen),
        matching: find.byType(ColoredBox),
      ).first,
    );
    final theme = voiceTestTheme();
    expect(colored.color, theme.colorScheme.primary);
  });

  testWidgets('StoryViewerScreen shows game tag chip on non-LFP story',
      (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => throw UnimplementedError()),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          storyDetailProvider('story-photo-tag')
              .overrideWith((ref) async => storyWithGameTag),
          gameCatalogProvider.overrideWith(
            (ref) async => GameListData(
              games: [
                CatalogGame(
                  id: 'game-dota',
                  name: 'Dota 2',
                  status: 'active',
                  config: const GameConfig(regions: ['eu'], modes: []),
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
          home: const StoryViewerScreen(storyIds: ['story-photo-tag']),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(StoryGameTagChip.chipKey), findsOneWidget);
    expect(find.text('Dota 2'), findsOneWidget);
  });

  testWidgets('StoryViewerScreen view count opens viewers sheet for author',
      (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => throw UnimplementedError()),
          ),
          authControllerProvider.overrideWith((ref) {
            final controller = AuthController(
              authClient: ref.watch(voiceAuthClientProvider),
              storage: ref.watch(authSessionStorageProvider),
              guestCredentialsStorage:
                  ref.watch(guestCredentialsStorageProvider),
            );
            controller.state = AuthState(
              session: AuthSession(
                accessToken: 'test-access',
                refreshToken: 'test-refresh',
                accountId: 'acc-test',
                activeProfileId: 'author-1',
                expiresInSeconds: 900,
              ),
            );
            return controller;
          }),
          storyDetailProvider('story-1').overrideWith((ref) async => story),
          storyViewersProvider('story-1').overrideWith(
            (ref) async => const ['viewer-1'],
          ),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const StoryViewerScreen(storyIds: ['story-1']),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('story_viewer_view_count')));
    await tester.pumpAndSettle();

    expect(find.byKey(StoryViewersSheet.sheetKey), findsOneWidget);
  });
}
