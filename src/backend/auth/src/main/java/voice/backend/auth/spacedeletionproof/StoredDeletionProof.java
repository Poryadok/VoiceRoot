package voice.backend.auth.spacedeletionproof;

import app.voice.auth.v1.ProofPurpose;
import app.voice.auth.v1.SpaceDeletionProofReceipt;
import app.voice.auth.v1.VerifiedFactor;
import com.google.protobuf.ByteString;
import java.time.Instant;
import java.util.List;
import java.util.UUID;

/** Hash-only proof row plus immutable deterministic binding and receipt evidence. */
public record StoredDeletionProof(
    StoredBinding binding,
    long securityRevision,
    Instant expiresAt,
    UUID receiptId,
    Instant consumedAt,
    byte[] bindingBytes,
    ByteString bindingSha256,
    SpaceDeletionProofReceipt receipt,
    byte[] receiptBytes,
    byte[] receiptSha256,
    Instant acknowledgedAt,
    byte[] receiptLookupHmac,
    Integer receiptHmacKeyVersion) {
  public StoredDeletionProof {
    if (binding == null || securityRevision <= 0 || expiresAt == null || receiptId == null
        || bindingSha256 == null || bindingSha256.size() != 32) {
      throw new IllegalArgumentException("invalid stored deletion proof");
    }
    bindingBytes = copy(bindingBytes);
    receiptBytes = copy(receiptBytes);
    receiptSha256 = copy(receiptSha256);
    receiptLookupHmac = copy(receiptLookupHmac);
  }

  @Override public byte[] bindingBytes() { return copy(bindingBytes); }
  @Override public byte[] receiptBytes() { return copy(receiptBytes); }
  @Override public byte[] receiptSha256() { return copy(receiptSha256); }
  @Override public byte[] receiptLookupHmac() { return copy(receiptLookupHmac); }

  public byte[] confirmationNameSha256() { return binding.confirmationNameSha256(); }
  public byte[] proofDigestSha256() { return binding.proofDigestSha256(); }
  public List<VerifiedFactor> verifiedFactors() { return binding.verifiedFactors(); }

  public StoredDeletionProof consumed(
      Instant at, SpaceDeletionProofReceipt acceptedReceipt, byte[] bytes, byte[] digest) {
    return new StoredDeletionProof(binding, securityRevision, expiresAt, receiptId, at,
        bindingBytes, bindingSha256, acceptedReceipt, bytes, digest, acknowledgedAt,
        receiptLookupHmac, receiptHmacKeyVersion);
  }

  public StoredDeletionProof acknowledged(Instant at) {
    return new StoredDeletionProof(binding, securityRevision, expiresAt, receiptId, consumedAt,
        bindingBytes, bindingSha256, receipt, receiptBytes, receiptSha256, at,
        receiptLookupHmac, receiptHmacKeyVersion);
  }

  private static byte[] copy(byte[] value) { return value == null ? null : value.clone(); }

  /** Identity may be null only after account erasure; the durable non-identity bindings remain. */
  public record StoredBinding(
      UUID accountId,
      UUID profileId,
      long sessionEpoch,
      UUID spaceId,
      UUID operationId,
      byte[] confirmationNameSha256,
      byte[] proofDigestSha256,
      List<VerifiedFactor> verifiedFactors,
      ProofPurpose purpose) {
    public StoredBinding {
      if (sessionEpoch <= 0 || spaceId == null || operationId == null
          || confirmationNameSha256 == null || confirmationNameSha256.length != 32
          || proofDigestSha256 == null || proofDigestSha256.length != 32
          || verifiedFactors == null || verifiedFactors.isEmpty()
          || purpose != ProofPurpose.PROOF_PURPOSE_SPACE_DELETE
          || (accountId == null) != (profileId == null)) {
        throw new IllegalArgumentException("invalid stored deletion binding");
      }
      confirmationNameSha256 = confirmationNameSha256.clone();
      proofDigestSha256 = proofDigestSha256.clone();
      verifiedFactors = List.copyOf(verifiedFactors);
    }

    @Override public byte[] confirmationNameSha256() { return confirmationNameSha256.clone(); }
    @Override public byte[] proofDigestSha256() { return proofDigestSha256.clone(); }
  }
}
