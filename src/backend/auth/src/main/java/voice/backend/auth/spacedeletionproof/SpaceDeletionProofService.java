package voice.backend.auth.spacedeletionproof;

import app.voice.auth.v1.ProofPurpose;
import app.voice.auth.v1.SpaceDeletionProofBinding;
import app.voice.auth.v1.SpaceDeletionProofReceipt;
import app.voice.auth.v1.VerifiedFactor;
import com.google.protobuf.ByteString;
import com.google.protobuf.CodedOutputStream;
import com.google.protobuf.Message;
import com.google.protobuf.Timestamp;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.SecureRandom;
import java.time.Clock;
import java.time.Instant;
import java.time.temporal.ChronoUnit;
import java.util.ArrayList;
import java.util.Base64;
import java.util.List;
import java.util.UUID;
import voice.backend.auth.ownershipproof.ProofAccount;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.service.BackupCodeService;
import voice.backend.auth.service.TotpService;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;

/** Auth-owned factor verification and durable one-use Space deletion authorization. */
public final class SpaceDeletionProofService {
  private final SpaceDeletionProofStore store;
  private final BCryptPasswordHasher passwords;
  private final TotpService totp;
  private final BackupCodeService backup;
  private final SessionEpochFloorStore floors;
  private final Clock clock;
  private final SecureRandom random = new SecureRandom();

  public SpaceDeletionProofService(
      SpaceDeletionProofStore store,
      BCryptPasswordHasher passwords,
      TotpService totp,
      BackupCodeService backup,
      SessionEpochFloorStore floors,
      Clock clock) {
    this.store = java.util.Objects.requireNonNull(store, "store");
    this.passwords = java.util.Objects.requireNonNull(passwords, "passwords");
    this.totp = java.util.Objects.requireNonNull(totp, "totp");
    this.backup = java.util.Objects.requireNonNull(backup, "backup");
    this.floors = java.util.Objects.requireNonNull(floors, "floors");
    this.clock = java.util.Objects.requireNonNull(clock, "clock");
  }

  public IssuedProof issue(
      DeletionProofBinding requested,
      String confirmationName,
      String password,
      String totpCode,
      String backupCode) {
    if (confirmationName == null) throw new DeletionProofDeniedException();
    return store.withAccount(requested.accountId(), session -> {
      ProofAccount account = session.account();
      requireCurrent(account, requested);
      if (password == null || password.isBlank()
          || !passwords.matches(password, account.passwordHash())) {
        throw new DeletionProofDeniedException();
      }
      if (session.find(requested.operationId()).isPresent()) {
        throw new DeletionProofDeniedException();
      }

      List<VerifiedFactor> factors = new ArrayList<>();
      factors.add(VerifiedFactor.VERIFIED_FACTOR_PASSWORD);
      if (account.totpEnabled()) {
        boolean hasTotp = totpCode != null && !totpCode.isBlank();
        boolean hasBackup = backupCode != null && !backupCode.isBlank();
        if (hasTotp == hasBackup) throw new DeletionProofDeniedException();
        if (hasTotp) {
          if (account.totpSecret() == null
              || !totp.verifyEncrypted(account.totpSecret(), totpCode)) {
            throw new DeletionProofDeniedException();
          }
          factors.add(VerifiedFactor.VERIFIED_FACTOR_TOTP);
        } else {
          if (!backup.consume(requested.accountId(), backupCode)) {
            throw new DeletionProofDeniedException();
          }
          factors.add(VerifiedFactor.VERIFIED_FACTOR_BACKUP_CODE);
        }
      }

      ProofAccount afterFactors = session.account();
      requireCurrent(afterFactors, requested);
      byte[] entropy = new byte[32];
      random.nextBytes(entropy);
      String proof = Base64.getUrlEncoder().withoutPadding().encodeToString(entropy);
      byte[] nameDigest = sha256(confirmationName.getBytes(StandardCharsets.UTF_8));
      byte[] proofDigest = sha256(proof.getBytes(StandardCharsets.US_ASCII));
      var storedBinding = new StoredDeletionProof.StoredBinding(
          requested.accountId(), requested.profileId(), requested.sessionEpoch(), requested.spaceId(),
          requested.operationId(), nameDigest, proofDigest, factors,
          ProofPurpose.PROOF_PURPOSE_SPACE_DELETE);
      SpaceDeletionProofBinding canonical = canonicalBinding(storedBinding);
      byte[] bindingBytes = deterministic(canonical);
      byte[] bindingDigest = domainHash(canonical, bindingBytes);
      Instant expiresAt = now().plusSeconds(300);
      session.insert(new StoredDeletionProof(
          storedBinding, afterFactors.securityRevision(), expiresAt, UUID.randomUUID(), null,
          bindingBytes, ByteString.copyFrom(bindingDigest), null, null, null, null, null, null));
      return new IssuedProof(proof, expiresAt);
    });
  }

