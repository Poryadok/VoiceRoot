import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../l10n/app_localizations.dart';
import '../../state/space_providers.dart';
import '../../theme/voice_colors.dart';
import '../a11y/focus_trap.dart';
import '../a11y/voice_focus_return.dart';
import '../api_error_messages.dart';
import '../core/voice_avatar.dart';

/// API-backed creation panel. Templates/upload await their own contracts.
class CreateSpaceSheet extends ConsumerStatefulWidget {
  const CreateSpaceSheet({super.key});
  static const Key sheetKey = Key('create_space_sheet');
  static const Key nameFieldKey = Key('create_space_name');
  static const Key descriptionFieldKey = Key('create_space_description');
  static const Key iconFieldKey = Key('create_space_icon');
  static const Key submitKey = Key('create_space_submit');
  static const Key closeKey = Key('create_space_close');

  static Future<void> show(BuildContext context) async {
    final container = ProviderScope.containerOf(context);
    final focusReturn = VoiceFocusReturn.capture();
    await showGeneralDialog<void>(
      context: context,
      barrierDismissible: false,
      barrierColor: VoiceColors.of(context).overlay,
      pageBuilder: (context, _, _) => UncontrolledProviderScope(
        container: container,
        child: Align(
          alignment: Alignment.centerRight,
          child: SizedBox(
            width: MediaQuery.sizeOf(context).width >= 840
                ? 320
                : double.infinity,
            height: double.infinity,
            child: const CreateSpaceSheet(),
          ),
        ),
      ),
    );
    focusReturn.restore();
  }

  @override
  ConsumerState<CreateSpaceSheet> createState() => _CreateSpaceSheetState();
}

class _CreateSpaceSheetState extends ConsumerState<CreateSpaceSheet> {
  final _nameController = TextEditingController();
  final _descriptionController = TextEditingController();
  final _iconController = TextEditingController();
  var _submitting = false;
  var _visibility = 'private';
  SpaceCreateActionError? _error;
  String? _createdSpaceId;

  @override
  void dispose() {
    _nameController.dispose();
    _descriptionController.dispose();
    _iconController.dispose();
    super.dispose();
  }

  bool get _uncertain => _error?.kind == SpaceCreateErrorKind.uncertain;
  bool get _canEdit => !_submitting && !_uncertain && _createdSpaceId == null;
  bool get _validIcon {
    final text = _iconController.text.trim();
    if (text.isEmpty) return true;
    final uri = Uri.tryParse(text);
    return uri != null &&
        (uri.scheme == 'https' || uri.scheme == 'http') &&
        uri.host.isNotEmpty;
  }

  bool get _canSubmit =>
      !_submitting &&
      !_uncertain &&
      _nameController.text.trim().isNotEmpty &&
      _validIcon;
  void _close() {
    if (!_submitting) Navigator.of(context).maybePop();
  }

