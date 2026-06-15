import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';

import '../../backend/e2e_client.dart';
import '../../e2e/e2e_store_factory.dart';
import '../../e2e/e2e_verification_code.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/e2e_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_bottom_sheet.dart';

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

/// DM chat info: E2E opt-in/out + key backup entry (docs/features/encryption.md).
class DmE2eSettingsSection extends ConsumerStatefulWidget {
  const DmE2eSettingsSection({super.key, required this.chatId});

  final String chatId;

  @override
  ConsumerState<DmE2eSettingsSection> createState() =>
      _DmE2eSettingsSectionState();
}

class _DmE2eSettingsSectionState extends ConsumerState<DmE2eSettingsSection> {
  var _busy = false;

  Future<void> _setBusy(Future<void> Function() action) async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      await action();
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _enableE2e() async {
    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    final e2e = ref.read(voiceE2eClientProvider);
    final peerId = dmPeerProfileIdForChat(ref, widget.chatId);
    if (peerId != null) {
      final peerBundle = await e2e.getPreKeyBundle(
        authorization: auth,
        profileId: peerId,
      );
      if (peerBundle is E2eApiFailure &&
          (peerBundle.statusCode == 404 || peerBundle.errorCode == 'not_found')) {
        if (!mounted) return;
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(AppLocalizations.of(context)!.e2ePeerMissingPreKeys)),
        );
        return;
      }
    }
    final upload = await e2e.uploadPreKeyBundle(authorization: auth);
    if (upload is E2eApiFailure) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(upload.message)),
      );
      return;
    }
    final result = await e2e.enableChatE2e(
      authorization: auth,
      chatId: widget.chatId,
    );
    if (result is E2eApiFailure) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(result.message)),
      );
      return;
    }
    await ref.read(chatListControllerProvider.notifier).loadInitial();
  }

  Future<void> _disableE2e() async {
    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    final result = await ref.read(voiceE2eClientProvider).disableChatE2e(
      authorization: auth,
      chatId: widget.chatId,
    );
    if (result is E2eApiFailure) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(result.message)),
      );
      return;
    }
    await ref.read(chatListControllerProvider.notifier).loadInitial();
  }

  void _openKeyBackupSheet() {
    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    final e2e = ref.read(voiceE2eClientProvider);
    showVoiceBottomSheet<void>(
      context: context,
      scrollable: false,
      child: E2eKeyBackupSheet(
        onSave: (password, hint) async {
          final result = await e2e.putKeyBackup(
            authorization: auth,
            password: password,
            passwordHint: hint,
          );
          if (result is E2eApiFailure) throw StateError(result.message);
          if (context.mounted) Navigator.of(context).pop();
        },
        onRestore: (password) async {
          final result = await e2e.restoreKeyBackup(
            authorization: auth,
            password: password,
          );
          if (result is E2eApiFailure) throw StateError(result.message);
          if (context.mounted) Navigator.of(context).pop();
        },
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final enabled = ref.watch(chatE2eEnabledProvider(widget.chatId));

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        SwitchListTile(
          key: const Key('chat_info_e2e_toggle'),
          title: Text(l10n.e2eChatInfoSwitchLabel),
          subtitle: Text(
            enabled ? l10n.e2eEnableBody.split('\n').first : l10n.e2eEnableTitle,
            style: TextStyle(color: voice.textSecondary, fontSize: 12),
          ),
          value: enabled,
          onChanged: _busy
              ? null
              : (next) async {
                  if (next) {
                    await showDialog<void>(
                      context: context,
                      builder: (context) => E2eEnableConfirmDialog(
                        chatId: widget.chatId,
                        onConfirmed: () => _setBusy(_enableE2e),
                      ),
                    );
                  } else {
                    await showDialog<void>(
                      context: context,
                      builder: (context) => E2eDisableConfirmDialog(
                        chatId: widget.chatId,
                        onConfirmed: () => _setBusy(_disableE2e),
                      ),
                    );
                  }
                },
        ),
        if (enabled) _EncryptionCodeBlock(chatId: widget.chatId),
        if (enabled)
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 0, 16, 8),
            child: Text(
              l10n.e2eFileRetentionNotice,
              style: TextStyle(color: voice.textSecondary, fontSize: 12),
            ),
          ),
        ListTile(
          leading: const Icon(Icons.backup_outlined),
          title: Text(l10n.e2eChatInfoKeyBackup),
          onTap: _busy ? null : _openKeyBackupSheet,
        ),
        Divider(height: 1, color: voice.borderDefault),
      ],
    );
  }
}

class _EncryptionCodeBlock extends ConsumerStatefulWidget {
  const _EncryptionCodeBlock({required this.chatId});

  final String chatId;

  @override
  ConsumerState<_EncryptionCodeBlock> createState() =>
      _EncryptionCodeBlockState();
}

class _EncryptionCodeBlockState extends ConsumerState<_EncryptionCodeBlock> {
  String? _code;
  var _loading = true;

  @override
  void initState() {
    super.initState();
    unawaited(_loadCode());
  }

  Future<void> _loadCode() async {
    final auth = ref.read(authorizationHeaderProvider);
    final peerId = dmPeerProfileIdForChat(ref, widget.chatId);
    final localId = ref.read(authControllerProvider).activeProfileId;
    if (auth == null || peerId == null || localId == null) {
      if (mounted) setState(() => _loading = false);
      return;
    }
    try {
      final adapter = ref.read(e2eCryptoAdapterProvider);
      final localStore = await adapter.sessionManager.storeForProfile(localId);
      final localIdentity = await localStore.getIdentityKeyPair();
      final peerBundleResult = await ref.read(voiceE2eClientProvider).getPreKeyBundle(
        authorization: auth,
        profileId: peerId,
      );
      if (peerBundleResult is! E2eApiOk<String> || peerBundleResult.data.isEmpty) {
        if (mounted) setState(() => _loading = false);
        return;
      }
      final parsed = parseSerializedPreKeyBundle(peerBundleResult.data);
      if (parsed == null) {
        if (mounted) setState(() => _loading = false);
        return;
      }
      final code = computeVerificationCode(
        localIdentityKey: identityKeyBytesFromSerialized(
          localIdentity.getPublicKey().serialize(),
        ),
        remoteIdentityKey: identityKeyBytesFromSerialized(
          parsed.getIdentityKey().serialize(),
        ),
        localProfileId: localId,
        remoteProfileId: peerId,
      );
      if (mounted) setState(() {
        _code = code;
        _loading = false;
      });
    } catch (_) {
      if (mounted) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    if (_loading) {
      return const Padding(
        padding: EdgeInsets.symmetric(horizontal: 16, vertical: 8),
        child: LinearProgressIndicator(minHeight: 2),
      );
    }
    if (_code == null) return const SizedBox.shrink();

    return Padding(
      key: const Key('e2e_encryption_code_block'),
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(l10n.e2eEncryptionCodeTitle, style: Theme.of(context).textTheme.titleSmall),
          const SizedBox(height: 8),
          Text(
            _code!,
            style: Theme.of(context).textTheme.headlineSmall?.copyWith(
              letterSpacing: 2,
              fontFeatures: const [FontFeature.tabularFigures()],
            ),
          ),
          const SizedBox(height: 8),
          Text(
            l10n.e2eEncryptionCodeBody,
            style: TextStyle(color: voice.textSecondary, fontSize: 12),
          ),
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
