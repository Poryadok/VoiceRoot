import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../backend/users_client.dart';
import '../../state/auth_providers.dart';
import '../../state/social_providers.dart';
import '../../state/subscription_providers.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_theme_providers.dart';
import '../core/voice_bottom_sheet.dart';
import '../core/voice_skeleton.dart';
import '../profile/create_profile_sheet.dart';
import '../profile/manage_profiles_sheet.dart';
import 'privacy_settings_screen.dart';
import 'notification_settings_screen.dart';
import 'security_settings_screen.dart';
import 'subscription_settings_screen.dart';
import '../../settings/reduced_motion.dart';
import '../../settings/voice_input_settings.dart';
import 'help_sheet.dart';
import 'appeal_sheet.dart';
import 'appearance_settings_screen.dart';
import 'verification_settings_sheet.dart';

class SettingsSheet extends ConsumerWidget {
  const SettingsSheet({super.key});

  static const Key sheetKey = Key('settings_sheet');
  static const Key accentKey = Key('settings_accent');
  static const Key pttModeKey = Key('settings_ptt_mode');
  static const Key pttKeybindKey = Key('settings_ptt_keybind');

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final catalogAsync = ref.watch(voiceTokenCatalogProvider);
    final profileId = ref.watch(authControllerProvider).activeProfileId;
    final subscription = ref.watch(subscriptionProvider).valueOrNull;
    final reducedMotion = ref.watch(reducedMotionEnabledProvider);

