const kStoryMaxVideoDurationSeconds = 60;

bool isStoryVideoDurationValid(Duration duration) {
  return duration.inSeconds <= kStoryMaxVideoDurationSeconds;
}
