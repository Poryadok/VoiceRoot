import 'dart:typed_data';

import 'package:file_selector/file_selector.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/users_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/social_providers.dart';
import '../../state/subscription_providers.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_theme_providers.dart';
import '../../backend/user_privacy_client.dart';
import '../../state/trust_providers.dart';
import 'package:flutter/services.dart' show LogicalKeyboardKey;
import '../settings/privacy_presets.dart';
import '../matchmaking/player_profile_sheet.dart';

typedef ProfileAvatarPicker = Future<ProfileAvatarFile?> Function();

class ProfileAvatarFile {
  const ProfileAvatarFile({
    required this.bytes,
    required this.contentType,
    required this.name,
  });

  final Uint8List bytes;
  final String contentType;
  final String name;
}

class ProfileEditSheet extends ConsumerStatefulWidget {
  const ProfileEditSheet({super.key, required this.profile, this.avatarPicker});

  static const Key sheetKey = Key('profile_edit_sheet');
  static const Key displayNameFieldKey = Key('profile_edit_display_name');
  static const Key bioFieldKey = Key('profile_edit_bio');
  static const Key avatarButtonKey = Key('profile_edit_avatar');
  static const Key saveButtonKey = Key('profile_edit_save');

  final VoiceProfile profile;
  final ProfileAvatarPicker? avatarPicker;

  @override
  ConsumerState<ProfileEditSheet> createState() => _ProfileEditSheetState();
}

