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

class PinnedMessagesPanel extends StatelessWidget {
  const PinnedMessagesPanel({
    super.key,
    required this.messages,
    required this.onOpenMessage,
  });

  static const Key panelKey = Key('chat_pinned_messages_panel');
  static const Key closeKey = Key('chat_pinned_messages_close');

  final List<VoiceMessage> messages;
  final ValueChanged<String> onOpenMessage;

  static Future<void> show(
    BuildContext context, {
    required List<VoiceMessage> messages,
    required ValueChanged<String> onOpenMessage,
  }) {
    return showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (sheetContext) => SafeArea(
        child: PinnedMessagesPanel(
          messages: messages,
          onOpenMessage: (messageId) {
            Navigator.of(sheetContext).pop();
            onOpenMessage(messageId);
          },
        ),
      ),
    );
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
        key: panelKey,
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
                    key: closeKey,
                    tooltip: MaterialLocalizations.of(
                      context,
                    ).closeButtonTooltip,
                    onPressed: () => Navigator.of(context).pop(),
                    icon: const Icon(Icons.close),
                  ),
                ],
              ),
            ),
            Flexible(
              fit: FlexFit.loose,
              child: ListView.separated(
                shrinkWrap: true,
                itemCount: messages.length,
                separatorBuilder: (_, _) =>
                    Divider(height: 1, color: voice.borderDefault, indent: 56),
                itemBuilder: (context, index) {
                  final message = messages[index];
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
                    trailing: const Icon(Icons.arrow_forward),
                    onTap: () => onOpenMessage(message.id),
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
