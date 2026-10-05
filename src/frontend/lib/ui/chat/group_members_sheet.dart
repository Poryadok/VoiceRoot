import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/chats_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/group_members_management_providers.dart';
import '../../state/social_providers.dart';
import '../api_error_messages.dart';
import '../core/voice_avatar.dart';
import '../core/voice_bottom_sheet.dart';
import '../core/voice_skeleton.dart';
import '../core/voice_state_panel.dart';
import '../social/profile_detail_sheet.dart';
import 'group_member_picker_sheet.dart';

/// Group members list (embedded in side panel or bottom sheet).
class GroupMembersContent extends ConsumerWidget {
  const GroupMembersContent({
    super.key,
    required this.chatId,
    this.groupName,
    this.showHeader = true,
  });

  final String chatId;
  final String? groupName;
  final bool showHeader;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    final management = ref.watch(groupMembersManagementProvider(chatId));
    final activeId = ref.watch(authControllerProvider).activeProfileId;
    final role = _myRole(management.members, activeId);
    final canAdd = management.members.length < kGroupMembersPageSize;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        if (showHeader) ...[
          Text(
            groupName ?? l10n.chatGroupMembersTitle,
            style: theme.textTheme.titleMedium,
          ),
          const SizedBox(height: 4),
          Text(l10n.chatGroupMembersSubtitle, style: theme.textTheme.bodySmall),
          const SizedBox(height: 12),
        ],
        Expanded(
          child: management.isLoading && management.members.isEmpty
              ? const VoiceListSkeleton()
              : management.error != null && management.members.isEmpty
              ? SingleChildScrollView(
                  child: VoiceStatePanel(
                    title: l10n.chatGroupMembersLoadError,
                    message: groupMembersErrorMessage(l10n, management.error!),
                    icon: Icons.cloud_off_outlined,
                    actionLabel: l10n.commonRetry,
                    onAction: () => ref
                        .read(groupMembersManagementProvider(chatId).notifier)
                        .load(),
                  ),
                )
              : Column(
                  children: [
                    if (management.error != null)
                      MaterialBanner(
                        content: Text(
                          groupMembersErrorMessage(l10n, management.error!),
                        ),
                        actions: [
                          TextButton(
                            onPressed: () => ref
                                .read(
                                  groupMembersManagementProvider(
                                    chatId,
                                  ).notifier,
                                )
                                .load(),
                            child: Text(l10n.commonRetry),
                          ),
                        ],
                      ),
                    Expanded(
                      child: ListView.builder(
                        itemCount: management.members.length,
                        itemBuilder: (context, index) {
                          final member = management.members[index];
                          final isSelf = member.profileId == activeId;
                          final canKick =
                              !isSelf &&
                              (role == kChatRoleOwner ||
                                  (role == kChatRoleAdmin &&
                                      member.role == kChatRoleMember));
                          return _MemberTile(
                            key: GroupMembersSheet.memberTileKey(
                              member.profileId,
                            ),
                            member: member,
                            isSelf: isSelf,
                            canKick: canKick,
                            kickKey: GroupMembersSheet.kickMemberKey(
                              member.profileId,
                            ),
                            onTap: () =>
                                _openProfile(context, ref, member.profileId),
                            onKick: () => _confirmKick(
                              context,
                              ref,
                              chatId,
                              member.profileId,
                            ),
                          );
                        },
                      ),
                    ),
                    if (role == kChatRoleOwner)
                      for (final member in management.members)
                        if (!member.isOwner && member.profileId != activeId)
                          _TransferOwnershipButton(
                            key: GroupMembersSheet.transferOwnerKey(
                              member.profileId,
                            ),
                            chatId: chatId,
                            profileId: member.profileId,
                          ),
                    Row(
                      children: [
                        if (management.members.isNotEmpty)
                          IconButton(
                            key: GroupMembersSheet.addMembersKey,
                            tooltip: canAdd
                                ? l10n.chatGroupAddMembers
                                : l10n.chatGroupMembersLimitReached,
                            onPressed: canAdd && !management.isMutating
                                ? () => _openPicker(
                                    context,
                                    ref,
                                    chatId,
                                    management.members,
                                  )
                                : null,
                            icon: const Icon(Icons.person_add_alt_1_outlined),
                          ),
                        Expanded(
                          child: OutlinedButton(
                            key: GroupMembersSheet.leaveKey,
                            onPressed: () =>
                                _confirmLeave(context, ref, chatId),
                            child: Text(l10n.chatGroupLeave),
                          ),
                        ),
                      ],
                    ),
                  ],
                ),
        ),
      ],
    );
  }
}

Future<void> _openPicker(
  BuildContext context,
  WidgetRef ref,
  String chatId,
  List<ChatMember> members,
) async {
  final controller = ref.read(groupMembersManagementProvider(chatId).notifier);
  final expectedContext = controller.captureContext();
  if (expectedContext == null) return;
  final selected = await GroupMemberPickerSheet.show(
    context,
    existingProfileIds: members.map((member) => member.profileId).toSet(),
    remainingCapacity: kGroupMembersPageSize - members.length,
  );
  if (selected == null || selected.isEmpty || !context.mounted) return;
  final err = await controller.addMembers(
    selected,
    expectedContext: expectedContext,
  );
  if (!context.mounted ||
      err == kGroupMembersStaleContext ||
      !controller.isCurrentContext(expectedContext)) {
    return;
  }
  if (err != null) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(commonActionErrorMessage(AppLocalizations.of(context)!)),
      ),
    );
  }
}

