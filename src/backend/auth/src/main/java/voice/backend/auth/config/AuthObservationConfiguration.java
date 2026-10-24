package voice.backend.auth.config;

import io.micrometer.core.instrument.MeterRegistry;
import java.time.Clock;
import org.springframework.beans.factory.ObjectProvider;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import voice.backend.auth.events.AuthObservationDispatcher;
import voice.backend.auth.events.NatsAuthEventPublisher;

/** Observation delivery is separate from the durable guest/deletion worker configuration. */
@Configuration
public class AuthObservationConfiguration {
  @Bean(destroyMethod = "close")
  AuthObservationDispatcher authObservationDispatcher(
      ObjectProvider<NatsAuthEventPublisher> nats, Clock clock, MeterRegistry meters) {
    NatsAuthEventPublisher publisher = nats.getIfAvailable();
    return new AuthObservationDispatcher(
        publisher == null ? null : publisher::publishObservation, clock, meters);
  }
}
