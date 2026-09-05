package voice.backend.auth.service;

import java.time.Instant;
import java.util.UUID;
import voice.backend.auth.repository.Account;

/** Atomically begins (or resumes) one account deletion generation. */
public interface AccountDeletionOperationStarter {
  AccountDeletionStartResult startOrResume(
      Account account, UUID proposedOperationId, String restoreTokenHash, Instant now);
}
