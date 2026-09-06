import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../a11y/focus_trap.dart';
import '../core/voice_primary_button.dart';
import 'auth_errors.dart';
import 'auth_screen.dart';

/// Modal to convert a guest account to regular (email + password).
class GuestConvertSheet extends ConsumerStatefulWidget {
  const GuestConvertSheet({super.key});

  static const Key modalKey = Key('guest_convert_modal');
  static const Key emailFieldKey = Key('guest_convert_email');
  static const Key passwordFieldKey = Key('guest_convert_password');
  static const Key errorKey = Key('guest_convert_error');
  static const Key codeFieldKey = Key('guest_convert_code');
  static const Key verifyButtonKey = Key('guest_convert_verify');
  static const Key resendButtonKey = Key('guest_convert_resend');
  static const Key refreshPromotionButtonKey =
      Key('guest_convert_refresh_promotion');

  static Future<void> show(BuildContext context) {
    return showModalBottomSheet<void>(
      context: context,
      useRootNavigator: true,
      isScrollControlled: true,
      builder: (_) => const VoiceFocusTrap(child: GuestConvertSheet()),
    );
  }

  @override
  ConsumerState<GuestConvertSheet> createState() => _GuestConvertSheetState();
}

class _GuestConvertSheetState extends ConsumerState<GuestConvertSheet> {
  final _formKey = GlobalKey<FormState>();
  final _emailController = TextEditingController();
  final _passwordController = TextEditingController();
  final _codeController = TextEditingController();
  var _submitting = false;
  String? _apiErrorKey;

  @override
  void dispose() {
    _emailController.dispose();
    _passwordController.dispose();
    _codeController.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!(_formKey.currentState?.validate() ?? false)) {
      return;
    }
    final email = _emailController.text.trim();
    final password = _passwordController.text;
    setState(() {
      _submitting = true;
      _apiErrorKey = null;
    });
    final err = await ref
        .read(authControllerProvider.notifier)
        .convertGuest(email: email, password: password);
    if (!mounted) return;
    setState(() {
      _submitting = false;
      _apiErrorKey = err;
    });
  }

  Future<void> _verify() async {
    final code = _codeController.text.trim();
    if (code.isEmpty) {
      setState(() => _apiErrorKey = AuthErrorKeys.invalidOtp);
      return;
    }
    setState(() {
      _submitting = true;
      _apiErrorKey = null;
    });
    final err = await ref
        .read(authControllerProvider.notifier)
        .verifyGuestConversionEmail(code);
    if (!mounted) return;
    if (err == null) {
      Navigator.of(context).pop();
      return;
    }
    setState(() {
      _submitting = false;
      _apiErrorKey = err;
    });
  }

  Future<void> _resend() async {
    setState(() {
      _submitting = true;
      _apiErrorKey = null;
    });
    final err = await ref
        .read(authControllerProvider.notifier)
        .resendGuestConversionEmailOtp();
    if (!mounted) return;
    setState(() {
      _submitting = false;
      _apiErrorKey = err;
    });
  }

  Future<void> _refreshPromotion() async {
    setState(() {
      _submitting = true;
      _apiErrorKey = null;
    });
    final err = await ref
        .read(authControllerProvider.notifier)
        .resumeGuestConversionPromotion();
    if (!mounted) return;
    if (err == null) {
      Navigator.of(context).pop();
      return;
    }
    setState(() {
      _submitting = false;
      _apiErrorKey = err;
    });
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
    final bottom = MediaQuery.viewInsetsOf(context).bottom;
    final pendingEmail = ref.watch(
      authControllerProvider.select(
        (state) => state.pendingGuestConversionEmail,
      ),
    );
    final promotionPending = ref.watch(
      authControllerProvider.select(
        (state) => state.isGuestConversionPromotionPending,
      ),
    );
    return SafeArea(
      key: GuestConvertSheet.modalKey,
      child: Padding(
        padding: EdgeInsets.fromLTRB(24, 16, 24, 24 + bottom),
        child: Form(
          key: _formKey,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                l10n.guestConvertTitle,
                style: Theme.of(context).textTheme.titleLarge,
              ),
              const SizedBox(height: 8),
              Text(
                pendingEmail == null
                    ? l10n.guestConvertSubtitle
                    : l10n.guestConvertVerificationSubtitle(pendingEmail),
              ),
              const SizedBox(height: 16),
              if (_apiErrorKey != null) ...[
                Text(
                  authErrorMessage(l10n, _apiErrorKey!),
                  key: GuestConvertSheet.errorKey,
                  style: TextStyle(color: Theme.of(context).colorScheme.error),
                ),
                const SizedBox(height: 8),
              ],
              if (pendingEmail == null) ...[
                TextFormField(
                  key: GuestConvertSheet.emailFieldKey,
                  controller: _emailController,
                  keyboardType: TextInputType.emailAddress,
                  autofillHints: const [AutofillHints.email],
                  decoration: InputDecoration(labelText: l10n.authEmailLabel),
                  validator: (v) => _emailValidator(v, l10n),
                ),
                const SizedBox(height: 12),
                TextFormField(
                  key: GuestConvertSheet.passwordFieldKey,
                  controller: _passwordController,
                  obscureText: true,
                  autofillHints: const [AutofillHints.newPassword],
                  decoration: InputDecoration(
                    labelText: l10n.authPasswordLabel,
                  ),
                  validator: (v) => _passwordValidator(v, l10n),
                ),
                const SizedBox(height: 20),
                VoicePrimaryButton(
                  onPressed: _submitting ? null : _submit,
                  isLoading: _submitting,
                  child: Text(l10n.guestConvertSubmit),
                ),
              ] else if (promotionPending) ...[
                Text(l10n.guestConvertPromotionPending),
                const SizedBox(height: 20),
                VoicePrimaryButton(
                  key: GuestConvertSheet.refreshPromotionButtonKey,
                  onPressed: _submitting ? null : _refreshPromotion,
                  isLoading: _submitting,
                  child: Text(l10n.guestConvertRefreshStatus),
                ),
              ] else ...[
                TextFormField(
                  key: GuestConvertSheet.codeFieldKey,
                  controller: _codeController,
                  keyboardType: TextInputType.number,
                  autofillHints: const [AutofillHints.oneTimeCode],
                  decoration: InputDecoration(
                    labelText: l10n.guestConvertCodeLabel,
                  ),
                ),
                const SizedBox(height: 20),
                VoicePrimaryButton(
                  key: GuestConvertSheet.verifyButtonKey,
                  onPressed: _submitting ? null : _verify,
                  isLoading: _submitting,
                  child: Text(l10n.guestConvertVerify),
                ),
                const SizedBox(height: 8),
                TextButton(
                  key: GuestConvertSheet.resendButtonKey,
                  onPressed: _submitting ? null : _resend,
                  child: Text(l10n.guestConvertResend),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}
