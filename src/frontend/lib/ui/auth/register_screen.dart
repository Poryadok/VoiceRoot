import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_layout.dart';
import '../../backend/users_client.dart';
import '../core/voice_primary_button.dart';
import '../core/voice_secondary_button.dart';
import 'auth_errors.dart';
import 'auth_screen.dart';

enum _RegisterAccountType { personal, organization }

/// Registration v3: dedicated form with personal/organization semantics,
/// display name, tag base preview, email and password pair, submit via the
/// existing register contract, and back navigation to the login form.
class RegisterScreen extends ConsumerStatefulWidget {
  const RegisterScreen({super.key});

  static const Key screenKey = Key('register_screen');
  static const Key typePickerKey = Key('register_type_picker');
  static const Key displayNameFieldKey = Key('register_display_name');
  static const Key tagFieldKey = Key('register_tag');
  static const Key emailFieldKey = Key('register_email');
  static const Key passwordFieldKey = Key('register_password');
  static const Key confirmPasswordFieldKey = Key(
    'register_confirm_password',
  );
  static const Key submitButtonKey = Key('register_submit');
  static const Key backButtonKey = Key('register_back');

  @override
  ConsumerState<RegisterScreen> createState() => _RegisterScreenState();
}

class _RegisterScreenState extends ConsumerState<RegisterScreen> {
  final _formKey = GlobalKey<FormState>();
  final _displayNameController = TextEditingController();
  final _tagController = TextEditingController();
  final _emailController = TextEditingController();
  final _passwordController = TextEditingController();
  final _confirmPasswordController = TextEditingController();

  _RegisterAccountType _accountType = _RegisterAccountType.personal;
  var _busy = false;
  var _completed = false;
  String? _errorKey;

  @override
  void initState() {
    super.initState();
  }

  @override
  void dispose() {
    _displayNameController.dispose();
    _tagController.dispose();
    _emailController.dispose();
    _passwordController.dispose();
    _confirmPasswordController.dispose();
    super.dispose();
  }

  bool get _isOrganization => _accountType == _RegisterAccountType.organization;

  Future<void> _submitRegistration() async {
    if (_busy || _completed) return;
    if (!(_formKey.currentState?.validate() ?? false)) return;
    final email = _emailController.text.trim();
    final password = _passwordController.text;
    final confirmPassword = _confirmPasswordController.text;

    if (password != confirmPassword) {
      setState(() => _errorKey = AuthErrorKeys.passwordMismatch);
      return;
    }

    setState(() {
      _busy = true;
      _errorKey = null;
    });

    await ref.read(authControllerProvider.notifier).register(
      email: email,
      password: password,
    );

    if (!mounted) return;
    final auth = ref.read(authControllerProvider);
    setState(() {
      _busy = false;
      if (auth.isAuthenticated) {
        _completed = true;
      } else {
        _errorKey = auth.errorKey ?? '';
      }
    });
  }

  String? _displayNameValidator(String? value, AppLocalizations l10n) {
    final name = value?.trim() ?? '';
    if (name.isEmpty) {
      return l10n.profileErrorDisplayNameRequired;
    }
    if (name.length > kProfileDisplayNameMaxLength) {
      return l10n.profileErrorDisplayNameTooLong;
    }
    return null;
  }

