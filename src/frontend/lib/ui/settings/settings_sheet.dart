import 'package:flutter/services.dart';
import '../../theme/voice_layout.dart';
import '../a11y/voice_focus_return.dart';
import '../shell/desktop_shell_rail.dart';
import 'settings_scaffold.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../backend/users_client.dart';
import '../../state/auth_providers.dart';
import '../../state/social_providers.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_theme_providers.dart';
import '../core/voice_skeleton.dart';
import '../profile/create_profile_sheet.dart';
import '../profile/manage_profiles_sheet.dart';
import 'privacy_settings_screen.dart';
import 'notification_settings_screen.dart';
import 'security_settings_screen.dart';
import 'subscription_settings_screen.dart';
import 'help_sheet.dart';
import 'appeal_sheet.dart';
import 'appearance_settings_screen.dart';
import 'accessibility_settings_screen.dart';
import 'verification_settings_sheet.dart';

enum SettingsDestination {
  security,
  privacy,
  notifications,
  subscription,
  verification,
  help,
  appearance,
  accessibility,
  appeal,
}

class SettingsSheet extends ConsumerStatefulWidget {
  const SettingsSheet({super.key, this.initialDestination});

  final SettingsDestination? initialDestination;
  static const Key sheetKey = Key('settings_sheet');
  static const Key accentKey = Key('settings_accent');
  static const Key accessibilityKey = Key('settings_accessibility');

  static Future<void> show(BuildContext context) async {
    final focusReturn = VoiceFocusReturn.capture();
    await Navigator.of(
      context,
    ).push(MaterialPageRoute<void>(builder: (_) => const SettingsSheet()));
    focusReturn.restore();
  }

  @override
  ConsumerState<SettingsSheet> createState() => _SettingsSheetState();
}

class _SettingsSheetState extends ConsumerState<SettingsSheet> {
  late SettingsDestination _selected;
  final _leaveGuards = SettingsLeaveGuards();
  bool _leaving = false;

  @override
  void initState() {
    super.initState();
    _selected = widget.initialDestination ?? SettingsDestination.security;
  }

  Future<bool> _leaveDetail() async {
    return await _leaveGuards.canLeave() && mounted;
  }

  Future<void> _close() async {
    if (_leaving) return;
    _leaving = true;
    try {
      if (await _leaveDetail() && mounted && Navigator.of(context).canPop()) {
        Navigator.of(context).pop();
      }
    } finally {
      _leaving = false;
    }
  }

  Future<void> _select(SettingsDestination destination, bool desktop) async {
    if (!desktop) {
      await Navigator.of(
        context,
      ).push(MaterialPageRoute<void>(builder: (_) => _detail(destination)));
      return;
    }
    if (_selected == destination || !await _leaveDetail() || !mounted) return;
    setState(() => _selected = destination);
  }

  Widget _detail(SettingsDestination destination) => switch (destination) {
    SettingsDestination.security => const SecuritySettingsScreen(),
    SettingsDestination.privacy => const PrivacySettingsScreen(),
    SettingsDestination.notifications => const NotificationSettingsScreen(),
    SettingsDestination.subscription => const SubscriptionSettingsScreen(),
    SettingsDestination.verification => const Scaffold(
      body: VerificationSettingsSheet(),
    ),
    SettingsDestination.help => const Scaffold(body: HelpSheet()),
    SettingsDestination.appearance => const AppearanceSettingsScreen(),
    SettingsDestination.accessibility => const AccessibilitySettingsScreen(),
    SettingsDestination.appeal => const Scaffold(
      body: SingleChildScrollView(child: AppealSheet()),
    ),
  };

  String _title(AppLocalizations l10n, SettingsDestination destination) =>
      switch (destination) {
        SettingsDestination.security => l10n.securitySettingsTitle,
        SettingsDestination.privacy => l10n.privacySettingsTitle,
        SettingsDestination.notifications => l10n.notificationSettingsTitle,
        SettingsDestination.subscription => l10n.subscriptionSettingsTitle,
        SettingsDestination.verification => l10n.verificationSettingsTitle,
        SettingsDestination.help => l10n.settingsHelp,
        SettingsDestination.appearance => l10n.settingsAppearance,
        SettingsDestination.accessibility => l10n.settingsAccessibilityTitle,
        SettingsDestination.appeal => l10n.appealSettingsTitle,
      };

