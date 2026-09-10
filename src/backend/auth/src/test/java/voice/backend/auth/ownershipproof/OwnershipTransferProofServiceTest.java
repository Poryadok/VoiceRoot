package voice.backend.auth.ownershipproof;

import static org.assertj.core.api.Assertions.*;
import static org.mockito.Mockito.*;

import java.time.*;
import java.util.*;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.service.BackupCodeService;
import voice.backend.auth.service.TotpService;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;

class OwnershipTransferProofServiceTest {
  final UUID account = UUID.randomUUID();
  final UUID profile = UUID.randomUUID();
  final Instant now = Instant.parse("2026-09-10T10:00:00Z");
  final BCryptPasswordHasher passwords = mock(BCryptPasswordHasher.class);
  final TotpService totp = mock(TotpService.class);
  final BackupCodeService backup = mock(BackupCodeService.class);
  final SessionEpochFloorStore floors = mock(SessionEpochFloorStore.class);
  final MemoryStore store = new MemoryStore();
  OwnershipTransferProofService service;
  ProofBinding binding;

  @BeforeEach void setUp() {
    binding = new ProofBinding(account, profile, UUID.randomUUID(), UUID.randomUUID(), UUID.randomUUID(), 7);
    store.security = new ProofAccount(account, "hash", new byte[]{1}, false, 7, 1, true);
    when(passwords.matches("correct", "hash")).thenReturn(true);
    when(floors.requireFloor(account)).thenReturn(7L);
    service = serviceAt(now);
  }

  OwnershipTransferProofService serviceAt(Instant at) {
    return new OwnershipTransferProofService(store, passwords, totp, backup, floors, Clock.fixed(at, ZoneOffset.UTC));
  }

  @Test void passwordIsAlwaysRequiredAndOnlyHashAndVerifiedFactorsAreStored() throws Exception {
    assertThatThrownBy(() -> service.issue(binding, "wrong", "", "")).isInstanceOf(ProofDeniedException.class);
    assertThat(store.rows).isEmpty();
    var issued = service.issue(binding, "correct", "", "");
    assertThat(issued.proof()).hasSize(43);
    assertThat(issued.toString()).doesNotContain(issued.proof()).containsIgnoringCase("redacted");
    assertThat(issued.expiresAt()).isEqualTo(now.plusSeconds(300));
    var row = store.rows.get(binding.operationId());
    assertThat(row.proofHash()).isEqualTo(HexFormat.of().formatHex(java.security.MessageDigest.getInstance("SHA-256").digest(issued.proof().getBytes(java.nio.charset.StandardCharsets.UTF_8))));
    assertThat(row.factors()).containsExactly("password");
    verifyNoInteractions(totp, backup);
  }

  @Test void enabledTwoFactorRequiresTotpOrOneUnusedBackupCodeAfterPassword() {
    store.security = new ProofAccount(account, "hash", new byte[]{1}, true, 7, 1, true);
    assertThatThrownBy(() -> service.issue(binding, "wrong", "123456", "backup")).isInstanceOf(ProofDeniedException.class);
    verifyNoInteractions(totp, backup);
    assertThatThrownBy(() -> service.issue(binding, "correct", "bad", "bad")).isInstanceOf(ProofDeniedException.class);
    when(backup.consume(account, "backup")).thenReturn(true).thenReturn(false);
    service.issue(binding, "correct", "", "backup");
    assertThat(store.rows.get(binding.operationId()).factors()).containsExactly("password", "backup_code");
    var next = new ProofBinding(account, profile, binding.spaceId(), binding.newOwnerProfileId(), UUID.randomUUID(), 7);
    assertThatThrownBy(() -> service.issue(next, "correct", "", "backup")).isInstanceOf(ProofDeniedException.class);
  }

  @Test void totpFactorsAreBoundAndBackupIsNotSpent() {
    store.security = new ProofAccount(account, "hash", new byte[]{1}, true, 7, 1, true);
    when(totp.verifyEncrypted(any(), eq("123456"))).thenReturn(true);
    service.issue(binding, "correct", "123456", "backup");
    assertThat(store.rows.get(binding.operationId()).factors()).containsExactly("password", "totp");
    verifyNoInteractions(backup);
  }

  @Test void exactRetryReturnsSameReceiptEvenAfterExpiryButChangedProofFails() {
    var issued = service.issue(binding, "correct", "", "");
    var receipt = service.consume(binding, issued.proof());
    assertThat(serviceAt(now.plusSeconds(301)).consume(binding, issued.proof())).isEqualTo(receipt);
    assertThat(receipt.binding()).isEqualTo(binding);
    assertThat(receipt.consumedAt()).isEqualTo(now);
    assertThatThrownBy(() -> service.consume(binding, issued.proof() + "x")).isInstanceOf(ProofDeniedException.class);
  }

