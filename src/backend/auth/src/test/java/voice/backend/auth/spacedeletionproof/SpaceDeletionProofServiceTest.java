package voice.backend.auth.spacedeletionproof;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

import app.voice.auth.v1.ProofPurpose;
import app.voice.auth.v1.SpaceDeletionProofBinding;
import app.voice.auth.v1.VerifiedFactor;
import com.google.protobuf.ByteString;
import com.google.protobuf.CodedOutputStream;
import com.google.protobuf.Message;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.time.Clock;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.Arrays;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;
import java.util.function.Function;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import voice.backend.auth.ownershipproof.ProofAccount;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.service.BackupCodeService;
import voice.backend.auth.service.TotpService;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;

class SpaceDeletionProofServiceTest {
  private final UUID accountId = UUID.randomUUID();
  private final UUID profileId = UUID.randomUUID();
  private final UUID spaceId = UUID.randomUUID();
  private final Instant now = Instant.parse("2026-09-12T08:00:00.123456Z");
  private final BCryptPasswordHasher passwords = mock(BCryptPasswordHasher.class);
  private final TotpService totp = mock(TotpService.class);
  private final BackupCodeService backup = mock(BackupCodeService.class);
  private final SessionEpochFloorStore floors = mock(SessionEpochFloorStore.class);
  private final MemoryStore store = new MemoryStore();
  private DeletionProofBinding binding;
  private SpaceDeletionProofService service;

  @BeforeEach
  void setUp() {
    binding = new DeletionProofBinding(accountId, profileId, 7, spaceId, UUID.randomUUID());
    store.account = new ProofAccount(accountId, "password-hash", new byte[] {1}, false, 7, 1, true);
    when(passwords.matches("correct-password", "password-hash")).thenReturn(true);
    when(floors.requireFloor(accountId)).thenReturn(7L);
    service = at(now);
  }

  private SpaceDeletionProofService at(Instant instant) {
    return new SpaceDeletionProofService(
        store, passwords, totp, backup, floors, Clock.fixed(instant, ZoneOffset.UTC));
  }

  @Test
  void issueAlwaysRequiresPasswordAndStoresOnlyExactUtf8NameAndProofDigests() throws Exception {
    assertThatThrownBy(() -> service.issue(binding, "Космос 🚀", "wrong", "", ""))
        .isInstanceOf(DeletionProofDeniedException.class);
    assertThat(store.rows).isEmpty();

    var issued = service.issue(binding, "Космос 🚀", "correct-password", "", "");
    assertThat(issued.expiresAt()).isEqualTo(now.plusSeconds(300));
    assertThat(issued.proof()).matches("[A-Za-z0-9_-]{43}");
    assertThat(issued.toString()).containsIgnoringCase("redacted").doesNotContain(issued.proof());

    StoredDeletionProof row = store.rows.get(binding.operationId());
    assertThat(row.confirmationNameSha256()).containsExactly(sha256("Космос 🚀"));
    assertThat(row.proofDigestSha256()).containsExactly(sha256(issued.proof()));
    assertThat(row.verifiedFactors())
        .containsExactly(VerifiedFactor.VERIFIED_FACTOR_PASSWORD);
    assertThat(row.toString()).doesNotContain("Космос 🚀", issued.proof(), "correct-password");
    verifyNoInteractions(totp, backup);
  }

  @Test
  void exactUtf8NameIsNotUnicodeNormalizedTrimmedOrCaseFolded() {
    String accepted = "Café 🚀";
    var issued = service.issue(binding, accepted, "correct-password", "", "");
    for (String changed : List.of("Cafe\u0301 🚀", "café 🚀", "Café 🚀 ", " Café 🚀")) {
      assertThatThrownBy(() -> service.consume(binding, changed, issued.proof()))
          .isInstanceOf(DeletionProofDeniedException.class);
    }
    assertThat(service.consume(binding, accepted, issued.proof()).getOperationId())
        .isEqualTo(binding.operationId().toString());
  }

