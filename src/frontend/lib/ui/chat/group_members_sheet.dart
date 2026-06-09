import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/chats_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/social_providers.dart';
import '../api_error_messages.dart';
import '../core/voice_avatar.dart';
import '../core/voice_bottom_sheet.dart';
import '../core/voice_state_panel.dart';

/// Bottom sheet: group members with owner badge; owner can kick; members can leave.
class GroupMembersSheet extends ConsumerWidget {
  const GroupMembersSheet({super.key, required this.chatId, this.groupName});

  static const Key sheetKey = Key('group_members_sheet');
  static const Key leaveKey = Key('group_members_leave');
  static const Key ownerLeaveHintKey = Key('group_members_owner_leave_hint');

  static Key memberTileKey(String profileId) =>
      Key('group_member_tile_$profileId');

  static Key kickMemberKey(String profileId) =>
      Key('group_member_kick_$profileId');

  final String chatId;
  final String? groupName;

  static Future<void> show(
    BuildContext context, {
    required String chatId,
    String? groupName,
  }) {
    final container = ProviderScope.containerOf(context);
    return showVoiceBottomSheet<void>(
      context: context,
      scrollable: false,
      child: UncontrolledProviderScope(
        container: container,
        child: GroupMembersSheet(chatId: chatId, groupName: groupName),
      ),
    );
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    final membersAsync = ref.watch(groupMembersProvider(chatId));
    final activeId = ref.watch(authControllerProvider).activeProfileId;

    return SafeArea(
      child: Padding(
        key: sheetKey,
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              groupName ?? l10n.chatGroupMembersTitle,
              style: theme.textTheme.titleMedium,
            ),
            const SizedBox(height: 4),
            Text(
              l10n.chatGroupMembersSubtitle,
              style: theme.textTheme.bodySmall,
            ),
            const SizedBox(height: 12),
            Expanded(
              child: membersAsync.when(
                loading: () => const Center(child: CircularProgressIndicator()),
                error: (error, _) => VoiceStatePanel(
                  title: l10n.chatGroupMembersLoadError,
                  message: '$error',
                  icon: Icons.cloud_off_outlined,
                  actionLabel: l10n.commonRetry,
                  onAction: () => ref.invalidate(groupMembersProvider(chatId)),
                ),
                data: (data) {
                  final myRole = _myRole(data.members, activeId);
                  final isOwner = myRole == kChatRoleOwner;
                  return Column(
                    children: [
                      Expanded(
                        child: ListView.builder(
                          itemCount: data.members.length,
                          itemBuilder: (context, index) {
                            final member = data.members[index];
                            return _MemberTile(
                              key: memberTileKey(member.profileId),
                              member: member,
                              isSelf: member.profileId == activeId,
                              canKick: isOwner && !member.isOwner,
                              kickKey: kickMemberKey(member.profileId),
                              onKick: () => _confirmKick(
                                context,
                                ref,
                                member.profileId,
                              ),
                            );
                          },
                        ),
                      ),
                      const SizedBox(height: 8),
                      if (isOwner)
                        Text(
                          key: ownerLeaveHintKey,
                          l10n.chatGroupOwnerLeaveHint,
                          style: theme.textTheme.bodySmall,
                          textAlign: TextAlign.center,
                        )
                      else
                        OutlinedButton(
                          key: leaveKey,
                          onPressed: () => _confirmLeave(context, ref),
                          child: Text(l10n.chatGroupLeave),
                        ),
                    ],
                  );
                },
              ),
            ),
          ],
        ),
      ),
    );
  }

  String? _myRole(List<ChatMember> members, String? activeId) {
    if (activeId == null) return null;
    for (final member in members) {
      if (member.profileId == activeId) return member.role;
    }
    return null;
  }

  Future<void> _confirmKick(
    BuildContext context,
    WidgetRef ref,
    String profileId,
  ) async {
    final l10n = AppLocalizations.of(context)!;
    final profile = ref.read(profileProvider(profileId)).valueOrNull;
    final name =
        profile?.displayName ?? profile?.handle ?? profileId.substring(0, 8);
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) {
        final dialogL10n = AppLocalizations.of(ctx)!;
        return AlertDialog(
          title: Text(dialogL10n.chatGroupKickConfirmTitle),
          content: Text(dialogL10n.chatGroupKickConfirmMessage(name)),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(ctx).pop(false),
              child: Text(dialogL10n.commonCancel),
            ),
            FilledButton(
              onPressed: () => Navigator.of(ctx).pop(true),
              child: Text(dialogL10n.chatGroupKick),
            ),
          ],
        );
      },
    );
    if (confirmed != true || !context.mounted) return;
    final err = await ref.read(chatActionsProvider).removeGroupMember(
      chatId: chatId,
      profileId: profileId,
    );
    if (!context.mounted) return;
    if (err != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(socialActionErrorMessage(l10n, err))),
      );
      return;
    }
    ref.invalidate(groupMembersProvider(chatId));
  }

  Future<void> _confirmLeave(BuildContext context, WidgetRef ref) async {
    final l10n = AppLocalizations.of(context)!;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) {
        final dialogL10n = AppLocalizations.of(ctx)!;
        return AlertDialog(
          title: Text(dialogL10n.chatGroupLeaveConfirmTitle),
          content: Text(dialogL10n.chatGroupLeaveConfirmMessage),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(ctx).pop(false),
              child: Text(dialogL10n.commonCancel),
            ),
            FilledButton(
              onPressed: () => Navigator.of(ctx).pop(true),
              child: Text(dialogL10n.chatGroupLeave),
            ),
          ],
        );
      },
    );
    if (confirmed != true || !context.mounted) return;
    final err = await ref.read(chatActionsProvider).leaveGroup(chatId);
    if (!context.mounted) return;
    if (err != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(socialActionErrorMessage(l10n, err))),
      );
      return;
    }
    Navigator.of(context).pop();
  }
}

class _MemberTile extends ConsumerWidget {
  const _MemberTile({
    super.key,
    required this.member,
    required this.isSelf,
    required this.canKick,
    required this.kickKey,
    required this.onKick,
  });

  final ChatMember member;
  final bool isSelf;
  final bool canKick;
  final Key kickKey;
  final VoidCallback onKick;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final profileAsync = ref.watch(profileProvider(member.profileId));
    final profile = profileAsync.valueOrNull;
    final label =
        profile?.displayName ?? profile?.handle ?? member.profileId;

    return ListTile(
      leading: VoiceAvatar(
        imageUrl: profile?.avatarUrl,
        label: label,
        radius: 20,
      ),
      title: Text(isSelf ? l10n.chatGroupMemberYou(label) : label),
      subtitle: member.isOwner ? Text(l10n.chatGroupRoleOwner) : null,
      trailing: canKick
          ? IconButton(
              key: kickKey,
              tooltip: l10n.chatGroupKick,
              icon: const Icon(Icons.person_remove_outlined),
              onPressed: onKick,
            )
          : null,
    );
  }
}
