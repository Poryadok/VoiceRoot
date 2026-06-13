import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/matchmaking_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/call_providers.dart';
import '../../state/gateway_providers.dart';
import '../../state/matchmaking_providers.dart';
import '../../state/matchmaking_rating_controller.dart';
import '../call/active_call_panel.dart';
import '../chat/chat_room_panel.dart';

/// Voice + text shell for an active match squad.
class MatchSquadScreen extends ConsumerStatefulWidget {
  const MatchSquadScreen({super.key, required this.match});

  static const Key leaveButtonKey = Key('match_squad_leave');
  static const Key voiceSectionKey = Key('match_squad_voice_section');

  final MatchData match;

  @override
  ConsumerState<MatchSquadScreen> createState() => _MatchSquadScreenState();
}

class _MatchSquadScreenState extends ConsumerState<MatchSquadScreen> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _joinSquadVoice());
  }

  Future<void> _joinSquadVoice() async {
    final voiceRoomId = widget.match.voiceRoomId;
    if (voiceRoomId == null || voiceRoomId.isEmpty) return;
    if (!ref.read(gatewayConfigProvider).canPlaceVoiceCalls) return;
    await ref.read(callControllerProvider.notifier).joinGroupVoice(
          roomId: voiceRoomId,
        );
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final chatId = widget.match.chatId;
    final hasVoice = widget.match.voiceRoomId != null &&
        widget.match.voiceRoomId!.isNotEmpty;
    return Scaffold(
      appBar: AppBar(
        title: Text('${l10n.matchFoundTitle} · ${widget.match.mode}'),
        actions: [
          TextButton(
            key: MatchSquadScreen.leaveButtonKey,
            onPressed: () => _leaveSquad(context),
            child: Text(l10n.matchSquadLeave),
          ),
        ],
      ),
      body: chatId == null || chatId.isEmpty
          ? Center(child: Text(l10n.matchFoundRespondError))
          : Column(
              children: [
                if (hasVoice)
                  const KeyedSubtree(
                    key: MatchSquadScreen.voiceSectionKey,
                    child: ActiveCallPanel(),
                  ),
                Expanded(child: ChatRoomPanel(chatId: chatId)),
              ],
            ),
    );
  }

  Future<void> _leaveSquad(BuildContext context) async {
    final l10n = AppLocalizations.of(context)!;
    final token = ref.read(authControllerProvider).session?.accessToken;
    if (token == null || token.isEmpty) return;

    final call = ref.read(callControllerProvider);
    if (call.isActive && call.session != null) {
      await ref.read(callControllerProvider.notifier).hangUp();
    }

    final client = ref.read(voiceMatchmakingClientProvider);
    final result = await client.completeMatch(
      authorization: 'Bearer $token',
      matchId: widget.match.id,
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
