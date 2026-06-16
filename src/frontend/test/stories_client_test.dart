import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/stories_client.dart';

import 'support/gateway_test_client.dart';

void main() {
  const config = GatewayConfig(baseUrl: 'http://api.test');
  const auth = 'Bearer access-token';

  group('VoiceStoriesClient.createTextStory', () {
    test('POST /api/v1/stories parses story response', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'POST');
        expect(req.url.path, '/api/v1/stories');
        expect(req.headers['Authorization'], auth);
        final body = jsonDecode(req.body) as Map<String, dynamic>;
        expect(body['type'], 'text');
        expect(body['text_content'], 'hello');
        return http.Response(
          jsonEncode({
            'story': {
              'id': 'story-1',
              'author_profile_id': 'profile-1',
              'type': 'text',
              'text_content': 'hello',
              'visibility': 'friends',
            },
          }),
          200,
        );
      });
      final client = VoiceStoriesClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
      final result = await client.createTextStory(
        authorization: auth,
        text: 'hello',
      );
      expect(result, isA<StoriesApiOk<StoryData>>());
      final story = (result as StoriesApiOk<StoryData>).data;
      expect(story.id, 'story-1');
      expect(story.type, 'text');
      expect(story.textContent, 'hello');
    });
  });

  group('VoiceStoriesClient.getFeed', () {
    test('GET /api/v1/stories/feed parses stories list', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'GET');
        expect(req.url.path, '/api/v1/stories/feed');
        expect(req.url.queryParameters['limit'], '20');
        return http.Response(
          jsonEncode({
            'stories': [
              {
                'id': 'story-1',
                'author_profile_id': 'profile-2',
                'type': 'text',
              },
            ],
            'next_cursor': 'cursor-2',
          }),
          200,
        );
      });
      final client = VoiceStoriesClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
      final result = await client.getFeed(authorization: auth);
      expect(result, isA<StoriesApiOk<StoryFeedPage>>());
      final page = (result as StoriesApiOk<StoryFeedPage>).data;
      expect(page.stories, hasLength(1));
      expect(page.stories.first.id, 'story-1');
      expect(page.nextCursor, 'cursor-2');
    });
  });

  group('VoiceStoriesClient.markViewed', () {
    test('POST /api/v1/stories/{id}/views returns success', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'POST');
        expect(req.url.path, '/api/v1/stories/story-1/views');
        return http.Response('', 204);
      });
      final client = VoiceStoriesClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
      final result = await client.markViewed(
        authorization: auth,
        storyId: 'story-1',
      );
      expect(result, isA<StoriesApiOk<void>>());
    });
  });

  group('VoiceStoriesClient.reactToStory', () {
    test('POST /api/v1/stories/{id}/reactions', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'POST');
        expect(req.url.path, '/api/v1/stories/story-1/reactions');
        final body = jsonDecode(req.body) as Map<String, dynamic>;
        expect(body['emoji'], '🔥');
        return http.Response('', 204);
      });
      final client = VoiceStoriesClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
      final result = await client.reactToStory(
        authorization: auth,
        storyId: 'story-1',
        emoji: '🔥',
      );
      expect(result, isA<StoriesApiOk<void>>());
    });
  });
}
