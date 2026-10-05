import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:file_selector/file_selector.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/bots_client.dart';
import '../../backend/files_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/bot_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/space_providers.dart';
import '../../theme/voice_colors.dart';
import '../a11y/voice_focus_return.dart';

/// Collects slash command option values; supports autocomplete for string options.
Future<Map<String, dynamic>?> showSlashCommandOptionsSheet({
  required BuildContext context,
  required WidgetRef ref,
  required String chatId,
  required BotSlashCommand command,
}) {
  final focusReturn = VoiceFocusReturn.capture();
  return showModalBottomSheet<Map<String, dynamic>>(
    context: context,
    isScrollControlled: true,
    builder: (ctx) =>
        _SlashCommandOptionsSheet(chatId: chatId, command: command),
  ).whenComplete(focusReturn.restore);
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
  final _attachmentNames = <String, String>{};
  final _debounces = <String, Timer>{};
  final _autocompleteGeneration = <String, int>{};
  final _autocompleteErrors = <String, String>{};
  final _attachmentErrors = <String, String>{};
  String? _openedAuthorization;
  var _uploadingAttachment = false;

  @override
  void initState() {
    super.initState();
    _openedAuthorization = ref.read(authorizationHeaderProvider);
  }

  @override
  void dispose() {
    for (final timer in _debounces.values) {
      timer.cancel();
    }
    for (final c in _controllers.values) {
      c.dispose();
    }
    super.dispose();
  }

  TextEditingController _controllerFor(String name) {
    return _controllers.putIfAbsent(name, TextEditingController.new);
  }

  Future<void> _fetchAutocomplete(
    String optionName,
    String focused,
    int generation,
  ) async {
    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null || auth != _openedAuthorization) return;
    final chatType =
        ref.read(chatTypeForChatProvider(widget.chatId)) ?? 'CHAT_TYPE_CHANNEL';
    final selected = Map<String, dynamic>.from(_values);
    const maxAttempts = 12;
    const retryDelay = Duration(milliseconds: 200);
    BotAutocompleteResult? lastResult;
    String? lastError;

    for (var attempt = 0; attempt < maxAttempts; attempt++) {
      if (!_isCurrentAutocomplete(auth, optionName, generation)) return;
      final result = await ref
          .read(voiceBotsClientProvider)
          .autocompleteOption(
            authorization: auth,
            chatId: widget.chatId,
            chatType: chatType,
            botId: widget.command.botId,
            commandName: widget.command.fullCommandName,
            optionName: optionName,
            focusedValue: focused,
            optionsJson: jsonEncode(selected),
          );
      if (!_isCurrentAutocomplete(auth, optionName, generation)) return;
      switch (result) {
        case BotsApiOk(:final data):
          lastResult = data;
          lastError = null;
          if (data.choices.isNotEmpty || !data.pending) {
            setState(() {
              _suggestions[optionName] = data.choices;
              _autocompleteErrors.remove(optionName);
            });
            return;
          }
        case BotsApiFailure(:final message):
          lastError = message;
      }
      if (attempt < maxAttempts - 1) {
        await Future<void>.delayed(retryDelay);
        if (!_isCurrentAutocomplete(auth, optionName, generation)) return;
      }
    }

    if (!_isCurrentAutocomplete(auth, optionName, generation)) return;
    setState(() {
      _suggestions[optionName] = lastResult?.choices ?? const [];
      if (lastError == null) {
        _autocompleteErrors.remove(optionName);
      } else {
        _autocompleteErrors[optionName] = lastError;
      }
    });
  }

  void _onOptionChanged(BotSlashCommandOption option, String value) {
    final generation = (_autocompleteGeneration[option.name] ?? 0) + 1;
    _autocompleteGeneration[option.name] = generation;
    setState(() {
      _values[option.name] = value;
      _suggestions[option.name] = const [];
      _autocompleteErrors.remove(option.name);
    });
    if (!option.autocomplete) return;
    _debounces.remove(option.name)?.cancel();
    _debounces[option.name] = Timer(const Duration(milliseconds: 250), () {
      _fetchAutocomplete(option.name, value, generation);
    });
  }

  bool _isCurrentAutocomplete(
    String authorization,
    String optionName,
    int generation,
  ) =>
      _isCurrentAuthorization(authorization) &&
      _autocompleteGeneration[optionName] == generation;

  Future<void> _pickAttachment(BotSlashCommandOption option) async {
    if (_uploadingAttachment) return;
    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null || auth != _openedAuthorization) return;

    setState(() => _uploadingAttachment = true);
    try {
      final picked = await openFile();
      if (picked == null || !_isCurrentAuthorization(auth)) return;
      setState(() => _attachmentErrors.remove(option.name));
      final bytes = await picked.readAsBytes();
      if (!_isCurrentAuthorization(auth)) return;
      final chatType = ref.read(chatTypeForChatProvider(widget.chatId));
      final files = ref.read(voiceFilesClientProvider);
      final ticketResult = await files.requestUpload(
        authorization: auth,
        originalName: picked.name,
        mimeType: picked.mimeType ?? 'application/octet-stream',
        sizeBytes: bytes.length,
        chatId: widget.chatId,
        chatType: chatType,
      );
      if (ticketResult is! FilesApiOk<FileUploadTicket> ||
          !_isCurrentAuthorization(auth)) {
        if (_isCurrentAuthorization(auth)) _showAttachmentError(option.name);
        return;
      }
      final ticket = ticketResult.data;
      final put = await files.putBytes(
        uploadUrl: ticket.presignedPutUrl,
        bytes: Uint8List.fromList(bytes),
        mimeType: picked.mimeType ?? 'application/octet-stream',
      );
      if (put is! FilesApiOk<void> || !_isCurrentAuthorization(auth)) {
        if (_isCurrentAuthorization(auth)) _showAttachmentError(option.name);
        return;
      }
      final confirmed = await files.confirmUpload(
        authorization: auth,
        fileId: ticket.fileId,
        bytes: Uint8List.fromList(bytes),
      );
      if (confirmed is! FilesApiOk<FileMetadataData> ||
          !_isCurrentAuthorization(auth)) {
        if (_isCurrentAuthorization(auth)) _showAttachmentError(option.name);
        return;
      }
      setState(() {
        _values[option.name] = ticket.fileId;
        _attachmentNames[option.name] = picked.name;
        _attachmentErrors.remove(option.name);
      });
    } catch (_) {
      if (_isCurrentAuthorization(auth)) _showAttachmentError(option.name);
    } finally {
      if (mounted) {
        setState(() => _uploadingAttachment = false);
      }
    }
  }

  void _showAttachmentError(String optionName) {
    if (!mounted) return;
    setState(() {
      _attachmentErrors[optionName] = AppLocalizations.of(
        context,
      )!.chatAttachmentUploadFailed;
    });
  }

  bool get _canSubmit {
    if (_uploadingAttachment) return false;
    for (final opt in widget.command.options) {
      final value = _values[opt.name];
      if (opt.required && (value == null || value.trim().isEmpty)) {
        return false;
      }
      if (opt.type == 'integer' &&
          value != null &&
          value.trim().isNotEmpty &&
          int.tryParse(value.trim()) == null) {
        return false;
      }
    }
    return true;
  }

  bool _isCurrentAuthorization(String authorization) =>
      mounted && ref.read(authorizationHeaderProvider) == authorization;

  Map<String, dynamic> _typedValues() {
    final result = <String, dynamic>{};
    for (final option in widget.command.options) {
      final value = _values[option.name];
      if (value == null) continue;
      if (option.type == 'integer') {
        if (value.trim().isNotEmpty) {
          result[option.name] = int.parse(value.trim());
        }
      } else {
        result[option.name] = option.type == 'boolean'
            ? value == 'true'
            : value;
      }
    }
    return result;
  }

  Widget _buildOptionField(
    BotSlashCommandOption opt,
    AppLocalizations l10n, {
    required bool autofocus,
  }) {
    switch (opt.type) {
      case 'boolean':
        return SwitchListTile(
          title: Text(opt.name + (opt.required ? ' *' : '')),
          value: _values[opt.name] == 'true',
          onChanged: (v) {
            setState(() {
              _values[opt.name] = v ? 'true' : 'false';
            });
          },
        );
      case 'user':
        return _UserOptionPicker(
          key: Key('slash_option_user_picker_${opt.name}'),
          chatId: widget.chatId,
          option: opt,
          value: _values[opt.name],
          onChanged: (v) => setState(() => _values[opt.name] = v),
        );
      case 'channel':
        return _ChannelOptionPicker(
          key: Key('slash_option_channel_picker_${opt.name}'),
          chatId: widget.chatId,
          option: opt,
          value: _values[opt.name],
          onChanged: (v) => setState(() => _values[opt.name] = v),
        );
      case 'role':
        return _RoleOptionPicker(
          key: Key('slash_option_role_picker_${opt.name}'),
          chatId: widget.chatId,
          option: opt,
          value: _values[opt.name],
          onChanged: (v) => setState(() => _values[opt.name] = v),
        );
      case 'attachment':
        return _AttachmentOptionPicker(
          key: Key('slash_option_attachment_picker_${opt.name}'),
          option: opt,
          fileName: _attachmentNames[opt.name],
          errorMessage: _attachmentErrors[opt.name],
          uploading: _uploadingAttachment,
          onPick: () => _pickAttachment(opt),
        );
      default:
        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            TextField(
              controller: _controllerFor(opt.name),
              autofocus: autofocus,
              keyboardType: opt.type == 'integer'
                  ? TextInputType.number
                  : TextInputType.text,
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
                        _debounces.remove(opt.name)?.cancel();
                        _autocompleteGeneration[opt.name] =
                            (_autocompleteGeneration[opt.name] ?? 0) + 1;
                        setState(() {
                          _values[opt.name] = choice.value;
                          _suggestions[opt.name] = const [];
                          _autocompleteErrors.remove(opt.name);
                        });
                      },
                    ),
                ],
              ),
          ],
        );
    }
  }

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    final l10n = AppLocalizations.of(context)!;
    final sessionMatches =
        _openedAuthorization != null &&
        ref.watch(authorizationHeaderProvider) == _openedAuthorization;
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
            for (final error in _autocompleteErrors.values) ...[
              const SizedBox(height: 8),
              Text(
                error,
                style: TextStyle(color: Theme.of(context).colorScheme.error),
              ),
            ],
            const SizedBox(height: 12),
            for (
              var index = 0;
              index < widget.command.options.length;
              index++
            ) ...[
              _buildOptionField(
                widget.command.options[index],
                l10n,
                autofocus: index == 0,
              ),
              const SizedBox(height: 8),
            ],
            FilledButton(
              style: FilledButton.styleFrom(
                backgroundColor: voice.profileAccent,
              ),
              onPressed: _canSubmit && sessionMatches
                  ? () => Navigator.pop(context, _typedValues())
                  : null,
              child: Text(l10n.slashCommandRun),
            ),
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: Text(l10n.commonCancel),
            ),
          ],
        ),
      ),
    );
  }
}

