package voice.backend.auth.config;

import org.springframework.boot.autoconfigure.condition.ConditionalOnExpression;
import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.context.annotation.Primary;
import org.springframework.context.annotation.Profile;
import voice.backend.auth.events.AuthEventPublisher;
import voice.backend.auth.events.NatsAuthEventPublisher;
import voice.backend.auth.events.NoopAuthEventPublisher;
import voice.backend.auth.support.RecordingAuthEventPublisher;

@Configuration
public class AuthEventsConfiguration {
  @Bean
  @ConditionalOnExpression("!'${auth.nats.url:}'.isBlank()")
  AuthEventPublisher natsAuthEventPublisher(AuthProperties properties) {
    return new NatsAuthEventPublisher(properties.getNats().getUrl());
  }

  @Bean
  @ConditionalOnMissingBean(AuthEventPublisher.class)
  AuthEventPublisher noopAuthEventPublisher() {
    return new NoopAuthEventPublisher();
  }

  @Bean
  @Profile("test")
  @Primary
  RecordingAuthEventPublisher recordingAuthEventPublisher() {
    return new RecordingAuthEventPublisherImpl(new NoopAuthEventPublisher());
  }

  @Bean
  @Profile("test")
  @Primary
  AuthEventPublisher testAuthEventPublisher(RecordingAuthEventPublisher recording) {
    return (AuthEventPublisher) recording;
  }
}
