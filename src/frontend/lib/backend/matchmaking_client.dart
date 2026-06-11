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
