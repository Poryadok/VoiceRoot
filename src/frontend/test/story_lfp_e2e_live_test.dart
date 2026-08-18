import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/matchmaking_client.dart';
import 'package:voice_frontend/backend/stories_client.dart';

import 'support/live_gateway_harness.dart';

/// ST-04: LFP story → JOIN → DecideLfp ACCEPT → party.
void main() {
  test(
    'LFP story JOIN accept creates party (ST-04)',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final author = await ctx.registerUser('st04-author');
      final responder = await ctx.registerUser('st04-resp');
      await ctx.inviteAndAcceptFriends(author, responder);
      await ctx.allowOpenGamingPrivacy(author);

      final mm = VoiceMatchmakingClient(gateway: ctx.gatewayHttp());
      final games = await mm.listGames(authorization: author.authorizationHeader);
      expect(games, isA<MatchmakingApiOk<GameListData>>());
      final duo = (games as MatchmakingApiOk<GameListData>).data.games
          .firstWhere((g) => g.name == 'MM Duo Live');

      final stories = VoiceStoriesClient(gateway: ctx.gatewayHttp());
      final criteria = jsonEncode({
        'game_id': duo.id,
        'mode': 'Duo',
        'region': 'eu',
        'visibility': 'everyone',
      });
      final created = await stories.createLookingForParty(
        authorization: author.authorizationHeader,
        criteriaJson: criteria,
      );
      expect(created, isA<StoriesApiOk<StoryData>>());
      final storyId = (created as StoriesApiOk<StoryData>).data.id;

      final responded = await stories.respondToLfpStory(
        authorization: responder.authorizationHeader,
        storyId: storyId,
        responseType: 'JOIN',
      );
      expect(responded, isA<StoriesApiOk<void>>(), reason: responded is StoriesApiFailure ? '${responded.message} ${responded.errorCode} ${responded.statusCode}' : '$responded');

      DecideLfpRequestData? decided;
      for (var i = 0; i < 40; i++) {
        final result = await mm.decideLfpRequest(
          authorization: author.authorizationHeader,
          storyId: storyId,
          responderProfileId: responder.activeProfileId,
          responseType: 'JOIN',
          decision: 'ACCEPT',
        );
        if (result is MatchmakingApiOk<DecideLfpRequestData> &&
            result.data.status == 'accepted') {
          decided = result.data;
          break;
        }
        await Future<void>.delayed(const Duration(milliseconds: 500));
      }
      expect(decided, isNotNull, reason: 'storyconsume must create pending LFP');
      expect(decided!.partyId, isNotEmpty);
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
    timeout: const Timeout(Duration(minutes: 2)),
  );
}