  public SpaceDeletionProofReceipt consume(
      DeletionProofBinding requested, String confirmationName, String proof) {
    if (confirmationName == null || proof == null || !proof.matches("[A-Za-z0-9_-]{43}")) {
      throw new DeletionProofDeniedException();
    }
    byte[] nameDigest = sha256(confirmationName.getBytes(StandardCharsets.UTF_8));
    byte[] proofDigest = sha256(proof.getBytes(StandardCharsets.US_ASCII));
    return store.withAccount(requested.accountId(), session -> {
      StoredDeletionProof stored =
          session.find(requested.operationId()).orElseThrow(DeletionProofDeniedException::new);
      if (!matches(stored.binding(), requested, nameDigest, proofDigest)) {
        throw new DeletionProofDeniedException();
      }
      if (stored.consumedAt() != null) return requireReceipt(stored);

      ProofAccount account = session.account();
      requireCurrent(account, requested);
      Instant at = now();
      if (stored.securityRevision() != account.securityRevision()
          || !at.isBefore(stored.expiresAt())) {
        throw new DeletionProofDeniedException();
      }
      SpaceDeletionProofReceipt receipt = SpaceDeletionProofReceipt.newBuilder()
          .setProtocolVersion(1)
          .setReceiptId(stored.receiptId().toString())
          .setOperationId(requested.operationId().toString())
          .setBindingSha256(stored.bindingSha256())
          .setConfirmationNameSha256(ByteString.copyFrom(nameDigest))
          .setConsumedAt(timestamp(at))
          .addAllVerifiedFactors(stored.verifiedFactors())
          .build();
      byte[] receiptBytes = deterministic(receipt);
      byte[] receiptDigest = domainHash(receipt, receiptBytes);
      session.save(stored.consumed(at, receipt, receiptBytes, receiptDigest));
      return receipt;
    });
  }

  /** Returns only an already committed receipt; it never consumes or reauthorizes a proof. */
  public SpaceDeletionProofReceipt lookup(DeletionProofLookup lookup) {
    StoredDeletionProof stored =
        store.findConsumed(lookup).orElseThrow(DeletionProofDeniedException::new);
    var binding = stored.binding();
    boolean common = binding.sessionEpoch() == lookup.sessionEpoch()
        && binding.spaceId().equals(lookup.spaceId())
        && binding.operationId().equals(lookup.operationId())
        && MessageDigest.isEqual(binding.confirmationNameSha256(), lookup.confirmationNameSha256())
        && MessageDigest.isEqual(binding.proofDigestSha256(), lookup.proofDigestSha256());
    boolean identity = binding.accountId() == null
        || (binding.accountId().equals(lookup.accountId())
            && binding.profileId().equals(lookup.profileId()));
    if (!common || !identity || stored.consumedAt() == null) {
      throw new DeletionProofDeniedException();
    }
    return requireReceipt(stored);
  }

