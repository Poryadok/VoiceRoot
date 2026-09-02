import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/auth_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_layout.dart';
import '../core/voice_primary_button.dart';
import '../core/voice_secondary_button.dart';
import 'auth_errors.dart';
import 'auth_screen.dart';

enum _PasswordResetStep { request, reset }

/// Password reset: request OTP by email, then set a new password with the code.
class PasswordResetScreen extends ConsumerStatefulWidget {
  const PasswordResetScreen({super.key, this.initialEmail});

  final String? initialEmail;

  static const Key screenKey = Key('password_reset_screen');
  static const Key emailFieldKey = Key('password_reset_email');
  static const Key sendLinkButtonKey = Key('password_reset_send');
  static const Key backButtonKey = Key('password_reset_back');
  static const Key codeFieldKey = Key('password_reset_code');
  static const Key newPasswordFieldKey = Key('password_reset_new_password');
  static const Key confirmPasswordFieldKey = Key('password_reset_confirm_password');
  static const Key resetButtonKey = Key('password_reset_submit');

  @override
  ConsumerState<PasswordResetScreen> createState() =>
      _PasswordResetScreenState();
}

class _PasswordResetScreenState extends ConsumerState<PasswordResetScreen> {
  final _formKey = GlobalKey<FormState>();
  final _emailController = TextEditingController();
  final _codeController = TextEditingController();
  final _newPasswordController = TextEditingController();
  final _confirmPasswordController = TextEditingController();

  _PasswordResetStep _step = _PasswordResetStep.request;
  var _busy = false;
  String? _errorKey;
  var _completed = false;

  @override
  void initState() {
    super.initState();
    final initialEmail = widget.initialEmail?.trim();
    if (initialEmail != null && initialEmail.isNotEmpty) {
      _emailController.text = initialEmail;
    }
  }

  @override
  void dispose() {
    _emailController.dispose();
    _codeController.dispose();
    _newPasswordController.dispose();
    _confirmPasswordController.dispose();
    super.dispose();
  }

  Future<void> _sendResetLink() async {
    if (!(_formKey.currentState?.validate() ?? false)) return;
    final email = _emailController.text.trim();
    if (email.isEmpty) {
      setState(() => _errorKey = AuthErrorKeys.emptyEmail);
      return;
    }

    setState(() {
      _busy = true;
      _errorKey = null;
    });

    final result = await ref
        .read(voiceAuthClientProvider)
        .sendPasswordResetOtp(email: email);

    if (!mounted) return;
    switch (result) {
      case AuthApiOk<void>():
        setState(() {
          _step = _PasswordResetStep.reset;
          _busy = false;
        });
      case AuthApiFailure(:final errorCode, :final statusCode, :final message):
        setState(() {
          _busy = false;
          _errorKey = resolveAuthErrorKey(
            errorCode: errorCode,
            statusCode: statusCode,
            message: message,
          );
        });
    }
  }

  Future<void> _submitReset() async {
    if (!(_formKey.currentState?.validate() ?? false)) return;
    final email = _emailController.text.trim();
    final code = _codeController.text.trim();
    final newPassword = _newPasswordController.text;
    final confirmPassword = _confirmPasswordController.text;

    if (newPassword != confirmPassword) {
      setState(() => _errorKey = AuthErrorKeys.passwordMismatch);
      return;
    }

    setState(() {
      _busy = true;
      _errorKey = null;
    });

    final result = await ref.read(voiceAuthClientProvider).resetPassword(
      email: email,
      code: code,
      newPassword: newPassword,
    );

    if (!mounted) return;
    switch (result) {
      case AuthApiOk<void>():
        setState(() {
          _completed = true;
          _busy = false;
        });
      case AuthApiFailure(:final errorCode, :final statusCode, :final message):
        setState(() {
          _busy = false;
          _errorKey = resolveAuthErrorKey(
            errorCode: errorCode,
            statusCode: statusCode,
            message: message,
          );
        });
    }
  }

  String? _emailValidator(String? value, AppLocalizations l10n) {
    if (value == null || value.trim().isEmpty) {
      return l10n.authErrorEmailRequired;
    }
    return null;
  }

  String? _passwordValidator(String? value, AppLocalizations l10n) {
    if (value == null || value.isEmpty) {
      return l10n.authErrorEmptyFields;
    }
    if (value.length < AuthScreen.minPasswordLength) {
      return l10n.authErrorPasswordTooShort;
    }
    return null;
  }

  String? _codeValidator(String? value, AppLocalizations l10n) {
    if (value == null || value.trim().isEmpty) {
      return l10n.authErrorInvalidOtp;
    }
    return null;
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);

