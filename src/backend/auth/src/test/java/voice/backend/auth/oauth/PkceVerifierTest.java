package voice.backend.auth.oauth;

import static org.assertj.core.api.Assertions.assertThat;

import org.junit.jupiter.api.Test;

class PkceVerifierTest {
  @Test
  void s256ChallengeMatchesVerifier() {
    String verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1hvtT0ZLU3-8xr4";
    String challenge = PkceVerifier.s256Challenge(verifier);
    assertThat(PkceVerifier.verifyS256(verifier, challenge)).isTrue();
  }

  @Test
  void s256RejectsMismatchedVerifier() {
    String verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1hvtT0ZLU3-8xr4";
    String challenge = PkceVerifier.s256Challenge(verifier);
    assertThat(PkceVerifier.verifyS256("wrong-verifier-value-012345678901234567890", challenge))
        .isFalse();
  }
}
