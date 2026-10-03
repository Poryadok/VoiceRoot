package voice.backend.auth.authoritysource;

import static org.assertj.core.api.Assertions.*;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.springframework.mock.env.MockEnvironment;
import java.nio.file.*;

class AuthSourceConfigTest {
  @TempDir Path directory;
  @Test void onlyExplicitPerOwnerFlagEnablesTheSource() {
    assertThat(AuthSourceConfig.load(new MockEnvironment())).isNull();
    assertThat(AuthSourceConfig.load(new MockEnvironment().withProperty("S2S_JWKS_URLS_JSON","{}"))).isNull();
    assertThatThrownBy(()->AuthSourceConfig.load(new MockEnvironment().withProperty("AUTH_AUTHORITY_SOURCE_GRPC_LISTEN",":9097"))).isInstanceOf(IllegalArgumentException.class);
    assertThat(AuthSourceConfig.load(new MockEnvironment().withProperty("AUTH_AUTHORITY_SOURCE_ENABLED","false").withProperty("AUTH_AUTHORITY_SOURCE_GRPC_LISTEN","retained"))).isNull();
    assertThatThrownBy(()->AuthSourceConfig.load(new MockEnvironment().withProperty("AUTH_AUTHORITY_SOURCE_ENABLED","TRUE"))).isInstanceOf(IllegalArgumentException.class);
    assertThatThrownBy(()->AuthSourceConfig.load(new MockEnvironment().withProperty("AUTH_AUTHORITY_SOURCE_ENABLED","true"))).isInstanceOf(IllegalArgumentException.class);
  }
  @Test void enabledConfigRequiresFederationHttpsAndAllMtlsReplayInputs()throws Exception {
    for(String name:java.util.List.of("TLS_CERT_FILE","TLS_KEY_FILE","CLIENT_CA_FILE")) Files.writeString(directory.resolve(name),"test fixture, TLS parsing occurs at construction");
    var env=new MockEnvironment().withProperty("AUTH_AUTHORITY_SOURCE_ENABLED","true")
        .withProperty("AUTH_AUTHORITY_SOURCE_TLS_CERT_FILE",directory.resolve("TLS_CERT_FILE").toString())
        .withProperty("AUTH_AUTHORITY_SOURCE_TLS_KEY_FILE",directory.resolve("TLS_KEY_FILE").toString())
        .withProperty("AUTH_AUTHORITY_SOURCE_CLIENT_CA_FILE",directory.resolve("CLIENT_CA_FILE").toString())
        .withProperty("AUTH_AUTHORITY_SOURCE_REPLAY_REDIS_ADDR","127.0.0.1:6379")
        .withProperty("S2S_JWKS_URLS_JSON","{\"gateway\":\"https://gateway.test/jwks\",\"space\":\"https://space.test/jwks\",\"federation\":\"https://federation.test/jwks\"}");
    assertThat(AuthSourceConfig.load(env).listen().getPort()).isEqualTo(9097);
    for(String bad:java.util.List.of("{}","{\"federation\":\"http://federation.test/jwks\"}","{\"federation\":\"https://user@federation.test/jwks\"}","{\"federation\":\"https://federation.test/jwks#fragment\"}")) {
      env.setProperty("S2S_JWKS_URLS_JSON",bad);
      assertThatThrownBy(()->AuthSourceConfig.load(env)).isInstanceOf(IllegalArgumentException.class);
    }
  }
}