    return Scaffold(
      key: PasswordResetScreen.screenKey,
      backgroundColor: voice.canvas,
      body: SafeArea(
        child: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: VoiceLayout.authFormMaxWidth),
            child: Padding(
              padding: const EdgeInsets.all(24),
              child: Material(
                color: voice.surface,
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(8),
                  side: BorderSide(color: voice.borderDefault),
                ),
                child: Padding(
                  padding: const EdgeInsets.all(24),
                  child: Form(
                    key: _formKey,
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        Text(
                          l10n.passwordResetTitle,
                          textAlign: TextAlign.center,
                          style: Theme.of(context).textTheme.headlineSmall,
                        ),
                        const SizedBox(height: 16),
                        if (_completed) ...[
                          Text(
                            l10n.passwordResetSuccess,
                            key: const Key('password_reset_success'),
                            textAlign: TextAlign.center,
                          ),
                          const SizedBox(height: 24),
                          VoicePrimaryButton(
                            key: PasswordResetScreen.backButtonKey,
                            onPressed: () => Navigator.of(context).pop(),
                            child: Text(l10n.passwordResetBackToLogin),
                          ),
                        ] else if (_step == _PasswordResetStep.request) ...[
                          Text(
                            l10n.passwordResetRequestSubtitle,
                            style: Theme.of(context).textTheme.bodyMedium,
                          ),
                          const SizedBox(height: 16),
                          TextFormField(
                            key: PasswordResetScreen.emailFieldKey,
                            controller: _emailController,
                            keyboardType: TextInputType.emailAddress,
                            autofillHints: const [AutofillHints.email],
                            decoration: InputDecoration(
                              labelText: l10n.authEmailLabel,
                            ),
                            validator: (v) => _emailValidator(v, l10n),
                            onFieldSubmitted: (_) => _sendResetLink(),
                          ),
                          if (_errorKey != null) ...[
                            const SizedBox(height: 12),
                            Text(
                              authErrorMessage(l10n, _errorKey!),
                              key: const Key('password_reset_error'),
                              style: TextStyle(
                                color: Theme.of(context).colorScheme.error,
                              ),
                            ),
                          ],
                          const SizedBox(height: 24),
                          VoicePrimaryButton(
                            key: PasswordResetScreen.sendLinkButtonKey,
                            onPressed: _busy ? null : _sendResetLink,
                            isLoading: _busy,
                            child: Text(l10n.passwordResetSendLink),
                          ),
                          const SizedBox(height: 8),
                          VoiceSecondaryButton(
                            key: PasswordResetScreen.backButtonKey,
                            onPressed: _busy
                                ? null
                                : () => Navigator.of(context).pop(),
                            child: Text(l10n.passwordResetBackToLogin),
                          ),
                        ] else ...[
                          Text(
                            l10n.passwordResetCodeSent(_emailController.text.trim()),
                            style: Theme.of(context).textTheme.bodyMedium,
                          ),
                          const SizedBox(height: 16),
                          TextFormField(
                            key: PasswordResetScreen.codeFieldKey,
                            controller: _codeController,
                            keyboardType: TextInputType.number,
                            autofillHints: const [AutofillHints.oneTimeCode],
                            decoration: InputDecoration(
                              labelText: l10n.passwordResetCodeLabel,
                            ),
                            validator: (v) => _codeValidator(v, l10n),
                          ),
                          const SizedBox(height: 12),
                          TextFormField(
                            key: PasswordResetScreen.newPasswordFieldKey,
                            controller: _newPasswordController,
                            obscureText: true,
                            autofillHints: const [AutofillHints.newPassword],
                            decoration: InputDecoration(
                              labelText: l10n.passwordResetNewPasswordLabel,
                              helperText: l10n.authPasswordHelper,
                            ),
                            validator: (v) => _passwordValidator(v, l10n),
                          ),
                          const SizedBox(height: 12),
                          TextFormField(
                            key: PasswordResetScreen.confirmPasswordFieldKey,
                            controller: _confirmPasswordController,
                            obscureText: true,
                            autofillHints: const [AutofillHints.newPassword],
                            decoration: InputDecoration(
                              labelText: l10n.passwordResetConfirmPasswordLabel,
                            ),
                            validator: (v) => _passwordValidator(v, l10n),
                            onFieldSubmitted: (_) => _submitReset(),
                          ),
                          if (_errorKey != null) ...[
                            const SizedBox(height: 12),
                            Text(
                              authErrorMessage(l10n, _errorKey!),
                              key: const Key('password_reset_error'),
                              style: TextStyle(
                                color: Theme.of(context).colorScheme.error,
                              ),
                            ),
                          ],
                          const SizedBox(height: 24),
                          VoicePrimaryButton(
                            key: PasswordResetScreen.resetButtonKey,
                            onPressed: _busy ? null : _submitReset,
                            isLoading: _busy,
                            child: Text(l10n.passwordResetSubmit),
                          ),
                          const SizedBox(height: 8),
                          VoiceSecondaryButton(
                            key: PasswordResetScreen.backButtonKey,
                            onPressed: _busy
                                ? null
                                : () => Navigator.of(context).pop(),
                            child: Text(l10n.passwordResetBackToLogin),
                          ),
                        ],
                      ],
                    ),
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
