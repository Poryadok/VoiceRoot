import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/stories_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/stories_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_bottom_sheet.dart';
import '../core/voice_primary_button.dart';
import 'story_audience_picker.dart';

/// Create or edit a highlight (name, visibility, story membership).
class HighlightEditSheet extends ConsumerStatefulWidget {
  const HighlightEditSheet._({this.highlight});

  static const Key sheetKey = Key('highlight_edit_sheet');
  static const Key nameFieldKey = Key('highlight_edit_name');
  static const Key saveButtonKey = Key('highlight_edit_save');

  final HighlightData? highlight;

  static Future<void> showCreate(BuildContext context) {
    return showVoiceBottomSheet<void>(
      context: context,
      scrollable: false,
      child: const HighlightEditSheet._(),
    );
  }

  static Future<void> showEdit(
    BuildContext context, {
    required HighlightData highlight,
  }) {
    return showVoiceBottomSheet<void>(
      context: context,
      scrollable: false,
      child: HighlightEditSheet._(highlight: highlight),
    );
  }

  @override
  ConsumerState<HighlightEditSheet> createState() => _HighlightEditSheetState();
}

class _HighlightEditSheetState extends ConsumerState<HighlightEditSheet> {
  late final TextEditingController _nameController;
  late String _visibility;
  var _saving = false;
  String? _error;
  late List<String> _storyIds;

  bool get _isEdit => widget.highlight != null;

  @override
  void initState() {
    super.initState();
    final highlight = widget.highlight;
    _nameController = TextEditingController(text: highlight?.name ?? '');
    _visibility = highlight?.visibility ?? 'everyone';
    _storyIds = List.of(highlight?.storyIds ?? const []);
  }

  @override
  void dispose() {
    _nameController.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    final auth = ref.read(authorizationHeaderProvider);
    final profileId = ref.read(authControllerProvider).activeProfileId;
    if (auth == null || profileId == null) return;

    final name = _nameController.text.trim();
    if (name.isEmpty) return;

    setState(() {
      _saving = true;
      _error = null;
    });

    final client = ref.read(voiceStoriesClientProvider);
    final StoriesApiResult<HighlightData> result;

    if (_isEdit) {
      result = await client.updateHighlight(
        authorization: auth,
        highlightId: widget.highlight!.id,
        name: name,
        visibility: _visibility,
      );
    } else {
      result = await client.createHighlight(
        authorization: auth,
        name: name,
        visibility: _visibility,
      );
    }

    if (!mounted) return;
    switch (result) {
      case StoriesApiOk(:final data):
        if (!_isEdit && _storyIds.isNotEmpty) {
          for (final storyId in _storyIds) {
            await client.addToHighlight(
              authorization: auth,
              highlightId: data.id,
              storyId: storyId,
            );
          }
        }
        ref.invalidate(profileHighlightsProvider(profileId));
        if (mounted) Navigator.of(context).pop();
      case StoriesApiFailure(:final message):
        setState(() {
          _saving = false;
          _error = message;
        });
    }
  }

  Future<void> _removeStory(String storyId) async {
    final highlight = widget.highlight;
    final auth = ref.read(authorizationHeaderProvider);
    final profileId = ref.read(authControllerProvider).activeProfileId;
    if (highlight == null || auth == null || profileId == null) return;

    final result = await ref.read(voiceStoriesClientProvider).removeFromHighlight(
          authorization: auth,
          highlightId: highlight.id,
          storyId: storyId,
        );

    if (!mounted) return;
    switch (result) {
      case StoriesApiOk():
        setState(() => _storyIds.remove(storyId));
        ref.invalidate(profileHighlightsProvider(profileId));
      case StoriesApiFailure(:final message):
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(message)),
        );
    }
  }

  Future<void> _addFromArchive() async {
    final archive = await ref.read(storyArchiveProvider.future);
    if (!mounted || archive.isEmpty) return;

    final storyId = await showVoiceBottomSheet<String>(
      context: context,
      initialSize: 0.5,
      child: SafeArea(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Padding(
              padding: const EdgeInsets.all(16),
              child: Text(
                AppLocalizations.of(context)!.storyHighlightAddStories,
                style: Theme.of(context).textTheme.titleMedium,
              ),
            ),
            Expanded(
              child: ListView.builder(
                itemCount: archive.length,
                itemBuilder: (ctx, index) {
                  final story = archive[index];
                  final label = story.textContent?.trim();
                  return ListTile(
                    title: Text(
                      label != null && label.isNotEmpty
                          ? label
                          : story.type,
                    ),
                    onTap: () => Navigator.of(ctx).pop(story.id),
                  );
                },
              ),
            ),
          ],
        ),
      ),
    );

    if (storyId == null || !mounted) return;

    final highlight = widget.highlight;
    final auth = ref.read(authorizationHeaderProvider);
    final profileId = ref.read(authControllerProvider).activeProfileId;
    if (auth == null || profileId == null) return;

    if (highlight != null) {
      final result =
          await ref.read(voiceStoriesClientProvider).addToHighlight(
                authorization: auth,
                highlightId: highlight.id,
                storyId: storyId,
              );
      if (!mounted) return;
      switch (result) {
        case StoriesApiOk():
          setState(() => _storyIds.add(storyId));
          ref.invalidate(profileHighlightsProvider(profileId));
        case StoriesApiFailure(:final message):
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text(message)),
          );
      }
    } else {
      setState(() {
        if (!_storyIds.contains(storyId)) _storyIds.add(storyId);
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);

    return SafeArea(
      child: Padding(
        key: HighlightEditSheet.sheetKey,
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              _isEdit ? l10n.storyHighlightEditTitle : l10n.storyHighlightCreate,
              style: Theme.of(context).textTheme.titleMedium,
            ),
            const SizedBox(height: 16),
            TextField(
              key: HighlightEditSheet.nameFieldKey,
              controller: _nameController,
              decoration: InputDecoration(
                labelText: l10n.storyHighlightNameHint,
                border: const OutlineInputBorder(),
              ),
            ),
            const SizedBox(height: 16),
            StoryAudiencePicker(
              value: _visibility,
              onChanged: (value) => setState(() => _visibility = value),
            ),
            if (_isEdit || _storyIds.isNotEmpty) ...[
              const SizedBox(height: 16),
              Row(
                children: [
                  Expanded(
                    child: Text(
                      l10n.storyHighlightStoriesSection,
                      style: TextStyle(color: voice.textSecondary),
                    ),
                  ),
                  TextButton(
                    key: const Key('highlight_edit_add_stories'),
                    onPressed: _saving ? null : _addFromArchive,
                    child: Text(l10n.storyHighlightAddStories),
                  ),
                ],
              ),
              ..._storyIds.map(
                (storyId) => ListTile(
                  key: Key('highlight_story_$storyId'),
                  dense: true,
                  title: Text(storyId, style: TextStyle(color: voice.textPrimary)),
                  trailing: _isEdit
                      ? IconButton(
                          icon: Icon(Icons.remove_circle_outline,
                              color: voice.error),
                          onPressed: _saving
                              ? null
                              : () => _removeStory(storyId),
                        )
                      : null,
                ),
              ),
            ],
            if (_error != null) ...[
              const SizedBox(height: 8),
              Text(_error!, style: TextStyle(color: voice.error)),
            ],
            const Spacer(),
            VoicePrimaryButton(
              key: HighlightEditSheet.saveButtonKey,
              isLoading: _saving,
              onPressed: _saving ? null : _save,
              child: Text(l10n.storyHighlightSave),
            ),
          ],
        ),
      ),
    );
  }
}
