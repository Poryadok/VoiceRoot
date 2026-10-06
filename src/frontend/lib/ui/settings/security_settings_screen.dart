import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter/services.dart';
import 'package:qr_flutter/qr_flutter.dart';

import '../../backend/auth_client.dart';
import '../../backend/auth_session.dart';
import '../../l10n/app_localizations.dart';
import '../auth/auth_errors.dart';
import '../../state/auth_providers.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_theme.dart';
import '../core/voice_primary_button.dart';
import '../core/voice_secondary_button.dart';
import '../chat/e2e_attachment_actions.dart';
import 'active_sessions_screen.dart';
import 'e2e_key_backup_screen.dart';

enum _SecurityStep { password, enroll, verify }

/// Enable 2FA: password → QR + backup codes → verify TOTP.
class SecuritySettingsScreen extends ConsumerStatefulWidget {
  const SecuritySettingsScreen({super.key});

  static const Key screenKey = Key('security_settings_screen');
  static const Key passwordFieldKey = Key('security_password');
  static const Key enableButtonKey = Key('security_enable');
  static const Key qrKey = Key('security_qr');
  static const Key statusRetryKey = Key('security_2fa_status_retry');
  static const Key statusUnavailableKey = Key(
    'security_2fa_status_unavailable',
  );
  static const Key manualSecretKey = Key('security_2fa_manual_secret');
  static const Key backupCodesKey = Key('security_2fa_backup_codes');
  static const Key copyBackupCodesKey = Key('security_2fa_copy_backup_codes');
  static const Key downloadBackupCodesKey = Key(
    'security_2fa_download_backup_codes',
  );
  static const Key disableButtonKey = Key('security_2fa_disable');
  static const Key totpFieldKey = Key('security_totp');
  static const Key verifyButtonKey = Key('security_verify');

  static const Key deleteAccountButtonKey = Key('security_delete_account');
  static const Key deleteAccountDialogKey = Key(
    'security_delete_account_dialog',
  );
  static const Key deleteAccountPasswordKey = Key(
    'security_delete_account_password',
  );
  static const Key deleteAccountTotpKey = Key('security_delete_account_totp');
  static const Key activeSessionsButtonKey = Key('security_active_sessions');
  static const Key changePasswordButtonKey = Key('security_change_password');
  static const Key changePasswordCurrentFieldKey = Key(
    'security_change_password_current',
  );
  static const Key changePasswordNewFieldKey = Key(
    'security_change_password_new',
  );
  static const Key changePasswordConfirmFieldKey = Key(
    'security_change_password_confirm',
  );
  static const Key changePasswordTotpFieldKey = Key(
    'security_change_password_totp',
  );
  static const Key changePasswordSubmitKey = Key(
    'security_change_password_submit',
  );

  @override
  ConsumerState<SecuritySettingsScreen> createState() =>
      _SecuritySettingsScreenState();
}

