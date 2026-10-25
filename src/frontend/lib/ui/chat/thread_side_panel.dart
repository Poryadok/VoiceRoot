import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/api_errors.dart' show isBackendUnavailable;
import '../../backend/messages_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../theme/voice_colors.dart';
import '../api_error_messages.dart';
import '../core/voice_skeleton.dart';
import '../core/voice_state_panel.dart';
import 'chat_composer_text_field.dart';
import 'mention_message_content.dart';
import 'message_view_count.dart';

/// Thread replies for a root message (roles/threads (docs/features/roles.md)).
class ThreadSidePanel extends ConsumerStatefulWidget {
  const ThreadSidePanel({
    super.key,
    required this.chatId,
    required this.parentMessageId,
    required this.parentPreview,
    this.onClose,
  });

  final String chatId;
  final String parentMessageId;
  final String parentPreview;
  final VoidCallback? onClose;

  @override
  ConsumerState<ThreadSidePanel> createState() => _ThreadSidePanelState();
}

class _ThreadSidePanelState extends ConsumerState<ThreadSidePanel> {
  final _composer = TextEditingController();
  final _composerFocus = FocusNode();
  var _loading = true;
  var _sending = false;
  List<VoiceMessage> _replies = const [];
  MessagesApiFailure? _error;
  String? _historyProfileId;
  int _loadGeneration = 0;
  final Set<String> _recordedViews = {};