    return SafeArea(
      child: Padding(
        key: sheetKey,
        padding: const EdgeInsets.fromLTRB(24, 16, 24, 24),
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                l10n.settingsTitle,
                style: Theme.of(context).textTheme.titleLarge,
              ),
              const SizedBox(height: 16),
              Text(
                l10n.settingsSecurity,
                style: TextStyle(color: voice.textSecondary),
              ),
              const SizedBox(height: 8),
              ListTile(
                key: const Key('settings_security'),
                contentPadding: EdgeInsets.zero,
                title: Text(l10n.securitySettingsTitle),
                trailing: const Icon(Icons.chevron_right),
                onTap: () {
                  Navigator.of(context).pop();
                  Navigator.of(context).push(
                    MaterialPageRoute<void>(
                      builder: (_) => const SecuritySettingsScreen(),
                    ),
                  );
                },
              ),
              ListTile(
                key: const Key('settings_appearance'),
                contentPadding: EdgeInsets.zero,
                title: Text(l10n.settingsAppearance),
                trailing: const Icon(Icons.chevron_right),
                onTap: () {
                  Navigator.of(context).pop();
                  Navigator.of(context).push(
                    MaterialPageRoute<void>(
                      builder: (_) => const AppearanceSettingsScreen(),
                    ),
                  );
                },
              ),
              ListTile(
                key: const Key('settings_appeal'),
                contentPadding: EdgeInsets.zero,
                title: Text(l10n.appealSettingsTitle),
                trailing: const Icon(Icons.chevron_right),
                onTap: () {
                  Navigator.of(context).pop();
                  showAppealSheet(context);
                },
              ),
              ListTile(
                key: const Key('settings_verification'),
                contentPadding: EdgeInsets.zero,
                title: Text(l10n.verificationSettingsTitle),
                trailing: const Icon(Icons.chevron_right),
                onTap: () {
                  Navigator.of(context).pop();
                  showVoiceBottomSheet<void>(
                    context: context,
                    child: const VerificationSettingsSheet(),
                  );
                },
              ),
              ListTile(
                key: const Key('settings_create_profile'),
                contentPadding: EdgeInsets.zero,
                title: Text(l10n.createProfileAddAction),
                trailing: const Icon(Icons.add),
                onTap: () async {
                  Navigator.of(context).pop();
                  await showCreateProfileSheet(context);
                },
              ),
              ListTile(
                key: const Key('settings_manage_profiles'),
                contentPadding: EdgeInsets.zero,
                title: Text(l10n.manageProfilesTitle),
                trailing: const Icon(Icons.chevron_right),
                onTap: () async {
                  Navigator.of(context).pop();
                  await showManageProfilesSheet(context);
                },
              ),
              ListTile(
                key: const Key('settings_privacy'),
                contentPadding: EdgeInsets.zero,
                title: Text(l10n.privacySettingsTitle),
                trailing: const Icon(Icons.chevron_right),
                onTap: () {
                  Navigator.of(context).pop();
                  Navigator.of(context).push(
                    MaterialPageRoute<void>(
                      builder: (_) => const PrivacySettingsScreen(),
                    ),
                  );
                },
              ),
              ListTile(
                key: const Key('settings_notifications'),
                contentPadding: EdgeInsets.zero,
                title: Text(l10n.notificationSettingsTitle),
                trailing: const Icon(Icons.chevron_right),
                onTap: () {
                  Navigator.of(context).pop();
                  Navigator.of(context).push(
                    MaterialPageRoute<void>(
                      builder: (_) => const NotificationSettingsScreen(),
                    ),
                  );
                },
              ),
              const SizedBox(height: 16),
              ListTile(
                key: const Key('settings_subscription'),
                contentPadding: EdgeInsets.zero,
                title: Text(l10n.subscriptionSettingsTitle),
                subtitle: subscription != null
                    ? Text(subscriptionPlanLabel(l10n, subscription))
                    : null,
                trailing: const Icon(Icons.chevron_right),
                onTap: () {
                  Navigator.of(context).pop();
                  Navigator.of(context).push(
                    MaterialPageRoute<void>(
                      builder: (_) => const SubscriptionSettingsScreen(),
                    ),
                  );
                },
              ),
              const SizedBox(height: 16),
              SwitchListTile(
                key: const Key('settings_reduced_motion'),
                contentPadding: EdgeInsets.zero,
                title: Text(l10n.settingsReducedMotion),
                value: reducedMotion,
                onChanged: (v) => ref
                    .read(reducedMotionEnabledProvider.notifier)
                    .setEnabled(v),
              ),
              SwitchListTile(
                key: pttModeKey,
                contentPadding: EdgeInsets.zero,
                title: Text(l10n.settingsPttMode),
                value:
                    ref.watch(voiceInputSettingsProvider).mode ==
                    VoiceInputMode.ptt,
                onChanged: (v) => ref
                    .read(voiceInputSettingsProvider.notifier)
                    .setMode(v ? VoiceInputMode.ptt : VoiceInputMode.vad),
              ),
              const _PttKeybindTile(),
              ListTile(
                key: const Key('settings_help'),
                contentPadding: EdgeInsets.zero,
                leading: const Icon(Icons.help_outline),
                title: Text(l10n.settingsHelp),
                onTap: () {
                  Navigator.of(context).pop();
                  HelpSheet.show(context);
                },
              ),
              const SizedBox(height: 16),
              if (profileId != null) ...[
                const SizedBox(height: 16),
                Text(
                  l10n.settingsAccent,
                  style: TextStyle(color: voice.textSecondary),
                ),
                const SizedBox(height: 8),
                catalogAsync.when(
                  data: (catalog) => _AccentPicker(
                    key: accentKey,
                    profileId: profileId,
                    swatches: catalog.profileAccentDefaults,
                  ),
                  loading: () => const LinearProgressIndicator(minHeight: 2),
                  error: (_, _) => const SizedBox.shrink(),
                ),
              ],
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

class _PttKeybindTile extends ConsumerStatefulWidget {
  const _PttKeybindTile();

  @override
  ConsumerState<_PttKeybindTile> createState() => _PttKeybindTileState();
}

class _PttKeybindTileState extends ConsumerState<_PttKeybindTile> {
  final _focusNode = FocusNode();
  var _capturing = false;

  @override
  void dispose() {
    _focusNode.dispose();
    super.dispose();
  }

  bool _isModifier(LogicalKeyboardKey key) {
    return key == LogicalKeyboardKey.control ||
        key == LogicalKeyboardKey.controlLeft ||
        key == LogicalKeyboardKey.controlRight ||
        key == LogicalKeyboardKey.shift ||
        key == LogicalKeyboardKey.shiftLeft ||
        key == LogicalKeyboardKey.shiftRight ||
        key == LogicalKeyboardKey.alt ||
        key == LogicalKeyboardKey.altLeft ||
        key == LogicalKeyboardKey.altRight ||
        key == LogicalKeyboardKey.meta ||
        key == LogicalKeyboardKey.metaLeft ||
        key == LogicalKeyboardKey.metaRight;
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final key = ref.watch(voiceInputSettingsProvider).pttKey;
    return Focus(
      focusNode: _focusNode,
      onKeyEvent: (node, event) {
        if (!_capturing || event is! KeyDownEvent) {
          return KeyEventResult.ignored;
        }
        if (_isModifier(event.logicalKey)) {
          return KeyEventResult.ignored;
        }
        ref
            .read(voiceInputSettingsProvider.notifier)
            .setPttKey(event.logicalKey);
        setState(() => _capturing = false);
        return KeyEventResult.handled;
      },
      child: ListTile(
        key: SettingsSheet.pttKeybindKey,
        contentPadding: EdgeInsets.zero,
        title: Text(l10n.settingsPttKeybind),
        subtitle: Text(
          _capturing
              ? l10n.settingsPttKeybindHint
              : (key.keyLabel.isNotEmpty
                    ? key.keyLabel
                    : key.debugName ?? key.keyId.toString()),
        ),
        onTap: () {
          setState(() => _capturing = true);
          _focusNode.requestFocus();
        },
      ),
    );
  }
}
