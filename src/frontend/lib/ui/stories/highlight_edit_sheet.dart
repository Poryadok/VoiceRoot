import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/stories_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/stories_providers.dart';
import '../../theme/voice_colors.dart';
import '../api_error_messages.dart';
import '../core/voice_bottom_sheet.dart';
import '../core/voice_primary_button.dart';
import '../core/voice_skeleton.dart';
import 'story_audience_picker.dart';
import 'story_author_session.dart';

/// Panel / Stories / HighlightEdit v3 destination (see docs/design/screen-
/// controls.md §46 and docs/design/screens.md): one sheet editing name, cover
/// preview, selected-stories checklist over the archive, audience, save with
/// validation, and delete confirm.
///
/// States: loading archive (checklist skeleton while storyArchiveProvider is
/// loading), empty archive (checklist only shows committed membership),
/// archive error + retry (localized message, retry invalidates the provider),
/// disabled save (kept enabled but validation-blocked with a localized hint
/// when the name or the selection is missing), permission (stale author
/// session replaces the sheet with StorySessionExpiredPanel), validation
/// (blank name never writes), busy spinner while saving, retry after a
/// failed save (PATCH + only the not-yet-committed membership writes),
/// cancel (Escape pops the sheet via showVoiceBottomSheet), and delete
/// confirm (AlertDialog cancel/confirm; cancel never writes).
///
/// The highlight cover is display-only: story.proto has no cover field and
/// UpdateHighlight accepts only name and visibility, so the picked cover
/// persists just for this sheet session. Recorded in docs/todo/backend.md.
class HighlightEditSheet extends ConsumerStatefulWidget {
  const HighlightEditSheet._({
    this.highlight,
    this.initialStoryIds = const [],
    this.authorSession,
  });

  static const Key sheetKey = Key('highlight_edit_sheet');
  static const Key nameFieldKey = Key('highlight_edit_name');
  static const Key saveButtonKey = Key('highlight_edit_save');
  static const Key coverPreviewKey = Key('highlight_edit_cover_preview');
  static const Key coverButtonKey = Key('highlight_edit_cover_button');
  static const Key audiencePickerKey = Key('highlight_edit_audience');
  static const Key deleteButtonKey = Key('highlight_edit_delete');
  static const Key archiveRetryKey = Key('highlight_edit_archive_retry');
  static const String storyRowKeyPrefix = 'highlight_story_';
  static const String storyToggleKeySuffix = '_toggle';

  final HighlightData? highlight;
  final List<String> initialStoryIds;
  final StoryAuthorSession? authorSession;