  @Test
  void enabledTwoFactorRequiresExactlyOneValidTotpOrUnusedBackupCode() {
    store.account = new ProofAccount(accountId, "password-hash", new byte[] {1}, true, 7, 1, true);
    assertThatThrownBy(() -> service.issue(binding, "Space", "correct-password", "", ""))
        .isInstanceOf(DeletionProofDeniedException.class);
    assertThatThrownBy(
            () -> service.issue(binding, "Space", "correct-password", "123456", "BACKUP"))
        .isInstanceOf(DeletionProofDeniedException.class);

    when(totp.verifyEncrypted(any(), eq("123456"))).thenReturn(true);
    var totpBinding = new DeletionProofBinding(accountId, profileId, 7, spaceId, UUID.randomUUID());
    service.issue(totpBinding, "Space", "correct-password", "123456", "");
    assertThat(store.rows.get(totpBinding.operationId()).verifiedFactors())
        .containsExactly(
            VerifiedFactor.VERIFIED_FACTOR_PASSWORD, VerifiedFactor.VERIFIED_FACTOR_TOTP);

    when(backup.consume(accountId, "BACKUP")).thenReturn(true).thenReturn(false);
    var backupBinding = new DeletionProofBinding(accountId, profileId, 7, spaceId, UUID.randomUUID());
    service.issue(backupBinding, "Space", "correct-password", "", "BACKUP");
    assertThat(store.rows.get(backupBinding.operationId()).verifiedFactors())
        .containsExactly(
            VerifiedFactor.VERIFIED_FACTOR_PASSWORD,
            VerifiedFactor.VERIFIED_FACTOR_BACKUP_CODE);
    var reusedBinding = new DeletionProofBinding(accountId, profileId, 7, spaceId, UUID.randomUUID());
    assertThatThrownBy(
            () -> service.issue(reusedBinding, "Space", "correct-password", "", "BACKUP"))
        .isInstanceOf(DeletionProofDeniedException.class);
  }

  @Test
  void fiveMinuteBoundaryIsExpiredAndSecurityRevisionRevokesOnlyUnconsumedProof() {
    var issued = service.issue(binding, "Space", "correct-password", "", "");
    assertThat(at(now.plusSeconds(299)).consume(binding, "Space", issued.proof())).isNotNull();

    var expiring = new DeletionProofBinding(accountId, profileId, 7, spaceId, UUID.randomUUID());
    var expiringProof = service.issue(expiring, "Space", "correct-password", "", "");
    assertThatThrownBy(
            () -> at(now.plusSeconds(300)).consume(expiring, "Space", expiringProof.proof()))
        .isInstanceOf(DeletionProofDeniedException.class);

    var revoked = new DeletionProofBinding(accountId, profileId, 7, spaceId, UUID.randomUUID());
    var revokedProof = service.issue(revoked, "Space", "correct-password", "", "");
    store.account = new ProofAccount(accountId, "password-hash", new byte[] {1}, false, 7, 3, true);
    assertThatThrownBy(() -> service.consume(revoked, "Space", revokedProof.proof()))
        .isInstanceOf(DeletionProofDeniedException.class);
    assertThat(store.rows.get(revoked.operationId()).receiptBytes()).isNull();
  }

