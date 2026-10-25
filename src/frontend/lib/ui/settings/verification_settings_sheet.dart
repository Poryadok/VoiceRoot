import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../backend/auth_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../theme/voice_colors.dart';
import '../api_error_messages.dart';
import '../core/voice_primary_button.dart';

/// Profile-scoped linked accounts and verification entry points.
class VerificationSettingsSheet extends ConsumerStatefulWidget {
  const VerificationSettingsSheet({super.key});

  static const Key sheetKey = Key('verification_settings_sheet');
  static const Key linkedAccountsKey = Key('verification_linked_accounts');
  static const Key selectedProfileKey = Key('verification_selected_profile');
  static const Key twitchLinkKey = Key('verification_twitch_link');
  static const Key youtubeLinkKey = Key('verification_youtube_link');
  static const Key retryKey = Key('verification_retry');
  static const Key closeKey = Key('verification_close');
  static const Key unlinkConfirmKey = Key('verification_unlink_confirm');
  static const Key verificationDestinationKey = Key(
    'linked_accounts_verification_destination',
  );
  static const Key organizationDestinationKey = Key(
    'linked_accounts_organization_destination',
  );

  @override
  ConsumerState<VerificationSettingsSheet> createState() =>
      _VerificationSettingsSheetState();
}

enum _VerificationErrorSource { load, link, unlink }

