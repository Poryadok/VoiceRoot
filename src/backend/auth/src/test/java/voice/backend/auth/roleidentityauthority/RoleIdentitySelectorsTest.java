package voice.backend.auth.roleidentityauthority;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.util.Arrays;
import java.util.HexFormat;
import java.util.Map;
import java.util.UUID;
import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import org.junit.jupiter.api.Test;

class RoleIdentitySelectorsTest {
  private static final UUID ACCOUNT = UUID.fromString("e756fd2b-b744-45d5-86b4-a4ec81b33e96");

  private static byte[] key(int fill) {
    byte[] key = new byte[32];
    Arrays.fill(key, (byte) fill);
    return key;
  }

  @Test
  void accountSelectorUsesExactAuthRolePurposeAndCanonicalUuidBytes() throws Exception {
    byte[] secret = key(17);
    var selectors = new RoleIdentitySelectors(7, 2, Map.of(1, key(9), 2, secret));
    Mac reference = Mac.getInstance("HmacSHA256");
    reference.init(new SecretKeySpec(secret, "HmacSHA256"));
    reference.update("voice-role-identity-selector-v1\0auth\0account\0".getBytes(StandardCharsets.US_ASCII));
    byte[] uuid = ByteBuffer.allocate(16).putLong(ACCOUNT.getMostSignificantBits())
        .putLong(ACCOUNT.getLeastSignificantBits()).array();

    assertThat(selectors.activeAccount(ACCOUNT)).isEqualTo(reference.doFinal(uuid));
    assertThat(selectors.activeAccount(ACCOUNT)).hasSize(32);
    assertThat(HexFormat.of().formatHex(selectors.activeAccount(ACCOUNT)))
        .doesNotContain(ACCOUNT.toString().replace("-", ""));
  }

  @Test
  void lookupIncludesEveryRetainedVersionAndKeepsOldSelectorStable() {
    var old = new RoleIdentitySelectors(1, 1, Map.of(1, key(3)));
    var rotated = new RoleIdentitySelectors(2, 2, Map.of(1, key(3), 2, key(4)));

    assertThat(rotated.allAccounts(ACCOUNT)).containsOnlyKeys(1, 2);
    assertThat(rotated.allAccounts(ACCOUNT).get(1)).isEqualTo(old.activeAccount(ACCOUNT));
    assertThat(rotated.allAccounts(ACCOUNT).get(2)).isNotEqualTo(old.activeAccount(ACCOUNT));
    assertThat(rotated.keysetRevision()).isEqualTo(2);
    assertThat(rotated.activeVersion()).isEqualTo(2);
    assertThat(rotated.retainedVersions()).containsExactly(1, 2);
  }

  @Test
  void missingRequiredRetainedVersionNeverBecomesAnEmptyLookup() {
    var selectors = new RoleIdentitySelectors(2, 2, Map.of(2, key(4)));
    assertThatThrownBy(() -> selectors.requireRetainedVersions(java.util.List.of(1, 2)))
        .isInstanceOf(IllegalStateException.class).hasMessage("identity selector keyset unavailable");
  }

  @Test
  void invalidKeysetsAndNilIdentityAreRejectedWithoutEchoingInputs() {
    assertThatThrownBy(() -> new RoleIdentitySelectors(0, 1, Map.of(1, key(1))))
        .isInstanceOf(IllegalArgumentException.class);
    assertThatThrownBy(() -> new RoleIdentitySelectors(1, 2, Map.of(1, key(1))))
        .isInstanceOf(IllegalArgumentException.class);
    assertThatThrownBy(() -> new RoleIdentitySelectors(1, 1, Map.of(1, new byte[16])))
        .isInstanceOf(IllegalArgumentException.class);
    assertThatThrownBy(() -> new RoleIdentitySelectors(2, 2, Map.of(1, key(1), 2, key(1))))
        .isInstanceOf(IllegalArgumentException.class);
    var selectors = new RoleIdentitySelectors(1, 1, Map.of(1, key(1)));
    assertThatThrownBy(() -> selectors.activeAccount(new UUID(0, 0)))
        .isInstanceOf(IllegalArgumentException.class);
  }

  @Test
  void keyInputAndSelectorOutputsCannotMutateRetainedMaterial() {
    byte[] secret = key(5);
    var selectors = new RoleIdentitySelectors(1, 1, Map.of(1, secret));
    byte[] expected = selectors.activeAccount(ACCOUNT);
    secret[0] ^= 1;
    byte[] output = selectors.allAccounts(ACCOUNT).get(1);
    output[0] ^= 1;
    assertThat(selectors.activeAccount(ACCOUNT)).isEqualTo(expected);
  }
}
