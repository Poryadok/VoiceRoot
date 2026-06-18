import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_picker/image_picker.dart';

import '../../backend/files_client.dart';
import '../../backend/matchmaking_client.dart';
import '../../backend/stories_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/social_providers.dart';
import '../../state/stories_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_primary_button.dart';
import '../matchmaking/game_catalog_screen.dart';
import 'story_audience_picker.dart';

enum StoryCreateType { text, photo, video }

/// Create a text, photo, or video story (Phase 17 basics).
class StoryCreateScreen extends ConsumerStatefulWidget {
  const StoryCreateScreen({super.key});

  static const Key screenKey = Key('story_create_screen');
  static const Key textFieldKey = Key('story_create_text');
  static const Key submitKey = Key('story_create_submit');
  static const Key mentionPickerKey = Key('story_create_mention_picker');

  @override
  ConsumerState<StoryCreateScreen> createState() => _StoryCreateScreenState();
}

class _StoryCreateScreenState extends ConsumerState<StoryCreateScreen> {
  final _textController = TextEditingController();
  final _mentionQueryController = TextEditingController();
  final _imagePicker = ImagePicker();
  StoryCreateType _type = StoryCreateType.text;
  var _submitting = false;
  String? _error;
  String? _pickedFileName;
  List<int>? _pickedBytes;
  String? _pickedMime;
  String _visibility = 'friends';
  final _mentionedProfileIds = <String>{};
  String? _gameTag;
  String? _gameTagLabel;
  String _textBackground = 'accent';

  static const _backgroundOptions = ['accent', 'elevated', 'muted'];

  @override
  void dispose() {
    _textController.dispose();
    _mentionQueryController.dispose();
    super.dispose();
  }

  String? _textStyleJson() {
    if (_type != StoryCreateType.text) return null;
    return jsonEncode({'background': _textBackground});
  }

  Future<void> _pickMedia({required bool video}) async {
    setState(() {
      _type = video ? StoryCreateType.video : StoryCreateType.photo;
      _error = null;
    });
    final file = video
        ? await _imagePicker.pickVideo(source: ImageSource.gallery)
        : await _imagePicker.pickImage(source: ImageSource.gallery);
    if (file == null) return;
    final bytes = await file.readAsBytes();
    if (!mounted) return;
    setState(() {
      _pickedFileName = file.name;
      _pickedBytes = bytes;
      _pickedMime = video ? 'video/mp4' : 'image/jpeg';
    });
  }

  Future<void> _pickGameTag() async {
    final game = await Navigator.of(context).push<CatalogGame>(
      MaterialPageRoute(
        builder: (_) => const GameCatalogScreen(selectMode: true),
      ),
    );
    if (game == null || !mounted) return;
    setState(() {
      _gameTag = game.id;
      _gameTagLabel = game.name;
    });
  }

  Future<String?> _uploadMedia() async {
    final auth = ref.read(authorizationHeaderProvider);
    final bytes = _pickedBytes;
    final mime = _pickedMime;
    final name = _pickedFileName;
    if (auth == null || bytes == null || mime == null || name == null) {
      return null;
    }
    final files = ref.read(voiceFilesClientProvider);
    final ticketResult = await files.requestUpload(
      authorization: auth,
      originalName: name,
      mimeType: mime,
      sizeBytes: bytes.length,
    );
    if (ticketResult is! FilesApiOk<FileUploadTicket>) return null;
    final ticket = ticketResult.data;
    final put = await files.putBytes(
      uploadUrl: ticket.presignedPutUrl,
      bytes: Uint8List.fromList(bytes),
      mimeType: mime,
    );
    if (put is FilesApiFailure) return null;
    final confirm = await files.confirmUpload(
      authorization: auth,
      fileId: ticket.fileId,
      bytes: Uint8List.fromList(bytes),
    );
    return switch (confirm) {
      FilesApiOk() => ticket.fileId,
      FilesApiFailure() => null,
    };
  }

  Future<void> _submit() async {
    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null) return;

    setState(() {
      _submitting = true;
      _error = null;
    });

