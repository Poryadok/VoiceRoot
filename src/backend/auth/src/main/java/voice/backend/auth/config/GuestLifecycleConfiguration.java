package voice.backend.auth.config;

import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.scheduling.annotation.EnableScheduling;
import voice.backend.auth.lifecycle.GuestAccountSweeper;
import voice.backend.auth.repository.AccountRepository;
import java.time.Clock;

@Configuration
@EnableScheduling
public class GuestLifecycleConfiguration {
  @Bean
  GuestAccountSweeper guestAccountSweeper(AccountRepository accounts, Clock clock) {
    return new GuestAccountSweeper(accounts, clock);
  }
}
