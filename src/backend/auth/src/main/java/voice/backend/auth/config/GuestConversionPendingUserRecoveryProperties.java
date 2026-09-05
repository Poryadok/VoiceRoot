package voice.backend.auth.config;

import jakarta.validation.constraints.AssertTrue;
import jakarta.validation.constraints.Positive;
import java.time.Duration;
import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.validation.annotation.Validated;

/** Bounded, enabled-by-default recovery settings for durable PENDING_USER work. */
@Validated
@ConfigurationProperties(prefix = "auth.guest-conversion.pending-user")
public class GuestConversionPendingUserRecoveryProperties {
  private boolean enabled = true;

  @Positive private int batchSize = 100;

  private Duration leaseDuration = Duration.ofMinutes(1);
  private Duration interval = Duration.ofSeconds(30);

  public boolean isEnabled() { return enabled; }
  public void setEnabled(boolean enabled) { this.enabled = enabled; }
  public int getBatchSize() { return batchSize; }
  public void setBatchSize(int batchSize) { this.batchSize = batchSize; }
  public Duration getLeaseDuration() { return leaseDuration; }
  public void setLeaseDuration(Duration leaseDuration) { this.leaseDuration = leaseDuration; }
  public Duration getInterval() { return interval; }
  public void setInterval(Duration interval) { this.interval = interval; }

  @AssertTrue(message = "lease-duration must be positive")
  public boolean isLeaseDurationPositive() {
    return leaseDuration != null && !leaseDuration.isZero() && !leaseDuration.isNegative();
  }

  @AssertTrue(message = "interval must be positive")
  public boolean isIntervalPositive() {
    return interval != null && !interval.isZero() && !interval.isNegative();
  }
}
