import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/ui/stories/story_video_validation.dart';

void main() {
  test('isStoryVideoDurationValid accepts up to 60 seconds', () {
    expect(isStoryVideoDurationValid(const Duration(seconds: 60)), isTrue);
    expect(isStoryVideoDurationValid(const Duration(seconds: 59)), isTrue);
    expect(isStoryVideoDurationValid(const Duration(seconds: 61)), isFalse);
  });
}