  Future<void> _logout() async {
    final l10n = AppLocalizations.of(context)!;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        key: const Key('settings_logout_confirm'),
        title: Text(l10n.authLogout),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: Text(l10n.commonCancel),
          ),
          TextButton(
            key: const Key('settings_logout_accept'),
            onPressed: () => Navigator.of(context).pop(true),
            child: Text(l10n.authLogout),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted || !await _leaveDetail() || !mounted) {
      return;
    }
    await ref.read(authControllerProvider.notifier).logout();
    if (mounted) Navigator.of(context).maybePop();
  }

  Widget _account() {
    final colors = VoiceColors.of(context);
    final profile = ref.watch(activeProfileProvider);
    final l10n = AppLocalizations.of(context)!;
    return Container(
      key: const Key('settings_account'),
      height: 80,
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        border: Border(bottom: BorderSide(color: colors.borderDefault)),
      ),
      child: profile.when(
        loading: () => const VoiceListSkeleton(rowCount: 1),
        error: (_, _) => Row(
          children: [
            Expanded(
              child: Text(
                l10n.backendUnavailable,
                style: TextStyle(color: colors.textSecondary, fontSize: 13),
              ),
            ),
            IconButton(
              tooltip: l10n.commonRetry,
              onPressed: () => ref.invalidate(activeProfileProvider),
              icon: const Icon(Icons.refresh),
            ),
          ],
        ),
        data: (profile) => Row(
          children: [
            CircleAvatar(
              radius: 20,
              backgroundColor: colors.profileAccent,
              backgroundImage: profile?.avatarUrl == null
                  ? null
                  : NetworkImage(profile!.avatarUrl!),
              child: profile?.avatarUrl != null
                  ? null
                  : Text(
                      profile?.displayName.characters.firstOrNull ?? '',
                      style: TextStyle(color: colors.textPrimary),
                    ),
            ),
            const SizedBox(width: 10),
            Expanded(
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    profile?.displayName ?? l10n.settingsTitle,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                      fontSize: 13,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  if (profile != null)
                    Text(
                      '@${profile.username}',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: 12,
                        color: colors.textSecondary,
                      ),
                    ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _navigation(bool desktop) {
    final l10n = AppLocalizations.of(context)!;
    final colors = VoiceColors.of(context);
    final profileId = ref.watch(authControllerProvider).activeProfileId;
    final catalog = ref.watch(voiceTokenCatalogProvider);
    return Column(
      children: [
        if (!desktop)
          Container(
            height: 56,
            decoration: BoxDecoration(
              border: Border(bottom: BorderSide(color: colors.borderDefault)),
            ),
            padding: const EdgeInsets.symmetric(horizontal: 8),
            child: Row(
              children: [
                IconButton(
                  key: const Key('settings_back'),
                  tooltip: MaterialLocalizations.of(context).backButtonTooltip,
                  onPressed: _close,
                  icon: const Icon(Icons.arrow_back, size: 20),
                ),
                const SizedBox(width: 4),
                Text(
                  l10n.settingsTitle,
                  style: const TextStyle(
                    fontSize: 16,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ],
            ),
          ),
        _account(),
        Expanded(
          child: ListView(
            padding: const EdgeInsets.only(top: 8, bottom: 16),
            children: [
              for (final destination in SettingsDestination.values)
                _SettingsRow(
                  key: Key('settings_${destination.name}'),
                  label: _title(l10n, destination),
                  selected: desktop && _selected == destination,
                  showChevron: !desktop,
                  onTap: () => _select(destination, desktop),
                ),
              _SettingsRow(
                key: const Key('settings_logout'),
                label: l10n.authLogout,
                color: colors.error,
                onTap: _logout,
              ),
              const Divider(height: 16),
              _SettingsRow(
                key: const Key('settings_create_profile'),
                label: l10n.createProfileAddAction,
                onTap: () => showCreateProfileSheet(context),
              ),
              _SettingsRow(
                key: const Key('settings_manage_profiles'),
                label: l10n.manageProfilesTitle,
                onTap: () => showManageProfilesSheet(context),
              ),
              if (profileId != null)
                Padding(
                  padding: const EdgeInsets.all(16),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        l10n.settingsAccent,
                        style: TextStyle(
                          color: colors.textSecondary,
                          fontSize: 12,
                        ),
                      ),
                      const SizedBox(height: 8),
                      catalog.when(
                        data: (catalog) => _AccentPicker(
                          key: SettingsSheet.accentKey,
                          profileId: profileId,
                          swatches: catalog.profileAccentDefaults,
                        ),
                        loading: () =>
                            const LinearProgressIndicator(minHeight: 2),
                        error: (_, _) => TextButton(
                          onPressed: () =>
                              ref.invalidate(voiceTokenCatalogProvider),
                          child: Text(l10n.commonRetry),
                        ),
                      ),
                    ],
                  ),
                ),
            ],
          ),
        ),
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    final colors = VoiceColors.of(context);
    return Scaffold(
      key: SettingsSheet.sheetKey,
      backgroundColor: colors.canvas,
      body: CallbackShortcuts(
        bindings: {const SingleActivator(LogicalKeyboardKey.escape): _close},
        child: Focus(
          autofocus: true,
          child: SafeArea(
            child: LayoutBuilder(
              builder: (context, constraints) {
                final desktop = !VoiceLayout.isNarrow(constraints.maxWidth);
                if (!desktop) return _navigation(false);
                return SettingsScaffold(
                  rail: DesktopShellRail(onOpenSettings: _close),
                  navigation: _navigation(true),
                  onClose: _close,
                  leaveGuards: _leaveGuards,
                  detail: _detail(_selected),
                );
              },
            ),
          ),
        ),
      ),
    );
  }
}

class _SettingsRow extends StatelessWidget {
  const _SettingsRow({
    super.key,
    required this.label,
    required this.onTap,
    this.selected = false,
    this.showChevron = false,
    this.color,
  });
  final String label;
  final VoidCallback onTap;
  final bool selected;
  final bool showChevron;
  final Color? color;

  @override
  Widget build(BuildContext context) {
    final colors = VoiceColors.of(context);
    return Semantics(
      selected: selected,
      button: true,
      child: InkWell(
        onTap: onTap,
        child: Container(
          constraints: const BoxConstraints(minHeight: 64),
          decoration: BoxDecoration(
            border: Border(
              left: BorderSide(
                width: 3,
                color: selected ? colors.profileAccent : colors.surface,
              ),
            ),
          ),
          padding: const EdgeInsets.fromLTRB(13, 12, 16, 12),
          child: Row(
            children: [
              Expanded(
                child: Text(
                  label,
                  style: TextStyle(
                    fontSize: 14,
                    height: 20 / 14,
                    fontWeight: selected ? FontWeight.w600 : FontWeight.w400,
                    color:
                        color ??
                        (selected ? colors.textPrimary : colors.textSecondary),
                  ),
                ),
              ),
              if (showChevron)
                Icon(Icons.chevron_right, size: 20, color: colors.textDisabled),
            ],
          ),
        ),
      ),
    );
  }
}

class _AccentPicker extends ConsumerStatefulWidget {
  const _AccentPicker({
    super.key,
    required this.profileId,
    required this.swatches,
  });

  final String profileId;
  final List<Color> swatches;

  @override
  ConsumerState<_AccentPicker> createState() => _AccentPickerState();
}

class _AccentPickerState extends ConsumerState<_AccentPicker> {
  int? _selectedIndex;
  int _selectionGeneration = 0;
  bool _saving = false;

  @override
  void initState() {
    super.initState();
    _loadSelection();
  }

  @override
  void didUpdateWidget(covariant _AccentPicker oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.profileId == widget.profileId) return;
    _selectionGeneration++;
    _saving = false;
    _selectedIndex = null;
    _loadSelection();
  }

  Future<void> _loadSelection() async {
    final generation = ++_selectionGeneration;
    final profileId = widget.profileId;
    VoiceProfile? profile;
    try {
      profile = await ref.read(profileProvider(profileId).future);
    } on Object {
      if (mounted && generation == _selectionGeneration) {
        setState(() => _selectedIndex = -1);
      }
      return;
    }
    if (!mounted ||
        generation != _selectionGeneration ||
        widget.profileId != profileId) {
      return;
    }

    final serverIndex = _indexForAccent(profile?.accentColor);
    if (profile != null && profile.accentColor?.isNotEmpty == true) {
      setState(() => _selectedIndex = serverIndex ?? -1);
      return;
    }
    if (profile == null) {
      setState(() => _selectedIndex = -1);
      return;
    }

    final storage = ref.read(profileAccentStorageProvider);
    final override = await storage.readOverride(profileId);
    final index = await storage.readProfileIndex(profileId);
    if (!mounted ||
        generation != _selectionGeneration ||
        widget.profileId != profileId) {
      return;
    }
    setState(
      () =>
          _selectedIndex = _indexForAccent(override) ?? _validIndex(index) ?? 0,
    );
  }

  Future<void> _select(int index) async {
    if (_saving || index < 0 || index >= widget.swatches.length) return;
    final profileId = widget.profileId;
    if (ref.read(authControllerProvider).activeProfileId != profileId) return;
    final storage = ref.read(profileAccentStorageProvider);
    final hex =
        '#${(widget.swatches[index].toARGB32() & 0xFFFFFF).toRadixString(16).padLeft(6, '0').toUpperCase()}';
    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    final generation = ++_selectionGeneration;
    setState(() => _saving = true);
    try {
      final result = await ref
          .read(voiceUsersClientProvider)
          .updateProfile(authorization: auth, accentColor: hex);
      if (result is! UsersApiOk<VoiceProfile>) return;

      await storage.clearOverride(profileId);
      await storage.clearProfileIndex(profileId);
      ref.invalidate(profileProvider(profileId));
      ref.invalidate(profileAccentColorProvider(profileId));
      VoiceProfile? updatedProfile;
      try {
        updatedProfile = await ref.read(profileProvider(profileId).future);
      } on Object {
        updatedProfile = result.data;
      }
      final authoritativeAccent =
          updatedProfile?.accentColor ?? result.data.accentColor;
      final selectedIndex = _indexForAccent(authoritativeAccent) ?? -1;
      if (mounted &&
          widget.profileId == profileId &&
          ref.read(authControllerProvider).activeProfileId == profileId &&
          generation == _selectionGeneration) {
        setState(() => _selectedIndex = selectedIndex);
      }
    } finally {
      if (mounted && widget.profileId == profileId) {
        setState(() => _saving = false);
      }
    }
  }

  int? _indexForAccent(String? hex) {
    if (hex == null || hex.isEmpty) return null;
    final normalized = hex.toUpperCase();
    for (var index = 0; index < widget.swatches.length; index++) {
      final candidate =
          '#${(widget.swatches[index].toARGB32() & 0xFFFFFF).toRadixString(16).padLeft(6, '0').toUpperCase()}';
      if (candidate == normalized) return index;
    }
    return null;
  }

  int? _validIndex(int? index) {
    if (index == null || index < 0 || index >= widget.swatches.length) {
      return null;
    }
    return index;
  }

  @override
  Widget build(BuildContext context) {
    final selected = _selectedIndex;
    if (selected == null) {
      return const SizedBox(height: 36, child: VoiceListSkeleton(rowCount: 1));
    }
    return Wrap(
      spacing: 8,
      runSpacing: 8,
      children: [
        for (var i = 0; i < widget.swatches.length; i++)
          GestureDetector(
            onTap: _saving ? null : () => _select(i),
            child: Container(
              width: 32,
              height: 32,
              decoration: BoxDecoration(
                color: widget.swatches[i],
                shape: BoxShape.circle,
                border: Border.all(
                  color: selected == i
                      ? VoiceColors.of(context).textPrimary
                      : VoiceColors.of(context).borderDefault,
                  width: selected == i ? 2 : 1,
                ),
              ),
            ),
          ),
      ],
    );
  }
}
