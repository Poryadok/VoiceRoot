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