  @override
  void didUpdateWidget(covariant ThreadSidePanel oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.chatId != widget.chatId ||
        oldWidget.parentMessageId != widget.parentMessageId) {
      _loadGeneration++;
      _historyProfileId = null;
      _replies = const [];
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) _load();
      });
    }
  }

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  @override
  void dispose() {
    _composer.dispose();
    _composerFocus.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    final auth = ref.read(authorizationHeaderProvider);
    final profileId = ref.read(authControllerProvider).activeProfileId;
    if (auth == null || profileId == null) return;
    final generation = ++_loadGeneration;
    setState(() {
      _loading = true;
      _error = null;
    });
    final result = await ref
        .read(voiceMessagesClientProvider)
        .getThreadMessages(
          authorization: auth,
          chatId: widget.chatId,
          threadParentId: widget.parentMessageId,
        );
    if (!mounted ||
        generation != _loadGeneration ||
        ref.read(authControllerProvider).activeProfileId != profileId ||
        ref.read(authorizationHeaderProvider) != auth) {
      return;
    }
    switch (result) {
      case MessagesApiOk(:final data):
        setState(() {
          _replies = data.messages;
          _historyProfileId = profileId;
          _loading = false;
        });
      case final failure as MessagesApiFailure:
        setState(() {
          _error = failure;
          _historyProfileId = profileId;
          _loading = false;
        });
    }
  }

  Future<bool> _recordView(VoiceMessage message) async {
    final profileId = ref.read(authControllerProvider).activeProfileId;
    final auth = ref.read(authorizationHeaderProvider);
    final generation = _loadGeneration;
    if (profileId == null || auth == null || profileId != _historyProfileId) {
      return false;
    }
    final key = '$profileId:${message.id}';
    if (_recordedViews.contains(key)) return true;
    final result = await ref
        .read(voiceMessagesClientProvider)
        .recordMessageView(
          authorization: auth,
          chatId: widget.chatId,
          messageId: message.id,
        );
    if (!mounted ||
        generation != _loadGeneration ||
        ref.read(authControllerProvider).activeProfileId != profileId ||
        ref.read(authorizationHeaderProvider) != auth) {
      return false;
    }
    if (result is! MessagesApiOk<int>) return false;
    _recordedViews.add(key);
    setState(() {
      _replies = [
        for (final reply in _replies)
          if (reply.id == message.id)
            reply.copyWith(viewCount: result.data)
          else
            reply,
      ];
    });
    return true;
  }

  String? _loadErrorMessage(AppLocalizations l10n) {
    final failure = _error!;
    if (failure.errorCode == 'permission_denied') {
      return chatRoomErrorMessage(
        l10n,
        'permission_denied',
        statusCode: failure.statusCode,
      );
    }
    if (isBackendUnavailable(failure.statusCode)) {
      return chatThreadErrorMessage(l10n, '', statusCode: failure.statusCode);
    }
    return null;
  }

  Future<void> _send() async {
    final text = _composer.text.trim();
    if (text.isEmpty || _sending) return;
    setState(() => _sending = true);
    final err = await ref
        .read(chatRoomControllerProvider(widget.chatId).notifier)
        .sendMessage(text, threadParentId: widget.parentMessageId);
    if (!mounted) return;
    setState(() => _sending = false);
    if (err == null) {
      _composer.clear();
      ref.read(chatReplyTargetProvider(widget.chatId).notifier).state = null;
      await _load();
    }
  }

  @override
  Widget build(BuildContext context) {
    final profileId = ref.watch(authControllerProvider).activeProfileId;
    ref.listen<AuthState>(authControllerProvider, (previous, next) {
      if (previous?.activeProfileId != next.activeProfileId ||
          previous?.session?.accountId != next.session?.accountId) {
        _loadGeneration++;
        setState(() {
          _historyProfileId = null;
          _replies = const [];
          _loading = true;
        });
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (mounted) _load();
        });
      }
    });
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    return Material(
      color: voice.surface,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(8, 8, 4, 4),
            child: Row(
              children: [
                Expanded(
                  child: Text(
                    l10n.chatThreadTitle,
                    style: Theme.of(context).textTheme.titleSmall,
                  ),
                ),
                IconButton(
                  onPressed: widget.onClose,
                  icon: const Icon(Icons.close),
                ),
              ],
            ),
          ),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 12),
            child: Text(
              widget.parentPreview,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
              style: Theme.of(context).textTheme.bodySmall,
            ),
          ),
          const Divider(height: 1),
          Expanded(
            child: _loading || _historyProfileId != profileId
                ? const VoiceListSkeleton()
                : _error != null
                ? VoiceStatePanel(
                    title: l10n.chatThreadLoadError,
                    message: _loadErrorMessage(l10n),
                    icon: Icons.cloud_off_outlined,
                    actionLabel: l10n.commonRetry,
                    onAction: _load,
                  )
                : _replies.isEmpty
                ? VoiceStatePanel(
                    title: l10n.chatThreadEmpty,
                    icon: Icons.forum_outlined,
                  )
                : ListView.builder(
                    padding: const EdgeInsets.all(12),
                    itemCount: _replies.length,
                    itemBuilder: (context, index) {
                      final msg = _replies[index];
                      final content = Padding(
                        padding: const EdgeInsets.only(bottom: 8),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            MentionMessageContent(content: msg.content),
                            if (msg.viewCount != null)
                              MessageViewCount(count: msg.viewCount!),
                          ],
                        ),
                      );
                      if (msg.viewCount == null) return content;
                      return MessageViewportView(
                        key: ValueKey('thread-view:$profileId:${msg.id}'),
                        onVisible: () => _recordView(msg),
                        child: content,
                      );
                    },
                  ),
          ),
          SafeArea(
            top: false,
            child: Padding(
              padding: const EdgeInsets.fromLTRB(12, 4, 12, 8),
              child: Row(
                children: [
                  Expanded(
                    child: ChatComposerTextField(
                      controller: _composer,
                      focusNode: _composerFocus,
                      decoration: InputDecoration(
                        hintText: l10n.chatRoomInputHint,
                        isDense: true,
                      ),
                      onSend: _sending ? null : _send,
                    ),
                  ),
                  const SizedBox(width: 8),
                  IconButton(
                    onPressed: _sending ? null : _send,
                    icon: _sending
                        ? const SizedBox(
                            width: 20,
                            height: 20,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Icon(Icons.send),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}
