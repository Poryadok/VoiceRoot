package voice.backend.auth.spacedeletionproof;

import static org.assertj.core.api.Assertions.assertThat;

import app.voice.auth.v1.ProofPurpose;
import app.voice.auth.v1.SpaceDeletionProofBinding;
import app.voice.auth.v1.VerifiedFactor;
import com.google.protobuf.ByteString;
import com.google.protobuf.CodedOutputStream;
import java.time.Duration;
import java.time.Instant;
import java.util.HexFormat;
import org.junit.jupiter.api.Test;

class SpaceDeletionReceiptErasurePolicyTest {
  private static final Instant ACTIVATED_AT = Instant.parse("2026-01-01T00:00:00Z");

  @Test
  void preAcknowledgementErasureIndexMatchesValidCanonicalBindingHmacVector()
      throws Exception {
    byte[] key = new byte[32];
    byte[] nameDigest = new byte[32];
    byte[] proofDigest = new byte[32];
    for (int index = 0; index < 32; index++) {
      key[index] = (byte) index;
      nameDigest[index] = (byte) index;
      proofDigest[index] = (byte) (index + 32);
    }
    var binding = SpaceDeletionProofBinding.newBuilder()
        .setProtocolVersion(1)
        .setAccountId("00000000-0000-4000-8000-000000000001")
        .setProfileId("00000000-0000-4000-8000-000000000002")
        .setSessionEpoch(7)
        .setSpaceId("00000000-0000-4000-8000-000000000003")
        .setOperationId("00000000-0000-4000-8000-000000000004")
        .setConfirmationNameSha256(ByteString.copyFrom(nameDigest))
        .setProofDigestSha256(ByteString.copyFrom(proofDigest))
        .addVerifiedFactors(VerifiedFactor.VERIFIED_FACTOR_PASSWORD)
        .addVerifiedFactors(VerifiedFactor.VERIFIED_FACTOR_TOTP)
        .setPurpose(ProofPurpose.PROOF_PURPOSE_SPACE_DELETE)
        .build();
    byte[] deterministicBinding = deterministic(binding);
    assertThat(HexFormat.of().formatHex(deterministicBinding)).isEqualTo(
        "0801122430303030303030302d303030302d343030302d383030302d303030303030303030303031"
            + "1a2430303030303030302d303030302d343030302d383030302d3030303030303030303030322007"
            + "2a2430303030303030302d303030302d343030302d383030302d303030303030303030303033"
            + "322430303030303030302d303030302d343030302d383030302d303030303030303030303034"
            + "3a20000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
            + "4220202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f"
            + "4a0201025001");

    assertThat(HexFormat.of().formatHex(
            ReceiptErasureIndex.hmacSha256(key, deterministicBinding)))
        .isEqualTo("61f3d6dfaabe3f99502d0d94db34c40b822d33d5b4b359b6bd6103c5b502519b");
  }

  @Test
  void rotationIsDueAtP90dAndNeverExtendsTheP30dBackupDependencyWindow() {
    var policy = new ReceiptErasureKeyPolicy(Duration.ofDays(90), Duration.ofDays(30));

    assertThat(policy.rotationDue(ACTIVATED_AT, ACTIVATED_AT.plus(Duration.ofDays(90)).minusNanos(1)))
        .isFalse();
    assertThat(policy.rotationDue(ACTIVATED_AT, ACTIVATED_AT.plus(Duration.ofDays(90))))
        .isTrue();
    assertThat(policy.rotationDue(ACTIVATED_AT, ACTIVATED_AT.plus(Duration.ofDays(180))))
        .isTrue();
  }

  @Test
  void destructionRequiresNoRetainedRowsAndNoRestorableBackupNeedsTheVersion() {
    var policy = new ReceiptErasureKeyPolicy(Duration.ofDays(90), Duration.ofDays(30));
    Instant lastBackup = Instant.parse("2026-04-01T00:00:00Z");

    assertThat(policy.canDestroy(7, ACTIVATED_AT.plus(Duration.ofDays(200)), 1, lastBackup))
        .isFalse();
    assertThat(policy.canDestroy(7, lastBackup.plus(Duration.ofDays(30)), 0, lastBackup))
        .isFalse();
    assertThat(policy.canDestroy(7, lastBackup.plus(Duration.ofDays(30)).plusNanos(1), 0, lastBackup))
        .isTrue();
  }

  private static byte[] deterministic(SpaceDeletionProofBinding binding) throws Exception {
    byte[] bytes = new byte[binding.getSerializedSize()];
    CodedOutputStream output = CodedOutputStream.newInstance(bytes);
    output.useDeterministicSerialization();
    binding.writeTo(output);
    output.checkNoSpaceLeft();
    return bytes;
  }
}