  Future<void> _submit() async {
    if (!_canSubmit) return;
    FocusScope.of(context).unfocus();
    setState(() => _submitting = true);
    final error = await ref
        .read(spaceActionsProvider)
        .createSpace(
          name: _nameController.text,
          description: _descriptionController.text,
          iconUrl: _iconController.text,
          visibility: _visibility,
          retryIconForSpaceId: _createdSpaceId,
        );
    if (!mounted) return;
    setState(() {
      _submitting = false;
      _error = error;
      _createdSpaceId = error?.createdSpaceId ?? _createdSpaceId;
    });
    if (error == null) Navigator.of(context).pop();
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    final colors = VoiceColors.of(context);
    final name = _nameController.text.trim();
    final labels = {
      'public': l10n.spaceCreatePublic,
      'invite_only': l10n.spaceCreateInviteOnly,
      'private': l10n.spaceCreatePrivate,
    };
    final errorText = switch (_error?.kind) {
      SpaceCreateErrorKind.iconUpdate => l10n.spaceCreateIconFailed,
      SpaceCreateErrorKind.uncertain => l10n.spaceCreateUncertain,
      SpaceCreateErrorKind.rejected => l10n.spaceCreateError(
        commonActionErrorMessage(l10n),
      ),
      null => null,
    };
    return PopScope(
      canPop: !_submitting,
      child: VoiceFocusTrap(
        onEscape: _close,
        child: Material(
          color: colors.canvas,
          child: SafeArea(
            key: CreateSpaceSheet.sheetKey,
            child: Padding(
              padding: EdgeInsets.only(
                bottom: MediaQuery.viewInsetsOf(context).bottom,
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Container(
                    constraints: const BoxConstraints(minHeight: 56),
                    padding: const EdgeInsets.fromLTRB(16, 4, 8, 4),
                    decoration: BoxDecoration(
                      border: Border(
                        bottom: BorderSide(color: colors.borderDefault),
                      ),
                    ),
                    child: Row(
                      children: [
                        Expanded(
                          child: Text(
                            l10n.spaceCreateTitle,
                            style: theme.textTheme.titleMedium,
                          ),
                        ),
                        IconButton(
                          key: CreateSpaceSheet.closeKey,
                          tooltip: MaterialLocalizations.of(
                            context,
                          ).closeButtonTooltip,
                          onPressed: _submitting ? null : _close,
                          icon: const Icon(Icons.close),
                        ),
                      ],
                    ),
                  ),
                  Expanded(
                    child: SingleChildScrollView(
                      padding: const EdgeInsets.all(16),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        children: [
                          Row(
                            children: [
                              VoiceAvatar(
                                label: name.isEmpty ? '+' : name,
                                radius: 20,
                              ),
                              const SizedBox(width: 14),
                              Expanded(
                                child: TextField(
                                  key: CreateSpaceSheet.iconFieldKey,
                                  controller: _iconController,
                                  keyboardType: TextInputType.url,
                                  decoration: InputDecoration(
                                    labelText: l10n.spaceCreateIconLabel,
                                    hintText: l10n.spaceCreateIconHint,
                                    errorText: _validIcon
                                        ? null
                                        : l10n.spaceCreateIconInvalid,
                                  ),
                                  enabled: !_submitting && !_uncertain,
                                  onChanged: (_) => setState(() {}),
                                ),
                              ),
                            ],
                          ),
                          const SizedBox(height: 8),
                          Text(
                            l10n.spaceCreateIconHelp,
                            style: theme.textTheme.bodySmall,
                          ),
                          const SizedBox(height: 16),
                          TextField(
                            key: CreateSpaceSheet.nameFieldKey,
                            controller: _nameController,
                            decoration: InputDecoration(
                              labelText: l10n.spaceCreateNameLabel,
                              hintText: l10n.spaceCreateNameHint,
                            ),
                            textCapitalization: TextCapitalization.sentences,
                            enabled: _canEdit,
                            onChanged: (_) => setState(() {}),
                          ),
                          const SizedBox(height: 16),
                          Text(
                            l10n.spaceCreateAccess,
                            style: theme.textTheme.labelMedium,
                          ),
                          const SizedBox(height: 6),
                          Wrap(
                            spacing: 6,
                            runSpacing: 6,
                            children: [
                              for (final entry in labels.entries)
                                ChoiceChip(
                                  label: Text(entry.value),
                                  showCheckmark: false,
                                  shape: RoundedRectangleBorder(
                                    borderRadius: BorderRadius.circular(6),
                                  ),
                                  side: BorderSide(
                                    color: _visibility == entry.key
                                        ? colors.profileAccent
                                        : colors.borderDefault,
                                  ),
                                  selected: _visibility == entry.key,
                                  selectedColor: colors.profileAccent,
                                  labelStyle: theme.textTheme.labelLarge
                                      ?.copyWith(
                                        color: _visibility == entry.key
                                            ? theme.colorScheme.onPrimary
                                            : colors.textPrimary,
                                      ),
                                  onSelected: _canEdit
                                      ? (_) => setState(
                                          () => _visibility = entry.key,
                                        )
                                      : null,
                                ),
                            ],
                          ),
                          const SizedBox(height: 16),
                          Text(
                            l10n.spaceCreateTemplate,
                            style: theme.textTheme.labelMedium,
                          ),
                          const SizedBox(height: 8),
                          LayoutBuilder(
                            builder: (context, constraints) => Wrap(
                              spacing: 10,
                              runSpacing: 10,
                              children: [
                                for (final label in [
                                  l10n.spaceCreateGaming,
                                  l10n.spaceCreateWork,
                                  l10n.spaceCreateSocial,
                                ])
                                  SizedBox(
                                    width:
                                        MediaQuery.sizeOf(context).width >= 840
                                        ? (constraints.maxWidth - 10) / 2
                                        : constraints.maxWidth,
                                    child: OutlinedButton(
                                      onPressed: null,
                                      style: OutlinedButton.styleFrom(
                                        alignment: Alignment.centerLeft,
                                        padding: const EdgeInsets.all(12),
                                        shape: RoundedRectangleBorder(
                                          borderRadius: BorderRadius.circular(
                                            8,
                                          ),
                                        ),
                                      ),
                                      child: Text(label),
                                    ),
                                  ),
                              ],
                            ),
                          ),
                          const SizedBox(height: 8),
                          Text(
                            l10n.spaceCreateTemplatesUnavailable,
                            style: theme.textTheme.bodySmall,
                          ),
                          const SizedBox(height: 16),
                          TextField(
                            key: CreateSpaceSheet.descriptionFieldKey,
                            controller: _descriptionController,
                            decoration: InputDecoration(
                              labelText: l10n.spaceCreateDescriptionLabel,
                              hintText: l10n.spaceCreateDescriptionHint,
                            ),
                            textCapitalization: TextCapitalization.sentences,
                            maxLines: 3,
                            enabled: _canEdit,
                          ),
                        ],
                      ),
                    ),
                  ),
                  Container(
                    padding: const EdgeInsets.fromLTRB(16, 10, 16, 10),
                    decoration: BoxDecoration(
                      border: Border(
                        top: BorderSide(color: colors.borderDefault),
                      ),
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        if (errorText != null) ...[
                          Semantics(
                            liveRegion: true,
                            child: Text(
                              errorText,
                              style: theme.textTheme.bodyMedium?.copyWith(
                                color: colors.error,
                              ),
                            ),
                          ),
                          const SizedBox(height: 8),
                        ],
                        OverflowBar(
                          alignment: MainAxisAlignment.end,
                          spacing: 8,
                          overflowSpacing: 8,
                          children: [
                            TextButton(
                              onPressed: _submitting ? null : _close,
                              child: Text(l10n.commonCancel),
                            ),
                            FilledButton(
                              key: CreateSpaceSheet.submitKey,
                              onPressed: _canSubmit ? _submit : null,
                              child: _submitting
                                  ? const SizedBox(
                                      width: 20,
                                      height: 20,
                                      child: CircularProgressIndicator(
                                        strokeWidth: 2,
                                      ),
                                    )
                                  : Text(
                                      _createdSpaceId == null
                                          ? l10n.spaceCreateSubmit
                                          : l10n.spaceCreateRetryIcon,
                                    ),
                            ),
                          ],
                        ),
                      ],
                    ),
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
