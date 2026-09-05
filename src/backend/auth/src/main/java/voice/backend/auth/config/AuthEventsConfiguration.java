package voice.backend.auth.config;

import org.springframework.boot.autoconfigure.condition.ConditionalOnExpression;
import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.context.annotation.Primary;
import org.springframework.context.annotation.Profile;
import org.springframework.beans.factory.annotation.Qualifier;
import voice.backend.auth.events.AuthEventPublisher;
import voice.backend.auth.events.NatsAuthEventPublisher;
import voice.backend.auth.events.NoopAuthEventPublisher;
import voice.backend.auth.support.RecordingAuthEventPublisher;
import voice.backend.auth.service.GuestConversionEventPublisher;
import voice.backend.auth.service.AccountDeletionEventPublisher;
import voice.backend.auth.service.GuestConversionPendingEventRecoveryRunner;
import voice.backend.auth.service.GuestConversionPendingEventWorker;
import voice.backend.auth.service.GuestConversionPublishAck;
import voice.backend.auth.service.UnavailableGuestConversionEventPublisher;
import voice.backend.auth.service.UnavailableAccountDeletionEventPublisher;

@Configuration
public class AuthEventsConfiguration {
  @Bean
  @Profile("!test")
  @ConditionalOnExpression("!'${auth.nats.url:}'.isBlank()")
  NatsAuthEventPublisher natsAuthEventPublisher(AuthProperties properties) {
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

  @Bean
  @Profile("test")
  @ConditionalOnMissingBean(GuestConversionEventPublisher.class)
  GuestConversionEventPublisher testGuestConversionEventPublisher(
      @Qualifier("testAuthEventPublisher") AuthEventPublisher events) {
    return (subject, envelope, natsMessageId) -> {
      if (!AuthEventPublisher.SUBJECT_GUEST_CONVERTED.equals(subject)
          || !natsMessageId.equals(envelope.getEventId())
          || !envelope.hasUserGuestConverted()) {
        throw new IllegalArgumentException("invalid guest conversion event envelope");
      }
      events.publishGuestConverted(java.util.UUID.fromString(envelope.getUserGuestConverted().getAccountId()));
      return new GuestConversionPublishAck("test", 1L);
    };
  }

  @Bean
  @Profile("test")
  @ConditionalOnMissingBean(AccountDeletionEventPublisher.class)
  AccountDeletionEventPublisher testAccountDeletionEventPublisher(
      @Qualifier("testAuthEventPublisher") AuthEventPublisher events) {
    return (subject, envelope, natsMessageId) -> {
      if (!AuthEventPublisher.SUBJECT_ACCOUNT_DELETED.equals(subject)
          || !natsMessageId.equals(envelope.getEventId())
          || !envelope.hasUserAccountDeleted()) {
        throw new IllegalArgumentException("invalid account deletion event envelope");
      }
      events.publishAccountDeleted(
          java.util.UUID.fromString(envelope.getUserAccountDeleted().getAccountId()));
      return new GuestConversionPublishAck("test", 1L);
    };
  }

  @Bean
  @Profile("!test")
  @ConditionalOnMissingBean(GuestConversionEventPublisher.class)
  GuestConversionEventPublisher unavailableGuestConversionEventPublisher() {
    return new UnavailableGuestConversionEventPublisher();
  }

  @Bean
  @Profile("!test")
  @ConditionalOnMissingBean(AccountDeletionEventPublisher.class)
  AccountDeletionEventPublisher unavailableAccountDeletionEventPublisher() {
    return new UnavailableAccountDeletionEventPublisher();
  }

  @Bean
  @org.springframework.boot.autoconfigure.condition.ConditionalOnBean(GuestConversionEventPublisher.class)
  GuestConversionPendingEventWorker guestConversionPendingEventWorker(
      voice.backend.auth.repository.GuestConversionOperationRepository operations,
      GuestConversionEventPublisher publisher,
      java.time.Clock clock) {
    return new GuestConversionPendingEventWorker(operations, publisher, clock);
  }

  @Bean
  @org.springframework.boot.autoconfigure.condition.ConditionalOnBean(GuestConversionPendingEventWorker.class)
  @org.springframework.boot.autoconfigure.condition.ConditionalOnProperty(
      prefix = "auth.guest-conversion.pending-event", name = "enabled", havingValue = "true", matchIfMissing = true)
  GuestConversionPendingEventRecoveryRunner guestConversionPendingEventRecoveryRunner(
      GuestConversionPendingEventWorker worker,
      GuestConversionPendingEventRecoveryProperties properties) {
    return new GuestConversionPendingEventRecoveryRunner(worker, properties);
  }
}