  @Test void everyBindingMismatchDeniesWithoutConsumingOriginalProof() {
    var issued = service.issue(binding, "correct", "", "");
    var mismatches = List.of(
        new ProofBinding(UUID.randomUUID(), profile, binding.spaceId(), binding.newOwnerProfileId(), binding.operationId(), 7),
        new ProofBinding(account, UUID.randomUUID(), binding.spaceId(), binding.newOwnerProfileId(), binding.operationId(), 7),
        new ProofBinding(account, profile, UUID.randomUUID(), binding.newOwnerProfileId(), binding.operationId(), 7),
        new ProofBinding(account, profile, binding.spaceId(), UUID.randomUUID(), binding.operationId(), 7),
        new ProofBinding(account, profile, binding.spaceId(), binding.newOwnerProfileId(), UUID.randomUUID(), 7),
        new ProofBinding(account, profile, binding.spaceId(), binding.newOwnerProfileId(), binding.operationId(), 8));
    for (var mismatch : mismatches) {
      assertThatThrownBy(() -> service.consume(mismatch, issued.proof())).isInstanceOf(ProofDeniedException.class);
    }
    assertThat(service.consume(binding, issued.proof()).binding()).isEqualTo(binding);
    for (var mismatch : mismatches) {
      assertThatThrownBy(() -> service.consume(mismatch, issued.proof())).isInstanceOf(ProofDeniedException.class);
    }
  }

  @Test void expiryBoundaryAndMissingProofDeny() {
    var issued = service.issue(binding, "correct", "", "");
    assertThatThrownBy(() -> serviceAt(now.plusSeconds(300)).consume(binding, issued.proof())).isInstanceOf(ProofDeniedException.class);
    assertThatThrownBy(() -> service.consume(binding, "")).isInstanceOf(ProofDeniedException.class);
    assertThat(serviceAt(now.plusSeconds(299)).consume(binding, issued.proof())).isNotNull();
  }

  @Test void monotonicSecurityRevisionRevokesEvenIfCredentialsReturnToPriorValue() {
    var issued = service.issue(binding, "correct", "", "");
    store.security = new ProofAccount(account, "hash", new byte[]{1}, false, 7, 3, true);
    assertThatThrownBy(() -> service.consume(binding, issued.proof())).isInstanceOf(ProofDeniedException.class);
  }

  @Test void epochAccountStatusAndUnavailableFloorFailClosed() {
    var issued = service.issue(binding, "correct", "", "");
    store.security = new ProofAccount(account, "hash", new byte[]{1}, false, 8, 1, true);
    assertThatThrownBy(() -> service.consume(binding, issued.proof())).isInstanceOf(ProofDeniedException.class);
    store.security = new ProofAccount(account, "hash", new byte[]{1}, false, 7, 1, false);
    assertThatThrownBy(() -> service.consume(binding, issued.proof())).isInstanceOf(ProofDeniedException.class);
    store.security = new ProofAccount(account, "hash", new byte[]{1}, false, 7, 1, true);
    when(floors.requireFloor(account)).thenThrow(new IllegalStateException("offline"));
    assertThatThrownBy(() -> service.consume(binding, issued.proof())).isInstanceOf(IllegalStateException.class);
    assertThat(store.rows.get(binding.operationId()).consumedAt()).isNull();
  }

  @Test void secondIssueCannotReplaceOrLeakOriginalProof() {
    var issued = service.issue(binding, "correct", "", "");
    assertThatThrownBy(() -> service.issue(binding, "correct", "", "")).isInstanceOf(ProofDeniedException.class);
    assertThat(service.consume(binding, issued.proof())).isNotNull();
  }

  @Test void invalidOrAdvancedSessionFloorDeniesIssueAndUnconsumedProof() {
    var issued = service.issue(binding, "correct", "", "");
    for (long floor : new long[]{0, -1, 8}) {
      when(floors.requireFloor(account)).thenReturn(floor);
      var next = new ProofBinding(account, profile, binding.spaceId(), binding.newOwnerProfileId(), UUID.randomUUID(), 7);
      assertThatThrownBy(() -> service.issue(next, "correct", "", "")).isInstanceOf(ProofDeniedException.class);
      assertThatThrownBy(() -> service.consume(binding, issued.proof())).isInstanceOf(ProofDeniedException.class);
    }
    assertThat(store.rows).hasSize(1);
    assertThat(store.rows.get(binding.operationId()).consumedAt()).isNull();
  }

  @Test void missingPasswordAndUnavailableAccountDenyWithoutIssuingProof() {
    assertThatThrownBy(() -> service.issue(binding, "", "", "")).isInstanceOf(ProofDeniedException.class);
    assertThatThrownBy(() -> service.issue(binding, null, "", "")).isInstanceOf(ProofDeniedException.class);
    store.security = new ProofAccount(UUID.randomUUID(), "hash", new byte[]{1}, false, 7, 1, true);
    assertThatThrownBy(() -> service.issue(binding, "correct", "", "")).isInstanceOf(ProofDeniedException.class);
    assertThat(store.rows).isEmpty();
  }
  static class MemoryStore implements OwnershipTransferProofStore {
    public Optional<StoredProof> findConsumed(UUID operation) { return Optional.ofNullable(rows.get(operation)).filter(row -> row.consumedAt() != null); }
    ProofAccount security;
    final Map<UUID, StoredProof> rows = new HashMap<>();
    @Override public synchronized <T> T withAccount(UUID id, java.util.function.Function<Session, T> action) {
      return action.apply(new Session() {
        public ProofAccount account() { return security.accountId().equals(id) ? security : null; }
        public Optional<StoredProof> find(UUID operation) { return Optional.ofNullable(rows.get(operation)); }
        public void insert(StoredProof proof) { rows.put(proof.binding().operationId(), proof); }
        public void markConsumed(UUID operation, Instant at) { rows.put(operation, rows.get(operation).consumed(at)); }
      });
    }
  }
}