  String? _tagValidator(String? value, AppLocalizations l10n) {
    if (value == null || value.trim().isEmpty) {
      return l10n.registerErrorTagRequired;
    }
    return null;
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

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final displayNameLabel = _isOrganization
        ? l10n.registerDisplayNameOrganization
        : l10n.profileDisplayNameLabel;
    final tagLabel = _isOrganization
        ? l10n.registerTagOrganizationLabel
        : l10n.profileCreateUsernameLabel;
    final tagBase = _tagController.text.trim();

    return Scaffold(
      key: RegisterScreen.screenKey,
      backgroundColor: voice.canvas,
      body: SafeArea(
        child: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(
              maxWidth: VoiceLayout.authFormMaxWidth,
            ),
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
                          l10n.registerTitle,
                          textAlign: TextAlign.center,
                          style: Theme.of(context).textTheme.headlineSmall,
                        ),
                        const SizedBox(height: 16),
                        Text(
                          l10n.registerTypePickerLabel,
                          style: TextStyle(color: voice.textSecondary),
                        ),
                        const SizedBox(height: 8),
                        SegmentedButton<_RegisterAccountType>(
                          key: RegisterScreen.typePickerKey,
                          segments: [
                            ButtonSegment<_RegisterAccountType>(
                              value: _RegisterAccountType.personal,
                              label: Text(l10n.registerTypePersonal),
                            ),
                            ButtonSegment<_RegisterAccountType>(
                              value: _RegisterAccountType.organization,
                              label: Text(l10n.registerTypeOrganization),
                            ),
                          ],
                          selected: {_accountType},
                          onSelectionChanged: _busy || _completed
                              ? null
                              : (next) => setState(
                                  () => _accountType = next.first,
                                ),
                        ),
                        const SizedBox(height: 16),
                        Text(
                          l10n.registerSubtitle,
                          style: Theme.of(context).textTheme.bodyMedium,
                        ),
                        const SizedBox(height: 12),
                        TextFormField(
                          key: RegisterScreen.displayNameFieldKey,
                          controller: _displayNameController,
                          decoration: InputDecoration(
                            labelText: displayNameLabel,
                          ),
                          validator: (v) => _displayNameValidator(v, l10n),
                        ),
                        const SizedBox(height: 12),
                        TextFormField(
                          key: RegisterScreen.tagFieldKey,
                          controller: _tagController,
                          decoration: InputDecoration(
                            labelText: tagLabel,
                          ),
                          validator: (v) => _tagValidator(v, l10n),
                        ),
                        if (tagBase.isNotEmpty)
                          Text(
                            l10n.registerTagPreview(tagBase),
                            key: const Key('register_tag_preview'),
                            style: TextStyle(color: voice.textSecondary),
                          ),
                        Text(
                          l10n.registerTagAssignedHint,
                          key: const Key('register_tag_hint'),
                          style: TextStyle(color: voice.textSecondary),
                        ),
                        const SizedBox(height: 12),
                        TextFormField(
                          key: RegisterScreen.emailFieldKey,
                          controller: _emailController,
                          keyboardType: TextInputType.emailAddress,
                          autofillHints: const [AutofillHints.email],
                          decoration: InputDecoration(
                            labelText: l10n.authEmailLabel,
                          ),
                          validator: (v) => _emailValidator(v, l10n),
                        ),
                        const SizedBox(height: 12),
                        TextFormField(
                          key: RegisterScreen.passwordFieldKey,
                          controller: _passwordController,
                          obscureText: true,
                          autofillHints: const [AutofillHints.password],
                          decoration: InputDecoration(
                            labelText: l10n.authPasswordLabel,
                            helperText: l10n.authPasswordHelper,
                          ),
                          validator: (v) => _passwordValidator(v, l10n),
                        ),
                        const SizedBox(height: 12),
                        TextFormField(
                          key: RegisterScreen.confirmPasswordFieldKey,
                          controller: _confirmPasswordController,
                          obscureText: true,
                          autofillHints: const [AutofillHints.password],
                          decoration: InputDecoration(
                            labelText: l10n.authPasswordLabel,
                          ),
                          validator: (v) => _passwordValidator(v, l10n),
                          onFieldSubmitted: (_) => _submitRegistration(),
                        ),
                        if (_errorKey != null) ...[
                          const SizedBox(height: 12),
                          Text(
                            authFormErrorMessage(l10n, _errorKey!),
                            key: const Key('register_error'),
                            style: TextStyle(color: voice.error),
                          ),
                        ],
                        const SizedBox(height: 24),
                        if (_completed)
                          Text(
                            l10n.registerSuccess,
                            key: const Key('register_success'),
                            textAlign: TextAlign.center,
                          )
                        else
                          VoicePrimaryButton(
                            key: RegisterScreen.submitButtonKey,
                            onPressed: _busy ? null : _submitRegistration,
                            isLoading: _busy,
                            child: Text(l10n.authRegister),
                          ),
                        const SizedBox(height: 8),
                        VoiceSecondaryButton(
                          key: RegisterScreen.backButtonKey,
                          onPressed: _busy
                              ? null
                              : () => Navigator.of(context).pop(),
                          child: Text(l10n.registerBackToLogin),
                        ),
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
