package voice.backend.auth.sdkidentity;

import static org.assertj.core.api.Assertions.assertThat;

import java.util.UUID;
import org.junit.jupiter.api.Test;

class AuthGameBindingSubjectDigestTest {
  @Test
  void keyedDigestIsVersionedAndNamespacedAcrossAppEnvironmentAndProvider() {
    var digest = new AuthGameBindingSubjectDigest("digest-2026", "0123456789abcdef0123456789abcdef".getBytes(java.nio.charset.StandardCharsets.US_ASCII));
    UUID app = UUID.fromString("10000000-0000-4000-8000-000000000001");
    UUID env = UUID.fromString("20000000-0000-4000-8000-000000000002");
    String first = digest.digest("google", "https://accounts.google.com", app, env, "subject-123");

    assertThat(first).matches("hmac-sha256-v1:digest-2026:[0-9a-f]{64}");
    assertThat(digest.digest("google", "https://accounts.google.com", app, env, "subject-123")).isEqualTo(first);
    assertThat(digest.digest("apple", "https://accounts.google.com", app, env, "subject-123")).isNotEqualTo(first);
    assertThat(digest.digest("google", "https://accounts.google.com", app, UUID.randomUUID(), "subject-123"))
        .isNotEqualTo(first);
  }
}
