package voice.backend.auth.sdkidentity;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import ch.qos.logback.classic.Level;
import ch.qos.logback.classic.Logger;
import ch.qos.logback.classic.spi.ILoggingEvent;
import ch.qos.logback.core.read.ListAppender;
import java.time.Clock;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.slf4j.LoggerFactory;

class SdkRegistryPolicyDiagnosticsTest {
  @Test void invalidEndpointLogsOnlyStageAndStatusWithoutConfigurationOrThrowable() {
    Logger logger = (Logger) LoggerFactory.getLogger(SdkGameIntegrationPolicyClient.class);
    Level original = logger.getLevel();
    var appender = new ListAppender<ILoggingEvent>();
    appender.start();
    logger.addAppender(appender);
    logger.setLevel(Level.DEBUG);
    String endpoint = "https://synthetic-private-user:private-password@registry.invalid";
    String key = "synthetic-private-workload-key";
    var client = new SdkGameIntegrationPolicyClient(endpoint, key, false, Clock.systemUTC());
    try {
      assertThatThrownBy(() -> client.resolve(UUID.randomUUID(), UUID.randomUUID()))
          .isInstanceOf(SdkIdentityDeniedException.class).hasMessage("invalid_sdk_identity");
      assertThat(appender.list).hasSize(1);
      ILoggingEvent event = appender.list.getFirst();
      assertThat(event.getFormattedMessage()).isEqualTo("SDK registry policy denied stage=ENDPOINT status=0");
      assertThat(event.getArgumentArray()).containsExactly("ENDPOINT", 0);
      assertThat(event.getThrowableProxy()).isNull();
      assertThat(event.getFormattedMessage()).doesNotContain(endpoint, key);
    } finally {
      logger.detachAppender(appender);
      logger.setLevel(original);
      appender.stop();
    }
  }
}
