import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/users_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/social_providers.dart';
import '../../theme/voice_theme_providers.dart';
import '../api_error_messages.dart';

/// The existing active-profile EN/RU preference control, hosted by Appearance.
class ProfileLanguagePicker extends ConsumerStatefulWidget {
  const ProfileLanguagePicker({super.key});

  static const Key controlKey = Key('settings_language');

  @override
  ConsumerState<ProfileLanguagePicker> createState() =>
      _ProfileLanguagePickerState();
}

class _ProfileLanguagePickerState extends ConsumerState<ProfileLanguagePicker> {
  late final ProviderSubscription<AuthState> _contextSubscription;
  int _generation = 0;
  bool _saving = false;
  String? _retryLocale;
  int? _errorStatusCode;

  @override
  void initState() {
    super.initState();
    _contextSubscription = ref.listenManual(authControllerProvider, (
      previous,
      next,
    ) {
      if (previous?.activeProfileId == next.activeProfileId &&
          previous?.session?.authorizationHeader ==
              next.session?.authorizationHeader) {
        return;
      }
      _generation++;
      if (mounted) {
        setState(() {
          _saving = false;
          _retryLocale = null;
          _errorStatusCode = null;
        });
      }
    });
  }

  @override
  void dispose() {
    _contextSubscription.close();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final locale = ref.watch(appLocalePreferenceProvider);
    final selected = switch (locale?.languageCode) {
      'en' => 'en',
      'ru' => 'ru',
      _ => 'system',
    };
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        SegmentedButton<String>(
          key: ProfileLanguagePicker.controlKey,
          segments: [
            ButtonSegment(
              value: 'system',
              label: Text(l10n.settingsLanguageSystem),
            ),
            ButtonSegment(value: 'en', label: Text(l10n.settingsLanguageEn)),
            ButtonSegment(value: 'ru', label: Text(l10n.settingsLanguageRu)),
          ],
          selected: {selected},
          onSelectionChanged: _saving
              ? null
              : (next) {
                  final choice = next.single;
                  if (choice == 'system') {
                    final profileId = ref
                        .read(authControllerProvider)
                        .activeProfileId;
                    ref
                        .read(appLocaleOverrideProvider.notifier)
                        .state = ProfileLocaleOverride(
                      profileId: profileId,
                      locale: null,
                    );
                    setState(() {
                      _retryLocale = null;
                      _errorStatusCode = null;
                    });
                  } else {
                    _save(choice);
                  }
                },
        ),
        if (_saving) ...[
          const SizedBox(height: 8),
          const LinearProgressIndicator(minHeight: 2),
        ],
        if (_errorStatusCode != null || _retryLocale != null && !_saving)
          Padding(
            padding: const EdgeInsets.only(top: 8),
            child: Row(
              children: [
                Expanded(
                  child: Text(
                    commonActionErrorMessage(
                      l10n,
                      statusCode: _errorStatusCode,
                    ),
                    key: const Key('settings_language_error'),
                  ),
                ),
                TextButton(
                  onPressed: _retryLocale == null
                      ? null
                      : () => _save(_retryLocale!),
                  child: Text(l10n.commonRetry),
                ),
              ],
            ),
          ),
      ],
    );
  }

  Future<void> _save(String locale) async {
    if (_saving || (locale != 'en' && locale != 'ru')) return;
    final profileId = ref.read(authControllerProvider).activeProfileId;
    final authorization = ref.read(authorizationHeaderProvider);
    if (profileId == null || authorization == null) return;
    final generation = ++_generation;
    setState(() {
      _saving = true;
      _retryLocale = locale;
      _errorStatusCode = null;
    });
    try {
      final result = await ref
          .read(voiceUsersClientProvider)
          .updateProfile(authorization: authorization, locale: locale);
      if (!_isCurrent(generation, profileId, authorization)) return;
      if (result case UsersApiOk<VoiceProfile>(
        :final data,
      ) when data.id == profileId && data.locale == locale) {
        ref.read(appLocaleOverrideProvider.notifier).state =
            ProfileLocaleOverride(profileId: profileId, locale: Locale(locale));
        ref.invalidate(profileProvider(profileId));
        ref.invalidate(activeProfileProvider);
        setState(() {
          _retryLocale = null;
          _errorStatusCode = null;
        });
        return;
      }
      final statusCode = switch (result) {
        UsersApiFailure(:final statusCode) => statusCode,
        _ => null,
      };
      setState(() => _errorStatusCode = statusCode);
    } on Object {
      if (_isCurrent(generation, profileId, authorization)) {
        setState(() => _errorStatusCode = null);
      }
    } finally {
      if (_isCurrent(generation, profileId, authorization)) {
        setState(() => _saving = false);
      }
    }
  }

  bool _isCurrent(int generation, String profileId, String authorization) =>
      mounted &&
      generation == _generation &&
      ref.read(authControllerProvider).activeProfileId == profileId &&
      ref.read(authorizationHeaderProvider) == authorization;
}