    final client = ref.read(voiceStoriesClientProvider);
    final mentionIds = _mentionedProfileIds.toList(growable: false);
    final gameTag = _gameTag;
    final textStyleJson = _textStyleJson();
    StoriesApiResult<StoryData> result;

    switch (_type) {
      case StoryCreateType.text:
        final text = _textController.text.trim();
        if (text.isEmpty) {
          setState(() {
            _submitting = false;
            _error = AppLocalizations.of(context)!.storyCreateTextRequired;
          });
          return;
        }
        result = await client.createTextStory(
          authorization: auth,
          text: text,
          textStyleJson: textStyleJson,
          gameTag: gameTag,
          mentionProfileIds: mentionIds,
          visibility: _visibility,
        );
      case StoryCreateType.photo:
      case StoryCreateType.video:
        final fileId = await _uploadMedia();
        if (fileId == null) {
          if (!mounted) return;
          setState(() {
            _submitting = false;
            _error = AppLocalizations.of(context)!.storyCreateMediaRequired;
          });
          return;
        }
        result = await client.createStory(
          authorization: auth,
          type: _type == StoryCreateType.photo ? 'photo' : 'video',
          mediaFileId: fileId,
          textContent: _textController.text.trim().isEmpty
              ? null
              : _textController.text.trim(),
          gameTag: gameTag,
          mentionProfileIds: mentionIds,
          visibility: _visibility,
        );
    }

    if (!mounted) return;
    switch (result) {
      case StoriesApiOk():
        ref.invalidate(storyFeedProvider);
        Navigator.of(context).pop(true);
      case StoriesApiFailure(:final message):
        setState(() {
          _submitting = false;
          _error = message;
        });
    }
  }

  Color _backgroundSwatch(VoiceColors voice, String key) {
    return switch (key) {
      'elevated' => voice.elevated,
      'muted' => voice.muted,
      _ => voice.profileAccent,
    };
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final friendsAsync = ref.watch(friendsListProvider);
    final friendIds = friendsAsync.valueOrNull?.friends ?? const [];

    return Scaffold(
      key: StoryCreateScreen.screenKey,
      appBar: AppBar(title: Text(l10n.storyCreateTitle)),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          SegmentedButton<StoryCreateType>(
            segments: [
              ButtonSegment(
                value: StoryCreateType.text,
                label: Text(l10n.storyCreateTypeText),
                icon: const Icon(Icons.text_fields),
              ),
              ButtonSegment(
                value: StoryCreateType.photo,
                label: Text(l10n.storyCreateTypePhoto),
                icon: const Icon(Icons.photo_outlined),
              ),
              ButtonSegment(
                value: StoryCreateType.video,
                label: Text(l10n.storyCreateTypeVideo),
                icon: const Icon(Icons.videocam_outlined),
              ),
            ],
            selected: {_type},
            onSelectionChanged: (value) {
              setState(() => _type = value.first);
            },
          ),
          const SizedBox(height: 16),
          StoryAudiencePicker(
            value: _visibility,
            onChanged: (value) => setState(() => _visibility = value),
          ),
          const SizedBox(height: 16),
          _MentionPicker(
            key: StoryCreateScreen.mentionPickerKey,
            queryController: _mentionQueryController,
            friendIds: friendIds,
            selectedIds: _mentionedProfileIds,
            onQueryChanged: () => setState(() {}),
            onToggle: (profileId, selected) {
              setState(() {
                if (selected) {
                  _mentionedProfileIds.add(profileId);
                } else {
                  _mentionedProfileIds.remove(profileId);
                }
              });
            },
          ),
          const SizedBox(height: 16),
          Row(
            children: [
              Expanded(
                child: Text(
                  l10n.storyCreateGameTagLabel,
                  style: TextStyle(color: voice.textSecondary),
                ),
              ),
              TextButton(
                onPressed: _submitting ? null : _pickGameTag,
                child: Text(
                  _gameTagLabel ?? l10n.storyCreateGameTagPick,
                ),
              ),
              if (_gameTag != null)
                TextButton(
                  onPressed: _submitting
                      ? null
                      : () => setState(() {
                            _gameTag = null;
                            _gameTagLabel = null;
                          }),
                  child: Text(l10n.storyCreateGameTagClear),
                ),
            ],
          ),
          if (_type == StoryCreateType.text) ...[
            const SizedBox(height: 8),
            Text(
              l10n.storyCreateTextStyleLabel,
              style: TextStyle(color: voice.textSecondary),
            ),
            const SizedBox(height: 8),
            Wrap(
              spacing: 8,
              children: [
                for (final option in _backgroundOptions)
                  ChoiceChip(
                    label: Text(option),
                    selected: _textBackground == option,
                    selectedColor: _backgroundSwatch(voice, option),
                    onSelected: _submitting
                        ? null
                        : (selected) {
                            if (selected) {
                              setState(() => _textBackground = option);
                            }
                          },
                  ),
              ],
            ),
            const SizedBox(height: 12),
          ],
          TextField(
            key: StoryCreateScreen.textFieldKey,
            controller: _textController,
            maxLines: _type == StoryCreateType.text ? 6 : 2,
            decoration: InputDecoration(
              labelText: _type == StoryCreateType.text
                  ? l10n.storyCreateTextLabel
                  : l10n.storyCreateCaptionLabel,
              border: const OutlineInputBorder(),
            ),
          ),
          if (_type != StoryCreateType.text) ...[
            const SizedBox(height: 12),
            OutlinedButton.icon(
              onPressed: _submitting
                  ? null
                  : () => _pickMedia(video: _type == StoryCreateType.video),
              icon: const Icon(Icons.attach_file),
              label: Text(_pickedFileName ?? l10n.storyCreatePickMedia),
            ),
          ],
          if (_error != null) ...[
            const SizedBox(height: 12),
            Text(_error!, style: TextStyle(color: voice.error)),
          ],
          const SizedBox(height: 24),
          VoicePrimaryButton(
            key: StoryCreateScreen.submitKey,
            isLoading: _submitting,
            onPressed: _submitting ? null : _submit,
            child: Text(l10n.storyCreateSubmit),
          ),
        ],
      ),
    );
  }
}