  public Instant acknowledge(DeletionProofAcknowledgement acknowledgement) {
    return store.acknowledge(acknowledgement, now());
  }

  private void requireCurrent(ProofAccount account, DeletionProofBinding requested) {
    if (account == null || !requested.accountId().equals(account.accountId()) || !account.active()
        || account.sessionEpoch() != requested.sessionEpoch() || account.securityRevision() <= 0) {
      throw new DeletionProofDeniedException();
    }
    long floor = floors.requireFloor(requested.accountId());
    if (floor <= 0 || floor > requested.sessionEpoch()) throw new DeletionProofDeniedException();
  }

  private static boolean matches(
      StoredDeletionProof.StoredBinding stored,
      DeletionProofBinding requested,
      byte[] nameDigest,
      byte[] proofDigest) {
    return stored.accountId() != null
        && stored.accountId().equals(requested.accountId())
        && stored.profileId().equals(requested.profileId())
        && stored.sessionEpoch() == requested.sessionEpoch()
        && stored.spaceId().equals(requested.spaceId())
        && stored.operationId().equals(requested.operationId())
        && stored.purpose() == ProofPurpose.PROOF_PURPOSE_SPACE_DELETE
        && MessageDigest.isEqual(stored.confirmationNameSha256(), nameDigest)
        && MessageDigest.isEqual(stored.proofDigestSha256(), proofDigest);
  }

  static SpaceDeletionProofBinding canonicalBinding(StoredDeletionProof.StoredBinding binding) {
    return SpaceDeletionProofBinding.newBuilder()
        .setProtocolVersion(1)
        .setAccountId(binding.accountId().toString())
        .setProfileId(binding.profileId().toString())
        .setSessionEpoch(binding.sessionEpoch())
        .setSpaceId(binding.spaceId().toString())
        .setOperationId(binding.operationId().toString())
        .setConfirmationNameSha256(ByteString.copyFrom(binding.confirmationNameSha256()))
        .setProofDigestSha256(ByteString.copyFrom(binding.proofDigestSha256()))
        .addAllVerifiedFactors(binding.verifiedFactors())
        .setPurpose(binding.purpose())
        .build();
  }

  static byte[] deterministic(Message message) {
    try {
      byte[] bytes = new byte[message.getSerializedSize()];
      CodedOutputStream output = CodedOutputStream.newInstance(bytes);
      output.useDeterministicSerialization();
      message.writeTo(output);
      output.checkNoSpaceLeft();
      return bytes;
    } catch (java.io.IOException impossible) {
      throw new IllegalStateException("deterministic protobuf serialization failed", impossible);
    }
  }

  static byte[] domainHash(Message message, byte[] deterministicBytes) {
    byte[] name = message.getDescriptorForType().getFullName().getBytes(StandardCharsets.UTF_8);
    MessageDigest digest = sha256Digest();
    digest.update(name);
    digest.update((byte) 0);
    return digest.digest(deterministicBytes);
  }

  private static byte[] sha256(byte[] input) { return sha256Digest().digest(input); }

  private static MessageDigest sha256Digest() {
    try {
      return MessageDigest.getInstance("SHA-256");
    } catch (java.security.NoSuchAlgorithmException impossible) {
      throw new IllegalStateException("SHA-256 unavailable", impossible);
    }
  }

  private static SpaceDeletionProofReceipt requireReceipt(StoredDeletionProof stored) {
    if (stored.receipt() == null || stored.receiptBytes() == null || stored.receiptSha256() == null) {
      throw new DeletionProofDeniedException();
    }
    return stored.receipt();
  }

  private Instant now() { return clock.instant().truncatedTo(ChronoUnit.MICROS); }

  private static Timestamp timestamp(Instant instant) {
    return Timestamp.newBuilder().setSeconds(instant.getEpochSecond()).setNanos(instant.getNano()).build();
  }

  public record IssuedProof(String proof, Instant expiresAt) {
    @Override public String toString() { return "IssuedProof[redacted]"; }
  }
}
