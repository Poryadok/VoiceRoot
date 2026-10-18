import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:uuid/uuid.dart';

import '../../backend/matchmaking_client.dart';
import '../../backend/voice_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/call_providers.dart';
import '../../state/gateway_providers.dart';
import '../../state/matchmaking_providers.dart';
import '../../state/matchmaking_rating_controller.dart';
import '../../state/social_providers.dart';
import '../call/active_call_panel.dart';
import '../chat/chat_room_panel.dart';
import 'matchmaking_player_profile_sheet.dart';

/// Voice + text shell for an active match squad.
class MatchSquadScreen extends ConsumerStatefulWidget {
  const MatchSquadScreen({super.key, required this.match});

  static const Key leaveButtonKey = Key('match_squad_leave');
  static const Key voiceSectionKey = Key('match_squad_voice_section');
  static const Key joinVoiceButtonKey = Key('match_squad_join_voice');
  static const Key openChatButtonKey = Key('match_squad_open_chat');
  static Key playerProfileKey(String profileId) =>
      Key('match_squad_player_$profileId');

  final MatchData match;

  @override
  ConsumerState<MatchSquadScreen> createState() => _MatchSquadScreenState();
}

class _MatchSquadScreenState extends ConsumerState<MatchSquadScreen> {
  static const _uuid = Uuid();
  String? _completeOperationId;
  bool _joiningVoice = false;
  bool _leavingSquad = false;

  Future<void> _joinSquadVoice() async {
    final voiceRoomId = widget.match.voiceRoomId;
    if (voiceRoomId == null || voiceRoomId.isEmpty) return;
    if (!ref.read(gatewayConfigProvider).canPlaceVoiceCalls) return;
    if (_joiningVoice || _leavingSquad) return;
    setState(() => _joiningVoice = true);
    try {
      await ref
          .read(callControllerProvider.notifier)
          .joinGroupVoice(roomId: voiceRoomId, matchId: widget.match.id);
    } finally {
      if (mounted) setState(() => _joiningVoice = false);
    }
  }

