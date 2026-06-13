import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../backend/messages_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/chat_providers.dart';
import '../../state/shared_media_providers.dart';
import '../../state/shell_providers.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_layout.dart';
import '../core/voice_state_panel.dart';
import 'group_members_sheet.dart';

/// Chat info with shared media tabs (Phase 10).
class ChatInfoPanel extends ConsumerStatefulWidget {
  const ChatInfoPanel({
    super.key,
    required this.chatId,
    this.groupName,
    this.isGroup = false,
  });

  static const Key panelKey = Key('chat_info_panel');
  static const Key mediaTabKey = Key('chat_info_tab_media');
  static const Key filesTabKey = Key('chat_info_tab_files');
  static const Key linksTabKey = Key('chat_info_tab_links');
  static const Key voiceTabKey = Key('chat_info_tab_voice');

  final String chatId;
  final String? groupName;
  final bool isGroup;

  @override
  ConsumerState<ChatInfoPanel> createState() => _ChatInfoPanelState();
}

class _ChatInfoPanelState extends ConsumerState<ChatInfoPanel>
    with SingleTickerProviderStateMixin {
  late final TabController _tabs;

  @override
  void initState() {
    super.initState();
    _tabs = TabController(length: 4, vsync: this);
  }

  @override
  void dispose() {
    _tabs.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);

    return Column(
      key: ChatInfoPanel.panelKey,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        if (widget.isGroup) ...[
          SizedBox(
            height: 220,
            child: GroupMembersContent(
              chatId: widget.chatId,
              groupName: widget.groupName,
              showHeader: true,
            ),
          ),
          Divider(height: 1, color: voice.borderDefault),
        ],
        TabBar(
          controller: _tabs,
          isScrollable: true,
          tabs: [
            Tab(key: ChatInfoPanel.mediaTabKey, text: l10n.chatSharedMediaTabMedia),
            Tab(key: ChatInfoPanel.filesTabKey, text: l10n.chatSharedMediaTabFiles),
            Tab(key: ChatInfoPanel.linksTabKey, text: l10n.chatSharedMediaTabLinks),
            Tab(key: ChatInfoPanel.voiceTabKey, text: l10n.chatSharedMediaTabVoice),
          ],
        ),
        Expanded(
          child: TabBarView(
            controller: _tabs,
            children: [
              _SharedMediaTab(chatId: widget.chatId, kind: SharedMediaTabKind.media),
              _SharedMediaTab(chatId: widget.chatId, kind: SharedMediaTabKind.files),
              _SharedMediaTab(chatId: widget.chatId, kind: SharedMediaTabKind.links),
              _SharedMediaTab(chatId: widget.chatId, kind: SharedMediaTabKind.voice),
            ],
          ),
        ),
      ],
    );
  }
}

class _SharedMediaTab extends ConsumerWidget {
  const _SharedMediaTab({required this.chatId, required this.kind});

  final String chatId;
  final SharedMediaTabKind kind;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final async = ref.watch(sharedMediaListProvider((chatId, kind)));

    return async.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (error, _) => VoiceStatePanel(
        title: l10n.chatSharedMediaLoadError,
        message: '$error',
        icon: Icons.cloud_off_outlined,
        actionLabel: l10n.commonRetry,
        onAction: () =>
            ref.invalidate(sharedMediaListProvider((chatId, kind))),
      ),
      data: (data) {
        if (data.items.isEmpty) {
          return VoiceStatePanel(
            title: l10n.chatSharedMediaEmpty,
            icon: Icons.perm_media_outlined,
          );
        }
        return NotificationListener<ScrollNotification>(
          onNotification: (n) {
            if (n.metrics.pixels >= n.metrics.maxScrollExtent - 120) {
              ref
                  .read(sharedMediaListProvider((chatId, kind)).notifier)
                  .loadMore();
            }
            return false;
          },
          child: kind == SharedMediaTabKind.media
              ? GridView.builder(
                  padding: const EdgeInsets.all(8),
                  gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
                    crossAxisCount: 3,
                    crossAxisSpacing: 4,
                    mainAxisSpacing: 4,
                  ),
                  itemCount: data.items.length,
                  itemBuilder: (context, index) =>
                      _MediaTile(item: data.items[index], chatId: chatId),
                )
              : ListView.builder(
                  padding: const EdgeInsets.symmetric(vertical: 8),
                  itemCount: data.items.length,
                  itemBuilder: (context, index) => _ListTile(
                    item: data.items[index],
                    chatId: chatId,
                  ),
                ),
        );
      },
    );
  }
}

