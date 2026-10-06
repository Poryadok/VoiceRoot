import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/create_group_friends_provider.dart';
import '../../state/social_providers.dart';
import '../api_error_messages.dart';
import '../core/voice_bottom_sheet.dart';

/// Minimum invitees besides the creator (3 people total per text-chat.md).
const int kMinGroupInvitees = 2;

/// Bottom sheet: group name + multi-select friends → POST /api/v1/chats + members.
class CreateGroupSheet extends ConsumerStatefulWidget {
  const CreateGroupSheet({
    super.key,
    this.requiredMemberProfileId,
    this.expectedViewerProfileId,
  });

  final String? requiredMemberProfileId;
  final String? expectedViewerProfileId;

  static const Key sheetKey = Key('create_group_sheet');
  static const Key nameFieldKey = Key('create_group_name');
  static const Key searchFieldKey = Key('create_group_friend_search');
  static const Key submitKey = Key('create_group_submit');

  static Key memberTileKey(String profileId) =>
      Key('create_group_member_$profileId');

  static Future<void> show(
    BuildContext context, {
    String? requiredMemberProfileId,
    String? expectedViewerProfileId,
  }) {
    final container = ProviderScope.containerOf(context);
    return showVoiceBottomSheet<void>(
      context: context,
      scrollable: false,
      child: UncontrolledProviderScope(
        container: container,
        child: CreateGroupSheet(
          requiredMemberProfileId: requiredMemberProfileId,
          expectedViewerProfileId: expectedViewerProfileId,
        ),
      ),
    );
  }

  @override
  ConsumerState<CreateGroupSheet> createState() => _CreateGroupSheetState();
}

class _CreateGroupSheetState extends ConsumerState<CreateGroupSheet> {
  final _nameController = TextEditingController();
  final _searchController = TextEditingController();
  final _selected = <String>{};
  var _submitting = false;

  @override
  void initState() {
    super.initState();
    final requiredId = widget.requiredMemberProfileId;
    if (requiredId != null && requiredId.isNotEmpty) {
      _selected.add(requiredId);
    }
  }

  @override
  void dispose() {
    _nameController.dispose();
    _searchController.dispose();
    super.dispose();
  }

  bool get _canSubmit {
    final name = _nameController.text.trim();
    return !_submitting &&
        name.isNotEmpty &&
        _selected.length >= kMinGroupInvitees &&
        (widget.expectedViewerProfileId == null ||
            ref.read(authControllerProvider).activeProfileId ==
                widget.expectedViewerProfileId);
  }

