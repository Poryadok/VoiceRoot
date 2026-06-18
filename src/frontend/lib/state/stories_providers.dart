import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/stories_client.dart';
import 'auth_providers.dart';

final voiceStoriesClientProvider = Provider<VoiceStoriesClient>((ref) {
  return VoiceStoriesClient(gateway: ref.watch(gatewayHttpClientProvider));
});

final storyFeedProvider = FutureProvider<StoryFeedPage>((ref) async {
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null) {
    return const StoryFeedPage(stories: []);
  }
  final result =
      await ref.watch(voiceStoriesClientProvider).getFeed(authorization: auth);
  return switch (result) {
    StoriesApiOk(:final data) => data,
    StoriesApiFailure() => const StoryFeedPage(stories: []),
  };
});

/// Profile ids with at least one active story in the current user's feed.
final activeStoryAuthorIdsProvider = Provider<Set<String>>((ref) {
  final feed = ref.watch(storyFeedProvider);
  return feed.when(
    data: (page) {
      if (page.feedGroups.isNotEmpty) {
        return page.feedGroups.map((g) => g.authorProfileId).toSet();
      }
      return page.stories.map((s) => s.authorProfileId).toSet();
    },
    loading: () => const {},
    error: (_, _) => const {},
  );
});

final profileHighlightsProvider =
    FutureProvider.family<List<HighlightData>, String>((ref, profileId) async {
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null || profileId.isEmpty) return const [];
  final result = await ref
      .watch(voiceStoriesClientProvider)
      .getHighlights(authorization: auth, profileId: profileId);
  return switch (result) {
    StoriesApiOk(:final data) => data,
    StoriesApiFailure() => const [],
  };
});

final profileStoriesProvider =
    FutureProvider.family<List<StoryData>, String>((ref, profileId) async {
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null || profileId.isEmpty) return const [];
  final result = await ref.watch(voiceStoriesClientProvider).getProfileStories(
        authorization: auth,
        profileId: profileId,
      );
  return switch (result) {
    StoriesApiOk(:final data) => data,
    StoriesApiFailure() => const [],
  };
});

final storyDetailProvider =
    FutureProvider.family<StoryData?, String>((ref, storyId) async {
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null || storyId.isEmpty) return null;
  final result = await ref
      .watch(voiceStoriesClientProvider)
      .getStory(authorization: auth, storyId: storyId);
  return switch (result) {
    StoriesApiOk(:final data) => data,
    StoriesApiFailure() => null,
  };
});

final storyArchiveProvider = FutureProvider<List<StoryData>>((ref) async {
  final auth = ref.watch(authorizationHeaderProvider);
  final profileId = ref.watch(authControllerProvider).activeProfileId;
  if (auth == null || profileId == null) return const [];
  final result = await ref.watch(voiceStoriesClientProvider).getArchive(
        authorization: auth,
        profileId: profileId,
      );
  return switch (result) {
    StoriesApiOk(:final data) => data,
    StoriesApiFailure() => const [],
  };
});

final storyViewersProvider =
    FutureProvider.family<List<String>, String>((ref, storyId) async {
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null || storyId.isEmpty) return const [];
  final result = await ref
      .watch(voiceStoriesClientProvider)
      .getViewers(authorization: auth, storyId: storyId);
  return switch (result) {
    StoriesApiOk(:final data) => data,
    StoriesApiFailure() => const [],
  };
});
