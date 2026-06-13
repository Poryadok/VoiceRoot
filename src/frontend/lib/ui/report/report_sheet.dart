import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/moderation_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/trust_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_bottom_sheet.dart';
import '../core/voice_primary_button.dart';

/// Report target for moderation API.
sealed class ReportTarget {
  const ReportTarget();

  String get targetType;

  String get targetId;

  Map<String, Object?> get evidence;
}

final class ReportMessageTarget extends ReportTarget {
  const ReportMessageTarget({
    required this.messageId,
    required this.chatId,
  });

  final String messageId;
  final String chatId;

  @override
  String get targetType => 'message';

  @override
  String get targetId => messageId;

  @override
  Map<String, Object?> get evidence => {
    'chat_id': chatId,
    'message_id': messageId,
  };
}

final class ReportUserTarget extends ReportTarget {
  const ReportUserTarget({required this.profileId});

  final String profileId;

  @override
  String get targetType => 'user';

  @override
  String get targetId => profileId;

  @override
  Map<String, Object?> get evidence => const {};
}

final class ReportSpaceTarget extends ReportTarget {
  const ReportSpaceTarget({required this.spaceId});

  final String spaceId;

  @override
  String get targetType => 'space';

  @override
  String get targetId => spaceId;

  @override
  Map<String, Object?> get evidence => const {};
}

/// Category ids sent to the moderation API (`mm_toxic` is remapped server-side).
abstract final class ReportCategories {
  static const spam = 'spam';
  static const harassment = 'harassment';
  static const offensive = 'offensive';
  static const fake = 'fake';
  static const mmToxic = 'mm_toxic';
  static const other = 'other';

  static const all = [
    spam,
    harassment,
    offensive,
    fake,
    mmToxic,
    other,
  ];
}

const int kReportCommentMaxLength = 500;

/// Report flow: pick category, optional comment, submit.
class ReportSheet extends ConsumerStatefulWidget {
  const ReportSheet({super.key, required this.target});

  static const Key sheetKey = Key('report_sheet');
  static const Key categoryListKey = Key('report_category_list');
  static const Key commentFieldKey = Key('report_comment');
  static const Key submitButtonKey = Key('report_submit');
  static const Key acceptedKey = Key('report_accepted');

  final ReportTarget target;

  static Future<void> show(BuildContext context, {required ReportTarget target}) {
    return showVoiceBottomSheet<void>(
      context: context,
      initialSize: 0.65,
      minSize: 0.35,
      child: ReportSheet(target: target),
    );
  }

  @override
  ConsumerState<ReportSheet> createState() => _ReportSheetState();
}

class _ReportSheetState extends ConsumerState<ReportSheet> {
  String? _category;
  final _commentController = TextEditingController();
  var _submitting = false;
  var _accepted = false;
  String? _error;

  @override
  void dispose() {
    _commentController.dispose();
    super.dispose();
  }

  bool get _canSubmit {
    if (_category == null || _submitting) return false;
    if (_category == ReportCategories.other) {
      return _commentController.text.trim().isNotEmpty;
    }
    return true;
  }

  Future<void> _submit() async {
    if (!_canSubmit || _category == null) return;
    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null) return;

    setState(() {
      _submitting = true;
      _error = null;
    });

    final description = _commentController.text.trim();
    final result = await ref.read(voiceModerationClientProvider).createReport(
      authorization: auth,
      targetType: widget.target.targetType,
      targetId: widget.target.targetId,
      category: _category!,
      description: description.isEmpty ? null : description,
      evidence: widget.target.evidence,
    );

    if (!mounted) return;
    switch (result) {
      case ModerationApiOk():
        setState(() {
          _submitting = false;
          _accepted = true;
        });
      case ModerationApiFailure(:final message):
        setState(() {
          _submitting = false;
          _error = message;
        });
    }
  }

  String _categoryLabel(AppLocalizations l10n, String category) {
    return switch (category) {
      ReportCategories.spam => l10n.reportCategorySpam,
      ReportCategories.harassment => l10n.reportCategoryHarassment,
      ReportCategories.offensive => l10n.reportCategoryOffensive,
      ReportCategories.fake => l10n.reportCategoryFake,
      ReportCategories.mmToxic => l10n.reportCategoryMmToxic,
      ReportCategories.other => l10n.reportCategoryOther,
      _ => category,
    };
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);

    if (_accepted) {
      return SafeArea(
        key: ReportSheet.acceptedKey,
        child: Padding(
          padding: const EdgeInsets.fromLTRB(24, 16, 24, 24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Icon(Icons.check_circle_outline, size: 48, color: voice.profileAccent),
              const SizedBox(height: 16),
              Text(
                l10n.reportAcceptedTitle,
                textAlign: TextAlign.center,
                style: Theme.of(context).textTheme.titleLarge,
              ),
              const SizedBox(height: 8),
              Text(
                l10n.reportAcceptedMessage,
                textAlign: TextAlign.center,
                style: TextStyle(color: voice.textSecondary),
              ),
              const SizedBox(height: 24),
              VoicePrimaryButton(
                onPressed: () => Navigator.of(context).pop(),
                child: Text(l10n.commonCancel),
              ),
            ],
          ),
        ),
      );
    }

    return SafeArea(
      child: Padding(
        key: ReportSheet.sheetKey,
        padding: const EdgeInsets.fromLTRB(24, 16, 24, 24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              l10n.reportTitle,
              style: Theme.of(context).textTheme.titleLarge,
            ),
            const SizedBox(height: 8),
            Text(
              l10n.reportSubtitle,
              style: TextStyle(color: voice.textSecondary),
            ),
            const SizedBox(height: 16),
            ListView(
              key: ReportSheet.categoryListKey,
              shrinkWrap: true,
              physics: const NeverScrollableScrollPhysics(),
              children: [
                  for (final category in ReportCategories.all)
                    RadioListTile<String>(
                      value: category,
                      groupValue: _category,
                      title: Text(_categoryLabel(l10n, category)),
                      onChanged: _submitting
                          ? null
                          : (value) => setState(() => _category = value),
                    ),
                ],
              ),
            if (_category == ReportCategories.other) ...[
              const SizedBox(height: 8),
              TextFormField(
                key: ReportSheet.commentFieldKey,
                controller: _commentController,
                maxLength: kReportCommentMaxLength,
                maxLines: 3,
                decoration: InputDecoration(
                  labelText: l10n.reportCommentLabel,
                  helperText: l10n.reportCommentRequired,
                ),
                onChanged: (_) => setState(() {}),
              ),
            ],
            if (_error != null) ...[
              const SizedBox(height: 8),
              Text(
                _error!,
                style: TextStyle(color: Theme.of(context).colorScheme.error),
              ),
            ],
            const SizedBox(height: 16),
            VoicePrimaryButton(
              key: ReportSheet.submitButtonKey,
              onPressed: _canSubmit ? _submit : null,
              isLoading: _submitting,
              child: Text(l10n.reportSubmit),
            ),
          ],
        ),
      ),
    );
  }
}

/// Returns a localized validation error for report form state, or null if valid.
String? reportFormValidationError({
  required String? category,
  required String comment,
  required String otherCategoryCommentRequired,
}) {
  if (category == null) return null;
  if (category == ReportCategories.other && comment.trim().isEmpty) {
    return otherCategoryCommentRequired;
  }
  return null;
}
