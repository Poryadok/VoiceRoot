package voice.backend.auth.sdkidentity;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.Mockito.mock;

import ch.qos.logback.classic.Level;
import ch.qos.logback.classic.Logger;
import ch.qos.logback.classic.spi.ILoggingEvent;
import ch.qos.logback.core.read.ListAppender;
import java.nio.charset.StandardCharsets;
import java.time.Clock;
import java.util.Map;
import org.junit.jupiter.api.Test;
import org.slf4j.LoggerFactory;

class SdkDeviceAuthorityDiagnosticsTest {
  @Test void malformedProofLogsOnlyBoundedStageWithoutCredentialOrThrowable() {
    Logger logger = (Logger) LoggerFactory.getLogger(SdkIdentityService.class);
    Level original = logger.getLevel();
    var appender = new ListAppender<ILoggingEvent>();
    appender.start();
    logger.addAppender(appender);
    logger.setLevel(Level.DEBUG);
    String token = "x".repeat(43);
    String proof = "synthetic-private-proof-with-hostile-raw-details";
    var service = new SdkIdentityService(null, null, null, Map.of(), null, Clock.systemUTC(),
        mock(SdkDeviceStatusIssuer.class), mock(SdkBindingAuthority.class));
    try {
      assertThatThrownBy(() -> service.deviceAuthority(token, proof, proof.getBytes(StandardCharsets.US_ASCII)))
          .isInstanceOf(SdkIdentityDeniedException.class).hasMessage("invalid_sdk_identity");
      assertThat(appender.list).hasSize(1);
      ILoggingEvent event = appender.list.getFirst();
      assertThat(event.getFormattedMessage()).isEqualTo("SDK device authority denied stage=PARSE");
      assertThat(event.getArgumentArray()).containsExactly("PARSE");
      assertThat(event.getThrowableProxy()).isNull();
      assertThat(event.getFormattedMessage()).doesNotContain(token, proof);
    } finally {
      logger.detachAppender(appender);
      logger.setLevel(original);
      appender.stop();
    }
  }
}
