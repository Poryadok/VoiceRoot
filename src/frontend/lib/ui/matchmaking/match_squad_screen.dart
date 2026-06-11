import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/matchmaking_client.dart';
import '../../l10n/app_localizations.dart';
import '../chat/chat_room_panel.dart';

/// Voice + text shell for an active match squad.
class MatchSquadScreen extends ConsumerWidget {
  const MatchSquadScreen({super.key, required this.match});

  final MatchData match;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final chatId = match.chatId;
    return Scaffold(
      appBar: AppBar(
        title: Text('${l10n.matchFoundTitle} · ${match.mode}'),
      ),
      body: chatId == null || chatId.isEmpty
          ? Center(child: Text(l10n.matchFoundRespondError))
          : ChatRoomPanel(chatId: chatId),
    );
  }
}
