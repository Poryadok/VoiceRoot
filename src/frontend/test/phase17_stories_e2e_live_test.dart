import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/stories_client.dart';

import 'support/live_gateway_harness.dart';

void main() {
  test(
    'phase17: friend sees story in feed after publish',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final author = await ctx.registerUser('p17-story-author');
      final viewer = await ctx.registerUser('p17-story-viewer');
      await ctx.inviteAndAcceptFriends(author, viewer);

      final stories = VoiceStoriesClient(gateway: ctx.gatewayHttp());
      final created = await stories.createTextStory(
        authorization: author.authorizationHeader,
        text: 'Phase 17 live e2e',
      );
      expect(created, isA<StoriesApiOk<StoryData>>());
      final story = (created as StoriesApiOk<StoryData>).data;
      expect(story.id, isNotEmpty);

      final feed = await stories.getFeed(authorization: viewer.authorizationHeader);
      expect(feed, isA<StoriesApiOk<StoryFeedPage>>());
      final items = (feed as StoriesApiOk<StoryFeedPage>).data.stories;
      expect(
        items.map((s) => s.id),
        contains(story.id),
        reason: 'friend must see active story in feed',
      );

      final viewed = await stories.markViewed(
        authorization: viewer.authorizationHeader,
        storyId: story.id,
      );
      expect(viewed, isA<StoriesApiOk<void>>());

      final highlights = await stories.getHighlights(
        authorization: viewer.authorizationHeader,
        profileId: author.activeProfileId,
      );
      expect(highlights, isA<StoriesApiOk<List<HighlightData>>>());
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
