package voice.backend.auth.spacedeletionproof;

import app.voice.auth.v1.ProofPurpose;
import app.voice.auth.v1.SpaceDeletionProofReceipt;
import app.voice.auth.v1.VerifiedFactor;
import java.security.MessageDigest;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.sql.Timestamp;
import java.time.Instant;
import java.util.Arrays;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;
import java.util.function.Function;
import org.springframework.dao.DuplicateKeyException;
import org.springframework.jdbc.core.RowMapper;
import org.springframework.jdbc.core.namedparam.MapSqlParameterSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.TransactionDefinition;
import org.springframework.transaction.support.TransactionTemplate;
import voice.backend.auth.ownershipproof.ProofAccount;

/** PostgreSQL implementation with account-first locking and committed-only receipt recovery. */
public final class JdbcSpaceDeletionProofStore implements SpaceDeletionProofStore {
  private static final RowMapper<StoredDeletionProof> PROOF_ROW =
      (rs, row) -> fromRow(rs);

  private final NamedParameterJdbcTemplate jdbc;
  private final TransactionTemplate transactions;
  private final TransactionTemplate receiptReads;
  private final ReceiptErasureKeyring keyring;

  public JdbcSpaceDeletionProofStore(
      NamedParameterJdbcTemplate jdbc, PlatformTransactionManager manager) {
    this(jdbc, manager, ReceiptErasureKeyring.unavailable());
  }

  public JdbcSpaceDeletionProofStore(
      NamedParameterJdbcTemplate jdbc,
      PlatformTransactionManager manager,
      ReceiptErasureKeyring keyring) {
    this.jdbc = java.util.Objects.requireNonNull(jdbc, "jdbc");
    this.keyring = java.util.Objects.requireNonNull(keyring, "keyring");
    this.transactions = new TransactionTemplate(manager);
    this.receiptReads = new TransactionTemplate(manager);
    this.receiptReads.setPropagationBehavior(TransactionDefinition.PROPAGATION_REQUIRES_NEW);
    this.receiptReads.setIsolationLevel(TransactionDefinition.ISOLATION_READ_COMMITTED);
    this.receiptReads.setReadOnly(true);
  }

  @Override
  public Optional<StoredDeletionProof> findConsumed(UUID operationId) {
    return receiptReads.execute(status -> queryConsumed(operationId));
  }

  @Override
  public Optional<StoredDeletionProof> findConsumed(DeletionProofLookup lookup) {
    return receiptReads.execute(status -> {
      Optional<StoredDeletionProof> found = queryConsumed(lookup.operationId());
      if (found.isEmpty()) return found;
      StoredDeletionProof row = found.get();
      if (row.binding().accountId() != null) return found;
      Integer version = row.receiptHmacKeyVersion();
      byte[] expected = row.receiptLookupHmac();
      if (version == null || expected == null) throw new ReceiptErasureKeyUnavailableException(0);
      byte[] key = keyring.requireKey(version);
      var candidate = new StoredDeletionProof.StoredBinding(
          lookup.accountId(), lookup.profileId(), row.binding().sessionEpoch(), row.binding().spaceId(),
          row.binding().operationId(), row.binding().confirmationNameSha256(),
          row.binding().proofDigestSha256(), row.binding().verifiedFactors(),
          ProofPurpose.PROOF_PURPOSE_SPACE_DELETE);
      byte[] actual = ReceiptErasureIndex.hmacSha256(
          key, SpaceDeletionProofService.deterministic(
              SpaceDeletionProofService.canonicalBinding(candidate)));
      return MessageDigest.isEqual(expected, actual) ? found : Optional.empty();
    });
  }

  @Override
  public Instant acknowledge(DeletionProofAcknowledgement acknowledgement, Instant at) {
    return transactions.execute(status -> {
      StoredDeletionProof row = jdbc.query(
          "SELECT * FROM space_deletion_proofs WHERE operation_id=:operation FOR UPDATE",
          Map.of("operation", acknowledgement.operationId()), PROOF_ROW)
          .stream().findFirst().orElseThrow(DeletionProofDeniedException::new);
      if (row.consumedAt() == null || row.receipt() == null
          || !row.binding().spaceId().equals(acknowledgement.spaceId())
          || !row.receiptId().equals(acknowledgement.receiptId())
          || !MessageDigest.isEqual(row.receiptSha256(), acknowledgement.receiptSha256())) {
        throw new DeletionProofDeniedException();
      }
      if (row.acknowledgedAt() != null) return row.acknowledgedAt();
      int changed = jdbc.update("""
          UPDATE space_deletion_proofs SET acknowledged_at=:at
          WHERE operation_id=:operation AND acknowledged_at IS NULL
          """, Map.of("at", Timestamp.from(at), "operation", acknowledgement.operationId()));
      if (changed != 1) throw new DeletionProofDeniedException();
      return at;
    });
  }

