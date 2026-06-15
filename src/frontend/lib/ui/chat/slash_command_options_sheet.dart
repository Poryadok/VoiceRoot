import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/bots_client.dart';
import '../../state/auth_providers.dart';
import '../../state/bot_providers.dart';
import '../../theme/voice_colors.dart';

/// Collects slash command option values; supports autocomplete for string options.
Future<Map<String, dynamic>?> showSlashCommandOptionsSheet({
  required BuildContext context,
  required WidgetRef ref,
  required String chatId,
  required BotSlashCommand command,
}) {
  return showModalBottomSheet<Map<String, dynamic>>(
    context: context,
    isScrollControlled: true,
    builder: (ctx) => _SlashCommandOptionsSheet(
      chatId: chatId,
      command: command,
    ),
  );
}

class _SlashCommandOptionsSheet extends ConsumerStatefulWidget {
  const _SlashCommandOptionsSheet({
    required this.chatId,
    required this.command,
  });

  final String chatId;
  final BotSlashCommand command;

  @override
  ConsumerState<_SlashCommandOptionsSheet> createState() =>
      _SlashCommandOptionsSheetState();
}

class _SlashCommandOptionsSheetState
    extends ConsumerState<_SlashCommandOptionsSheet> {
  final _values = <String, String>{};
  final _controllers = <String, TextEditingController>{};
  final _suggestions = <String, List<BotAutocompleteChoice>>{};
  Timer? _debounce;

  @override
  void dispose() {
    _debounce?.cancel();
    for (final c in _controllers.values) {
      c.dispose();
    }
    super.dispose();
  }

  TextEditingController _controllerFor(String name) {
    return _controllers.putIfAbsent(name, TextEditingController.new);
  }

  Future<void> _fetchAutocomplete(String optionName, String focused) async {
    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    final chatType =
        ref.read(chatTypeForChatProvider(widget.chatId)) ?? 'CHAT_TYPE_CHANNEL';
    final selected = Map<String, dynamic>.from(_values);
    final result = await ref.read(voiceBotsClientProvider).autocompleteOption(
      authorization: auth,
      chatId: widget.chatId,
      chatType: chatType,
      botId: widget.command.botId,
      commandName: widget.command.fullCommandName,
      optionName: optionName,
      focusedValue: focused,
      optionsJson: jsonEncode(selected),
    );
    if (!mounted) return;
    setState(() {
      _suggestions[optionName] = switch (result) {
        BotsApiOk(:final data) => data,
        BotsApiFailure() => const [],
      };
    });
  }

  void _onOptionChanged(BotSlashCommandOption option, String value) {
    _values[option.name] = value;
    if (!option.autocomplete) return;
    _debounce?.cancel();
    _debounce = Timer(const Duration(milliseconds: 250), () {
      _fetchAutocomplete(option.name, value);
    });
  }

  bool get _canSubmit {
    for (final opt in widget.command.options) {
      if (opt.required && (_values[opt.name]?.trim().isEmpty ?? true)) {
        return false;
      }
    }
    return true;
  }

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    return SafeArea(
      child: Padding(
        padding: EdgeInsets.only(
          left: 16,
          right: 16,
          top: 16,
          bottom: MediaQuery.viewInsetsOf(context).bottom + 16,
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              widget.command.displayName,
              style: Theme.of(context).textTheme.titleMedium,
            ),
            const SizedBox(height: 12),
            for (final opt in widget.command.options) ...[
              TextField(
                controller: _controllerFor(opt.name),
                decoration: InputDecoration(
                  labelText: opt.name + (opt.required ? ' *' : ''),
                ),
                onChanged: (v) => _onOptionChanged(opt, v),
              ),
              if (opt.autocomplete &&
                  (_suggestions[opt.name]?.isNotEmpty ?? false))
                Wrap(
                  spacing: 8,
                  children: [
                    for (final choice in _suggestions[opt.name]!.take(25))
                      ActionChip(
                        label: Text(choice.name),
                        onPressed: () {
                          _controllerFor(opt.name).text = choice.value;
                          _values[opt.name] = choice.value;
                          setState(() => _suggestions[opt.name] = const []);
                        },
                      ),
                  ],
                ),
              const SizedBox(height: 8),
            ],
            FilledButton(
              style: FilledButton.styleFrom(
                backgroundColor: voice.profileAccent,
              ),
              onPressed: _canSubmit
                  ? () => Navigator.pop(context, Map<String, dynamic>.from(_values))
                  : null,
              child: const Text('Run command'),
            ),
          ],
        ),
      ),
    );
  }
}
