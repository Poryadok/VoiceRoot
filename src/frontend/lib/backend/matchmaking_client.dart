import 'dart:convert';

import 'gateway_http.dart';

sealed class MatchmakingApiResult<T> {
  const MatchmakingApiResult();
}

final class MatchmakingApiOk<T> extends MatchmakingApiResult<T> {
  const MatchmakingApiOk(this.data);
  final T data;
}

final class MatchmakingApiFailure extends MatchmakingApiResult<Never> {
  const MatchmakingApiFailure({
    required this.message,
    this.statusCode,
  });

  final String message;
  final int? statusCode;
}

class GameRole {
  const GameRole({required this.name, required this.required});
  final String name;
  final bool required;
}

class GameRank {
  const GameRank({required this.name, required this.value});
  final String name;
  final int value;
}

class GameMode {
  const GameMode({
    required this.name,
    required this.slots,
    required this.partySizeMin,
    required this.partySizeMax,
    required this.rolesRequired,
    required this.rankRequired,
    required this.roles,
    required this.ranks,
  });

  final String name;
  final int slots;
  final int partySizeMin;
  final int partySizeMax;
  final bool rolesRequired;
  final bool rankRequired;
  final List<GameRole> roles;
  final List<GameRank> ranks;
}

class GameConfig {
  const GameConfig({
    this.genre,
    this.platforms = const [],
    required this.regions,
    required this.modes,
  });

  final String? genre;
  final List<String> platforms;
  final List<String> regions;
  final List<GameMode> modes;

  static GameConfig fromJson(Map<String, dynamic> json) {
    final modesRaw = json['modes'];
    final modes = <GameMode>[];
    if (modesRaw is List) {
      for (final item in modesRaw) {
        if (item is! Map<String, dynamic>) continue;
        modes.add(GameMode(
          name: item['name'] as String? ?? '',
          slots: (item['slots'] as num?)?.toInt() ?? 0,
          partySizeMin: (item['party_size_min'] as num?)?.toInt() ?? 1,
          partySizeMax: (item['party_size_max'] as num?)?.toInt() ?? 1,
          rolesRequired: item['roles_required'] as bool? ?? false,
          rankRequired: item['rank_required'] as bool? ?? false,
          roles: _parseRoles(item['roles']),
          ranks: _parseRanks(item['ranks']),
        ));
      }
    }
    return GameConfig(
      genre: json['genre'] as String?,
      platforms: _stringList(json['platforms']),
      regions: _stringList(json['regions']),
      modes: modes,
    );
  }

  static List<GameRole> _parseRoles(Object? raw) {
    if (raw is! List) return const [];
    return [
      for (final item in raw)
        if (item is Map<String, dynamic>)
          GameRole(
            name: item['name'] as String? ?? '',
            required: item['required'] as bool? ?? false,
          ),
    ];
  }

  static List<GameRank> _parseRanks(Object? raw) {
    if (raw is! List) return const [];
    return [
      for (final item in raw)
        if (item is Map<String, dynamic>)
          GameRank(
            name: item['name'] as String? ?? '',
            value: (item['value'] as num?)?.toInt() ?? 0,
          ),
    ];
  }

  static List<String> _stringList(Object? raw) {
    if (raw is! List) return const [];
    return [for (final v in raw) if (v is String) v];
  }
}

class CatalogGame {
  const CatalogGame({
    required this.id,
    required this.name,
    required this.status,
    required this.config,
    this.iconUrl,
  });

  final String id;
  final String name;
  final String status;
  final GameConfig config;
  final String? iconUrl;

  static CatalogGame fromGatewayJson(Map<String, dynamic> json) {
    final configRaw = json['configJson'] as String? ?? json['config_json'] as String? ?? '{}';
    final decoded = jsonDecode(configRaw);
    return CatalogGame(
      id: json['id'] as String? ?? '',
      name: json['name'] as String? ?? '',
      status: json['status'] as String? ?? 'active',
      iconUrl: json['iconUrl'] as String? ?? json['icon_url'] as String?,
      config: decoded is Map<String, dynamic>
          ? GameConfig.fromJson(decoded)
          : const GameConfig(regions: [], modes: []),
    );
  }
}

