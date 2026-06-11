import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/matchmaking_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/matchmaking_providers.dart';
import '../../state/matchmaking_rating_controller.dart';
import '../chat/chat_room_panel.dart';

/// Voice + text shell for an active match squad.
class MatchSquadScreen extends ConsumerWidget {
  const MatchSquadScreen({super.key, required this.match});

  static const Key leaveButtonKey = Key('match_squad_leave');

  final MatchData match;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final chatId = match.chatId;
    return Scaffold(
      appBar: AppBar(
        title: Text('${l10n.matchFoundTitle} · ${match.mode}'),
        actions: [
          TextButton(
            key: leaveButtonKey,
            onPressed: () => _leaveSquad(context, ref),
            child: Text(l10n.matchSquadLeave),
          ),
        ],
      ),
      body: chatId == null || chatId.isEmpty
          ? Center(child: Text(l10n.matchFoundRespondError))
          : ChatRoomPanel(chatId: chatId),
    );
  }

  Future<void> _leaveSquad(BuildContext context, WidgetRef ref) async {
    final l10n = AppLocalizations.of(context)!;
    final token = ref.read(authControllerProvider).session?.accessToken;
    if (token == null || token.isEmpty) return;

    final client = ref.read(voiceMatchmakingClientProvider);
    final result = await client.completeMatch(
      authorization: 'Bearer $token',
      matchId: match.id,
    );

    if (!context.mounted) return;

    if (result is MatchmakingApiFailure) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(l10n.matchSquadLeaveError)),
      );
      return;
    }

    final completed = (result as MatchmakingApiOk<MatchData>).data;
    ref.read(matchmakingRatingControllerProvider.notifier).showRatingForMatch(
          completed,
        );
    Navigator.of(context).pop();
  }
}