  @Override
  public int deleteRetainedReceipts(Instant at) {
    return transactions.execute(status -> jdbc.update("""
        DELETE FROM space_deletion_proofs
        WHERE acknowledged_at IS NOT NULL
          AND :at >= GREATEST(consumed_at + INTERVAL '30 days',
                              acknowledged_at + INTERVAL '24 hours')
        """, Map.of("at", Timestamp.from(at))));
  }

  @Override
  public int pseudonymizeAccount(UUID accountId) {
    int version = keyring.activeVersion();
    if (version <= 0) throw new ReceiptErasureKeyUnavailableException(version);
    byte[] key = keyring.requireKey(version);
    return transactions.execute(status -> {
      jdbc.query("SELECT id FROM accounts WHERE id=:id FOR UPDATE", Map.of("id", accountId),
          (rs, row) -> rs.getObject("id", UUID.class));
      List<StoredDeletionProof> rows = jdbc.query(
          "SELECT * FROM space_deletion_proofs WHERE account_id=:id FOR UPDATE",
          Map.of("id", accountId), PROOF_ROW);
      jdbc.update("DELETE FROM space_deletion_proofs WHERE account_id=:id AND consumed_at IS NULL",
          Map.of("id", accountId));
      int changed = 0;
      for (StoredDeletionProof row : rows) {
        if (row.consumedAt() == null) continue;
        byte[] bindingBytes = row.bindingBytes();
        if (bindingBytes == null) throw new IllegalStateException("raw receipt binding unavailable");
        byte[] index = ReceiptErasureIndex.hmacSha256(key, bindingBytes);
        changed += jdbc.update("""
            UPDATE space_deletion_proofs
            SET account_id=NULL, profile_id=NULL, binding_bytes=NULL,
                receipt_lookup_hmac=:lookup, receipt_hmac_key_version=:version
            WHERE operation_id=:operation AND account_id=:account
            """, new MapSqlParameterSource()
            .addValue("lookup", index).addValue("version", version)
            .addValue("operation", row.binding().operationId()).addValue("account", accountId));
      }
      return changed;
    });
  }

  @Override
  public <T> T withAccount(UUID accountId, Function<Session, T> action) {
    try {
      return transactions.execute(status -> {
        jdbc.query("SELECT id FROM accounts WHERE id=:id FOR UPDATE", Map.of("id", accountId),
            (rs, row) -> rs.getObject("id", UUID.class));
        return action.apply(new JdbcSession(accountId));
      });
    } catch (DuplicateKeyException conflict) {
      throw new DeletionProofDeniedException();
    }
  }

  private Optional<StoredDeletionProof> queryConsumed(UUID operationId) {
    return jdbc.query("""
        SELECT * FROM space_deletion_proofs
        WHERE operation_id=:operation AND consumed_at IS NOT NULL
        """, Map.of("operation", operationId), PROOF_ROW).stream().findFirst();
  }

  private final class JdbcSession implements Session {
    private final UUID accountId;

    private JdbcSession(UUID accountId) { this.accountId = accountId; }

    @Override
    public ProofAccount account() {
      return jdbc.query("""
          SELECT id,password_hash,totp_secret,totp_enabled,session_epoch,
                 security_revision,status,deleted_at
          FROM accounts WHERE id=:id
          """, Map.of("id", accountId), (rs, row) -> new ProofAccount(
              rs.getObject("id", UUID.class), rs.getString("password_hash"),
              rs.getBytes("totp_secret"), rs.getBoolean("totp_enabled"),
              rs.getLong("session_epoch"), rs.getLong("security_revision"),
              "active".equals(rs.getString("status")) && rs.getTimestamp("deleted_at") == null))
          .stream().findFirst().orElse(null);
    }

    @Override
    public Optional<StoredDeletionProof> find(UUID operationId) {
      return jdbc.query("SELECT * FROM space_deletion_proofs WHERE operation_id=:operation",
          Map.of("operation", operationId), PROOF_ROW).stream().findFirst();
    }