class _MediaTile extends ConsumerWidget {
  const _MediaTile({required this.item, required this.chatId});

  final SharedMediaItemData item;
  final String chatId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final voice = VoiceColors.of(context);
    final fileId = item.fileId;
    if (fileId == null) return const SizedBox.shrink();
    final urlAsync = ref.watch(fileAttachmentUrlProvider(fileId));

    return InkWell(
      key: Key('shared_media_item_${item.messageId}_${item.sortOrder}'),
      onTap: () => _openMessage(context, ref, chatId, item.messageId),
      child: urlAsync.when(
        loading: () => ColoredBox(
          color: voice.muted,
          child: const Center(child: CircularProgressIndicator(strokeWidth: 2)),
        ),
        error: (_, _) => ColoredBox(
          color: voice.muted,
          child: const Icon(Icons.broken_image_outlined),
        ),
        data: (url) {
          if (url == null || url.isEmpty) {
            return ColoredBox(
              color: voice.muted,
              child: const Icon(Icons.image_outlined),
            );
          }
          return Image.network(url, fit: BoxFit.cover);
        },
      ),
    );
  }
}

class _ListTile extends ConsumerWidget {
  const _ListTile({required this.item, required this.chatId});

  final SharedMediaItemData item;
  final String chatId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final title = item.isLink
        ? (item.title?.isNotEmpty == true ? item.title! : item.externalUrl!)
        : (item.originalName ?? item.attachmentType ?? 'file');
    final subtitle = item.isLink ? item.externalUrl : _formatSize(item.sizeBytes);

    return ListTile(
      key: Key('shared_media_item_${item.messageId}_${item.sortOrder}'),
      leading: Icon(
        item.isLink
            ? Icons.link
            : item.attachmentType == 'audio' ||
                  item.attachmentType == 'voice_message'
            ? Icons.mic_outlined
            : Icons.insert_drive_file_outlined,
      ),
      title: Text(title, maxLines: 2, overflow: TextOverflow.ellipsis),
      subtitle: subtitle != null
          ? Text(subtitle, maxLines: 1, overflow: TextOverflow.ellipsis)
          : null,
      onTap: () {
        if (item.isLink) {
          final uri = Uri.tryParse(item.externalUrl ?? '');
          if (uri != null) {
            launchUrl(uri, mode: LaunchMode.externalApplication);
          }
        }
        _openMessage(context, ref, chatId, item.messageId);
      },
      titleTextStyle: theme.textTheme.bodyMedium,
      subtitleTextStyle: theme.textTheme.bodySmall,
    );
  }

  String? _formatSize(int? bytes) {
    if (bytes == null || bytes <= 0) return null;
    if (bytes < 1024) return '$bytes B';
    if (bytes < 1024 * 1024) return '${(bytes / 1024).toStringAsFixed(1)} KB';
    return '${(bytes / (1024 * 1024)).toStringAsFixed(1)} MB';
  }
}

void _openMessage(
  BuildContext context,
  WidgetRef ref,
  String chatId,
  String messageId,
) {
  ref.read(shellNavigationProvider).closeSidePanel();
  Navigator.of(context).maybePop();
  ref.read(pendingChatMessageScrollProvider(chatId).notifier).state =
      messageId;
}

/// Opens chat info in side panel (desktop) or bottom sheet (narrow).
void openChatInfoPanel(
  BuildContext context,
  WidgetRef ref, {
  required String chatId,
  String? groupName,
  bool isGroup = false,
}) {
  final narrow = VoiceLayout.isNarrow(MediaQuery.sizeOf(context).width);
  final l10n = AppLocalizations.of(context)!;
  final body = ChatInfoPanel(
    chatId: chatId,
    groupName: groupName,
    isGroup: isGroup,
  );
  if (narrow) {
    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (ctx) => SafeArea(
        child: SizedBox(
          height: MediaQuery.sizeOf(ctx).height * 0.75,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Padding(
                padding: const EdgeInsets.fromLTRB(16, 12, 8, 8),
                child: Row(
                  children: [
                    Expanded(
                      child: Text(
                        l10n.chatInfoTitle,
                        style: Theme.of(ctx).textTheme.titleMedium,
                      ),
                    ),
                    IconButton(
                      onPressed: () => Navigator.of(ctx).pop(),
                      icon: const Icon(Icons.close),
                    ),
                  ],
                ),
              ),
              Expanded(child: body),
            ],
          ),
        ),
      ),
    );
    return;
  }
  ref.read(shellNavigationProvider).toggleSidePanel(ShellSidePanel.chatInfo);
}