  Future<void> _openMatchChat(BuildContext context) async {
    final chatId = widget.match.chatId;
    if (chatId == null || chatId.isEmpty) return;
    final l10n = AppLocalizations.of(context)!;
    await Navigator.of(context).push<void>(
      MaterialPageRoute<void>(
        builder: (_) => Scaffold(
          appBar: AppBar(title: Text(l10n.matchSquadOpenChat)),
          body: ChatRoomPanel(chatId: chatId),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final chatId = widget.match.chatId;
    final activeProfileId = ref.watch(authControllerProvider).activeProfileId;
    final canPlaceVoiceCalls = ref
        .watch(gatewayConfigProvider)
        .canPlaceVoiceCalls;
    final callState = ref.watch(callControllerProvider);
    final participantIds = widget.match.profileIds
        .where((id) => id.isNotEmpty)
        .toSet()
        .toList(growable: false);
    final hasVoice =
        widget.match.voiceRoomId != null &&
        widget.match.voiceRoomId!.isNotEmpty;
    final isJoinedToThisMatch = callState.session?.matchId == widget.match.id;
    return Scaffold(
      appBar: AppBar(title: Text(l10n.matchFoundTitle)),
      body: LayoutBuilder(
        builder: (context, constraints) => Center(
          child: SizedBox(
            width: constraints.maxWidth < 620 ? constraints.maxWidth : 620,
            height: constraints.maxHeight,
            child: ListView(
              padding: const EdgeInsets.fromLTRB(16, 16, 16, 24),
              children: [
                _MatchSquadHero(match: widget.match),
                const SizedBox(height: 20),
                Text(
                  l10n.matchSquadRosterTitle,
                  style: Theme.of(context).textTheme.titleMedium,
                ),
                const SizedBox(height: 8),
                Card(
                  clipBehavior: Clip.antiAlias,
                  child: Column(
                    children: [
                      for (
                        var index = 0;
                        index < participantIds.length;
                        index++
                      ) ...[
                        if (index > 0) const Divider(height: 1, indent: 72),
                        _MatchParticipantButton(
                          profileId: participantIds[index],
                          onPressed: participantIds[index] == activeProfileId
                              ? null
                              : () => showModalBottomSheet<void>(
                                  context: context,
                                  isScrollControlled: true,
                                  showDragHandle: true,
                                  builder: (_) => MatchmakingPlayerProfileSheet(
                                    profileId: participantIds[index],
                                    gameId: widget.match.gameId,
                                    canBanFromCurrentMatch:
                                        widget.match.status == 'active' &&
                                        widget.match.gameId.isNotEmpty,
                                  ),
                                ),
                        ),
                      ],
                    ],
                  ),
                ),
                if (hasVoice && isJoinedToThisMatch)
                  const Padding(
                    padding: EdgeInsets.only(top: 20),
                    child: KeyedSubtree(
                      key: MatchSquadScreen.voiceSectionKey,
                      child: ActiveCallPanel(),
                    ),
                  ),
                if (chatId == null || chatId.isEmpty)
                  Padding(
                    padding: const EdgeInsets.only(top: 20),
                    child: Center(child: Text(l10n.matchFoundRespondError)),
                  ),
              ],
            ),
          ),
        ),
      ),
      bottomNavigationBar: SafeArea(
        top: false,
        child: DecoratedBox(
          decoration: BoxDecoration(
            color: Theme.of(context).colorScheme.surface,
            border: Border(
              top: BorderSide(
                color: Theme.of(context).colorScheme.outlineVariant,
              ),
            ),
          ),
          child: Center(
            heightFactor: 1,
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 620),
              child: Padding(
                padding: const EdgeInsets.all(16),
                child: FocusTraversalGroup(
                  policy: OrderedTraversalPolicy(),
                  child: Wrap(
                    spacing: 10,
                    runSpacing: 10,
                    children: [
                      if (chatId != null && chatId.isNotEmpty)
                        FocusTraversalOrder(
                          order: const NumericFocusOrder(1),
                          child: _ActionFocusOutline(
                            child: OutlinedButton.icon(
                              key: MatchSquadScreen.openChatButtonKey,
                              onPressed: () => _openMatchChat(context),
                              icon: const Icon(Icons.chat_bubble_outline),
                              label: Text(l10n.matchSquadOpenChat),
                            ),
                          ),
                        ),
                      FocusTraversalOrder(
                        order: const NumericFocusOrder(2),
                        child: _ActionFocusOutline(
                          child: OutlinedButton.icon(
                            key: MatchSquadScreen.leaveButtonKey,
                            onPressed: _joiningVoice || _leavingSquad
                                ? null
                                : () => _leaveSquad(context),
                            icon: const Icon(Icons.logout),
                            label: Text(l10n.matchSquadLeave),
                            style: OutlinedButton.styleFrom(
                              foregroundColor: Theme.of(
                                context,
                              ).colorScheme.error,
                            ),
                          ),
                        ),
                      ),
                      if (hasVoice && !isJoinedToThisMatch)
                        FocusTraversalOrder(
                          order: const NumericFocusOrder(3),
                          child: _ActionFocusOutline(
                            child: FilledButton.icon(
                              key: MatchSquadScreen.joinVoiceButtonKey,
                              onPressed: canPlaceVoiceCalls && !_joiningVoice
                                  ? _joinSquadVoice
                                  : null,
                              icon: const Icon(Icons.headset_mic_outlined),
                              label: Text(l10n.callGroupVoiceJoin),
                            ),
                          ),
                        ),
                    ],
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }

  Future<void> _leaveSquad(BuildContext context) async {
    if (_joiningVoice || _leavingSquad) return;
    final l10n = AppLocalizations.of(context)!;
    final token = ref.read(authControllerProvider).session?.accessToken;
    if (token == null || token.isEmpty) return;

    setState(() => _leavingSquad = true);
    try {
      final voiceLeave = await ref
          .read(callControllerProvider.notifier)
          .leaveMatchSquad(matchId: widget.match.id);
      if (voiceLeave case VoiceApiFailure()) {
        if (!context.mounted) return;
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(l10n.matchSquadLeaveError)));
        return;
      }

      final client = ref.read(voiceMatchmakingClientProvider);
      final result = await client.completeMatch(
        authorization: 'Bearer $token',
        matchId: widget.match.id,
        operationId: _completeOperationId ??= _uuid.v4(),
      );

      if (!context.mounted) return;

      if (result is MatchmakingApiFailure) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(l10n.matchSquadLeaveError)));
        return;
      }

      final completed = (result as MatchmakingApiOk<MatchData>).data;
      _completeOperationId = null;
      ref
          .read(matchmakingRatingControllerProvider.notifier)
          .showRatingForMatch(completed);
      Navigator.of(context).pop();
    } finally {
      if (mounted) setState(() => _leavingSquad = false);
    }
  }
}

class _ActionFocusOutline extends StatelessWidget {
  const _ActionFocusOutline({required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Focus(
      skipTraversal: true,
      canRequestFocus: false,
      child: Builder(
        builder: (context) {
          final hasKeyboardFocus =
              Focus.of(context).hasFocus &&
              FocusManager.instance.highlightMode ==
                  FocusHighlightMode.traditional;
          final colors = Theme.of(context).colorScheme;
          return Stack(
            clipBehavior: Clip.none,
            children: [
              child,
              if (hasKeyboardFocus)
                Positioned.fill(
                  left: -3,
                  top: -3,
                  right: -3,
                  bottom: -3,
                  child: IgnorePointer(
                    child: DecoratedBox(
                      decoration: BoxDecoration(
                        border: Border.all(color: colors.primary, width: 2),
                        borderRadius: BorderRadius.circular(16),
                      ),
                    ),
                  ),
                ),
            ],
          );
        },
      ),
    );
  }
}

class _MatchSquadHero extends StatelessWidget {
  const _MatchSquadHero({required this.match});

  final MatchData match;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final gameName = match.gameName?.trim().isNotEmpty == true
        ? match.gameName!.trim()
        : match.gameId;
    return LayoutBuilder(
      builder: (context, constraints) {
        final height = constraints.maxWidth < 480 ? 210.0 : 170.0;
        return Container(
          height: height,
          padding: const EdgeInsets.all(20),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(16),
            gradient: LinearGradient(
              begin: Alignment.topLeft,
              end: Alignment.bottomRight,
              colors: [colors.primary.withValues(alpha: 0.30), colors.surface],
            ),
            border: Border.all(color: colors.outlineVariant),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              DecoratedBox(
                decoration: BoxDecoration(
                  color: colors.surface.withValues(alpha: 0.78),
                  borderRadius: BorderRadius.circular(999),
                ),
                child: Padding(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 10,
                    vertical: 5,
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(Icons.circle, size: 8, color: colors.primary),
                      const SizedBox(width: 6),
                      Text(
                        match.status == 'active'
                            ? AppLocalizations.of(context)!.matchFoundTitle
                            : match.status,
                        style: Theme.of(context).textTheme.labelMedium,
                      ),
                    ],
                  ),
                ),
              ),
              const Spacer(),
              Text(
                AppLocalizations.of(
                  context,
                )!.matchFoundSubtitle(gameName, match.mode),
                style: Theme.of(context).textTheme.headlineSmall,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
              ),
              if (match.region.isNotEmpty) ...[
                const SizedBox(height: 8),
                Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Icon(
                      Icons.location_on_outlined,
                      size: 16,
                      color: colors.onSurfaceVariant,
                    ),
                    const SizedBox(width: 4),
                    Text(
                      match.region,
                      style: Theme.of(context).textTheme.bodyMedium,
                    ),
                  ],
                ),
              ],
            ],
          ),
        );
      },
    );
  }
}

class _MatchParticipantButton extends ConsumerWidget {
  const _MatchParticipantButton({
    required this.profileId,
    required this.onPressed,
  });

  final String profileId;
  final VoidCallback? onPressed;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final profile = ref.watch(profileProvider(profileId));
    final label = profile.when(
      skipLoadingOnRefresh: false,
      loading: () => profileId,
      error: (error, stackTrace) => profileId,
      data: (value) => value?.displayName.trim().isNotEmpty == true
          ? value!.displayName
          : profileId,
    );
    return ListTile(
      key: MatchSquadScreen.playerProfileKey(profileId),
      onTap: onPressed,
      leading: CircleAvatar(
        backgroundColor: Theme.of(context).colorScheme.primaryContainer,
        foregroundColor: Theme.of(context).colorScheme.onPrimaryContainer,
        child: Text(label.isEmpty ? '?' : label.substring(0, 1).toUpperCase()),
      ),
      title: Text(label),
      trailing: onPressed == null ? null : const Icon(Icons.chevron_right),
    );
  }
}
