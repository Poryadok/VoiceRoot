package voice.backend.auth.repository;

import java.time.Instant;
import java.util.List;
import java.util.UUID;

/** Auth-owned durable guest-to-regular conversion operation store. */
public interface GuestConversionOperationRepository {
  GuestConversionOperation createOrResume(UUID accountId, UUID otpCodeId, Instant now);

  List<GuestConversionOperation> leaseDue(int batchSize, Instant now, Instant leaseUntil);
}
