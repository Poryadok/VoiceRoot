import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/matchmaking_client.dart';
import '../../backend/users_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/presence_providers.dart';
import '../../state/matchmaking_providers.dart';
import '../../state/social_providers.dart';
import '../core/voice_avatar.dart';
import '../report/report_sheet.dart';
import 'presence_indicator.dart';

String _mmEntryLabel(AsyncValue<GameListData> catalogAsync, PlayerGameEntry entry) {
  final games = catalogAsync.valueOrNull?.games ?? const [];
  var name = entry.gameId;
  for (final g in games) {
    if (g.id == entry.gameId) {
      name = g.name;
      break;
    }
  }
  final parts = <String>[name, entry.region];
  if (entry.role != null && entry.role!.isNotEmpty) parts.add(entry.role!);
  if (entry.rank != null && entry.rank!.isNotEmpty) parts.add(entry.rank!);
  return parts.join(' · ');
}

/// Bottom sheet with profile details, presence, and friend-request action.
class ProfileDetailSheet extends ConsumerWidget {
  const ProfileDetailSheet({super.key, required this.profileId});

  static const Key sheetKey = Key('profile_detail_sheet');
  static const Key onlineIndicatorKey = Key('profile_online_indicator');
  static const Key addFriendKey = Key('profile_add_friend');
  static const Key removeFriendKey = Key('profile_remove_friend');
  static const Key messageKey = Key('profile_message');
  static const Key blockKey = Key('profile_block');

  final String profileId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final profileAsync = ref.watch(profileProvider(profileId));
    final mmProfileAsync = ref.watch(playerProfileProvider(profileId));
    final catalogAsync = ref.watch(gameCatalogProvider);
    final firstGameId = mmProfileAsync.valueOrNull?.entries.firstOrNull?.gameId;
    final mmRatingAsync = firstGameId == null
        ? const AsyncValue<PlayerRatingData?>.data(null)
        : ref.watch(
            playerRatingProvider(
              (profileId: profileId, gameId: firstGameId),
            ),
          );
    final presence = ref.watch(presenceProvider(profileId));
    final requestsAsync = ref.watch(friendRequestsProvider);
    final activeId = ref.watch(authControllerProvider).activeProfileId;
    final isGuest = ref.watch(authControllerProvider).isGuest;
    final isSelf = activeId == profileId;

    final outgoing = requestsAsync.valueOrNull?.outgoing ?? const [];
    final incoming = requestsAsync.valueOrNull?.incoming ?? const [];
    final pendingOutgoing = outgoing.contains(profileId);
    final pendingIncoming = incoming.contains(profileId);
    final isFriend = ref.watch(isFriendProvider(profileId));

