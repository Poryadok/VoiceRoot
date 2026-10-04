import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/chats_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/chat_navigation_providers.dart';
import '../../state/chat_providers.dart';
import '../api_error_messages.dart';
import '../core/voice_bottom_sheet.dart';
import '../core/voice_skeleton.dart';
import '../core/voice_state_panel.dart';

/// Create, rename, and delete custom chat folders (navigation.md § folders).
class ManageFoldersSheet extends ConsumerStatefulWidget {
  const ManageFoldersSheet({super.key});

  static const sheetKey = Key('manage_folders_sheet');

  static Future<void> show(BuildContext context) {
    return showVoiceBottomSheet<void>(
      context: context,
      child: const ManageFoldersSheet(),
    );
  }

  @override
  ConsumerState<ManageFoldersSheet> createState() => _ManageFoldersSheetState();
}

class _ManageFoldersSheetState extends ConsumerState<ManageFoldersSheet> {
  final _createController = TextEditingController();
  String? _renamingFolderId;
  final _renameController = TextEditingController();
  bool _reordering = false;

  @override
  void dispose() {
    _createController.dispose();
    _renameController.dispose();
    super.dispose();
  }

  Future<void> _createFolder() async {
    if (_reordering) return;
    final name = _createController.text.trim();
    if (name.isEmpty) return;
    final error = await ref.read(folderActionsProvider).createFolder(name);
    if (!mounted) return;
    if (error != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            commonActionErrorMessage(AppLocalizations.of(context)!),
          ),
        ),
      );
      return;
    }
    _createController.clear();
    setState(() {});
  }

  Future<void> _renameFolder(String folderId) async {
    if (_reordering) return;
    final name = _renameController.text.trim();
    if (name.isEmpty) return;
    final error = await ref
        .read(folderActionsProvider)
        .updateFolder(folderId: folderId, name: name);
    if (!mounted) return;
    if (error != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            commonActionErrorMessage(AppLocalizations.of(context)!),
          ),
        ),
      );
      return;
    }
    setState(() {
      _renamingFolderId = null;
      _renameController.clear();
    });
  }

  Future<void> _deleteFolder(String folderId, String folderName) async {
    if (_reordering) return;
    final l10n = AppLocalizations.of(context)!;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(l10n.chatFolderDeleteTitle),
        content: Text(l10n.chatFolderDeleteMessage(folderName)),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(l10n.commonCancel),
          ),
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: Text(l10n.commonDelete),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;
    final error = await ref.read(folderActionsProvider).deleteFolder(folderId);
    if (!mounted) return;
    if (error != null) {
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(commonActionErrorMessage(l10n))));
    } else {
      setState(() {});
    }
  }

  Future<void> _reorderFolders(
    List<VoiceFolder> folders,
    int oldIndex,
    int newIndex,
  ) async {
    if (_reordering) return;
    if (newIndex > oldIndex) newIndex--;
    if (newIndex == oldIndex) return;
    final reordered = [...folders];
    final moved = reordered.removeAt(oldIndex);
    reordered.insert(newIndex, moved);
    setState(() => _reordering = true);
    final error = await ref
        .read(folderActionsProvider)
        .reorderCustomFolders(reordered);
    if (!mounted) return;
    if (error != null && error != kChatActionStaleContext) {
      final message = error == kFolderReorderMayBePartial
          ? AppLocalizations.of(context)!.chatFolderReorderMayBePartial
          : commonActionErrorMessage(AppLocalizations.of(context)!);
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(message)));
    }
    setState(() => _reordering = false);
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final foldersAsync = ref.watch(chatFoldersProvider);

    return SafeArea(
      key: ManageFoldersSheet.sheetKey,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              l10n.chatFoldersManageTitle,
              style: Theme.of(context).textTheme.titleLarge,
            ),
            const SizedBox(height: 12),
            Row(
              children: [
                Expanded(
                  child: TextField(
                    key: const Key('manage_folders_create_field'),
                    controller: _createController,
                    enabled: !_reordering,
                    decoration: InputDecoration(
                      labelText: l10n.chatFolderCreateLabel,
                    ),
                    onSubmitted: (_) => _createFolder(),
                  ),
                ),
                const SizedBox(width: 8),
                IconButton(
                  key: const Key('manage_folders_create_button'),
                  onPressed: _reordering ? null : _createFolder,
                  icon: const Icon(Icons.add),
                  tooltip: l10n.chatFolderCreateAction,
                ),
              ],
            ),
            const SizedBox(height: 8),
            Flexible(
              child: foldersAsync.when(
                loading: () => const VoiceListSkeleton(rowCount: 3),
                error: (_, _) => VoiceStatePanel(
                  title: l10n.backendUnavailable,
                  icon: Icons.cloud_off_outlined,
                ),
                data: (data) {
                  final custom = data.folders
                      .where((f) => !f.isSystem)
                      .toList();
                  if (custom.isEmpty) {
                    return Text(
                      l10n.chatFoldersCustomEmpty,
                      style: Theme.of(context).textTheme.bodyMedium,
                    );
                  }
                  return ReorderableListView.builder(
                    shrinkWrap: true,
                    buildDefaultDragHandles: false,
                    itemCount: custom.length,
                    onReorder: (oldIndex, newIndex) =>
                        _reorderFolders(custom, oldIndex, newIndex),
                    itemBuilder: (context, index) {
                      final folder = custom[index];
                      final isRenaming = _renamingFolderId == folder.id;
                      return ListTile(
                        key: Key('manage_folder_${folder.id}'),
                        leading: ReorderableDragStartListener(
                          index: index,
                          enabled: !_reordering,
                          child: const Icon(Icons.drag_handle),
                        ),
                        title: isRenaming
                            ? TextField(
                                key: Key('manage_folder_rename_${folder.id}'),
                                controller: _renameController,
                                enabled: !_reordering,
                                autofocus: true,
                                onSubmitted: (_) => _renameFolder(folder.id),
                              )
                            : Text(folder.name),
                        trailing: Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            IconButton(
                              key: Key('manage_folder_edit_${folder.id}'),
                              icon: const Icon(Icons.edit_outlined),
                              tooltip: l10n.commonEdit,
                              onPressed: _reordering
                                  ? null
                                  : () {
                                      setState(() {
                                        _renamingFolderId = folder.id;
                                        _renameController.text = folder.name;
                                      });
                                    },
                            ),
                            IconButton(
                              key: Key('manage_folder_delete_${folder.id}'),
                              icon: const Icon(Icons.delete_outline),
                              tooltip: l10n.commonDelete,
                              onPressed: _reordering
                                  ? null
                                  : () => _deleteFolder(folder.id, folder.name),
                            ),
                          ],
                        ),
                      );
                    },
                  );
                },
              ),
            ),
          ],
        ),
      ),
    );
  }
}
