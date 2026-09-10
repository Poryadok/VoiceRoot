package voice.backend.auth.ownershipproof;

import java.sql.Timestamp;
import java.time.Instant;
import java.util.Arrays;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;
import java.util.function.Function;
import org.springframework.dao.DuplicateKeyException;
import org.springframework.jdbc.core.namedparam.MapSqlParameterSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.TransactionDefinition;
import org.springframework.transaction.support.TransactionTemplate;

/** PostgreSQL transaction serializes proof writes against every account security mutation. */
public final class JdbcOwnershipTransferProofStore implements OwnershipTransferProofStore {
  private static final org.springframework.jdbc.core.RowMapper<StoredProof> PROOF_ROW = (rs, row) -> new StoredProof(
              new ProofBinding(rs.getObject("account_id", UUID.class), rs.getObject("profile_id", UUID.class),
                  rs.getObject("space_id", UUID.class), rs.getObject("new_owner_profile_id", UUID.class),
                  rs.getObject("operation_id", UUID.class), rs.getLong("session_epoch")),
              rs.getString("proof_hash"), rs.getLong("security_revision"),
              Arrays.asList(rs.getString("verified_factors").split(",")), rs.getTimestamp("expires_at").toInstant(),
              rs.getObject("receipt_id", UUID.class),
              rs.getTimestamp("consumed_at") == null ? null : rs.getTimestamp("consumed_at").toInstant());

  private final NamedParameterJdbcTemplate jdbc;
  private final TransactionTemplate transactions;
  private final TransactionTemplate receiptReads;

  public JdbcOwnershipTransferProofStore(NamedParameterJdbcTemplate jdbc, PlatformTransactionManager manager) {
    this.jdbc = jdbc;
    this.transactions = new TransactionTemplate(manager);
    this.receiptReads = new TransactionTemplate(manager);
    this.receiptReads.setPropagationBehavior(TransactionDefinition.PROPAGATION_REQUIRES_NEW);
    this.receiptReads.setIsolationLevel(TransactionDefinition.ISOLATION_READ_COMMITTED);
    this.receiptReads.setReadOnly(true);
  }

  @Override public Optional<StoredProof> findConsumed(UUID operationId) {
    // Suspend any caller transaction so its own uncommitted consume cannot become recovery authority.
    return receiptReads.execute(status -> jdbc.query(
        "SELECT * FROM ownership_transfer_proofs WHERE operation_id=:operation AND consumed_at IS NOT NULL",
        Map.of("operation", operationId), PROOF_ROW).stream().findFirst());
  }
  @Override public <T> T withAccount(UUID id, Function<Session, T> action) {
    try {
      return transactions.execute(status -> {
        jdbc.query("SELECT id FROM accounts WHERE id=:id FOR UPDATE", Map.of("id", id),
            (rs, row) -> rs.getObject("id", UUID.class));
        return action.apply(new JdbcSession(id));
      });
    } catch (DuplicateKeyException conflict) {
      // Includes races where different accounts attempted to reuse the same operation UUID.
      throw new ProofDeniedException();
    }
  }

  private final class JdbcSession implements Session {
    private final UUID id;
    JdbcSession(UUID id) { this.id = id; }

    @Override public ProofAccount account() {
      return jdbc.query("""
          SELECT id,password_hash,totp_secret,totp_enabled,session_epoch,security_revision,status,deleted_at
          FROM accounts WHERE id=:id
          """, Map.of("id", id), (rs, row) -> new ProofAccount(rs.getObject("id", UUID.class),
              rs.getString("password_hash"), rs.getBytes("totp_secret"), rs.getBoolean("totp_enabled"),
              rs.getLong("session_epoch"), rs.getLong("security_revision"),
              "active".equals(rs.getString("status")) && rs.getTimestamp("deleted_at") == null))
          .stream().findFirst().orElse(null);
    }

    @Override public Optional<StoredProof> find(UUID operation) {
      return jdbc.query("SELECT * FROM ownership_transfer_proofs WHERE operation_id=:operation",
          Map.of("operation", operation), PROOF_ROW)
          .stream().findFirst();
    }

    @Override public void insert(StoredProof proof) {
      ProofBinding binding = proof.binding();
      jdbc.update("""
          INSERT INTO ownership_transfer_proofs
          (operation_id,account_id,profile_id,space_id,new_owner_profile_id,session_epoch,
           proof_hash,security_revision,verified_factors,expires_at,receipt_id)
          VALUES (:operation,:account,:profile,:space,:target,:epoch,:hash,:revision,:factors,:expires,:receipt)
          """, new MapSqlParameterSource()
          .addValue("operation", binding.operationId()).addValue("account", binding.accountId())
          .addValue("profile", binding.profileId()).addValue("space", binding.spaceId())
          .addValue("target", binding.newOwnerProfileId()).addValue("epoch", binding.sessionEpoch())
          .addValue("hash", proof.proofHash()).addValue("revision", proof.securityRevision())
          .addValue("factors", String.join(",", proof.factors()))
          .addValue("expires", Timestamp.from(proof.expiresAt())).addValue("receipt", proof.receiptId()));
    }

    @Override public void markConsumed(UUID operation, Instant at) {
      int changed = jdbc.update("""
          UPDATE ownership_transfer_proofs SET consumed_at=:at
          WHERE operation_id=:operation AND account_id=:account AND consumed_at IS NULL
          """, Map.of("at", Timestamp.from(at), "operation", operation, "account", id));
      if (changed != 1) throw new ProofDeniedException();
    }
  }
}
