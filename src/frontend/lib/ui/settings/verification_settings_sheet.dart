import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../backend/auth_client.dart';
import '../../backend/auth_session.dart';
import '../../backend/gateway_http.dart';
import '../../backend/users_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/social_providers.dart';
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
  static const Key verificationStatusKey = Key(
    'verification_status_destination',
  );
  static const Key verificationStatusViewKey = Key('verification_status_view');
  static const Key verificationStatusRetryKey = Key(
    'verification_status_retry',
  );
  static const Key verificationStatusValueKey = Key(
    'verification_status_value',
  );
  static const Key pendingChallengeKey = Key(
    'verification_pending_organization_challenge',
  );
  static const Key pendingChallengeDomainKey = Key(
    'verification_pending_challenge_domain',
  );
  static const Key pendingChallengeTxtRecordKey = Key(
    'verification_pending_challenge_txt_record',
  );
  static const Key pendingChallengeExpiresAtKey = Key(
    'verification_pending_challenge_expires_at',
  );
  static const Key organizationUnavailableKey = Key(
    'verification_organization_unavailable',
  );

  @override
  ConsumerState<VerificationSettingsSheet> createState() =>
      _VerificationSettingsSheetState();
}

enum _VerificationErrorSource { load, link, unlink }

class _SessionActionSnapshot {
  const _SessionActionSnapshot({
    required this.accountId,
    required this.profileId,
    required this.installGeneration,
    required this.requestIdentity,
  });

  final String accountId;
  final String profileId;
  final int installGeneration;
  final GatewayRequestIdentity requestIdentity;
}

