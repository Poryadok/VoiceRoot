package voice.backend.auth.repository;

import java.time.Instant;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.List;
import java.util.Optional;
import java.util.UUID;

public class InMemoryOtpCodeRepository implements OtpCodeRepository {
  private final List<OtpCodeRecord> records = new ArrayList<>();

  @Override
  public synchronized OtpCodeRecord create(
      UUID accountId, String codeHash, String type, Instant expiresAt, Instant now) {
    OtpCodeRecord record =
        new OtpCodeRecord(UUID.randomUUID(), accountId, codeHash, type, expiresAt, null);
    records.add(record);
    return record;
  }

  @Override
  public synchronized Optional<OtpCodeRecord> findLatestValid(UUID accountId, String type, Instant now) {
    return records.stream()
        .filter(r -> r.accountId().equals(accountId) && r.type().equals(type) && r.isUsable(now))
        .max(Comparator.comparing(OtpCodeRecord::expiresAt));
  }

  @Override
  public synchronized void markUsed(UUID id, Instant usedAt) {
    for (int i = 0; i < records.size(); i++) {
      OtpCodeRecord current = records.get(i);
      if (current.id().equals(id)) {
        records.set(
            i,
            new OtpCodeRecord(
                current.id(),
                current.accountId(),
                current.codeHash(),
                current.type(),
                current.expiresAt(),
                usedAt));
        return;
      }
    }
  }
}
