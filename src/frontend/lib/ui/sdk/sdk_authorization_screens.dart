import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../backend/sdk_authorization_client.dart';
import '../../backend/users_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/sdk_authorization_providers.dart';
import '../../state/subscription_providers.dart';

/// Consent page shows Auth's authoritative app/environment/scopes and asks the
/// user to explicitly select one of their own active profiles.
class SdkAuthorizationConsentScreen extends ConsumerStatefulWidget {
  const SdkAuthorizationConsentScreen({required this.requestId, super.key});
  final String requestId;

  @override
  ConsumerState<SdkAuthorizationConsentScreen> createState() =>
      _SdkAuthorizationConsentScreenState();
}

class _SdkAuthorizationConsentScreenState
    extends ConsumerState<SdkAuthorizationConsentScreen> {
  late Future<SdkConsentView> _consent;
  String? _selectedProfileId;
  bool _submitting = false;

  @override
  void initState() {
    super.initState();
    _consent = _loadConsent();
  }

  Future<SdkConsentView> _loadConsent() {
    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null) {
      return Future.error(
        const SdkAuthorizationException('Sign in to Voice first'),
      );
    }
    return ref
        .read(sdkAuthorizationClientProvider)
        .loadConsentView(requestId: widget.requestId, voiceAuthorization: auth);
  }

  Future<void> _approve(SdkConsentView consent) async {
    final profileId = _selectedProfileId;
    final authState = ref.read(authControllerProvider);
    final auth = authState.session?.authorizationHeader;
    if (profileId == null || auth == null || authState.isGuest) return;
    setState(() => _submitting = true);
    try {
      final approval = await ref
          .read(sdkAuthorizationClientProvider)
          .approveAuthorization(
            requestId: widget.requestId,
            voiceAuthorization: auth,
            profileId: profileId,
            policyRevision: consent.policyRevision,
          );
      if (!mounted) return;
      // Callback consumption is a separate route and receives the URI as data;
      // it does not inspect or depend on WebView/browser navigation events.
      context.go(
        '/sdk/authorization/callback',
        extra: Uri.parse(approval.redirectUri),
      );
    } on Object catch (error) {
      if (mounted) _showError(error);
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  void _showError(Object error) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(
          error is SdkAuthorizationException
              ? error.message
              : 'Authorization failed',
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authControllerProvider);
    final l10n = AppLocalizations.of(context)!;
    final session = authState.session;
    final profileList = ref.watch(myProfilesProvider);
    return Scaffold(
      appBar: AppBar(title: Text(l10n.sdkAuthorizationTitle)),
      body: FutureBuilder<SdkConsentView>(
        future: _consent,
        builder: (context, snapshot) {
          if (snapshot.hasError) {
            return Center(child: Text(l10n.sdkAuthorizationLoadError));
          }
          if (!snapshot.hasData) {
            return const Center(child: CircularProgressIndicator());
          }
          final consent = snapshot.data!;
          final profiles =
              profileList.valueOrNull
                  ?.where(
                    (profile) =>
                        session != null &&
                        profile.accountId == session.accountId &&
                        !profile.isFrozen,
                  )
                  .toList(growable: false) ??
              const <VoiceProfile>[];
          final regular =
              session?.accountType == 'regular' && !authState.isGuest;
          return SdkConsentApprovalPanel(
            l10n: l10n,
            consent: consent,
            profiles: profiles,
            profilesLoading: profileList.isLoading,
            profilesFailed: profileList.hasError,
            canApprove: regular,
            selectedProfileId: _selectedProfileId,
            submitting: _submitting,
            onProfileSelected: (id) => setState(() => _selectedProfileId = id),
            onApprove: () => _approve(consent),
            onCancel: () => context.pop(),
          );
        },
      ),
    );
  }
}

class SdkConsentApprovalPanel extends StatelessWidget {
  const SdkConsentApprovalPanel({
    required this.l10n,
    required this.consent,
    required this.profiles,
    required this.profilesLoading,
    required this.profilesFailed,
    required this.canApprove,
    required this.selectedProfileId,
    required this.submitting,
    required this.onProfileSelected,
    required this.onApprove,
    required this.onCancel,
    super.key,
  });
  final AppLocalizations l10n;
  final SdkConsentView consent;
  final List<VoiceProfile> profiles;
  final bool profilesLoading;
  final bool profilesFailed;
  final bool canApprove;
  final String? selectedProfileId;
  final bool submitting;
  final ValueChanged<String?> onProfileSelected;
  final VoidCallback onApprove;
  final VoidCallback onCancel;

