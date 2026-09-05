package voice.backend.auth.service;

import java.time.Instant;
import voice.backend.auth.repository.GuestConversionAdvanceResult;
import voice.backend.auth.repository.GuestConversionOperation;

/** Performs the Auth-local half of a leased guest conversion. */
public interface GuestConversionLocalPromotion {
  GuestConversionAdvanceResult promoteAndAdvance(GuestConversionOperation operation, Instant now);
}
