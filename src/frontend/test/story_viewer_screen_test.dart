import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/stories_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/matchmaking_providers.dart';
import 'package:voice_frontend/backend/matchmaking_client.dart';
import 'package:voice_frontend/state/stories_providers.dart';
import 'package:voice_frontend/ui/stories/story_game_tag_chip.dart';
import 'package:voice_frontend/ui/stories/story_viewer_screen.dart';
import 'package:voice_frontend/ui/stories/lfp_story_card.dart';
import 'package:voice_frontend/ui/stories/story_viewers_sheet.dart';
import 'package:voice_frontend/ui/report/report_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/gateway_test_client.dart';
import 'support/voice_test_theme.dart';

void main() {
  setUp(() => SharedPreferences.setMockInitialValues({}));

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

  Widget wrap(
    Widget child, {
    String activeProfileId = 'prof-test',
    http.Client? client,
    StoryData data = story,
    Future<StoryData?> Function()? load,
  }) {
    return ProviderScope(
      overrides: [
        ...voiceAppTestOverrides(
          client: client ?? MockClient((_) async => throw UnimplementedError()),
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
        storyDetailProvider(
          'story-1',
        ).overrideWith((ref) async => load == null ? data : load()),
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

  testWidgets('empty story route renders recovery without clamping error', (
    tester,
  ) async {
    await tester.pumpWidget(wrap(const StoryViewerScreen(storyIds: [])));
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
    expect(find.text('No stories to show'), findsOneWidget);
  });

  testWidgets(
    'LFP viewer retains reply/react and sends real discovery responses',
    (tester) async {
      final responses = <String>[];
      final mock = MockClient((req) async {
        if (req.url.path.endsWith('/lfp-response')) {
          responses.add(jsonDecode(req.body)['response_type'] as String);
        }
        return http.Response('', 204);
      });
      await tester.pumpWidget(
        wrap(
          const StoryViewerScreen(storyIds: ['story-1']),
          client: mock,
          data: const StoryData(
            id: 'story-1',
            authorProfileId: 'author-1',
            type: 'text',
            textContent: 'Need a fifth player',
            isLookingForParty: true,
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.byKey(StoryViewerScreen.replyButtonKey), findsOneWidget);
      expect(find.byKey(StoryViewerScreen.reactButtonKey), findsOneWidget);
      await tester.tap(find.byKey(LfpStoryCard.joinKey));
      await tester.pumpAndSettle();
      expect(find.text('Join request sent'), findsOneWidget);
      await tester.tap(find.byKey(LfpStoryCard.inviteKey));
      await tester.pumpAndSettle();
      expect(responses, ['JOIN', 'INVITE']);
    },
  );

  testWidgets('expired LFP disables join/invite but keeps private write', (
    tester,
  ) async {
    await tester.pumpWidget(
      wrap(
        const StoryViewerScreen(storyIds: ['story-1']),
        client: MockClient((_) async => http.Response('', 204)),
        data: StoryData(
          id: 'story-1',
          authorProfileId: 'author-1',
          type: 'text',
          isLookingForParty: true,
          expiresAt: DateTime.utc(2020),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(
      tester.widget<FilledButton>(find.byKey(LfpStoryCard.joinKey)).onPressed,
      isNull,
    );
    expect(
      tester.widget<FilledButton>(find.byKey(LfpStoryCard.inviteKey)).onPressed,
      isNull,
    );
    expect(
      tester
          .widget<OutlinedButton>(find.byKey(LfpStoryCard.writeKey))
          .onPressed,
      isNotNull,
    );
    expect(find.text('This announcement is no longer active'), findsOneWidget);
  });

  testWidgets('loading then retry recovers from a sanitized detail error', (
    tester,
  ) async {
    final pending = Completer<StoryData?>();
    var attempts = 0;
    await tester.pumpWidget(
      wrap(
        const StoryViewerScreen(storyIds: ['story-1']),
        client: MockClient((_) async => http.Response('', 204)),
        load: () {
          attempts++;
          return attempts == 1 ? pending.future : Future.value(story);
        },
      ),
    );
    await tester.pump();
    expect(find.text('Hello story'), findsNothing);
    pending.completeError(const StoriesLoadException(500));
    await tester.pumpAndSettle();
    expect(find.text('Could not load story'), findsOneWidget);
    final l10n = AppLocalizations.of(
      tester.element(find.byType(StoryViewerScreen)),
    )!;
    await tester.tap(find.text(l10n.commonRetry));
    await tester.pumpAndSettle();
    expect(attempts, 2);
    expect(find.text('Hello story'), findsOneWidget);
  });

  testWidgets('segments and keyboard change story and mark previous viewed', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    try {
      final viewed = <String>[];
      final mock = MockClient((req) async {
        if (req.url.path.endsWith('/views')) viewed.add(req.url.path);
        return http.Response('', 204);
      });
      await tester.pumpWidget(
        wrap(
          const StoryViewerScreen(storyIds: ['story-1', 'story-1']),
          client: mock,
        ),
      );
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('story_viewer_segment_0')), findsOneWidget);
      expect(find.byKey(const Key('story_viewer_segment_1')), findsOneWidget);
      await tester.sendKeyEvent(LogicalKeyboardKey.arrowRight);
      await tester.pumpAndSettle();
      expect(find.bySemanticsLabel('Story 2 of 2'), findsOneWidget);
      await tester.sendKeyEvent(LogicalKeyboardKey.arrowLeft);
      await tester.pumpAndSettle();
      expect(find.bySemanticsLabel('Story 1 of 2'), findsOneWidget);
      expect(viewed, ['/api/v1/stories/story-1/views']);
    } finally {
      semantics.dispose();
    }
  });

  testWidgets(
    'successful owner delete removes current story and reaches the next',
    (tester) async {
      var deletes = 0;
      final response = Completer<http.Response>();
      final mock = MockClient((req) async {
        if (req.method == 'DELETE') {
          deletes++;
          return response.future;
        }
        if (req.url.path.endsWith('/story-2')) {
          return http.Response(
            '{"story":{"id":"story-2","author_profile_id":"author-1","type":"text","text_content":"Second story"}}',
            200,
          );
        }
        return http.Response('', 204);
      });
      await tester.pumpWidget(
        wrap(
          const StoryViewerScreen(storyIds: ['story-1', 'story-2']),
          activeProfileId: 'author-1',
          client: mock,
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('story_viewer_delete')));
      await tester.pumpAndSettle();
      await tester.tap(find.widgetWithText(FilledButton, 'Delete'));
      await tester.pump();
      await tester.sendKeyEvent(LogicalKeyboardKey.arrowRight);
      await tester.tap(find.byKey(const Key('story_viewer_segment_1')));
      await tester.pump();
      expect(find.text('Hello story'), findsOneWidget);
      expect(find.text('Second story'), findsNothing);
      response.complete(http.Response('', 204));
      await tester.pumpAndSettle();
      expect(deletes, 1);
      expect(find.text('Second story'), findsOneWidget);
      expect(find.text('Hello story'), findsNothing);
      expect(find.byKey(const Key('story_viewer_segment_1')), findsNothing);
    },
  );

  for (final status in [204, 500]) {
    testWidgets(
      'late delete $status cannot mutate a replacement profile viewer',
      (tester) async {
        final response = Completer<http.Response>();
        final mock = MockClient(
          (req) async =>
              req.method == 'DELETE' ? response.future : http.Response('', 204),
        );
        await tester.pumpWidget(
          wrap(
            const StoryViewerScreen(storyIds: ['story-1']),
            activeProfileId: 'author-1',
            client: mock,
          ),
        );
        await tester.pumpAndSettle();
        final container = ProviderScope.containerOf(
          tester.element(find.byType(StoryViewerScreen)),
        );
        await tester.tap(find.byKey(const Key('story_viewer_delete')));
        await tester.pumpAndSettle();
        await tester.tap(find.widgetWithText(FilledButton, 'Delete'));
        await tester.pump();
        container.read(authControllerProvider.notifier).state = const AuthState(
          session: AuthSession(
            accessToken: 'replacement-token',
            refreshToken: 'replacement-refresh',
            accountId: 'acc-test',
            activeProfileId: 'profile-b',
            expiresInSeconds: 900,
          ),
        );
        await tester.pumpAndSettle();
        response.complete(http.Response('', status));
        await tester.pumpAndSettle();
        expect(find.text('Hello story'), findsOneWidget);
        expect(find.byType(SnackBar), findsNothing);
        expect(
          container.read(authControllerProvider).activeProfileId,
          'profile-b',
        );
      },
    );
  }

  testWidgets(
    'delete transaction is invalidated by a switch attempt returning to the same profile',
    (tester) async {
      final response = Completer<http.Response>();
      await tester.pumpWidget(
        wrap(
          const StoryViewerScreen(storyIds: ['story-1']),
          activeProfileId: 'author-1',
          client: MockClient(
            (req) async => req.method == 'DELETE'
                ? response.future
                : http.Response('', 204),
          ),
        ),
      );
      await tester.pumpAndSettle();
      final container = ProviderScope.containerOf(
        tester.element(find.byType(StoryViewerScreen)),
      );
      await tester.tap(find.byKey(const Key('story_viewer_delete')));
      await tester.pumpAndSettle();
      await tester.tap(find.widgetWithText(FilledButton, 'Delete'));
      await tester.pump();
      container.read(profileSwitchInProgressProvider.notifier).state = true;
      await tester.pump();
      container.read(profileSwitchInProgressProvider.notifier).state = false;
      await tester.pump();
      response.complete(http.Response('', 204));
      await tester.pumpAndSettle();
      expect(find.text('Hello story'), findsOneWidget);
    },
  );

  testWidgets('LFP expiry updates mounted controls and guards a stale tap', (
    tester,
  ) async {
    var now = DateTime.utc(2099);
    var joins = 0;
    var invites = 0;
    Widget app(DateTime expiresAt) => MaterialApp(
      theme: voiceTestTheme(),
      locale: const Locale('en'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: Scaffold(
        body: LfpStoryCard(
          story: StoryData(
            id: 'expiring',
            authorProfileId: 'author',
            type: 'text',
            expiresAt: expiresAt,
          ),
          now: () => now,
          onJoin: () => joins++,
          onInvite: () => invites++,
          onWrite: () {},
        ),
      ),
    );
    await tester.pumpWidget(app(now.add(const Duration(seconds: 2))));
    await tester.pumpAndSettle();
    expect(
      tester.widget<FilledButton>(find.byKey(LfpStoryCard.joinKey)).onPressed,
      isNotNull,
    );
    now = now.add(const Duration(seconds: 2));
    await tester.pump(const Duration(seconds: 2));
    expect(
      tester.widget<FilledButton>(find.byKey(LfpStoryCard.joinKey)).onPressed,
      isNull,
    );
    expect(
      tester.widget<FilledButton>(find.byKey(LfpStoryCard.inviteKey)).onPressed,
      isNull,
    );
    expect(
      tester
          .widget<OutlinedButton>(find.byKey(LfpStoryCard.writeKey))
          .onPressed,
      isNotNull,
    );
    await tester.pumpWidget(app(now.add(const Duration(seconds: 2))));
    await tester.pump();
    now = now.add(const Duration(seconds: 2));
    await tester.tap(find.byKey(LfpStoryCard.joinKey));
    await tester.pump();
    expect(joins, 0);
    expect(invites, 0);
    await tester.pumpWidget(const SizedBox.shrink());
  });

  testWidgets('report button opens the shared Story report form', (
    tester,
  ) async {
    await tester.pumpWidget(
      wrap(
        const StoryViewerScreen(storyIds: ['story-1']),
        client: MockClient((_) async => http.Response('', 204)),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(StoryViewerScreen.moreButtonKey));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(StoryViewerScreen.reportButtonKey));
    await tester.pumpAndSettle();
    expect(find.byKey(ReportSheet.sheetKey), findsOneWidget);
    expect(
      tester.widget<ReportSheet>(find.byType(ReportSheet)).target,
      isA<ReportStoryTarget>(),
    );
  });

  testWidgets(
    'owner deletion requires confirmation and preserves story on failure',
    (tester) async {
      var deletes = 0;
      final mock = MockClient((req) async {
        if (req.method == 'DELETE') {
          deletes++;
          return http.Response('', 500);
        }
        return http.Response('', 204);
      });
      await tester.pumpWidget(
        wrap(
          const StoryViewerScreen(storyIds: ['story-1']),
          activeProfileId: 'author-1',
          client: mock,
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('story_viewer_delete')));
      await tester.pumpAndSettle();
      await tester.tap(find.widgetWithText(TextButton, 'Cancel'));
      await tester.pumpAndSettle();
      expect(deletes, 0);
      await tester.tap(find.byKey(const Key('story_viewer_delete')));
      await tester.pumpAndSettle();
      await tester.tap(find.widgetWithText(FilledButton, 'Delete'));
      await tester.pumpAndSettle();
      expect(deletes, 1);
      expect(find.byKey(StoryViewerScreen.screenKey), findsOneWidget);
      expect(find.text('Could not complete this action.'), findsOneWidget);
    },
  );

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

  testWidgets('StoryViewerScreen applies text background from textStyleJson', (
    tester,
  ) async {
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
              guestCredentialsStorage: ref.watch(
                guestCredentialsStorageProvider,
              ),
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
          storyDetailProvider(
            'story-text-accent',
          ).overrideWith((ref) async => textStoryAccent),
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
      find
          .descendant(
            of: find.byType(StoryViewerScreen),
            matching: find.byType(ColoredBox),
          )
          .first,
    );
    final theme = voiceTestTheme();
    expect(colored.color, theme.colorScheme.primary);
  });

  testWidgets('StoryViewerScreen shows game tag chip on non-LFP story', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => throw UnimplementedError()),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          storyDetailProvider(
            'story-photo-tag',
          ).overrideWith((ref) async => storyWithGameTag),
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

  testWidgets('StoryViewerScreen view count opens viewers sheet for author', (
    tester,
  ) async {
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
              guestCredentialsStorage: ref.watch(
                guestCredentialsStorageProvider,
              ),
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
          storyViewersProvider(
            'story-1',
          ).overrideWith((ref) async => const ['viewer-1']),
          storyReactionsProvider(
            'story-1',
          ).overrideWith((ref) async => const []),
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
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));

    expect(find.byKey(StoryViewersSheet.sheetKey), findsOneWidget);
  });

  testWidgets('StoryViewerScreen react picker sends emoji to backend', (
    tester,
  ) async {
    String? reactedEmoji;
    final mock = MockClient((req) async {
      if (req.method == 'POST' && req.url.path.endsWith('/views')) {
        return http.Response('', 204);
      }
      if (req.method == 'POST' && req.url.path.endsWith('/reactions')) {
        final body = jsonDecode(req.body) as Map<String, dynamic>;
        reactedEmoji = body['emoji'] as String?;
        return http.Response('', 204);
      }
      return http.Response('not found', 404);
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceAppTestOverrides(client: mock),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://localhost:9999'),
          ),
          voiceStoriesClientProvider.overrideWith(
            (ref) => VoiceStoriesClient(
              gateway: gatewayHttpForTest(
                mock,
                config: const GatewayConfig(baseUrl: 'http://localhost:9999'),
              ),
            ),
          ),
          storyDetailProvider('story-1').overrideWith((ref) async => story),
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

    await tester.tap(find.byKey(const Key('story_viewer_react')));
    await tester.pumpAndSettle();

    expect(find.byKey(StoryViewerScreen.reactPickerKey), findsOneWidget);

    await tester.tap(find.byKey(const Key('story_viewer_react_emoji_❤️')));
    await tester.pumpAndSettle();

    expect(reactedEmoji, '❤️');
    expect(find.text('Reaction sent'), findsOneWidget);
  });

  testWidgets('StoryViewerScreen hides upstream reaction error', (
    tester,
  ) async {
    final mock = MockClient((req) async {
      if (req.url.path.endsWith('/views')) return http.Response('', 204);
      if (req.url.path.endsWith('/reactions')) {
        return http.Response(
          '{"error":"internal","message":"private reaction diagnostic"}',
          500,
          headers: const {'content-type': 'application/json'},
        );
      }
      return http.Response('', 404);
    });
    await tester.pumpWidget(
      wrap(const StoryViewerScreen(storyIds: ['story-1']), client: mock),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(StoryViewerScreen.reactButtonKey));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('story_viewer_react_emoji_❤️')));
    await tester.pumpAndSettle();

    expect(find.text('private reaction diagnostic'), findsNothing);
    expect(find.text('Could not complete this action.'), findsOneWidget);
  });

  testWidgets('StoryViewerScreen hides upstream reply error', (tester) async {
    http.Request? replyRequest;
    final mock = MockClient((req) async {
      if (req.url.path.endsWith('/views')) return http.Response('', 204);
      if (req.url.path.endsWith('/reply')) {
        replyRequest = req;
        return http.Response(
          '{"error":"internal","message":"private reply diagnostic"}',
          503,
          headers: const {'content-type': 'application/json'},
        );
      }
      return http.Response('', 404);
    });
    await tester.pumpWidget(
      wrap(const StoryViewerScreen(storyIds: ['story-1']), client: mock),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(StoryViewerScreen.replyButtonKey));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField).last, '  Please respond  ');
    await tester.tap(find.widgetWithText(FilledButton, 'Reply'));
    await tester.pumpAndSettle();

    expect(replyRequest?.method, 'POST');
    expect(replyRequest?.url.path, '/api/v1/stories/story-1/reply');
    expect(jsonDecode(replyRequest!.body)['text'], 'Please respond');
    expect(find.text('private reply diagnostic'), findsNothing);
    expect(
      find.text('The service is temporarily unavailable. Please try again.'),
      findsOneWidget,
    );
  });

  testWidgets('StoryViewerScreen preserves reply cancel, empty, and success', (
    tester,
  ) async {
    final replyBodies = <String>[];
    final mock = MockClient((req) async {
      if (req.url.path.endsWith('/views')) return http.Response('', 204);
      if (req.url.path.endsWith('/reply')) {
        replyBodies.add(
          (jsonDecode(req.body) as Map<String, dynamic>)['text'] as String,
        );
        return http.Response(
          '{"chatId":"chat-1","messageId":"message-1"}',
          200,
          headers: const {'content-type': 'application/json'},
        );
      }
      return http.Response('', 404);
    });
    await tester.pumpWidget(
      wrap(const StoryViewerScreen(storyIds: ['story-1']), client: mock),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(StoryViewerScreen.replyButtonKey));
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(TextButton, 'Cancel'));
    await tester.pumpAndSettle();
    expect(replyBodies, isEmpty);

    await tester.tap(find.byKey(StoryViewerScreen.replyButtonKey));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField).last, '   ');
    await tester.tap(find.widgetWithText(FilledButton, 'Reply'));
    await tester.pumpAndSettle();
    expect(replyBodies, isEmpty);

    await tester.tap(find.byKey(StoryViewerScreen.replyButtonKey));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField).last, '  Hello there  ');
    await tester.tap(find.widgetWithText(FilledButton, 'Reply'));
    await tester.pumpAndSettle();

    expect(replyBodies, ['Hello there']);
    expect(find.text('Reply sent'), findsOneWidget);
  });

  testWidgets('StoryViewerScreen shows author-only reaction chips', (
    tester,
  ) async {
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
              guestCredentialsStorage: ref.watch(
                guestCredentialsStorageProvider,
              ),
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
          storyReactionsProvider('story-1').overrideWith(
            (ref) async => const [
              StoryReactionData(reactorProfileId: 'viewer-1', emoji: '🔥'),
              StoryReactionData(reactorProfileId: 'viewer-2', emoji: '🔥'),
              StoryReactionData(reactorProfileId: 'viewer-3', emoji: '❤️'),
            ],
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

    expect(find.byKey(StoryViewerScreen.reactionsRowKey), findsOneWidget);
    expect(find.text('2'), findsOneWidget);
  });
}