class _UserOptionPicker extends ConsumerWidget {
  const _UserOptionPicker({
    super.key,
    required this.chatId,
    required this.option,
    required this.value,
    required this.onChanged,
  });

  final String chatId;
  final BotSlashCommandOption option;
  final String? value;
  final ValueChanged<String> onChanged;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final spaceId = ref.watch(spaceIdForChatProvider(chatId));
    if (spaceId == null) {
      return InputDecorator(
        decoration: InputDecoration(
          labelText: option.name + (option.required ? ' *' : ''),
        ),
        child: Text(l10n.slashOptionPickerUnavailable),
      );
    }

    final membersAsync = ref.watch(spaceMembersProvider(spaceId));
    return membersAsync.when(
      loading: () => InputDecorator(
        decoration: InputDecoration(
          labelText: option.name + (option.required ? ' *' : ''),
        ),
        child: const LinearProgressIndicator(),
      ),
      error: (e, _) => InputDecorator(
        decoration: InputDecoration(
          labelText: option.name + (option.required ? ' *' : ''),
        ),
        child: Text(l10n.chatRoomError('$e')),
      ),
      data: (members) {
        return DropdownButtonFormField<String>(
          initialValue: value,
          decoration: InputDecoration(
            labelText: l10n.slashOptionPickUser(option.name),
          ),
          items: [
            for (final member in members)
              DropdownMenuItem(
                value: member.profileId,
                child: Text(member.nickname ?? member.profileId),
              ),
          ],
          onChanged: (v) {
            if (v != null) onChanged(v);
          },
        );
      },
    );
  }
}

