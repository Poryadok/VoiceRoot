import 'package:flutter/material.dart';

import '../../backend/messages_client.dart';
import '../../gen/voice/messaging/v1/messaging.pbenum.dart';
import '../../l10n/app_localizations.dart';
import '../../theme/voice_colors.dart';

String? pinnedMessageContentTypeLabel(
  AppLocalizations l10n,
  MessageContentType? contentType,
) {
  return switch (contentType?.value) {
    2 => l10n.chatPinnedTypePhoto,
    3 => l10n.chatPinnedTypeVideo,
    4 => l10n.chatPinnedTypeFile,
    5 => l10n.chatPinnedTypeVoice,
    6 => l10n.chatPinnedTypeSticker,
    7 => l10n.chatPinnedTypeGif,
    8 => l10n.chatPinnedTypeArticle,
    9 => l10n.chatPinnedTypeLocation,
    10 => l10n.chatPinnedTypeVideoMessage,
    11 => l10n.chatPinnedTypeMusic,
    _ => null,
  };
}

class PinnedMessagesPanel extends StatefulWidget {
  const PinnedMessagesPanel({
    super.key,
    required this.messages,
    required this.onOpenMessage,
    this.onCancel,
    this.onMessageOpened,
  });

  static const Key panelKey = Key('chat_pinned_messages_panel');
  static const Key closeKey = Key('chat_pinned_messages_close');

  final List<VoiceMessage> messages;
  final Future<bool> Function(String messageId) onOpenMessage;
  final VoidCallback? onCancel;
  final VoidCallback? onMessageOpened;

  static Future<void> show(
    BuildContext context, {
    required List<VoiceMessage> messages,
    required Future<bool> Function(String messageId) onOpenMessage,
    VoidCallback? onCancel,
    VoidCallback? onMessageOpened,
  }) {
    return showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (_) => SafeArea(
        child: PinnedMessagesPanel(
          messages: messages,
          onOpenMessage: onOpenMessage,
          onCancel: onCancel,
          onMessageOpened: onMessageOpened,
        ),
      ),
    );
  }

  @override
  State<PinnedMessagesPanel> createState() => _PinnedMessagesPanelState();
}

class _PinnedMessagesPanelState extends State<PinnedMessagesPanel> {
  String? _openingMessageId;
  String? _failedMessageId;
  var _openedMessage = false;

  @override
  void dispose() {
    if (!_openedMessage) widget.onCancel?.call();
    super.dispose();
  }

  Future<void> _openMessage(String messageId) async {
    if (_openingMessageId != null) return;
    setState(() {
      _openingMessageId = messageId;
      _failedMessageId = null;
    });
    var opened = false;
    try {
      opened = await widget.onOpenMessage(messageId);
    } catch (_) {
      opened = false;
    }
    if (!mounted) return;
    setState(() {
      _openingMessageId = null;
      _failedMessageId = opened ? null : messageId;
    });
    if (opened) {
      _openedMessage = true;
      Navigator.of(context).pop();
      widget.onMessageOpened?.call();
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    return ConstrainedBox(
      constraints: BoxConstraints(
        maxHeight: MediaQuery.sizeOf(context).height * 0.75,
        maxWidth: 560,
      ),
      child: Material(
        key: PinnedMessagesPanel.panelKey,
        color: voice.surface,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Padding(
              padding: const EdgeInsetsDirectional.only(
                start: 16,
                top: 12,
                end: 8,
                bottom: 8,
              ),
              child: Row(
                children: [
                  Expanded(
                    child: Text(
                      l10n.chatPinnedMessagesTitle,
                      style: Theme.of(context).textTheme.titleMedium,
                    ),
                  ),
                  IconButton(
                    key: PinnedMessagesPanel.closeKey,
                    tooltip: MaterialLocalizations.of(
                      context,
                    ).closeButtonTooltip,
                    onPressed: () {
                      widget.onCancel?.call();
                      Navigator.of(context).pop();
                    },
                    icon: const Icon(Icons.close),
                  ),
                ],
              ),
            ),
            Flexible(
              fit: FlexFit.loose,
              child: ListView.separated(
                shrinkWrap: true,
                itemCount:
                    widget.messages.length + (_failedMessageId == null ? 0 : 1),
                separatorBuilder: (_, _) =>
                    Divider(height: 1, color: voice.borderDefault, indent: 56),
                itemBuilder: (context, index) {
                  if (index == widget.messages.length) {
                    return ListTile(
                      key: const Key('chat_pinned_jump_error'),
                      leading: const Icon(Icons.cloud_off_outlined),
                      title: Text(l10n.commonActionFailed),
                      trailing: TextButton.icon(
                        key: const Key('chat_pinned_jump_retry'),
                        onPressed: () => _openMessage(_failedMessageId!),
                        icon: const Icon(Icons.refresh),
                        label: Text(l10n.commonRetry),
                      ),
                    );
                  }
                  final message = widget.messages[index];
                  final opening = _openingMessageId == message.id;
                  final typeLabel = pinnedMessageContentTypeLabel(
                    l10n,
                    message.contentType,
                  );
                  final preview = message.content.trim();
                  return ListTile(
                    key: ValueKey('chat_pinned_message_${message.id}'),
                    leading: Icon(
                      Icons.push_pin_outlined,
                      color: voice.textSecondary,
                    ),
                    title: Text(
                      preview.isNotEmpty
                          ? preview
                          : (typeLabel ?? l10n.chatPinnedMessagesTitle),
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                    subtitle: typeLabel == null ? null : Text(typeLabel),
                    trailing: opening
                        ? const SizedBox(
                            width: 20,
                            height: 20,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Icon(Icons.arrow_forward),
                    onTap: _openingMessageId == null
                        ? () => _openMessage(message.id)
                        : null,
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
