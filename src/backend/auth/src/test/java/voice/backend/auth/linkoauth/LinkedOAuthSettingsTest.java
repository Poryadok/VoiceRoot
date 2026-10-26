package voice.backend.auth.linkoauth;

import static org.assertj.core.api.Assertions.*;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.*;
import org.junit.jupiter.api.Test;
import org.springframework.boot.test.context.runner.ApplicationContextRunner;
import org.springframework.mock.env.MockEnvironment;

class LinkedOAuthSettingsTest {
  @Test void disabledNeedsNoStoreSecretsOrProviderAndEnabledIncompleteFailsStartup() {
    new ApplicationContextRunner().withUserConfiguration(LinkedOAuthConfiguration.class).run(context->{
      assertThat(context).hasNotFailed();assertThat(context).doesNotHaveBean(LinkedOAuthService.class);
    });
    assertThatThrownBy(()->new LinkedOAuthConfiguration().linkedOAuthSettings(new MockEnvironment()
        .withProperty("auth.linked-oauth.twitch.enabled","true"))).isInstanceOf(IllegalStateException.class)
        .hasMessageContaining("auth.linked-oauth.twitch.authorization-url");
  }
  @Test void endpointsNeverAcceptUserinfoQueriesFragmentsWildcardsEncodedSeparatorsOrSuffixTricks() {
    for(String uri:List.of("http://api.example.test/callback","https://user@api.example.test/callback","https://api.example.test/callback?next=evil",
        "https://api.example.test/callback#secret","https://*.example.test/callback","https://api.example.test/%2fcallback","https://api.example.test/a/../callback"))
      assertThatThrownBy(()->LinkedOAuthSettings.exactHttps(uri)).isInstanceOf(IllegalArgumentException.class);
    LinkedOAuthSettings.exactHttps("https://api.example.test/api/v1/auth/linked-accounts/twitch/oauth-callback");
    assertThatThrownBy(()->LinkedOAuthSettings.exactWindows("https://app.example.test/return")).isInstanceOf(IllegalArgumentException.class);
    LinkedOAuthSettings.exactWindows("voice://settings/oauth-return");
  }
  @Test void lifetimeLimitsAndS256RegistrationAreFailClosed() {
    byte[] key="0123456789abcdef0123456789abcdef".getBytes(StandardCharsets.US_ASCII);
    for(Duration ttl:List.of(Duration.ZERO,Duration.ofSeconds(-1),Duration.ofSeconds(301)))
      assertThatThrownBy(()->new LinkedOAuthSettings("revision",ttl,Duration.ofSeconds(60),key,Map.of())).isInstanceOf(IllegalArgumentException.class);
    assertThatThrownBy(()->new LinkedOAuthSettings("revision",Duration.ofSeconds(300),Duration.ofSeconds(61),key,Map.of())).isInstanceOf(IllegalArgumentException.class);
    var p=new LinkedOAuthSettings.Provider(true,false,"https://id.example.test/auth","https://id.example.test/token","https://id.example.test/user","id","secret",
        "https://api.example.test/api/v1/auth/linked-accounts/twitch/oauth-callback",Map.of("web","https://app.example.test/settings/oauth-return"));
    var settings=new LinkedOAuthSettings("revision",Duration.ofSeconds(300),Duration.ofSeconds(60),key,Map.of("twitch",p));
    assertThatThrownBy(settings::validateProduction).isInstanceOf(IllegalArgumentException.class);
    assertThat(settings.toString()).doesNotContain("secret");assertThat(p.toString()).doesNotContain("secret");
  }
}