class _ChannelOptionPicker extends ConsumerWidget {
  const _ChannelOptionPicker({
    super.key,
    required this.chatId,
    required this.option,
    required this.value,
    required this.onChanged,
  });

  final String chatId;
  final BotSlashCommandOption option;
  final String? value;
  final ValueChanged<String> onChanged;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final spaceId = ref.watch(spaceIdForChatProvider(chatId));
    if (spaceId == null) {
      return InputDecorator(
        decoration: InputDecoration(
          labelText: option.name + (option.required ? ' *' : ''),
        ),
        child: Text(l10n.slashOptionPickerUnavailable),
      );
    }

    final treeAsync = ref.watch(spaceTreeProvider(spaceId));
    return treeAsync.when(
      loading: () => InputDecorator(
        decoration: InputDecoration(
          labelText: option.name + (option.required ? ' *' : ''),
        ),
        child: const LinearProgressIndicator(),
      ),
      error: (e, _) => InputDecorator(
        decoration: InputDecoration(
          labelText: option.name + (option.required ? ' *' : ''),
        ),
        child: Text(l10n.chatRoomError('$e')),
      ),
      data: (tree) {
        final textChats = tree.nodes
            .where((n) => n.isTextChat && n.linkedChatId != null)
            .toList();
        return DropdownButtonFormField<String>(
          initialValue: value,
          decoration: InputDecoration(
            labelText: l10n.slashOptionPickChannel(option.name),
          ),
          items: [
            for (final node in textChats)
              DropdownMenuItem(
                value: node.linkedChatId,
                child: Text(node.displayName),
              ),
          ],
          onChanged: (v) {
            if (v != null) onChanged(v);
          },
        );
      },
    );
  }
}

