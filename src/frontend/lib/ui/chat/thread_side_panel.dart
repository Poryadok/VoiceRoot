import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/messages_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../theme/voice_colors.dart';
import 'mention_message_content.dart';

/// Thread replies for a root message (Phase 10).
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
  var _loading = true;
  var _sending = false;
  List<VoiceMessage> _replies = const [];
  String? _error;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  @override
  void dispose() {
    _composer.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    setState(() {
      _loading = true;
      _error = null;
    });
    final result = await ref.read(voiceMessagesClientProvider).getThreadMessages(
      authorization: auth,
      chatId: widget.chatId,
      threadParentId: widget.parentMessageId,
    );
    if (!mounted) return;
    switch (result) {
      case MessagesApiOk(:final data):
        setState(() {
          _replies = data.messages;
          _loading = false;
        });
      case MessagesApiFailure(:final message):
        setState(() {
          _error = message;
          _loading = false;
        });
    }
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
            child: _loading
                ? const Center(child: CircularProgressIndicator())
                : _error != null
                ? Center(child: Text(_error!))
                : _replies.isEmpty
                ? Center(child: Text(l10n.chatThreadEmpty))
                : ListView.builder(
                    padding: const EdgeInsets.all(12),
                    itemCount: _replies.length,
                    itemBuilder: (context, index) {
                      final msg = _replies[index];
                      return Padding(
                        padding: const EdgeInsets.only(bottom: 8),
                        child: MentionMessageContent(content: msg.content),
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
                    child: TextField(
                      controller: _composer,
                      decoration: InputDecoration(
                        hintText: l10n.chatRoomInputHint,
                        isDense: true,
                      ),
                      onSubmitted: _sending ? null : (_) => _send(),
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
