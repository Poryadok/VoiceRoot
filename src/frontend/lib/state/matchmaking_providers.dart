import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/matchmaking_client.dart';
import 'auth_providers.dart';

final voiceMatchmakingClientProvider = Provider<VoiceMatchmakingClient>((ref) {
  return VoiceMatchmakingClient(gateway: ref.watch(gatewayHttpClientProvider));
});

final gameCatalogProvider = FutureProvider.autoDispose<GameListData>((ref) async {
  final auth = ref.watch(authControllerProvider);
  final token = auth.session?.accessToken;
  if (token == null || token.isEmpty) {
    throw StateError('not authenticated');
  }
  final client = ref.watch(voiceMatchmakingClientProvider);
  final result = await client.listGames(authorization: 'Bearer $token');
  return switch (result) {
    MatchmakingApiOk(:final data) => data,
    MatchmakingApiFailure(:final message) => throw Exception(message),
  };
});

final gameCatalogSearchQueryProvider = StateProvider<String>((ref) => '');

final gameCatalogSearchProvider =
    FutureProvider.autoDispose<GameListData>((ref) async {
  final query = ref.watch(gameCatalogSearchQueryProvider);
  if (query.trim().isEmpty) {
    return ref.watch(gameCatalogProvider.future);
  }
  final auth = ref.watch(authControllerProvider);
  final token = auth.session?.accessToken;
  if (token == null || token.isEmpty) {
    throw StateError('not authenticated');
  }
  final client = ref.watch(voiceMatchmakingClientProvider);
  final result = await client.searchGames(
    authorization: 'Bearer $token',
    query: query.trim(),
  );
  return switch (result) {
    MatchmakingApiOk(:final data) => data,
    MatchmakingApiFailure(:final message) => throw Exception(message),
  };
});

final selectedCatalogGameIdProvider = StateProvider<String?>((ref) => null);

final myPlayerProfileProvider =
    FutureProvider.autoDispose<PlayerProfileData>((ref) async {
  final auth = ref.watch(authControllerProvider);
  final token = auth.session?.accessToken;
  if (token == null || token.isEmpty) {
    throw StateError('not authenticated');
  }
  final client = ref.watch(voiceMatchmakingClientProvider);
  final result = await client.getMyPlayerProfile(authorization: 'Bearer $token');
  return switch (result) {
    MatchmakingApiOk(:final data) => data,
    MatchmakingApiFailure(:final message) => throw Exception(message),
  };
});

final playerProfileProvider =
    FutureProvider.autoDispose.family<PlayerProfileData, String>((ref, profileId) async {
  final auth = ref.watch(authControllerProvider);
  final token = auth.session?.accessToken;
  if (token == null || token.isEmpty) {
    throw StateError('not authenticated');
  }
  final client = ref.watch(voiceMatchmakingClientProvider);
  final result = await client.getPlayerProfile(
    authorization: 'Bearer $token',
    profileId: profileId,
  );
  return switch (result) {
    MatchmakingApiOk(:final data) => data,
    MatchmakingApiFailure(:final message) => throw Exception(message),
  };
});

class PlayerProfileActions {
  PlayerProfileActions(this._ref);

  final Ref _ref;

  Future<void> upsertEntry({
    required String gameId,
    required String region,
    String? role,
    String? rank,
  }) async {
    final auth = _ref.read(authControllerProvider);
    final token = auth.session?.accessToken;
    if (token == null || token.isEmpty) {
      throw StateError('not authenticated');
    }
    final client = _ref.read(voiceMatchmakingClientProvider);
    final result = await client.upsertPlayerGameEntry(
      authorization: 'Bearer $token',
      gameId: gameId,
      region: region,
      role: role,
      rank: rank,
    );
    switch (result) {
      case MatchmakingApiOk():
        _ref.invalidate(myPlayerProfileProvider);
      case MatchmakingApiFailure(:final message):
        throw Exception(message);
    }
  }

  Future<void> deleteEntry(String gameId) async {
    final auth = _ref.read(authControllerProvider);
    final token = auth.session?.accessToken;
    if (token == null || token.isEmpty) {
      throw StateError('not authenticated');
    }
    final client = _ref.read(voiceMatchmakingClientProvider);
    final result = await client.deletePlayerGameEntry(
      authorization: 'Bearer $token',
      gameId: gameId,
    );
    switch (result) {
      case MatchmakingApiOk():
        _ref.invalidate(myPlayerProfileProvider);
      case MatchmakingApiFailure(:final message):
        throw Exception(message);
    }
  }
}

final playerProfileActionsProvider = Provider<PlayerProfileActions>((ref) {
  return PlayerProfileActions(ref);
});

/// Active MM search session for queue UI (solo v1).
final activeSearchSessionProvider = StateProvider<SearchSessionData?>((ref) => null);

/// Active match squad to navigate into after full accept.
final activeSquadMatchProvider = StateProvider<MatchData?>((ref) => null);

final selectedCatalogGameProvider =
    FutureProvider.autoDispose<CatalogGame?>((ref) async {
  final gameId = ref.watch(selectedCatalogGameIdProvider);
  if (gameId == null || gameId.isEmpty) return null;
  final auth = ref.watch(authControllerProvider);
  final token = auth.session?.accessToken;
  if (token == null || token.isEmpty) {
    throw StateError('not authenticated');
  }
  final client = ref.watch(voiceMatchmakingClientProvider);
  final result = await client.getGame(
    authorization: 'Bearer $token',
    gameId: gameId,
  );
  return switch (result) {
    MatchmakingApiOk(:final data) => data,
    MatchmakingApiFailure(:final message) => throw Exception(message),
  };
});