class _VerificationSettingsSheetState
    extends ConsumerState<VerificationSettingsSheet> {
  List<LinkedAccount>? _accounts;
  var _loading = true;
  String? _error;
  _VerificationErrorSource? _errorSource;
  String? _selectedProfileId;
  String? _busyPlatform;
  var _showVerificationStatus = false;
  var _verificationStatusLoading = false;
  VerificationStatus? _verificationStatus;
  PendingOrganizationChallenge? _pendingOrganizationChallenge;
  String? _verificationStatusError;
  var _hasObservedSession = false;
  int? _observedSessionInstallGeneration;
  String? _observedAccountId;
  String? _observedProfileId;
  var _verificationStatusRequestGeneration = 0;

  @override
  void initState() {
    super.initState();
    _observedSessionInstallGeneration = ref
        .read(authControllerProvider.notifier)
        .sessionInstallGeneration;
    _loadLinkedAccounts();
  }

  _SessionActionSnapshot? _snapshotFor(AuthSession session) {
    if (ref.read(profileSwitchInProgressProvider)) return null;
    final controller = ref.read(authControllerProvider.notifier);
    final requestIdentity = controller.gatewayRequestIdentity;
    if (requestIdentity == null ||
        requestIdentity.accountId != session.accountId ||
        requestIdentity.profileId != session.activeProfileId) {
      return null;
    }
    return _SessionActionSnapshot(
      accountId: session.accountId,
      profileId: session.activeProfileId,
      installGeneration: controller.sessionInstallGeneration,
      requestIdentity: requestIdentity,
    );
  }

  bool _isCurrent(_SessionActionSnapshot snapshot) {
    if (!mounted || ref.read(profileSwitchInProgressProvider)) return false;
    final current = ref.read(authControllerProvider).session;
    final controller = ref.read(authControllerProvider.notifier);
    return current != null &&
        current.accountId == snapshot.accountId &&
        current.activeProfileId == snapshot.profileId &&
        controller.sessionInstallGeneration == snapshot.installGeneration &&
        controller.gatewayRequestIdentity == snapshot.requestIdentity;
  }

  void _clearForSessionChange({required bool switching}) {
    if (!mounted) return;
    final session = ref.read(authControllerProvider).session;
    setState(() {
      _accounts = null;
      _selectedProfileId = null;
      _busyPlatform = null;
      _error = session == null && !switching ? 'not_authenticated' : null;
      _errorSource = session == null && !switching
          ? _VerificationErrorSource.load
          : null;
      _loading = session != null || switching;
    });
    if (session != null && !switching) _loadLinkedAccounts();
  }

  Future<void> _loadLinkedAccounts() async {
    final session = ref.read(authControllerProvider).session;
    if (ref.read(profileSwitchInProgressProvider)) return;
    if (session == null) {
      setState(() {
        _loading = false;
        _error = 'not_authenticated';
        _errorSource = _VerificationErrorSource.load;
      });
      return;
    }
    final snapshot = _snapshotFor(session);
    if (snapshot == null) return;
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
    if (!_isCurrent(snapshot)) return;
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
    final snapshot = _snapshotFor(session);
    if (snapshot == null) return;
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
    if (!_isCurrent(snapshot)) return;
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
        if (!_isCurrent(snapshot)) return;
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
    final snapshot = _snapshotFor(session);
    if (snapshot == null) return;
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
    if (confirmed != true || !_isCurrent(snapshot)) return;
    setState(() => _busyPlatform = platform);
    final result = await ref
        .read(voiceAuthClientProvider)
        .unlinkLinkedAccount(session: session, platform: platform);
    if (!_isCurrent(snapshot)) return;
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

  Future<void> _openVerificationStatus() async {
    setState(() => _showVerificationStatus = true);
    await _loadVerificationStatus();
  }

  Future<void> _loadVerificationStatus() async {
    final requestGeneration = ++_verificationStatusRequestGeneration;
    final session = ref.read(authControllerProvider).session;
    if (session == null) {
      setState(() {
        _verificationStatusLoading = false;
        _verificationStatus = null;
        _pendingOrganizationChallenge = null;
        _verificationStatusError = 'not_authenticated';
      });
      return;
    }
    final authController = ref.read(authControllerProvider.notifier);
    final requestIdentity = authController.gatewayRequestIdentity;
    final installGeneration = authController.sessionInstallGeneration;
    if (requestIdentity == null || ref.read(profileSwitchInProgressProvider)) {
      setState(() {
        _verificationStatusLoading = false;
        _verificationStatus = null;
        _pendingOrganizationChallenge = null;
      });
      return;
    }
    setState(() {
      _verificationStatusLoading = true;
      _verificationStatus = null;
      _pendingOrganizationChallenge = null;
      _verificationStatusError = null;
    });
    final result = await ref
        .read(voiceUsersClientProvider)
        .getVerificationStatus(authorization: session.authorizationHeader);
    if (!mounted || requestGeneration != _verificationStatusRequestGeneration) {
      return;
    }
    if (ref.read(profileSwitchInProgressProvider) ||
        authController.sessionInstallGeneration != installGeneration ||
        authController.gatewayRequestIdentity != requestIdentity ||
        ref.read(authControllerProvider).session?.activeProfileId !=
            session.activeProfileId) {
      // Session/profile listeners clear and reload their own state. This
      // response no longer owns the visible request and must not publish over
      // it (or its loading/error state).
      return;
    }
    switch (result) {
      case UsersApiOk(:final data):
        if (data.profileId != session.activeProfileId) {
          setState(() {
            _verificationStatusLoading = false;
            _verificationStatus = null;
            _pendingOrganizationChallenge = null;
            _verificationStatusError = AppLocalizations.of(
              context,
            )!.commonActionFailed;
          });
          return;
        }
        final challenge = data.pendingOrganizationChallenge;
        setState(() {
          _verificationStatusLoading = false;
          _verificationStatus = data;
          _pendingOrganizationChallenge =
              data.verificationType == 'none' &&
                  challenge != null &&
                  challenge.expiresAt.isAfter(DateTime.now())
              ? challenge
              : null;
          _verificationStatusError = null;
        });
      case UsersApiFailure(:final message, :final statusCode):
        setState(() {
          _verificationStatusLoading = false;
          _verificationStatus = null;
          _pendingOrganizationChallenge = null;
          _verificationStatusError = settingsLoadErrorMessage(
            AppLocalizations.of(context)!,
            message,
            statusCode: statusCode,
            fallback: AppLocalizations.of(context)!.commonActionFailed,
          );
        });
    }
  }

  String _verificationStatusLabel(
    AppLocalizations l10n,
    VerificationStatus status,
  ) => status.verificationType == 'none'
      ? l10n.verificationNotVerified
      : l10n.verificationVerified;

  void _syncObservedSession() {
    final authController = ref.read(authControllerProvider.notifier);
    final session = ref.read(authControllerProvider).session;
    _observedSessionInstallGeneration = authController.sessionInstallGeneration;
    _observedAccountId = session?.accountId;
    _observedProfileId = session?.activeProfileId;
    _hasObservedSession = true;
  }

  void _clearVerificationForSessionChange({required bool reload}) {
    _verificationStatusRequestGeneration++;
    final session = ref.read(authControllerProvider).session;
    setState(() {
      _selectedProfileId = session?.activeProfileId;
      _verificationStatusLoading = false;
      _verificationStatus = null;
      _pendingOrganizationChallenge = null;
      _verificationStatusError = session == null ? 'not_authenticated' : null;
    });
    if (reload && _showVerificationStatus && session != null) {
      _loadVerificationStatus();
    }
  }

  @override
  Widget build(BuildContext context) {
    if (!_hasObservedSession) _syncObservedSession();
    ref.listen(authControllerProvider, (previous, next) {
      final authController = ref.read(authControllerProvider.notifier);
      final session = next.session;
      final generation = authController.sessionInstallGeneration;
      final changed =
          generation != _observedSessionInstallGeneration ||
          session?.accountId != _observedAccountId ||
          session?.activeProfileId != _observedProfileId;
      if (!changed) return;
      _syncObservedSession();
      _clearForSessionChange(
        switching: ref.read(profileSwitchInProgressProvider),
      );
      _clearVerificationForSessionChange(
        reload: !ref.read(profileSwitchInProgressProvider),
      );
    });
    ref.listen<bool>(profileSwitchInProgressProvider, (previous, switching) {
      if (switching) {
        _clearForSessionChange(switching: true);
        _clearVerificationForSessionChange(reload: false);
      } else if (previous == true) {
        _syncObservedSession();
        _clearForSessionChange(switching: false);
        _clearVerificationForSessionChange(reload: true);
      }
    });
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);

    if (_showVerificationStatus) {
      return SafeArea(
        child: Padding(
          key: VerificationSettingsSheet.verificationStatusViewKey,
          padding: const EdgeInsets.fromLTRB(24, 16, 24, 24),
          child: SingleChildScrollView(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Row(
                  children: [
                    IconButton(
                      tooltip: MaterialLocalizations.of(
                        context,
                      ).backButtonTooltip,
                      onPressed: () =>
                          setState(() => _showVerificationStatus = false),
                      icon: const Icon(Icons.arrow_back),
                    ),
                    Expanded(
                      child: Text(
                        l10n.verificationStatusTitle,
                        style: Theme.of(context).textTheme.titleLarge,
                      ),
                    ),
                  ],
                ),
                if (_selectedProfileId != null)
                  Text(
                    l10n.verificationSelectedProfile(_selectedProfileId!),
                    style: TextStyle(color: voice.textSecondary),
                  ),
                const SizedBox(height: 16),
                if (_verificationStatusLoading)
                  const Center(child: CircularProgressIndicator())
                else if (_verificationStatusError != null) ...[
                  Text(
                    _verificationStatusError == 'not_authenticated'
                        ? l10n.settingsNotAuthenticated
                        : _verificationStatusError!,
                    style: TextStyle(color: voice.error),
                  ),
                  const SizedBox(height: 8),
                  TextButton.icon(
                    key: VerificationSettingsSheet.verificationStatusRetryKey,
                    onPressed: _loadVerificationStatus,
                    icon: const Icon(Icons.refresh),
                    label: Text(l10n.commonRetry),
                  ),
                ] else if (_verificationStatus != null)
                  Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      Semantics(
                        liveRegion: true,
                        child: ListTile(
                          key: VerificationSettingsSheet
                              .verificationStatusValueKey,
                          contentPadding: EdgeInsets.zero,
                          leading: Icon(
                            _verificationStatus!.verificationType == 'none'
                                ? Icons.verified_outlined
                                : Icons.verified,
                          ),
                          title: Text(
                            _verificationStatusLabel(
                              l10n,
                              _verificationStatus!,
                            ),
                          ),
                          subtitle: _verificationStatus!.badge == null
                              ? null
                              : Text(_verificationStatus!.badge!),
                        ),
                      ),
                      if (_pendingOrganizationChallenge != null) ...[
                        const SizedBox(height: 16),
                        _PendingOrganizationChallengeView(
                          challenge: _pendingOrganizationChallenge!,
                        ),
                      ],
                    ],
                  ),
              ],
            ),
          ),
        ),
      );
    }

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
                    key: VerificationSettingsSheet.verificationStatusKey,
                    onPressed: _openVerificationStatus,
                    icon: const Icon(Icons.verified_outlined),
                    label: Text(l10n.verificationStatusTitle),
                  ),
                  const SizedBox(height: 8),
                  OutlinedButton.icon(
                    key: VerificationSettingsSheet.organizationUnavailableKey,
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

class _PendingOrganizationChallengeView extends StatelessWidget {
  const _PendingOrganizationChallengeView({required this.challenge});

  final PendingOrganizationChallenge challenge;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final localExpiry = challenge.expiresAt.toLocal();
    final material = MaterialLocalizations.of(context);
    final expiry =
        '${material.formatMediumDate(localExpiry)} '
        '${material.formatTimeOfDay(TimeOfDay.fromDateTime(localExpiry))}';
    final textTheme = Theme.of(context).textTheme;

    return Semantics(
      container: true,
      child: Column(
        key: VerificationSettingsSheet.pendingChallengeKey,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            l10n.verificationPendingDnsChallenge,
            style: textTheme.titleSmall,
          ),
          const SizedBox(height: 8),
          Text(l10n.verificationPendingDnsDomainLabel),
          SelectableText(
            challenge.domain,
            key: VerificationSettingsSheet.pendingChallengeDomainKey,
          ),
          const SizedBox(height: 8),
          Text(l10n.verificationPendingDnsTxtRecordLabel),
          SelectableText(
            challenge.txtRecord,
            key: VerificationSettingsSheet.pendingChallengeTxtRecordKey,
          ),
          const SizedBox(height: 8),
          Text(
            '${l10n.verificationPendingDnsExpiresAtLabel}: $expiry',
            key: VerificationSettingsSheet.pendingChallengeExpiresAtKey,
          ),
        ],
      ),
    );
  }
}
