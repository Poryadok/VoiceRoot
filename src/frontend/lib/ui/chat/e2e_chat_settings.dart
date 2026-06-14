import 'package:flutter/material.dart';
import 'package:intl/intl.dart';

import '../../l10n/app_localizations.dart';
import '../../theme/voice_colors.dart';

/// Opt-in confirmation for enabling E2E in a DM (docs/features/encryption.md).
class E2eEnableConfirmDialog extends StatelessWidget {
  const E2eEnableConfirmDialog({
    super.key,
    required this.chatId,
    required this.onConfirmed,
  });

  static const Key dialogKey = Key('e2e_enable_confirm_dialog');
  static const Key confirmButtonKey = Key('e2e_enable_confirm_button');
  static const Key cancelButtonKey = Key('e2e_enable_cancel_button');

  final String chatId;
  final VoidCallback onConfirmed;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);

    return AlertDialog(
      key: dialogKey,
      title: Text(l10n.e2eEnableTitle),
      content: Text(
        l10n.e2eEnableBody,
        style: TextStyle(color: voice.textSecondary),
      ),
      actions: [
        TextButton(
          key: cancelButtonKey,
          onPressed: () => Navigator.of(context).pop(),
          child: Text(l10n.e2eEnableCancel),
        ),
        FilledButton(
          key: confirmButtonKey,
          onPressed: () {
            Navigator.of(context).pop();
            onConfirmed();
          },
          child: Text(l10n.e2eEnableConfirm),
        ),
      ],
    );
  }
}

/// Opt-out confirmation: warns that new messages revert to plaintext on the server.
class E2eDisableConfirmDialog extends StatelessWidget {
  const E2eDisableConfirmDialog({
    super.key,
    required this.chatId,
    required this.onConfirmed,
  });

  static const Key dialogKey = Key('e2e_disable_confirm_dialog');
  static const Key confirmButtonKey = Key('e2e_disable_confirm_button');
  static const Key cancelButtonKey = Key('e2e_disable_cancel_button');

  final String chatId;
  final VoidCallback onConfirmed;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);

    return AlertDialog(
      key: dialogKey,
      title: Text(l10n.e2eDisableTitle),
      content: Text(
        l10n.e2eDisableBody,
        style: TextStyle(color: voice.textSecondary),
      ),
      actions: [
        TextButton(
          key: cancelButtonKey,
          onPressed: () => Navigator.of(context).pop(),
          child: Text(l10n.e2eDisableCancel),
        ),
        FilledButton(
          key: confirmButtonKey,
          onPressed: () {
            Navigator.of(context).pop();
            onConfirmed();
          },
          child: Text(l10n.e2eDisableConfirm),
        ),
      ],
    );
  }
}

/// Minimal password set/restore UI for encrypted key backup (Auth API).
class E2eKeyBackupSheet extends StatefulWidget {
  const E2eKeyBackupSheet({
    super.key,
    required this.onSave,
    this.onRestore,
  });

  static const Key sheetKey = Key('e2e_key_backup_sheet');
  static const Key passwordFieldKey = Key('e2e_key_backup_password');
  static const Key saveButtonKey = Key('e2e_key_backup_save');
  static const Key restoreButtonKey = Key('e2e_key_backup_restore');

  final Future<void> Function(String password, String? hint) onSave;
  final Future<void> Function(String password)? onRestore;

  @override
  State<E2eKeyBackupSheet> createState() => _E2eKeyBackupSheetState();
}

class _E2eKeyBackupSheetState extends State<E2eKeyBackupSheet> {
  final _password = TextEditingController();
  final _hint = TextEditingController();
  var _busy = false;

  @override
  void dispose() {
    _password.dispose();
    _hint.dispose();
    super.dispose();
  }

  Future<void> _run(Future<void> Function() action) async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      await action();
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;

    return Padding(
      key: E2eKeyBackupSheet.sheetKey,
      padding: EdgeInsets.only(
        left: 16,
        right: 16,
        top: 16,
        bottom: MediaQuery.paddingOf(context).bottom + 16,
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(l10n.e2eKeyBackupTitle, style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: 8),
          Text(l10n.e2eKeyBackupHint),
          const SizedBox(height: 12),
          TextField(
            key: E2eKeyBackupSheet.passwordFieldKey,
            controller: _password,
            obscureText: true,
            decoration: InputDecoration(labelText: l10n.e2eKeyBackupPasswordLabel),
          ),
          const SizedBox(height: 8),
          TextField(
            controller: _hint,
            decoration: InputDecoration(labelText: l10n.e2eKeyBackupPasswordHintLabel),
          ),
          const SizedBox(height: 16),
          FilledButton(
            key: E2eKeyBackupSheet.saveButtonKey,
            onPressed: _busy
                ? null
                : () => _run(
                    () => widget.onSave(
                      _password.text,
                      _hint.text.trim().isEmpty ? null : _hint.text.trim(),
                    ),
                  ),
            child: Text(l10n.e2eKeyBackupSave),
          ),
          if (widget.onRestore != null) ...[
            const SizedBox(height: 8),
            OutlinedButton(
              key: E2eKeyBackupSheet.restoreButtonKey,
              onPressed: _busy
                  ? null
                  : () => _run(() => widget.onRestore!(_password.text)),
              child: Text(l10n.e2eKeyBackupRestore),
            ),
          ],
        ],
      ),
    );
  }
}

/// Undecryptable E2E history placeholder (encryption.md).
class E2eUndecryptableMessagePlaceholder extends StatelessWidget {
  const E2eUndecryptableMessagePlaceholder({
    super.key,
    this.beforeDate,
  });

  final DateTime? beforeDate;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final label = beforeDate == null
        ? l10n.e2eUndecryptableGeneric
        : l10n.e2eUndecryptableBefore(
            DateFormat.yMMMd().format(beforeDate!.toLocal()),
          );

    return Text(
      label,
      style: Theme.of(context).textTheme.bodyMedium?.copyWith(
        fontStyle: FontStyle.italic,
        color: voice.textSecondary,
      ),
    );
  }
}