  @Test
  void consumeCreatesOneDeterministicPurposeBoundReceiptAndExactReplayReturnsIt() throws Exception {
    var issued = service.issue(binding, "Space", "correct-password", "", "");
    var first = service.consume(binding, "Space", issued.proof());
    var replay = at(now.plusSeconds(86400)).consume(binding, "Space", issued.proof());

    assertThat(deterministic(first)).containsExactly(deterministic(replay));
    StoredDeletionProof row = store.rows.get(binding.operationId());
    assertThat(row.receiptBytes()).containsExactly(deterministic(first));
    assertThat(row.receiptSha256()).containsExactly(domainHash(first));
    assertThat(row.binding().purpose()).isEqualTo(ProofPurpose.PROOF_PURPOSE_SPACE_DELETE);
    SpaceDeletionProofBinding canonicalBinding = SpaceDeletionProofBinding.newBuilder()
        .setProtocolVersion(1)
        .setAccountId(accountId.toString())
        .setProfileId(profileId.toString())
        .setSessionEpoch(7)
        .setSpaceId(spaceId.toString())
        .setOperationId(binding.operationId().toString())
        .setConfirmationNameSha256(ByteString.copyFrom(sha256("Space")))
        .setProofDigestSha256(ByteString.copyFrom(sha256(issued.proof())))
        .addVerifiedFactors(VerifiedFactor.VERIFIED_FACTOR_PASSWORD)
        .setPurpose(ProofPurpose.PROOF_PURPOSE_SPACE_DELETE)
        .build();
    assertThat(first.getBindingSha256())
        .isEqualTo(ByteString.copyFrom(domainHash(canonicalBinding)))
        .isEqualTo(row.bindingSha256());
    assertThat(first.getProtocolVersion()).isEqualTo(1);
    assertThat(first.getReceiptId()).isEqualTo(row.receipt().getReceiptId());
    assertThat(first.getOperationId()).isEqualTo(binding.operationId().toString());
    assertThat(first.getConsumedAt()).isEqualTo(com.google.protobuf.Timestamp.newBuilder()
        .setSeconds(now.getEpochSecond()).setNanos(now.getNano()).build());
    assertThat(first.getConfirmationNameSha256())
        .isEqualTo(ByteString.copyFrom(sha256("Space")));
    assertThat(first.getVerifiedFactorsList())
        .containsExactly(VerifiedFactor.VERIFIED_FACTOR_PASSWORD);
  }

  @Test
  void everyChangedBindingProofOrNameDeniesWithoutChangingAcceptedReceipt() throws Exception {
    var issued = service.issue(binding, "Space", "correct-password", "", "");
    var accepted = service.consume(binding, "Space", issued.proof());
    StoredDeletionProof acceptedRow = store.rows.get(binding.operationId());
    var changes = List.of(
        new DeletionProofBinding(UUID.randomUUID(), profileId, 7, spaceId, binding.operationId()),
        new DeletionProofBinding(accountId, UUID.randomUUID(), 7, spaceId, binding.operationId()),
        new DeletionProofBinding(accountId, profileId, 8, spaceId, binding.operationId()),
        new DeletionProofBinding(accountId, profileId, 7, UUID.randomUUID(), binding.operationId()),
        new DeletionProofBinding(accountId, profileId, 7, spaceId, UUID.randomUUID()));
    for (DeletionProofBinding changed : changes) {
      assertConsumeDeniedWithoutMutation(changed, "Space", issued.proof(), acceptedRow);
    }
    assertConsumeDeniedWithoutMutation(binding, "Changed", issued.proof(), acceptedRow);
    String rejectedProof = alternateProof(issued.proof());
    assertThat(rejectedProof).hasSize(43).matches("[A-Za-z0-9_-]{43}");
    assertConsumeDeniedWithoutMutation(binding, "Space", rejectedProof, acceptedRow);
    assertThat(deterministic(acceptedRow.receipt())).containsExactly(deterministic(accepted));
  }