class GameListData {
  const GameListData({required this.games, this.nextCursor});

  final List<CatalogGame> games;
  final String? nextCursor;
}

class PlayerGameEntry {
  const PlayerGameEntry({
    required this.gameId,
    required this.region,
    this.role,
    this.rank,
    this.updatedAt,
  });

  final String gameId;
  final String region;
  final String? role;
  final String? rank;
  final DateTime? updatedAt;

  static PlayerGameEntry fromGatewayJson(Map<String, dynamic> json) {
    final updatedRaw = json['updatedAt'] as String? ?? json['updated_at'] as String?;
    return PlayerGameEntry(
      gameId: json['gameId'] as String? ?? json['game_id'] as String? ?? '',
      region: json['region'] as String? ?? '',
      role: json['role'] as String?,
      rank: json['rank'] as String?,
      updatedAt: updatedRaw != null ? DateTime.tryParse(updatedRaw) : null,
    );
  }
}

class PlayerProfileData {
  const PlayerProfileData({required this.entries});

  final List<PlayerGameEntry> entries;
}

class SearchSessionData {
  const SearchSessionData({
    required this.id,
    required this.profileId,
    required this.gameId,
    required this.mode,
    required this.criteriaJson,
    required this.status,
    this.matchId,
    this.timeoutAt,
    this.createdAt,
  });

  final String id;
  final String profileId;
  final String gameId;
  final String mode;
  final String criteriaJson;
  final String status;
  final String? matchId;
  final DateTime? timeoutAt;
  final DateTime? createdAt;

  static DateTime? _parseTimestamp(dynamic value) {
    if (value == null) return null;
    if (value is String && value.isNotEmpty) {
      return DateTime.tryParse(value);
    }
    if (value is Map<String, dynamic>) {
      final seconds = value['seconds'];
      if (seconds is String) {
        final sec = int.tryParse(seconds);
        if (sec != null) {
          return DateTime.fromMillisecondsSinceEpoch(sec * 1000, isUtc: true);
        }
      }
      if (seconds is num) {
        return DateTime.fromMillisecondsSinceEpoch(seconds.toInt() * 1000,
            isUtc: true);
      }
    }
    return null;
  }

  static SearchSessionData fromGatewayJson(Map<String, dynamic> json) {
    final session = json['searchSession'] as Map<String, dynamic>? ??
        json['search_session'] as Map<String, dynamic>? ??
        json;
    return SearchSessionData(
      id: session['id'] as String? ?? '',
      profileId: session['profileId'] as String? ?? session['profile_id'] as String? ?? '',
      gameId: session['gameId'] as String? ?? session['game_id'] as String? ?? '',
      mode: session['mode'] as String? ?? '',
      criteriaJson: session['criteriaJson'] as String? ?? session['criteria_json'] as String? ?? '',
      status: session['status'] as String? ?? '',
      matchId: session['matchId'] as String? ?? session['match_id'] as String?,
      timeoutAt: _parseTimestamp(session['timeoutAt'] ?? session['timeout_at']),
      createdAt: _parseTimestamp(session['createdAt'] ?? session['created_at']),
    );
  }
}

class PlayerRatingData {
  const PlayerRatingData({
    required this.profileId,
    required this.gameId,
    required this.ratingValue,
    required this.gamesPlayed,
  });

  final String profileId;
  final String gameId;
  final double ratingValue;
  final int gamesPlayed;

