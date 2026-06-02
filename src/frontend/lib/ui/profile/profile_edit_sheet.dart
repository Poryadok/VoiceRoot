import 'dart:typed_data';

import 'package:file_selector/file_selector.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/users_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/social_providers.dart';
import '../../theme/voice_colors.dart';

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
  }

  @override
  void dispose() {
    _displayNameController.dispose();
    _bioController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    return SafeArea(
      child: SingleChildScrollView(
        child: Padding(
          key: ProfileEditSheet.sheetKey,
          padding: const EdgeInsets.fromLTRB(24, 16, 24, 24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                l10n.profileEditTitle,
                style: Theme.of(context).textTheme.titleLarge,
              ),
              const SizedBox(height: 16),
              Row(
                children: [
                  CircleAvatar(
                    radius: 28,
                    backgroundImage: _avatar != null
                        ? MemoryImage(_avatar!.bytes)
                        : (widget.profile.avatarUrl != null
                              ? NetworkImage(widget.profile.avatarUrl!)
                              : null),
                    child: _avatar == null && widget.profile.avatarUrl == null
                        ? Text(_avatarFallback)
                        : null,
                  ),
                  const SizedBox(width: 16),
                  OutlinedButton.icon(
                    key: ProfileEditSheet.avatarButtonKey,
                    onPressed: _saving ? null : _pickAvatar,
                    icon: const Icon(Icons.image_outlined),
                    label: Text(l10n.profileAvatarChange),
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
                maxLength: 64,
                enabled: !_saving,
                decoration: InputDecoration(
                  labelText: l10n.profileDisplayNameLabel,
                  border: const OutlineInputBorder(),
                ),
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
                  border: const OutlineInputBorder(),
                ),
              ),
              if (_error != null) ...[
                const SizedBox(height: 12),
                Text(
                  _error!,
                  style: TextStyle(color: Theme.of(context).colorScheme.error),
                ),
              ],
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
                      onPressed: _saving ? null : _save,
                      child: _saving
                          ? const SizedBox(
                              height: 18,
                              width: 18,
                              child: CircularProgressIndicator(strokeWidth: 2),
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
    );
  }

  String get _avatarFallback {
    final name = widget.profile.displayName;
    if (name.isEmpty) return '?';
    return name[0].toUpperCase();
  }

  Future<void> _pickAvatar() async {
    final l10n = AppLocalizations.of(context)!;
    final picked = await (widget.avatarPicker ?? _defaultPickAvatar).call();
    if (!mounted || picked == null) return;
    final contentType = picked.contentType.trim().toLowerCase();
    if (!kProfileAvatarContentTypes.contains(contentType)) {
      setState(() => _error = l10n.profileErrorAvatarType);
      return;
    }
    if (picked.bytes.isEmpty || picked.bytes.length > kProfileAvatarMaxBytes) {
      setState(() => _error = l10n.profileErrorAvatarTooLarge);
      return;
    }
    setState(() {
      _avatar = picked;
      _error = null;
    });
  }

  Future<void> _save() async {
    final l10n = AppLocalizations.of(context)!;
    final displayName = _displayNameController.text.trim();
    final bio = _bioController.text;
    final validationError = _validate(l10n, displayName, bio);
    if (validationError != null) {
      setState(() => _error = validationError);
      return;
    }

    setState(() {
      _saving = true;
      _error = null;
    });

    String? avatarUrl;
    if (_avatar != null) {
      final uploaded = await _uploadAvatar(l10n, _avatar!);
      if (!mounted) return;
      if (uploaded == null) {
        setState(() => _saving = false);
        return;
      }
      avatarUrl = uploaded;
    }

    final err = await ref
        .read(profileActionsProvider)
        .updateBasicProfile(
          displayName: displayName,
          bio: bio,
          avatarUrl: avatarUrl,
        );
    if (!mounted) return;
    if (err != null) {
      setState(() {
        _saving = false;
        _error = l10n.profileEditSaveError(err);
      });
      return;
    }
    Navigator.of(context).pop();
  }

  String? _validate(AppLocalizations l10n, String displayName, String bio) {
    if (displayName.isEmpty) return l10n.profileErrorDisplayNameRequired;
    if (displayName.length > 64) return l10n.profileErrorDisplayNameTooLong;
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
    switch (presignResult) {
      case UsersApiFailure(:final message):
        setState(() => _error = l10n.profileEditSaveError(message));
        return null;
      case UsersApiOk(:final data):
        final uploadResult = await client.uploadAvatarBytes(
          uploadUrl: Uri.parse(data.uploadUrl),
          requiredHeaders: data.requiredHeaders,
          bytes: avatar.bytes,
        );
        switch (uploadResult) {
          case UsersApiFailure(:final message):
            setState(() => _error = l10n.profileEditSaveError(message));
            return null;
          case UsersApiOk():
            return data.publicUrl;
        }
    }
  }
}

Future<ProfileAvatarFile?> _defaultPickAvatar() async {
  final file = await openFile(
    acceptedTypeGroups: [
      XTypeGroup(
        label: 'Static images',
        extensions: const ['jpg', 'jpeg', 'png', 'webp'],
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
  return 'application/octet-stream';
}