  @Test
  void lookupRecoversOnlyExactCommittedReceiptWithoutCurrentAccountChecksOrMutation()
      throws Exception {
    var issued = service.issue(binding, "Space", "correct-password", "", "");
    var lookup = lookup(binding, "Space", issued.proof());
    StoredDeletionProof unconsumed = store.rows.get(binding.operationId());
    assertLookupDeniedWithoutMutation(lookup, unconsumed);
    var receipt = service.consume(binding, "Space", issued.proof());
    StoredDeletionProof before = store.rows.get(binding.operationId());

    store.account = null;
    when(floors.requireFloor(accountId)).thenThrow(new AssertionError("historical lookup used floor"));
    assertThat(deterministic(service.lookup(lookup))).containsExactly(deterministic(receipt));
    assertThat(store.rows.get(binding.operationId())).isSameAs(before);

    List<DeletionProofLookup> oneFieldChanges = List.of(
        new DeletionProofLookup(UUID.randomUUID(), profileId, 7, spaceId,
            binding.operationId(), sha256("Space"), sha256(issued.proof())),
        new DeletionProofLookup(accountId, UUID.randomUUID(), 7, spaceId,
            binding.operationId(), sha256("Space"), sha256(issued.proof())),
        new DeletionProofLookup(accountId, profileId, 8, spaceId,
            binding.operationId(), sha256("Space"), sha256(issued.proof())),
        new DeletionProofLookup(accountId, profileId, 7, UUID.randomUUID(),
            binding.operationId(), sha256("Space"), sha256(issued.proof())),
        new DeletionProofLookup(accountId, profileId, 7, spaceId,
            UUID.randomUUID(), sha256("Space"), sha256(issued.proof())),
        new DeletionProofLookup(accountId, profileId, 7, spaceId,
            binding.operationId(), sha256("Other"), sha256(issued.proof())),
        new DeletionProofLookup(accountId, profileId, 7, spaceId,
            binding.operationId(), sha256("Space"), sha256(alternateProof(issued.proof()))));
    oneFieldChanges.forEach(changed -> assertLookupDeniedWithoutMutation(changed, before));
  }

  @Test
  void acknowledgementIsExactIdempotentAndCannotReplaceAcceptedEvidence() throws Exception {
    var issued = service.issue(binding, "Space", "correct-password", "", "");
    var receipt = service.consume(binding, "Space", issued.proof());
    byte[] receiptHash = domainHash(receipt);
    var acknowledgement = new DeletionProofAcknowledgement(
        UUID.fromString(receipt.getReceiptId()), spaceId, binding.operationId(), receiptHash);
    StoredDeletionProof unacknowledged = store.rows.get(binding.operationId());
    List<DeletionProofAcknowledgement> oneFieldChanges = List.of(
        new DeletionProofAcknowledgement(UUID.randomUUID(), spaceId, binding.operationId(), receiptHash),
        new DeletionProofAcknowledgement(acknowledgement.receiptId(), UUID.randomUUID(),
            binding.operationId(), receiptHash),
        new DeletionProofAcknowledgement(acknowledgement.receiptId(), spaceId,
            UUID.randomUUID(), receiptHash),
        new DeletionProofAcknowledgement(acknowledgement.receiptId(), spaceId,
            binding.operationId(), sha256("wrong receipt")));
    oneFieldChanges.forEach(changed ->
        assertAcknowledgementDeniedWithoutMutation(changed, unacknowledged));

    Instant first = service.acknowledge(acknowledgement);
    assertThat(first).isEqualTo(now);
    assertThat(service.acknowledge(acknowledgement)).isEqualTo(first);
    StoredDeletionProof acknowledged = store.rows.get(binding.operationId());
    assertThat(acknowledged.acknowledgedAt()).isEqualTo(first);
    oneFieldChanges.forEach(changed ->
        assertAcknowledgementDeniedWithoutMutation(changed, acknowledged));
  }

  private void assertConsumeDeniedWithoutMutation(
      DeletionProofBinding changed,
      String confirmationName,
      String proof,
      StoredDeletionProof expected) {
    Map<UUID, StoredDeletionProof> before = Map.copyOf(store.rows);
    assertThatThrownBy(() -> service.consume(changed, confirmationName, proof))
        .isInstanceOf(DeletionProofDeniedException.class);
    assertThat(store.rows).containsExactlyInAnyOrderEntriesOf(before);
    assertThat(store.rows.get(binding.operationId())).isSameAs(expected);
  }

  private void assertLookupDeniedWithoutMutation(
      DeletionProofLookup changed, StoredDeletionProof expected) {
    Map<UUID, StoredDeletionProof> before = Map.copyOf(store.rows);
    assertThatThrownBy(() -> service.lookup(changed))
        .isInstanceOf(DeletionProofDeniedException.class);
    assertThat(store.rows).containsExactlyInAnyOrderEntriesOf(before);
    assertThat(store.rows.get(binding.operationId())).isSameAs(expected);
  }

