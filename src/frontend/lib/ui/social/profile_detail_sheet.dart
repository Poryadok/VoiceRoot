import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/users_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/social_providers.dart';
import 'presence_indicator.dart';

/// Bottom sheet with profile details, presence, and friend-request action.
class ProfileDetailSheet extends ConsumerWidget {
  const ProfileDetailSheet({super.key, required this.profileId});

  static const Key sheetKey = Key('profile_detail_sheet');
  static const Key onlineIndicatorKey = Key('profile_online_indicator');
  static const Key addFriendKey = Key('profile_add_friend');
  static const Key messageKey = Key('profile_message');

  final String profileId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final profileAsync = ref.watch(profileProvider(profileId));
    final presenceAsync = ref.watch(presenceProvider(profileId));
    final requestsAsync = ref.watch(friendRequestsProvider);
    final activeId = ref.watch(authControllerProvider).activeProfileId;
    final isSelf = activeId == profileId;

    final outgoing = requestsAsync.valueOrNull?.outgoing ?? const [];
    final incoming = requestsAsync.valueOrNull?.incoming ?? const [];
    final pendingOutgoing = outgoing.contains(profileId);
    final pendingIncoming = incoming.contains(profileId);

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
            final presence = presenceAsync.valueOrNull;
            return Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Row(
                  children: [
                    CircleAvatar(
                      radius: 28,
                      backgroundImage: profile.avatarUrl != null
                          ? NetworkImage(profile.avatarUrl!)
                          : null,
                      child: profile.avatarUrl == null
                          ? Text(
                              profile.displayName.isNotEmpty
                                  ? profile.displayName[0].toUpperCase()
                                  : '?',
                            )
                          : null,
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
                              Text(_presenceLabel(l10n, presence)),
                            ],
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
                if (!isSelf) ...[
                  const SizedBox(height: 20),
                  OutlinedButton(
                    key: ProfileDetailSheet.messageKey,
                    onPressed: () => _openDm(context, ref, profileId),
                    child: Text(l10n.profileMessage),
                  ),
                  const SizedBox(height: 8),
                  _FriendActionButton(
                    profileId: profileId,
                    pendingOutgoing: pendingOutgoing,
                    pendingIncoming: pendingIncoming,
                  ),
                ],
              ],
            );
          },
        ),
      ),
    );
  }

  Future<void> _openDm(
    BuildContext context,
    WidgetRef ref,
    String profileId,
  ) async {
    final l10n = AppLocalizations.of(context)!;
    final err = await ref.read(chatActionsProvider).openDmWithProfile(profileId);
    if (!context.mounted) return;
    if (err != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(l10n.socialActionError(err))),
      );
      return;
    }
    Navigator.of(context).pop();
  }

  String _presenceLabel(AppLocalizations l10n, VoicePresence? presence) {
    if (presence == null) return l10n.socialPresenceUnknown;
    return switch (presence.status) {
      'online' => l10n.socialPresenceOnline,
      'idle' => l10n.socialPresenceIdle,
      'dnd' => l10n.socialPresenceDnd,
      _ => l10n.socialPresenceOffline,
    };
  }
}

class _FriendActionButton extends ConsumerStatefulWidget {
  const _FriendActionButton({
    required this.profileId,
    required this.pendingOutgoing,
    required this.pendingIncoming,
  });

  final String profileId;
  final bool pendingOutgoing;
  final bool pendingIncoming;

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
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        FilledButton(
          key: ProfileDetailSheet.addFriendKey,
          onPressed: _busy ? null : _sendRequest,
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
