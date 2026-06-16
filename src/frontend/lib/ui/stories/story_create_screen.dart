import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/files_client.dart';
import '../../backend/stories_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/stories_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_primary_button.dart';

enum StoryCreateType { text, photo, video }

/// Create a text, photo, or video story (Phase 17 basics).
class StoryCreateScreen extends ConsumerStatefulWidget {
  const StoryCreateScreen({super.key});

  static const Key screenKey = Key('story_create_screen');
  static const Key textFieldKey = Key('story_create_text');
  static const Key submitKey = Key('story_create_submit');

  @override
  ConsumerState<StoryCreateScreen> createState() => _StoryCreateScreenState();
}

class _StoryCreateScreenState extends ConsumerState<StoryCreateScreen> {
  final _textController = TextEditingController();
  StoryCreateType _type = StoryCreateType.text;
  var _submitting = false;
  String? _error;
  String? _pickedFileName;
  List<int>? _pickedBytes;
  String? _pickedMime;

  @override
  void dispose() {
    _textController.dispose();
    super.dispose();
  }

  Future<void> _pickMedia({required bool video}) async {
    setState(() {
      _type = video ? StoryCreateType.video : StoryCreateType.photo;
      _error = null;
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

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);

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
