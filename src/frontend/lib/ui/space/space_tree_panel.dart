import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/spaces_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/space_providers.dart';
import '../../state/voice_room_providers.dart';
import '../core/voice_list_row.dart';
import '../core/voice_skeleton.dart';
import '../core/voice_state_panel.dart';

/// Sidebar tree: categories with text chats and voice rooms.
class SpaceTreePanel extends ConsumerWidget {
  const SpaceTreePanel({
    super.key,
    required this.spaceId,
    this.selectedChatId,
    required this.onTextChatSelected,
  });

  static const Key panelKey = Key('space_tree_panel');
  static Key categoryKey(String id) => Key('space_tree_category_$id');
  static Key nodeKey(String id) => Key('space_tree_node_$id');

  final String spaceId;
  final String? selectedChatId;
  final ValueChanged<String> onTextChatSelected;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final treeAsync = ref.watch(spaceTreeProvider(spaceId));

    return KeyedSubtree(
      key: panelKey,
      child: treeAsync.when(
        loading: () => const VoiceListSkeleton(),
        error: (e, _) => VoiceStatePanel(
          title: l10n.spaceTreeLoadError,
          message: e.toString(),
          icon: Icons.account_tree_outlined,
          actionLabel: l10n.commonRetry,
          onAction: () => ref.invalidate(spaceTreeProvider(spaceId)),
        ),
        data: (tree) {
          if (tree.nodes.isEmpty && tree.categories.isEmpty) {
            return VoiceStatePanel(
              title: l10n.spaceTreeEmpty,
              icon: Icons.account_tree_outlined,
            );
          }
          return ListView(
            padding: const EdgeInsets.symmetric(vertical: 8),
            children: _buildSections(context, l10n, tree),
          );
        },
      ),
    );
  }

  List<Widget> _buildSections(
    BuildContext context,
    AppLocalizations l10n,
    SpaceTreeData tree,
  ) {
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
      widgets.add(_CategorySection(
        key: categoryKey(category.id),
        title: category.name,
        nodes: categorized[category.id] ?? const [],
        l10n: l10n,
        selectedChatId: selectedChatId,
        onTextChatSelected: onTextChatSelected,
      ));
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
        ),
      );
    }

    return widgets;
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
    this.initiallyExpanded = true,
  });

  final String title;
  final List<SpaceTreeNodeData> nodes;
  final AppLocalizations l10n;
  final String? selectedChatId;
  final ValueChanged<String> onTextChatSelected;
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
        if (_expanded)
          ...widget.nodes.map(
            (node) => _TreeNodeTile(
              key: SpaceTreePanel.nodeKey(node.id),
              node: node,
              l10n: widget.l10n,
              selectedChatId: widget.selectedChatId,
              onTextChatSelected: widget.onTextChatSelected,
            ),
          ),
      ],
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
  });

  final SpaceTreeNodeData node;
  final AppLocalizations l10n;
  final String? selectedChatId;
  final ValueChanged<String> onTextChatSelected;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final selectedVoiceRoomId = ref.watch(selectedVoiceRoomIdProvider);
    final voiceRoomId = node.voiceRoomId;
    final isVoiceSelected =
        node.isVoiceRoom && voiceRoomId != null && selectedVoiceRoomId == voiceRoomId;
    final textSelected =
        node.isTextChat && node.linkedChatId == selectedChatId;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        VoiceListRow(
          selected: textSelected || isVoiceSelected,
          title: node.displayName,
          subtitle: node.isVoiceRoom
              ? l10n.spaceTreeVoiceRoom
              : l10n.spaceTreeTextChat,
          leading: Icon(_nodeIcon(node), size: 20),
          onTap: () => _onTap(ref),
        ),
        if (isVoiceSelected)
          _VoiceRoomParticipants(voiceRoomId: voiceRoomId),
      ],
    );
  }

  IconData _nodeIcon(SpaceTreeNodeData node) {
    if (node.isVoiceRoom) return Icons.volume_up_outlined;
    if (node.isChannelChat) return Icons.tag_outlined;
    return Icons.forum_outlined;
  }

  void _onTap(WidgetRef ref) {
    if (node.isTextChat && node.linkedChatId != null) {
      onTextChatSelected(node.linkedChatId!);
      return;
    }
    final voiceRoomId = node.voiceRoomId;
    if (!node.isVoiceRoom || voiceRoomId == null) return;
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
    final participantsAsync = ref.watch(voiceRoomParticipantsProvider(voiceRoomId));
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
