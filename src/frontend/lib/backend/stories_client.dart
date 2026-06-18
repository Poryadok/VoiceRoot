import 'package:voice_frontend/backend/api_result.dart';
import 'package:voice_frontend/backend/gateway_api_error.dart';
import 'package:voice_frontend/backend/gateway_http.dart';

sealed class StoriesApiResult<T> {
  const StoriesApiResult();
}

final class StoriesApiOk<T> extends StoriesApiResult<T> {
  const StoriesApiOk(this.data);
  final T data;
}

final class StoriesApiFailure extends StoriesApiResult<Never> {
  const StoriesApiFailure({
    required this.message,
    this.errorCode,
    this.statusCode,
  });

  final String message;
  final String? errorCode;
  final int? statusCode;
}

class StoryData {
  const StoryData({
    required this.id,
    required this.authorProfileId,
    required this.type,
    this.textContent,
    this.mediaFileId,
    this.gameTag,
    this.lfpCriteriaJson,
    this.textStyleJson,
    this.visibility = 'friends',
    this.viewCount = 0,
    this.isLookingForParty = false,
  });

  final String id;
  final String authorProfileId;
  final String type;
  final String? textContent;
  final String? mediaFileId;
  final String? textStyleJson;
  final String? gameTag;
  final String? lfpCriteriaJson;
  final String visibility;
  final int viewCount;
  final bool isLookingForParty;

  factory StoryData.fromJson(Map<String, dynamic> json) {
    return StoryData(
      id: json['id'] as String? ?? '',
      authorProfileId: json['authorProfileId'] as String? ??
          json['author_profile_id'] as String? ??
          '',
      type: json['type'] as String? ?? '',
      textContent:
          json['textContent'] as String? ?? json['text_content'] as String?,
      mediaFileId:
          json['mediaFileId'] as String? ?? json['media_file_id'] as String?,
      textStyleJson: json['textStyleJson'] as String? ??
          json['text_style_json'] as String?,
      gameTag: json['gameTag'] as String? ?? json['game_tag'] as String?,
      lfpCriteriaJson: json['lfpCriteriaJson'] as String? ??
          json['lfp_criteria_json'] as String?,
      visibility: json['visibility'] as String? ?? 'friends',
      viewCount: json['viewCount'] as int? ?? json['view_count'] as int? ?? 0,
      isLookingForParty: json['isLookingForParty'] as bool? ??
          json['is_looking_for_party'] as bool? ??
          false,
    );
  }
}

class StoryFeedGroup {
  const StoryFeedGroup({
    required this.authorProfileId,
    this.stories = const [],
  });

  final String authorProfileId;
  final List<StoryData> stories;

  factory StoryFeedGroup.fromJson(Map<String, dynamic> json) {
    final rawStories = json['stories'] as List<dynamic>? ?? const [];
    return StoryFeedGroup(
      authorProfileId: json['authorProfileId'] as String? ??
          json['author_profile_id'] as String? ??
          '',
      stories: rawStories
          .map((e) => StoryData.fromJson(e as Map<String, dynamic>))
          .toList(),
    );
  }
}

class StoryFeedPage {
  const StoryFeedPage({
    required this.stories,
    this.feedGroups = const [],
    this.nextCursor,
    this.hasMore = false,
  });

  final List<StoryData> stories;
  final List<StoryFeedGroup> feedGroups;
  final String? nextCursor;
  final bool hasMore;
}

class HighlightData {
  const HighlightData({
    required this.id,
    required this.name,
    this.profileId,
    this.storyIds = const [],
    this.visibility = 'everyone',
  });

  final String id;
  final String name;
  final String? profileId;
  final List<String> storyIds;
  final String visibility;

  factory HighlightData.fromJson(Map<String, dynamic> json) {
    final rawIds = json['storyIds'] ?? json['story_ids'];
    return HighlightData(
      id: json['id'] as String? ?? '',
      name: json['name'] as String? ?? '',
      profileId:
          json['profileId'] as String? ?? json['profile_id'] as String?,
      storyIds: rawIds is List ? rawIds.map((e) => '$e').toList() : const [],
      visibility: json['visibility'] as String? ?? 'everyone',
    );
  }
}

class StoryReactionData {
  const StoryReactionData({
    required this.reactorProfileId,
    required this.emoji,
  });

  final String reactorProfileId;
  final String emoji;

  factory StoryReactionData.fromJson(Map<String, dynamic> json) {
    return StoryReactionData(
      reactorProfileId: json['reactorProfileId'] as String? ??
          json['reactor_profile_id'] as String? ??
          '',
      emoji: json['emoji'] as String? ?? '',
    );
  }
}

