package voice.backend.auth.repository;

/** Result of a fenced guest-conversion state transition. */
public enum GuestConversionAdvanceResult {
  /** The caller's lease was current and the requested transition was persisted. */
  APPLIED,

  /** A prior caller already completed this transition or a later valid transition. */
  ALREADY_APPLIED,

  /** The operation still needs the requested transition, but the caller no longer owns its lease. */
  LEASE_LOST,

  /** No guest-conversion operation exists for the requested operation ID. */
  NOT_FOUND
}
