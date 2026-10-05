import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/e2e_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/e2e_providers.dart';
import '../../theme/voice_colors.dart';

/// Manage the account-owned encrypted E2E key backup from Settings → Security.
class E2eKeyBackupScreen extends ConsumerStatefulWidget {
  const E2eKeyBackupScreen({super.key});

  static const screenKey = Key('e2e_key_backup_settings_screen');
  static const passwordKey = Key('e2e_key_backup_settings_password');
  static const hintKey = Key('e2e_key_backup_settings_hint');
  static const saveKey = Key('e2e_key_backup_settings_save');
  static const restoreKey = Key('e2e_key_backup_settings_restore');
  static const deleteKey = Key('e2e_key_backup_settings_delete');
  static const confirmDeleteKey = Key('e2e_key_backup_settings_confirm_delete');

  @override
  ConsumerState<E2eKeyBackupScreen> createState() => _E2eKeyBackupScreenState();
}

class _E2eKeyBackupScreenState extends ConsumerState<E2eKeyBackupScreen> {
  final _password = TextEditingController();
  final _hint = TextEditingController();
  E2eKeyBackupData? _backup;
  bool _loading = true;
  bool _busy = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _refresh();
  }

  @override
  void dispose() {
    _password.dispose();
    _hint.dispose();
    super.dispose();
  }

  Future<void> _refresh() async {
    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null) {
      if (mounted) {
        setState(() {
          _loading = false;
          _backup = null;
        });
      }
      return;
    }
    final result = await ref
        .read(voiceE2eClientProvider)
        .getKeyBackup(authorization: auth);
    if (!mounted || auth != ref.read(authorizationHeaderProvider)) return;
    setState(() {
      _loading = false;
      if (result is E2eApiOk<E2eKeyBackupData>) {
        _backup = result.data;
        _hint.text = result.data.passwordHint ?? '';
        _error = null;
      } else if (result is E2eApiFailure && result.statusCode == 404) {
        _backup = null;
        _error = null;
      } else if (result is E2eApiFailure) {
        _error = AppLocalizations.of(context)!.e2eBackupActionFailed;
      }
    });
  }

  Future<void> _save() async {
    final auth = ref.read(authorizationHeaderProvider);
    final password = _password.text;
    if (_busy || auth == null || password.isEmpty) return;
    setState(() {
      _busy = true;
      _error = null;
    });
    final result = await ref
        .read(voiceE2eClientProvider)
        .putKeyBackup(
          authorization: auth,
          password: password,
          passwordHint: _hint.text.trim().isEmpty ? null : _hint.text.trim(),
        );
    if (!mounted) return;
    if (auth != ref.read(authorizationHeaderProvider)) {
      setState(() => _busy = false);
      return;
    }
    setState(() {
      _busy = false;
      if (result is E2eApiOk<void>) {
        _backup = E2eKeyBackupData(
          encryptedBlob: '',
          passwordHint: _hint.text.trim().isEmpty ? null : _hint.text.trim(),
        );
        _password.clear();
      } else {
        _error = AppLocalizations.of(context)!.e2eBackupActionFailed;
      }
    });
  }

  Future<void> _restore() async {
    final auth = ref.read(authorizationHeaderProvider);
    if (_busy || auth == null || _password.text.isEmpty) return;
    setState(() {
      _busy = true;
      _error = null;
    });
    final result = await ref
        .read(voiceE2eClientProvider)
        .restoreKeyBackup(
          authorization: auth,
          password: _password.text,
          isAuthorizationCurrent: () =>
              mounted && auth == ref.read(authorizationHeaderProvider),
        );
    if (!mounted) return;
    setState(() {
      _busy = false;
      if (result is E2eApiOk<void>) {
        _password.clear();
      } else {
        _error = AppLocalizations.of(context)!.e2eBackupRestoreFailed;
      }
    });
  }

  Future<void> _delete() async {
    final l10n = AppLocalizations.of(context)!;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(l10n.e2eBackupDeleteTitle),
        content: Text(l10n.e2eBackupDeleteConfirm),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, false),
            child: Text(l10n.commonCancel),
          ),
          FilledButton(
            key: E2eKeyBackupScreen.confirmDeleteKey,
            onPressed: () => Navigator.pop(dialogContext, true),
            child: Text(l10n.e2eBackupDelete),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;
    final auth = ref.read(authorizationHeaderProvider);
    if (_busy || auth == null) return;
    setState(() {
      _busy = true;
      _error = null;
    });
    final result = await ref
        .read(voiceE2eClientProvider)
        .deleteKeyBackup(authorization: auth);
    if (!mounted) return;
    if (auth != ref.read(authorizationHeaderProvider)) {
      setState(() => _busy = false);
      return;
    }
    setState(() {
      _busy = false;
      if (result is E2eApiOk<void>) {
        _backup = null;
        _password.clear();
        _hint.clear();
      } else {
        _error = l10n.e2eBackupActionFailed;
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final present = _backup != null;
    return Scaffold(
      key: E2eKeyBackupScreen.screenKey,
      appBar: AppBar(title: Text(l10n.e2eKeyBackupTitle)),
      body: SafeArea(
        child: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 520),
            child: ListView(
              padding: const EdgeInsets.all(24),
              children: [
                Text(
                  _loading
                      ? l10n.e2eBackupStatusChecking
                      : present
                      ? l10n.e2eBackupStatusPresent
                      : l10n.e2eBackupStatusAbsent,
                  style: Theme.of(context).textTheme.titleMedium,
                ),
                const SizedBox(height: 8),
                Text(
                  l10n.e2eKeyBackupHint,
                  style: TextStyle(color: voice.textSecondary),
                ),
                const SizedBox(height: 20),
                TextField(
                  key: E2eKeyBackupScreen.passwordKey,
                  controller: _password,
                  obscureText: true,
                  decoration: InputDecoration(
                    labelText: l10n.e2eKeyBackupPasswordLabel,
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  key: E2eKeyBackupScreen.hintKey,
                  controller: _hint,
                  decoration: InputDecoration(
                    labelText: l10n.e2eKeyBackupPasswordHintLabel,
                  ),
                ),
                if (_error != null) ...[
                  const SizedBox(height: 12),
                  Text(
                    _error!,
                    style: TextStyle(
                      color: Theme.of(context).colorScheme.error,
                    ),
                  ),
                ],
                const SizedBox(height: 20),
                FilledButton(
                  key: E2eKeyBackupScreen.saveKey,
                  onPressed: _busy || _loading ? null : _save,
                  child: Text(
                    present ? l10n.e2eBackupChange : l10n.e2eKeyBackupSave,
                  ),
                ),
                if (present) ...[
                  const SizedBox(height: 8),
                  OutlinedButton(
                    key: E2eKeyBackupScreen.restoreKey,
                    onPressed: _busy || _loading ? null : _restore,
                    child: Text(l10n.e2eKeyBackupRestore),
                  ),
                  const SizedBox(height: 8),
                  TextButton(
                    key: E2eKeyBackupScreen.deleteKey,
                    onPressed: _busy || _loading ? null : _delete,
                    child: Text(l10n.e2eBackupDelete),
                  ),
                ],
                if (_busy) const LinearProgressIndicator(),
                if (!_loading && _error != null)
                  TextButton(
                    onPressed: _busy ? null : _refresh,
                    child: Text(l10n.commonRetry),
                  ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