  private void assertAcknowledgementDeniedWithoutMutation(
      DeletionProofAcknowledgement changed, StoredDeletionProof expected) {
    Map<UUID, StoredDeletionProof> before = Map.copyOf(store.rows);
    assertThatThrownBy(() -> service.acknowledge(changed))
        .isInstanceOf(DeletionProofDeniedException.class);
    assertThat(store.rows).containsExactlyInAnyOrderEntriesOf(before);
    assertThat(store.rows.get(binding.operationId())).isSameAs(expected);
  }

  private static String alternateProof(String issuedProof) {
    String candidate = "A".repeat(43);
    return candidate.equals(issuedProof) ? "B".repeat(43) : candidate;
  }

  private static DeletionProofLookup lookup(
      DeletionProofBinding binding, String name, String proof) throws Exception {
    return new DeletionProofLookup(
        binding.accountId(), binding.profileId(), binding.sessionEpoch(), binding.spaceId(),
        binding.operationId(), sha256(name), sha256(proof));
  }

  private static byte[] sha256(String value) throws Exception {
    return MessageDigest.getInstance("SHA-256").digest(value.getBytes(StandardCharsets.UTF_8));
  }

  private static byte[] deterministic(Message message) throws Exception {
    byte[] bytes = new byte[message.getSerializedSize()];
    CodedOutputStream output = CodedOutputStream.newInstance(bytes);
    output.useDeterministicSerialization();
    message.writeTo(output);
    output.checkNoSpaceLeft();
    return bytes;
  }

  private static byte[] domainHash(Message message) throws Exception {
    byte[] name = message.getDescriptorForType().getFullName().getBytes(StandardCharsets.UTF_8);
    byte[] bytes = deterministic(message);
    byte[] material = Arrays.copyOf(name, name.length + 1 + bytes.length);
    System.arraycopy(bytes, 0, material, name.length + 1, bytes.length);
    return MessageDigest.getInstance("SHA-256").digest(material);
  }

  private static final class MemoryStore implements SpaceDeletionProofStore {
    private ProofAccount account;
    private final Map<UUID, StoredDeletionProof> rows = new ConcurrentHashMap<>();

    @Override
    public Optional<StoredDeletionProof> findConsumed(UUID operationId) {
      return Optional.ofNullable(rows.get(operationId)).filter(row -> row.receiptBytes() != null);
    }

    @Override
    public Instant acknowledge(DeletionProofAcknowledgement acknowledgement, Instant at) {
      synchronized (rows) {
        StoredDeletionProof row = rows.get(acknowledgement.operationId());
        if (row == null
            || !row.binding().spaceId().equals(acknowledgement.spaceId())
            || row.receiptBytes() == null
            || !UUID.fromString(row.receipt().getReceiptId()).equals(acknowledgement.receiptId())
            || !MessageDigest.isEqual(row.receiptSha256(), acknowledgement.receiptSha256())) {
          throw new DeletionProofDeniedException();
        }
        if (row.acknowledgedAt() != null) {
          return row.acknowledgedAt();
        }
        rows.put(row.binding().operationId(), row.acknowledged(at));
        return at;
      }
    }

    @Override
    public int deleteRetainedReceipts(Instant at) {
      throw new UnsupportedOperationException("JDBC retention behavior only");
    }

    @Override
    public int pseudonymizeAccount(UUID accountId) {
      throw new UnsupportedOperationException("JDBC erasure behavior only");
    }

    @Override
    public <T> T withAccount(UUID accountId, Function<Session, T> action) {
      synchronized (rows) {
        return action.apply(new Session() {
          @Override
          public ProofAccount account() {
            return account;
          }

          @Override
          public Optional<StoredDeletionProof> find(UUID operationId) {
            return Optional.ofNullable(rows.get(operationId));
          }

          @Override
          public void insert(StoredDeletionProof proof) {
            if (rows.putIfAbsent(proof.binding().operationId(), proof) != null) {
              throw new DeletionProofDeniedException();
            }
          }

          @Override
          public void save(StoredDeletionProof proof) {
            rows.put(proof.binding().operationId(), proof);
          }
        });
      }
    }
  }
}
