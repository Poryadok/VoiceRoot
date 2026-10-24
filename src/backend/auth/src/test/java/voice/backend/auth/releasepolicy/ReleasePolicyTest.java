package voice.backend.auth.releasepolicy;

import static org.assertj.core.api.Assertions.*;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Map;
import org.junit.jupiter.api.Test;

class ReleasePolicyTest {
  static byte[] fixture() throws Exception {
    return Files.readAllBytes(Path.of("../pkg/releasepolicy/testdata/free-invite-alpha.json"));
  }
  static ReleasePolicy alpha() throws Exception {
    byte[] bytes = fixture();
    return ReleasePolicy.parse(bytes, ReleasePolicy.digest(bytes), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa");
  }
  @Test void sharedFixtureIsBoundAndProjectionsCannotMutateAuthority() throws Exception {
    var policy = alpha();
    assertThat(policy.allowsPhoneAuth()).isFalse();
    assertThat(policy.profile()).isEqualTo("free-invite-alpha");
    assertThatThrownBy(() -> policy.capabilityStates().put("phone_auth", "qualified"))
        .isInstanceOf(UnsupportedOperationException.class);
    assertThat(policy.allowsPhoneAuth()).isFalse();
  }
  @Test void rejectsClosedSchemaAndBindingViolations() throws Exception {
    String json = new String(fixture(), StandardCharsets.UTF_8);
    for (String invalid : new String[] {
        json.replace("\"schema_version\": 2", "\"schema_version\": 2, \"schema_version\": 2"),
        json.replace("\"schema_version\": 2", "\"schema_version\": 2.0"),
        json.replace("\"schema_version\": 2", "\"schema_version\": 4294967298"),
        json.replace("\"schema_version\": 2", "\"schema_version\": \"2\""),
        json.replace("\"schema_version\": 2", "\"unknown\": true, \"schema_version\": 2"),
        json.replace("\"phone_auth\": \"disabled\"", "\"phone_auth\": \"qualified\""),
        json.replace("\"phone_auth\": \"disabled\"", "\"phone_auth\": \"disabled\", \"phone_auth\": \"disabled\""),
        json.replace("\"text\": \"qualified\"", "\"text\": \"disabled\""),
        json.replace("\"phone_auth\": \"disabled\",", ""),
        json + " {}", json + " ".repeat(65536), "[]"}) {
      byte[] bytes = invalid.getBytes(StandardCharsets.UTF_8);
      assertThatThrownBy(() -> ReleasePolicy.parse(bytes, ReleasePolicy.digest(bytes),
          "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
          .as(invalid.substring(0, Math.min(90, invalid.length())))
          .isInstanceOf(IllegalArgumentException.class);
    }
    assertThatThrownBy(() -> ReleasePolicy.parse(fixture(), "0".repeat(64), "1".repeat(40)))
        .isInstanceOf(IllegalArgumentException.class);
    assertThatThrownBy(() -> ReleasePolicy.parse(fixture(), ReleasePolicy.digest(fixture()), "2".repeat(40)))
        .isInstanceOf(IllegalArgumentException.class);
  }
  @Test void developmentIsExplicitAndCannotCarryArtifactOrSnapshotBindings() {
    assertThat(ReleasePolicy.load(Map.of("VOICE_RELEASE_PROFILE", "development"), "").allowsPhoneAuth()).isTrue();
    for (var env : java.util.List.of(Map.<String,String>of(),
        Map.of("VOICE_RELEASE_PROFILE", "development", "VOICE_RELEASE_POLICY_FILE", ""),
        Map.of("VOICE_RELEASE_PROFILE", "development", "VOICE_RELEASE_SNAPSHOT_FILE", "/tmp/snapshot"),
        Map.of("VOICE_RELEASE_PROFILE", "free-invite-alpha", "VOICE_SOURCE_SHA", "1".repeat(40)))) {
      assertThatThrownBy(() -> ReleasePolicy.load(env, "")).isInstanceOf(IllegalArgumentException.class);
    }
  }

  @Test void loadedFileIsReadOnceAndCannotChangeTheServingPolicy(@org.junit.jupiter.api.io.TempDir Path directory) throws Exception {
    Path file = directory.resolve("policy.json"); Files.write(file, fixture());
    var policy = ReleasePolicy.load(Map.of("VOICE_RELEASE_PROFILE", ReleasePolicy.ALPHA,
        "VOICE_RELEASE_POLICY_FILE", file.toString(), "VOICE_RELEASE_POLICY_SHA256", ReleasePolicy.digest(fixture())), "a".repeat(40));
    Files.writeString(file, "{}");
    assertThat(policy.allowsPhoneAuth()).isFalse();
    assertThat(policy.sha256()).isEqualTo(ReleasePolicy.digest(fixture()));
  }
}
