import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/social_providers.dart';

/// First-run screen for auto-registered guest accounts: nickname only.
class GuestNicknameScreen extends ConsumerStatefulWidget {
  const GuestNicknameScreen({super.key});

  @override
  ConsumerState<GuestNicknameScreen> createState() =>
      _GuestNicknameScreenState();
}

class _GuestNicknameScreenState extends ConsumerState<GuestNicknameScreen> {
  final _nicknameController = TextEditingController();
  var _busy = false;
  String? _error;

  @override
  void dispose() {
    _nicknameController.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final nickname = _nicknameController.text.trim();
    if (nickname.isEmpty) return;
    setState(() {
      _busy = true;
      _error = null;
    });
    final err = await ref.read(profileActionsProvider).updateBasicProfile(
      displayName: nickname,
      bio: '',
    );
    if (!mounted) return;
    if (err != null) {
      setState(() {
        _busy = false;
        _error = err;
      });
      return;
    }
    ref.read(authControllerProvider.notifier).completeGuestNickname();
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final scheme = Theme.of(context).colorScheme;

    return Scaffold(
      key: const Key('guest_nickname_screen'),
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                l10n.guestNicknameTitle,
                style: Theme.of(context).textTheme.headlineSmall,
              ),
              const SizedBox(height: 8),
              Text(
                l10n.guestNicknameSubtitle,
                style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                  color: scheme.onSurfaceVariant,
                ),
              ),
              const SizedBox(height: 24),
              TextField(
                key: const Key('guest_nickname_field'),
                controller: _nicknameController,
                decoration: InputDecoration(
                  labelText: l10n.guestNicknameLabel,
                  hintText: l10n.guestNicknameHint,
                ),
                textInputAction: TextInputAction.done,
                onSubmitted: _busy ? null : (_) => _submit(),
              ),
              if (_error != null) ...[
                const SizedBox(height: 12),
                Text(
                  _error!,
                  style: TextStyle(color: scheme.error),
                ),
              ],
              const SizedBox(height: 24),
              FilledButton(
                key: const Key('guest_nickname_submit'),
                onPressed: _busy ? null : _submit,
                child: _busy
                    ? const SizedBox(
                        width: 20,
                        height: 20,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : Text(l10n.guestNicknameContinue),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