  Future<void> _submit() async {
    if (!_canSubmit) return;
    setState(() => _submitting = true);
    final l10n = AppLocalizations.of(context)!;
    final err = await ref
        .read(chatActionsProvider)
        .createGroupWithMembers(
          name: _nameController.text.trim(),
          memberProfileIds: _selected.toList(growable: false),
        );
    if (!mounted) return;
    setState(() => _submitting = false);
    if (err != null) {
      final errorMessage = err == 'not_authenticated'
          ? l10n.chatCreateGroupError(err)
          : commonActionErrorMessage(l10n);
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(errorMessage)));
      return;
    }
    Navigator.of(context).pop();
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final friendsAsync = ref.watch(createGroupFriendsProvider);
    final activeProfileId = ref.watch(authControllerProvider).activeProfileId;
    final viewerMatches =
        widget.expectedViewerProfileId == null ||
        activeProfileId == widget.expectedViewerProfileId;
    final theme = Theme.of(context);

    return SafeArea(
      key: CreateGroupSheet.sheetKey,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(l10n.chatCreateGroupTitle, style: theme.textTheme.titleLarge),
            const SizedBox(height: 12),
            TextField(
              key: CreateGroupSheet.nameFieldKey,
              controller: _nameController,
              decoration: InputDecoration(
                labelText: l10n.chatCreateGroupNameLabel,
                hintText: l10n.chatCreateGroupNameHint,
              ),
              textCapitalization: TextCapitalization.sentences,
              enabled: !_submitting,
              onChanged: (_) => setState(() {}),
            ),
            const SizedBox(height: 16),
            Text(
              l10n.chatCreateGroupMembers,
              style: theme.textTheme.titleSmall,
            ),
            const SizedBox(height: 4),
            Text(
              l10n.chatCreateGroupMembersHint,
              style: theme.textTheme.bodySmall,
            ),
            if (widget.requiredMemberProfileId case final requiredId?)
              _requiredMemberTile(requiredId),
            const SizedBox(height: 8),
            TextField(
              key: CreateGroupSheet.searchFieldKey,
              controller: _searchController,
              decoration: InputDecoration(
                hintText: l10n.socialSearchHint,
                prefixIcon: const Icon(Icons.search),
              ),
              textInputAction: TextInputAction.search,
              enabled: !_submitting,
              onChanged: (_) => setState(() {}),
            ),
            const SizedBox(height: 8),
            Expanded(
              child: friendsAsync.when(
                loading: () => const Center(child: CircularProgressIndicator()),
                error: (e, st) => _createGroupStatePanel(
                  context,
                  title: socialListErrorMessage(l10n, e),
                  icon: Icons.cloud_off_outlined,
                  actionLabel: l10n.commonRetry,
                  onAction: () => ref.invalidate(createGroupFriendsProvider),
                ),
                data: (ids) {
                  final availableIds = widget.requiredMemberProfileId == null
                      ? ids
                      : ids
                            .where((id) => id != widget.requiredMemberProfileId)
                            .toList(growable: false);
                  if (availableIds.isEmpty) {
                    return _createGroupStatePanel(
                      context,
                      title: l10n.socialFriendsEmpty,
                      message: l10n.chatCreateGroupFriendsEmptyHint,
                      icon: Icons.people_outline,
                    );
                  }
                  final query = _searchController.text.trim().toLowerCase();
                  final searchQuery = query.startsWith('@')
                      ? query.substring(1)
                      : query;
                  var visibleIds = availableIds;
                  if (searchQuery.isNotEmpty) {
                    final profiles = {
                      for (final profileId in availableIds)
                        profileId: ref.watch(profileProvider(profileId)),
                    };
                    final failedProfileIds = profiles.entries
                        .where((entry) => entry.value.hasError)
                        .map((entry) => entry.key)
                        .toList(growable: false);
                    if (failedProfileIds.isNotEmpty) {
                      final error = profiles[failedProfileIds.first]!.error!;
                      return _createGroupStatePanel(
                        context,
                        title: socialListErrorMessage(l10n, error),
                        icon: Icons.cloud_off_outlined,
                        actionLabel: l10n.commonRetry,
                        onAction: () {
                          for (final profileId in failedProfileIds) {
                            ref.invalidate(profileProvider(profileId));
                          }
                        },
                      );
                    }
                    if (profiles.values.any(
                      (profile) => profile.isLoading && !profile.hasValue,
                    )) {
                      return const Center(child: CircularProgressIndicator());
                    }
                    visibleIds = availableIds
                        .where((profileId) {
                          final profile = profiles[profileId]?.valueOrNull;
                          final displayName = profile?.displayName
                              .toLowerCase();
                          final handle = profile?.handle
                              .replaceFirst(RegExp(r'^@'), '')
                              .toLowerCase();
                          return (displayName?.contains(searchQuery) ??
                                  false) ||
                              (handle?.contains(searchQuery) ?? false);
                        })
                        .toList(growable: false);
                    if (visibleIds.isEmpty) {
                      return _createGroupStatePanel(
                        context,
                        title: l10n.socialSearchEmpty,
                        message: l10n.socialSearchEmptyHint,
                        icon: Icons.search_off,
                      );
                    }
                  }
                  return ListView.builder(
                    itemCount: visibleIds.length,
                    itemBuilder: (context, index) {
                      final profileId = visibleIds[index];
                      final profileAsync = ref.watch(
                        profileProvider(profileId),
                      );
                      final profile = profileAsync.valueOrNull;
                      final label =
                          profile?.displayName ?? profile?.handle ?? profileId;
                      final selected = _selected.contains(profileId);
                      return CheckboxListTile(
                        key: CreateGroupSheet.memberTileKey(profileId),
                        value: selected,
                        onChanged: _submitting
                            ? null
                            : (next) {
                                setState(() {
                                  if (next ?? false) {
                                    _selected.add(profileId);
                                  } else {
                                    _selected.remove(profileId);
                                  }
                                });
                              },
                        title: Text(label),
                        subtitle: profile != null ? Text(profile.handle) : null,
                        secondary: CircleAvatar(
                          child: Text(
                            label.isNotEmpty ? label[0].toUpperCase() : '?',
                          ),
                        ),
                        controlAffinity: ListTileControlAffinity.leading,
                      );
                    },
                  );
                },
              ),
            ),
            if (_selected.length < kMinGroupInvitees)
              Padding(
                padding: const EdgeInsets.only(bottom: 8),
                child: Text(
                  l10n.chatCreateGroupMinMembers,
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: theme.colorScheme.error,
                  ),
                ),
              ),
            FilledButton(
              key: CreateGroupSheet.submitKey,
              onPressed: _canSubmit && viewerMatches ? _submit : null,
              child: _submitting
                  ? const SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : Text(l10n.chatCreateGroupSubmit),
            ),
          ],
        ),
      ),
    );
  }
}

Widget _createGroupStatePanel(
  BuildContext context, {
  required String title,
  IconData? icon,
  String? message,
  String? actionLabel,
  VoidCallback? onAction,
}) {
  final theme = Theme.of(context);
  return Semantics(
    container: true,
    label: title,
    child: Center(
      child: Padding(
        padding: const EdgeInsets.all(8),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (icon != null) ...[
              Icon(icon, size: 20, color: theme.colorScheme.onSurfaceVariant),
              const SizedBox(height: 4),
            ],
            ExcludeSemantics(
              child: Text(
                title,
                textAlign: TextAlign.center,
                style: theme.textTheme.titleSmall,
              ),
            ),
            if (message != null && message.isNotEmpty) ...[
              const SizedBox(height: 4),
              Text(
                message,
                textAlign: TextAlign.center,
                style: theme.textTheme.bodySmall,
              ),
            ],
            if (actionLabel != null && onAction != null)
              TextButton(onPressed: onAction, child: Text(actionLabel)),
          ],
        ),
      ),
    ),
  );
}

extension on _CreateGroupSheetState {
  Widget _requiredMemberTile(String profileId) {
    final profile = ref.watch(profileProvider(profileId)).valueOrNull;
    final label = profile?.displayName ?? profile?.handle ?? profileId;
    return CheckboxListTile(
      key: CreateGroupSheet.memberTileKey(profileId),
      value: true,
      onChanged: null,
      title: Text(label),
      subtitle: profile == null ? null : Text(profile.handle),
      secondary: const Icon(Icons.lock_outline),
      controlAffinity: ListTileControlAffinity.leading,
    );
  }
}
