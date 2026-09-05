package voice.backend.auth.config;

import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.boot.context.properties.EnableConfigurationProperties;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.scheduling.annotation.EnableScheduling;
import voice.backend.auth.lifecycle.GuestAccountSweeper;
import voice.backend.auth.repository.AccountRepository;
import java.time.Clock;
import voice.backend.auth.service.GuestConversionPendingUserRecoveryRunner;
import voice.backend.auth.service.GuestConversionPendingUserWorker;

@Configuration
@EnableScheduling
@EnableConfigurationProperties({
    GuestConversionPendingUserRecoveryProperties.class,
    GuestConversionPendingEventRecoveryProperties.class
})
public class GuestLifecycleConfiguration {
  @Bean
  GuestAccountSweeper guestAccountSweeper(AccountRepository accounts, Clock clock) {
    return new GuestAccountSweeper(accounts, clock);
  }

  @Bean
  @ConditionalOnProperty(
      prefix = "auth.guest-conversion.pending-user", name = "enabled", havingValue = "true", matchIfMissing = true)
  GuestConversionPendingUserRecoveryRunner guestConversionPendingUserRecoveryRunner(
      GuestConversionPendingUserWorker worker, GuestConversionPendingUserRecoveryProperties properties) {
    return new GuestConversionPendingUserRecoveryRunner(worker, properties);
  }
}
