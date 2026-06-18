import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/stories_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/stories_providers.dart';
import 'package:voice_frontend/ui/stories/story_viewer_screen.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
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
}