  static PlayerRatingData fromGatewayJson(Map<String, dynamic> json) {
    final rating = json['playerRating'] as Map<String, dynamic>? ??
        json['player_rating'] as Map<String, dynamic>? ??
        json;
    return PlayerRatingData(
      profileId: rating['profileId'] as String? ??
          rating['profile_id'] as String? ??
          '',
      gameId: rating['gameId'] as String? ?? rating['game_id'] as String? ?? '',
      ratingValue: (rating['ratingValue'] as num?)?.toDouble() ??
          (rating['rating_value'] as num?)?.toDouble() ??
          0,
      gamesPlayed: (rating['gamesPlayed'] as num?)?.toInt() ??
          (rating['games_played'] as num?)?.toInt() ??
          0,
    );
  }
}

class MatchData {
  const MatchData({
    required this.id,
    required this.gameId,
    required this.mode,
    required this.region,
    required this.status,
    required this.profileIds,
    this.voiceRoomId,
    this.chatId,
    this.gameName,
  });

  final String id;
  final String gameId;
  final String mode;
  final String region;
  final String status;
  final List<String> profileIds;
  final String? voiceRoomId;
  final String? chatId;
  final String? gameName;

  static MatchData fromGatewayJson(Map<String, dynamic> json) {
    final match = json['match'] as Map<String, dynamic>? ?? json;
    final profilesRaw = match['profileIds'] as List<dynamic>? ??
        match['profile_ids'] as List<dynamic>? ??
        const [];
    return MatchData(
      id: match['id'] as String? ?? '',
      gameId: match['gameId'] as String? ?? match['game_id'] as String? ?? '',
      mode: match['mode'] as String? ?? '',
      region: match['region'] as String? ?? '',
      status: match['status'] as String? ?? '',
      voiceRoomId: match['voiceRoomId'] as String? ?? match['voice_room_id'] as String?,
      chatId: match['chatId'] as String? ?? match['chat_id'] as String?,
      profileIds: [
        for (final p in profilesRaw)
          if (p is String) p,
      ],
    );
  }
}

class RespondToMatchData {
  const RespondToMatchData({
    required this.match,
    required this.searchSession,
  });

  final MatchData match;
  final SearchSessionData searchSession;

  static RespondToMatchData fromGatewayJson(Map<String, dynamic> json) {
    return RespondToMatchData(
      match: MatchData.fromGatewayJson(json),
      searchSession: SearchSessionData.fromGatewayJson(json),
    );
  }
}

/// Gateway client for /api/v1/matchmaking/games/** (Phase 7 catalog).
class VoiceMatchmakingClient {
  VoiceMatchmakingClient({required GatewayHttpClient gateway}) : _gateway = gateway;

  final GatewayHttpClient _gateway;

  Future<MatchmakingApiResult<GameListData>> listGames({
    required String authorization,
    String? cursor,
    int? pageSize,
  }) async {
    final params = <String, String>{};
    if (cursor != null && cursor.isNotEmpty) params['cursor'] = cursor;
    if (pageSize != null) params['page_size'] = '$pageSize';
    final uri = _gateway.replace(
      path: '/api/v1/matchmaking/games',
      queryParameters: params.isEmpty ? null : params,
    );
    final result = await _gateway.getJson(uri, authorization: authorization);
    return _mapList(result);
  }

  Future<MatchmakingApiResult<CatalogGame>> getGame({
    required String authorization,
    required String gameId,
  }) async {
    final result = await _gateway.getJson(
      _gateway.resolve('/api/v1/matchmaking/games/$gameId'),
      authorization: authorization,
    );
    return _map(result, (data) {
      final game = data['game'] as Map<String, dynamic>? ?? data;
      return CatalogGame.fromGatewayJson(game);
    });
  }

  Future<MatchmakingApiResult<GameListData>> searchGames({
    required String authorization,
    required String query,
  }) async {
    final uri = _gateway.replace(
      path: '/api/v1/matchmaking/games/search',
      queryParameters: {'query': query},
    );
    final result = await _gateway.getJson(uri, authorization: authorization);
    return _mapList(result);
  }

