import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/stories_client.dart';
import 'package:voice_frontend/state/stories_providers.dart';
import 'support/auth_test_overrides.dart';

void main() {
  test('Story public wire preserves timestamp and mention audience fields', () {
    for (final camelCase in [false, true]) {
      final story = StoryData.fromJson({
        'id': 'story-1',
        'type': 'text',
        camelCase ? 'createdAt' : 'created_at': '2026-11-03T10:00:00Z',
        camelCase ? 'expiresAt' : 'expires_at': '2026-11-04T10:00:00Z',
        camelCase ? 'mentionProfileIdsJson' : 'mention_profile_ids_json':
            '["viewer-1"]',
      });
      expect(story.createdAt, DateTime.utc(2026, 11, 3, 10));
      expect(story.expiresAt, DateTime.utc(2026, 11, 4, 10));
      expect(story.mentionProfileIds, ['viewer-1']);
    }
    expect(
      StoryData.fromJson({'mention_profile_ids_json': '[1]'}).mentionProfileIds,
      isEmpty,
    );
    expect(
      StoryData.fromJson({
        'mention_profile_ids_json': 'invalid',
      }).mentionProfileIds,
      isEmpty,
    );
  });

  test(
    'viewer detail/viewers/reactions failures stay retryable, not empty data',
    () async {
      final container = ProviderContainer(
        overrides: voiceAppTestOverrides(
          client: MockClient(
            (_) async => http.Response('{"message":"private upstream"}', 500),
          ),
        ),
      );
      addTearDown(container.dispose);
      for (final provider in [
        storyDetailProvider('story-1'),
        storyViewersProvider('story-1'),
        storyReactionsProvider('story-1'),
      ]) {
        await expectLater(
          container.read(provider.future),
          throwsA(
            isA<StoriesLoadException>().having(
              (error) => error.statusCode,
              'status',
              500,
            ),
          ),
        );
      }
    },
  );
}
