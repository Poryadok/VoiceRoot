import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/spaces_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/space_providers.dart';
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

    return treeAsync.when(
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
          key: panelKey,
          padding: const EdgeInsets.symmetric(vertical: 8),
          children: _buildSections(context, l10n, tree),
        );
      },
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
              selected: node.linkedChatId == widget.selectedChatId,
              onTextChatSelected: widget.onTextChatSelected,
            ),
          ),
      ],
    );
  }
}

class _TreeNodeTile extends StatelessWidget {
  const _TreeNodeTile({
    super.key,
    required this.node,
    required this.l10n,
    required this.selected,
    required this.onTextChatSelected,
  });

  final SpaceTreeNodeData node;
  final AppLocalizations l10n;
  final bool selected;
  final ValueChanged<String> onTextChatSelected;

  @override
  Widget build(BuildContext context) {
    final icon = node.isVoiceRoom ? Icons.volume_up_outlined : Icons.tag_outlined;
    final subtitle = node.isVoiceRoom
        ? l10n.spaceTreeVoiceRoom
        : l10n.spaceTreeTextChat;

    return VoiceListRow(
      selected: selected,
      title: node.displayName,
      subtitle: subtitle,
      leading: Icon(icon, size: 20),
      onTap: node.isTextChat && node.linkedChatId != null
          ? () => onTextChatSelected(node.linkedChatId!)
          : null,
    );
  }
}
