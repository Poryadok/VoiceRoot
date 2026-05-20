import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import 'auth_errors.dart';

/// Register / login form; persists tokens and active [profile_id] via [AuthController].
class AuthScreen extends ConsumerStatefulWidget {
  const AuthScreen({super.key});

  static const Key screenKey = Key('auth_screen');
  static const Key emailFieldKey = Key('auth_email');
  static const Key passwordFieldKey = Key('auth_password');
  static const Key loginButtonKey = Key('auth_login');
  static const Key registerButtonKey = Key('auth_register');

  static const int minPasswordLength = 8;

  @override
  ConsumerState<AuthScreen> createState() => _AuthScreenState();
}

class _AuthScreenState extends ConsumerState<AuthScreen> {
  final _formKey = GlobalKey<FormState>();
  final _emailController = TextEditingController();
  final _passwordController = TextEditingController();

  @override
  void dispose() {
    _emailController.dispose();
    _passwordController.dispose();
    super.dispose();
  }

  Future<void> _submit(bool register) async {
    if (!(_formKey.currentState?.validate() ?? false)) {
      return;
    }
    final email = _emailController.text.trim();
    final password = _passwordController.text;
    if (email.isEmpty || password.isEmpty) {
      ref.read(authControllerProvider.notifier).setClientError(
            AuthErrorKeys.emptyFields,
          );
      return;
    }
    final controller = ref.read(authControllerProvider.notifier);
    if (register) {
      await controller.register(email: email, password: password);
    } else {
      await controller.login(email: email, password: password);
    }
  }

  String? _emailValidator(String? value, AppLocalizations l10n) {
    if (value == null || value.trim().isEmpty) {
      return l10n.authErrorEmptyFields;
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
    final auth = ref.watch(authControllerProvider);

    return Scaffold(
      key: AuthScreen.screenKey,
      body: SafeArea(
        child: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 400),
            child: Padding(
              padding: const EdgeInsets.all(24),
              child: Form(
                key: _formKey,
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.center,
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Text(
                      l10n.authTitle,
                      style: Theme.of(context).textTheme.headlineSmall,
                    ),
                    const SizedBox(height: 24),
                    TextFormField(
                      key: AuthScreen.emailFieldKey,
                      controller: _emailController,
                      keyboardType: TextInputType.emailAddress,
                      autofillHints: const [AutofillHints.email],
                      decoration: InputDecoration(labelText: l10n.authEmailLabel),
                      validator: (v) => _emailValidator(v, l10n),
                    ),
                    const SizedBox(height: 12),
                    TextFormField(
                      key: AuthScreen.passwordFieldKey,
                      controller: _passwordController,
                      obscureText: true,
                      autofillHints: const [AutofillHints.password],
                      decoration: InputDecoration(
                        labelText: l10n.authPasswordLabel,
                        helperText: l10n.authPasswordHelper,
                      ),
                      validator: (v) => _passwordValidator(v, l10n),
                      onFieldSubmitted: (_) => _submit(false),
                    ),
                    if (auth.errorKey != null) ...[
                      const SizedBox(height: 12),
                      Text(
                        authErrorMessage(l10n, auth.errorKey!),
                        key: const Key('auth_error'),
                        style: TextStyle(
                          color: Theme.of(context).colorScheme.error,
                        ),
                      ),
                    ],
                    const SizedBox(height: 24),
                    FilledButton(
                      key: AuthScreen.loginButtonKey,
                      onPressed: auth.isSubmitting ? null : () => _submit(false),
                      child: Text(l10n.authLogin),
                    ),
                    const SizedBox(height: 8),
                    OutlinedButton(
                      key: AuthScreen.registerButtonKey,
                      onPressed: auth.isSubmitting ? null : () => _submit(true),
                      child: Text(l10n.authRegister),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