class _SecuritySettingsScreenState
    extends ConsumerState<SecuritySettingsScreen> {
  _SecurityStep _step = _SecurityStep.password;
  final _passwordController = TextEditingController();
  final _totpController = TextEditingController();
  TotpEnrollmentData? _enrollment;
  AuthSession? _enrollmentOwner;
  String? _manualSecret;
  bool? _twoFactorEnabled;
  var _twoFactorLoading = true;
  var _twoFactorUnavailable = false;
  var _backupCodesVerified = false;
  var _backupActionBusy = false;
  var _busy = false;
  String? _error;
  var _authContextGeneration = 0;
  var _statusRequestGeneration = 0;
  late final ProviderSubscription<AuthState> _authSubscription;
  final _deletePasswordController = TextEditingController();
  final _deleteTotpController = TextEditingController();
  final _changeCurrentPasswordController = TextEditingController();
  final _changeNewPasswordController = TextEditingController();
  final _changeConfirmPasswordController = TextEditingController();
  final _changeTotpController = TextEditingController();
  var _changePasswordOpen = false;
  var _changePasswordSecondFactorRequired = false;
  String? _changePasswordError;

  @override
  void initState() {
    super.initState();
    _authSubscription = ref.listenManual(authControllerProvider, (
      previous,
      next,
    ) {
      final before = previous?.session;
      final after = next.session;
      if (before?.accountId == after?.accountId &&
          before?.activeProfileId == after?.activeProfileId) {
        return;
      }
      _authContextGeneration++;
      _passwordController.clear();
      _totpController.clear();
      _changeCurrentPasswordController.clear();
      _changeNewPasswordController.clear();
      _changeConfirmPasswordController.clear();
      _changeTotpController.clear();
      if (!mounted) return;
      setState(() {
        _step = _SecurityStep.password;
        _enrollment = null;
        _enrollmentOwner = null;
        _manualSecret = null;
        _backupCodesVerified = false;
        _twoFactorEnabled = null;
        _twoFactorLoading = after != null;
        _twoFactorUnavailable = after == null;
        _busy = false;
        _backupActionBusy = false;
        _error = null;
        _changePasswordOpen = false;
        _changePasswordSecondFactorRequired = false;
        _changePasswordError = null;
      });
      if (after != null) _loadTwoFactorStatus();
    });
    _loadTwoFactorStatus();
  }

  Future<void> _loadTwoFactorStatus() async {
    final generation = _authContextGeneration;
    final request = ++_statusRequestGeneration;
    final session = ref.read(authControllerProvider).session;
    if (mounted) {
      setState(() {
        _twoFactorLoading = session != null;
        _twoFactorUnavailable = session == null;
        _twoFactorEnabled = null;
      });
    }
    if (session == null) return;

    bool? enabled;
    try {
      enabled = await ref
          .read(voiceAuthClientProvider)
          .is2FAEnabled(session: session);
    } on Object {
      enabled = null;
    }
    if (!mounted ||
        generation != _authContextGeneration ||
        request != _statusRequestGeneration) {
      return;
    }
    if (ref.read(authControllerProvider).session != session) {
      unawaited(_loadTwoFactorStatus());
      return;
    }
    setState(() {
      _twoFactorLoading = false;
      _twoFactorUnavailable = enabled == null;
      _twoFactorEnabled = enabled;
    });
  }

  bool _isCurrentSession(AuthSession session, int generation) =>
      mounted &&
      generation == _authContextGeneration &&
      ref.read(authControllerProvider).session == session;

  bool _isCurrentPasswordChangeSession(
    AuthSession session,
    int contextGeneration,
    int sessionInstallGeneration,
  ) {
    final controller = ref.read(authControllerProvider.notifier);
    final current = ref.read(authControllerProvider).session;
    return mounted &&
        contextGeneration == _authContextGeneration &&
        controller.sessionInstallGeneration == sessionInstallGeneration &&
        current?.accountId == session.accountId &&
        current?.activeProfileId == session.activeProfileId;
  }

  bool _continueWithCurrentSession(AuthSession session, int generation) {
    if (_isCurrentSession(session, generation)) return true;
    if (!mounted || generation != _authContextGeneration) return false;
    _passwordController.clear();
    _totpController.clear();
    setState(() {
      _step = _SecurityStep.password;
      _enrollment = null;
      _enrollmentOwner = null;
      _manualSecret = null;
      _backupCodesVerified = false;
      _busy = false;
      _backupActionBusy = false;
      _error = null;
      _twoFactorEnabled = null;
      _twoFactorUnavailable = false;
      _twoFactorLoading = true;
    });
    unawaited(_loadTwoFactorStatus());
    return false;
  }

  String? _secretFromTotpUri(String value) {
    final uri = Uri.tryParse(value);
    if (uri == null ||
        uri.scheme != 'otpauth' ||
        uri.host != 'totp' ||
        !uri.hasQuery) {
      return null;
    }
    final secretValues = uri.queryParametersAll['secret'] ?? const <String>[];
    if (secretValues.length != 1) return null;
    final secret = secretValues.single.trim();
    return secret.isEmpty ? null : secret;
  }

  @override
  void dispose() {
    _authSubscription.close();
    _statusRequestGeneration++;
    _passwordController.dispose();
    _totpController.dispose();
    _deletePasswordController.dispose();
    _deleteTotpController.dispose();
    _changeCurrentPasswordController.dispose();
    _changeNewPasswordController.dispose();
    _changeConfirmPasswordController.dispose();
    _changeTotpController.dispose();
    super.dispose();
  }

  Future<void> _enable2FA() async {
    final password = _passwordController.text;
    if (password.isEmpty) return;
    final session = ref.read(authControllerProvider).session;
    if (session == null) return;
    final generation = _authContextGeneration;

    setState(() {
      _busy = true;
      _error = null;
    });

    Enable2FAResult result;
    try {
      result = await ref
          .read(voiceAuthClientProvider)
          .enable2FA(session: session, password: password);
    } on Object {
      if (_continueWithCurrentSession(session, generation)) {
        setState(() {
          _busy = false;
          _error = 'commonActionFailed';
        });
      }
      return;
    }

    if (!_continueWithCurrentSession(session, generation)) return;
    switch (result) {
      case Enable2FAOk(:final enrollment):
        final manualSecret = _secretFromTotpUri(enrollment.totpUri);
        if (manualSecret == null ||
            enrollment.backupCodes.isEmpty ||
            enrollment.backupCodes.any((code) => code.trim().isEmpty)) {
          setState(() {
            _busy = false;
            _error = 'commonActionFailed';
          });
          return;
        }
        setState(() {
          _enrollment = enrollment;
          _enrollmentOwner = session;
          _manualSecret = manualSecret;
          _backupCodesVerified = false;
          _step = _SecurityStep.enroll;
          _busy = false;
        });
      case Enable2FAFailure(
        :final message,
        :final errorCode,
        :final statusCode,
      ):
        setState(() {
          _busy = false;
          _error =
              resolveAuthErrorKey(
                errorCode: errorCode,
                statusCode: statusCode,
                message: message,
              ) ??
              'commonActionFailed';
        });
    }
  }

  Future<void> _verifyTotp() async {
    final code = _totpController.text.trim();
    if (code.isEmpty) return;
    final session = ref.read(authControllerProvider).session;
    if (session == null) return;
    final generation = _authContextGeneration;

    setState(() {
      _busy = true;
      _error = null;
    });

    AuthSessionResult result;
    try {
      result = await ref
          .read(voiceAuthClientProvider)
          .verify2FA(session: session, totpCode: code);
    } on Object {
      if (_continueWithCurrentSession(session, generation)) {
        setState(() {
          _busy = false;
          _error = 'commonActionFailed';
        });
      }
      return;
    }

    if (!_continueWithCurrentSession(session, generation)) return;
    switch (result) {
      case AuthSessionOk(:final session):
        final owner = _enrollmentOwner;
        if (owner == null ||
            owner.accountId != session.accountId ||
            owner.activeProfileId != session.activeProfileId) {
          setState(() {
            _busy = false;
            _error = 'commonActionFailed';
          });
          return;
        }
        try {
          await ref.read(authControllerProvider.notifier).applySession(session);
        } on Object {
          if (_continueWithCurrentSession(owner, generation)) {
            setState(() {
              _busy = false;
              _error = 'commonActionFailed';
            });
          }
          return;
        }
        if (!mounted || generation != _authContextGeneration) return;
        if (ref.read(authControllerProvider).session != session) {
          unawaited(_loadTwoFactorStatus());
          return;
        }
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(AppLocalizations.of(context)!.security2faEnabled),
          ),
        );
        setState(() {
          _twoFactorEnabled = true;
          _twoFactorLoading = false;
          _twoFactorUnavailable = false;
          _backupCodesVerified = true;
          _enrollmentOwner = session;
          _busy = false;
          _error = null;
        });
      case AuthSessionFailure(
        :final message,
        :final errorCode,
        :final statusCode,
      ):
        setState(() {
          _busy = false;
          _error =
              resolveAuthErrorKey(
                errorCode: errorCode,
                statusCode: statusCode,
                message: message,
              ) ??
              'commonActionFailed';
        });
    }
  }

  Future<void> _disable2FA() async {
    final password = _passwordController.text;
    final code = _totpController.text.trim();
    final session = ref.read(authControllerProvider).session;
    if (password.isEmpty || code.isEmpty || session == null) return;
    final generation = _authContextGeneration;
    setState(() {
      _busy = true;
      _error = null;
    });
    AuthApiResult<void> result;
    try {
      result = await ref
          .read(voiceAuthClientProvider)
          .disable2FA(session: session, password: password, totpCode: code);
    } on Object {
      if (_continueWithCurrentSession(session, generation)) {
        setState(() {
          _busy = false;
          _error = 'commonActionFailed';
        });
      }
      return;
    }
    if (!_continueWithCurrentSession(session, generation)) return;
    switch (result) {
      case AuthApiOk<void>():
        await ref.read(authControllerProvider.notifier).logout();
        if (mounted) Navigator.of(context).popUntil((route) => route.isFirst);
      case AuthApiFailure(:final message, :final errorCode, :final statusCode):
        setState(() {
          _busy = false;
          _error =
              resolveAuthErrorKey(
                errorCode: errorCode,
                statusCode: statusCode,
                message: message,
              ) ??
              'commonActionFailed';
        });
    }
  }

  List<String>? _verifiedBackupCodesForCurrentAccount() {
    final session = ref.read(authControllerProvider).session;
    final owner = _enrollmentOwner;
    final codes = _enrollment?.backupCodes;
    if (!_backupCodesVerified ||
        session == null ||
        owner == null ||
        codes == null ||
        owner.accountId != session.accountId ||
        owner.activeProfileId != session.activeProfileId) {
      return null;
    }
    return codes;
  }

  Future<void> _copyBackupCodes() async {
    final codes = _verifiedBackupCodesForCurrentAccount();
    final session = ref.read(authControllerProvider).session;
    final generation = _authContextGeneration;
    if (codes == null || session == null || _backupActionBusy) return;
    setState(() => _backupActionBusy = true);
    try {
      await Clipboard.setData(ClipboardData(text: codes.join('\n')));
    } on Object {
      if (!mounted ||
          generation != _authContextGeneration ||
          ref.read(authControllerProvider).session?.accountId !=
              session.accountId ||
          ref.read(authControllerProvider).session?.activeProfileId !=
              session.activeProfileId) {
        return;
      }
      setState(() => _backupActionBusy = false);
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(AppLocalizations.of(context)!.commonActionFailed),
        ),
      );
      return;
    }
    if (!mounted ||
        generation != _authContextGeneration ||
        ref.read(authControllerProvider).session?.accountId !=
            session.accountId ||
        ref.read(authControllerProvider).session?.activeProfileId !=
            session.activeProfileId) {
      return;
    }
    setState(() => _backupActionBusy = false);
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(
          AppLocalizations.of(context)!.security2faBackupCodesCopied,
        ),
      ),
    );
  }

  Future<void> _downloadBackupCodes() async {
    final codes = _verifiedBackupCodesForCurrentAccount();
    final session = ref.read(authControllerProvider).session;
    final generation = _authContextGeneration;
    if (codes == null || session == null || _backupActionBusy) return;
    setState(() => _backupActionBusy = true);
    bool saved;
    try {
      saved = await saveDecryptedE2eAttachment(
        bytes: Uint8List.fromList(utf8.encode(codes.join('\n'))),
        fileName: 'voice-2fa-backup-codes.txt',
      );
    } on Object {
      saved = false;
    }
    if (!mounted ||
        generation != _authContextGeneration ||
        ref.read(authControllerProvider).session?.accountId !=
            session.accountId ||
        ref.read(authControllerProvider).session?.activeProfileId !=
            session.activeProfileId) {
      return;
    }
    setState(() => _backupActionBusy = false);
    if (saved) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(
          AppLocalizations.of(context)!.security2faBackupCodesDownloadFailed,
        ),
      ),
    );
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

  void _openChangePassword() {
    _changeCurrentPasswordController.clear();
    _changeNewPasswordController.clear();
    _changeConfirmPasswordController.clear();
    _changeTotpController.clear();
    setState(() {
      _changePasswordOpen = true;
      _changePasswordSecondFactorRequired = false;
      _changePasswordError = null;
    });
  }

  void _cancelChangePassword() {
    _changeCurrentPasswordController.clear();
    _changeNewPasswordController.clear();
    _changeConfirmPasswordController.clear();
    _changeTotpController.clear();
    setState(() {
      _changePasswordOpen = false;
      _changePasswordSecondFactorRequired = false;
      _changePasswordError = null;
    });
  }

  Future<void> _submitChangePassword() async {
    final currentPassword = _changeCurrentPasswordController.text;
    final newPassword = _changeNewPasswordController.text;
    final confirmation = _changeConfirmPasswordController.text;
    final session = ref.read(authControllerProvider).session;
    final sessionInstallGeneration = ref
        .read(authControllerProvider.notifier)
        .sessionInstallGeneration;
    final generation = _authContextGeneration;
    if (session == null || _busy) return;
    if (_twoFactorLoading ||
        _twoFactorUnavailable ||
        _twoFactorEnabled == null) {
      setState(() => _changePasswordError = 'commonActionFailed');
      return;
    }
    if (currentPassword.isEmpty) {
      setState(() => _changePasswordError = AuthErrorKeys.emptyFields);
      return;
    }
    if (newPassword.length < 8) {
      setState(() => _changePasswordError = AuthErrorKeys.passwordTooShort);
      return;
    }
    if (newPassword != confirmation) {
      setState(() => _changePasswordError = AuthErrorKeys.passwordMismatch);
      return;
    }

    setState(() {
      _busy = true;
      _changePasswordError = null;
    });
    AuthApiResult<void> result;
    try {
      result = await ref
          .read(voiceAuthClientProvider)
          .changePassword(
            session: session,
            currentPassword: currentPassword,
            newPassword: newPassword,
            totpCode:
                _twoFactorEnabled == true || _changePasswordSecondFactorRequired
                ? _changeTotpController.text
                : null,
          );
    } on Object {
      if (_isCurrentSession(session, generation)) {
        setState(() {
          _busy = false;
          _changePasswordError = 'commonActionFailed';
        });
      }
      return;
    }
    if (!_isCurrentPasswordChangeSession(
      session,
      generation,
      sessionInstallGeneration,
    )) {
      return;
    }

    switch (result) {
      case AuthApiOk<void>():
        final didLogout = await ref
            .read(authControllerProvider.notifier)
            .logoutIfCurrent(
              session,
              serverAlreadyRevoked: true,
              sessionInstallGeneration: sessionInstallGeneration,
            );
        if (!didLogout || !mounted) return;
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              AppLocalizations.of(context)!.securityChangePasswordSuccess,
            ),
          ),
        );
        Navigator.of(context).popUntil((route) => route.isFirst);
      case AuthApiFailure(:final message, :final errorCode, :final statusCode):
        final errorKey = resolveAuthErrorKey(
          errorCode: errorCode,
          statusCode: statusCode,
          message: message,
        );
        setState(() {
          _busy = false;
          _changePasswordError = errorKey ?? 'commonActionFailed';
          if (errorKey == AuthErrorKeys.totpRequired) {
            _changePasswordSecondFactorRequired = true;
          }
        });
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
                VoiceSecondaryButton(
                  key: SecuritySettingsScreen.changePasswordButtonKey,
                  onPressed: _busy
                      ? null
                      : _changePasswordOpen
                      ? _cancelChangePassword
                      : _openChangePassword,
                  child: Text(l10n.securityChangePasswordAction),
                ),
                if (_changePasswordOpen) ...[
                  const SizedBox(height: 16),
                  _buildChangePasswordForm(context, l10n),
                ],
                const SizedBox(height: 24),
                if (_twoFactorLoading)
                  Center(
                    child: Semantics(
                      liveRegion: true,
                      child: CircularProgressIndicator(),
                    ),
                  )
                else if (_twoFactorUnavailable ||
                    _twoFactorEnabled == null ||
                    ref.watch(authControllerProvider).session == null)
                  _buildTwoFactorUnavailable(context, l10n)
                else if (_twoFactorEnabled == true) ...[
                  if (_verifiedBackupCodesForCurrentAccount() case final codes?)
                    _buildVerifiedBackupCodes(context, l10n, codes),
                  _buildDisableStep(context, l10n, voice),
                ] else
                  switch (_step) {
                    _SecurityStep.password => _buildPasswordStep(
                      context,
                      l10n,
                      voice,
                    ),
                    _SecurityStep.enroll => _buildEnrollStep(
                      context,
                      l10n,
                      voice,
                    ),
                    _SecurityStep.verify => _buildVerifyStep(
                      context,
                      l10n,
                      voice,
                    ),
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
                const SizedBox(height: 16),
                Text(
                  l10n.e2eKeyBackupTitle,
                  style: Theme.of(context).textTheme.titleMedium,
                ),
                const SizedBox(height: 8),
                Text(l10n.e2eKeyBackupHint),
                const SizedBox(height: 12),
                VoiceSecondaryButton(
                  key: const Key('security_e2e_key_backup'),
                  onPressed: _busy
                      ? null
                      : () => Navigator.of(context).push(
                          MaterialPageRoute<void>(
                            builder: (_) => const E2eKeyBackupScreen(),
                          ),
                        ),
                  child: Text(l10n.e2eKeyBackupManage),
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
                    style: TextStyle(
                      color: Theme.of(context).colorScheme.error,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildChangePasswordForm(BuildContext context, AppLocalizations l10n) {
    final factorRequired =
        _twoFactorEnabled == true || _changePasswordSecondFactorRequired;
    final statusKnown =
        !_twoFactorLoading &&
        !_twoFactorUnavailable &&
        _twoFactorEnabled != null;
    return Column(
      key: const Key('security_change_password_form'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          l10n.securityChangePasswordAction,
          style: Theme.of(context).textTheme.titleMedium,
        ),
        const SizedBox(height: 12),
        TextField(
          key: SecuritySettingsScreen.changePasswordCurrentFieldKey,
          controller: _changeCurrentPasswordController,
          obscureText: true,
          textInputAction: TextInputAction.next,
          decoration: InputDecoration(labelText: l10n.authPasswordLabel),
        ),
        const SizedBox(height: 12),
        TextField(
          key: SecuritySettingsScreen.changePasswordNewFieldKey,
          controller: _changeNewPasswordController,
          obscureText: true,
          textInputAction: TextInputAction.next,
          decoration: InputDecoration(
            labelText: l10n.passwordResetNewPasswordLabel,
            helperText: l10n.authPasswordHelper,
          ),
        ),
        const SizedBox(height: 12),
        TextField(
          key: SecuritySettingsScreen.changePasswordConfirmFieldKey,
          controller: _changeConfirmPasswordController,
          obscureText: true,
          textInputAction: factorRequired
              ? TextInputAction.next
              : TextInputAction.done,
          onSubmitted: factorRequired ? null : (_) => _submitChangePassword(),
          decoration: InputDecoration(
            labelText: l10n.passwordResetConfirmPasswordLabel,
          ),
        ),
        if (factorRequired) ...[
          const SizedBox(height: 12),
          TextField(
            key: SecuritySettingsScreen.changePasswordTotpFieldKey,
            controller: _changeTotpController,
            keyboardType: TextInputType.number,
            textInputAction: TextInputAction.done,
            onSubmitted: (_) => _submitChangePassword(),
            decoration: InputDecoration(
              labelText: l10n.authTotpLabel,
              helperText: l10n.security2faVerifyHint,
            ),
          ),
        ],
        if (!statusKnown) ...[
          const SizedBox(height: 12),
          Text(
            l10n.security2faStatusUnavailable,
            style: TextStyle(color: Theme.of(context).colorScheme.error),
          ),
          const SizedBox(height: 8),
          VoiceSecondaryButton(
            onPressed: _twoFactorLoading ? null : _loadTwoFactorStatus,
            child: Text(l10n.commonRetry),
          ),
        ],
        if (_changePasswordError != null) ...[
          const SizedBox(height: 12),
          Text(
            authFormErrorMessage(l10n, _changePasswordError!),
            style: TextStyle(color: Theme.of(context).colorScheme.error),
          ),
        ],
        const SizedBox(height: 16),
        VoicePrimaryButton(
          key: SecuritySettingsScreen.changePasswordSubmitKey,
          onPressed: _busy || !statusKnown ? null : _submitChangePassword,
          isLoading: _busy,
          child: Text(l10n.commonSave),
        ),
        const SizedBox(height: 8),
        VoiceSecondaryButton(
          onPressed: _busy ? null : _cancelChangePassword,
          child: Text(l10n.commonCancel),
        ),
      ],
    );
  }

  Widget _buildTwoFactorUnavailable(
    BuildContext context,
    AppLocalizations l10n,
  ) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          l10n.security2faStatusUnavailable,
          key: SecuritySettingsScreen.statusUnavailableKey,
          style: TextStyle(color: Theme.of(context).colorScheme.error),
        ),
        const SizedBox(height: 12),
        VoiceSecondaryButton(
          key: SecuritySettingsScreen.statusRetryKey,
          onPressed: _twoFactorLoading ? null : _loadTwoFactorStatus,
          child: Text(l10n.commonRetry),
        ),
      ],
    );
  }

  Widget _buildVerifiedBackupCodes(
    BuildContext context,
    AppLocalizations l10n,
    List<String> codes,
  ) {
    return Column(
      key: SecuritySettingsScreen.backupCodesKey,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          l10n.security2faBackupCodesTitle,
          style: Theme.of(context).textTheme.titleSmall,
        ),
        const SizedBox(height: 8),
        for (final code in codes)
          Padding(
            padding: const EdgeInsets.symmetric(vertical: 2),
            child: SelectableText(
              code,
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                fontFamily: VoiceTheme.fontFamily,
              ),
            ),
          ),
        const SizedBox(height: 16),
        VoiceSecondaryButton(
          key: SecuritySettingsScreen.copyBackupCodesKey,
          onPressed: _busy || _backupActionBusy ? null : _copyBackupCodes,
          child: Text(l10n.security2faCopyBackupCodes),
        ),
        const SizedBox(height: 8),
        VoiceSecondaryButton(
          key: SecuritySettingsScreen.downloadBackupCodesKey,
          onPressed: _busy || _backupActionBusy ? null : _downloadBackupCodes,
          child: Text(l10n.security2faDownloadBackupCodes),
        ),
        const SizedBox(height: 24),
      ],
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
            authFormErrorMessage(l10n, _error!),
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

  Widget _buildDisableStep(
    BuildContext context,
    AppLocalizations l10n,
    VoiceColors voice,
  ) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          l10n.security2faDisableTitle,
          style: Theme.of(context).textTheme.titleMedium,
        ),
        const SizedBox(height: 8),
        Text(
          l10n.security2faDisableHint,
          style: TextStyle(color: voice.textSecondary),
        ),
        const SizedBox(height: 16),
        TextField(
          key: SecuritySettingsScreen.passwordFieldKey,
          controller: _passwordController,
          obscureText: true,
          decoration: InputDecoration(labelText: l10n.authPasswordLabel),
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
            authFormErrorMessage(l10n, _error!),
            style: TextStyle(color: Theme.of(context).colorScheme.error),
          ),
        ],
        const SizedBox(height: 24),
        VoiceSecondaryButton(
          key: SecuritySettingsScreen.disableButtonKey,
          onPressed: _busy ? null : _disable2FA,
          child: Text(
            l10n.security2faDisable,
            style: TextStyle(color: Theme.of(context).colorScheme.error),
          ),
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
                backgroundColor: Colors.white,
                eyeStyle: const QrEyeStyle(
                  eyeShape: QrEyeShape.square,
                  color: Colors.black,
                ),
                dataModuleStyle: const QrDataModuleStyle(
                  dataModuleShape: QrDataModuleShape.square,
                  color: Colors.black,
                ),
              ),
            ),
          ),
        ),
        const SizedBox(height: 16),
        Text(l10n.security2faManualSecretLabel),
        const SizedBox(height: 8),
        SelectableText(
          _manualSecret ?? '',
          key: SecuritySettingsScreen.manualSecretKey,
          style: Theme.of(
            context,
          ).textTheme.bodyMedium?.copyWith(fontFamily: VoiceTheme.fontFamily),
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
            authFormErrorMessage(l10n, _error!),
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
