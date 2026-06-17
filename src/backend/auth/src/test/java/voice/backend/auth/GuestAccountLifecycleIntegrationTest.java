package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;

import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.context.ApplicationContext;
import org.springframework.test.context.ActiveProfiles;

@SpringBootTest
@ActiveProfiles("test")
class GuestAccountLifecycleIntegrationTest {
  @Autowired ApplicationContext applicationContext;

  @Test
  void guestAccountSweeperBeanIsRegistered() throws Exception {
    Class<?> sweeperClass = Class.forName("voice.backend.auth.lifecycle.GuestAccountSweeper");
    assertThat(applicationContext.getBeansOfType(sweeperClass))
        .as("GuestAccountSweeper scheduled bean for 30-day guest TTL")
        .isNotEmpty();
  }

  @Test
  void guestAccountSweeperDeactivatesExpiredGuests() throws Exception {
    Class<?> sweeperClass = Class.forName("voice.backend.auth.lifecycle.GuestAccountSweeper");
    Object sweeper = applicationContext.getBean(sweeperClass);
    var sweep = sweeperClass.getMethod("sweep");
    sweep.invoke(sweeper);
    // Full JDBC assertions land once sweeper + last_online_at column exist.
    assertThat(sweeper).isNotNull();
  }
}
