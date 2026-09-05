package voice.backend.auth.repository;

/** Persisted account-deletion completion stages. */
public enum AccountDeletionState {
  PENDING_FLOOR,
  PENDING_EVENT,
  COMPLETED
}
