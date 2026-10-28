import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/matchmaking_client.dart';
import 'auth_providers.dart';
import 'matchmaking_providers.dart';

/// Recent distinct games come from actual MM history, never profile preferences.
final gameCatalogRecentProvider = FutureProvider.autoDispose<List<CatalogGame>>(
  (ref) async {
    final token = ref.watch(authControllerProvider).session?.accessToken;
    if (token == null || token.isEmpty) return const [];
    final client = ref.watch(voiceMatchmakingClientProvider);
    final history = await client.listMatchHistory(
      authorization: 'Bearer $token',
      pageSize: 20,
    );
    final matches = switch (history) {
      MatchmakingApiOk(:final data) => data.matches,
      MatchmakingApiFailure(:final message) => throw Exception(message),
    };
    final seen = <String>{};
    final games = <CatalogGame>[];
    for (final match in matches) {
      if (match.gameId.isEmpty || !seen.add(match.gameId)) continue;
      final result = await client.getGame(
        authorization: 'Bearer $token',
        gameId: match.gameId,
      );
      switch (result) {
        case MatchmakingApiOk(:final data):
          if (data.status == 'active') games.add(data);
        case MatchmakingApiFailure(:final statusCode, :final message):
          if (statusCode != 404) throw Exception(message);
      }
      if (games.length == 3) break;
    }
    return games;
  },
);
