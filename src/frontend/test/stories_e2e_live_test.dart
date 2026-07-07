import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/stories_client.dart';

import 'support/live_gateway_harness.dart';

void main() {
  test(
    'stories: mention, feed, reply→DM, highlight privacy',
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
      final stranger = await ctx.registerUser('p17-story-stranger');
      await ctx.inviteAndAcceptFriends(author, viewer);

      final stories = VoiceStoriesClient(gateway: ctx.gatewayHttp());
      final created = await stories.createTextStory(
        authorization: author.authorizationHeader,
        text: 'app stack7 live e2e @viewer',
        mentionProfileIds: [viewer.activeProfileId],
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

      final reply = await stories.replyToStory(
        authorization: viewer.authorizationHeader,
        storyId: story.id,
        text: 'phase17 private reply',
      );
      expect(reply, isA<StoriesApiOk<StoryReplyResult>>());
      final replyData = (reply as StoriesApiOk<StoryReplyResult>).data;
      expect(replyData.chatId, isNotEmpty);
      expect(replyData.messageId, isNotEmpty);

      final messages = ctx.messagesClient();
      final dmHistory = await messages.getMessages(
        authorization: author.authorizationHeader,
        chatId: replyData.chatId,
      );
      expect(dmHistory, isA<MessagesApiOk<MessageListData>>());
      final dmMsgs = (dmHistory as MessagesApiOk<MessageListData>).data.messages;
      expect(
        dmMsgs.any(
          (m) =>
              m.id == replyData.messageId &&
              m.content == 'phase17 private reply',
        ),
        isTrue,
        reason: 'story reply must land in private DM for author',
      );

      final highlight = await stories.createHighlight(
        authorization: author.authorizationHeader,
        name: 'Live wins',
        visibility: 'friends',
      );
      expect(highlight, isA<StoriesApiOk<HighlightData>>());
      final highlightId = (highlight as StoriesApiOk<HighlightData>).data.id;
      expect(highlightId, isNotEmpty);

      final added = await stories.addToHighlight(
        authorization: author.authorizationHeader,
        highlightId: highlightId,
        storyId: story.id,
      );
      expect(added, isA<StoriesApiOk<void>>());

      final friendHighlights = await stories.getHighlights(
        authorization: viewer.authorizationHeader,
        profileId: author.activeProfileId,
      );
      expect(friendHighlights, isA<StoriesApiOk<List<HighlightData>>>());
      final friendIds =
          (friendHighlights as StoriesApiOk<List<HighlightData>>)
              .data
              .map((h) => h.id)
              .toList();
      expect(friendIds, contains(highlightId),
          reason: 'friend must see friends-only highlight');

      final strangerHighlights = await stories.getHighlights(
        authorization: stranger.authorizationHeader,
        profileId: author.activeProfileId,
      );
      expect(strangerHighlights, isA<StoriesApiOk<List<HighlightData>>>());
      final strangerIds =
          (strangerHighlights as StoriesApiOk<List<HighlightData>>)
              .data
              .map((h) => h.id)
              .toList();
      expect(
        strangerIds,
        isNot(contains(highlightId)),
        reason: 'stranger must not see friends-only highlight',
      );
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
