import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:qr_flutter/qr_flutter.dart';

import '../../backend/auth_client.dart';
import '../../l10n/app_localizations.dart';
import '../auth/auth_errors.dart';
import '../../state/auth_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_primary_button.dart';
import '../core/voice_secondary_button.dart';
import 'active_sessions_screen.dart';

enum _SecurityStep { password, enroll, verify }

/// Enable 2FA: password → QR + backup codes → verify TOTP.
class SecuritySettingsScreen extends ConsumerStatefulWidget {
  const SecuritySettingsScreen({super.key});

  static const Key screenKey = Key('security_settings_screen');
  static const Key passwordFieldKey = Key('security_password');
  static const Key enableButtonKey = Key('security_enable');
  static const Key qrKey = Key('security_qr');
  static const Key totpFieldKey = Key('security_totp');
  static const Key verifyButtonKey = Key('security_verify');

  static const Key deleteAccountButtonKey = Key('security_delete_account');
  static const Key deleteAccountDialogKey = Key('security_delete_account_dialog');
  static const Key deleteAccountPasswordKey = Key('security_delete_account_password');
  static const Key deleteAccountTotpKey = Key('security_delete_account_totp');
  static const Key activeSessionsButtonKey = Key('security_active_sessions');

  @override
  ConsumerState<SecuritySettingsScreen> createState() =>
      _SecuritySettingsScreenState();
}

class _SecuritySettingsScreenState extends ConsumerState<SecuritySettingsScreen> {
  _SecurityStep _step = _SecurityStep.password;
  final _passwordController = TextEditingController();
  final _totpController = TextEditingController();
  TotpEnrollmentData? _enrollment;
  var _busy = false;
  String? _error;
  final _deletePasswordController = TextEditingController();
  final _deleteTotpController = TextEditingController();

  @override
  void dispose() {
    _passwordController.dispose();
    _totpController.dispose();
    _deletePasswordController.dispose();
    _deleteTotpController.dispose();
    super.dispose();
  }

  Future<void> _enable2FA() async {
    final password = _passwordController.text;
    if (password.isEmpty) return;
    final session = ref.read(authControllerProvider).session;
    if (session == null) return;

    setState(() {
      _busy = true;
      _error = null;
    });

    final result = await ref
        .read(voiceAuthClientProvider)
        .enable2FA(session: session, password: password);

    if (!mounted) return;
    switch (result) {
      case Enable2FAOk(:final enrollment):
        setState(() {
          _enrollment = enrollment;
          _step = _SecurityStep.enroll;
          _busy = false;
        });
      case Enable2FAFailure(:final message):
        setState(() {
          _busy = false;
          _error = message;
        });
    }
  }

