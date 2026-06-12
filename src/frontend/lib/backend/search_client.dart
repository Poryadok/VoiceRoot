import 'dart:async';

import 'gateway_api_error.dart';
import 'gateway_http.dart';

sealed class SearchApiResult<T> {
  const SearchApiResult();
}

final class SearchApiOk<T> extends SearchApiResult<T> {
  const SearchApiOk(this.data);
  final T data;
}

final class SearchApiErr extends SearchApiResult<Never> {
  const SearchApiErr(this.error);
  final GatewayApiError error;
}

class SearchHit {
  const SearchHit({
    required this.messageId,
    required this.snippet,
    required this.score,
  });

  final String messageId;
  final String snippet;
  final double score;
}

class InChatSearchData {
  const InChatSearchData({required this.hits, this.nextCursor = ''});

  final List<SearchHit> hits;
  final String nextCursor;
}

class GlobalSearchData {
  const GlobalSearchData({
    required this.messages,
    required this.profileIds,
    required this.matchedChatIds,
    required this.spaceIds,
    this.nextCursor = '',
  });

  final List<SearchHit> messages;
  final List<String> profileIds;
  final List<String> matchedChatIds;
  final List<String> spaceIds;
  final String nextCursor;
}

class UserSearchData {
  const UserSearchData({required this.profileIds});
  final List<String> profileIds;
}

class SpaceSearchData {
  const SpaceSearchData({required this.spaceIds, this.nextCursor = ''});
  final List<String> spaceIds;
  final String nextCursor;
}

class SearchQueryDebouncer {
  SearchQueryDebouncer({
    required this.onQuery,
    this.delay = defaultDelay,
  });

  static const Duration defaultDelay = Duration(milliseconds: 300);

  final Duration delay;
  final void Function(String query) onQuery;
  Timer? _timer;

  void schedule(String query) {
    _timer?.cancel();
    _timer = Timer(delay, () => onQuery(query));
  }

  void dispose() {
    _timer?.cancel();
  }
}

class VoiceSearchClient {
  VoiceSearchClient({required GatewayHttpClient gateway}) : _gateway = gateway;

  final GatewayHttpClient _gateway;

  String? _normalizeQuery(String query) {
    final trimmed = query.trim();
    return trimmed.isEmpty ? null : trimmed;
  }

  List<SearchHit> _parseHits(List<dynamic>? raw) {
    if (raw == null) return const [];
    return [
      for (final item in raw)
        if (item is Map<String, dynamic>)
          SearchHit(
            messageId: item['message_id'] as String? ??
                item['messageId'] as String? ??
                '',
            snippet: item['snippet'] as String? ?? '',
            score: (item['score'] as num?)?.toDouble() ?? 0,
          ),
    ];
  }

  Future<SearchApiResult<InChatSearchData>> searchInChat({
    required String authorization,
    required String chatId,
    required String query,
    String cursor = '',
    int pageSize = 20,
  }) async {
    final q = _normalizeQuery(query);
    if (q == null) {
      return const SearchApiErr(
        GatewayApiError(
          errorCode: 'invalid_query',
          message: 'query required',
          statusCode: 400,
        ),
      );
    }
    final uri = _gateway.replace(
      path: '/api/v1/search/in-chat',
      queryParameters: {
        'chat_id': chatId,
        'q': q,
        if (cursor.isNotEmpty) 'cursor': cursor,
        'page_size': '$pageSize',
      },
    );
    final result = await _gateway.getJson(uri, authorization: authorization);
    return switch (result) {
      GatewayHttpOk(:final data) => SearchApiOk(_parseInChat(data)),
      GatewayHttpFailure(:final error) => SearchApiErr(error),
    };
  }

  InChatSearchData _parseInChat(Map<String, dynamic> data) {
    final results =
        data['search_results'] as Map<String, dynamic>? ??
        data['searchResults'] as Map<String, dynamic>? ??
        {};
    return InChatSearchData(
      hits: _parseHits(results['hits'] as List<dynamic>?),
      nextCursor: results['next_cursor'] as String? ??
          results['nextCursor'] as String? ??
          '',
    );
  }