  @override
  Widget build(BuildContext context) => ListView(
    padding: const EdgeInsets.all(24),
    children: [
      Text(
        consent.displayName,
        style: Theme.of(context).textTheme.headlineSmall,
      ),
      const SizedBox(height: 16),
      _Detail(
        label: l10n.sdkAuthorizationApplication,
        value: consent.applicationId,
      ),
      _Detail(
        label: l10n.sdkAuthorizationEnvironment,
        value: consent.environmentId,
      ),
      _Detail(
        label: l10n.sdkAuthorizationGameAccount,
        value: consent.gameSubject,
      ),
      const SizedBox(height: 12),
      Text(
        l10n.sdkAuthorizationPermissions,
        style: Theme.of(context).textTheme.titleMedium,
      ),
      for (final scope in consent.scopes.toList()..sort())
        ListTile(leading: const Icon(Icons.lock_outline), title: Text(scope)),
      const Divider(),
      Text(
        l10n.sdkAuthorizationSelectProfile,
        style: Theme.of(context).textTheme.titleMedium,
      ),
      if (profilesLoading) const LinearProgressIndicator(),
      if (profilesFailed) Text(l10n.sdkAuthorizationProfilesError),
      IgnorePointer(
        ignoring: submitting,
        child: RadioGroup<String>(
          groupValue: selectedProfileId,
          onChanged: onProfileSelected,
          child: Column(
            children: [
              for (final profile in profiles)
                RadioListTile<String>(
                  value: profile.id,
                  title: Text(profile.displayName),
                  subtitle: Text(profile.handle),
                ),
            ],
          ),
        ),
      ),
      if (profiles.isEmpty && !profilesLoading)
        Text(l10n.sdkAuthorizationNoProfiles),
      const SizedBox(height: 12),
      FilledButton(
        onPressed: canApprove && selectedProfileId != null && !submitting
            ? onApprove
            : null,
        child: submitting
            ? const CircularProgressIndicator()
            : Text(l10n.sdkAuthorizationApprove),
      ),
      TextButton(
        onPressed: submitting ? null : onCancel,
        child: Text(l10n.sdkAuthorizationCancel),
      ),
      if (!canApprove) Text(l10n.sdkAuthorizationRegularRequired),
    ],
  );
}

class SdkAuthorizationCallbackScreen extends ConsumerStatefulWidget {
  const SdkAuthorizationCallbackScreen({required this.callback, super.key});
  final Uri callback;

  @override
  ConsumerState<SdkAuthorizationCallbackScreen> createState() =>
      _SdkAuthorizationCallbackScreenState();
}

class _SdkAuthorizationCallbackScreenState
    extends ConsumerState<SdkAuthorizationCallbackScreen> {
  late Future<SdkLinkedSession> _resume;

  @override
  void initState() {
    super.initState();
    _resume = ref
        .read(sdkAuthorizationClientProvider)
        .acceptCallbackAndResume(widget.callback);
  }

  @override
  Widget build(BuildContext context) => Scaffold(
    appBar: AppBar(
      title: Text(AppLocalizations.of(context)!.sdkAuthorizationCallbackTitle),
    ),
    body: FutureBuilder<SdkLinkedSession>(
      future: _resume,
      builder: (context, snapshot) {
        if (snapshot.hasError) {
          final error = snapshot.error;
          if (error is SdkAuthorizationException && error.canRetryResume) {
            final l10n = AppLocalizations.of(context)!;
            return Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(l10n.sdkAuthorizationResumeTemporaryError),
                  const SizedBox(height: 12),
                  FilledButton(
                    onPressed: () => setState(() {
                      _resume = ref
                          .read(sdkAuthorizationClientProvider)
                          .resumePendingLinkedSession();
                    }),
                    child: Text(l10n.sdkAuthorizationRetryResume),
                  ),
                ],
              ),
            );
          }
          return Center(
            child: Text(
              AppLocalizations.of(context)!.sdkAuthorizationCallbackInvalid,
            ),
          );
        }
        if (!snapshot.hasData) {
          return const Center(child: CircularProgressIndicator());
        }
        final session = snapshot.data!;
        return Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const Icon(Icons.check_circle_outline, size: 48),
              Text(
                AppLocalizations.of(
                  context,
                )!.sdkAuthorizationConnected(session.applicationId),
              ),
              Text(
                AppLocalizations.of(
                  context,
                )!.sdkAuthorizationProfile(session.profileId),
              ),
              Text(
                AppLocalizations.of(
                  context,
                )!.sdkAuthorizationGrantedPermissions(
                  (session.scopes.toList()..sort()).join(', '),
                ),
              ),
            ],
          ),
        );
      },
    ),
  );
}

class _Detail extends StatelessWidget {
  const _Detail({required this.label, required this.value});
  final String label;
  final String value;
  @override
  Widget build(BuildContext context) => ListTile(
    dense: true,
    title: Text(label),
    subtitle: SelectableText(value),
  );
}
