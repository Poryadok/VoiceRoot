import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/auth_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_primary_button.dart';

/// Stub: linked accounts and verification entry points (multi-profile/verification (docs/features/multi-profile.md)).
class VerificationSettingsSheet extends ConsumerStatefulWidget {
  const VerificationSettingsSheet({super.key});

  static const Key sheetKey = Key('verification_settings_sheet');
  static const Key linkedAccountsKey = Key('verification_linked_accounts');
  static const Key twitchLinkKey = Key('verification_twitch_link');

  @override
  ConsumerState<VerificationSettingsSheet> createState() =>
      _VerificationSettingsSheetState();
}

class _VerificationSettingsSheetState
    extends ConsumerState<VerificationSettingsSheet> {
  List<LinkedAccount>? _accounts;
  var _loading = true;
  String? _error;
  var _linkingTwitch = false;

  @override
  void initState() {
    super.initState();
    _loadLinkedAccounts();
  }

  Future<void> _loadLinkedAccounts() async {
    final session = ref.read(authControllerProvider).session;
    if (session == null) {
      setState(() {
        _loading = false;
        _error = 'not_authenticated';
      });
      return;
    }
    final result = await ref
        .read(voiceAuthClientProvider)
        .listLinkedAccounts(session: session);
    if (!mounted) return;
    setState(() {
      _loading = false;
      switch (result) {
        case AuthApiOk(:final data):
          _accounts = data;
          _error = null;
        case AuthApiFailure(:final message):
          _accounts = const [];
          _error = message;
      }
    });
  }

  Future<void> _linkTwitch() async {
    final session = ref.read(authControllerProvider).session;
    if (session == null) return;
    setState(() => _linkingTwitch = true);
    final result = await ref
        .read(voiceAuthClientProvider)
        .startLinkedAccountLink(
          session: session,
          platform: 'twitch',
          redirectUri: 'https://app.voice.test/oauth/twitch',
        );
    if (!mounted) return;
    setState(() => _linkingTwitch = false);
    switch (result) {
      case AuthApiOk(:final data):
        if (data.isNotEmpty) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text('OAuth: $data')),
          );
        }
      case AuthApiFailure(:final message):
        setState(() => _error = message);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);

    return SafeArea(
      child: Padding(
        key: VerificationSettingsSheet.sheetKey,
        padding: const EdgeInsets.fromLTRB(24, 16, 24, 24),
        child: SingleChildScrollView(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                l10n.verificationSettingsTitle,
                style: Theme.of(context).textTheme.titleLarge,
              ),
              const SizedBox(height: 8),
              Text(
                l10n.verificationSettingsHint,
                style: TextStyle(color: voice.textSecondary),
              ),
              const SizedBox(height: 16),
              Text(
                l10n.verificationLinkedAccountsTitle,
                style: TextStyle(color: voice.textSecondary),
              ),
              const SizedBox(height: 8),
              if (_loading)
                const Center(child: CircularProgressIndicator())
              else if (_error != null)
                Text(_error!, style: TextStyle(color: voice.error))
              else
                KeyedSubtree(
                  key: VerificationSettingsSheet.linkedAccountsKey,
                  child: _accounts!.isEmpty
                      ? Text(
                          l10n.verificationLinkedAccountsEmpty,
                          style: TextStyle(color: voice.textSecondary),
                        )
                      : Column(
                          children: [
                            for (final account in _accounts!)
                              ListTile(
                                contentPadding: EdgeInsets.zero,
                                title: Text(account.platform),
                                subtitle: account.displayName != null
                                    ? Text(account.displayName!)
                                    : null,
                              ),
                          ],
                        ),
                ),
              const SizedBox(height: 16),
              VoicePrimaryButton(
                key: VerificationSettingsSheet.twitchLinkKey,
                onPressed: _linkingTwitch ? null : _linkTwitch,
                isLoading: _linkingTwitch,
                child: Text(l10n.verificationLinkTwitch),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
