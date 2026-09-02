import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/chat_navigation_providers.dart';
import '../core/voice_bottom_sheet.dart';

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

  @override
  void dispose() {
    _createController.dispose();
    _renameController.dispose();
    super.dispose();
  }

  Future<void> _createFolder() async {
    final name = _createController.text.trim();
    if (name.isEmpty) return;
    final error = await ref.read(folderActionsProvider).createFolder(name);
    if (!mounted) return;
    if (error != null) {
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(error)));
      return;
    }
    _createController.clear();
    setState(() {});
  }

  Future<void> _renameFolder(String folderId) async {
    final name = _renameController.text.trim();
    if (name.isEmpty) return;
    final error = await ref
        .read(folderActionsProvider)
        .updateFolder(folderId: folderId, name: name);
    if (!mounted) return;
    if (error != null) {
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(error)));
      return;
    }
    setState(() {
      _renamingFolderId = null;
      _renameController.clear();
    });
  }

  Future<void> _deleteFolder(String folderId, String folderName) async {
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
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(error)));
    } else {
      setState(() {});
    }
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
                    decoration: InputDecoration(
                      labelText: l10n.chatFolderCreateLabel,
                    ),
                    onSubmitted: (_) => _createFolder(),
                  ),
                ),
                const SizedBox(width: 8),
                IconButton(
                  key: const Key('manage_folders_create_button'),
                  onPressed: _createFolder,
                  icon: const Icon(Icons.add),
                  tooltip: l10n.chatFolderCreateAction,
                ),
              ],
            ),
            const SizedBox(height: 8),
            Flexible(
              child: foldersAsync.when(
                loading: () => const Center(child: CircularProgressIndicator()),
                error: (e, _) => Text(l10n.backendUnavailable),
                data: (data) {
                  final custom = data.folders.where((f) => !f.isSystem).toList();
                  if (custom.isEmpty) {
                    return Text(
                      l10n.chatFoldersCustomEmpty,
                      style: Theme.of(context).textTheme.bodyMedium,
                    );
                  }
                  return ListView.builder(
                    shrinkWrap: true,
                    itemCount: custom.length,
                    itemBuilder: (context, index) {
                      final folder = custom[index];
                      final isRenaming = _renamingFolderId == folder.id;
                      return ListTile(
                        key: Key('manage_folder_${folder.id}'),
                        title: isRenaming
                            ? TextField(
                                key: Key('manage_folder_rename_${folder.id}'),
                                controller: _renameController,
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
                              onPressed: () {
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
                              onPressed: () =>
                                  _deleteFolder(folder.id, folder.name),
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
