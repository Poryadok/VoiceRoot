import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../backend/messages_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/chat_providers.dart';
import '../../state/e2e_providers.dart';
import '../../state/shared_media_providers.dart';
import '../../state/shell_providers.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_layout.dart';
import '../core/voice_state_panel.dart';
import 'group_members_sheet.dart';
import '../space/space_chat_override_sheet.dart';
import '../../backend/space_permissions.dart';
import '../../backend/bots_client.dart';
import '../../state/auth_providers.dart';
import '../../state/bot_providers.dart';
import '../../state/space_providers.dart';
import 'e2e_attachment_actions.dart';
import 'e2e_chat_settings.dart';

/// Chat info with shared media tabs (app stack0).
class ChatInfoPanel extends ConsumerStatefulWidget {
  const ChatInfoPanel({
    super.key,
    required this.chatId,
    this.groupName,
    this.isGroup = false,
  });

  static const Key panelKey = Key('chat_info_panel');
  static const Key e2eToggleKey = Key('chat_info_e2e_toggle');
  static const Key mediaTabKey = Key('chat_info_tab_media');
  static const Key filesTabKey = Key('chat_info_tab_files');
  static const Key linksTabKey = Key('chat_info_tab_links');
  static const Key voiceTabKey = Key('chat_info_tab_voice');
  static const Key e2eVideoTileKey = Key('chat_info_e2e_video_tile');

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
    final spaceId = _spaceIdForChat(ref, widget.chatId);

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
        if (spaceId != null)
          _ChatOverrideBar(spaceId: spaceId, chatId: widget.chatId),
        if (spaceId != null && widget.isGroup)
          ChatBotsSettingsSection(
            chatId: widget.chatId,
            spaceId: spaceId,
          ),
        if (!widget.isGroup) DmE2eSettingsSection(chatId: widget.chatId),
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
    if (item.isE2eEncrypted) {
      if (item.isVideo) {
        return InkWell(
          key: ChatInfoPanel.e2eVideoTileKey,
          onTap: () => _openMessage(context, ref, chatId, item.messageId),
          child: ColoredBox(
            color: voice.muted,
            child: Center(
              child: Icon(Icons.videocam_outlined, color: voice.textSecondary),
            ),
          ),
        );
      }
      final decryptRequest = E2eAttachmentDecryptRequest(
        fileId: fileId,
        e2eKeyWire: item.e2eKeyWire!,
        senderProfileId: item.senderProfileId,
        chatId: chatId,
      );
      final bytesAsync =
          ref.watch(e2eDecryptedAttachmentThumbProvider(decryptRequest));
      return InkWell(
        key: Key('shared_media_item_${item.messageId}_${item.sortOrder}'),
        onTap: () => _openMessage(context, ref, chatId, item.messageId),
        child: ColoredBox(
          color: voice.muted,
          child: bytesAsync.isLoading || bytesAsync.valueOrNull == null
              ? Center(
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    color: voice.textSecondary,
                  ),
                )
              : Image.memory(bytesAsync.valueOrNull!, fit: BoxFit.cover),
        ),
      );
    }
    final e2eChat = ref.watch(chatE2eEnabledProvider(chatId));
    if (e2eChat) {
      return InkWell(
        key: Key('shared_media_item_${item.messageId}_${item.sortOrder}'),
        onTap: () => _openMessage(context, ref, chatId, item.messageId),
        child: ColoredBox(
          color: voice.muted,
          child: Icon(Icons.lock_outline, color: voice.textSecondary),
        ),
      );
    }
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
    final voice = VoiceColors.of(context);
    final l10n = AppLocalizations.of(context)!;
    final e2eChat = ref.watch(chatE2eEnabledProvider(chatId));
    final title = item.isLink
        ? (item.title?.isNotEmpty == true ? item.title! : item.externalUrl!)
        : (item.originalName ?? item.attachmentType ?? 'file');
    final subtitle = item.isLink ? item.externalUrl : _formatSize(item.sizeBytes);
    final isLockedE2eFile = e2eChat && !item.isLink && !item.isE2eEncrypted;

    return ListTile(
      key: Key('shared_media_item_${item.messageId}_${item.sortOrder}'),
      leading: Icon(
        item.isLink
            ? Icons.link
            : item.isE2eEncrypted
            ? Icons.download_outlined
            : isLockedE2eFile
            ? Icons.lock_outline
            : item.attachmentType == 'audio' ||
                  item.attachmentType == 'voice_message'
            ? Icons.mic_outlined
            : Icons.insert_drive_file_outlined,
        color: isLockedE2eFile ? voice.textSecondary : null,
      ),
      title: Text(title, maxLines: 2, overflow: TextOverflow.ellipsis),
      subtitle: subtitle != null
          ? Text(subtitle, maxLines: 1, overflow: TextOverflow.ellipsis)
          : item.isE2eEncrypted
          ? Text(
              l10n.e2eAttachmentTapToDownload,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            )
          : null,
      onTap: () {
        if (item.isLink) {
          final uri = Uri.tryParse(item.externalUrl ?? '');
          if (uri != null) {
            launchUrl(uri, mode: LaunchMode.externalApplication);
          }
        } else if (item.isE2eEncrypted && item.fileId != null) {
          unawaited(
            _downloadSharedE2eFile(
              context,
              ref,
              chatId: chatId,
              item: item,
              fileName: title,
            ),
          );
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

Future<void> _downloadSharedE2eFile(
  BuildContext context,
  WidgetRef ref, {
  required String chatId,
  required SharedMediaItemData item,
  required String fileName,
}) async {
  final l10n = AppLocalizations.of(context)!;
  final fileId = item.fileId;
  final keyWire = item.e2eKeyWire;
  if (fileId == null || keyWire == null || keyWire.isEmpty) return;
  final decryptRequest = E2eAttachmentDecryptRequest(
    fileId: fileId,
    e2eKeyWire: keyWire,
    senderProfileId: item.senderProfileId,
    chatId: chatId,
  );
  final bytes =
      await ref.read(e2eDecryptedAttachmentBytesProvider(decryptRequest).future);
  if (!context.mounted) return;
  if (bytes == null) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(l10n.e2eAttachmentDecryptFailed)),
    );
    return;
  }
  final saved = await saveDecryptedE2eAttachment(
    bytes: bytes,
    fileName: fileName,
  );
  if (!context.mounted) return;
  if (!saved) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(l10n.e2eAttachmentDownloadFailed)),
    );
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

