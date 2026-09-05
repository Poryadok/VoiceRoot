package voice.backend.auth.repository;

/** Fenced completion outcome for an account-deletion stage. */
public enum AccountDeletionAdvanceResult {
  APPLIED,
  ALREADY_APPLIED,
  LEASE_LOST,
  NOT_FOUND
}