  static Future<void> showCreate(
    BuildContext context, {
    List<String> initialStoryIds = const [],
    StoryAuthorSession? authorSession,
  }) {
    return showVoiceBottomSheet<void>(
      context: context,
      scrollable: false,
      child: HighlightEditSheet._(
        initialStoryIds: initialStoryIds,
        authorSession: authorSession,
      ),
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
  var _deleting = false;
  String? _error;
  late List<String> _storyIds;
  late List<String> _selection;
  final Set<String> _savedStoryIds = {};
  final Set<String> _removedStoryIds = {};
  String? _createdHighlightId;
  String? _coverStoryId;
  late final StoryAuthorSession _authorSession;
  bool _stale = false;

  bool _isCurrent() {
    if (!mounted || _stale) return false;
    if (!_authorSession.isCurrent(ref)) {
      _stale = true;
      return false;
    }
    return true;
  }

  bool get _isEdit => widget.highlight != null;

  @override
  void initState() {
    super.initState();
    _authorSession = widget.authorSession ?? StoryAuthorSession.capture(ref);
    final highlight = widget.highlight;
    _nameController = TextEditingController(text: highlight?.name ?? '');
    _visibility = highlight?.visibility ?? 'everyone';
    // Create mode: initialStoryIds seeds the selection only, nothing is
    // committed yet, so Save still adds those stories to the new archive.
    // Edit mode: committed membership comes from the loaded highlight.
    _storyIds = List.of(highlight?.storyIds ?? const <String>[]);
    _selection = List.of(highlight?.storyIds ?? widget.initialStoryIds);
    if (_selection.isNotEmpty) _coverStoryId = _selection.first;
  }

  @override
  void dispose() {
    _nameController.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    if (_saving || !_isCurrent()) return;
    final auth = ref.read(authorizationHeaderProvider);
    final profileId = _authorSession.profileId;
    if (auth == null || profileId == null) return;

    final name = _nameController.text.trim();
    if (name.isEmpty) return;
    if (_selection.isEmpty) {
      setState(() {
        _error = AppLocalizations.of(context)!.storyHighlightNeedsStories;
      });
      return;
    }

    setState(() {
      _saving = true;
      _error = null;
    });

    final client = ref.read(voiceStoriesClientProvider);
    final StoriesApiResult<HighlightData> result;

    if (_createdHighlightId != null) {
      result = await client.updateHighlight(
        authorization: auth,
        highlightId: _createdHighlightId!,
        name: name,
        visibility: _visibility,
      );
    } else if (_isEdit) {
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

    if (!mounted || !_isCurrent()) return;
    switch (result) {
      case StoriesApiOk(:final data):
        if (!_isEdit) _createdHighlightId ??= data.id;
        final highlightId = _createdHighlightId ?? widget.highlight!.id;
        if (!await _commitStories(client, auth, highlightId)) return;
        setState(() => _storyIds = List.of(_selection));
        ref.invalidate(profileHighlightsProvider(profileId));
        if (mounted && _isCurrent()) Navigator.of(context).pop();
      case StoriesApiFailure(:final statusCode):
        setState(() {
          _saving = false;
          _error = commonActionErrorMessage(
            AppLocalizations.of(context)!,
            statusCode: statusCode,
          );
        });
    }
  }

  Future<bool> _commitStories(
    VoiceStoriesClient client,
    String auth,
    String highlightId,
  ) async {
    for (final storyId in [..._selection]) {
      if (_storyIds.contains(storyId) || _savedStoryIds.contains(storyId)) {
        continue;
      }
      if (!mounted || !_isCurrent()) return false;
      final added = await client.addToHighlight(
        authorization: auth,
        highlightId: highlightId,
        storyId: storyId,
      );
      if (!mounted || !_isCurrent()) return false;
      switch (added) {
        case StoriesApiOk():
          _savedStoryIds.add(storyId);
        case StoriesApiFailure(:final statusCode):
          setState(() {
            _saving = false;
            _error = commonActionErrorMessage(
              AppLocalizations.of(context)!,
              statusCode: statusCode,
            );
          });
          return false;
      }
    }
    for (final storyId in [..._storyIds]) {
      if (_selection.contains(storyId) || _removedStoryIds.contains(storyId)) {
        continue;
      }
      if (!mounted || !_isCurrent()) return false;
      final removed = await client.removeFromHighlight(
        authorization: auth,
        highlightId: highlightId,
        storyId: storyId,
      );
      if (!mounted || !_isCurrent()) return false;
      switch (removed) {
        case StoriesApiOk():
          _removedStoryIds.add(storyId);
        case StoriesApiFailure(:final statusCode):
          setState(() {
            _saving = false;
            _error = commonActionErrorMessage(
              AppLocalizations.of(context)!,
              statusCode: statusCode,
            );
          });
          return false;
      }
    }
    return mounted && _isCurrent();
  }

  bool _toggleStory(String storyId) {
    if (_saving || _deleting || !_isCurrent()) return false;
    setState(() {
      if (_selection.contains(storyId)) {
        _selection.remove(storyId);
        if (_coverStoryId == storyId) {
          _coverStoryId = _selection.isEmpty ? null : _selection.first;
        }
      } else {
        _selection.add(storyId);
        _coverStoryId ??= storyId;
      }
    });
    return true;
  }

  bool _cycleCover() {
    if (_saving || _deleting || _selection.isEmpty || !_isCurrent()) {
      return false;
    }
    setState(() {
      final index = _selection.indexOf(_coverStoryId ?? '');
      _coverStoryId = _selection[(index + 1) % _selection.length];
    });
    return true;
  }

  bool _canToggleStory(String storyId) =>
      !_saving && !_deleting && _isCurrent();

  Future<void> _deleteHighlight() async {
    if (_deleting || !_isEdit || !_isCurrent()) return;
    final highlight = widget.highlight!;
    setState(() => _deleting = true);

    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(AppLocalizations.of(ctx)!.storyHighlightDeleteConfirm),
        // Name the highlight being deleted; the action word lives on the
        // confirm button only, since the sheet hides its own label meanwhile.
        content: Text(highlight.name),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(AppLocalizations.of(ctx)!.commonCancel),
          ),
          FilledButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: Text(AppLocalizations.of(ctx)!.storyHighlightDelete),
          ),
        ],
      ),
    );

    if (!mounted) return;
    setState(() => _deleting = false);
    if (confirmed != true || !_isCurrent()) return;

    final auth = ref.read(authorizationHeaderProvider);
    final profileId = _authorSession.profileId;
    if (auth == null || profileId == null) return;

    final result = await ref
        .read(voiceStoriesClientProvider)
        .deleteHighlight(authorization: auth, highlightId: highlight.id);

    if (!mounted || !_isCurrent()) return;
    switch (result) {
      case StoriesApiOk():
        ref.invalidate(profileHighlightsProvider(profileId));
        Navigator.of(context).pop();
      case StoriesApiFailure(:final statusCode):
        setState(() {
          _deleting = false;
          _error = commonActionErrorMessage(
            AppLocalizations.of(context)!,
            statusCode: statusCode,
          );
        });
    }
  }

  @override
  Widget build(BuildContext context) {
    ref.watch(authControllerProvider);
    if (!_isCurrent()) return const StorySessionExpiredPanel();
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final archiveAsync = ref.watch(storyArchiveProvider);

    return SafeArea(
      child: Padding(
        key: HighlightEditSheet.sheetKey,
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 16),
        child: LayoutBuilder(
          builder: (context, size) {
            // The v3 mockup cover tile is 110x150; short sheet windows shrink
            // it so the checklist rows stay laid out and hittable.
            final fullCover = size.maxHeight >= 560;
            return Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text(
                  _isEdit
                      ? l10n.storyHighlightEditTitle
                      : l10n.storyHighlightCreate,
                  style: Theme.of(context).textTheme.titleMedium,
                ),
                const SizedBox(height: 8),
                TextField(
                  key: HighlightEditSheet.nameFieldKey,
                  controller: _nameController,
                  decoration: InputDecoration(
                    labelText: l10n.storyHighlightNameHint,
                    border: const OutlineInputBorder(),
                  ),
                ),
                const SizedBox(height: 8),
                if (_selection.isNotEmpty) ...[
                  const SizedBox(height: 16),
                  Text(
                    l10n.storyHighlightCover,
                    style: TextStyle(color: voice.textSecondary),
                  ),
                  const SizedBox(height: 8),
                  Row(
                    children: [
                      // v3 mockup (fragment s16) draws the cover as a portrait
                      // 110x150 story tile next to the change-cover button.
                      Container(
                        key: HighlightEditSheet.coverPreviewKey,
                        width: fullCover ? 110 : 60,
                        height: fullCover ? 150 : 72,
                        decoration: BoxDecoration(
                          gradient: LinearGradient(
                            begin: Alignment.topLeft,
                            end: Alignment.bottomRight,
                            colors: [voice.profileAccent, voice.elevated],
                          ),
                          borderRadius: BorderRadius.circular(12),
                        ),
                        child: Padding(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 12,
                            vertical: 6,
                          ),
                          child: Text(
                            _archiveLabel(
                              archiveAsync.valueOrNull,
                              _coverStoryId ?? _selection.first,
                            ),
                            style: TextStyle(color: voice.textPrimary),
                          ),
                        ),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: TextButton(
                          key: HighlightEditSheet.coverButtonKey,
                          onPressed: _saving || _deleting || !_isCurrent()
                              ? null
                              : _cycleCover,
                          child: Text(l10n.storyHighlightCoverChange),
                        ),
                      ),
                    ],
                  ),
                ],
                const SizedBox(height: 8),
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        l10n.storyHighlightStoriesSection,
                        style: TextStyle(color: voice.textSecondary),
                      ),
                    ),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        l10n.storyHighlightStoryCount(_selection.length),
                        style: TextStyle(color: voice.textSecondary),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 8),
                Expanded(
                  child: archiveAsync.when(
                    loading: () => const VoiceListSkeleton(),
                    error: (error, _) => ListView(
                      children: [
                        Text(
                          storyArchiveErrorMessage(l10n, error),
                          style: TextStyle(color: voice.error),
                        ),
                        const SizedBox(height: 8),
                        Align(
                          alignment: Alignment.centerRight,
                          child: TextButton(
                            key: HighlightEditSheet.archiveRetryKey,
                            onPressed: () =>
                                ref.invalidate(storyArchiveProvider),
                            child: Text(l10n.commonRetry),
                          ),
                        ),
                      ],
                    ),
                    data: (archive) => ListView(
                      children: [
                        for (final story in archive)
                          ListTile(
                            key: Key(
                              '${HighlightEditSheet.storyRowKeyPrefix}${story.id}',
                            ),
                            dense: true,
                            title: Text(
                              _storyLabel(story),
                              style: TextStyle(color: voice.textPrimary),
                            ),
                            leading: _selection.contains(story.id)
                                ? Icon(Icons.check, color: voice.success)
                                : null,
                            trailing: TextButton(
                              key: Key(
                                '${HighlightEditSheet.storyRowKeyPrefix}'
                                '${story.id}'
                                '${HighlightEditSheet.storyToggleKeySuffix}',
                              ),
                              onPressed: _canToggleStory(story.id)
                                  ? () => _toggleStory(story.id)
                                  : null,
                              child: Text(
                                _selection.contains(story.id)
                                    ? l10n.storyHighlightUnselectStory
                                    : l10n.storyHighlightSelectStory,
                              ),
                            ),
                          ),
                        for (final storyId in _storyIds)
                          if (!archive.any((story) => story.id == storyId))
                            ListTile(
                              key: Key(
                                '${HighlightEditSheet.storyRowKeyPrefix}$storyId',
                              ),
                              dense: true,
                              title: Text(
                                storyId,
                                style: TextStyle(color: voice.textPrimary),
                              ),
                              leading: Icon(Icons.check, color: voice.success),
                              trailing: TextButton(
                                key: Key(
                                  '${HighlightEditSheet.storyRowKeyPrefix}'
                                  '$storyId'
                                  '${HighlightEditSheet.storyToggleKeySuffix}',
                                ),
                                onPressed: _canToggleStory(storyId)
                                    ? () => _toggleStory(storyId)
                                    : null,
                                child: Text(
                                  _selection.contains(storyId)
                                      ? l10n.storyHighlightUnselectStory
                                      : l10n.storyHighlightSelectStory,
                                ),
                              ),
                            ),
                      ],
                    ),
                  ),
                ),
                if (_error != null) ...[
                  const SizedBox(height: 8),
                  Semantics(
                    liveRegion: true,
                    child: Text(_error!, style: TextStyle(color: voice.error)),
                  ),
                ],
                const SizedBox(height: 8),
                StoryAudiencePicker(
                  key: HighlightEditSheet.audiencePickerKey,
                  value: _visibility,
                  onChanged: (value) => setState(() => _visibility = value),
                ),
                const SizedBox(height: 8),
                Row(
                  children: [
                    if (_isEdit && !_deleting)
                      // While the confirm dialog is open the sheet hides its
                      // own delete label so the dialog action stays unambiguous.
                      Expanded(
                        child: TextButton(
                          key: HighlightEditSheet.deleteButtonKey,
                          onPressed: _saving || _deleting || !_isCurrent()
                              ? null
                              : _deleteHighlight,
                          child: Text(l10n.storyHighlightDelete),
                        ),
                      ),
                    VoicePrimaryButton(
                      key: HighlightEditSheet.saveButtonKey,
                      isLoading: _saving,
                      onPressed: _saving || _deleting ? null : _save,
                      child: Text(l10n.storyHighlightSave),
                    ),
                  ],
                ),
              ],
            );
          },
        ),
      ),
    );
  }

  String _storyLabel(StoryData story) {
    final label = story.textContent?.trim();
    return label != null && label.isNotEmpty ? label : story.type;
  }

  String _archiveLabel(List<StoryData>? archive, String storyId) {
    if (archive == null) return storyId;
    for (final story in archive) {
      if (story.id == storyId) return _storyLabel(story);
    }
    return storyId;
  }
}