    @Override
    public void insert(StoredDeletionProof proof) {
      var binding = proof.binding();
      jdbc.update("""
          INSERT INTO space_deletion_proofs
            (operation_id,account_id,profile_id,session_epoch,space_id,
             confirmation_name_sha256,proof_digest_sha256,security_revision,
             verified_factors,expires_at,receipt_id,binding_bytes,binding_sha256)
          VALUES
            (:operation,:account,:profile,:epoch,:space,:name_digest,:proof_digest,
             :revision,:factors,:expires,:receipt,:binding_bytes,:binding_digest)
          """, new MapSqlParameterSource()
          .addValue("operation", binding.operationId()).addValue("account", binding.accountId())
          .addValue("profile", binding.profileId()).addValue("epoch", binding.sessionEpoch())
          .addValue("space", binding.spaceId())
          .addValue("name_digest", binding.confirmationNameSha256())
          .addValue("proof_digest", binding.proofDigestSha256())
          .addValue("revision", proof.securityRevision())
          .addValue("factors", encodeFactors(binding.verifiedFactors()))
          .addValue("expires", Timestamp.from(proof.expiresAt()))
          .addValue("receipt", proof.receiptId())
          .addValue("binding_bytes", proof.bindingBytes())
          .addValue("binding_digest", proof.bindingSha256().toByteArray()));
    }

    @Override
    public void save(StoredDeletionProof proof) {
      int changed = jdbc.update("""
          UPDATE space_deletion_proofs
          SET consumed_at=:consumed, receipt_bytes=:receipt_bytes, receipt_sha256=:receipt_digest
          WHERE operation_id=:operation AND account_id=:account AND consumed_at IS NULL
          """, new MapSqlParameterSource()
          .addValue("consumed", Timestamp.from(proof.consumedAt()))
          .addValue("receipt_bytes", proof.receiptBytes())
          .addValue("receipt_digest", proof.receiptSha256())
          .addValue("operation", proof.binding().operationId())
          .addValue("account", accountId));
      if (changed != 1) throw new DeletionProofDeniedException();
    }
  }

  private static StoredDeletionProof fromRow(ResultSet rs) throws SQLException {
    UUID accountId = rs.getObject("account_id", UUID.class);
    UUID profileId = rs.getObject("profile_id", UUID.class);
    List<VerifiedFactor> factors = decodeFactors(rs.getString("verified_factors"));
    var binding = new StoredDeletionProof.StoredBinding(
        accountId, profileId, rs.getLong("session_epoch"), rs.getObject("space_id", UUID.class),
        rs.getObject("operation_id", UUID.class), rs.getBytes("confirmation_name_sha256"),
        rs.getBytes("proof_digest_sha256"), factors, ProofPurpose.PROOF_PURPOSE_SPACE_DELETE);
    byte[] receiptBytes = rs.getBytes("receipt_bytes");
    SpaceDeletionProofReceipt receipt = null;
    if (receiptBytes != null) {
      try {
        receipt = SpaceDeletionProofReceipt.parseFrom(receiptBytes);
      } catch (com.google.protobuf.InvalidProtocolBufferException malformed) {
        throw new SQLException("invalid stored deletion receipt", malformed);
      }
    }
    int rawVersion = rs.getInt("receipt_hmac_key_version");
    Integer version = rs.wasNull() ? null : rawVersion;
    return new StoredDeletionProof(
        binding, rs.getLong("security_revision"), rs.getTimestamp("expires_at").toInstant(),
        rs.getObject("receipt_id", UUID.class), instant(rs, "consumed_at"),
        rs.getBytes("binding_bytes"), com.google.protobuf.ByteString.copyFrom(
            rs.getBytes("binding_sha256")), receipt,
        receiptBytes, rs.getBytes("receipt_sha256"), instant(rs, "acknowledged_at"),
        rs.getBytes("receipt_lookup_hmac"), version);
  }

  private static Instant instant(ResultSet rs, String column) throws SQLException {
    Timestamp value = rs.getTimestamp(column);
    return value == null ? null : value.toInstant();
  }

  private static String encodeFactors(List<VerifiedFactor> factors) {
    return factors.stream().map(factor -> switch (factor) {
      case VERIFIED_FACTOR_PASSWORD -> "password";
      case VERIFIED_FACTOR_TOTP -> "totp";
      case VERIFIED_FACTOR_BACKUP_CODE -> "backup_code";
      default -> throw new IllegalArgumentException("unsupported verified factor");
    }).collect(java.util.stream.Collectors.joining(","));
  }

  private static List<VerifiedFactor> decodeFactors(String encoded) {
    return Arrays.stream(encoded.split(",")).map(factor -> switch (factor) {
      case "password" -> VerifiedFactor.VERIFIED_FACTOR_PASSWORD;
      case "totp" -> VerifiedFactor.VERIFIED_FACTOR_TOTP;
      case "backup_code" -> VerifiedFactor.VERIFIED_FACTOR_BACKUP_CODE;
      default -> throw new IllegalArgumentException("invalid stored verified factor");
    }).toList();
  }
}
