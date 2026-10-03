package voice.backend.auth.sdkidentity;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.springframework.mock.env.MockEnvironment;

class AuthGameBindingApprovalCodeVaultTest {
  private static final byte[] KEY = "abcdef0123456789abcdef0123456789".getBytes(java.nio.charset.StandardCharsets.US_ASCII);

  @Test
  void encryptedReceiptRecoversOnlyForExactAuthorizationAndCiphertext() {
    var vault = new AuthGameBindingApprovalCodeVault(KEY);
    UUID request = UUID.randomUUID();
    String code = "A".repeat(43);
    var sealed = vault.seal(request, code);
    assertThat(new String(sealed.ciphertext(), java.nio.charset.StandardCharsets.US_ASCII)).doesNotContain(code);
    assertThat(vault.open(request, sealed.nonce(), sealed.ciphertext())).isEqualTo(code);
    assertThatThrownBy(() -> vault.open(UUID.randomUUID(), sealed.nonce(), sealed.ciphertext()))
        .isInstanceOf(SdkIdentityDeniedException.class);
    byte[] changed = sealed.ciphertext().clone(); changed[0] ^= 1;
    assertThatThrownBy(() -> vault.open(request, sealed.nonce(), changed)).isInstanceOf(SdkIdentityDeniedException.class);
  }

  @Test
  void configurationIsOptInAndRequiresCanonicalNonzeroAes256Key() {
    assertThat(AuthGameBindingApprovalCodeVault.configured(new MockEnvironment())).isNull();
    MockEnvironment configured = new MockEnvironment().withProperty("AUTH_GAME_BINDING_APPROVAL_CODE_KEY_B64",
        java.util.Base64.getEncoder().encodeToString(KEY));
    assertThat(AuthGameBindingApprovalCodeVault.configured(configured)).isNotNull();
    MockEnvironment invalid = new MockEnvironment().withProperty("AUTH_GAME_BINDING_APPROVAL_CODE_KEY_B64", "short");
    assertThatThrownBy(() -> AuthGameBindingApprovalCodeVault.configured(invalid)).isInstanceOf(IllegalStateException.class);
  }
}
