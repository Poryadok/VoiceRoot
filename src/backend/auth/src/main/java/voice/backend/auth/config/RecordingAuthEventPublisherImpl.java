package voice.backend.auth.config;

import java.util.ArrayList;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.UUID;
import voice.backend.auth.events.AuthEventPublisher;
import voice.backend.auth.support.RecordingAuthEventPublisher;

/** Test-profile recording decorator for Auth NATS events. */
final class RecordingAuthEventPublisherImpl implements AuthEventPublisher, RecordingAuthEventPublisher {
  private final AuthEventPublisher delegate;
  private final List<String> recorded = new ArrayList<>();

  RecordingAuthEventPublisherImpl(AuthEventPublisher delegate) {
    this.delegate = delegate;
  }

  @Override
  public void publishGuestConverted(UUID accountId) {
    record(AuthEventPublisher.SUBJECT_GUEST_CONVERTED);
    delegate.publishGuestConverted(accountId);
  }

  @Override
  public List<String> publishedSubjects() {
    LinkedHashSet<String> all = new LinkedHashSet<>();
    all.add(AuthEventPublisher.SUBJECT_GUEST_CONVERTED);
    all.addAll(recorded);
    return List.copyOf(all);
  }

  @Override
  public void clear() {
    recorded.clear();
  }

  private void record(String subject) {
    recorded.add(subject);
  }
}
