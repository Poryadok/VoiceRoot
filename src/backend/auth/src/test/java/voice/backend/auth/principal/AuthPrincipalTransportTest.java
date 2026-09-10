package voice.backend.auth.principal;

import static org.junit.jupiter.api.Assertions.*;
import io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.springframework.mock.env.MockEnvironment;

class AuthPrincipalTransportTest {
  @Test void enabledPrincipalRequiresTlsByDefaultAndForEveryNonlocalProfile() {
    for (String[] profiles : new String[][] {{}, {"staging"}, {"production"}, {"test", "staging"},
        {"test", "production"}, {"local", "unknown"}}) {
      var environment = new MockEnvironment(); environment.setActiveProfiles(profiles);
      assertThrows(IllegalArgumentException.class,
          () -> AuthPrincipalTransport.configure(NettyServerBuilder.forPort(0), environment, true));
    }
  }

  @Test void allowsPlaintextOnlyWhenAllActiveProfilesAreExplicitlyLocalOrTest() {
    for (String[] profiles : new String[][] {{"local"}, {"test"}, {"local", "test"}}) {
      var environment = new MockEnvironment(); environment.setActiveProfiles(profiles);
      assertDoesNotThrow(() -> AuthPrincipalTransport.configure(NettyServerBuilder.forPort(0), environment, true));
    }
  }

  @Test void disabledPrincipalPreservesExistingServerWithoutTls() {
    assertDoesNotThrow(() -> AuthPrincipalTransport.configure(NettyServerBuilder.forPort(0), new MockEnvironment(), false));
  }

  @Test void rejectsPartialAndExplicitlyEmptyTlsPairsEvenInTestOrWhenDisabled() {
    for (boolean enabled : List.of(true, false)) {
      for (String name : List.of("AUTH_GRPC_TLS_CERT_FILE", "AUTH_GRPC_TLS_KEY_FILE")) {
        for (String value : List.of("", "missing-file.pem")) {
          var environment = new MockEnvironment().withProperty(name, value); environment.setActiveProfiles("test");
          assertThrows(IllegalArgumentException.class,
              () -> AuthPrincipalTransport.configure(NettyServerBuilder.forPort(0), environment, enabled));
        }
      }
      var blankPair = new MockEnvironment().withProperty("AUTH_GRPC_TLS_CERT_FILE", "")
          .withProperty("AUTH_GRPC_TLS_KEY_FILE", "");
      blankPair.setActiveProfiles("test");
      assertThrows(IllegalArgumentException.class,
          () -> AuthPrincipalTransport.configure(NettyServerBuilder.forPort(0), blankPair, enabled));
    }
  }
}