class _VerificationSettingsSheetState
    extends ConsumerState<VerificationSettingsSheet> {
  List<LinkedAccount>? _accounts;
  var _loading = true;
  String? _error;
  _VerificationErrorSource? _errorSource;
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
        _errorSource = _VerificationErrorSource.load;
      });
      return;
    }
    if (mounted && !_loading) {
      setState(() {
        _loading = true;
        _error = null;
        _errorSource = null;
      });
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
          _errorSource = null;
        case AuthApiFailure(:final message, :final statusCode):
          _accounts = const [];
          _error = settingsLoadErrorMessage(
            AppLocalizations.of(context)!,
            message,
            statusCode: statusCode,
            fallback: AppLocalizations.of(context)!.commonActionFailed,
          );
          _errorSource = _VerificationErrorSource.load;
      }
    });
  }

  Future<void> _linkProvider(String platform) async {
    final session = ref.read(authControllerProvider).session;
    if (session == null) return;
    setState(() {
      _busyPlatform = platform;
      if (_errorSource == _VerificationErrorSource.link) {
        _error = null;
        _errorSource = null;
      }
    });
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
        if (_errorSource == _VerificationErrorSource.link) {
          setState(() {
            _error = null;
            _errorSource = null;
          });
        }
        final uri = Uri.tryParse(data);
        if (uri == null ||
            !const {'http', 'https'}.contains(uri.scheme) ||
            uri.host.isEmpty) {
          _setActionError(_VerificationErrorSource.link);
          return;
        }
        var launched = false;
        try {
          launched = await launchUrl(uri, mode: LaunchMode.externalApplication);
        } on Object {
          launched = false;
        }
        if (!mounted) return;
        if (!launched) {
          _setActionError(_VerificationErrorSource.link);
        }
      case AuthApiFailure(:final statusCode):
        _setActionError(_VerificationErrorSource.link, statusCode: statusCode);
    }
  }

  Future<void> _unlinkProvider(String platform) async {
    final session = ref.read(authControllerProvider).session;
    if (session == null) return;
    final l10n = AppLocalizations.of(context)!;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(l10n.verificationUnlinkConfirmTitle),
        content: Text(
          l10n.verificationUnlinkConfirmBody(_platformName(l10n, platform)),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(false),
            child: Text(l10n.commonCancel),
          ),
          TextButton(
            key: VerificationSettingsSheet.unlinkConfirmKey,
            onPressed: () => Navigator.of(dialogContext).pop(true),
            child: Text(l10n.verificationUnlink),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;
    setState(() => _busyPlatform = platform);
    final result = await ref
        .read(voiceAuthClientProvider)
        .unlinkLinkedAccount(session: session, platform: platform);
    if (!mounted) return;
    switch (result) {
      case AuthApiOk<void>():
        await _loadLinkedAccounts();
      case AuthApiFailure(:final statusCode):
        _setActionError(
          _VerificationErrorSource.unlink,
          statusCode: statusCode,
        );
    }
    if (mounted) setState(() => _busyPlatform = null);
  }

  void _setActionError(_VerificationErrorSource source, {int? statusCode}) {
    if (!mounted) return;
    setState(() {
      _error = commonActionErrorMessage(
        AppLocalizations.of(context)!,
        statusCode: statusCode,
      );
      _errorSource = source;
      _busyPlatform = null;
    });
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);

    return SafeArea(
      child: Align(
        alignment: Alignment.topCenter,
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 560),
          child: Padding(
            key: VerificationSettingsSheet.sheetKey,
            padding: const EdgeInsets.fromLTRB(24, 16, 24, 24),
            child: SingleChildScrollView(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Row(
                    children: [
                      Expanded(
                        child: Text(
                          l10n.verificationLinkedAccountsTitle,
                          style: Theme.of(context).textTheme.titleLarge,
                        ),
                      ),
                      IconButton(
                        key: VerificationSettingsSheet.closeKey,
                        tooltip: MaterialLocalizations.of(
                          context,
                        ).closeButtonTooltip,
                        onPressed: () => Navigator.of(context).maybePop(),
                        icon: const Icon(Icons.close),
                      ),
                    ],
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
                  OutlinedButton.icon(
                    key: VerificationSettingsSheet.verificationDestinationKey,
                    onPressed: null,
                    icon: const Icon(Icons.verified_outlined),
                    label: Text(l10n.verificationStatusUnavailable),
                  ),
                  const SizedBox(height: 8),
                  OutlinedButton.icon(
                    key: VerificationSettingsSheet.organizationDestinationKey,
                    onPressed: null,
                    icon: const Icon(Icons.business_outlined),
                    label: Text(l10n.verificationOrganizationUnavailable),
                  ),
                  const SizedBox(height: 8),
                  if (_loading)
                    const Center(child: CircularProgressIndicator())
                  else if (_error != null) ...[
                    Text(
                      _error == 'not_authenticated'
                          ? l10n.settingsNotAuthenticated
                          : _error!,
                      style: TextStyle(color: voice.error),
                    ),
                    const SizedBox(height: 8),
                    TextButton.icon(
                      key: VerificationSettingsSheet.retryKey,
                      onPressed: _busyPlatform == null
                          ? _loadLinkedAccounts
                          : null,
                      icon: const Icon(Icons.refresh),
                      label: Text(l10n.commonRetry),
                    ),
                  ] else
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
                                    leading: Icon(
                                      _platformIcon(account.platform),
                                    ),
                                    title: Text(
                                      _platformName(l10n, account.platform),
                                    ),
                                    subtitle: account.displayName != null
                                        ? Text(
                                            '${account.displayName} · ${l10n.verificationConnected}',
                                          )
                                        : Text(l10n.verificationConnected),
                                    trailing: TextButton(
                                      key: ValueKey(
                                        'verification_${account.platform}_unlink',
                                      ),
                                      onPressed: _busyPlatform == null
                                          ? () => _unlinkProvider(
                                              account.platform,
                                            )
                                          : null,
                                      child: Text(l10n.verificationUnlink),
                                    ),
                                  ),
                              ],
                            ),
                    ),
                  const SizedBox(height: 16),
                  if (_accounts != null &&
                      !(_accounts ?? const <LinkedAccount>[]).any(
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
                  if (_accounts != null &&
                      !(_accounts ?? const <LinkedAccount>[]).any(
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
        ),
      ),
    );
  }

  IconData _platformIcon(String platform) => switch (platform) {
    'twitch' => Icons.live_tv_outlined,
    'youtube' => Icons.smart_display_outlined,
    _ => Icons.link,
  };

  String _platformName(AppLocalizations l10n, String platform) =>
      switch (platform) {
        'twitch' => 'Twitch',
        'youtube' => 'YouTube',
        _ => platform,
      };
}
