import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/stories_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/ui/stories/lfp_story_card.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  const story = StoryData(
    id: 'lfp-1',
    authorProfileId: 'author-1',
    type: 'text',
    textContent: 'Need duo',
    gameTag: 'dota-2',
    lfpCriteriaJson: '{"mode":"5v5"}',
    isLookingForParty: true,
    visibility: 'everyone',
  );

  Widget appWithRealChatsClient(http.Client client) {
    return ProviderScope(
      overrides: [
        ...voiceAppTestOverrides(client: client),
        voiceChatsClientProvider.overrideWithValue(
          VoiceChatsClient(
            gateway: GatewayHttpClient(
              httpClient: client,
              config: const GatewayConfig(baseUrl: 'http://localhost:9999'),
            ),
          ),
        ),
      ],
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: const Scaffold(body: LfpStoryCard(story: story)),
      ),
    );
  }

  testWidgets('LfpStoryCard shows join action per stories.md', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: const Scaffold(body: LfpStoryCard(story: story)),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('lfp_story_join')), findsOneWidget);
    expect(find.byKey(const Key('lfp_story_invite')), findsOneWidget);
  });

  testWidgets('LFP join hides upstream failure details', (tester) async {
    const upstreamDetail = 'private story service diagnostic';
    final client = MockClient(
      (_) async => http.Response(
        jsonEncode({'error_code': 'internal_error', 'message': upstreamDetail}),
        500,
      ),
    );
    await tester.pumpWidget(
      ProviderScope(
        overrides: voiceAppTestOverrides(client: client),
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: LfpStoryCard(story: story)),
        ),
      ),
    );

    await tester.tap(find.byKey(LfpStoryCard.joinKey));
    await tester.pump();
    await tester.pumpAndSettle();

    expect(find.byType(SnackBar), findsOneWidget);
    expect(find.text('Could not complete this action.'), findsOneWidget);
    expect(find.text(upstreamDetail), findsNothing);
  });

  testWidgets('LFP invite maps backend unavailable without raw details', (
    tester,
  ) async {
    const upstreamDetail = 'private story service diagnostic';
    final client = MockClient(
      (_) async => http.Response(
        jsonEncode({'error_code': 'unavailable', 'message': upstreamDetail}),
        503,
      ),
    );
    await tester.pumpWidget(
      ProviderScope(
        overrides: voiceAppTestOverrides(client: client),
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: LfpStoryCard(story: story)),
        ),
      ),
    );

    await tester.tap(find.byKey(LfpStoryCard.inviteKey));
    await tester.pumpAndSettle();

    expect(
      find.text(
        'Social and chat features are unavailable. '
        'Start the full API stack (docker compose --profile app).',
      ),
      findsOneWidget,
    );
    expect(find.text(upstreamDetail), findsNothing);
  });

  testWidgets('LFP write hides upstream DM failure details', (tester) async {
    const upstreamDetail = 'private chat service diagnostic';
    final client = MockClient(
      (_) async => http.Response(
        jsonEncode({'error_code': 'internal_error', 'message': upstreamDetail}),
        500,
      ),
    );
    await tester.pumpWidget(appWithRealChatsClient(client));

    await tester.tap(find.byKey(LfpStoryCard.writeKey));
    await tester.pumpAndSettle();

    expect(find.byType(SnackBar), findsOneWidget);
    expect(find.text('Could not complete this action.'), findsOneWidget);
    expect(find.text(upstreamDetail), findsNothing);
  });

  testWidgets('LFP write opens the existing DM on success', (tester) async {
    http.Request? createRequest;
    final client = MockClient((request) async {
      if (request.url.path == '/api/v1/chats/dm') {
        createRequest = request;
        return http.Response(
          jsonEncode({
            'chat': {'id': 'dm-created'},
          }),
          200,
        );
      }
      return http.Response('{}', 404);
    });
    await tester.pumpWidget(appWithRealChatsClient(client));

    await tester.tap(find.byKey(LfpStoryCard.writeKey));
    await tester.pumpAndSettle();

    expect(createRequest, isNotNull);
    expect(createRequest!.method, 'POST');
    final container = ProviderScope.containerOf(
      tester.element(find.byType(LfpStoryCard)),
    );
    expect(container.read(selectedChatIdProvider), 'dm-created');
    expect(find.byType(SnackBar), findsNothing);
  });
}
