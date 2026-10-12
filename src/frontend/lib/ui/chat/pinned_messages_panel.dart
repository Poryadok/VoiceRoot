import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/messages_client.dart';
import '../../gen/voice/messaging/v1/messaging.pbenum.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/group_members_management_providers.dart';
import '../../state/space_providers.dart';
import '../../theme/voice_colors.dart';
import '../api_error_messages.dart';

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

String pinMutationErrorText(AppLocalizations l10n, PinMutationResult result) {
  if (result.pinLimitReached) return l10n.chatPinnedLimitReached;
  if (result.permissionDenied) {
    return l10n.spacePermissionDeniedGeneric('TEXT_CHAT_PIN_MESSAGES');
  }
  return commonActionErrorMessage(l10n, statusCode: result.statusCode);
}

void refreshPinManagementPermission(
  WidgetRef ref, {
  required String chatId,
  required bool isGroup,
  String? spaceId,
}) {
  if (spaceId != null && spaceId.isNotEmpty) {
    ref.invalidate(
      spacePermissionProvider((
        spaceId: spaceId,
        permission: 'TEXT_CHAT_PIN_MESSAGES',
        chatId: chatId,
        voiceRoomId: null,
      )),
    );
  } else if (isGroup) {
    ref.read(groupMembersManagementProvider(chatId).notifier).load();
  }
}

enum _PinManagementAccess { allowed, denied, loading, failed }

class PinnedMessagesPanel extends ConsumerStatefulWidget {
  const PinnedMessagesPanel({
    super.key,
    this.chatId = '',
    this.spaceId,
    this.isGroup = false,
    required this.messages,
    required this.onOpenMessage,
    this.onUnpin,
    this.onCancel,
    this.onMessageOpened,
  });

  static const Key panelKey = Key('chat_pinned_messages_panel');
  static const Key closeKey = Key('chat_pinned_messages_close');

  final String chatId;
  final String? spaceId;
  final bool isGroup;
  final List<VoiceMessage> messages;
  final Future<bool> Function(String messageId) onOpenMessage;
  final Future<PinMutationResult> Function(String messageId)? onUnpin;
  final VoidCallback? onCancel;
  final VoidCallback? onMessageOpened;

  static Future<void> show(
    BuildContext context, {
    String chatId = '',
    String? spaceId,
    bool isGroup = false,
    required List<VoiceMessage> messages,
    required Future<bool> Function(String messageId) onOpenMessage,
    Future<PinMutationResult> Function(String messageId)? onUnpin,
    VoidCallback? onCancel,
    VoidCallback? onMessageOpened,
  }) {
    return showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (_) => SafeArea(
        child: PinnedMessagesPanel(
          chatId: chatId,
          spaceId: spaceId,
          isGroup: isGroup,
          messages: messages,
          onOpenMessage: onOpenMessage,
          onUnpin: onUnpin,
          onCancel: onCancel,
          onMessageOpened: onMessageOpened,
        ),
      ),
    );
  }

  @override
  ConsumerState<PinnedMessagesPanel> createState() =>
      _PinnedMessagesPanelState();
}

class _PinnedMessagesPanelState extends ConsumerState<PinnedMessagesPanel> {
  String? _openingMessageId;
  String? _failedMessageId;
  var _openedMessage = false;
  late List<VoiceMessage> _messages;
  final Set<String> _unpinningMessageIds = {};
  final Map<String, PinMutationResult> _unpinErrors = {};

  @override
  void initState() {
    super.initState();
    _messages = List<VoiceMessage>.of(widget.messages);
  }

  @override
  void didUpdateWidget(covariant PinnedMessagesPanel oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (!listEquals(oldWidget.messages, widget.messages)) {
      _messages = List<VoiceMessage>.of(widget.messages);
    }
  }

  @override
  void dispose() {
    if (!_openedMessage) widget.onCancel?.call();
    super.dispose();
  }

