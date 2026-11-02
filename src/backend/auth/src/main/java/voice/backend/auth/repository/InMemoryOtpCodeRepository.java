package voice.backend.auth.repository;

import java.time.Instant;
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;
import java.util.UUID;

public class InMemoryOtpCodeRepository implements OtpCodeRepository {
  private final List<OtpCodeRecord> records = new ArrayList<>();

  @Override
  public synchronized OtpCodeRecord create(
      UUID accountId, String codeHash, String type, Instant expiresAt, Instant now) {
    records.replaceAll(current -> current.accountId().equals(accountId)
        && current.type().equals(type) && current.usedAt() == null
        ? new OtpCodeRecord(current.id(), current.accountId(), current.codeHash(),
            current.type(), current.expiresAt(), now) : current);
    OtpCodeRecord record =
        new OtpCodeRecord(UUID.randomUUID(), accountId, codeHash, type, expiresAt, null);
    records.add(record);
    return record;
  }

  @Override
  public synchronized Optional<OtpCodeRecord> findLatestValid(UUID accountId, String type, Instant now) {
    for (int i = records.size() - 1; i >= 0; i--) {
      var record = records.get(i);
      if (record.accountId().equals(accountId) && record.type().equals(type)) {
        return Optional.of(record).filter(r -> r.isUsable(now));
      }
    }
    return Optional.empty();
  }

  @Override
  public synchronized void markUsed(UUID id, Instant usedAt) {
    for (int i = 0; i < records.size(); i++) {
      OtpCodeRecord current = records.get(i);
      if (current.id().equals(id) && current.isUsable(usedAt)) {
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
    throw new voice.backend.auth.service.AuthException("invalid_otp");
  }
}
