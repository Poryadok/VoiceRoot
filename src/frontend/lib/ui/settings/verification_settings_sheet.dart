import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/auth_client.dart';
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
        if (data.isNotEmpty) {
          ScaffoldMessenger.of(
            context,
          ).showSnackBar(SnackBar(content: Text('OAuth: $data')));
        }
      case AuthApiFailure(:final statusCode):
        setState(() {
          _error = commonActionErrorMessage(
            AppLocalizations.of(context)!,
            statusCode: statusCode,
          );
          _errorSource = _VerificationErrorSource.link;
        });
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
      case AuthApiFailure(:final statusCode):
        setState(() {
          _error = commonActionErrorMessage(
            AppLocalizations.of(context)!,
            statusCode: statusCode,
          );
          _errorSource = _VerificationErrorSource.unlink;
        });
    }
    if (mounted) setState(() => _busyPlatform = null);
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
      _clearVerificationForSessionChange(
        reload: !ref.read(profileSwitchInProgressProvider),
      );
    });
    ref.listen<bool>(profileSwitchInProgressProvider, (previous, switching) {
      if (switching) {
        _clearVerificationForSessionChange(reload: false);
      } else if (previous == true) {
        _syncObservedSession();
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
              Text(
                l10n.verificationLinkedAccountsTitle,
                style: TextStyle(color: voice.textSecondary),
              ),
              const SizedBox(height: 8),
              if (_loading)
                const Center(child: CircularProgressIndicator())
              else if (_error != null)
                Text(
                  _error == 'not_authenticated'
                      ? l10n.settingsNotAuthenticated
                      : _error!,
                  style: TextStyle(color: voice.error),
                )
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
