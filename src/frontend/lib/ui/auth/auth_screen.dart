import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';

/// Register / login form; persists tokens and active [profile_id] via [AuthController].
class AuthScreen extends ConsumerStatefulWidget {
  const AuthScreen({super.key});

  static const Key screenKey = Key('auth_screen');
  static const Key emailFieldKey = Key('auth_email');
  static const Key passwordFieldKey = Key('auth_password');
  static const Key loginButtonKey = Key('auth_login');
  static const Key registerButtonKey = Key('auth_register');

  @override
  ConsumerState<AuthScreen> createState() => _AuthScreenState();
}

class _AuthScreenState extends ConsumerState<AuthScreen> {
  final _emailController = TextEditingController();
  final _passwordController = TextEditingController();

  @override
  void dispose() {
    _emailController.dispose();
    _passwordController.dispose();
    super.dispose();
  }

  Future<void> _submit(bool register) async {
    final email = _emailController.text.trim();
    final password = _passwordController.text;
    if (email.isEmpty || password.isEmpty) return;
    final controller = ref.read(authControllerProvider.notifier);
    if (register) {
      await controller.register(email: email, password: password);
    } else {
      await controller.login(email: email, password: password);
    }
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
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Text(l10n.authTitle, style: Theme.of(context).textTheme.headlineSmall),
                  const SizedBox(height: 24),
                  TextField(
                    key: AuthScreen.emailFieldKey,
                    controller: _emailController,
                    keyboardType: TextInputType.emailAddress,
                    autofillHints: const [AutofillHints.email],
                    decoration: InputDecoration(labelText: l10n.authEmailLabel),
                  ),
                  const SizedBox(height: 12),
                  TextField(
                    key: AuthScreen.passwordFieldKey,
                    controller: _passwordController,
                    obscureText: true,
                    autofillHints: const [AutofillHints.password],
                    decoration: InputDecoration(labelText: l10n.authPasswordLabel),
                    onSubmitted: (_) => _submit(false),
                  ),
                  if (auth.errorMessage != null) ...[
                    const SizedBox(height: 12),
                    Text(
                      l10n.authError(auth.errorMessage!),
                      key: const Key('auth_error'),
                      style: TextStyle(color: Theme.of(context).colorScheme.error),
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
    );
  }
}