  Future<SearchApiResult<GlobalSearchData>> searchGlobal({
    required String authorization,
    required String query,
    String cursor = '',
    int pageSize = 20,
  }) async {
    final q = _normalizeQuery(query);
    if (q == null) {
      return const SearchApiErr(
        GatewayApiError(
          errorCode: 'invalid_query',
          message: 'query required',
          statusCode: 400,
        ),
      );
    }
    final uri = _gateway.replace(
      path: '/api/v1/search/global',
      queryParameters: {
        'q': q,
        if (cursor.isNotEmpty) 'cursor': cursor,
        'page_size': '$pageSize',
      },
    );
    final result = await _gateway.getJson(uri, authorization: authorization);
    return switch (result) {
      GatewayHttpOk(:final data) => SearchApiOk(_parseGlobal(data)),
      GatewayHttpFailure(:final error) => SearchApiErr(error),
    };
  }

  GlobalSearchData _parseGlobal(Map<String, dynamic> data) {
    final results =
        data['global_search_results'] as Map<String, dynamic>? ??
        data['globalSearchResults'] as Map<String, dynamic>? ??
        {};
    final chats = results['matched_chats'] as List<dynamic>? ??
        results['matchedChats'] as List<dynamic>? ??
        [];
    return GlobalSearchData(
      messages: _parseHits(results['messages'] as List<dynamic>?),
      profileIds: [
        for (final id in (results['profile_ids'] as List<dynamic>? ??
            results['profileIds'] as List<dynamic>? ??
            []))
          '$id',
      ],
      matchedChatIds: [
        for (final chat in chats)
          if (chat is Map<String, dynamic>)
            chat['id'] as String? ?? chat['chatId'] as String? ?? '',
      ],
      spaceIds: [
        for (final id in (results['space_ids'] as List<dynamic>? ??
            results['spaceIds'] as List<dynamic>? ??
            []))
          '$id',
      ],
      nextCursor: results['next_cursor'] as String? ??
          results['nextCursor'] as String? ??
          '',
    );
  }

  Future<SearchApiResult<UserSearchData>> searchUsers({
    required String authorization,
    required String query,
    int limit = 20,
  }) async {
    final q = _normalizeQuery(query);
    if (q == null) {
      return const SearchApiErr(
        GatewayApiError(
          errorCode: 'invalid_query',
          message: 'query required',
          statusCode: 400,
        ),
      );
    }
    final uri = _gateway.replace(
      path: '/api/v1/search/users',
      queryParameters: {'q': q, 'limit': '$limit'},
    );
    final result = await _gateway.getJson(uri, authorization: authorization);
    return switch (result) {
      GatewayHttpOk(:final data) => SearchApiOk(_parseUsers(data)),
      GatewayHttpFailure(:final error) => SearchApiErr(error),
    };
  }

  UserSearchData _parseUsers(Map<String, dynamic> data) {
    final results =
        data['user_search_results'] as Map<String, dynamic>? ??
        data['userSearchResults'] as Map<String, dynamic>? ??
        {};
    return UserSearchData(
      profileIds: [
        for (final id in (results['profile_ids'] as List<dynamic>? ??
            results['profileIds'] as List<dynamic>? ??
            []))
          '$id',
      ],
    );
  }

  Future<SearchApiResult<SpaceSearchData>> searchSpaces({
    required String authorization,
    required String query,
    String cursor = '',
    int pageSize = 20,
  }) async {
    final q = _normalizeQuery(query);
    if (q == null) {
      return const SearchApiErr(
        GatewayApiError(
          errorCode: 'invalid_query',
          message: 'query required',
          statusCode: 400,
        ),
      );
    }
    final uri = _gateway.replace(
      path: '/api/v1/search/spaces',
      queryParameters: {
        'q': q,
        if (cursor.isNotEmpty) 'cursor': cursor,
        'page_size': '$pageSize',
      },
    );
    final result = await _gateway.getJson(uri, authorization: authorization);
    return switch (result) {
      GatewayHttpOk(:final data) => SearchApiOk(_parseSpaces(data)),
      GatewayHttpFailure(:final error) => SearchApiErr(error),
    };
  }

  SpaceSearchData _parseSpaces(Map<String, dynamic> data) {
    final results =
        data['space_search_results'] as Map<String, dynamic>? ??
        data['spaceSearchResults'] as Map<String, dynamic>? ??
        {};
    return SpaceSearchData(
      spaceIds: [
        for (final id in (results['space_ids'] as List<dynamic>? ??
            results['spaceIds'] as List<dynamic>? ??
            []))
          '$id',
      ],
      nextCursor: results['next_cursor'] as String? ??
          results['nextCursor'] as String? ??
          '',
    );
  }
}
