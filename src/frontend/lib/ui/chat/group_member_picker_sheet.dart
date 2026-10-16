import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/create_group_friends_provider.dart';
import '../../state/social_providers.dart';
import '../api_error_messages.dart';
import '../core/voice_bottom_sheet.dart';

class GroupMemberPickerSheet extends ConsumerStatefulWidget {
  const GroupMemberPickerSheet({
    super.key,
    required this.existingProfileIds,
    required this.remainingCapacity,
  });

  static const Key searchFieldKey = Key('group_member_picker_search');
  static const Key submitKey = Key('group_member_picker_submit');
  static Key memberKey(String profileId) =>
      Key('group_member_picker_$profileId');

  final Set<String> existingProfileIds;
  final int remainingCapacity;

  static Future<List<String>?> show(
    BuildContext context, {
    required Set<String> existingProfileIds,
    required int remainingCapacity,
  }) {
    final container = ProviderScope.containerOf(context);
    return showVoiceBottomSheet<List<String>>(
      context: context,
      scrollable: false,
      child: UncontrolledProviderScope(
        container: container,
        child: GroupMemberPickerSheet(
          existingProfileIds: existingProfileIds,
          remainingCapacity: remainingCapacity,
        ),
      ),
    );
  }

  @override
  ConsumerState<GroupMemberPickerSheet> createState() =>
      _GroupMemberPickerSheetState();
}

class _GroupMemberPickerSheetState
    extends ConsumerState<GroupMemberPickerSheet> {
  final _searchController = TextEditingController();
  final _selected = <String>{};

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    final friends = ref.watch(createGroupFriendsProvider);
    final search = _searchController.text.trim().toLowerCase().replaceFirst(
      RegExp(r'^@'),
      '',
    );

    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(l10n.chatGroupAddMembers, style: theme.textTheme.titleLarge),
            const SizedBox(height: 12),
            TextField(
              key: GroupMemberPickerSheet.searchFieldKey,
              controller: _searchController,
              autofocus: true,
              decoration: InputDecoration(
                hintText: l10n.socialSearchHint,
                prefixIcon: const Icon(Icons.search),
              ),
              onChanged: (_) => setState(() {}),
            ),
            const SizedBox(height: 8),
            Expanded(
              child: friends.when(
                loading: () => const Center(child: CircularProgressIndicator()),
                error: (error, _) => _pickerStatePanel(
                  context,
                  title: socialListErrorMessage(l10n, error),
                  actionLabel: l10n.commonRetry,
                  onAction: () => ref.invalidate(createGroupFriendsProvider),
                ),
                data: (ids) {
                  final availableIds = ids
                      .where((id) => !widget.existingProfileIds.contains(id))
                      .toList(growable: false);
                  final profiles = <String, AsyncValue<dynamic>>{};
                  if (search.isNotEmpty) {
                    for (final id in availableIds) {
                      profiles[id] = ref.watch(profileProvider(id));
                    }
                    final failed = profiles.entries
                        .where((entry) => entry.value.hasError)
                        .toList(growable: false);
                    if (failed.isNotEmpty) {
                      return _pickerStatePanel(
                        context,
                        title: socialListErrorMessage(
                          l10n,
                          failed.first.value.error!,
                        ),
                        actionLabel: l10n.commonRetry,
                        onAction: () {
                          for (final entry in failed) {
                            ref.invalidate(profileProvider(entry.key));
                          }
                        },
                      );
                    }
                    if (profiles.values.any(
                      (value) => value.isLoading && !value.hasValue,
                    )) {
                      return const Center(child: CircularProgressIndicator());
                    }
                  }
                  final visible = search.isEmpty
                      ? availableIds
                      : availableIds
                            .where((id) {
                              final profile = profiles[id]?.valueOrNull;
                              return (profile?.displayName
                                          .toLowerCase()
                                          .contains(search) ??
                                      false) ||
                                  (profile?.handle
                                          .replaceFirst(RegExp(r'^@'), '')
                                          .toLowerCase()
                                          .contains(search) ??
                                      false);
                            })
                            .toList(growable: false);
                  if (visible.isEmpty) {
                    return _pickerStatePanel(
                      context,
                      title: search.isEmpty
                          ? l10n.socialFriendsEmpty
                          : l10n.socialSearchEmpty,
                    );
                  }
                  return ListView.builder(
                    itemCount: visible.length,
                    itemBuilder: (context, index) {
                      final id = visible[index];
                      final profile = ref
                          .watch(profileProvider(id))
                          .valueOrNull;
                      final label =
                          profile?.displayName ?? profile?.handle ?? id;
                      final selected = _selected.contains(id);
                      final capacityReached =
                          _selected.length >= widget.remainingCapacity;
                      return CheckboxListTile(
                        key: GroupMemberPickerSheet.memberKey(id),
                        value: selected,
                        onChanged: !selected && capacityReached
                            ? null
                            : (next) => setState(() {
                                if (next ?? false) {
                                  _selected.add(id);
                                } else {
                                  _selected.remove(id);
                                }
                              }),
                        title: Text(label),
                        subtitle: profile == null ? null : Text(profile.handle),
                        secondary: CircleAvatar(
                          child: Text(
                            label.isEmpty ? '?' : label[0].toUpperCase(),
                          ),
                        ),
                        controlAffinity: ListTileControlAffinity.leading,
                      );
                    },
                  );
                },
              ),
            ),
            Row(
              children: [
                Expanded(
                  child: TextButton(
                    onPressed: () => Navigator.of(context).pop(),
                    child: Text(l10n.commonCancel),
                  ),
                ),
                Expanded(
                  child: FilledButton(
                    key: GroupMemberPickerSheet.submitKey,
                    onPressed: _selected.isEmpty
                        ? null
                        : () => Navigator.of(
                            context,
                          ).pop(List<String>.unmodifiable(_selected)),
                    child: Text(l10n.chatGroupAddMembers),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

Widget _pickerStatePanel(
  BuildContext context, {
  required String title,
  String? actionLabel,
  VoidCallback? onAction,
}) => Center(
  child: Column(
    mainAxisSize: MainAxisSize.min,
    children: [
      Text(title),
      if (actionLabel != null && onAction != null)
        TextButton(onPressed: onAction, child: Text(actionLabel)),
    ],
  ),
);