class GroupMembersSheet extends ConsumerWidget {
  const GroupMembersSheet({super.key, required this.chatId, this.groupName});

  static const Key sheetKey = Key('group_members_sheet');
  static const Key leaveKey = Key('group_members_leave');
  static const Key addMembersKey = Key('group_members_add');

  static Key transferOwnerKey(String profileId) =>
      Key('group_members_transfer_owner_$profileId');
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
  Widget build(BuildContext context, WidgetRef ref) => SafeArea(
    child: Padding(
      key: sheetKey,
      padding: const EdgeInsets.fromLTRB(16, 12, 16, 16),
      child: GroupMembersContent(chatId: chatId, groupName: groupName),
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

void _openProfile(BuildContext context, WidgetRef ref, String profileId) {
  final container = ProviderScope.containerOf(context);
  showModalBottomSheet<void>(
    context: context,
    isScrollControlled: true,
    builder: (sheetContext) => UncontrolledProviderScope(
      container: container,
      child: ProfileDetailSheet(profileId: profileId),
    ),
  );
}

Future<void> _confirmKick(
  BuildContext context,
  WidgetRef ref,
  String chatId,
  String profileId,
) async {
  final l10n = AppLocalizations.of(context)!;
  final controller = ref.read(groupMembersManagementProvider(chatId).notifier);
  final expectedContext = controller.captureContext();
  if (expectedContext == null) return;
  final profile = ref.read(profileProvider(profileId)).valueOrNull;
  final name = profile?.displayName ?? profile?.handle ?? profileId;
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
  final err = await controller.removeMember(
    profileId,
    expectedContext: expectedContext,
  );
  if (!context.mounted ||
      err == kGroupMembersStaleContext ||
      !controller.isCurrentContext(expectedContext)) {
    return;
  }
  if (err != null) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(socialActionErrorMessage(l10n, err))),
    );
  }
}

Future<void> _confirmLeave(
  BuildContext context,
  WidgetRef ref,
  String chatId,
) async {
  final l10n = AppLocalizations.of(context)!;
  final controller = ref.read(groupMembersManagementProvider(chatId).notifier);
  final expectedContext = controller.captureContext();
  if (expectedContext == null) return;
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
  final err = await controller.leaveGroup(expectedContext: expectedContext);
  if (!context.mounted ||
      err == kGroupMembersStaleContext ||
      !controller.isCurrentContext(expectedContext)) {
    return;
  }
  if (err != null) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(socialActionErrorMessage(l10n, err))),
    );
  } else if (Navigator.of(context).canPop()) {
    Navigator.of(context).pop();
  }
}

Future<void> _confirmTransferOwnership(
  BuildContext context,
  WidgetRef ref,
  String chatId,
  String profileId,
  String name,
) async {
  final l10n = AppLocalizations.of(context)!;
  final controller = ref.read(groupMembersManagementProvider(chatId).notifier);
  final expectedContext = controller.captureContext();
  if (expectedContext == null) return;
  final confirmed = await showDialog<bool>(
    context: context,
    builder: (ctx) {
      final dialogL10n = AppLocalizations.of(ctx)!;
      return AlertDialog(
        title: Text(dialogL10n.chatGroupTransferOwnershipTitle),
        content: Text(dialogL10n.chatGroupTransferOwnershipMessage(name)),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(dialogL10n.commonCancel),
          ),
          FilledButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: Text(dialogL10n.chatGroupTransferOwnershipConfirm),
          ),
        ],
      );
    },
  );
  if (confirmed != true || !context.mounted) return;
  final err = await controller.transferOwnership(
    profileId,
    expectedContext: expectedContext,
  );
  if (!context.mounted ||
      err == kGroupMembersStaleContext ||
      !controller.isCurrentContext(expectedContext)) {
    return;
  }
  if (err != null) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(socialActionErrorMessage(l10n, err))),
    );
  }
}

class _TransferOwnershipButton extends ConsumerWidget {
  const _TransferOwnershipButton({
    super.key,
    required this.chatId,
    required this.profileId,
  });

  final String chatId;
  final String profileId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final profile = ref.watch(profileProvider(profileId)).valueOrNull;
    final name = profile?.displayName ?? profile?.handle ?? profileId;
    return TextButton(
      onPressed: () =>
          _confirmTransferOwnership(context, ref, chatId, profileId, name),
      child: Text(l10n.chatGroupTransferOwnershipTo(name)),
    );
  }
}

class _MemberTile extends ConsumerWidget {
  const _MemberTile({
    super.key,
    required this.member,
    required this.isSelf,
    required this.canKick,
    required this.kickKey,
    required this.onTap,
    required this.onKick,
  });

  final ChatMember member;
  final bool isSelf;
  final bool canKick;
  final Key kickKey;
  final VoidCallback onTap;
  final VoidCallback onKick;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final profile = ref.watch(profileProvider(member.profileId)).valueOrNull;
    final label = profile?.displayName ?? profile?.handle ?? member.profileId;
    final roleLabel = switch (member.role) {
      kChatRoleOwner => l10n.chatGroupRoleOwner,
      kChatRoleAdmin => l10n.chatGroupRoleAdmin,
      _ => null,
    };

    return ListTile(
      onTap: onTap,
      leading: VoiceAvatar(
        imageUrl: profile?.avatarUrl,
        label: label,
        radius: 20,
      ),
      title: Text(isSelf ? l10n.chatGroupMemberYou(label) : label),
      subtitle: roleLabel == null ? null : Text(roleLabel),
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