  Future<void> _verifyTotp() async {
    final code = _totpController.text.trim();
    if (code.isEmpty) return;
    final session = ref.read(authControllerProvider).session;
    if (session == null) return;

    setState(() {
      _busy = true;
      _error = null;
    });

    final result = await ref
        .read(voiceAuthClientProvider)
        .verify2FA(session: session, totpCode: code);

    if (!mounted) return;
    switch (result) {
      case AuthSessionOk(:final session):
        await ref.read(authControllerProvider.notifier).applySession(session);
        if (!mounted) return;
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(AppLocalizations.of(context)!.security2faEnabled),
          ),
        );
        Navigator.of(context).pop();
      case AuthSessionFailure(:final message, :final errorCode, :final statusCode):
        setState(() {
          _busy = false;
          _error =
              resolveAuthErrorKey(
                errorCode: errorCode,
                statusCode: statusCode,
                message: message,
              ) ??
              message;
        });
    }
  }

  Future<void> _confirmDeleteAccount() async {
    final l10n = AppLocalizations.of(context)!;
    final isGuest = ref.read(authControllerProvider).isGuest;
    if (isGuest) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(l10n.securityDeleteAccountGuestUnavailable)),
      );
      return;
    }

    final password = await _showDeletePasswordDialog(l10n);
    if (password == null || password.isEmpty || !mounted) return;

    final session = ref.read(authControllerProvider).session;
    if (session == null) return;

    String? totpCode;
    while (mounted) {
      setState(() {
        _busy = true;
        _error = null;
      });

      final result = await ref
          .read(voiceAuthClientProvider)
          .deleteAccount(
            session: session,
            password: password,
            totpCode: totpCode,
          );

      if (!mounted) return;
      setState(() => _busy = false);

      switch (result) {
        case AuthApiOk<void>():
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text(l10n.securityDeleteAccountSuccess)),
          );
          await ref.read(authControllerProvider.notifier).logout();
          if (!mounted) return;
          Navigator.of(context).popUntil((route) => route.isFirst);
          return;
        case AuthApiFailure(
          :final message,
          :final errorCode,
          :final statusCode,
        ):
          final errorKey = resolveAuthErrorKey(
            errorCode: errorCode,
            statusCode: statusCode,
            message: message,
          );
          if (errorKey == AuthErrorKeys.totpRequired ||
              errorKey == AuthErrorKeys.invalidTotp) {
            totpCode = await _showDeleteTotpDialog(
              l10n,
              errorKey: errorKey == AuthErrorKeys.invalidTotp ? errorKey : null,
            );
            if (totpCode == null || totpCode.trim().isEmpty || !mounted) {
              return;
            }
            continue;
          }
          setState(() => _error = errorKey ?? message);
          return;
      }
    }
  }

  Future<String?> _showDeletePasswordDialog(AppLocalizations l10n) {
    _deletePasswordController.clear();
    return showDialog<String>(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          key: SecuritySettingsScreen.deleteAccountDialogKey,
          title: Text(l10n.securityDeleteAccountConfirmTitle),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(l10n.securityDeleteAccountConfirmMessage),
              const SizedBox(height: 16),
              TextField(
                key: SecuritySettingsScreen.deleteAccountPasswordKey,
                controller: _deletePasswordController,
                obscureText: true,
                decoration: InputDecoration(labelText: l10n.authPasswordLabel),
                autofocus: true,
              ),
            ],
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(dialogContext).pop(),
              child: Text(l10n.commonCancel),
            ),
            TextButton(
              onPressed: () => Navigator.of(
                dialogContext,
              ).pop(_deletePasswordController.text),
              child: Text(l10n.securityDeleteAccountConfirmAction),
            ),
          ],
        );
      },
    );
  }

  Future<String?> _showDeleteTotpDialog(
    AppLocalizations l10n, {
    String? errorKey,
  }) {
    _deleteTotpController.clear();
    return showDialog<String>(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          key: SecuritySettingsScreen.deleteAccountDialogKey,
          title: Text(l10n.securityDeleteAccountConfirmTitle),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(l10n.securityDeleteAccountConfirmMessage),
              const SizedBox(height: 16),
              TextField(
                key: SecuritySettingsScreen.deleteAccountTotpKey,
                controller: _deleteTotpController,
                decoration: InputDecoration(
                  labelText: l10n.authTotpLabel,
                  helperText: l10n.authTotpHelper,
                ),
                autofocus: true,
              ),
              if (errorKey != null) ...[
                const SizedBox(height: 12),
                Text(
                  authErrorMessage(l10n, errorKey),
                  style: TextStyle(
                    color: Theme.of(dialogContext).colorScheme.error,
                  ),
                ),
              ],
            ],
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(dialogContext).pop(),
              child: Text(l10n.commonCancel),
            ),
            TextButton(
              onPressed: () => Navigator.of(
                dialogContext,
              ).pop(_deleteTotpController.text.trim()),
              child: Text(l10n.securityDeleteAccountConfirmAction),
            ),
          ],
        );
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);

    return Scaffold(
      key: SecuritySettingsScreen.screenKey,
      backgroundColor: voice.canvas,
      appBar: AppBar(
        title: Text(l10n.securitySettingsTitle),
        backgroundColor: voice.surface,
      ),
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(24),
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 480),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                switch (_step) {
                  _SecurityStep.password => _buildPasswordStep(
                    context,
                    l10n,
                    voice,
                  ),
                  _SecurityStep.enroll => _buildEnrollStep(context, l10n, voice),
                  _SecurityStep.verify => _buildVerifyStep(context, l10n, voice),
                },
                const SizedBox(height: 32),
                Divider(color: voice.borderDefault),
                const SizedBox(height: 16),
                Text(
                  l10n.securitySessionsTitle,
                  style: Theme.of(context).textTheme.titleMedium,
                ),
                const SizedBox(height: 8),
                Text(
                  l10n.securitySessionsHint,
                  style: TextStyle(color: voice.textSecondary),
                ),
                const SizedBox(height: 16),
                VoiceSecondaryButton(
                  key: SecuritySettingsScreen.activeSessionsButtonKey,
                  onPressed: _busy
                      ? null
                      : () {
                          Navigator.of(context).push(
                            MaterialPageRoute<void>(
                              builder: (_) => const ActiveSessionsScreen(),
                            ),
                          );
                        },
                  child: Text(l10n.securitySessionsManage),
                ),
                const SizedBox(height: 32),
                Divider(color: voice.borderDefault),
                const SizedBox(height: 16),
                Text(
                  l10n.securityDeleteAccountTitle,
                  style: Theme.of(context).textTheme.titleMedium,
                ),
                const SizedBox(height: 8),
                Text(
                  l10n.securityDeleteAccountHint,
                  style: TextStyle(color: voice.textSecondary),
                ),
                const SizedBox(height: 16),
                VoiceSecondaryButton(
                  key: SecuritySettingsScreen.deleteAccountButtonKey,
                  onPressed: _busy ? null : _confirmDeleteAccount,
                  child: Text(
                    l10n.securityDeleteAccountButton,
                    style: TextStyle(color: Theme.of(context).colorScheme.error),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildPasswordStep(
    BuildContext context,
    AppLocalizations l10n,
    VoiceColors voice,
  ) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          l10n.security2faEnableTitle,
          style: Theme.of(context).textTheme.titleMedium,
        ),
        const SizedBox(height: 8),
        Text(
          l10n.security2faEnableHint,
          style: TextStyle(color: voice.textSecondary),
        ),
        const SizedBox(height: 16),
        TextField(
          key: SecuritySettingsScreen.passwordFieldKey,
          controller: _passwordController,
          obscureText: true,
          decoration: InputDecoration(labelText: l10n.authPasswordLabel),
        ),
        if (_error != null) ...[
          const SizedBox(height: 12),
          Text(
            authErrorMessage(l10n, _error!),
            style: TextStyle(color: Theme.of(context).colorScheme.error),
          ),
        ],
        const SizedBox(height: 24),
        VoicePrimaryButton(
          key: SecuritySettingsScreen.enableButtonKey,
          onPressed: _busy ? null : _enable2FA,
          isLoading: _busy,
          child: Text(l10n.security2faContinue),
        ),
      ],
    );
  }

  Widget _buildEnrollStep(
    BuildContext context,
    AppLocalizations l10n,
    VoiceColors voice,
  ) {
    final enrollment = _enrollment;
    if (enrollment == null) return const SizedBox.shrink();

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          l10n.security2faScanQr,
          style: Theme.of(context).textTheme.titleMedium,
        ),
        const SizedBox(height: 16),
        Center(
          child: DecoratedBox(
            key: SecuritySettingsScreen.qrKey,
            decoration: BoxDecoration(
              color: voice.surface,
              border: Border.all(color: voice.borderDefault),
              borderRadius: BorderRadius.circular(8),
            ),
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: QrImageView(
                data: enrollment.totpUri,
                size: 200,
                backgroundColor: voice.surface,
              ),
            ),
          ),
        ),
        const SizedBox(height: 16),
        Text(
          l10n.security2faBackupCodesTitle,
          style: Theme.of(context).textTheme.titleSmall,
        ),
        const SizedBox(height: 8),
        ...enrollment.backupCodes.map(
          (code) => Padding(
            padding: const EdgeInsets.symmetric(vertical: 2),
            child: SelectableText(
              code,
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                fontFamily: 'monospace',
              ),
            ),
          ),
        ),
        const SizedBox(height: 24),
        VoicePrimaryButton(
          onPressed: () => setState(() => _step = _SecurityStep.verify),
          child: Text(l10n.security2faContinue),
        ),
      ],
    );
  }

  Widget _buildVerifyStep(
    BuildContext context,
    AppLocalizations l10n,
    VoiceColors voice,
  ) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          l10n.security2faVerifyTitle,
          style: Theme.of(context).textTheme.titleMedium,
        ),
        const SizedBox(height: 8),
        Text(
          l10n.security2faVerifyHint,
          style: TextStyle(color: voice.textSecondary),
        ),
        const SizedBox(height: 16),
        TextField(
          key: SecuritySettingsScreen.totpFieldKey,
          controller: _totpController,
          keyboardType: TextInputType.number,
          decoration: InputDecoration(labelText: l10n.authTotpLabel),
        ),
        if (_error != null) ...[
          const SizedBox(height: 12),
          Text(
            authErrorMessage(l10n, _error!),
            style: TextStyle(color: Theme.of(context).colorScheme.error),
          ),
        ],
        const SizedBox(height: 24),
        VoicePrimaryButton(
          key: SecuritySettingsScreen.verifyButtonKey,
          onPressed: _busy ? null : _verifyTotp,
          isLoading: _busy,
          child: Text(l10n.security2faVerify),
        ),
        const SizedBox(height: 8),
        VoiceSecondaryButton(
          onPressed: _busy
              ? null
              : () => setState(() => _step = _SecurityStep.enroll),
          child: Text(l10n.security2faBackToQr),
        ),
      ],
    );
  }
}
