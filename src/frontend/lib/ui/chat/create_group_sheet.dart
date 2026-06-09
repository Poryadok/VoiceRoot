import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/chat_providers.dart';
import '../../state/social_providers.dart';
import '../api_error_messages.dart';
import '../core/voice_bottom_sheet.dart';
import '../core/voice_state_panel.dart';

/// Minimum invitees besides the creator (3 people total per text-chat.md).
const int kMinGroupInvitees = 2;

/// Bottom sheet: group name + multi-select friends → POST /api/v1/chats + members.
class CreateGroupSheet extends ConsumerStatefulWidget {
  const CreateGroupSheet({super.key});

  static const Key sheetKey = Key('create_group_sheet');
  static const Key nameFieldKey = Key('create_group_name');
  static const Key submitKey = Key('create_group_submit');

  static Key memberTileKey(String profileId) =>
      Key('create_group_member_$profileId');

  static Future<void> show(BuildContext context) {
    final container = ProviderScope.containerOf(context);
    return showVoiceBottomSheet<void>(
      context: context,
      scrollable: false,
      child: UncontrolledProviderScope(
        container: container,
        child: const CreateGroupSheet(),
      ),
    );
  }

  @override
  ConsumerState<CreateGroupSheet> createState() => _CreateGroupSheetState();
}

class _CreateGroupSheetState extends ConsumerState<CreateGroupSheet> {
  final _nameController = TextEditingController();
  final _selected = <String>{};
  var _submitting = false;

  @override
  void dispose() {
    _nameController.dispose();
    super.dispose();
  }

  bool get _canSubmit {
    final name = _nameController.text.trim();
    return !_submitting &&
        name.isNotEmpty &&
        _selected.length >= kMinGroupInvitees;
  }

  Future<void> _submit() async {
    if (!_canSubmit) return;
    setState(() => _submitting = true);
    final l10n = AppLocalizations.of(context)!;
    final err = await ref.read(chatActionsProvider).createGroupWithMembers(
      name: _nameController.text.trim(),
      memberProfileIds: _selected.toList(growable: false),
    );
    if (!mounted) return;
    setState(() => _submitting = false);
    if (err != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(l10n.chatCreateGroupError(err))),
      );
      return;
    }
    Navigator.of(context).pop();
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final friendsAsync = ref.watch(friendsListProvider);
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
            Text(l10n.chatCreateGroupMembers, style: theme.textTheme.titleSmall),
            const SizedBox(height: 4),
            Text(
              l10n.chatCreateGroupMembersHint,
              style: theme.textTheme.bodySmall,
            ),
            const SizedBox(height: 8),
            Expanded(
              child: friendsAsync.when(
                loading: () => const Center(child: CircularProgressIndicator()),
                error: (e, st) => VoiceStatePanel(
                  title: socialListErrorMessage(l10n, e),
                  icon: Icons.cloud_off_outlined,
                  actionLabel: l10n.commonRetry,
                  onAction: () => ref.invalidate(friendsListProvider),
                ),
                data: (data) {
                  final ids = data.friends;
                  if (ids.isEmpty) {
                    return VoiceStatePanel(
                      title: l10n.socialFriendsEmpty,
                      message: l10n.chatCreateGroupFriendsEmptyHint,
                      icon: Icons.people_outline,
                    );
                  }
                  return ListView.builder(
                    itemCount: ids.length,
                    itemBuilder: (context, index) {
                      final profileId = ids[index];
                      final profileAsync = ref.watch(profileProvider(profileId));
                      final profile = profileAsync.valueOrNull;
                      final label =
                          profile?.displayName ??
                          profile?.handle ??
                          profileId;
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
              onPressed: _canSubmit ? _submit : null,
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