class _ProfileEditSheetState extends ConsumerState<ProfileEditSheet> {
  late final TextEditingController _displayNameController;
  late final TextEditingController _bioController;
  final _nameFocus = FocusNode();
  Object? _identity;
  String? _accent;
  String? _preset;
  String? _initialPreset;
  bool _privacyLoading = true;
  bool _privacyFailed = false;
  late String _savedName;
  late String _savedBio;
  String? _savedAccent;
  String? _uploadedAvatarUrl;
  ProfileAvatarFile? _avatar;
  var _saving = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _displayNameController = TextEditingController(
      text: widget.profile.displayName,
    );
    _bioController = TextEditingController(text: widget.profile.bio ?? '');
    _savedName = widget.profile.displayName;
    _savedBio = widget.profile.bio ?? '';
    _accent = _savedAccent = widget.profile.accentColor;
    _identity = ref
        .read(authControllerProvider.notifier)
        .gatewayRequestIdentity;
    _displayNameController.addListener(_draftChanged);
    _bioController.addListener(_draftChanged);
    _loadPrivacy();
  }

  void _draftChanged() => setState(() {});

  bool get _currentIdentity {
    final session = ref.read(authControllerProvider).session;
    return session?.accountId == widget.profile.accountId &&
        session?.activeProfileId == widget.profile.id &&
        _identity ==
            ref.read(authControllerProvider.notifier).gatewayRequestIdentity;
  }

  bool get _profileDirty =>
      _displayNameController.text.trim() != _savedName ||
      _bioController.text != _savedBio ||
      _avatar != null ||
      _accent != _savedAccent;
  bool get _dirty =>
      _profileDirty || (_preset != null && _preset != _initialPreset);

  Future<void> _loadPrivacy() async {
    final auth = ref.read(authorizationHeaderProvider);
    if (!_currentIdentity || auth == null) {
      if (mounted) {
        setState(() {
          _privacyLoading = false;
          _privacyFailed = true;
        });
      }
      return;
    }
    setState(() {
      _privacyLoading = true;
      _privacyFailed = false;
    });
    final result = await ref
        .read(voiceUserPrivacyClientProvider)
        .getPrivacy(authorization: auth);
    if (!mounted || !_currentIdentity) return;
    setState(() {
      _privacyLoading = false;
      switch (result) {
        case UserPrivacyApiOk(:final data):
          if (data.profileId != widget.profile.id) {
            _privacyFailed = true;
            break;
          }
          _preset = _initialPreset = data.preset;
        case UserPrivacyApiFailure():
          _privacyFailed = true;
      }
    });
  }

  @override
  void dispose() {
    _displayNameController.dispose();
    _bioController.dispose();
    _nameFocus.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    ref.watch(authControllerProvider);
    final catalog = ref.watch(voiceTokenCatalogProvider).valueOrNull;
    return Shortcuts(
      shortcuts: const {
        SingleActivator(LogicalKeyboardKey.escape): DismissIntent(),
      },
      child: Actions(
        actions: {
          DismissIntent: CallbackAction<DismissIntent>(
            onInvoke: (_) {
              if (!_saving) Navigator.of(context).maybePop();
              return null;
            },
          ),
        },
        child: FocusTraversalGroup(
          child: SafeArea(
            child: SingleChildScrollView(
              child: Center(
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 720),
                  child: Padding(
                    key: ProfileEditSheet.sheetKey,
                    padding: const EdgeInsets.fromLTRB(24, 16, 24, 24),
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        Row(
                          children: [
                            Expanded(
                              child: Text(
                                l10n.profileEditTitle,
                                style: Theme.of(context).textTheme.titleLarge,
                              ),
                            ),
                            IconButton(
                              tooltip: MaterialLocalizations.of(
                                context,
                              ).closeButtonTooltip,
                              onPressed: _saving
                                  ? null
                                  : () => Navigator.of(context).maybePop(),
                              icon: const Icon(Icons.close),
                            ),
                          ],
                        ),
                        const SizedBox(height: 16),
                        Row(
                          children: [
                            CircleAvatar(
                              radius: 28,
                              backgroundImage: _avatar != null
                                  ? MemoryImage(_avatar!.bytes)
                                  : (widget.profile.avatarUrl != null
                                        ? NetworkImage(
                                            widget.profile.avatarUrl!,
                                          )
                                        : null),
                              child:
                                  _avatar == null &&
                                      widget.profile.avatarUrl == null
                                  ? Text(_avatarFallback)
                                  : null,
                            ),
                            const SizedBox(width: 16),
                            Flexible(
                              child: OutlinedButton.icon(
                                key: ProfileEditSheet.avatarButtonKey,
                                onPressed: _saving ? null : _pickAvatar,
                                icon: const Icon(Icons.image_outlined),
                                label: Text(l10n.profileAvatarChange),
                              ),
                            ),
                          ],
                        ),
                        if (_avatar != null) ...[
                          const SizedBox(height: 8),
                          Text(
                            l10n.profileAvatarSelected(_avatar!.name),
                            style: TextStyle(color: voice.textSecondary),
                          ),
                        ],
                        const SizedBox(height: 16),
                        TextField(
                          key: ProfileEditSheet.displayNameFieldKey,
                          controller: _displayNameController,
                          focusNode: _nameFocus,
                          textInputAction: TextInputAction.next,
                          maxLength: kProfileDisplayNameMaxLength,
                          enabled: !_saving,
                          decoration: InputDecoration(
                            labelText: l10n.profileDisplayNameLabel,
                          ),
                        ),
                        const SizedBox(height: 12),
                        InputDecorator(
                          decoration: InputDecoration(
                            labelText: l10n.profileCreateUsernameLabel,
                          ),
                          child: SelectableText(widget.profile.handle),
                        ),
                        const SizedBox(height: 12),
                        TextField(
                          key: ProfileEditSheet.bioFieldKey,
                          controller: _bioController,
                          maxLength: 500,
                          maxLines: 4,
                          enabled: !_saving,
                          decoration: InputDecoration(
                            labelText: l10n.profileBioLabel,
                            helperText: l10n.profileBioHelper,
                          ),
                        ),
                        const SizedBox(height: 20),
                        Text(
                          l10n.settingsAccent,
                          style: Theme.of(context).textTheme.titleMedium,
                        ),
                        const SizedBox(height: 8),
                        Wrap(
                          spacing: 8,
                          runSpacing: 8,
                          children: [
                            if (catalog != null)
                              for (
                                var i = 0;
                                i < catalog.profileAccentDefaults.length;
                                i++
                              )
                                Builder(
                                  builder: (context) {
                                    final color =
                                        catalog.profileAccentDefaults[i];
                                    final hex =
                                        '#${color.toARGB32().toRadixString(16).substring(2).toUpperCase()}';
                                    final selected =
                                        _accent?.toUpperCase() == hex ||
                                        (_accent == null && i == 0);
                                    return Semantics(
                                      selected: selected,
                                      label: '${l10n.settingsAccent} ${i + 1}',
                                      child: Tooltip(
                                        message: hex,
                                        child: OutlinedButton(
                                          key: Key('profile_edit_accent_$i'),
                                          style: OutlinedButton.styleFrom(
                                            backgroundColor: color,
                                            foregroundColor: Colors.black,
                                            minimumSize: const Size(44, 44),
                                            padding: EdgeInsets.zero,
                                          ),
                                          onPressed: _saving
                                              ? null
                                              : () => setState(
                                                  () => _accent = hex,
                                                ),
                                          child: selected
                                              ? const Icon(Icons.check)
                                              : const SizedBox(width: 20),
                                        ),
                                      ),
                                    );
                                  },
                                ),
                          ],
                        ),
                        const SizedBox(height: 20),
                        Text(
                          l10n.privacyPresetTitle,
                          style: Theme.of(context).textTheme.titleMedium,
                        ),
                        const SizedBox(height: 8),
                        if (_privacyLoading)
                          const LinearProgressIndicator()
                        else if (_privacyFailed)
                          Row(
                            children: [
                              Expanded(child: Text(l10n.privacyLoadError)),
                              TextButton(
                                onPressed: _saving ? null : _loadPrivacy,
                                child: Text(l10n.commonRetry),
                              ),
                            ],
                          )
                        else
                          Wrap(
                            spacing: 8,
                            runSpacing: 8,
                            children: [
                              for (final preset
                                  in PrivacyPresetDefaults.presets)
                                ChoiceChip(
                                  label: Text(switch (preset) {
                                    'personal' => l10n.privacyPresetPersonal,
                                    'gaming' => l10n.privacyPresetGaming,
                                    _ => l10n.privacyPresetWork,
                                  }),
                                  selected: _preset == preset,
                                  onSelected: _saving
                                      ? null
                                      : (_) => setState(() => _preset = preset),
                                ),
                            ],
                          ),
                        if (_error != null) ...[
                          const SizedBox(height: 12),
                          Semantics(
                            liveRegion: true,
                            child: Text(
                              _error!,
                              style: TextStyle(
                                color: Theme.of(context).colorScheme.error,
                              ),
                            ),
                          ),
                        ],
                        const SizedBox(height: 12),
                        OutlinedButton.icon(
                          onPressed: _saving
                              ? null
                              : () {
                                  showModalBottomSheet<void>(
                                    context: context,
                                    isScrollControlled: true,
                                    builder: (_) => const PlayerProfileSheet(),
                                  );
                                },
                          icon: const Icon(Icons.sports_esports_outlined),
                          label: Text(l10n.playerProfileEntry),
                        ),
                        const SizedBox(height: 20),
                        Row(
                          children: [
                            Expanded(
                              child: OutlinedButton(
                                onPressed: _saving
                                    ? null
                                    : () => Navigator.of(context).pop(),
                                child: Text(l10n.commonCancel),
                              ),
                            ),
                            const SizedBox(width: 12),
                            Expanded(
                              child: FilledButton(
                                key: ProfileEditSheet.saveButtonKey,
                                onPressed: _saving || !_dirty ? null : _save,
                                child: _saving
                                    ? const SizedBox(
                                        height: 18,
                                        width: 18,
                                        child: CircularProgressIndicator(
                                          strokeWidth: 2,
                                        ),
                                      )
                                    : Text(l10n.profileSave),
                              ),
                            ),
                          ],
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }

  String get _avatarFallback {
    final name = widget.profile.displayName;
    if (name.isEmpty) return '?';
    return name[0].toUpperCase();
  }

  Future<void> _pickAvatar() async {
    final l10n = AppLocalizations.of(context)!;
    ProfileAvatarFile? picked;
    try {
      picked = await (widget.avatarPicker ?? defaultPickProfileAvatar).call();
    } catch (_) {
      if (mounted) setState(() => _error = l10n.commonActionFailed);
      return;
    }
    if (!mounted || picked == null) return;
    final contentType = picked.contentType.trim().toLowerCase();
    if (!kProfileAvatarContentTypes.contains(contentType)) {
      setState(() => _error = l10n.profileErrorAvatarType);
      return;
    }
    if (contentType == 'image/gif' && !ref.read(accountIsPremiumProvider)) {
      setState(() => _error = l10n.profileErrorAvatarGifRequiresPremium);
      return;
    }
    if (picked.bytes.isEmpty || picked.bytes.length > kProfileAvatarMaxBytes) {
      setState(() => _error = l10n.profileErrorAvatarTooLarge);
      return;
    }
    setState(() {
      _avatar = picked;
      _uploadedAvatarUrl = null;
      _error = null;
    });
  }

  Future<void> _save() async {
    if (_saving || !_dirty) return;
    final l10n = AppLocalizations.of(context)!;
    final displayName = _displayNameController.text.trim();
    final bio = _bioController.text;
    final validationError = _validate(l10n, displayName, bio);
    if (validationError != null) {
      setState(() => _error = validationError);
      if (displayName.isEmpty ||
          displayName.length > kProfileDisplayNameMaxLength) {
        _nameFocus.requestFocus();
      }
      return;
    }
    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null || !_currentIdentity) {
      setState(
        () => _error = auth == null
            ? l10n.profileEditSaveError('not_authenticated')
            : l10n.commonActionFailed,
      );
      return;
    }

    setState(() {
      _saving = true;
      _error = null;
    });

    try {
      String? avatarUrl = _uploadedAvatarUrl;
      if (_avatar != null && avatarUrl == null) {
        final uploaded = await _uploadAvatar(l10n, _avatar!);
        if (!mounted) return;
        if (uploaded == null) {
          setState(() {
            _saving = false;
            _error ??= l10n.commonActionFailed;
          });
          return;
        }
        avatarUrl = uploaded;
        _uploadedAvatarUrl = uploaded;
      }
      if (!_currentIdentity) {
        _saveFailed(l10n);
        return;
      }
      if (_profileDirty) {
        final result = await ref
            .read(voiceUsersClientProvider)
            .updateProfile(
              authorization: ref.read(authorizationHeaderProvider)!,
              displayName: displayName,
              bio: bio,
              avatarUrl: avatarUrl,
              accentColor: _accent != _savedAccent ? _accent : null,
            );
        if (!mounted) return;
        if (!_currentIdentity) {
          _saveFailed(l10n);
          return;
        }
        if (result is! UsersApiOk<VoiceProfile> ||
            result.data.id != widget.profile.id ||
            result.data.accountId != widget.profile.accountId) {
          _saveFailed(l10n);
          return;
        }
        _savedName = displayName;
        _savedBio = bio;
        _savedAccent = _accent;
        _avatar = null;
        _uploadedAvatarUrl = null;
        ref.invalidate(profileProvider(widget.profile.id));
        ref.invalidate(activeProfileProvider);
        ref.invalidate(myProfilesProvider);
        ref.invalidate(profileAccentColorProvider(widget.profile.id));
      }
      if (_preset != null && _preset != _initialPreset) {
        if (!_currentIdentity) {
          _saveFailed(l10n);
          return;
        }
        final result = await ref
            .read(voiceUserPrivacyClientProvider)
            .updatePrivacy(
              authorization: ref.read(authorizationHeaderProvider)!,
              settings: PrivacyPresetDefaults.forPreset(
                _preset!,
                profileId: widget.profile.id,
              ),
            );
        if (!mounted) return;
        if (!_currentIdentity ||
            result is! UserPrivacyApiOk<VoicePrivacySettings> ||
            result.data.profileId != widget.profile.id) {
          _saveFailed(l10n);
          return;
        }
        _initialPreset = _preset;
      }
      if (!mounted || !_currentIdentity) return;
      Navigator.of(context).pop();
    } catch (_) {
      _saveFailed(l10n);
    }
  }

  void _saveFailed(AppLocalizations l10n) {
    if (mounted) {
      setState(() {
        _saving = false;
        _error = l10n.commonActionFailed;
      });
    }
  }

  String? _validate(AppLocalizations l10n, String displayName, String bio) {
    if (displayName.isEmpty) return l10n.profileErrorDisplayNameRequired;
    if (displayName.length > kProfileDisplayNameMaxLength) {
      return l10n.profileErrorDisplayNameTooLong;
    }
    if (bio.length > 500) return l10n.profileErrorBioTooLong;
    return null;
  }

  Future<String?> _uploadAvatar(
    AppLocalizations l10n,
    ProfileAvatarFile avatar,
  ) async {
    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null) {
      setState(() => _error = l10n.profileEditSaveError('not_authenticated'));
      return null;
    }
    final client = ref.read(voiceUsersClientProvider);
    final presignResult = await client.createAvatarPresignedUpload(
      authorization: auth,
      contentType: avatar.contentType,
      contentLength: avatar.bytes.length,
    );
    if (!mounted || !_currentIdentity) return null;
    switch (presignResult) {
      case UsersApiFailure():
        setState(() => _error = l10n.commonActionFailed);
        return null;
      case UsersApiOk(:final data):
        final uploadResult = await client.uploadAvatarBytes(
          uploadUrl: Uri.parse(data.uploadUrl),
          requiredHeaders: data.requiredHeaders,
          bytes: avatar.bytes,
        );
        if (!mounted || !_currentIdentity) return null;
        switch (uploadResult) {
          case UsersApiFailure():
            setState(() => _error = l10n.commonActionFailed);
            return null;
          case UsersApiOk():
            return data.publicUrl;
        }
    }
  }
}

Future<ProfileAvatarFile?> defaultPickProfileAvatar() async {
  final file = await openFile(
    acceptedTypeGroups: [
      XTypeGroup(
        label: 'Images',
        extensions: const ['jpg', 'jpeg', 'png', 'webp', 'gif'],
        mimeTypes: kProfileAvatarContentTypes.toList(),
      ),
    ],
  );
  if (file == null) return null;
  final bytes = await file.readAsBytes();
  return ProfileAvatarFile(
    bytes: bytes,
    contentType: file.mimeType ?? _contentTypeFromName(file.name),
    name: file.name,
  );
}

String _contentTypeFromName(String name) {
  final lower = name.toLowerCase();
  if (lower.endsWith('.jpg') || lower.endsWith('.jpeg')) return 'image/jpeg';
  if (lower.endsWith('.png')) return 'image/png';
  if (lower.endsWith('.webp')) return 'image/webp';
  if (lower.endsWith('.gif')) return 'image/gif';
  return 'application/octet-stream';
}
