package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;

import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.context.ApplicationContext;
import org.springframework.test.context.ActiveProfiles;
import voice.backend.auth.support.RecordingAuthEventPublisher;

@SpringBootTest
@ActiveProfiles("test")
class GuestConvertNatsEventIntegrationTest {
  @Autowired ApplicationContext applicationContext;

  @Test
  void convertGuestPublishesUserGuestConvertedEvent() {
    RecordingAuthEventPublisher events = findRecordingPublisher(applicationContext);
    assertThat(events)
        .as("AuthEventPublisher bean for user.events (mock/recording in test profile)")
        .isNotNull();
    events.clear();

    // Exercise convert-guest via REST in a follow-up once publisher is wired; for now assert contract.
    assertThat(events.publishedSubjects())
        .as("convert-guest must publish user.guest_converted to NATS")
        .contains("user.guest_converted");
  }

  private static RecordingAuthEventPublisher findRecordingPublisher(ApplicationContext ctx) {
    for (String name : ctx.getBeanDefinitionNames()) {
      Object bean = ctx.getBean(name);
      if (bean instanceof RecordingAuthEventPublisher recording) {
        return recording;
      }
    }
    return null;
  }
}
