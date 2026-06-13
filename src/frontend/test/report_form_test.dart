import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/ui/report/report_sheet.dart';

void main() {
  group('reportFormValidationError', () {
    test('requires comment for other category', () {
      expect(
        reportFormValidationError(
          category: ReportCategories.other,
          comment: '',
          otherCategoryCommentRequired: 'required',
        ),
        'required',
      );
    });

    test('allows other category with comment', () {
      expect(
        reportFormValidationError(
          category: ReportCategories.other,
          comment: 'details',
          otherCategoryCommentRequired: 'required',
        ),
        isNull,
      );
    });

    test('allows non-other categories without comment', () {
      expect(
        reportFormValidationError(
          category: ReportCategories.spam,
          comment: '',
          otherCategoryCommentRequired: 'required',
        ),
        isNull,
      );
    });
  });
}
