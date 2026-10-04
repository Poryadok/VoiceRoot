import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/spaces_client.dart';
import '../../backend/space_permissions.dart';
import '../../l10n/app_localizations.dart';
import '../../state/space_providers.dart';
import '../../state/voice_room_providers.dart';
import '../../gen/voice/chat/v1/chat.pbenum.dart';
import '../api_error_messages.dart';
import '../core/voice_disabled_action.dart';
import '../core/voice_skeleton.dart';
import '../core/voice_state_panel.dart';
import 'space_voice_room_override_sheet.dart';

/// Sidebar tree: categories with text chats and voice rooms.
class SpaceTreePanel extends ConsumerWidget {
  const SpaceTreePanel({
    super.key,
    required this.spaceId,
    this.selectedChatId,
    required this.onTextChatSelected,
  });

  static const Key panelKey = Key('space_tree_panel');
  static const Key emptyKey = Key('space_tree_empty');
  static const Key errorKey = Key('space_tree_error');
  static const Key createTextChatButtonKey = Key('space_tree_create_chat');
  static const Key createCategoryButtonKey = Key('space_tree_create_category');
  static const Key createNameFieldKey = Key('space_tree_create_name');
  static const Key createSubmitButtonKey = Key('space_tree_create_submit');
  static const Key createChannelTypeKey = Key('space_tree_create_type_channel');
  static const Key createGroupTypeKey = Key('space_tree_create_type_group');
  static const Key createCategoryDropdownKey = Key(
    'space_tree_create_category_picker',
  );
  static Key categoryKey(String id) => Key('space_tree_category_$id');
  static Key nodeKey(String id) => Key('space_tree_node_$id');
  static Key reorderHandleKey(String id) => Key('space_tree_reorder_$id');

  final String spaceId;
  final String? selectedChatId;
  final ValueChanged<String> onTextChatSelected;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final treeAsync = ref.watch(spaceTreeProvider(spaceId));
    final canManageTree =
        ref
            .watch(
              spacePermissionProvider((
                spaceId: spaceId,
                permission: 'TEXT_CHAT_CREATE_IN_SPACE',
                chatId: null,
                voiceRoomId: null,
              )),
            )
            .valueOrNull ??
        false;