class _RoleOptionPicker extends ConsumerWidget {
  const _RoleOptionPicker({
    super.key,
    required this.chatId,
    required this.option,
    required this.value,
    required this.onChanged,
  });

  final String chatId;
  final BotSlashCommandOption option;
  final String? value;
  final ValueChanged<String> onChanged;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final spaceId = ref.watch(spaceIdForChatProvider(chatId));
    if (spaceId == null) {
      return InputDecorator(
        decoration: InputDecoration(
          labelText: option.name + (option.required ? ' *' : ''),
        ),
        child: Text(l10n.slashOptionPickerUnavailable),
      );
    }

    final rolesAsync = ref.watch(spaceRolesProvider(spaceId));
    return rolesAsync.when(
      loading: () => InputDecorator(
        decoration: InputDecoration(
          labelText: option.name + (option.required ? ' *' : ''),
        ),
        child: const LinearProgressIndicator(),
      ),
      error: (e, _) => InputDecorator(
        decoration: InputDecoration(
          labelText: option.name + (option.required ? ' *' : ''),
        ),
        child: Text(l10n.chatRoomError('$e')),
      ),
      data: (roles) {
        return DropdownButtonFormField<String>(
          initialValue: value,
          decoration: InputDecoration(
            labelText: l10n.slashOptionPickRole(option.name),
          ),
          items: [
            for (final role in roles)
              DropdownMenuItem(value: role.id, child: Text(role.name)),
          ],
          onChanged: (v) {
            if (v != null) onChanged(v);
          },
        );
      },
    );
  }
}

class _AttachmentOptionPicker extends StatelessWidget {
  const _AttachmentOptionPicker({
    super.key,
    required this.option,
    required this.fileName,
    required this.errorMessage,
    required this.uploading,
    required this.onPick,
  });

  final BotSlashCommandOption option;
  final String? fileName;
  final String? errorMessage;
  final bool uploading;
  final VoidCallback onPick;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        OutlinedButton.icon(
          onPressed: uploading ? null : onPick,
          icon: uploading
              ? SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    color: voice.profileAccent,
                  ),
                )
              : const Icon(Icons.attach_file),
          label: Text(l10n.slashOptionPickAttachment(option.name)),
        ),
        if (fileName != null) ...[
          const SizedBox(height: 4),
          Text(
            l10n.slashOptionAttachmentSelected(fileName!),
            style: Theme.of(context).textTheme.bodySmall,
          ),
        ],
        if (errorMessage != null) ...[
          const SizedBox(height: 4),
          Text(
            errorMessage!,
            style: TextStyle(color: Theme.of(context).colorScheme.error),
          ),
        ],
      ],
    );
  }
}
