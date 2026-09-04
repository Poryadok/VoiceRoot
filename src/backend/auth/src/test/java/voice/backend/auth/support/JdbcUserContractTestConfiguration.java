package voice.backend.auth.support;

import java.util.Collection;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.CopyOnWriteArrayList;
import org.springframework.boot.test.context.TestConfiguration;
import org.springframework.context.annotation.Bean;
import voice.backend.auth.userdb.PhoneHashResolver;
import voice.backend.auth.userdb.PrimaryProfileProvisioner;
import voice.backend.auth.userdb.ProfileSwitchValidator;
import voice.backend.auth.userdb.UserVerificationSync;

/** Test-side User contract ports for JDBC integration tests that exercise only {@code auth_db}. */
@TestConfiguration(proxyBeanMethods = false)
public class JdbcUserContractTestConfiguration {
  @Bean
  RecordingUserContractPorts recordingUserContractPorts() {
    return new RecordingUserContractPorts();
  }

  public static final class RecordingUserContractPorts
      implements PrimaryProfileProvisioner,
          PhoneHashResolver,
          ProfileSwitchValidator,
          UserVerificationSync {
    private final Map<UUID, String> profileIds = new ConcurrentHashMap<>();
    private final List<EnsurePrimaryProfileCall> ensurePrimaryProfileCalls =
        new CopyOnWriteArrayList<>();

    @Override
    public String ensurePrimaryProfile(UUID accountId, String displayHint, boolean guestAccount) {
      String profileId =
          profileIds.computeIfAbsent(accountId, ignored -> UUID.randomUUID().toString());
      ensurePrimaryProfileCalls.add(
          new EnsurePrimaryProfileCall(accountId, displayHint, guestAccount, profileId));
      return profileId;
    }

    @Override
    public Map<String, String> resolvePrimaryProfileIdsByPhoneHashes(
        Collection<String> phoneHashes) {
      return Map.of();
    }

    @Override
    public void validateOwnedSwitchable(UUID accountId, UUID profileId) {}

    @Override
    public void setPersonalVerification(UUID profileId, String badge) {}

    @Override
    public void clearVerification(UUID profileId) {}

    public List<EnsurePrimaryProfileCall> ensurePrimaryProfileCalls() {
      return List.copyOf(ensurePrimaryProfileCalls);
    }

    public void reset() {
      profileIds.clear();
      ensurePrimaryProfileCalls.clear();
    }
  }

  public record EnsurePrimaryProfileCall(
      UUID accountId, String displayHint, boolean guestAccount, String profileId) {}
}