    return KeyedSubtree(
      key: panelKey,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          if (canManageTree)
            Padding(
              padding: const EdgeInsets.fromLTRB(4, 4, 8, 0),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.end,
                children: [
                  IconButton(
                    key: createTextChatButtonKey,
                    tooltip: l10n.spaceTreeCreateTextChat,
                    icon: const Icon(Icons.add_comment_outlined),
                    onPressed: () => _SpaceTreeCreateDialog.show(
                      context,
                      title: l10n.spaceTreeCreateTextChat,
                      l10n: l10n,
                      allowChatType: true,
                      categories: treeAsync.valueOrNull?.categories ?? const [],
                      onCreate: (name, chatType, categoryId) => ref
                          .read(spaceTreeActionsProvider)
                          .createTextChat(
                            spaceId: spaceId,
                            name: name,
                            chatType: chatType,
                            categoryId: categoryId,
                          ),
                    ),
                  ),
                  IconButton(
                    key: createCategoryButtonKey,
                    tooltip: l10n.spaceTreeCreateCategory,
                    icon: const Icon(Icons.create_new_folder_outlined),
                    onPressed: () => _SpaceTreeCreateDialog.show(
                      context,
                      title: l10n.spaceTreeCreateCategory,
                      l10n: l10n,
                      allowChatType: false,
                      categories: const [],
                      onCreate: (name, _, _) => ref
                          .read(spaceTreeActionsProvider)
                          .createCategory(spaceId: spaceId, name: name),
                    ),
                  ),
                ],
              ),
            ),
          Expanded(
            child: treeAsync.when(
              loading: () => const VoiceListSkeleton(),
              error: (e, _) => VoiceStatePanel(
                key: errorKey,
                title: l10n.spaceTreeLoadError,
                message: spaceTreeErrorMessage(l10n, e),
                icon: Icons.account_tree_outlined,
                actionLabel: l10n.commonRetry,
                onAction: () => ref.invalidate(spaceTreeProvider(spaceId)),
              ),
              data: (tree) {
                if (tree.nodes.isEmpty && tree.categories.isEmpty) {
                  return VoiceStatePanel(
                    key: emptyKey,
                    title: l10n.spaceTreeEmpty,
                    icon: Icons.account_tree_outlined,
                  );
                }
                return ListView(
                  padding: const EdgeInsets.symmetric(vertical: 8),
                  children: _buildSections(
                    context,
                    ref,
                    l10n,
                    tree,
                    canReorder: canManageTree,
                  ),
                );
              },
            ),
          ),
        ],
      ),
    );
  }

  List<Widget> _buildSections(
    BuildContext context,
    WidgetRef ref,
    AppLocalizations l10n,
    SpaceTreeData tree, {
    required bool canReorder,
  }) {
    final widgets = <Widget>[];
    final categorized = <String, List<SpaceTreeNodeData>>{};
    final uncategorized = <SpaceTreeNodeData>[];

    for (final node in tree.nodes) {
      final catId = node.categoryId;
      if (catId == null || catId.isEmpty) {
        uncategorized.add(node);
      } else {
        categorized.putIfAbsent(catId, () => []).add(node);
      }
    }

    for (final category in tree.categories) {
      widgets.add(
        _CategorySection(
          key: categoryKey(category.id),
          title: category.name,
          nodes: categorized[category.id] ?? const [],
          l10n: l10n,
          selectedChatId: selectedChatId,
          onTextChatSelected: onTextChatSelected,
          canReorder: canReorder,
          onReorder: (ids) async {
            final error = await ref
                .read(spaceTreeActionsProvider)
                .reorderTreeNodes(spaceId: spaceId, orderedNodeIds: ids);
            if (context.mounted && error != null) {
              ScaffoldMessenger.of(context).showSnackBar(
                SnackBar(
                  content: Text(l10n.spaceTreeReorderError(error.message)),
                ),
              );
            }
          },
        ),
      );
    }

    if (uncategorized.isNotEmpty) {
      widgets.add(
        _CategorySection(
          key: categoryKey('uncategorized'),
          title: l10n.spaceTreeUncategorized,
          nodes: uncategorized,
          l10n: l10n,
          selectedChatId: selectedChatId,
          onTextChatSelected: onTextChatSelected,
          initiallyExpanded: true,
          canReorder: canReorder,
          onReorder: (ids) async {
            final error = await ref
                .read(spaceTreeActionsProvider)
                .reorderTreeNodes(spaceId: spaceId, orderedNodeIds: ids);
            if (context.mounted && error != null) {
              ScaffoldMessenger.of(context).showSnackBar(
                SnackBar(
                  content: Text(l10n.spaceTreeReorderError(error.message)),
                ),
              );
            }
          },
        ),
      );
    }

    return widgets;
  }
}

class _SpaceTreeCreateDialog extends StatefulWidget {
  const _SpaceTreeCreateDialog({
    required this.title,
    required this.l10n,
    required this.allowChatType,
    required this.categories,
    required this.onCreate,
  });

  final String title;
  final AppLocalizations l10n;
  final bool allowChatType;
  final List<SpaceCategory> categories;
  final Future<SpaceTreeActionError?> Function(
    String name,
    ChatType chatType,
    String? categoryId,
  )
  onCreate;

  static Future<void> show(
    BuildContext context, {
    required String title,
    required AppLocalizations l10n,
    required bool allowChatType,
    required List<SpaceCategory> categories,
    required Future<SpaceTreeActionError?> Function(String, ChatType, String?)
    onCreate,
  }) {
    return showDialog<void>(
      context: context,
      builder: (_) => _SpaceTreeCreateDialog(
        title: title,
        l10n: l10n,
        allowChatType: allowChatType,
        categories: categories,
        onCreate: onCreate,
      ),
    );
  }

  @override
  State<_SpaceTreeCreateDialog> createState() => _SpaceTreeCreateDialogState();
}

class _SpaceTreeCreateDialogState extends State<_SpaceTreeCreateDialog> {
  final _nameController = TextEditingController();
  ChatType _chatType = ChatType.CHAT_TYPE_GROUP;
  String _categoryId = '';
  bool _submitting = false;