class StoryReplyResult {
  const StoryReplyResult({required this.chatId, required this.messageId});

  final String chatId;
  final String messageId;

  factory StoryReplyResult.fromJson(Map<String, dynamic> json) {
    return StoryReplyResult(
      chatId: json['chatId'] as String? ?? json['chat_id'] as String? ?? '',
      messageId:
          json['messageId'] as String? ?? json['message_id'] as String? ?? '',
    );
  }
}

/// REST client for Story routes via API Gateway (`/api/v1/stories/**`).
class VoiceStoriesClient {
  VoiceStoriesClient({required GatewayHttpClient gateway}) : _gateway = gateway;

  final GatewayHttpClient _gateway;

  StoriesApiFailure _failure(GatewayApiError error) => StoriesApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      );

  StoryData _parseStory(Map<String, dynamic> data) {
    return StoryData.fromJson(
      data['story'] as Map<String, dynamic>? ?? data,
    );
  }

  List<StoryData> _parseStoriesList(Map<String, dynamic> data) {
    final raw = data['stories'] as List<dynamic>? ??
        (data['storyList'] as Map<String, dynamic>?)?['stories']
            as List<dynamic>? ??
        (data['story_list'] as Map<String, dynamic>?)?['stories']
            as List<dynamic>? ??
        const [];
    return raw
        .map((e) => StoryData.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  List<HighlightData> _parseHighlightsList(Map<String, dynamic> data) {
    final raw = data['highlights'] as List<dynamic>? ??
        (data['highlightList'] as Map<String, dynamic>?)?['highlights']
            as List<dynamic>? ??
        (data['highlight_list'] as Map<String, dynamic>?)?['highlights']
            as List<dynamic>? ??
        const [];
    return raw
        .map((e) => HighlightData.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  List<StoryFeedGroup> _parseFeedGroups(Map<String, dynamic> data) {
    final raw = data['feedGroups'] as List<dynamic>? ??
        data['feed_groups'] as List<dynamic>? ??
        const [];
    return raw
        .map((e) => StoryFeedGroup.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<StoriesApiResult<StoryData>> createStory({
    required String authorization,
    required String type,
    String? textContent,
    String? mediaFileId,
    String? textStyleJson,
    String? gameTag,
    List<String> mentionProfileIds = const [],
    String visibility = 'friends',
  }) async {
    final body = <String, dynamic>{
      'type': type,
      'visibility': visibility,
    };
    if (textContent != null) body['text_content'] = textContent;
    if (mediaFileId != null) body['media_file_id'] = mediaFileId;
    if (textStyleJson != null) body['text_style_json'] = textStyleJson;
    if (gameTag != null && gameTag.isNotEmpty) body['game_tag'] = gameTag;
    if (mentionProfileIds.isNotEmpty) {
      body['mention_profile_ids'] = mentionProfileIds;
    }

    final result = await _gateway.postJson(
      uri: _gateway.resolve('/api/v1/stories'),
      authorization: authorization,
      body: body,
    );
    return switch (result) {
      GatewayHttpOk(:final data) => StoriesApiOk(_parseStory(data)),
      GatewayHttpFailure(:final error) => _failure(error),
    };
  }

  Future<StoriesApiResult<StoryData>> createTextStory({
    required String authorization,
    required String text,
    String? textStyleJson,
    String? gameTag,
    List<String> mentionProfileIds = const [],
    String visibility = 'friends',
  }) {
    return createStory(
      authorization: authorization,
      type: 'text',
      textContent: text,
      textStyleJson: textStyleJson,
      gameTag: gameTag,
      mentionProfileIds: mentionProfileIds,
      visibility: visibility,
    );
  }

  Future<StoriesApiResult<void>> deleteStory({
    required String authorization,
    required String storyId,
  }) async {
    final result = await _gateway.deleteEmpty(
      uri: _gateway.resolve('/api/v1/stories/$storyId'),
      authorization: authorization,
    );
    return switch (result) {
      GatewayHttpOk() => const StoriesApiOk(null),
      GatewayHttpFailure(:final error) => _failure(error),
    };
  }

  Future<StoriesApiResult<StoryData>> getStory({
    required String authorization,
    required String storyId,
  }) async {
    final result = await _gateway.getJson(
      _gateway.resolve('/api/v1/stories/$storyId'),
      authorization: authorization,
    );
    return switch (result) {
      GatewayHttpOk(:final data) => StoriesApiOk(_parseStory(data)),
      GatewayHttpFailure(:final error) => _failure(error),
    };
  }

  Future<StoriesApiResult<StoryFeedPage>> getFeed({
    required String authorization,
    String? cursor,
    int limit = 20,
  }) async {
    final query = <String, String>{'limit': '$limit'};
    if (cursor != null && cursor.isNotEmpty) {
      query['cursor'] = cursor;
    }
    final result = await _gateway.getJson(
      _gateway.replace(path: '/api/v1/stories/feed', queryParameters: query),
      authorization: authorization,
    );
    return switch (result) {
      GatewayHttpOk(:final data) => StoriesApiOk(
          StoryFeedPage(
            stories: _parseStoriesList(data),
            feedGroups: _parseFeedGroups(data),
            nextCursor: data['nextCursor'] as String? ??
                data['next_cursor'] as String?,
            hasMore: data['hasMore'] as bool? ??
                data['has_more'] as bool? ??
                (data['page'] as Map<String, dynamic>?)?['hasMore'] as bool? ??
                (data['page'] as Map<String, dynamic>?)?['has_more'] as bool? ??
                false,
          ),
        ),
      GatewayHttpFailure(:final error) => _failure(error),
    };
  }

  Future<StoriesApiResult<List<StoryData>>> getProfileStories({
    required String authorization,
    required String profileId,
  }) async {
    final result = await _gateway.getJson(
      _gateway.resolve('/api/v1/stories/profiles/$profileId'),
      authorization: authorization,
    );
    return switch (result) {
      GatewayHttpOk(:final data) => StoriesApiOk(_parseStoriesList(data)),
      GatewayHttpFailure(:final error) => _failure(error),
    };
  }

  Future<StoriesApiResult<void>> markViewed({
    required String authorization,
    required String storyId,
    bool anonymous = false,
  }) async {
    final result = await _gateway.postEmpty(
      uri: _gateway.resolve('/api/v1/stories/$storyId/views'),
      authorization: authorization,
      jsonBody: {'anonymous': anonymous},
    );
    return switch (result) {
      GatewayHttpOk() => const StoriesApiOk(null),
      GatewayHttpFailure(:final error) => _failure(error),
    };
  }

  Future<StoriesApiResult<List<String>>> getViewers({
    required String authorization,
    required String storyId,
  }) async {
    final result = await _gateway.getJson(
      _gateway.resolve('/api/v1/stories/$storyId/viewers'),
      authorization: authorization,
    );
    return switch (result) {
      GatewayHttpOk(:final data) => StoriesApiOk(
          ((data['viewerList'] as Map<String, dynamic>?)?['viewerProfileIds']
                      as List<dynamic>? ??
                  (data['viewer_list'] as Map<String, dynamic>?)?[
                      'viewer_profile_ids'] as List<dynamic>? ??
                  data['viewerProfileIds'] as List<dynamic>? ??
                  data['viewer_profile_ids'] as List<dynamic>? ??
                  const [])
              .map((e) => '$e')
              .toList(),
        ),
      GatewayHttpFailure(:final error) => _failure(error),
    };
  }

  Future<StoriesApiResult<List<StoryReactionData>>> getStoryReactions({
    required String authorization,
    required String storyId,
  }) async {
    final result = await _gateway.getJson(
      _gateway.resolve('/api/v1/stories/$storyId/reactions'),
      authorization: authorization,
    );
    return switch (result) {
      GatewayHttpOk(:final data) => StoriesApiOk(
          ((data['reactions'] as List<dynamic>?) ??
                  (data['storyReactions'] as Map<String, dynamic>?)?[
                      'reactions'] as List<dynamic>? ??
                  const [])
              .map(
                (e) => StoryReactionData.fromJson(e as Map<String, dynamic>),
              )
              .toList(),
        ),
      GatewayHttpFailure(:final error) => _failure(error),
    };
  }

  Future<StoriesApiResult<StoryReplyResult>> replyToStory({
    required String authorization,
    required String storyId,
    required String text,
  }) async {
    final result = await _gateway.postJson(
      uri: _gateway.resolve('/api/v1/stories/$storyId/reply'),
      authorization: authorization,
      body: {'text': text},
    );
    return switch (result) {
      GatewayHttpOk(:final data) => StoriesApiOk(StoryReplyResult.fromJson(data)),
      GatewayHttpFailure(:final error) => _failure(error),
    };
  }

  Future<StoriesApiResult<void>> reactToStory({
    required String authorization,
    required String storyId,
    required String emoji,
  }) async {
    final result = await _gateway.postEmpty(
      uri: _gateway.resolve('/api/v1/stories/$storyId/reactions'),
      authorization: authorization,
      jsonBody: {'emoji': emoji},
    );
    return switch (result) {
      GatewayHttpOk() => const StoriesApiOk(null),
      GatewayHttpFailure(:final error) => _failure(error),
    };
  }

  Future<StoriesApiResult<List<StoryData>>> getArchive({
    required String authorization,
    String? profileId,
  }) async {
    final query = <String, String>{};
    if (profileId != null && profileId.isNotEmpty) {
      query['profile_id'] = profileId;
    }
    final result = await _gateway.getJson(
      _gateway.replace(
        path: '/api/v1/stories/archive',
        queryParameters: query.isEmpty ? null : query,
      ),
      authorization: authorization,
    );
    return switch (result) {
      GatewayHttpOk(:final data) => StoriesApiOk(_parseStoriesList(data)),
      GatewayHttpFailure(:final error) => _failure(error),
    };
  }

  Future<StoriesApiResult<List<HighlightData>>> getHighlights({
    required String authorization,
    required String profileId,
  }) async {
    final result = await _gateway.getJson(
      _gateway.replace(
        path: '/api/v1/stories/highlights',
        queryParameters: {'profile_id': profileId},
      ),
      authorization: authorization,
    );
    return switch (result) {
      GatewayHttpOk(:final data) => StoriesApiOk(_parseHighlightsList(data)),
      GatewayHttpFailure(:final error) => _failure(error),
    };
  }

  Future<StoriesApiResult<HighlightData>> createHighlight({
    required String authorization,
    required String name,
    String visibility = 'everyone',
  }) async {
    final result = await _gateway.postJson(
      uri: _gateway.resolve('/api/v1/stories/highlights'),
      authorization: authorization,
      body: {'name': name, 'visibility': visibility},
    );
    return switch (result) {
      GatewayHttpOk(:final data) => StoriesApiOk(
          HighlightData.fromJson(
            data['highlight'] as Map<String, dynamic>? ?? data,
          ),
        ),
      GatewayHttpFailure(:final error) => _failure(error),
    };
  }

  Future<StoriesApiResult<HighlightData>> updateHighlight({
    required String authorization,
    required String highlightId,
    required String name,
    String? visibility,
  }) async {
    final body = <String, dynamic>{'name': name};
    if (visibility != null) body['visibility'] = visibility;
    final result = await _gateway.patchJson(
      uri: _gateway.resolve('/api/v1/stories/highlights/$highlightId'),
      authorization: authorization,
      body: body,
    );
    return switch (result) {
      GatewayHttpOk(:final data) => StoriesApiOk(
          HighlightData.fromJson(
            data['highlight'] as Map<String, dynamic>? ?? data,
          ),
        ),
      GatewayHttpFailure(:final error) => _failure(error),
    };
  }

  Future<StoriesApiResult<void>> deleteHighlight({
    required String authorization,
    required String highlightId,
  }) async {
    final result = await _gateway.deleteEmpty(
      uri: _gateway.resolve('/api/v1/stories/highlights/$highlightId'),
      authorization: authorization,
    );
    return switch (result) {
      GatewayHttpOk() => const StoriesApiOk(null),
      GatewayHttpFailure(:final error) => _failure(error),
    };
  }

  Future<StoriesApiResult<void>> addToHighlight({
    required String authorization,
    required String highlightId,
    required String storyId,
  }) async {
    final result = await _gateway.postEmpty(
      uri: _gateway.resolve(
        '/api/v1/stories/highlights/$highlightId/stories',
      ),
      authorization: authorization,
      jsonBody: {'story_id': storyId},
    );
    return switch (result) {
      GatewayHttpOk() => const StoriesApiOk(null),
      GatewayHttpFailure(:final error) => _failure(error),
    };
  }

  Future<StoriesApiResult<void>> removeFromHighlight({
    required String authorization,
    required String highlightId,
    required String storyId,
  }) async {
    final result = await _gateway.deleteEmpty(
      uri: _gateway.resolve(
        '/api/v1/stories/highlights/$highlightId/stories/$storyId',
      ),
      authorization: authorization,
    );
    return switch (result) {
      GatewayHttpOk() => const StoriesApiOk(null),
      GatewayHttpFailure(:final error) => _failure(error),
    };
  }

  Future<StoriesApiResult<StoryData>> createLookingForParty({
    required String authorization,
    required String criteriaJson,
    String? mediaFileId,
  }) async {
    final body = <String, dynamic>{'criteria_json': criteriaJson};
    if (mediaFileId != null) body['media_file_id'] = mediaFileId;

    final result = await _gateway.postJson(
      uri: _gateway.resolve('/api/v1/stories/looking-for-party'),
      authorization: authorization,
      body: body,
    );
    return switch (result) {
      GatewayHttpOk(:final data) => StoriesApiOk(_parseStory(data)),
      GatewayHttpFailure(:final error) => _failure(error),
    };
  }
}
