package voice.backend.auth.releasepolicy;

import static org.assertj.core.api.Assertions.*;
import java.nio.file.*;
import java.nio.file.attribute.PosixFilePermissions;
import java.util.*;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.springframework.mock.env.MockEnvironment;
import org.springframework.beans.factory.support.DefaultListableBeanFactory;
import com.fasterxml.jackson.databind.ObjectMapper;

class ReleasePolicyBootstrapTest {
  @TempDir Path directory;
  @Test void bootstrapRejectsMissingProfileBeforeOrdinaryBeansAndRuntimeSourceCannotBindBuild() throws Exception {
    var factory = new DefaultListableBeanFactory();
    assertThatThrownBy(() -> ReleasePolicyConfiguration.releasePolicyBootstrap(new MockEnvironment())
        .postProcessBeanFactory(factory)).isInstanceOf(IllegalArgumentException.class);
    assertThat(factory.containsSingleton("ownerReleasePolicy")).isFalse();
    Path policy = directory.resolve("policy.json"); Files.write(policy, ReleasePolicyTest.fixture());
    var env = new MockEnvironment().withProperty("VOICE_RELEASE_PROFILE", ReleasePolicy.ALPHA)
        .withProperty("VOICE_RELEASE_POLICY_FILE", policy.toString())
        .withProperty("VOICE_RELEASE_POLICY_SHA256", ReleasePolicy.digest(ReleasePolicyTest.fixture()))
        .withProperty("VOICE_RELEASE_SNAPSHOT_FILE", directory.resolve("snapshot.json").toString())
        .withProperty("VOICE_SOURCE_SHA", "a".repeat(40));
    assertThatThrownBy(() -> ReleasePolicyConfiguration.releasePolicyBootstrap(env)
        .postProcessBeanFactory(factory)).isInstanceOf(IllegalArgumentException.class);
    // Explicit development still installs the actual serving singleton before beans.
    ReleasePolicyConfiguration.releasePolicyBootstrap(new MockEnvironment()
        .withProperty("VOICE_RELEASE_PROFILE", "development")).postProcessBeanFactory(factory);
    assertThat(((ReleasePolicy) factory.getSingleton("ownerReleasePolicy")).allowsPhoneAuth()).isTrue();
  }
  @Test void snapshotIsPrivateExclusiveBoundAndNeverFollowsExistingSymlinks() throws Exception {
    var policy = ReleasePolicyTest.alpha();
    Path file = directory.resolve("snapshot.json");
    var adapters = Map.of("database", "jdbc", "mail", "real", "phone_provider", "absent");
    var routes = List.of("grpc:/app.voice.auth.v1.AuthService/Register");
    ReleasePolicyConfiguration.writeSnapshot(policy, file.toString(), "owned-instance", routes, adapters);
    assertThat(Files.getPosixFilePermissions(file)).isEqualTo(PosixFilePermissions.fromString("rw-------"));
    var json = new ObjectMapper().readTree(Files.readAllBytes(file));
    assertThat(json.get("policy_sha256").textValue()).isEqualTo(policy.sha256());
    assertThat(json.get("build_source_sha").textValue()).isEqualTo(policy.sourceSHA());
    assertThat(json.get("capability_states").get("phone_auth").textValue()).isEqualTo("disabled");
    assertThat(json.get("route_inventory_sha256").textValue())
        .isEqualTo(ReleasePolicy.digest(new ObjectMapper().writeValueAsBytes(routes)));
    byte[] original = Files.readAllBytes(file);
    assertThatThrownBy(() -> ReleasePolicyConfiguration.writeSnapshot(policy, file.toString(), "other", routes, adapters))
        .isInstanceOf(IllegalStateException.class);
    assertThat(Files.readAllBytes(file)).isEqualTo(original);
    Path link = directory.resolve("link"); Files.createSymbolicLink(link, file);
    assertThatThrownBy(() -> ReleasePolicyConfiguration.writeSnapshot(policy, link.toString(), "other", routes, adapters))
        .isInstanceOf(IllegalStateException.class);
    assertThat(Files.readAllBytes(file)).isEqualTo(original);
  }
}
