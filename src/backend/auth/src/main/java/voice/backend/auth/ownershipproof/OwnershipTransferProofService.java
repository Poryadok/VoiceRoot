package voice.backend.auth.ownershipproof;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.security.SecureRandom;
import java.time.Clock;
import java.time.Instant;
import java.util.Base64;
import java.util.HexFormat;
import java.util.List;
import java.util.UUID;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.service.BackupCodeService;
import voice.backend.auth.service.TotpService;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;

/** Auth-owned factor verification and durable, resource-bound one-use grants. */
public final class OwnershipTransferProofService {
  private final OwnershipTransferProofStore store;
  private final BCryptPasswordHasher passwords;
  private final TotpService totp;
  private final BackupCodeService backup;
  private final SessionEpochFloorStore floors;
  private final Clock clock;
  private final SecureRandom random = new SecureRandom();

  public OwnershipTransferProofService(OwnershipTransferProofStore store, BCryptPasswordHasher passwords,
      TotpService totp, BackupCodeService backup, SessionEpochFloorStore floors, Clock clock) {
    this.store = store;
    this.passwords = passwords;
    this.totp = totp;
    this.backup = backup;
    this.floors = floors;
    this.clock = clock;
  }

  public IssuedProof issue(ProofBinding binding, String password, String totpCode, String backupCode) {
    return store.withAccount(binding.accountId(), session -> {
      ProofAccount account = session.account();
      requireCurrent(account, binding);
      if (password == null || password.isBlank() || !passwords.matches(password, account.passwordHash())) {
        throw new ProofDeniedException();
      }
      // Plaintext can only be returned once: an issue retry must not replace an existing grant.
      if (session.find(binding.operationId()).isPresent()) throw new ProofDeniedException();
      List<String> factors = List.of("password");
      if (account.totpEnabled()) {
        if (totpCode != null && !totpCode.isBlank() && account.totpSecret() != null
            && totp.verifyEncrypted(account.totpSecret(), totpCode)) {
          factors = List.of("password", "totp");
        } else if (backup.consume(binding.accountId(), backupCode)) {
          factors = List.of("password", "backup_code");
        } else {
          throw new ProofDeniedException();
        }
      }
      // Spending a backup factor itself changes security state and revokes older proofs.
      ProofAccount afterFactors = session.account();
      requireCurrent(afterFactors, binding);
      byte[] entropy = new byte[32];
      random.nextBytes(entropy);
      String proof = Base64.getUrlEncoder().withoutPadding().encodeToString(entropy);
      Instant expiry = clock.instant().truncatedTo(java.time.temporal.ChronoUnit.MICROS).plusSeconds(300);
      session.insert(new StoredProof(binding, hash(proof), afterFactors.securityRevision(), factors,
          expiry, UUID.randomUUID(), null));
      return new IssuedProof(proof, expiry);
    });
  }

  public Receipt consume(ProofBinding binding, String proof) {
    if (proof == null || !proof.matches("[A-Za-z0-9_-]{43}")) throw new ProofDeniedException();
    String suppliedHash = hash(proof);
    return store.withAccount(binding.accountId(), session -> {
      StoredProof stored = session.find(binding.operationId()).orElseThrow(ProofDeniedException::new);
      if (!stored.binding().equals(binding) || !MessageDigest.isEqual(
          stored.proofHash().getBytes(StandardCharsets.US_ASCII), suppliedHash.getBytes(StandardCharsets.US_ASCII))) {
        throw new ProofDeniedException();
      }
      // A retry reads an already committed decision; it never grants a second authorization.
      if (stored.consumedAt() != null) return receipt(stored);
      ProofAccount account = session.account();
      requireCurrent(account, binding);
      Instant now = clock.instant().truncatedTo(java.time.temporal.ChronoUnit.MICROS);
      if (stored.securityRevision() != account.securityRevision() || !now.isBefore(stored.expiresAt())) {
        throw new ProofDeniedException();
      }
      session.markConsumed(binding.operationId(), now);
      return receipt(stored.consumed(now));
    });
  }

  /** Recovers an existing grant for trusted Space; never consumes a proof or reauthorizes it. */
  public Receipt lookup(ProofBinding binding, String proofDigest) {
    if (proofDigest == null || !proofDigest.matches("[0-9a-f]{64}")) {
      throw new IllegalArgumentException("invalid proof digest");
    }
    StoredProof stored = store.findConsumed(binding.operationId()).orElseThrow(ProofDeniedException::new);
    if (stored.consumedAt() == null || !stored.binding().equals(binding) || !MessageDigest.isEqual(
        stored.proofHash().getBytes(StandardCharsets.US_ASCII), proofDigest.getBytes(StandardCharsets.US_ASCII))) {
      throw new ProofDeniedException();
    }
    return receipt(stored);
  }
  private void requireCurrent(ProofAccount account, ProofBinding binding) {
    if (account == null || !account.accountId().equals(binding.accountId()) || !account.active()
        || account.sessionEpoch() != binding.sessionEpoch() || account.securityRevision() <= 0) {
      throw new ProofDeniedException();
    }
    long floor = floors.requireFloor(binding.accountId());
    if (floor <= 0 || floor > binding.sessionEpoch()) throw new ProofDeniedException();
  }

  private static String hash(String proof) {
    try {
      return HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256")
          .digest(proof.getBytes(StandardCharsets.UTF_8)));
    } catch (NoSuchAlgorithmException impossible) {
      throw new IllegalStateException("SHA-256 unavailable", impossible);
    }
  }

  private static Receipt receipt(StoredProof proof) {
    return new Receipt(proof.receiptId(), proof.binding(), proof.consumedAt(), proof.factors());
  }

  public record IssuedProof(String proof, Instant expiresAt) {
    @Override public String toString() { return "IssuedProof[redacted]"; }
  }
  public record Receipt(UUID receiptId, ProofBinding binding, Instant consumedAt, List<String> factors) {
    public Receipt { factors = List.copyOf(factors); }
  }
}
