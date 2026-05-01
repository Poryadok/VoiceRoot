package voice.backend.auth.security;

import static org.assertj.core.api.Assertions.assertThat;

import org.junit.jupiter.api.Test;

class RefreshTokenCodecTest {
  @Test
  void generatedRefreshTokensAreOpaqueUniqueAndOnlyStoredAsHash() {
    RefreshTokenCodec codec = new RefreshTokenCodec();

    String first = codec.generate();
    String second = codec.generate();

    assertThat(first).isNotBlank();
    assertThat(second).isNotBlank().isNotEqualTo(first);
    assertThat(first).doesNotContain(".");
    assertThat(codec.hash(first)).hasSize(64).matches("[0-9a-f]{64}");
    assertThat(codec.hash(first)).isNotEqualTo(first);
    assertThat(codec.hash(first)).isEqualTo(codec.hash(first));
  }

  @Test
  void malformedRefreshTokensAreRejectedBeforeRepositoryLookup() {
    RefreshTokenCodec codec = new RefreshTokenCodec();

    assertThat(codec.isWellFormed("")).isFalse();
    assertThat(codec.isWellFormed("short")).isFalse();
    assertThat(codec.isWellFormed("contains space")).isFalse();
    assertThat(codec.isWellFormed(codec.generate())).isTrue();
  }
}
