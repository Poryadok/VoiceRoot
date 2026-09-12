import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/auth_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_primary_button.dart';

/// Profile-scoped linked accounts and verification entry points.
class VerificationSettingsSheet extends ConsumerStatefulWidget {
  const VerificationSettingsSheet({super.key});

  static const Key sheetKey = Key('verification_settings_sheet');
  static const Key linkedAccountsKey = Key('verification_linked_accounts');
  static const Key selectedProfileKey = Key('verification_selected_profile');
  static const Key twitchLinkKey = Key('verification_twitch_link');
  static const Key youtubeLinkKey = Key('verification_youtube_link');

  @override
  ConsumerState<VerificationSettingsSheet> createState() =>
      _VerificationSettingsSheetState();
}

class _VerificationSettingsSheetState
    extends ConsumerState<VerificationSettingsSheet> {
  List<LinkedAccount>? _accounts;
  var _loading = true;
  String? _error;
  String? _selectedProfileId;
  String? _busyPlatform;

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
    final selectedProfileId = session.activeProfileId;
    final result = await ref
        .read(voiceAuthClientProvider)
        .listLinkedAccounts(session: session);
    if (!mounted) return;
    setState(() {
      _loading = false;
      switch (result) {
        case AuthApiOk(:final data):
          _selectedProfileId = selectedProfileId;
          _accounts = data
              .where((account) => account.profileId == selectedProfileId)
              .toList(growable: false);
          _error = null;
        case AuthApiFailure(:final message):
          _accounts = const [];
          _error = message;
      }
    });
  }

  Future<void> _linkProvider(String platform) async {
    final session = ref.read(authControllerProvider).session;
    if (session == null) return;
    setState(() => _busyPlatform = platform);
    final result = await ref
        .read(voiceAuthClientProvider)
        .startLinkedAccountLink(
          session: session,
          platform: platform,
          redirectUri: 'https://app.voice.test/oauth/$platform',
        );
    if (!mounted) return;
    setState(() => _busyPlatform = null);
    switch (result) {
      case AuthApiOk(:final data):
        if (data.isNotEmpty) {
          ScaffoldMessenger.of(
            context,
          ).showSnackBar(SnackBar(content: Text('OAuth: $data')));
        }
      case AuthApiFailure(:final message):
        setState(() => _error = message);
    }
  }

  Future<void> _unlinkProvider(String platform) async {
    final session = ref.read(authControllerProvider).session;
    if (session == null) return;
    setState(() => _busyPlatform = platform);
    final result = await ref
        .read(voiceAuthClientProvider)
        .unlinkLinkedAccount(session: session, platform: platform);
    if (!mounted) return;
    switch (result) {
      case AuthApiOk<void>():
        await _loadLinkedAccounts();
      case AuthApiFailure(:final message):
        setState(() => _error = message);
    }
    if (mounted) setState(() => _busyPlatform = null);
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
              if (_selectedProfileId != null) ...[
                const SizedBox(height: 8),
                Text(
                  l10n.verificationSelectedProfile(_selectedProfileId!),
                  key: VerificationSettingsSheet.selectedProfileKey,
                  style: TextStyle(color: voice.textSecondary),
                ),
              ],
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
                                trailing: TextButton(
                                  key: ValueKey(
                                    'verification_${account.platform}_unlink',
                                  ),
                                  onPressed: _busyPlatform == null
                                      ? () => _unlinkProvider(account.platform)
                                      : null,
                                  child: Text(l10n.verificationUnlink),
                                ),
                              ),
                          ],
                        ),
                ),
              const SizedBox(height: 16),
              if (!(_accounts ?? const <LinkedAccount>[]).any(
                (account) => account.platform == 'twitch',
              ))
                VoicePrimaryButton(
                  key: VerificationSettingsSheet.twitchLinkKey,
                  onPressed: _busyPlatform == null
                      ? () => _linkProvider('twitch')
                      : null,
                  isLoading: _busyPlatform == 'twitch',
                  child: Text(l10n.verificationLinkTwitch),
                ),
              if (!(_accounts ?? const <LinkedAccount>[]).any(
                (account) => account.platform == 'youtube',
              )) ...[
                const SizedBox(height: 8),
                VoicePrimaryButton(
                  key: VerificationSettingsSheet.youtubeLinkKey,
                  onPressed: _busyPlatform == null
                      ? () => _linkProvider('youtube')
                      : null,
                  isLoading: _busyPlatform == 'youtube',
                  child: Text(l10n.verificationLinkYoutube),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}
