import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../core/voice_primary_button.dart';

/// Modal to convert a guest account to regular (email + password).
class GuestConvertSheet extends ConsumerStatefulWidget {
  const GuestConvertSheet({super.key});

  static const Key modalKey = Key('guest_convert_modal');
  static const Key emailFieldKey = Key('guest_convert_email');
  static const Key passwordFieldKey = Key('guest_convert_password');

  static Future<void> show(BuildContext context) {
    return showModalBottomSheet<void>(
      context: context,
      useRootNavigator: true,
      isScrollControlled: true,
      builder: (_) => const GuestConvertSheet(),
    );
  }

  @override
  ConsumerState<GuestConvertSheet> createState() => _GuestConvertSheetState();
}

class _GuestConvertSheetState extends ConsumerState<GuestConvertSheet> {
  final _emailController = TextEditingController();
  final _passwordController = TextEditingController();
  var _submitting = false;
  String? _error;

  @override
  void dispose() {
    _emailController.dispose();
    _passwordController.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final email = _emailController.text.trim();
    final password = _passwordController.text;
    if (email.isEmpty || password.length < 8) {
      setState(() => _error = 'validation_failed');
      return;
    }
    setState(() {
      _submitting = true;
      _error = null;
    });
    final err = await ref.read(authControllerProvider.notifier).convertGuest(
      email: email,
      password: password,
    );
    if (!mounted) return;
    if (err == null) {
      Navigator.of(context).pop();
      return;
    }
    setState(() {
      _submitting = false;
      _error = err;
    });
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final bottom = MediaQuery.viewInsetsOf(context).bottom;
    return SafeArea(
      key: GuestConvertSheet.modalKey,
      child: Padding(
        padding: EdgeInsets.fromLTRB(24, 16, 24, 24 + bottom),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(
            l10n.guestConvertTitle,
            style: Theme.of(context).textTheme.titleLarge,
          ),
          const SizedBox(height: 8),
          Text(l10n.guestConvertSubtitle),
          const SizedBox(height: 16),
          TextField(
            key: GuestConvertSheet.emailFieldKey,
            controller: _emailController,
            keyboardType: TextInputType.emailAddress,
            autofillHints: const [AutofillHints.email],
            decoration: InputDecoration(labelText: l10n.authEmailLabel),
          ),
          const SizedBox(height: 12),
          TextField(
            key: GuestConvertSheet.passwordFieldKey,
            controller: _passwordController,
            obscureText: true,
            autofillHints: const [AutofillHints.newPassword],
            decoration: InputDecoration(labelText: l10n.authPasswordLabel),
          ),
          if (_error != null) ...[
            const SizedBox(height: 8),
            Text(
              _error!,
              style: TextStyle(color: Theme.of(context).colorScheme.error),
            ),
          ],
          const SizedBox(height: 20),
          VoicePrimaryButton(
            onPressed: _submitting ? null : _submit,
            isLoading: _submitting,
            child: Text(l10n.guestConvertSubmit),
          ),
        ],
      ),
      ),
    );
  }
}