  Future<MatchmakingApiResult<PlayerProfileData>> getMyPlayerProfile({
    required String authorization,
  }) async {
    final result = await _gateway.getJson(
      _gateway.resolve('/api/v1/matchmaking/profile/me'),
      authorization: authorization,
    );
    return _mapProfile(result);
  }

  Future<MatchmakingApiResult<PlayerProfileData>> getPlayerProfile({
    required String authorization,
    required String profileId,
  }) async {
    final result = await _gateway.getJson(
      _gateway.resolve('/api/v1/matchmaking/profile/$profileId'),
      authorization: authorization,
    );
    return _mapProfile(result);
  }

  Future<MatchmakingApiResult<PlayerGameEntry>> upsertPlayerGameEntry({
    required String authorization,
    required String gameId,
    required String region,
    String? role,
    String? rank,
  }) async {
    final body = <String, dynamic>{
      'region': region,
      if (role != null && role.isNotEmpty) 'role': role,
      if (rank != null && rank.isNotEmpty) 'rank': rank,
    };
    final result = await _gateway.putJson(
      uri: _gateway.resolve('/api/v1/matchmaking/profile/games/$gameId'),
      authorization: authorization,
      body: body,
    );
    return _map(result, (data) {
      final entry = data['entry'] as Map<String, dynamic>? ?? data;
      return PlayerGameEntry.fromGatewayJson(entry);
    });
  }

  Future<MatchmakingApiResult<SearchSessionData>> startSearch({
    required String authorization,
    required String gameId,
    required String mode,
    required Map<String, dynamic> criteria,
  }) async {
    final result = await _gateway.postJson(
      uri: _gateway.resolve('/api/v1/matchmaking/search'),
      authorization: authorization,
      body: {
        'gameId': gameId,
        'mode': mode,
        'criteriaJson': jsonEncode(criteria),
      },
    );
    return _map(result, SearchSessionData.fromGatewayJson);
  }

  Future<MatchmakingApiResult<SearchSessionData>> getSearchStatus({
    required String authorization,
    required String sessionId,
  }) async {
    final result = await _gateway.getJson(
      _gateway.resolve('/api/v1/matchmaking/search/$sessionId'),
      authorization: authorization,
    );
    return _map(result, SearchSessionData.fromGatewayJson);
  }

  Future<MatchmakingApiResult<MatchData>> getMatch({
    required String authorization,
    required String matchId,
  }) async {
    final result = await _gateway.getJson(
      _gateway.resolve('/api/v1/matchmaking/matches/$matchId'),
      authorization: authorization,
    );
    return _map(result, MatchData.fromGatewayJson);
  }

  Future<MatchmakingApiResult<MatchData>> completeMatch({
    required String authorization,
    required String matchId,
  }) async {
    final result = await _gateway.postJson(
      uri: _gateway.resolve('/api/v1/matchmaking/matches/$matchId/complete'),
      authorization: authorization,
      body: const <String, dynamic>{},
    );
    return _map(result, MatchData.fromGatewayJson);
  }

  Future<MatchmakingApiResult<void>> rateMatch({
    required String authorization,
    required String matchId,
    required String ratedProfileId,
    required int stars,
  }) async {
    final result = await _gateway.postJson(
      uri: _gateway.resolve('/api/v1/matchmaking/matches/$matchId/rate'),
      authorization: authorization,
      body: {
        'ratedProfileId': ratedProfileId,
        'stars': stars,
      },
    );
    return switch (result) {
      GatewayHttpOk() => const MatchmakingApiOk(null),
      GatewayHttpFailure(:final error) => MatchmakingApiFailure(
        message: error.message,
        statusCode: error.statusCode,
      ),
    };
  }

  Future<MatchmakingApiResult<PlayerRatingData>> getPlayerRating({
    required String authorization,
    required String profileId,
    required String gameId,
  }) async {
    final result = await _gateway.getJson(
      _gateway.replace(
        path: '/api/v1/matchmaking/players/$profileId/rating',
        queryParameters: {'game_id': gameId},
      ),
      authorization: authorization,
    );
    return _map(result, PlayerRatingData.fromGatewayJson);
  }