class _MentionPicker extends ConsumerWidget {
  const _MentionPicker({
    super.key,
    required this.queryController,
    required this.friendIds,
    required this.selectedIds,
    required this.onQueryChanged,
    required this.onToggle,
  });

  final TextEditingController queryController;
  final List<String> friendIds;
  final Set<String> selectedIds;
  final VoidCallback onQueryChanged;
  final void Function(String profileId, bool selected) onToggle;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final query = queryController.text.trim().toLowerCase();

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          l10n.storyCreateMentionLabel,
          style: TextStyle(color: voice.textSecondary),
        ),
        const SizedBox(height: 8),
        TextField(
          controller: queryController,
          decoration: InputDecoration(
            hintText: l10n.storyCreateMentionHint,
            prefixIcon: const Icon(Icons.alternate_email),
            border: const OutlineInputBorder(),
            isDense: true,
          ),
          onChanged: (_) => onQueryChanged(),
        ),
        if (friendIds.isNotEmpty) ...[
          const SizedBox(height: 8),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              for (final profileId in friendIds)
                _MentionChip(
                  profileId: profileId,
                  query: query,
                  selected: selectedIds.contains(profileId),
                  onToggle: onToggle,
                ),
            ],
          ),
        ],
      ],
    );
  }
}

class _MentionChip extends ConsumerWidget {
  const _MentionChip({
    required this.profileId,
    required this.query,
    required this.selected,
    required this.onToggle,
  });

  final String profileId;
  final String query;
  final bool selected;
  final void Function(String profileId, bool selected) onToggle;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final profileAsync = ref.watch(profileProvider(profileId));
    return profileAsync.when(
      loading: () => const SizedBox.shrink(),
      error: (_, _) => const SizedBox.shrink(),
      data: (profile) {
        if (profile == null) return const SizedBox.shrink();
        final handle = profile.handle.toLowerCase();
        final name = profile.displayName.toLowerCase();
        if (query.isNotEmpty &&
            !handle.contains(query) &&
            !name.contains(query)) {
          return const SizedBox.shrink();
        }
        return FilterChip(
          label: Text(profile.handle),
          selected: selected,
          onSelected: (value) => onToggle(profileId, value),
        );
      },
    );
  }
}
