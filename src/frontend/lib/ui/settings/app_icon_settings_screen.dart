import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../services/app_icon_runtime_binding.dart';
import '../../settings/app_icon_preference.dart';
import '../../state/subscription_providers.dart';
import '../../theme/voice_colors.dart';

class AppIconSettingsScreen extends ConsumerStatefulWidget {
  const AppIconSettingsScreen({super.key});

  static const Key screenKey = Key('app_icon_settings_screen');
  static const Key gridKey = Key('app_icon_grid');
  static const Key applyKey = Key('app_icon_apply');

  static Key optionKey(AppIconPreference icon) => Key('app_icon_${icon.id}');

  @override
  ConsumerState<AppIconSettingsScreen> createState() =>
      _AppIconSettingsScreenState();
}

class _AppIconSettingsScreenState extends ConsumerState<AppIconSettingsScreen> {
  AppIconPreference? _draft;

  bool get _hostSupported =>
      kIsWeb || defaultTargetPlatform == TargetPlatform.windows;

  String _iconName(AppIconPreference icon) => switch (icon) {
    AppIconPreference.voiceSky => 'Voice Sky',
    AppIconPreference.midnight => 'Midnight',
    AppIconPreference.violet => 'Violet',
    AppIconPreference.sunrise => 'Sunrise',
    AppIconPreference.mint => 'Mint',
    AppIconPreference.coral => 'Coral',
  };

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final colors = VoiceColors.of(context);
    final preference = ref.watch(appIconPreferenceProvider);
    final subscription = ref.watch(subscriptionProvider);
    final runtime = ref.watch(appIconRuntimeBindingProvider);
    final saved = preference.valueOrNull ?? AppIconPreference.voiceSky;
    final selected = _draft ?? saved;
    final plus = subscription.valueOrNull?.isPremium == true;
    final statusKnown =
        subscription.hasValue &&
        !subscription.isLoading &&
        !subscription.hasError;
    final canApply =
        _hostSupported &&
        !runtime.applying &&
        !runtime.failed &&
        (!selected.requiresPlus || (statusKnown && plus));

    return Scaffold(
      key: AppIconSettingsScreen.screenKey,
      backgroundColor: colors.canvas,
      appBar: AppBar(
        backgroundColor: colors.surface,
        leading: IconButton(
          key: const Key('app_icon_back'),
          tooltip: MaterialLocalizations.of(context).backButtonTooltip,
          icon: const Icon(Icons.arrow_back),
          onPressed: () => Navigator.of(context).maybePop(),
        ),
        title: Text(l10n.appIconTitle),
      ),
      body: SafeArea(
        child: LayoutBuilder(
          builder: (context, constraints) => SingleChildScrollView(
            padding: const EdgeInsets.all(24),
            child: Align(
              alignment: Alignment.topCenter,
              child: ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 480),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Text(l10n.appIconDeviceNotice),
                    if (!_hostSupported) ...[
                      const SizedBox(height: 12),
                      Text(l10n.appIconUnavailable),
                    ],
                    if (subscription.hasError) ...[
                      const SizedBox(height: 12),
                      Text(l10n.appIconSubscriptionStatusError),
                      Align(
                        alignment: Alignment.centerLeft,
                        child: TextButton(
                          onPressed: () => ref.invalidate(subscriptionProvider),
                          child: Text(l10n.appIconRetry),
                        ),
                      ),
                    ],
                    if (subscription.isLoading) ...[
                      const SizedBox(height: 12),
                      Semantics(
                        liveRegion: true,
                        child: Text(l10n.appIconSubscriptionStatusChecking),
                      ),
                    ],
                    if (saved.requiresPlus && !plus && statusKnown) ...[
                      const SizedBox(height: 12),
                      Text(l10n.appIconSavedChoiceNotice),
                    ],
                    const SizedBox(height: 20),
                    GridView.count(
                      key: AppIconSettingsScreen.gridKey,
                      // The page's 24 px horizontal padding leaves 342 px at the
                      // reference's 390 px phone viewport, where two columns fit.
                      crossAxisCount: constraints.maxWidth < 428 ? 2 : 3,
                      mainAxisExtent: 112,
                      mainAxisSpacing: 12,
                      crossAxisSpacing: 12,
                      shrinkWrap: true,
                      physics: const NeverScrollableScrollPhysics(),
                      children: [
                        for (final icon in AppIconPreference.values)
                          _AppIconOption(
                            key: AppIconSettingsScreen.optionKey(icon),
                            icon: icon,
                            label: _iconName(icon),
                            selected: selected == icon,
                            onSelected: () => setState(() => _draft = icon),
                          ),
                      ],
                    ),
                    if (selected.requiresPlus && !plus && statusKnown) ...[
                      const SizedBox(height: 12),
                      Text(l10n.appIconPlusRequired),
                    ],
                    if (runtime.failed) ...[
                      const SizedBox(height: 12),
                      Text(l10n.appIconApplyFailure),
                      Align(
                        alignment: Alignment.centerLeft,
                        child: TextButton(
                          onPressed: runtime.applying
                              ? null
                              : () => ref
                                    .read(appIconRuntimeBindingProvider)
                                    .retryCurrent(),
                          child: Text(l10n.appIconRetry),
                        ),
                      ),
                    ],
                    const SizedBox(height: 20),
                    FilledButton(
                      key: AppIconSettingsScreen.applyKey,
                      onPressed: canApply
                          ? () => ref
                                .read(appIconRuntimeBindingProvider)
                                .applySelection(selected)
                          : null,
                      child: runtime.applying
                          ? const SizedBox(
                              width: 18,
                              height: 18,
                              child: CircularProgressIndicator(strokeWidth: 2),
                            )
                          : Text(l10n.appIconApply(_iconName(selected))),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _AppIconOption extends StatelessWidget {
  const _AppIconOption({
    super.key,
    required this.icon,
    required this.label,
    required this.selected,
    required this.onSelected,
  });

  final AppIconPreference icon;
  final String label;
  final bool selected;
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
        child: InkWell(
          onTap: onSelected,
          borderRadius: radius,
          child: Padding(
            padding: const EdgeInsets.all(10),
            child: Column(
              children: [
                Expanded(
                  child: Image.asset(
                    'assets/app_icons/${icon.id}.png',
                    fit: BoxFit.contain,
                    errorBuilder: (_, _, _) => const Icon(Icons.broken_image),
                  ),
                ),
                const SizedBox(height: 6),
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
    );
  }
}