  @override
  void dispose() {
    _nameController.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final name = _nameController.text.trim();
    if (name.isEmpty || _submitting) return;
    setState(() => _submitting = true);
    final error = await widget.onCreate(
      name,
      _chatType,
      _categoryId.isEmpty ? null : _categoryId,
    );
    if (!mounted) return;
    setState(() => _submitting = false);
    if (error != null) {
      final messenger = ScaffoldMessenger.of(context);
      final mustClose = error.kind != SpaceTreeActionErrorKind.rejected;
      if (mustClose) Navigator.of(context).pop();
      final detail = switch (error.message) {
        'not_authenticated' => error.message,
        'name_required' => error.message,
        _ => widget.l10n.commonActionFailed,
      };
      final message = switch (error.kind) {
        SpaceTreeActionErrorKind.rejected => widget.l10n.spaceTreeCreateError(
          detail,
        ),
        SpaceTreeActionErrorKind.partialSuccess =>
          widget.l10n.spaceTreeCategoryAssignError(detail),
        SpaceTreeActionErrorKind.placementUncertain =>
          widget.l10n.spaceTreePlacementUnknown(detail),
        SpaceTreeActionErrorKind.outcomeUncertain =>
          widget.l10n.spaceTreeCreateOutcomeUnknown(detail),
        SpaceTreeActionErrorKind.refreshFailed =>
          widget.l10n.spaceTreeCreateRefreshFailed,
      };
      messenger.showSnackBar(SnackBar(content: Text(message)));
      return;
    }
    Navigator.of(context).pop();
  }

  @override
  Widget build(BuildContext context) {
    final l10n = widget.l10n;
    return AlertDialog(
      title: Text(widget.title),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          if (widget.allowChatType) ...[
            Wrap(
              spacing: 8,
              children: [
                ChoiceChip(
                  key: SpaceTreePanel.createGroupTypeKey,
                  label: Text(l10n.spaceTreeCreateGroup),
                  selected: _chatType == ChatType.CHAT_TYPE_GROUP,
                  onSelected: _submitting
                      ? null
                      : (_) => setState(
                          () => _chatType = ChatType.CHAT_TYPE_GROUP,
                        ),
                ),
                ChoiceChip(
                  key: SpaceTreePanel.createChannelTypeKey,
                  label: Text(l10n.spaceTreeCreateChannel),
                  selected: _chatType == ChatType.CHAT_TYPE_CHANNEL,
                  onSelected: _submitting
                      ? null
                      : (_) => setState(
                          () => _chatType = ChatType.CHAT_TYPE_CHANNEL,
                        ),
                ),
              ],
            ),
            const SizedBox(height: 12),
            if (widget.categories.isNotEmpty) ...[
              DropdownButtonFormField<String>(
                key: SpaceTreePanel.createCategoryDropdownKey,
                initialValue: _categoryId,
                decoration: InputDecoration(
                  labelText: l10n.spaceTreeCategoryLabel,
                ),
                items: [
                  DropdownMenuItem(
                    value: '',
                    child: Text(l10n.spaceTreeNoCategory),
                  ),
                  for (final category in widget.categories)
                    DropdownMenuItem(
                      value: category.id,
                      child: Text(category.name),
                    ),
                ],
                onChanged: _submitting
                    ? null
                    : (value) => setState(() => _categoryId = value ?? ''),
              ),
              const SizedBox(height: 12),
            ],
          ],
          TextField(
            key: SpaceTreePanel.createNameFieldKey,
            controller: _nameController,
            autofocus: true,
            enabled: !_submitting,
            textCapitalization: TextCapitalization.sentences,
            decoration: InputDecoration(
              labelText: l10n.spaceTreeCreateNameLabel,
            ),
            onChanged: (_) => setState(() {}),
            onSubmitted: (_) => _submit(),
          ),
        ],
      ),
      actions: [
        TextButton(
          onPressed: _submitting ? null : () => Navigator.of(context).pop(),
          child: Text(l10n.commonCancel),
        ),
        FilledButton(
          key: SpaceTreePanel.createSubmitButtonKey,
          onPressed: _submitting || _nameController.text.trim().isEmpty
              ? null
              : _submit,
          child: _submitting
              ? const SizedBox(
                  width: 18,
                  height: 18,
                  child: CircularProgressIndicator(strokeWidth: 2),
                )
              : Text(l10n.spaceTreeCreateSubmit),
        ),
      ],
    );
  }
}

class _CategorySection extends StatefulWidget {
  const _CategorySection({
    super.key,
    required this.title,
    required this.nodes,
    required this.l10n,
    required this.selectedChatId,
    required this.onTextChatSelected,
    required this.canReorder,
    required this.onReorder,
    this.initiallyExpanded = true,
  });

  final String title;
  final List<SpaceTreeNodeData> nodes;
  final AppLocalizations l10n;
  final String? selectedChatId;
  final ValueChanged<String> onTextChatSelected;
  final bool canReorder;
  final Future<void> Function(List<String> orderedNodeIds) onReorder;
  final bool initiallyExpanded;

