import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter/services.dart';

import '../../l10n/app_localizations.dart';
import '../../settings/theme_preference.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_theme.dart';
import '../../theme/voice_theme_providers.dart';
import 'profile_language_picker.dart';

/// Dedicated, responsive Appearance destination reached from Settings.
class AppearanceSettingsScreen extends ConsumerWidget {
  const AppearanceSettingsScreen({super.key});

  static const Key screenKey = Key('appearance_settings_screen');
  static const Key themeKey = Key('appearance_theme_picker');
  static const Key languageKey = ProfileLanguagePicker.controlKey;
  static const Key backKey = Key('appearance_back');

  static Key themeOptionKey(AppThemePreference preference) =>
      Key('appearance_theme_${preference.name}');

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final colors = VoiceColors.of(context);
    final preference = ref.watch(appThemePreferenceProvider);

    return Scaffold(
      key: screenKey,
      backgroundColor: colors.canvas,
      appBar: AppBar(
        backgroundColor: colors.surface,
        leading: IconButton(
          key: backKey,
          tooltip: MaterialLocalizations.of(context).backButtonTooltip,
          icon: const Icon(Icons.arrow_back),
          onPressed: () => Navigator.of(context).maybePop(),
        ),
        title: Text(l10n.settingsAppearance),
      ),
      body: SafeArea(
        child: LayoutBuilder(
          builder: (context, constraints) {
            return SingleChildScrollView(
              padding: const EdgeInsets.all(24),
              child: Align(
                alignment: Alignment.topCenter,
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 480),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      Text(
                        l10n.settingsTheme,
                        style: TextStyle(color: colors.textSecondary),
                      ),
                      const SizedBox(height: 8),
                      GridView.count(
                        key: themeKey,
                        crossAxisCount: 2,
                        mainAxisExtent: 112,
                        mainAxisSpacing: 10,
                        crossAxisSpacing: 10,
                        shrinkWrap: true,
                        physics: const NeverScrollableScrollPhysics(),
                        children: [
                          for (final mode in AppThemePreference.values)
                            _ThemeOption(
                              key: themeOptionKey(mode),
                              preference: mode,
                              selected: preference == mode,
                              label: _label(l10n, mode),
                              onSelected: () => ref
                                  .read(appThemePreferenceProvider.notifier)
                                  .setPreference(mode),
                            ),
                        ],
                      ),
                      const SizedBox(height: 24),
                      Text(
                        l10n.settingsLanguage,
                        style: TextStyle(color: colors.textSecondary),
                      ),
                      const SizedBox(height: 8),
                      const ProfileLanguagePicker(),
                    ],
                  ),
                ),
              ),
            );
          },
        ),
      ),
    );
  }

  String _label(AppLocalizations l10n, AppThemePreference preference) =>
      switch (preference) {
        AppThemePreference.system => l10n.settingsThemeSystem,
        AppThemePreference.light => l10n.settingsThemeLight,
        AppThemePreference.dark => l10n.settingsThemeDark,
        AppThemePreference.highContrast => l10n.settingsThemeHighContrast,
      };
}

class _ThemeOption extends StatelessWidget {
  const _ThemeOption({
    super.key,
    required this.preference,
    required this.selected,
    required this.label,
    required this.onSelected,
  });

  final AppThemePreference preference;
  final bool selected;
  final String label;
  final VoidCallback onSelected;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final colors = VoiceColors.of(context);
    final radius = BorderRadius.circular(8);

    return Semantics(
      label: label,
      button: true,
      selected: selected,
      container: true,
      excludeSemantics: true,
      child: Material(
        color: scheme.surface,
        shape: RoundedRectangleBorder(
          borderRadius: radius,
          side: BorderSide(
            color: selected ? scheme.primary : colors.borderDefault,
            width: selected ? 2 : 1,
          ),
        ),
        clipBehavior: Clip.antiAlias,
        child: FocusableActionDetector(
          actions: {
            ActivateIntent: CallbackAction<ActivateIntent>(
              onInvoke: (_) {
                onSelected();
                return null;
              },
            ),
          },
          shortcuts: {
            SingleActivator(LogicalKeyboardKey.enter): ActivateIntent(),
            SingleActivator(LogicalKeyboardKey.space): ActivateIntent(),
          },
          child: InkWell(
            onTap: onSelected,
            borderRadius: radius,
            child: Padding(
              padding: const EdgeInsets.all(10),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Expanded(child: _ThemePreview(preference: preference)),
                  const SizedBox(height: 7),
                  Row(
                    children: [
                      Expanded(
                        child: Text(
                          label,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: Theme.of(context).textTheme.labelMedium,
                        ),
                      ),
                      if (selected) ...[
                        const SizedBox(width: 4),
                        Icon(Icons.check, size: 16, color: scheme.primary),
                      ],
                    ],
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _ThemePreview extends StatelessWidget {
  const _ThemePreview({required this.preference});

  final AppThemePreference preference;

  @override
  Widget build(BuildContext context) {
    final swatch = switch (preference) {
      AppThemePreference.system => VoiceTheme.preferenceSystemSwatch,
      AppThemePreference.light => VoiceTheme.preferenceLightSwatch,
      AppThemePreference.dark => VoiceTheme.preferenceDarkSwatch,
      AppThemePreference.highContrast =>
        VoiceTheme.preferenceHighContrastSwatch,
    };

    return ClipRRect(
      borderRadius: BorderRadius.circular(6),
      child: DecoratedBox(
        decoration: BoxDecoration(
          border: Border.all(color: swatch.border),
          gradient: preference == AppThemePreference.system
              ? LinearGradient(
                  colors: [swatch.background, swatch.secondBackground!],
                  stops: const [0.5, 0.5],
                )
              : null,
          color: preference == AppThemePreference.system
              ? null
              : swatch.background,
        ),
        child: Align(
          alignment: Alignment.bottomLeft,
          child: Container(
            width: 36,
            height: 18,
            margin: const EdgeInsets.all(5),
            decoration: BoxDecoration(
              color: swatch.foreground,
              borderRadius: BorderRadius.circular(3),
            ),
          ),
        ),
      ),
    );
  }
}
