import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/subscription_providers.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_theme_providers.dart';
import 'privacy_settings_screen.dart';
import 'security_settings_screen.dart';
import 'subscription_settings_screen.dart';
import 'verification_settings_sheet.dart';

class SettingsSheet extends ConsumerWidget {
  const SettingsSheet({super.key});

  static const Key sheetKey = Key('settings_sheet');
  static const Key themeKey = Key('settings_theme');
  static const Key languageKey = Key('settings_language');
  static const Key accentKey = Key('settings_accent');

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final themePref = ref.watch(appThemePreferenceProvider);
    final localePref = ref.watch(appLocalePreferenceProvider);
    final catalogAsync = ref.watch(voiceTokenCatalogProvider);
    final profileId = ref.watch(authControllerProvider).activeProfileId;
    final subscription = ref.watch(subscriptionProvider).valueOrNull;

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
            Text(l10n.settingsSecurity, style: TextStyle(color: voice.textSecondary)),
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
              key: const Key('settings_verification'),
              contentPadding: EdgeInsets.zero,
              title: Text(l10n.verificationSettingsTitle),
              trailing: const Icon(Icons.chevron_right),
              onTap: () {
                Navigator.of(context).pop();
                showModalBottomSheet<void>(
                  context: context,
                  isScrollControlled: true,
                  builder: (_) => const VerificationSettingsSheet(),
                );
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
            const SizedBox(height: 16),
            ListTile(
              key: const Key('settings_subscription'),
              contentPadding: EdgeInsets.zero,
              title: Text(l10n.subscriptionSettingsTitle),
              subtitle: subscription != null
                  ? Text(subscription.plan)
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
            Text(l10n.settingsTheme, style: TextStyle(color: voice.textSecondary)),
            const SizedBox(height: 8),
            SegmentedButton<AppThemePreference>(
              key: themeKey,
              segments: [
                ButtonSegment(
                  value: AppThemePreference.system,
                  label: Text(l10n.settingsThemeSystem),
                ),
                ButtonSegment(
                  value: AppThemePreference.light,
                  label: Text(l10n.settingsThemeLight),
                ),
                ButtonSegment(
                  value: AppThemePreference.dark,
                  label: Text(l10n.settingsThemeDark),
                ),
                ButtonSegment(
                  value: AppThemePreference.highContrast,
                  label: Text(l10n.settingsThemeHighContrast),
                ),
              ],
              selected: {themePref},
              onSelectionChanged: (next) =>
                  ref.read(appThemePreferenceProvider.notifier).state =
                      next.single,
            ),
            const SizedBox(height: 16),
            Text(l10n.settingsLanguage, style: TextStyle(color: voice.textSecondary)),
            const SizedBox(height: 8),
            SegmentedButton<String>(
              key: languageKey,
              segments: [
                ButtonSegment(
                  value: 'system',
                  label: Text(l10n.settingsLanguageSystem),
                ),
                ButtonSegment(
                  value: 'en',
                  label: Text(l10n.settingsLanguageEn),
                ),
                ButtonSegment(
                  value: 'ru',
                  label: Text(l10n.settingsLanguageRu),
                ),
              ],
              selected: {
                localePref == null ? 'system' : localePref.languageCode,
              },
              onSelectionChanged: (next) {
                final v = next.single;
                ref.read(appLocalePreferenceProvider.notifier).state =
                    v == 'system' ? null : Locale(v);
              },
            ),
            if (profileId != null) ...[
              const SizedBox(height: 16),
              Text(l10n.settingsAccent, style: TextStyle(color: voice.textSecondary)),
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

  @override
  void initState() {
    super.initState();
    _loadIndex();
  }

  Future<void> _loadIndex() async {
    final storage = ref.read(profileAccentStorageProvider);
    final index = await storage.readProfileIndex(widget.profileId);
    if (!mounted) return;
    setState(() => _selectedIndex = index ?? 0);
  }

  Future<void> _select(int index) async {
    final storage = ref.read(profileAccentStorageProvider);
    await storage.writeProfileIndex(widget.profileId, index);
    await storage.clearOverride(widget.profileId);
    ref.invalidate(profileAccentColorProvider(widget.profileId));
    setState(() => _selectedIndex = index);
  }

  @override
  Widget build(BuildContext context) {
    final selected = _selectedIndex;
    if (selected == null) {
      return const SizedBox(height: 36, child: Center(child: CircularProgressIndicator(strokeWidth: 2)));
    }
    return Wrap(
      spacing: 8,
      runSpacing: 8,
      children: [
        for (var i = 0; i < widget.swatches.length; i++)
          GestureDetector(
            onTap: () => _select(i),
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