  _PinManagementAccess _pinAccess() {
    final auth = ref.watch(authControllerProvider);
    final profileId = auth.activeProfileId;
    if (auth.session == null || profileId == null || profileId.isEmpty) {
      return _PinManagementAccess.denied;
    }
    final spaceId = widget.spaceId;
    if (spaceId != null && spaceId.isNotEmpty) {
      final query = (
        spaceId: spaceId,
        permission: 'TEXT_CHAT_PIN_MESSAGES',
        chatId: widget.chatId,
        voiceRoomId: null,
      );
      final permission = ref.watch(spacePermissionProvider(query));
      if (permission.isLoading) return _PinManagementAccess.loading;
      if (permission.hasError) return _PinManagementAccess.failed;
      return permission.valueOrNull == true
          ? _PinManagementAccess.allowed
          : _PinManagementAccess.denied;
    }
    if (widget.isGroup) {
      final members = ref.watch(groupMembersManagementProvider(widget.chatId));
      if (members.isLoading) return _PinManagementAccess.loading;
      if (members.error != null) return _PinManagementAccess.failed;
      final actor = members.members
          .where((member) => member.profileId == profileId)
          .toList(growable: false);
      if (actor.length != 1) return _PinManagementAccess.denied;
      return switch (actor.single.role) {
        'owner' || 'admin' => _PinManagementAccess.allowed,
        _ => _PinManagementAccess.denied,
      };
    }
    return _PinManagementAccess.allowed;
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

  Future<void> _unpinMessage(String messageId) async {
    final onUnpin = widget.onUnpin;
    if (onUnpin == null ||
        _unpinningMessageIds.contains(messageId) ||
        _pinAccess() != _PinManagementAccess.allowed) {
      return;
    }
    setState(() {
      _unpinningMessageIds.add(messageId);
      _unpinErrors.remove(messageId);
    });
    PinMutationResult result;
    try {
      result = await onUnpin(messageId);
    } catch (_) {
      result = const PinMutationResult.failure(message: 'unknown_error');
    }
    if (!mounted) return;
    if (result.stale) {
      setState(() => _unpinningMessageIds.remove(messageId));
      return;
    }
    if (result.permissionDenied) {
      refreshPinManagementPermission(
        ref,
        chatId: widget.chatId,
        isGroup: widget.isGroup,
        spaceId: widget.spaceId,
      );
    }
    setState(() {
      _unpinningMessageIds.remove(messageId);
      if (result.succeeded) {
        _messages.removeWhere((message) => message.id == messageId);
        _unpinErrors.remove(messageId);
      } else {
        _unpinErrors[messageId] = result;
      }
    });
  }

  Future<void> _retryPermission() async {
    if (widget.chatId.isEmpty) return;
    refreshPinManagementPermission(
      ref,
      chatId: widget.chatId,
      isGroup: widget.isGroup,
      spaceId: widget.spaceId,
    );
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final access = widget.onUnpin == null
        ? _PinManagementAccess.denied
        : _pinAccess();
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
            if (access == _PinManagementAccess.loading)
              const LinearProgressIndicator(
                key: Key('chat_pinned_permission_loading'),
                minHeight: 2,
              ),
            if (access == _PinManagementAccess.failed)
              ListTile(
                key: const Key('chat_pinned_permission_error'),
                leading: const Icon(Icons.cloud_off_outlined),
                title: Text(l10n.commonActionFailed),
                trailing: IconButton(
                  tooltip: l10n.commonRetry,
                  onPressed: _retryPermission,
                  icon: const Icon(Icons.refresh),
                ),
              ),
            Flexible(
              fit: FlexFit.loose,
              child: ListView.separated(
                shrinkWrap: true,
                itemCount:
                    _messages.length + (_failedMessageId == null ? 0 : 1),
                separatorBuilder: (_, _) =>
                    Divider(height: 1, color: voice.borderDefault, indent: 56),
                itemBuilder: (context, index) {
                  if (index == _messages.length) {
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
                  final message = _messages[index];
                  final opening = _openingMessageId == message.id;
                  final unpinning = _unpinningMessageIds.contains(message.id);
                  final typeLabel = pinnedMessageContentTypeLabel(
                    l10n,
                    message.contentType,
                  );
                  final preview = message.content.trim();
                  final unpinError = _unpinErrors[message.id];
                  return Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      ListTile(
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
                        trailing: Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            if (unpinning)
                              const SizedBox(
                                key: Key('chat_pinned_unpin_busy'),
                                width: 20,
                                height: 20,
                                child: CircularProgressIndicator(
                                  strokeWidth: 2,
                                ),
                              )
                            else if (access == _PinManagementAccess.allowed &&
                                widget.onUnpin != null)
                              IconButton(
                                key: ValueKey(
                                  'chat_pinned_unpin_${message.id}',
                                ),
                                tooltip: l10n.chatMessageUnpin,
                                onPressed: () => _unpinMessage(message.id),
                                icon: const Icon(Icons.push_pin),
                              ),
                            const SizedBox(width: 4),
                            opening
                                ? const SizedBox(
                                    width: 20,
                                    height: 20,
                                    child: CircularProgressIndicator(
                                      strokeWidth: 2,
                                    ),
                                  )
                                : const Icon(Icons.arrow_forward),
                          ],
                        ),
                        onTap: _openingMessageId == null
                            ? () => _openMessage(message.id)
                            : null,
                      ),
                      if (unpinError != null)
                        Padding(
                          padding: const EdgeInsetsDirectional.only(
                            start: 56,
                            end: 12,
                            bottom: 8,
                          ),
                          child: Row(
                            children: [
                              Expanded(
                                child: Text(
                                  pinMutationErrorText(l10n, unpinError),
                                  key: ValueKey(
                                    'chat_pinned_unpin_error_${message.id}',
                                  ),
                                  style: Theme.of(context).textTheme.bodySmall
                                      ?.copyWith(color: voice.error),
                                ),
                              ),
                              if (access == _PinManagementAccess.allowed)
                                TextButton(
                                  onPressed: () => _unpinMessage(message.id),
                                  child: Text(l10n.commonRetry),
                                ),
                            ],
                          ),
                        ),
                    ],
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