  Future<MatchmakingApiResult<void>> banFromMM({
    required String authorization,
    required String targetProfileId,
    String? reason,
  }) async {
    final result = await _gateway.postJson(
      uri: _gateway.resolve('/api/v1/matchmaking/bans'),
      authorization: authorization,
      body: {
        'targetProfileId': targetProfileId,
        if (reason != null && reason.isNotEmpty) 'reason': reason,
      },
    );
    return switch (result) {
      GatewayHttpOk() => const MatchmakingApiOk(null),
      GatewayHttpFailure(:final error) => MatchmakingApiFailure(
        message: error.message,
        statusCode: error.statusCode,
      ),
    };
  }

  Future<MatchmakingApiResult<RespondToMatchData>> respondToMatch({
    required String authorization,
    required String matchId,
    required bool accept,
  }) async {
    final result = await _gateway.postJson(
      uri: _gateway.resolve('/api/v1/matchmaking/matches/$matchId/respond'),
      authorization: authorization,
      body: {'accept': accept},
    );
    return _map(result, RespondToMatchData.fromGatewayJson);
  }

  Future<MatchmakingApiResult<void>> cancelSearch({
    required String authorization,
    required String sessionId,
  }) async {
    final result = await _gateway.deleteEmpty(
      uri: _gateway.resolve('/api/v1/matchmaking/search/$sessionId'),
      authorization: authorization,
    );
    return switch (result) {
      GatewayHttpOk() => const MatchmakingApiOk(null),
      GatewayHttpFailure(:final error) => MatchmakingApiFailure(
        message: error.message,
        statusCode: error.statusCode,
      ),
    };
  }

  Future<MatchmakingApiResult<void>> deletePlayerGameEntry({
    required String authorization,
    required String gameId,
  }) async {
    final result = await _gateway.deleteEmpty(
      uri: _gateway.resolve('/api/v1/matchmaking/profile/games/$gameId'),
      authorization: authorization,
    );
    return switch (result) {
      GatewayHttpOk() => const MatchmakingApiOk(null),
      GatewayHttpFailure(:final error) => MatchmakingApiFailure(
        message: error.message,
        statusCode: error.statusCode,
      ),
    };
  }

  MatchmakingApiResult<PlayerProfileData> _mapProfile(
    GatewayHttpResult<Map<String, dynamic>> result,
  ) {
    return _map(result, (data) {
      final entriesRaw = data['entries'] as List<dynamic>? ?? const [];
      final entries = <PlayerGameEntry>[
        for (final item in entriesRaw)
          if (item is Map<String, dynamic>) PlayerGameEntry.fromGatewayJson(item),
      ];
      return PlayerProfileData(entries: entries);
    });
  }

  MatchmakingApiResult<GameListData> _mapList(GatewayHttpResult<Map<String, dynamic>> result) {
    return _map(result, (data) {
      final list = data['gameList'] as Map<String, dynamic>? ??
          data['game_list'] as Map<String, dynamic>? ??
          data;
      final gamesRaw = list['games'] as List<dynamic>? ?? const [];
      final games = <CatalogGame>[
        for (final item in gamesRaw)
          if (item is Map<String, dynamic>) CatalogGame.fromGatewayJson(item),
      ];
      return GameListData(
        games: games,
        nextCursor: list['nextCursor'] as String? ?? list['next_cursor'] as String?,
      );
    });
  }

  MatchmakingApiResult<T> _map<T>(
    GatewayHttpResult<Map<String, dynamic>> result,
    T Function(Map<String, dynamic> data) transform,
  ) {
    return switch (result) {
      GatewayHttpOk(:final data) => MatchmakingApiOk(transform(data)),
      GatewayHttpFailure(:final error) => MatchmakingApiFailure(
        message: error.message,
        statusCode: error.statusCode,
      ),
    };
  }
}