    return SafeArea(
      child: Padding(
        key: sheetKey,
        padding: const EdgeInsets.fromLTRB(24, 16, 24, 24),
        child: profileAsync.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (e, st) => Text(l10n.socialProfileLoadError),
          data: (profile) {
            if (profile == null) {
              return Text(l10n.socialProfileLoadError);
            }
            return Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Row(
                  children: [
                    VoiceAvatar(
                      imageUrl: profile.avatarUrl,
                      label: profile.displayName,
                      radius: 28,
                    ),
                    const SizedBox(width: 16),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            profile.displayName,
                            style: Theme.of(context).textTheme.titleLarge,
                          ),
                          Text(profile.handle),
                          const SizedBox(height: 4),
                          Row(
                            children: [
                              PresenceIndicator(
                                key: onlineIndicatorKey,
                                presence: presence,
                              ),
                              const SizedBox(width: 8),
                              Text(_presenceLabel(context, l10n, presence)),
                            ],
                          ),
                          mmRatingAsync.when(
                            loading: () => const SizedBox.shrink(),
                            error: (error, stackTrace) => const SizedBox.shrink(),
                            data: (rating) {
                              if (rating == null) return const SizedBox.shrink();
                              return Padding(
                                padding: const EdgeInsets.only(top: 4),
                                child: Text(
                                  l10n.profileMmRating(
                                    rating.ratingValue.toStringAsFixed(1),
                                  ),
                                  style: Theme.of(context).textTheme.bodySmall,
                                ),
                              );
                            },
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
                if (profile.bio != null && profile.bio!.isNotEmpty) ...[
                  const SizedBox(height: 16),
                  Text(profile.bio!),
                ],
                mmProfileAsync.when(
                  loading: () => const SizedBox.shrink(),
                  error: (error, stackTrace) => const SizedBox.shrink(),
                  data: (mmProfile) {
                    if (mmProfile.entries.isEmpty) return const SizedBox.shrink();
                    return Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        const SizedBox(height: 16),
                        Text(
                          l10n.playerProfileSection,
                          style: Theme.of(context).textTheme.titleSmall,
                        ),
                        const SizedBox(height: 8),
                        for (final entry in mmProfile.entries)
                          Text(
                            _mmEntryLabel(catalogAsync, entry),
                            style: Theme.of(context).textTheme.bodyMedium,
                          ),
                      ],
                    );
                  },
                ),
                if (!isSelf) ...[
                  const SizedBox(height: 20),
                  OutlinedButton(
                    key: ProfileDetailSheet.messageKey,
                    onPressed: isGuest ? null : () => _openDm(context, ref, profileId),
                    child: Text(l10n.profileMessage),
                  ),
                  const SizedBox(height: 8),
                  _FriendActionButton(
                    profileId: profileId,
                    pendingOutgoing: pendingOutgoing,
                    pendingIncoming: pendingIncoming,
                    isFriend: isFriend,
                    isGuest: isGuest,
                  ),
                  const SizedBox(height: 8),
                  TextButton(
                    key: ProfileDetailSheet.blockKey,
                    onPressed: () => _confirmBlock(context, ref, profile),
                    style: TextButton.styleFrom(
                      foregroundColor: Theme.of(context).colorScheme.error,
                    ),
                    child: Text(l10n.profileBlock),
                  ),
                  const SizedBox(height: 8),
                  TextButton(
                    key: const Key('profile_report'),
                    onPressed: () {
                      Navigator.of(context).pop();
                      ReportSheet.show(
                        context,
                        target: ReportUserTarget(profileId: profileId),
                      );
                    },
                    child: Text(l10n.reportAction),
                  ),
                ],
              ],
            );
          },
        ),
      ),
    );
  }

  Future<void> _confirmBlock(
    BuildContext context,
    WidgetRef ref,
    VoiceProfile profile,
  ) async {
    final l10n = AppLocalizations.of(context)!;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) {
        final dialogL10n = AppLocalizations.of(ctx)!;
        return AlertDialog(
          title: Text(dialogL10n.profileBlockConfirmTitle),
          content: Text(dialogL10n.profileBlockConfirmMessage),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(ctx).pop(false),
              child: Text(dialogL10n.commonCancel),
            ),
            FilledButton(
              onPressed: () => Navigator.of(ctx).pop(true),
              child: Text(dialogL10n.profileBlock),
            ),
          ],
        );
      },
    );
    if (confirmed != true || !context.mounted) return;
    final err = await ref
        .read(socialActionsProvider)
        .blockAccount(profile.accountId);
    if (!context.mounted) return;
    if (err != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(l10n.socialActionError(err))),
      );
      return;
    }
    Navigator.of(context).pop();
  }

  Future<void> _openDm(
    BuildContext context,
    WidgetRef ref,
    String profileId,
  ) async {
    final l10n = AppLocalizations.of(context)!;
    final err = await ref
        .read(chatActionsProvider)
        .openDmWithProfile(profileId);
    if (!context.mounted) return;
    if (err != null) {
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(l10n.socialActionError(err))));
      return;
    }
    Navigator.of(context).pop();
  }

  String _presenceLabel(
    BuildContext context,
    AppLocalizations l10n,
    VoicePresence? presence,
  ) {
    if (presence == null) return l10n.socialPresenceUnknown;
    return switch (presence.status) {
      'online' => l10n.socialPresenceOnline,
      'idle' => l10n.socialPresenceIdle,
      'dnd' => l10n.socialPresenceDnd,
      _ =>
        presence.lastSeen == null
            ? l10n.socialPresenceOffline
            : l10n.socialPresenceLastSeen(
                _formatLastSeen(context, presence.lastSeen!),
              ),
    };
  }

  String _formatLastSeen(BuildContext context, DateTime lastSeen) {
    final local = lastSeen.toLocal();
    final material = MaterialLocalizations.of(context);
    final date = material.formatShortDate(local);
    final time = TimeOfDay.fromDateTime(local).format(context);
    return '$date $time';
  }
}

