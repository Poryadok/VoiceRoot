import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/space_providers.dart';
import '../core/voice_avatar.dart';
import '../core/voice_bottom_sheet.dart';

/// Bottom sheet: space name, description, optional icon URL → POST + PATCH.
class CreateSpaceSheet extends ConsumerStatefulWidget {
  const CreateSpaceSheet({super.key});

  static const Key sheetKey = Key('create_space_sheet');
  static const Key nameFieldKey = Key('create_space_name');
  static const Key descriptionFieldKey = Key('create_space_description');
  static const Key iconFieldKey = Key('create_space_icon');
  static const Key submitKey = Key('create_space_submit');

  static Future<void> show(BuildContext context) {
    final container = ProviderScope.containerOf(context);
    return showVoiceBottomSheet<void>(
      context: context,
      scrollable: false,
      child: UncontrolledProviderScope(
        container: container,
        child: const CreateSpaceSheet(),
      ),
    );
  }

  @override
  ConsumerState<CreateSpaceSheet> createState() => _CreateSpaceSheetState();
}

class _CreateSpaceSheetState extends ConsumerState<CreateSpaceSheet> {
  final _nameController = TextEditingController();
  final _descriptionController = TextEditingController();
  final _iconController = TextEditingController();
  var _submitting = false;

  @override
  void dispose() {
    _nameController.dispose();
    _descriptionController.dispose();
    _iconController.dispose();
    super.dispose();
  }

  bool get _canSubmit {
    final name = _nameController.text.trim();
    return !_submitting && name.isNotEmpty;
  }

  Future<void> _submit() async {
    if (!_canSubmit) return;
    setState(() => _submitting = true);
    final l10n = AppLocalizations.of(context)!;
    final err = await ref.read(spaceActionsProvider).createSpace(
      name: _nameController.text.trim(),
      description: _descriptionController.text,
      iconUrl: _iconController.text,
    );
    if (!mounted) return;
    setState(() => _submitting = false);
    if (err != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(l10n.spaceCreateError(err))),
      );
      return;
    }
    Navigator.of(context).pop();
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    final name = _nameController.text.trim();

    return SafeArea(
      key: CreateSpaceSheet.sheetKey,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(l10n.spaceCreateTitle, style: theme.textTheme.titleLarge),
            const SizedBox(height: 16),
            Row(
              children: [
                VoiceAvatar(
                  label: name.isEmpty ? '?' : name,
                  radius: 28,
                ),
                const SizedBox(width: 16),
                Expanded(
                  child: TextField(
                    key: CreateSpaceSheet.iconFieldKey,
                    controller: _iconController,
                    decoration: InputDecoration(
                      labelText: l10n.spaceCreateIconLabel,
                      hintText: l10n.spaceCreateIconHint,
                    ),
                    enabled: !_submitting,
                    onChanged: (_) => setState(() {}),
                  ),
                ),
              ],
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
              enabled: !_submitting,
              onChanged: (_) => setState(() {}),
            ),
            const SizedBox(height: 12),
            TextField(
              key: CreateSpaceSheet.descriptionFieldKey,
              controller: _descriptionController,
              decoration: InputDecoration(
                labelText: l10n.spaceCreateDescriptionLabel,
                hintText: l10n.spaceCreateDescriptionHint,
              ),
              textCapitalization: TextCapitalization.sentences,
              maxLines: 3,
              enabled: !_submitting,
            ),
            const SizedBox(height: 24),
            FilledButton(
              key: CreateSpaceSheet.submitKey,
              onPressed: _canSubmit ? _submit : null,
              child: _submitting
                  ? const SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : Text(l10n.spaceCreateSubmit),
            ),
          ],
        ),
      ),
    );
  }
}