  @override
  State<_CategorySection> createState() => _CategorySectionState();
}

class _CategorySectionState extends State<_CategorySection> {
  late bool _expanded = widget.initiallyExpanded;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        InkWell(
          onTap: () => setState(() => _expanded = !_expanded),
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
            child: Row(
              children: [
                Icon(
                  _expanded ? Icons.expand_more : Icons.chevron_right,
                  size: 18,
                ),
                const SizedBox(width: 4),
                Expanded(
                  child: Text(
                    widget.title.toUpperCase(),
                    style: Theme.of(context).textTheme.labelSmall,
                  ),
                ),
              ],
            ),
          ),
        ),
        if (_expanded) ...widget.nodes.map(_buildNode),
      ],
    );
  }

  Widget _buildNode(SpaceTreeNodeData node) {
    final tile = _TreeNodeTile(
      key: SpaceTreePanel.nodeKey(node.id),
      node: node,
      l10n: widget.l10n,
      selectedChatId: widget.selectedChatId,
      onTextChatSelected: widget.onTextChatSelected,
      showReorderHandle: widget.canReorder,
    );
    if (!widget.canReorder) return tile;

    final siblings = widget.nodes
        .where((candidate) => candidate.isPinned == node.isPinned)
        .toList(growable: false);
    return DragTarget<SpaceTreeNodeData>(
      onWillAcceptWithDetails: (details) =>
          details.data.id != node.id &&
          details.data.categoryId == node.categoryId &&
          details.data.isPinned == node.isPinned,
      onAcceptWithDetails: (details) {
        final ordered = [...siblings];
        final oldIndex = ordered.indexWhere(
          (item) => item.id == details.data.id,
        );
        final targetIndex = ordered.indexWhere((item) => item.id == node.id);
        if (oldIndex < 0 || targetIndex < 0 || oldIndex == targetIndex) return;
        ordered.removeAt(oldIndex);
        final newIndex = ordered.indexWhere((item) => item.id == node.id);
        ordered.insert(newIndex, details.data);
        widget.onReorder(ordered.map((item) => item.id).toList());
      },
      builder: (context, candidateData, rejectedData) =>
          LongPressDraggable<SpaceTreeNodeData>(
            data: node,
            feedback: Material(
              elevation: 6,
              child: SizedBox(
                width: MediaQuery.sizeOf(context).width * .65,
                child: tile,
              ),
            ),
            childWhenDragging: Opacity(opacity: .35, child: tile),
            child: DecoratedBox(
              decoration: BoxDecoration(
                border: candidateData.isEmpty
                    ? null
                    : Border.all(color: Theme.of(context).colorScheme.primary),
              ),
              child: tile,
            ),
          ),
    );
  }
}

class _TreeNodeTile extends ConsumerWidget {
  const _TreeNodeTile({
    super.key,
    required this.node,
    required this.l10n,
    required this.selectedChatId,
    required this.onTextChatSelected,
    required this.showReorderHandle,
  });

