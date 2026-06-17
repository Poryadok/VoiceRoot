package voice.backend.auth.support;

import java.util.List;

/**
 * Test double contract for Auth NATS publisher (user.guest_converted, etc.).
 * Production provides a real implementation; tests use a recording bean in test profile.
 */
public interface RecordingAuthEventPublisher {
  List<String> publishedSubjects();

  void clear();
}