class _FriendActionButton extends ConsumerStatefulWidget {
  const _FriendActionButton({
    required this.profileId,
    required this.pendingOutgoing,
    required this.pendingIncoming,
    required this.isFriend,
    required this.isGuest,
  });

  final String profileId;
  final bool pendingOutgoing;
  final bool pendingIncoming;
  final bool isFriend;
  final bool isGuest;

  @override
  ConsumerState<_FriendActionButton> createState() =>
      _FriendActionButtonState();
}

class _FriendActionButtonState extends ConsumerState<_FriendActionButton> {
  var _busy = false;
  String? _error;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    if (widget.pendingIncoming) {
      return Row(
        children: [
          Expanded(
            child: FilledButton(
              onPressed: _busy ? null : () => _accept(l10n),
              child: Text(l10n.socialAcceptRequest),
            ),
          ),
          const SizedBox(width: 8),
          Expanded(
            child: OutlinedButton(
              onPressed: _busy ? null : () => _decline(l10n),
              child: Text(l10n.socialDeclineRequest),
            ),
          ),
        ],
      );
    }
    if (widget.pendingOutgoing) {
      return Text(l10n.socialRequestPending);
    }
    if (widget.isFriend) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          OutlinedButton(
            key: ProfileDetailSheet.removeFriendKey,
            onPressed: _busy ? null : _removeFriend,
            child: Text(l10n.socialRemoveFriend),
          ),
          if (_error != null) ...[
            const SizedBox(height: 8),
            Text(
              l10n.socialActionError(_error!),
              style: TextStyle(color: Theme.of(context).colorScheme.error),
            ),
          ],
        ],
      );
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        FilledButton(
          key: ProfileDetailSheet.addFriendKey,
          onPressed: widget.isGuest || _busy ? null : _sendRequest,
          child: Text(l10n.socialAddFriend),
        ),
        if (_error != null) ...[
          const SizedBox(height: 8),
          Text(
            l10n.socialActionError(_error!),
            style: TextStyle(color: Theme.of(context).colorScheme.error),
          ),
        ],
      ],
    );
  }

  Future<void> _sendRequest() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    final err = await ref
        .read(socialActionsProvider)
        .sendFriendInvitation(widget.profileId);
    if (!mounted) return;
    setState(() {
      _busy = false;
      _error = err;
    });
    if (err == null) Navigator.of(context).pop();
  }

  Future<void> _removeFriend() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    final err = await ref
        .read(socialActionsProvider)
        .removeFriend(widget.profileId);
    if (!mounted) return;
    setState(() {
      _busy = false;
      _error = err;
    });
    if (err == null) Navigator.of(context).pop();
  }

  Future<void> _accept(AppLocalizations l10n) async {
    setState(() {
      _busy = true;
      _error = null;
    });
    final err = await ref
        .read(socialActionsProvider)
        .acceptFriendInvitation(widget.profileId);
    if (!mounted) return;
    setState(() => _busy = false);
    if (err != null) {
      setState(() => _error = err);
    } else {
      Navigator.of(context).pop();
    }
  }

  Future<void> _decline(AppLocalizations l10n) async {
    setState(() {
      _busy = true;
      _error = null;
    });
    final err = await ref
        .read(socialActionsProvider)
        .declineFriendInvitation(widget.profileId);
    if (!mounted) return;
    setState(() => _busy = false);
    if (err != null) {
      setState(() => _error = err);
    } else {
      Navigator.of(context).pop();
    }
  }
}