  final SpaceTreeNodeData node;
  final AppLocalizations l10n;
  final String? selectedChatId;
  final ValueChanged<String> onTextChatSelected;
  final bool showReorderHandle;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final selectedVoiceRoomId = ref.watch(selectedVoiceRoomIdProvider);
    final voiceRoomId = node.voiceRoomId;
    final isVoiceSelected =
        node.isVoiceRoom &&
        voiceRoomId != null &&
        selectedVoiceRoomId == voiceRoomId;
    final textSelected = node.isTextChat && node.linkedChatId == selectedChatId;
    final manageRoles = resolveSpacePermission(
      l10n,
      ref.watch(
        spacePermissionProvider((
          spaceId: node.spaceId,
          permission: SpacePermissions.spaceManageRoles,
          chatId: null,
          voiceRoomId: null,
        )),
      ),
      SpacePermissions.spaceManageRoles,
    );
    final ({bool allowed, String? deniedReason}) voiceJoin;
    if (voiceRoomId == null) {
      voiceJoin = (allowed: true, deniedReason: null);
    } else {
      voiceJoin = resolveSpacePermission(
        l10n,
        ref.watch(
          spacePermissionProvider((
            spaceId: node.spaceId,
            permission: SpacePermissions.voiceJoin,
            chatId: null,
            voiceRoomId: voiceRoomId,
          )),
        ),
        SpacePermissions.voiceJoin,
      );
    }
    final ({bool allowed, String? deniedReason}) channelPost;
    if (!node.isChannelChat || node.linkedChatId == null) {
      channelPost = (allowed: true, deniedReason: null);
    } else {
      channelPost = resolveSpacePermission(
        l10n,
        ref.watch(
          spacePermissionProvider((
            spaceId: node.spaceId,
            permission: SpacePermissions.textChatSendMessages,
            chatId: node.linkedChatId,
            voiceRoomId: null,
          )),
        ),
        SpacePermissions.textChatSendMessages,
      );
    }
    final disabledReason = node.isVoiceRoom
        ? voiceJoin.deniedReason
        : channelPost.deniedReason;
    final baseSubtitle = node.isVoiceRoom
        ? l10n.spaceTreeVoiceRoom
        : l10n.spaceTreeTextChat;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        VoicePermissionListRow(
          selected: textSelected || isVoiceSelected,
          title: node.displayName,
          baseSubtitle: baseSubtitle,
          disabledReason: disabledReason,
          leading: Icon(_nodeIcon(node), size: 20),
          trailing:
              showReorderHandle || (node.isVoiceRoom && voiceRoomId != null)
              ? Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    if (showReorderHandle)
                      Tooltip(
                        key: SpaceTreePanel.reorderHandleKey(node.id),
                        message: l10n.spaceTreeReorderTooltip,
                        child: const Icon(Icons.drag_handle, size: 18),
                      ),
                    if (node.isVoiceRoom && voiceRoomId != null)
                      VoiceDisabledIconButton(
                        key: Key('voice_room_overrides_$voiceRoomId'),
                        icon: const Icon(
                          Icons.admin_panel_settings_outlined,
                          size: 18,
                        ),
                        iconSize: 18,
                        tooltip: l10n.spaceVoiceOverrideTitle,
                        disabledReason: manageRoles.deniedReason,
                        onPressed: () => SpaceVoiceRoomOverrideSheet.show(
                          context,
                          spaceId: node.spaceId,
                          voiceRoomId: voiceRoomId,
                        ),
                      ),
                  ],
                )
              : null,
          onTap: () => _onTap(ref, voiceJoinAllowed: voiceJoin.allowed),
        ),
        if (isVoiceSelected) _VoiceRoomParticipants(voiceRoomId: voiceRoomId),
      ],
    );
  }

  IconData _nodeIcon(SpaceTreeNodeData node) {
    if (node.isVoiceRoom) return Icons.volume_up_outlined;
    if (node.isChannelChat) return Icons.tag_outlined;
    return Icons.forum_outlined;
  }

  void _onTap(WidgetRef ref, {required bool voiceJoinAllowed}) {
    if (node.isTextChat && node.linkedChatId != null) {
      onTextChatSelected(node.linkedChatId!);
      return;
    }
    final voiceRoomId = node.voiceRoomId;
    if (!node.isVoiceRoom || voiceRoomId == null || !voiceJoinAllowed) return;
    ref.read(selectedVoiceRoomIdProvider.notifier).state = voiceRoomId;
    unawaited(
      ref.read(joinVoiceRoomActionProvider)(
        voiceRoomId: voiceRoomId,
        spaceId: node.spaceId,
      ),
    );
  }
}

class _VoiceRoomParticipants extends ConsumerWidget {
  const _VoiceRoomParticipants({required this.voiceRoomId});

  final String voiceRoomId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final participantsAsync = ref.watch(
      voiceRoomParticipantsProvider(voiceRoomId),
    );
    return participantsAsync.when(
      loading: () => const Padding(
        padding: EdgeInsets.only(left: 40, bottom: 4),
        child: SizedBox(
          height: 16,
          width: 16,
          child: CircularProgressIndicator(strokeWidth: 2),
        ),
      ),
      error: (_, _) => const SizedBox.shrink(),
      data: (participants) {
        if (participants.isEmpty) {
          return const SizedBox.shrink();
        }
        return Padding(
          key: Key('voice_room_participants_$voiceRoomId'),
          padding: const EdgeInsets.only(left: 36, bottom: 4),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              for (final participant in participants)
                Padding(
                  padding: const EdgeInsets.symmetric(vertical: 2),
                  child: Text(
                    participant.displayName,
                    style: Theme.of(context).textTheme.bodySmall,
                  ),
                ),
            ],
          ),
        );
      },
    );
  }
}