class _ChatOverrideBar extends ConsumerWidget {
  const _ChatOverrideBar({required this.spaceId, required this.chatId});

  final String spaceId;
  final String chatId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final canManage = ref
            .watch(
              spacePermissionProvider((
                spaceId: spaceId,
                permission: SpacePermissions.spaceManageRoles,
                chatId: null,
                voiceRoomId: null,
              )),
            )
            .valueOrNull ??
        false;
    if (!canManage) return const SizedBox.shrink();
    return Align(
      alignment: Alignment.centerRight,
      child: TextButton.icon(
        key: const Key('chat_info_role_overrides'),
        onPressed: () => SpaceChatOverrideSheet.show(
          context,
          spaceId: spaceId,
          chatId: chatId,
        ),
        icon: const Icon(Icons.admin_panel_settings_outlined, size: 18),
        label: Text(l10n.spaceChatOverrideTitle),
      ),
    );
  }
}

String? _spaceIdForChat(WidgetRef ref, String chatId) {
  for (final item in ref.watch(chatListControllerProvider).items) {
    if (item.chatId == chatId) return item.chat.spaceId;
  }
  return null;
}

/// Per-chat bot enable toggles — docs/features/bots.md.
class ChatBotsSettingsSection extends ConsumerWidget {
  const ChatBotsSettingsSection({
    super.key,
    required this.chatId,
    required this.spaceId,
  });

  static const Key sectionKey = Key('chat_info_bots_section');

  final String chatId;
  final String spaceId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final canManage = ref
            .watch(
              spacePermissionProvider((
                spaceId: spaceId,
                permission: SpacePermissions.spaceManageBots,
                chatId: null,
                voiceRoomId: null,
              )),
            )
            .valueOrNull ??
        false;
    if (!canManage) return const SizedBox.shrink();

    final botsAsync = ref.watch(
      botsInChatProvider((chatId: chatId, spaceId: spaceId)),
    );

    return Column(
      key: sectionKey,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Divider(height: 1, color: voice.borderDefault),
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 12, 16, 4),
          child: Text(
            l10n.chatBotsSectionTitle,
            style: Theme.of(context).textTheme.titleMedium,
          ),
        ),
        botsAsync.when(
          loading: () => const Padding(
            padding: EdgeInsets.all(16),
            child: Center(child: CircularProgressIndicator()),
          ),
          error: (error, _) => Padding(
            padding: const EdgeInsets.all(16),
            child: VoiceStatePanel(
              title: l10n.chatBotsSectionTitle,
              message: l10n.chatBotsLoadError,
              icon: Icons.cloud_off_outlined,
              actionLabel: l10n.commonRetry,
              onAction: () => ref.invalidate(
                botsInChatProvider((chatId: chatId, spaceId: spaceId)),
              ),
            ),
          ),
          data: (bots) {
            if (bots.isEmpty) {
              return Padding(
                padding: const EdgeInsets.all(16),
                child: Text(l10n.chatBotsEmpty),
              );
            }
            return Column(
              children: [
                for (final entry in bots)
                  SwitchListTile(
                    key: Key('chat_bot_toggle_${entry.bot.id}'),
                    title: Text(entry.bot.name),
                    subtitle: entry.whitelisted
                        ? null
                        : Text(l10n.spaceBotsSelectChats),
                    value: entry.enabled && entry.whitelisted,
                    onChanged: (enabled) async {
                      final auth = ref.read(authorizationHeaderProvider);
                      if (auth == null) return;
                      final chatType = ref.read(chatTypeForChatProvider(chatId)) ??
                          'CHAT_TYPE_CHANNEL';
                      final result = await ref
                          .read(voiceBotsClientProvider)
                          .setBotChatEnabled(
                            authorization: auth,
                            botId: entry.bot.id,
                            chatId: chatId,
                            chatType: chatType,
                            spaceId: spaceId,
                            enabled: enabled,
                          );
                      if (!context.mounted) return;
                      switch (result) {
                        case BotsApiOk():
                          ref.invalidate(
                            botsInChatProvider((chatId: chatId, spaceId: spaceId)),
                          );
                          ref.invalidate(slashCommandsForChatProvider(chatId));
                        case BotsApiFailure(:final message):
                          ScaffoldMessenger.of(context).showSnackBar(
                            SnackBar(content: Text(l10n.chatRoomError(message))),
                          );
                      }
                    },
                  ),
              ],
            );
          },
        ),
      ],
    );
  }
}
